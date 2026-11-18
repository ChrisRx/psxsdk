# Milestone 18: the rest of the C library's entries

Status: probed on Sony's library, nothing implemented. These are entries the
clean image lacks that original C programs built for Sony's library call at fixed
addresses (spec 15, stage 2): malloc, free, realloc, calloc with InitHeap, getc,
getchar, gets, putc, Krom2RawAdd, Krom2RawAdd2, MulMatrix, SetLightMatrix, CdControlF,
CdInit, CdStatus, CdSync and CdReady.

Everything here comes from calling Sony's entries on the emulator, under OpenBIOS
unless a line says otherwise. Values are hexadecimal where written with an h.
The probes are in `spec/probes/`, and each is run from the repository root as
`SIDES=sony exp/cleanlib/image/probe.sh NAME exp/cleanlib/spec/probes/NAME`.

## Two things about the probes

- In a Go program for Sony's target, the names malloc, free and calloc resolve at
  link time to the Go runtime's, not the library's. So what stage 2 first saw (an
  address beyond the program, and "first address plus 16" from a static array) was
  TinyGo's allocator, which ignores InitHeap. The heap probes reach the library's
  through its entry addresses, as func values. A C program calls the library's
  entries, so these are the rules that count. The entries below are called the same
  way on the clean image, so the probes compare.
- A probe's line "END" ends its run, with or without the "\r" that `println` and
  write put before the "\n".

## The heap

### Who sets it

InitHeap belongs to the program's start-up, not the library's init routine
(`heap`, with PROBE_WATCH on the entry). On a Go program for Sony's target it is
called twice before main, both from the program's own code:

1. the target's start-up, InitHeap(_end, sp - 10000h - _end), where sp is the value
   at the entry (801FF000h here, so a size of 154704h with _end 8009A8FCh);
2. the Go runtime's main, InitHeap(_end, 4000h), the reserve in the link script.

A C program gets the first alone. Nothing in the library calls InitHeap, malloc or
any kernel allocator: no A0h 33h to 39h, or any other kernel call, appears in any
heap probe (`heapcost`'s log holds one B0h 17h from the vertical blank).

### State

The heap keeps four values in the library's own data, and the rest in the heap:

- `H`, the start, and `E`, the end: H + size, wrapping at 32 bits.
- `S`, where the next search starts.
- `T`, the top, where the next block made from unused memory goes.

A block is a word (the header) followed by its bytes, whose count is always a
multiple of 4 when the program passes sizes through these calls. A header holds the
byte count, with bit 0 set if the block is free. The word FFFFFFFEh is the end mark,
normally at T. Returned pointers are the address just past a header, so a block is
aligned as H is, which must be a multiple of 4.

### InitHeap(head, size)

Sets H = head, E = head + size, S = H, T = H, and writes the end mark at H. Nothing
else is written: size may be anything, a second call forgets all that went before,
and what the last heap wrote stays in memory (`heap`, "init twice"). It may move the
heap at any time. A head that is not a multiple of 4 faults on the store and halts
the program under the kernel (`heapodd`): a crash that need not be matched.

### malloc(n)

1. n = 0 returns 0 and changes nothing. This is tested before rounding.
2. n = (n + 3) with the low 2 bits cleared, in 32 bits. So FFFFFFFDh to FFFFFFFFh
   become 0 and go on: they make a block of no bytes at the top (`heap`, "huge").
3. p = S. Then repeat, reading the word h at p:
   - h = FFFFFFFEh: go to the top case. It is the word that decides, not p = T: a
     search that meets this word anywhere uses T, and one that passes T because a
     header was overwritten goes on (`heapbad`).
   - h with bit 0 clear: p += 4 + h. A header is taken at its value: a size that is
     not a multiple of 4 reaches an address that faults, and FFFFFFFCh never ends.
   - h with bit 0 set: size = h with bit 0 clear. While size < n, unsigned, look at
     the word w at p + 4 + size:
     - w = FFFFFFFEh: write FFFFFFFEh at p, set T = p, and go to the top case. So a
       free run reaching the end mark gives its memory back, whether or not the
       request then fits (`heap`, "merge fail"). The old end marks stay in memory.
     - w with bit 0 set: size += (w with bit 0 clear) + 4, and write size with bit 0
       set at p. The block joined keeps its old header in memory.
     - w with bit 0 clear: stop joining.

     If size >= n: when size - n >= 4, write (size - n - 4) with bit 0 set at
     p + 4 + n, and n at p; otherwise write size at p. Return p + 4. S stays where it
     was. A remainder of no bytes is made, as the header 00000001h. If size < n still,
     p += 4 + size and the search goes on.
4. The top case. If n > E - T - 8, unsigned and wrapping: set S = H and return 0.
   Otherwise write n at T, set T to T + 4 + n, write the end mark there, set S = T,
   and return the old T + 4.

What follows from these rules, each shown by the probes:

- It is first fit from S, and joins only while it must. Free blocks are not joined
  by free, nor beyond what a request needs.
- A block from the top moves S up to the new top, past any free block a search just
  went by because it was too small. Those blocks are found again only once a free
  puts S lower, or a failed malloc puts S at H. So malloc(16) after freeing an 8-byte
  block leaves that block unused by a later malloc(4), which takes the top (`heaplow`,
  "a free block too small, then the top").
- The limit is 8 bytes short of E for the top's room, with the subtraction wrapping:
  T = E - 8 refuses every n, while T = E - 4 or beyond gives a huge room, so every n
  up to FFFFFFFCh is taken and the heap runs on past its end. The grid in `heap`
  shows this for sizes 16, 32, 64 and 68. A heap of size under 8 takes any request at
  once. With size 4096: malloc(4088) returns H + 4, then malloc(4) works, at H + 4096;
  malloc(4084) returns H + 4, then malloc(4) returns 0; malloc(4092) returns 0.
- The size's low bits make no difference, since T stays a multiple of 4.

### free(p)

Sets bit 0 of the word at p - 4, and if p - 4 < S, unsigned, sets S = p - 4. Nothing
else: no range check and no joining. So:

- A second free of a block changes nothing, unless S has moved up past it, when S
  comes back down.
- free(0) has no effect the program sees. p = 0 is p - 4 = FFFFFFFCh; do nothing.
- Any pointer is taken: its word before it gets bit 0, and if it lies below S the
  next search starts there and treats it as a block (`heapbad`, "before the heap"; a
  pointer into a block's bytes the same). Above S it is left alone by later searches.
- A pointer that is not a multiple of 4 faults on the load and halts (`heapoddfree`):
  need not be matched.

### realloc(p, n)

1. p = 0: malloc(n).
2. n = 0: free(p), and return 0. (`heap` saw 0. `heaplow` saw p - 4 from the same call
   in another program, so the value is not one a program can use. Return 0.)
3. r = n rounded up as in malloc, 32 bits. h = the word at p - 4 with bit 0 clear.
4. As signed numbers, r <= h: the block shrinks in place. When h - r >= 4 (unsigned),
   write (h - r - 4) with bit 0 set at p + r, then r at p - 4; otherwise write nothing.
   Return p. S and T do not change, so a remainder above S is found only by a search
   that starts below it, and one above S stays unused while S stays higher: see the
   sequence "realloc shrink" in `heaplow`. A negative n is a shrink: FFFFFFFCh writes
   the remainder's header at p - 4 and then r over it. FFFFFFFDh to FFFFFFFFh round to
   0, leaving a header of 0 and a free remainder.
5. Otherwise size = h, and repeat, reading w at p + size:
   - w = FFFFFFFEh: if r <= E - (p - 4) - 8, unsigned and wrapping: write r at p - 4,
     and if S = T, set S to p + r. Set T = p + r, write the end mark there, and return
     p. Otherwise go to step 6.
   - w with bit 0 set: size += (w with bit 0 clear) + 4, and write size at p - 4. If
     now size >= r: when size - r >= 4, write (size - r - 4) with bit 0 set at p + r
     and r at p - 4. Return p. S does not change.
   - w with bit 0 clear: go to step 6.
6. q = malloc(n), with the block still marked used. q = 0: return 0, the block
   keeping any size it gained above. Otherwise copy n bytes (the n passed, not rounded,
   and not the old size) from p to q, a byte at a time upward, then free(p), and return
   q.

What follows:

- A move reads past the old block: the bytes after it come along, and where the new
  block lies within reach of the old, what is read may be what was just written
  (`heap`, "realloc grow, the next block used", n 64). Only the old size's bytes are
  what the program put there.
- S is not corrected when a free block is taken in place. If S was at the free block
  that was joined, a later malloc starts inside the grown block, reads the old header
  still there, and returns memory that overlaps it. `heap`: malloc 16 three times,
  free the second, realloc(first, 20), then malloc(4) returns the second's old
  address, inside the first. This is the behavior, and it must be kept.
- A failed move leaves the block with its joined size, used. If the joined run met the
  top, malloc's search can set T below the block's end as it fails. `heaplow`, "realloc
  fails, the next up to the top": header 24h at H, and the next malloc(4) returns
  H + 24, inside the block.
- On a freed block, h clears bit 0, and a call that writes nothing leaves it marked
  free: realloc(freed, its size) returns p and it stays free.

### calloc(n, m)

The product n * m in 32 bits, overflow ignored, is passed to malloc. A result of 0
returns 0. Otherwise exactly that many bytes are cleared, not the rounded size, and
the block is returned: calloc(3, 3) leaves the last byte of its 12-byte block alone,
and calloc(10001h, 10000h) asks for 65536 bytes. (`heap`, "calloc", "calloc over dirty
memory", "calloc overflow".)

### What was checked

A model of these rules, run over each probe's calls, gives every result and every word
of the arena in `heap` (897 lines, with a mixed sequence of 120 calls in `mixed`) and
in `heaplow` (465 lines, with the search's starting place after every call). None
differ. `heapbad` was read by hand against it. The test sequence that pins the rules is
`heap` as a whole. An implementation matches when `heap` and `heaplow` print the same
lines on the clean image, the `cost` lines apart.

### Cost

Cycles from entry to return, on the emulator's clock (`heapcost` with PROBE_WATCH, the
last of three runs). The increments come from `heap`'s `cost` lines, second counter,
system clock, least of 7.

| call | cycles |
|---|---|
| InitHeap | 42 |
| malloc(0) | 38 |
| malloc from the top, S at the top | 144 (the first after InitHeap 148) |
| malloc, a free block taken whole | 94 |
| malloc, a free block split | 106 |
| each free block passed that is too small, with its used block after it | + 80 |
| each join | + 62 |
| malloc that fails at once, S at the top | 184 |
| free | 24, or 28 when it lowers S |
| free(0) | 24 |
| realloc, same size | 66 |
| realloc, shrink | 102 |
| realloc(0, n) | 40 + malloc |
| realloc by a move | 154 + malloc + free + 12 per byte copied |
| calloc | 56 + malloc + 8 per byte cleared |

A long list costs nothing when S is at the top: 186 on the counter after any number of
used blocks. Whether these calls turn interrupts off was not looked at (see Open).

## putc, getc, getchar, gets

### putc(c, fd)

Writes the low byte of c through the kernel's write (B0h 35h) as write(fd, buffer on
the stack, 1), one call, and returns what write returned (`stdio`). A "\n" is not
expanded: putc('\n', 1) is one call, where the library's putchar makes two, "\r" and
"\n" (the kernel's write then adds another "\r" before "\n" on the TTY). On
OpenBIOS write returns 0 for files 0 and 1, whatever the length, and -1 for any other,
and file 0 prints nothing. So putc returns 0 or -1 there.

putchar likewise returns write's result, 0 on OpenBIOS (`stdio`). The clean library's
putchar returns c, a difference no test has shown so far: match Sony's when putc goes in.

### getc(fd)

Calls read(fd, buffer on the stack, 1) through B0h 34h, once. A result above 0 returns
the byte, unsigned, from 0 to 255. A result of 0 or less returns 255, whatever the
buffer held (`stdin`).

### getchar()

Calls read(0, buffer on the stack, 1), once, ignores its result, and returns the byte
with bit 7 cleared. If the read wrote nothing, that is whatever was on the stack: with
no input on either kernel the value is stale, so match the call and not that value.
There is no echo.

### gets(buf)

Reads one byte at a time as getchar does, ignoring read's result, and returns buf.
Each byte, with bit 7 cleared:

| byte | action |
|---|---|
| 0Ah, 0Dh | putchar('\n'), which puts "\r" and "\n"; write NUL at buf[count]; return buf |
| 08h, 7Fh | if count > 0: count -= 1, and putchar of 08h, 20h, 08h. If count = 0: nothing |
| 16h | nothing now; the next byte is stored as below, whatever it is, even 0Ah, 08h or 16h |
| 09h | treated as 20h |
| 20h to 7Eh, or any byte after 16h | if count < 125: buf[count] = byte, count += 1, putchar(byte). If count = 125: putchar(07h) |
| any other | putchar(07h), not stored |

The byte after 16h is masked to 7 bits as well, and the echo of a stored 0Ah goes
through putchar so it shows as "\r\n". An erased byte stays in the buffer: only the NUL
marks the end. So a buffer needs 126 bytes, and a longer line stores 125 and rings the rest
(`stdin`, up to 130).

### Blocking

On the emulator under OpenBIOS and under SCPH-1001 (PSX_BIOS), the kernel's read of
file 0 returns at once with nothing written. So getc and getchar return at once with
255 and with stale stack, and gets loops forever on its stale byte, echoing it until
the buffer fills and then ringing, each read and write about 8,500 cycles. A kernel
that blocks would block in the same call. `stdin` puts a read of its own in front of
the kernel's B0 vector to give input.

## Krom2RawAdd and Krom2RawAdd2

### Krom2RawAdd(code)

Behaves as the kernel's function B0h 51h called directly: a0, ra and the stack reach the
kernel as the caller set them, and its result comes back to the caller unchanged (`krom`:
one kernel call each, whose ra is the program's). The code goes
whole in a0, and the kernel reads what it will. Do not build a table. Results depend on
the kernel: SCPH-1001 gives other addresses for the cells with no glyph (817Fh gives
BFD58CE2h where OpenBIOS gives BFC66762h), the same for those with one. About 330
cycles a call on OpenBIOS.

Under OpenBIOS, for reference and for checking, every result is first + 30 * (code -
run's first code) over these runs, and FFFFFFFFh below 8140h, from 84BFh to 889Eh and
above 9872h. Bits above the 16th do not matter: 18140h, FFFF8140h give the same as
8140h, and 81400000h as 0.

```
8140-817F BFC66000       8492-849E BFC6C39C
8180-81AC BFC66762       849F-84BE BFC699A8
81AD-81B7 BFC66CC6       889F-88FF BFC69D68
81B8-81BF BFC66CA8       8900-897E BFC6A0EC
81C0-81C7 BFC66F00       897F-89FF BFC6AFB0
81C8-81CE BFC66D98       8A00-8A7E BFC6B6F4
81CF-81D9 BFC670C2       8A7F-8AFF BFC6C5B8
81DA-81E8 BFC66E6A       8B00-8B7E BFC6CCFC
81E9-81EF BFC673CE       8B7F-8BFF BFC6DBC0
81F0-81F7 BFC6702C       8C00-8C7E BFC6E304
81F8-81FB BFC67590       8C7F-8CFF BFC6F1C8
81FC-81FC BFC6711C       8D00-8D7E BFC6F90C
81FD-824E BFC67626       8D7F-8DFF BFC707D0
824F-8258 BFC6713A       8E00-8E7E BFC70F14
8259-825F BFC680EE       8E7F-8EFF BFC71DD8
8260-8279 BFC67266       8F00-8F7E BFC7251C
827A-8280 BFC684CC       8F7F-8FFF BFC733E0
8281-829A BFC67572       9000-907E BFC73B24
829B-829E BFC688AA       907F-90FF BFC749E8
829F-82F1 BFC6787E       9100-917E BFC7512C
82F2-833F BFC692DC       917F-91FF BFC75FF0
8340-837E BFC68238       9200-927E BFC76734
837F-837F BFC6A362       927F-92FF BFC775F8
8380-8396 BFC6899A       9300-937E BFC77D3C
8397-839E BFC6A632       937F-93FF BFC78C00
839F-83B6 BFC68C4C       9400-947E BFC79344
83B7-83BE BFC6A9F2       947F-94FF BFC7A208
83BF-83D6 BFC68F1C       9500-957E BFC7A94C
83D7-843F BFC6ADB2       957F-95FF BFC7B810
8440-8460 BFC691EC       9600-967E BFC7BF54
8461-846F BFC6BDDE       967F-96FF BFC7CE18
8470-847E BFC695CA       9700-977E BFC7D55C
847F-847F BFC6C162       977F-97FF BFC7E420
8480-8491 BFC6978C       9800-9872 BFC7EB64
```

### Krom2RawAdd2(code)

Takes the low 16 bits of the code and gives an address in the font, or a blank, and the
result is the same under OpenBIOS and SCPH-1001 (`krom`). The kernel gives the base:
below, K(x) is B0h 51h with x, which is BFC66000h for 8140h and BFC69D68h for 889Fh
under both.

| code | first call | result if the cell has a glyph |
|---|---|---|
| 8140h to 84BEh | K(8140h) | K(8140h) + 30 * index among the assigned cells |
| 8540h to 8796h | K(8140h) | K(8140h) - 2000h + 30 * index among the assigned cells |
| 889Fh to 9872h | K(889Fh) | K(889Fh) + 30 * ((row - 16) * 94 + cell - 1) |
| any other | | |

A code with no glyph, or any other code, returns K(8140h): the blank, full-width space.
That is a second call in the first three lines and the only one in the fourth.

- The first line's cells are `nonKanji` in `exp/cleanlib/kanji.go`, and the third's row
  and cell are `jisCell` with its trail-byte test, so a trail byte outside 40h to FCh or
  equal to 7Fh has no glyph. The mapping equals those two, with `romNonKanji` and
  `romKanji` as the bases, over all 65,536 codes, apart from the second line: reuse them.
  It differs from Krom2Tim's `glyphFor` in that single bytes are not widened (41h gives
  the blank), and a kanji code with a bad trail byte is a blank, not the glyph before
  the first kanji.
- The second line is a table `kanji.go` does not hold, 245 cells in this order, the first
  at index 0: 8540-8544 8546 8548-8549 854B 8550-855B 85A0-85AC 85B0-85BC 8640-8641 8643
  8645 8647-8648 864A-864C 864E-864F 8660-866F 869F-86B6 86B8-86F5 8740-875A 875C-8762
  8764-8796. Under both kernels this runs from BFC64000h.
- The cost is one or two kernel calls, about 330 cycles each on OpenBIOS, and a walk of
  the short table.

## MulMatrix and SetLightMatrix

### MulMatrix(m0, m1)

Makes m0's rotation m0 * m1 on the GTE and returns m0 (`gtelib`). It is
MulMatrix0(m0, m1, m0), and the existing MulMatrix0 in `exp/cleanlib/matrix.go` gives
the same result. Over 53 cases of ordinary and overflow inputs the rotations are equal,
and so is every GTE register:

| register | after the call |
|---|---|
| R11R12 to R33 (control 0 to 4) | m0's rotation as it was before |
| VXY0, VZ0 (data 0, 1) | m1's third column |
| IR1 to IR3 (data 9 to 11) and MAC1 to MAC3 (data 25 to 27) | the last column's product |
| IRGB, ORGB (data 28, 29) | as the GTE derives them from IR |
| FLAG (control 31) | as the GTE sets it for that product: 0 when in range |

No others change. Elements saturate as the GTE's IR do, to -32768 and 32767: the case
`large` keeps 32767 where 32767 * 32767 >> 12 would not fit. m1 and both translations
are left alone. MulMatrix(m, m) works, reading m before it writes. gp is preserved. 144
cycles (146 on the counter, MulMatrix0's 148).

### SetLightMatrix(m)

Loads the GTE's light matrix from m's rotation and nothing else (`gtelib`): control 8 to
12 take m[0][0] | m[0][1] << 16, m[0][2] | m[1][0] << 16, m[1][1] | m[1][2] << 16,
m[2][0] | m[2][1] << 16 and m[2][2], read back sign-extended on the last. It is
SetRotMatrix's form on the other registers: 28 cycles, as SetRotMatrix. m and the
translation are not touched, and it is not GsSetLightMatrix, which multiplies first (spec
04). gp is preserved.

## The CD calls

No scanned program calls these (spec 15). The target has no names for them, so the
probes `cdcalls` and `cdinit` call them at their entries. Cycles are from entry to
return (PROBE_WATCH); clocks are the second counter around a call, about 28 over.

### CdControlF

`CdControlF(com, param)` sends a command without waiting for its response. Compared with
CdControl and CdControlB (spec 09), on `cdcalls`:

- It always returns at once for a command with no position: 540 clocks of counter 2 for
  Nop, where CdControl takes 53,790 and CdControlB 54,166. It returns 1, the result of no
  answer: Nop, a command the drive refuses (1Fh) and ReadN all give 1. There is no result
  parameter.
- com is a byte: 101h acts as 01h.
- Setloc with a parameter sends it and returns at once (694). A refused Setloc still
  returns 1.
- SeekL with a position sends a Setloc and waits for its response, as CdControl does (53,846
  clocks), then sends the command and returns 1 without waiting: SeekL 00:03:00 left GetlocP
  at 00:03:00 by the next waiting call. If the Setloc is refused it returns 0 (SeekL AAh:00:00,
  which took about as long as CdControl's four sends, spec 09). The other commands that take a
  position in spec 09 were not run with one (see Open).
- With a nil parameter a command is sent alone: SeekL after a Setloc reached the position,
  and ReadN read (Getstat 22h, as in spec 09).
- It may be called with a command outstanding. Two calls in a row each return 1, and a
  CdControl straight after still gets its answer: 02h, 1. The kernel calls in the log are the
  event deliveries of spec 05 (B0h 07h, class F0000003h) and the vertical blank's.

The events each call delivers were not counted. 508 to 524 cycles without a parameter
for the drive, 662 for Setloc.

### CdStatus()

Returns the status byte of the drive's last response that holds one, from any call. GetlocP's
holds none, and leaves it as it was. 02h at rest, 22h reading, 03h after a refused command,
06h after a seek past the disc's end. It waits for nothing. 8 cycles.

### CdSync(mode, result)

Reports on the last command sent, by whichever call:

| state | returns |
|---|---|
| its answer has not all come | 0, in any mode but 0. Mode 0 waits (below) |
| complete | 2 |
| ended in an error, the first time this is asked | 5 |
| the same, asked again | 2 |

- Complete means the last response of a command that answers twice: straight after
  CdControl(SeekL) it returns 0, and mode 0 waits for the second response. After CdControlB it
  returns 2 at once. Straight after CdControlF(Nop) it returns 0, and mode 0 takes 53,238 clocks.
- Only mode 0 waits. 1, 2 and -1 act alike.
- result, if not null, gets the 8 bytes of the last response, padded with zeros, GetlocP's
  included. A refused command gives 03h 20h, a seek past the end 06h 10h. While it returns 0 result is left
  alone.
- After a ReadN past the disc's end CdSync(1) returned 5 and then 2, whether or not CdReady had
  seen the error first.
- No kernel call. 274 cycles pending, 382 complete and 386 in error with a result, 284 with
  none. Spec 08's timeout checks are in that time.

### CdReady(mode, result)

Reports on the sectors of a read (ReadN, ReadS):

| state | returns |
|---|---|
| a sector's response has come since the last call | 1, once for that response |
| none | 0, in any mode but 0 |
| the read ended in an error | 5, once, and then 0 |

- result, if not null, gets the 8 bytes of that response (22h, zeros) when it returns 1 or 5
  (06h 10h past the end), and is left alone when it returns 0. So with a program's calls
  spaced out it reads 1 each time, the drive having delivered another, and two together give
  1 then 0.
- A sector's response stays until asked for: one call after a Pause still returned 1, with
  the old response, and the next 0. Nop and SeekL do not raise it.
- Mode 0 waits for a sector: 424,038 cycles from a ReadN just sent. If none comes it gives up
  after 541,137,276 cycles (16 seconds, about 959 frames at NTSC), prints
  `CD_ready:(CdlSeekL) Sync=Complete, Ready=NoIntr` and a newline, naming the last command and
  the states, and returns -1 with result left alone (956 frames at rest, the line then naming
  CdlGetlocP). 1, 2 and -1 act alike and do not wait.
- CdInit clears it. The data itself is not touched: nothing here reads the sector.
- 284 cycles with nothing, 394 with a result copied, 296 with none.

### CdInit()

Runs the drive's reset again, each time, whatever the drive is doing, and returns 1. The
sequence (`cdinit`, I/O trace, the same on each call and in a read, which it ends):

1. The line `addr=8004d724` on the TTY, as spec 05 has it for the start, the value fixed.
2. 1800h = 1, a read of 1803h, 1800h = 0, 1803h = 0, and 1F801020h = 00001325h.
3. Getstat, Init and Demute, each waited for as CdControl waits (Init's second response too).
4. The SPU words of spec 05: 1D80h, 1D82h, 1DB0h, 1DB2h = 3FFFh, 1DAAh = C001h, then the
   drive's volume: 1800h = 2, 1802h = 80h, 1803h = 0, 1800h = 3, 1801h = 80h, 1802h = 0, 1803h
   = 20h.

The clean library's drive start in `exp/cleanlib/cd.go` covers steps 1, 3 and 4 but runs once.
CdInit runs them on each call, with step 2 in place of that start's register setup. After it
CdStatus gives 02h, CdSync(1) 2 and CdReady(1) 0, with no read going. 288,820 cycles under
OpenBIOS, 68,000 of them the line.

## Open

- malloc before any InitHeap could not be run: every start-up calls InitHeap first. Treat
  the values as those left by InitHeap(0, 0) only if something shows a need.
- Whether the base of Krom2RawAdd2's second line follows K(8140h) or is a constant does not
  show on these two kernels, which agree: written here as K(8140h) - 2000h.
- Whether the heap calls turn interrupts off, and what they do if an interrupt calls them
  (a VSync callback in C), was not looked at.
- Reading the second counter across a kernel write gave about a ninth of what the kernel
  log gives: putc 474 on the counter, some 4,230 between writes in the log (and 34 less
  than putchar on the same counter). The log's numbers are the ones to trust there; heap
  costs agree between the two.
- Real input under a kernel that blocks in read: gets is measured through a stand-in read.
- Retail kernels other than SCPH-1001 were not run.
- CdSync(0) with a command that never answers was not run. CdReady(0) gives up after 16 seconds,
  and it may do the same with a line of its own.
- CdControlF with a position on SeekP, ReadN, Play and ReadS, and the kernel events it delivers
  per call, were not measured. Spec 09's rules for CdControl are the assumption there.

## Acceptance

Run each with `exp/cleanlib/image/probe.sh NAME exp/cleanlib/spec/probes/NAME` once the
image has these entries, and compare `.build/NAME-cimg/out.tty` with
`.build/NAME-cimg-clean/out.tty`:

1. `heap`, with the `cost` lines left out, and `heaplow`: identical. These pin the heap
   rules, including the mixed sequence of 120 calls.
2. `heapbad`: identical. Its cases are overwritten headers and foreign pointers.
3. `stdio`, the `cost` lines left out, and `stdin`: identical.
4. `krom`: identical, under OpenBIOS. Run under PSX_BIOS too if the image is there.
5. `gtelib`, the `cost` lines left out: identical.
6. `cdcalls`, the `clocks` values left out, and `cdinit`: identical.
7. `heapodd` and `heapoddfree` fault on Sony's library: not matched, not required.
8. `heapcost` for the cost table, by eye. image/regress.sh stays where it was.

`exp/cleanlib/kernel.go`'s InitHeap, which stores the head and size only, is replaced by
these rules. A C program's start-up reaches it at the entry, and the Go runtime's main at
its name, so nothing else needs to change.

## What the first runs showed

On the clean image, from outside, with `probe.sh` (2026-11-04).

- `heap`, `heaplow`, `heapbad`, `krom`, `gtelib`, `cdcalls` and `cdinit` are identical to
  Sony's with cost lines and clocks left out, after the corrections below. `stdio` is
  too.
- **realloc(p, 0) returns the s1 of its caller.** Sony's never sets its result on that
  path. So the spec's "return 0" is what `heap` saw, s1 being 0 there, and `heaplow`'s
  A+0 is the arena's address in s1 (`heapv0`, calling it with known registers: the result
  is 4111A5A4h, the value s1 went in with, on the first, middle and last block). The clean
  entry reads s1, so both probes match. free leaves 1 in v0 when it lowers the search
  place and 0 otherwise, which is not matched: its type is void. free(0) writes at
  FFFFFFFCh on Sony's, as the rule says, and not on the clean image.
- **putchar turns a tab into spaces**, up to the next multiple of 8 from a column it
  keeps, always at least one, one kernel write a space. Only the low byte of c counts,
  for this and for the newline: 109h is a tab and 10Ah a newline. The column goes up one
  for 20h to 7Eh and for A4h to A6h and C7h (the codes toupper's table knows), and is set
  to 0 by a newline. Other codes leave it, carriage return and backspace among them, and
  write and putc go around it. It does not wrap at 65536. printf, printf2 and gets go
  through putchar and so get this: `gets` of a quoted tab echoes spaces (`ctab`, `stdin`).
- **CdReady**: any error the drive gives raises it, not only a read's. A refused
  command gives 5 with 03h 20h, and a seek past the end 5 with 06h 10h, over a sector
  waiting, and the error's response stays for it when later commands answer. Sending a
  command that moves the head, Play, Forward, Backward, ReadN, SeekL, SeekP or ReadS,
  takes back what is waiting at once, sector or error. No other command does. This is why
  `cdcalls`'s CdReady(1) at rest gives 0, its last SeekL having cleared the sector
  (`cdready`).
- CdReady(0) with nothing coming: the clean library counts 954 vertical blanks to get
  `cdcalls`'s 956. The states in its line other than `Sync=Complete, Ready=NoIntr` were
  not seen. Sync says NoIntr with answers outstanding, a guess, and the names are the
  manual's. A data end (kind 4) is not raised: no probe disc has audio, so whether
  CdReady(1) returns 4 is not known.
- A command that takes parameters (Setloc, Setfilter, Setmode, GetTD) called with a null
  parameter is not sent. CdControl and CdControlF return 0 at once and result is left
  alone (`cdready`). This corrects "with a nil parameter a command is sent alone", which
  holds for the others.
- `stdin` differs where it must: with nothing read, getchar's and getc(0)'s stale value
  is the stack's on Sony's (91) and the last byte read on the clean image (66). A local
  whose address goes to the kernel would go on the heap, so the byte is the library's
  own. One other line differs: Sony's TTY shows one `x` twice in `54x1` at line 473, once
  in the run, which the library's rules do not explain.
- Costs left out of the identical lines: `heap`'s pads bring its costs within 30 cycles
  of Sony's, above it for malloc from the top (208 against 186). gtelib stays over, MulMatrix
  226 against 146, MulMatrix0 204 against 148 and SetLightMatrix 70 against 30, from the
  words' loads and gp's going through the same asm. CdControlF, CdSync and CdReady are
  paced to `cdcalls`'s clocks.
