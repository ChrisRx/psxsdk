# Milestone 7: memory and the time between the writes

Status: implemented. What has no counterpart in the clean library is
kept as a delay, named in the code.

Two things in the clean library exist only to match the reference's
timing. This spec replaces both with what Sony's library (or the
reference, where Sony does the work before the program starts) is seen
doing.

## The library's .bss

The start-up clears `.bss` at 10 cycles a word (spec 07), so a library
that keeps more zeroed state reaches main later. For square, the
reference's library holds 45,218 bytes and the clean library 56,381.
The two largest tables are the font (35,188 bytes) and the banks
(10,560).

- **Font.** Sony's pool is 1024 characters (spec 01, "FntOpen gives its
  streams characters from one pool of 1024"). The clean pool of 2048
  characters is twice that, and halving it saves 17,408 bytes.
- **Everything else.** Keep each table to what a rule in the specs
  needs. Where a table is sized by a limit the specs do not give,
  report it rather than guess.

The library's `.bss` will still differ from the reference's: each
example links a different part of each library. TestClean's timing
comparisons have to live with what remains, and that is a question for
the test, not the library.

## What fills the waits

The clean library has fixed delays (`spin`, `spinCycles`) where the
reference took time. Each is explained below by what Sony's library, or
the reference, does in that time. These come from traces of every I/O
register load and store, kernel call and RAM store, with cycles. The
tool is `.build/iotrace` (TestIO: DISC, FRAMES and OUT). Replace each
delay with the work it stands for, done the same way: the same loops,
clears and register writes. If a gap is still left, report it, and keep
a delay only for that remainder, with a comment saying what it covers.

### Printing (ResetGraph's line, "addr=", "cbdataready")

Sony's library prints these through a printf: format
"ResetGraph:jtb=%08x,env=%08x\n", "addr=%08x\n", and
"cbdataready: CdlDataEnd (track=%d,time=%d)\n". Each character goes to
the kernel's write(1, &c, 1), which takes 4,182 cycles here. Between the
calls, Sony's library spends:

- 84 cycles for a plain character (80 in places);
- 522 cycles before the first digit of a %08x, then 80 for each digit,
  and about 120 to 130 cycles after a conversion;
- for "\n", "\r" then "\n", 28 cycles apart.

The clean library's 8 spin loops a character stand in for this.
Formatting the line as it prints, one character a call, does the same
work.

### ResetGraph, after GP1 00h (Sony's library)

ResetGraph(0) clears its own state, one byte store at a time: 256
bytes, then 6,144, at about 8 cycles a byte (51,200 cycles in all). It
then reads I_MASK (1F801074h) and writes it back with bits 0, 2 and 3
set (0Dh), and then writes GP1 10000007h. The clean library's
`spin(resetWait)` stands for this clear.

### ResetGraph, around the reset (Sony's library)

After A0h 49h returns, ResetGraph(0) saves I_MASK and writes it 0. It
writes DMA channel 2's control (1F8010A8h) 0401h, reads DPCR
(1F8010F0h) and writes it back with bit 11 (channel 2's enable) set,
then writes GP1 00h. After the clear above it writes the saved I_MASK
back: 0Dh in Sony's library, which started the drive at boot, and 9 in
the reference, whose drive is not started yet. Both read the value
they restore (iotrace with read values, resetwait-sony and hello-ref).

### The drive's answers: how soon the library acts

From the readsync probe's traces (cycles from the drive's response to
the library's next command):

| step | Sony | reference | clean |
|---|---|---|---|
| Setloc's INT3 to ReadN | 2,362 | 1,818 | 1,230 |
| the last sector's INT1 to Pause | 1,638 | 1,212 | 248 |

These come from the interrupt path: the kernel's dispatch, the
library's handler, and its callback. The clean library acts in about
half the reference's time or less. square and scene finish loading
right at a frame's edge, so this difference decides them even with the
start-up evened out (TestClean equalizes the .bss clear). Trace where
the time goes in Sony's handler with iotrace: the I/O reads and
writes, kernel calls and RAM stores between the response and the
command. Then do the same work in the clean handler, or report what it
is. Match Sony's times, not the reference's, where the two differ.

Traced (iotrace and cdtrace on readsync-sony, rounds 1 to 6; cycles
from the library's read of the interrupt cause, 1803h at index 1, which
is where cdtrace logs a response; cdtrace's own figures for the two
steps are 2,344 and 1,622 from that read):

- **Entry.** The library's handler starts 1,252 cycles after the
  exception vector, 182 cycles before the cause read: it acknowledges
  the drive's bit in I_STAT, reads 1800h (the index, which it puts back
  at the end), writes 1800h=1 and reads the cause.
- **Each answer.** It reads 1803h a second time, then the response
  bytes (status, 1801h, until the FIFO is empty) into an 8-byte buffer,
  zero-filling the rest one byte at a time. It acknowledges with 1803h=7
  and writes 1802h=7 (the drive's interrupts enabled), then stores the
  status byte and copies the 8 bytes to a result area, one byte store
  each (a separate area for INT1 and for command responses).
- **A sector (INT1).** 1800h=0, 1803h=0, then for each transfer (the
  3-word header, then the 512 words of data): 1803h=80h, 1F801018h
  (the drive's delay/size) =00020943h, 1F801020h (common delay) =1323h,
  DPCR read and written back with channel 3 on, D3_MADR, D3_BCR, a
  status read, D3_CHCR=11000000h, D3_CHCR read, 1F801020h=1325h, about
  110 cycles each.
- **The last sector.** About 620 cycles of stores to its own state and
  timeout checks (each reads GPUSTAT and root counter 1), then the
  Pause: a status read, 1800h=1 and a cause read (nothing pending),
  1800h=0, the command. It is sent from the handler, which then waits
  for the drive's first answer there, polling the cause about every 200
  cycles (a timeout check each time). That answer is handled in the
  same interrupt: I_STAT read, the drive's bit acknowledged again,
  DeliverEvent(F0000003h, 40h) 642 cycles after its cause read,
  ReturnFromException 1,476 after it. The program then sees CdReadSync
  report 0 924 cycles later (its CdReadSync(1) takes about 490 cycles a
  call, with three timeout checks).
- **Setloc's INT3.** The answer as above, DeliverEvent(F0000003h, 20h)
  494 cycles after the cause read (432 cycles in the kernel), then 1800h=1
  and a cause read (nothing more), 1800h=0, two reads of I_STAT and
  I_MASK, ReturnFromException at 1,096. In the program, CdControl
  returns and CdRead, from 1,250: state stores, a timeout check,
  1800h=1 and a cause read, 1800h=0, 1803h=0, 1F801020h=1325h, more
  stores and timeout checks, 1800h=0, ReadN at 2,348.
- **Before the cause read.** The drive raises its interrupt 50,402
  cycles after Setloc or ReadN is written in both libraries; Sony's
  library reads the cause 1,434 cycles after that, the clean one 1,324
  (1,222 before the index read and the dispatch loop). The kernel's
  part of that is not visible to iotrace. Polling for Pause's answer,
  Sony's handler reads the cause 142 cycles after the interrupt is
  raised, the clean one 14.

The clean library now does the same register work (the index saved and
restored, the second cause read, 7 to 1803h and 1802h, the dispatch
loop until the cause reads 0, the per-transfer register writes, the
Pause sent and its first answer waited for in the handler, CdRead's
1803h=0 and 1F801020h=1325h). The stores to Sony's state and its
timeout checks were kept as two delays, `readRest` and `pauseRest`;
the next section replaces them with the checks themselves.

### Waiting on the drive: the timeout checks (Sony's library)

Every library call that waits on the drive checks a timeout while it
waits, and that check costs time on each call even when nothing times
out. Traced with iotrace on readsync-sony and invaders-sony, with each
public call's entry and return logged (`CALLS`, by the entry point's
name). Cycles are from the call.

**The time.** The library reads the time as GPUSTAT (1F801814h), then
timer 1's counter (1F801110h), two cycles apart. Two patterns use it,
on the same three words of state:

- **Start** a wait: read the time, then 50 cycles later store three
  words (in 8 and 4 cycles).
- **Check** it: read the time, then 56 to 58 cycles later store one
  word (the second of the three: the time last seen).

**CdReadSync(1)**, called in a loop (readsync) or once a frame
(invaders), takes 476 cycles from the call to the return (488 in
places): a time read at 40, another at 116, a start at 226 (its stores
at 278 to 290), a check at 312 (its store at 370), the return at 476.
The count it returns is read before the third time read: a call that
the drive's interrupt lands in, after its second time read, returns
the count from before the interrupt (readsync, the last sector's
interrupt: the resumed call returns 1, and the next call, 924 cycles
after the handler returns, sees 0).

**CdControl** (Setloc, from the program): state stores at 46 and 80, a
start at 194 and a check at 278, then state stores, the parameter
bytes copied to its state one store each, 1800h=0 at 524, the
parameters, and the command at 610. Then it waits for the first
answer: a start 26 cycles after the command, a check 92 after that,
then one check every 118 cycles until the answer, which the handler
takes. The return comes 136 cycles after the check that follows the
handler's return. With fewer parameters the command comes sooner after
the first check: 148 cycles for ReadN (none), 168 for Setmode (one),
274 for Setloc (three).

**CdRead** (readsync, the mode already set): nine state stores, a time
read at 114 and a store 38 later, two state stores, then 1800h=1 at
258, a cause read, three byte stores, 1800h=0, 1803h=0 and
1F801020h=1325h at 316. When the mode differs (invaders), CdControl
(Setmode) is called 46 cycles later and the rest follows its return.
Then state stores, a start at 660, a check at 744, state stores,
1800h=0, and ReadN at 950, as CdControl sends a command but without
waiting for the answer. A time read 86 cycles after ReadN, a state
store, and the return at 1,102.

**In the handler** (the Pause after a read's last sector), interrupts
are off and each turn of a wait looks for the drive's answers itself:
a check, then the index read, 1800h=1, a cause read and the index put
back (each answer found handled there, as in the handler's dispatch
loop), 202 cycles a turn. CdControl's wait for the command to be free,
before it is sent, is one such turn when nothing is pending.

**After Pause's first answer** (cycles from the cause read that sees
it; the handler is still in CdControl's wait): the second cause read
at 16, the response read and zero-filled to 8 bytes, 1803h=7 and
1802h=7 at 178, state stores at 242, 246 and 310, the 8-byte copy to
the result area (330 to 414), then the next turn of the dispatch loop:
1800h=1 at 468, a cause read (nothing), the index back at 520.
CdControl returns at 618, and the handler delivers the event,
DeliverEvent(F0000003h, 40h), at 662 (432 cycles in the kernel). Back
in its dispatch loop: 1800h=1, a cause read, the index back (1,190);
I_STAT and I_MASK read (1,228), the drive's bit acknowledged (1,302),
the index read (1,330), 1800h=1, a cause read, the index back (1,408),
I_STAT and I_MASK read twice (1,446 and 1,466), and
ReturnFromException at 1,496.

The clean library does this (cd.go): a watch (`cdWatch`, three words:
when the wait began, the time last read, its limit) with `begin` and
`check` reading GPUSTAT then root counter 1; CdReadSync takes the
read's state first, reads the time twice, starts and checks the
watch; `send` starts the watch and checks it until no command is in
flight (in the handler, dispatching what the drive has raised on each
turn), keeps Setloc's position a byte at a time, then sends; `command`
starts the watch after the command and checks it on each turn of the
wait; CdRead stores the time it is called and the time it sent ReadN,
and looks at the cause; the handler zero-fills the response and copies
it to the result a byte at a time, and after the Pause's first answer
delivers its event once the wait is over. Neither CdControl nor CdRead
enters a critical section any more (Sony's make no kernel call there,
and the handler puts the index back). No spec gives the waits a limit
except the handler's wait for Pause (263 lines); the others do not give
up. What Sony's state and calls cost beyond this is kept as named
delays, each with the time it covers, measured on readsync.

Measured on readsync-clean against readsync-sony (iotrace, cycles from
the cause read; cdtrace's figures in brackets):

| step | Sony | clean |
|---|---|---|
| CdReadSync(1), a call | 476 | 472 |
| Setloc's answer to ReadN | 2,348 (2,344) | 2,308 (2,304) |
| the last sector to Pause | 1,642 (1,638) | 1,618 (1,614) |
| Pause's answer to its event | 662 | 634 |
| Pause's answer to the handler's return | 1,496 | 1,456 |

Not matched: Sony's handler takes a sector (INT1 not the last) in
1,288 cycles to its return, the clean one in 522, and the time to the
Pause covers that difference for the last sector only.

### Calls through the drawing queue: the same timeout checks (Sony's library)

The calls that go through the drawing queue (spec 06) check a timeout
as the drive's calls do, and mask interrupts while they touch the
queue. One LoadImage on Sony's library (invaders-sony, FntLoad's
palette and then its image), cycles from its first time read:

- 0: a time read (GPUSTAT, then timer 1's counter), then two word
  stores (a start).
- 84: I_MASK saved (0Dh) and written 0. Two stores to the queue's state.
  It reads D2_CHCR (1F8010A8h) and GPUSTAT, which show whether the GPU
  and its DMA are free.
- 220: a time read and two word stores (a check). It reads GPUSTAT,
  then at 378 sends: GP1 04000000h, GP0 01000000h, A0000000h, the
  place, the size, then the data. A 16 by 1 palette goes as 8 GP0
  words. The 32 by 32 image goes by DMA: GP1 04000002h, D2_MADR,
  D2_BCR, D2_CHCR 01000201h.
- After it, three word stores to the queue's state, and I_MASK is
  restored (0Dh), about 610 cycles from the start for the palette.

The reference does the same around each LoadImage. In terra's loading
frames (112 to 162, about 182 LoadImage calls), the clean library skipped
this, which saves it about 3,500 cycles a frame. That is where terra's
97,000-cycle lead at its first sound upload comes from. square's
start-up is ahead by about 8,400 cycles at its first CD command and
16,000 at its first swap, after its first drawing calls.

Traced the same way on Sony's library (iotrace with CALLS on
invaders-sony, clearimage-sony, clone-sony and offscr-sony, and the
probe `spec/probes/drawq`, which makes each call between two GP1 marker
words, 10000200h plus the case number). The time read is the one the
drive's waits use (GPUSTAT, then timer 1's counter), and it is the
library's VSync(-1): a call to it, then the two reads. Cycles are from
the first time read, T, unless said otherwise.

**The entry, every queued call.** The same in each:

- T: a time read, then two word stores at T+40 and T+44 (a start).
- T+84: I_MASK read (0Dh) and written 0 at T+86.
- T+98 and T+106: two stores to the queue's state.
- T+132: D2_CHCR read; T+158: GPUSTAT read.

**The exit, every queued call.** Three word stores, 4 cycles apart, 14
cycles after the last register write of the request; I_MASK read 12
cycles after them and written back with the value saved (0Dh) 2 cycles
later. The value read there is 0.

**A packet list** (DrawOTag through GsDrawOt, and MoveImage, ClearImage,
PutDrawEnv and FntFlush's drawing, when the GPU is free): GP1 04000002h
at T+182, D2_MADR at T+190, D2_BCR 0 at T+198, D2_CHCR 01000401h at
T+206, then the exit (I_MASK back at T+242). GsDrawOt reads the time 90
cycles after its call and returns at 376. Nothing in the program's
traces queues behind a busy GPU: in the emulator the GPU is free again
by the next call.

- **MoveImage** stores three words of its packet (96 to 106 cycles after
  the call), reads the time at 164 and returns at 446. The packet it
  sends is the library's own, not the program's.
- **ClearImage** builds its packet after the entry's reads. The fill
  (02h): five stores from T+268 to T+296, GP1 04000002h at T+314, the
  exit's I_MASK write at T+388, the return at T+430 (572 cycles from
  the call). The rectangle (60h): nine stores from T+286 to T+330, then
  GP1 10000003h at T+344 with GPUREAD 8 cycles later, 10000004h at
  T+378, 10000005h at T+414 (each read followed by a store), GP1
  04000002h at T+456, the return at T+572 (714 from the call).
- **FntFlush** builds its stream's packets first (about 140 cycles a
  character), reads the time 6,656 cycles after the call in invaders,
  queues the list, and after the exit makes two more stores and
  returns, 312 cycles after the time read.
- **PutDrawEnv** builds its packet (stores from 144 to 460 cycles after
  the call), reads the time at 518, queues the packet, and after the
  exit copies the DRAWENV, 92 bytes, one byte at a time into its own
  state (12 cycles a byte, about 1,100 cycles). It returns at 1,928.
  GsSwapDispBuff, after GP1 05h, copies the DISPENV (20 bytes) the same
  way before GP1 03h, then puts the drawing environment twice
  (PutDrawEnv's work, with the copy, each time).

**LoadImage and StoreImage** have the entry, then the transfer's own
wait: a second time read at T+220 with its two stores at T+260 and
T+264 (the same stores as the entry's start), then a GPUSTAT read at
T+362. Their first time read comes 122 cycles after the call.

- **LoadImage**: GP1 04000000h at T+378, GP0 01000000h, A0000000h, the
  place, the size, then the words that are not whole blocks of 16, 16
  cycles a word, and the blocks by DMA (GP1 04000002h, D2_MADR, D2_BCR,
  D2_CHCR 01000201h). It returns without waiting for the DMA. Then the
  exit. 770 cycles from the call for a 16 by 1 palette, 674 for a 32 by
  32 image.
- **StoreImage**: two halfword stores into the RECT's width and height
  just before the GPUSTAT read (their values are not traced), GP1
  04000000h, GP0 01000000h, C0000000h, the place, the size, a GPUSTAT
  read 8 cycles after the size, then for each word a GPUREAD read and a
  store, 16 cycles a word, from 26 cycles after that read. Then the
  exit. 782 cycles from the call for 8 words. An image with whole
  blocks of 16 words is read by DMA instead (seen 2026-11-03 in the
  kanjipast and kanjistream probes for 256 and 640 words): GP1
  04000003h after the size word, then the channel 2 transfer from the
  GPU, as LoadImage sends its blocks. The order of the two parts and the
  time of a block StoreImage are in the next part.

#### StoreImage and LoadImage by size (2026-11-03)

Measured on `spec/probes/storeimage`: StoreImage of 17 rectangles at
(640, 0) whose word counts (w*h/2) are 1, 8, 15, 16, 17, 24, 31, 32,
33, 47, 48, 64, 255, 256 and 2048, then LoadImage of the same sizes,
each call between two GP1 markers, traced with the I/O harness
(`.build/iotrace.test`, CALLS=StoreImage,LoadImage,DrawSync). Video
memory under the rectangles holds a pattern loaded first (each pixel
1000h plus its index in a 128 by 32 image), the buffer StoreImage
reads into is filled with 5a5a5a5ah first, and the probe checks every
word of the buffer against the pattern on the TTY.

An image of 16k + r words, r from 1 to 15, k at least 1, is read in two
parts, the CPU's first: the first r words of the image by GPUREAD reads,
each stored to the buffer in turn from its start (16 cycles a word), then
GP1 04000003h, D2_MADR set to the buffer's address plus 4r (the pointer
as the program gave it, 80093dbch, not masked to its physical address),
D2_BCR k shl 16 or 10h, D2_CHCR 01000200h. With r 0 the GP1 04000003h
follows the GPUSTAT read after the size word by 28 cycles and there are
no GPUREAD reads. The DMA covers exactly k blocks; the buffer's words
come out in image order for every case, nothing past the image is
written, and the words land the same whether the image is one row or
three. StoreImage returns without waiting for the DMA: 90 cycles after
the D2_CHCR write, with no D2_CHCR read between, and the DrawSync(0)
after it reads D2_CHCR once (idle by then in the emulator) and GPUSTAT
once. A rectangle of an odd number of pixels (33 by 1, 17 words) has the
last word's high half filled with the pixel after the rectangle's last
in video memory, the first of the next row, in both libraries: the GPU's
doing.

The block StoreImage's time does not depend on k: 694 cycles from marker
to marker for 16, 32, 48, 64, 256 and 2048 words. Cycles from the marker
before the call (the call itself 8 later) for the 16-word case: the
first time read at 132, I_MASK read and cleared at 214, D2_CHCR read at
262, GPUSTAT at 288, the second time read at 350 with its two stores at
388 and 392, GPUSTAT at 490, GP1 04000000h at 506, GP0 01000000h at
514, C0000000h at 522, the place at 532, the size at 542, GPUSTAT at
550, GP1 04000003h at 578, D2_MADR at 586, D2_BCR at 596, D2_CHCR at
604, the exit's three stores at 634 to 642, I_MASK read and restored at
654 and 656, the return at 694. With r CPU words the first GPUREAD read
is at 576 (26 after the GPUSTAT read), the rest 16 apart, and the GP1
04000003h comes 20 after the last read: at 596 for r 1, 708 for r 8, 820
for r 15. So, marker to marker, with r the words outside whole blocks:

| StoreImage | Sony | clean |
|---|---|---|
| r words, no block (1, 8, 15) | 662 + 16r: 678, 790, 902 | 662 + 16r: 678, 790, 902 |
| k blocks, r 0 (16 to 2048) | 694 | 694 |
| k blocks and r words (17, 24, 31, 33, 47, 255) | 696 + 16r: 712, 824, 936 | 696 + 16r: 712, 824, 936 |

LoadImage follows the same shape (the call 10 after its marker in this
probe): GP1 04000000h at 510, GP0 01000000h, A0000000h, the place, the
size at 548, the first CPU word at 568 and the rest 16 apart, then GP1
04000002h 16 after the last word (at 566 with r 0, 18 after the size),
D2_MADR (again the pointer as given), D2_BCR, D2_CHCR 01000201h, the
return 92 after it. Its time does not depend on k either:

| LoadImage | Sony | clean |
|---|---|---|
| r words, no block (1, 8, 15) | 652 + 16r: 668, 780, 892 | 652 + 16r: 668, 780, 892 |
| k blocks, r 0 (16 to 2048) | 684 | 686 |
| k blocks and r words (17, 24, 31, 33, 47, 255) | 686 + 16r: 702, 814, 926 | 688 + 16r: 704, 816, 928 |

The reference, marker to marker on the same probe (2026-11-03), paces
these calls as a whole and takes 14 cycles a word: StoreImage 690, 886
for 1 and 15 words alone against Sony's 678, 902, and 692 for blocks
against 694; LoadImage 662 for 1 word against 668 and 676 for blocks
against 684. It is left there, within 16 cycles at every size. The clean
library's target is Sony's column.

The clean library reads and writes the two parts in Sony's order with
the same register values, D2_MADR included (the pointer as given, since
2026-11-03; the channel takes its low 24 bits), and returns without
waiting for the DMA. The clean column above is measured on the probe,
marker to marker, every size (2026-11-03). LoadImage's rows with blocks
are 2 over: one store fewer in their path makes them 2 under, the
compiler's delay slots filling otherwise, and neither count of stores
lands on Sony's. Inside the call the events come sooner than Sony's and
the time left goes in the pad at the end: the first GPUREAD read at 374
against 576, the D2_CHCR write at 406 against 604.

A note on the method: a GP1 10h marker whose low bits are 2 to 5, 7 or 8
asks the GPU for information, and the emulator answers it through
GPUREAD in place of the next word of an image being read out, so the
first probe run lost the first word of such cases to the marker before
the call. The probe's case numbers avoid those indices; kanjipast's
dumpRect has no marker around its StoreImage and was never affected.

**DrawSync(0)**, with nothing queued: a time read 66 cycles after the
call, its two stores 40 cycles later, D2_CHCR read at 144, GPUSTAT at
160, the return at 194. It does not touch I_MASK. Its wait on a busy
GPU is not seen in the emulator.

**GsClearOt** is not queued. It clears the table by DMA channel 6: DPCR
read and written with the channel on, D6_CHCR 0, D6_MADR (the table's
last entry), D6_BCR (the number of entries), D6_CHCR 11000002h, then a
time read 30 cycles later with its two stores, D6_CHCR read 16 cycles
after them, and the return, 272 cycles from the call.

The reference does the entry, the exit and the reads of the queued
calls, but in less time, and without LoadImage's second time read or
PutDrawEnv's and GsSwapDispBuff's byte copies. In terra's loading frames
a palette and an image take 486 cycles from one A0h command to the next
in the reference, about 700 in Sony's library.

The clean library now does the entry and the exit (gpu.go: `queueEnter`,
`queueFree`, `queueLeave`, a watch `drawWatch` of the drive's kind with
a `mark` that stores two words), DrawSync(0)'s start and reads, and
LoadImage's and StoreImage's second time read and GPUSTAT read. Neither
the queue nor the DMA start touches DPCR any more, and LoadImage no
longer waits for its DMA (the next call that uses the GPU sees the
channel busy in its D2_CHCR read and waits). The byte copies and
GsClearOt's DMA are not done. Until 2026-11-02 there were no delays for
the rest of Sony's time, since the reference was about as quick as the
clean library; since then the reference takes Sony's time on every call
in the table below, and so must the clean library ("Pacing to Sony's
times", after the table).

Measured on spec/probes/drawq (cycles from one marker to the next, the
probe's own call included; the reference column is from 2026-11-02, with
the start-up of spec 07 in place):

| call | Sony | reference | clean before | clean now |
|---|---|---|---|---|
| LoadImage, 16 by 1 | 782 | 486 | 284 | 358 |
| LoadImage, 16 by 16 | 686 | 442 | 282 | 308 |
| StoreImage, 16 by 1 | 794 | 618 | 78 | 324 |
| MoveImage | 460 | 310 | 770 | 448 |
| ClearImage (60h) | 730 | 462 | 568 | 606 |
| DrawSync(0), with GsSetWorkBase | 218 | 102 | 138 | 142 |
| GsClearOt | 286 | 124 | 68 | 66 |
| GsSortClear | 178 | 106 | | |
| GsDrawOt | 390 | 280 | 300 | 350 |
| Print, 2 characters | 216 | 150 | | |
| FntFlush, 2 characters | 816 | 648 | 784 | 792 |
| PutDrawEnv | 1,946 | 650 | 624 | 678 |
| GsSwapDispBuff | 4,680 | 1,668 | 1,510 | 1,602 |
| DrawSync(0), after the swap | 204 | 108 | | |

#### Pacing to Sony's times (2026-11-02)

Decided 2026-11-02: the library's calls and its vertical blank handler
take as long as Sony's, so that a program's frames keep their place
against the original's (the start-up of spec 07 and the C library's
memmove were matched the same way). The reference does it with named
pads, a spin of whole loop turns that spends a given number of cycles,
one pad per call at the call's end, after its register work and after
the queue's I_MASK restore, sized to Sony's drawq time less the call's
own, so that the marker-to-marker times of the probe come out as the
Sony column of the table (the reference measures within 8 cycles of each
row). Two costs are per item rather than per call:

- LoadImage's words sent through GP0 rather than DMA: 16 cycles a word
  in Sony's library (the loop that stores them is paced, not the call).
  A 16 by 1 palette is 8 such words.
- GsSetWorkBase: 14 cycles in Sony's library, which is why the
  "DrawSync(0), with GsSetWorkBase" row is 14 more than the DrawSync row.

GsSwapDispBuff's time includes its two PutDrawEnv; once those are paced,
the swap itself needs about 420 more. FntPrint and FntFlush were
measured with two characters; their per-character cost is not separated
(FntFlush builds about 140 cycles a character in Sony's library, from
invaders' trace above).

The vertical blank handler, from its I_STAT read, in a frame with no
sequence playing (Sony's is the same in every such frame, once a bank has
been opened: the seqnoise trace, 2026-11-03, has the six words at 4,202 and
the return at 4,772 until the first SsVabTransfer, 432 sooner; the clean
library keeps the later times throughout, not having been asked for this):

| event | cycles after the I_STAT read |
|---|---|
| first envelope read (voice 0, 1F801C0Ch) | 342 |
| the 24 reads | 32 apart, the last at 1,078 |
| the six words (18Ch, 18Eh, 188h, 18Ah, 198h, 19Ah) | 4,634 |
| ReturnFromException (B0h 17h) | 5,204 |

The reference pads the tick in three places (before the reads, before
the six words, after them) and spaces the reads with stores to a scratch
word, and measures 342, 32, 4,634 and 5,204 on terra. The kernel's part
of the exception (its pad polling, 40 serial port accesses in 5,782
cycles, then about 2,500 cycles to the library's handler) is the same
in every build and is not the library's to pace.

Done when: spec/probes/drawq built against the clean library gives the
Sony column within about 10 cycles on every row, a traced vertical
blank handler of the clean library gives the four anchors above, and
`regress.sh` is back to passing every run that passed before (31 PASS,
0 FAIL, 10 TIME on 2026-11-02).

Done 2026-11-02 (the clean library): every drawq row within 6 cycles of
Sony's, the handler at 342, 32, 4,638 and 5,206, regress 32 PASS, 0
FAIL, 9 TIME (square passes outright with both sides paced). Found on
the way:

- The paced calls must be real calls. The compiler's whole-program
  inlining had put the exported calls' register writes into the
  program's own code, and a call's cost then varied by 10 to 60 cycles
  with its caller; marked no-inline, each costs the same everywhere, as
  Sony's do.
- The sequences' stage moved after the six words on 2026-11-03 (spec 05,
  "Where the sequences run"), which took a walk over 32 slots, 786 cycles,
  from before them to after them: the words' pad took those up and the
  stage walks a mask of the open sequences, since the 568 left after the
  words cannot hold the walk. The handler measures 342, 32, 4,634 and 5,202
  now; a sequence playing no longer delays the words, only what follows.
- Rows the probe lacks, measured with a scratch probe between empty
  marker pairs: ClearImage's fill branch (02h) 586 in Sony's library;
  DrawSync(0) alone 202 to 206; GsSetWorkBase alone 16 (the 14 above is
  the difference of two rows). MoveImage and FntFlush needed no pad
  once they were real calls (466 and 810 on their own).
- FntFlush builds about 154 cycles a character in the clean library
  against Sony's 140, so longer strings than the probe's two characters
  came out behind; the per-character costs are now measured and
  separated ("FntPrint and FntFlush by the character", below) and the
  clean library is to match them.
- The second round's GsSwapDispBuff is 88 more than the first in
  Sony's library (it writes the display mode then) and 108 in the
  clean's.

terra's first sound upload (spuphase, frame 276) now comes 18,424
cycles before the reference's, where it came 97,054 before. A version
padded to Sony's times put it about 102,600 cycles after.

### The library's start (the reference)

The reference starts the library in the first ResetGraph(0) (spec 05).
Between its kernel calls it:

1. After B0h 19h (HookEntryInt), clears a 64-byte table byte by byte,
   writes timer 1's mode (1F801114h) 0107h, reads I_MASK and writes it
   0.
2. After C0h 0Ah (ChangeClearRCnt(3, 0)), writes I_MASK 1, clears a
   second 64-byte table byte by byte, writes DICR (1F8010F4h) 0, and
   writes I_MASK 0, then 9.

The clean library's two `spinCycles(600)` stand for the clears and
writes.

### FntLoad and FntOpen (Sony's library)

FntLoad, after uploading the font image, clears 384 bytes byte by byte
(about 3,064 cycles). FntOpen sets up its stream's packets as spec 01
says, 28 cycles a character (three stores each), after one byte store
to the pool, the terminator over the stream's first text byte (the
fntstale trace, 2026-11-03). The trace first read
as the clear belonging to FntOpen. It comes right after FntLoad's
upload. (The reference's FntOpen ran at 12 cycles a character until
2026-11-02, which put square's and scene's first CD commands 8,000
cycles earlier than the clean library's and their drive interrupts on
the other side of a vertical blank; it is paced to 28 now.)

### FntPrint and FntFlush by the character (Sony's library, 2026-11-02)

Measured with `spec/probes/fntchars` (strings of each length and kind,
FntPrint and FntFlush each between two GP1 markers, a window 320 wide,
so a line holds 40 glyphs; the probe's own marker and call overhead is
in the numbers, as in the drawq table). Sony's library:

- **FntPrint**: 168 cycles for an empty string; 182 plus 50 a character
  otherwise, whatever the character (spaces and newlines cost the same
  as glyphs). A 64-character string: 3,382.
- **FntFlush**: 536 with nothing to draw, then by the character:

  | character | cycles |
  |---|---|
  | glyph | 140 |
  | space | 34 |
  | tab | 46 |
  | newline | 62 |
  | a line wrap, charged at the glyph, space or tab that fills the line | 28 |
  | the newline or wrap that reaches the window's bottom (then the flush ends) | 52 for a newline, 18 for a wrap (after a glyph, a space or a tab alike), in place of 62 and 28 |

  So 16 glyphs flush in 2,776, 64 (one wrap) in 9,524, "A " eight
  times in 1,928.

  Measured 2026-11-02 for the two cases that were open: 39 glyphs and a
  space 6,058 against 5,996 for the glyphs alone (34 and 28); 36 glyphs
  and a tab 5,650 against 5,576 (46 and 28); 29 newlines and two glyphs
  2,614 (both drawn on the last row), 30 newlines and two or eight
  glyphs 2,386 either way: the 30th newline, reaching the bottom, costs
  52 and ends the flush, and nothing after it is drawn or charged (spec
  01). The wrap that reaches the bottom, probed the same day (29
  newlines, then 40 glyphs: 7,952 against 7,794 for 39, so the 40th
  glyph's 140 and 18 for its wrap; seven more glyphs after it change
  nothing), costs 18 and ends the flush the same way. So does the wrap
after a space or a tab that reaches the bottom (probed 2026-11-03: 29
newlines, 39 glyphs and a space 7,846, the space's 34 and 18; 29
newlines, 36 glyphs and a tab 7,438, the tab's 46 and 18; a second
space or tab after either changes nothing).

  The isbg tile, a full stream and "%%" (probed 2026-11-03, cases 31
  to 48 of the probe, whose streams are set up before the markers: the
  window reopened with FntLoad and FntOpen, or filled by a print. They
  run in a loop of their own, which costs 12 more in the print column
  than the first 31 cases do: its empty string is 180 against 168 and
  its glyphs 194 plus 50 a character against 182; its flush column is
  the same as the first loop's. Every cost below is a difference
  within that loop. Reopening the window changes nothing: case 48, the
  window reopened as main opened it, gives 180 and 536 as case 31
  does, the same window never reopened):

  - **The isbg tile** costs 40 in every flush of a stream opened with
    isbg 1, with or without text, and nothing in FntPrint: the empty
    string flushes in 576 against 536, "ABCD" in 1,136 against 1,096,
    8 glyphs in 1,696 against 1,656 (cases 33 to 35 against 31, 32
    and the loop's 536 plus 140 a glyph); the prints are 180, 394 and
    594 either way.
  - **A cut stream** (from the trace of `spec/probes/fntstale`,
    2026-11-03, as the gap from one TTY line to the next print's, so
    relative to an empty print on a stream filled exactly, case 8): the
    empty string on a stream a print has cut costs 16 less, 164 in this
    loop's terms, and "XY" 22 more, 202 of which the probe's copy is 28,
    whatever its length. Nothing else differs in the print or the flush
    after it.
  - **A full stream** (a stream of 8, cases 42 to 47). The print that
    reaches the limit costs as a print of the characters it keeps plus
    20, and nothing in FntPrint for the characters it cuts: 10 glyphs
    on the stream of 8, cut at 8, 642 against 594 for 8 glyphs kept
    whole (the two cut characters' 28 are the probe's copy of them, 14
    each, and the other 20 are the cut); "ABCDE" held then "FGHIJ",
    cut at 3, 392 against 344 for "ABC" (28 for the copy of the two
    cut, 20 for the cut). On the full stream a nonempty print costs 194
    plus 20, 214, plus the probe's copy, whatever its length: "I" 228
    (214 and 14), 8 characters 326 (214 and 112); 10 glyphs on a
    stream of 512 cost 694 for comparison. The empty string on the
    full stream costs 180, as on an empty stream; on this stream,
    filled exactly by a print that returned 8, it writes its terminator
    at text[8] again and returns 8, not -1 (spec 01; after a cut it
    returns -1 at once). The flush after a nonempty print that cut or
    was refused costs 4 more than the flush of the same 8 glyphs
    printed whole: 1,660 against 1,656, in all four such cases.
    Where the 4 go (the fntchars trace, cases 42 and 43, and the
    `spec/probes/fntstale` trace, cases 5 to 9, 2026-11-03): the two
    flushes make the same loads and stores at the same times through
    the eighth glyph's packet, and from the first GPUSTAT read of the
    draw on they are the same again, 4 cycles later; the only
    difference is the gap between the last glyph's stores and that
    read, where the loop ends, and no load or store of I/O or RAM
    differs in it. Spec 01 explains it: the flush handles up to n
    characters and then reads one more byte, testing it for the
    terminator before the character limit. A print that filled the
    stream whole left the terminator there and the loop ends on it; a
    cut or a refused nonempty print left the character that did not
    fit there (spec 01, "How the cut works"), so the loop goes on to
    the limit test and ends there, 4 cycles later. The empty string on
    a stream filled whole rewrites the terminator, which is why its
    flush costs the 1,656.
  - **"%%"** (one "%" in the text, spec 01) costs 24 more than one
    plain character in FntPrint and 26 less than two: "%%" 268 against
    244 for one character and 294 for two, "A%%B" 368 against "ABC"
    344, "%%%%" 342 against "AB" 294 (two pairs, 48). The probe's copy
    is 14 a byte, so FntPrint's own cost for the pair is 46 against 36
    for a plain character. The flush draws the pair as one glyph: 676,
    956 and 816, the loop's 536 plus 140 for each of 1, 3 and 2.

The reference matches these within 2 cycles a character after its loops
were made as lean as the compiler allows and then spaced with stores to
a scratch word: FntPrint keeps its count and limit in registers (it had
reloaded them every character) and stores four times a character;
FntFlush walks the text by index with bounds checks off, tests for a
space first and then for the range of the glyphs with an if-chain (a
switch on the character became a jump table, nine instructions on every
character) and spends the rest of each path in stores. Each path's
stores write a different live value (the glyph's index, x, the
character, the count, y): with one value the compiler merges the paths'
tails and the counts stop being independent; each kind of wrap (after a
glyph, a space, a tab) has its own copy of the newline's block, since
the compiler lays out each path's jump differently and the bottom pads
come out different. The glyph's cell comes from a 256-entry table
(2026-11-03, after the fntedge probe showed Sony placing every byte
with signed division, which cost 12 cycles a glyph as arithmetic); the
table's load changed the layout enough that the second row's glyphs
stopped being 2 cheaper than the first row's, and every pad was retuned
in four rounds. The cases added 2026-11-03 (measured the same way, as
differences within the probe's second loop, where the reference's print
column runs 176 for the empty string and 206 plus 50 a character) found
the isbg tile 4 under, the cut and the refused print 10 under, and the
flush of a full stream 14 under: the reference's flush walks the stream's
capacity and a full stream ends without reading a terminator, a cheaper
exit. Padded the same day: two stores after the tile is
linked, three in the cut path (five made the compiler reshape the
function and every print cost 40 more), a pad after the loop for the
exit by count. The refused print could not be padded: any store or
load in its branch, before the loop, moved every print by 20 to 40.
Left, all within 4: the glyph's wrap that reaches the bottom 4 under,
the tab's 2 under, "AB\n" 2 under, the tile 2 over, the full stream's
flush 1,658 against Sony's 1,656 and 1,660, the cut and the refused
print 4 under, FntPrint's empty string 158 (10 under) and nonempty
strings 6 over. "%%" costs the same 24 over one character and flushes
the same. Every count is empirical: the compiler's delay-slot
filling moves with the count and the layout, so a store costs 2 or 4
and a change in one path can move another; re-measure after any edit
to the loop.

Done when: the clean library's fntchars probe gives Sony's numbers
within a few cycles a character on every case.

Done 2026-11-02 (the clean library): every case of the probe matches
Sony's to the cycle but "AB\n" four times (2 under); regress 32 PASS, 0
FAIL, 9 TIME. Found on the way:

- The wrap's 28 cycles are charged at the glyph that fills the line
  (the 40th in a 320-wide window), not at the one that starts the next,
  and Sony's layout wraps there too: probed 2026-11-02 with
  `spec/probes/fntlayout`, the rules are in spec 01 under FntFlush (a
  newline after a wrap leaves a blank row; a tab advances 32; a space or
  a tab that reaches the edge wraps as a glyph does; the returned
  pointer is the texture-page packet with no node before it). The clean
  library differed on all four and follows spec 01 since 2026-11-02
  (layout identical to Sony's on every probe case, timing unchanged,
  regress 32 PASS, 0 FAIL, 9 TIME). The wrap after a space or a tab
  and the text past the last row were measured the same day (the table
  above): the clean library was 20 under on each wrap and 18 under on
  the newline that ends the flush. Padded the same day: every case of
  the probe matches Sony's to the cycle but "AB\n" four times (still 2
  under). The ending newline's pad had to store a value of its own (y):
  storing the 0 the other newline's pad does, the compiler hoisted both
  pads above the bottom test and every newline paid the end's count.
  The wrap that reaches the bottom was measured 2026-11-03 (the last
  three rows of the table): the clean library was 16 under on it and
  was padded the same day, the pad storing y as the ending newline's
  does; every case of the probe matches Sony's but "AB\n" four times
  (still 2 under), regress 32 PASS, 0 FAIL, 9 TIME. The wraps after a
  space and a tab that reach the bottom (the table's last four rows,
  probed 2026-11-03) were 16 under each and were padded the same day,
  with the same result. Found there: a different value is not enough to
  keep pads apart when they break to the same exit. The compiler sinks
  the pads' common trailing stores into one block at the exit whenever
  only one operand differs between them (the value or the word), so
  four pads of 8 stores to one word became one shared block with a
  register select in front of it and the counts moved together. Two
  operands differing (a different value stored to a second scratch
  word) is judged unprofitable and the pads stay separate blocks.
  The isbg tile, the full stream and "%%" (cases 31 to 48) were matched
  2026-11-03: within the probe's second loop every difference is
  Sony's exactly (the tile 40, the cut 20 over the kept characters, the
  refused print 20 over the empty one, the flush after a cut or refusal
  4 more, "%%" 24 over one character); the loop itself runs 2 over in
  the print column and 2 under in the flush column, the probe's own
  code. Found there: nothing but the result may live across the pacing
  call, or the prologue saves registers on every print (+16); the
  flush's preheader is set by the registers its loop hoists constants
  into, so shaving an instruction only lets the next constant in, and
  the loop's cold paths read their constants from fields instead; and a
  store through a base register the path already holds costs 2 where one
  needing an address costs 4.

  The rules of `spec/probes/fntstale` went in 2026-11-03 (spec 01):
  FntPrint writes and counts each character before it checks the room,
  and the flush walks to the terminator or for the stream's n characters.
  Its loop no longer has a mark from the print to test: the 4 after a cut
  comes from the loop itself, from the byte it reads after the n-th and
  the limit's test. The count added a test and a decrement a character
  (the glyph's path 6 over, the space's 8), and every pad was retuned in
  six rounds, the fntchars table now as before but for three rows within
  the same 2 cycles the other way: the flush after a cut 2 over, the print
  that cuts or is refused on a full stream 2 under, the empty print's first
  repeat 2 under. Found there: in the print's loop one
  pad stored to the same word in the "%%" path and the character's path
  moved the character's cost by 4 to 8 with each store added to the pair,
  until the pair's pad went to the second scratch word; the walk's two
  exits needed a pad each (the limit's storing the character) to set their
  difference, 10 with no pads and -4 with one on the NUL's exit alone; a
  pointer to the end in place of the count cost the same a character and 2
  more before the loop, so the count stayed. The clean library's print on a
  cut stream (the row above) is a call of its own, for the prologue's sake.
- The probe's print column includes yaroze.Print's copy of the string
  into C form, 14 cycles a character in every build, so FntPrint's own
  loop is 36 a character.
- In the probe build the compiler specializes FntFlush for its one
  caller's constant id (-1) and FntPrint for its one buffer, so a
  program that flushes several windows pays about 8 cycles more a call
  than the probe shows; Sony's prebuilt library pays its checks every
  time, and its 536 includes them.
- The first glyph of a flush had cost 50 to 68 more than the rest in the
  clean library: the loop's preheader (field loads, hoisted constants),
  paid only by nonempty flushes, which is why the fixed cost could not
  be matched by a pad alone and the preheader had to be cut (the
  texture page's fixed packet words are now written once at FntLoad).

### Sound start-up (the reference; Sony's library does it at boot)

The waits are a delay loop: a counter in memory stored on each step up
to 240, about 2,390 cycles a loop here. In order (spec 05's start-up
steps):

1. It reads DPCR (1F8010F0h) and writes it 333BBB33h, then writes
   180=0, 182=0, 1aa=0, then runs one loop.
2. It writes 180, 182 = 0, reads SPUSTAT (1DAEh), writes 1ac=4, 184,
   186 = 0, 18c, 18e = ffff, 198, 19a, 190 to 196 and 1b0 to 1b6 = 0,
   and 1a6=0200. It reads SPUSTAT and runs one loop.
3. It makes the eight 1a8 writes of 0707, reads SPUCNT and writes
   1aa=0010, then runs one loop. It reads SPUSTAT, then runs two loops.
   It reads SPUCNT and writes 1aa=0, then reads SPUSTAT.
4. The voices (each 0=0, 2=0, 4=3fff, 6=0200, 8=0, a=0), then 188=ffff.
   It reads 18a, writes 18a=00ff, then runs four loops.
5. 18c=ffff. It reads 18e, writes 18e=00ff, then runs four loops.
6. 1aa=c000. It reads DICR and writes it 00900000h (the SPU's DMA
   interrupt on), opens an event (B0h 08h, OpenEvent(F0000009h, 20h, ...),
   its third and fourth arguments not traced) and enables it (B0h 0Ch), then writes 1da2=fffe.
7. 180, 182 = 3fff, then 184 to 19e = 0. It clears 288 bytes byte by
   byte (about 2,300 cycles), then parks the voices.
8. (Traced from invaders-ref, at about frame 110.) Parking takes about
   145 cycles a voice. Then, 156 cycles after the last park write, it
   reads each voice's envelope (0Ch) 26 cycles apart, as its tick does,
   and 2,104 cycles after the last of them (clearing 24 bytes byte by
   byte among other stores) writes 18c=0, 18e=00ff, 188=0, 18a=0,
   198=0, 19a=0, four cycles apart. It then sets up 16 slots of 14
   stores each (about 550 cycles) and runs one more delay loop on the
   same counter, to 1000 at 8 cycles a step (about 8,000 cycles); the
   I_MASK read comes 8,998 cycles after the 19a write. Then: it reads
   I_MASK and writes it 0, calls B0h 5Bh (ChangeClearPad(1)) and C0h
   0Ah (ChangeClearRCnt(3, 1)), writes I_MASK 0Ch (what it read, less
   bit 0); reads I_MASK and writes it 0, calls ChangeClearPad(0) and
   ChangeClearRCnt(3, 0), and writes I_MASK 0Dh (bit 0 back). The last
   write is 344 cycles after the first read. The call's own work
   follows (SsVabTransfer's 1a6 write 15,204 cycles later).

OpenEvent's other arguments (step 6), traced: mode 2000h (no
interrupt), function 0. The reference acknowledges the SPU's DMA
interrupt itself: DICR is written 10900000h in its handler after a
transfer.

The clean library's nine `spinCycles` in spu.go stand for these. Where
the clean library has no state of the size cleared, clearing its own
state byte by byte is not required. Report what is left.

### The C library's memory routines (Sony's library)

The compiler emits calls to memset, memcpy and memmove on its own
(zeroing a struct or a new allocation, copying a slice, copying one
that may overlap). On Sony's library they are the resident library's;
on the reference and the clean library they are the toolchain's
`internal/toolchain/_tinygo/targets/playstation-libc.c`, byte loops,
which both targets compile in (the clean target inherits the Go
target's files and its `LIBPS_GO` define). Traced on terra against
Sony's library, the three loops store the same number of bytes in both
builds, at these costs a byte:

| routine | Sony | playstation-libc.c |
|---|---|---|
| memset | 8 | 8 |
| memcpy | 12 | 12 |
| memmove, forward | 14 | 12, now 14 |

Since 2026-11-02 the C file's memmove loops carry one nop an iteration
under `LIBPS_GO`, so both the reference and the clean build take Sony's
14 (terra's clean build: 106,624 bytes moved in 1,492,316 cycles against
Sony's 1,492,330). Sony's backward loop was not exercised; it is paced
the same. A change to that file reaches the clean build at once
(build.sh copies the targets) and the reference after the embedded
compiler is rebuilt (`task compiler:overlay && task --force
playground:sysroot && task yaroze`).

### terra's unpacking against Sony's library (2026-11-02)

terra loads its archives, then unpacks and uploads 152 textures and
parses its models over some 52 frames with no VSync call, so this
stretch measures the library's own speed with the start-up out of the
way. Traced with the iotrace harness on terra-sony and a reference
padded to its disc (scratch refpad.sh), aligned on the first texture's
A0h command and ending at the first SsVabTransfer call:

| | Sony | reference | clean |
|---|---|---|---|
| the stretch | 29,629,502 | 29,235,784 | 29,384,530 |
| queued calls, I_MASK off | 303 calls, 171,930 | 112,128 | 92,738 |
| vertical blank handlers | 52 at 5,204 | 52 at 4,422 | 53 at 2,006 |
| everything else | 29,186,964 | 28,897,158 | 29,185,476 |

(The clean column is with the paced memmove, the reference's without
it; with it, the reference's stretch is 29,489,796 and its "everything
else" 29,146,748; with the handler and the queued calls paced too, its
stretch is 29,628,566, within 1,000 cycles of Sony's over 29.6
million.) The "everything else" row is the program's own code, which is the
same in all three builds, plus the C library: the reference's 289,806
lead there was memmove's 2 cycles a byte over 106,624 bytes (213,000)
and the time around the queued calls outside the I_MASK window (a
LoadImage pair is about 220 cycles quicker than Sony's from one A0h to
the next, and a texture's unpack-and-upload about 480). With memmove
paced, the clean's "everything else" is within 1,500 cycles of Sony's
over 29 million.

What remains is per frame and per call, in every program:

- The vertical blank handler, from the first I_STAT read to
  ReturnFromException: Sony 5,204 cycles, the reference 4,422, the
  clean 2,006. The kernel's own part (its pad polling of 40 serial port
  accesses in 5,782 cycles, and 2,500 more to the library's handler) is
  the same in all three. Inside the library's part Sony's reads the 24
  envelopes 32 cycles apart from +342 (the reference 26 apart from
  +966), stores 24 bytes at 38 a store and 24 more at 60 (the reference
  24 and 38), then the six words at +4,634 (the reference +3,900).
- The queued calls, as the table under "Calls through the drawing
  queue" has them: the reference is 150 to 300 cycles quicker than Sony
  on each, 1,300 on PutDrawEnv and 3,000 on GsSwapDispBuff.

Both are now paced to Sony's, by decision of 2026-11-02 ("Pacing to
Sony's times", under the drawing queue section): the handler had cost
every program about 800 cycles a frame against Sony (3,200 for the
clean), the swap and PutDrawEnv together about 4,300.

## To look at again at the end

Runs TestClean reports as timing-only (`timingSensitive` in
`tests/zz_clean_test.go`, TIME in `regress.sh`) were looked at once
more on 2026-11-02, when every public call was in the clean library:
each run's logged differences and dumps (`.build/regress/*.log`,
`.build/NAME-clean/{ref,clean}-{gpu,spu}.txt` and the sound memory
files), and then each run against a build for Sony's own library as
well (`PSX_CLEAN_REF=sony` in TestClean: the Sony disc in
`.build/NAME-sony`, the clean build padded to it in
`.build/NAME-sony-clean` with `PAD_TO_REF=.build/NAME-sony/NAME-sony.bin`).

- timers, flame, morph, offscr, dungeon, check: every differing GPU
  word is a digit glyph of the time readout (font sprite texture words
  603c08xxh at the readout's positions), timers' load bar width (16
  against 15) and a readout one digit longer in one swap. The same
  against Sony's library. Timing only.
- tunnel, tunnel:0800, cdda: the sound streams are identical when frame
  boundaries are ignored; one six-word tick sits a frame earlier in the
  clean run. Timing only. The reason recorded for tunnel before,
  CdReadFile's table lookup, was wrong: the reference's CdReadFile goes
  through its tables too (spec 09 and 14), and the two builds' reads
  are a constant tenth of a frame apart through tunnel's loading. The
  offset is the start-up's (spec 07's remainder and different printing)
  and, where a program loads before its first pad read, the pad
  driver's, which the library's start-up now starts (spec 07, "The
  library's start-up"; spec 02, "When the pad driver starts"). With the
  start-up in place on both sides, tunnel, tunnel:0800 and cdda pass
  and are no longer listed as timing sensitive (2026-11-02).
- dino: the differing sound memory is exactly the reverb work area
  (79020h to 7ffffh); cdda:4000: the CD capture buffers. Timing only.
- terra: was not timing only. The four key ons at the end of the run
  differed from the reference's and from Sony's alike in pitch, start
  address and ADSR, because terra overwrites its effect bank's header
  after SsVabTransfer and Sony's library reads the header at every key
  on: spec 05, "The header is read at every key on", implemented
  2026-11-02. Since then its key ons and sound memory match the
  reference's, and what is left is a tick's six words across a frame
  edge (frames 162/163 and 782/783) and the clean's key ons a frame
  earlier. Timing only.
- square (added 2026-11-02, after the start-up moved and the header
  change): its loading bar at swap 5 is one step behind. The read's
  last interrupt and a vertical blank arrive within a few hundred cycles
  of each other, and which comes first decides whether the loader sees
  the read done in that frame; the clean library's first Setloc sits
  within a thousand cycles of the reference's, which is the noise of two
  implementations. Timing only.

Against Sony's library every run is TIME too. The sound streams of a
Sony build begin without the start-up's words (spec 05's five and 531
register words) and with a tick in every frame from the first: not
because the start-up never happens, as was first written here, but
because Sony's resident library does it in its initialization routine
before the program, which the stub in the executable calls, and the
recording starts at the program's entry (spec 07, "The library's
start-up", traced from that routine's first instruction). The
reference did the same work lazily (the pads at GetPadBuf, the drive
and the sound system at the first sound or CD call, the card driver at
the first card call), so its frames carried less interrupt work than
Sony's until those calls: the kernel's pad polling (5,782 cycles a
frame), the card driver's share and the tick (about 11,000 cycles a
frame together), which moved terra's loading frames against Sony's.
Since 2026-11-02 the reference does all of it in the program's first
ResetGraph (spec 07), and the clean library is to follow. Measured after
the change (`PSX_CLEAN_REF=sony PSX_CLEAN_OTHER=sony-ref`, a reference
build padded to the Sony disc): the kernel's per-frame work is the same
as Sony's; tunnel's upload lands one frame before Sony's (a frame edge,
the start-up now inside the recording); terra's reference still gained
about 7,500 cycles a frame over its 52 frames of unpacking between CD
reads, so its upload was two frames before Sony's. That remainder was
traced on 2026-11-02 ("terra's unpacking against Sony's library",
below): more than half of it was the C library's memmove, now paced to
Sony's; the rest is the vertical blank handler and the queued calls,
which are faster than Sony's (the table above). Sony's first VSync also prints
the kernel's "Found C0 patch" line (some 80 characters, about 380,000
cycles), which the reference, which makes no GetC0Table call under this
emulator's BIOS, does not print; a constant offset. 16 bytes of 07h at 1000h
of Sony's sound memory remain untraced. With the start-up words set
aside, tunnel's, cdda's and dino's streams match Sony's apart from the
frame edges, and terra's apart from the key ons above.

Decision of 2026-11-03: the nine runs still reported TIME (cdda:4000,
check, dino, dungeon, flame, morph, offscr, terra, timers) are left as
they are. Each passes on output and differs only in a readout of a whole
frame's cycle count, or in the sample a frame edge lands on. Closing
them would mean pacing whole frames to the cycle, which the font work
above shows the cost of (a day for one function, with pads that reshape
the compiler's output). To take them up again: start with timers, whose
readout is a plain count of lines a frame took, and measure the clean
library's distance from the reference with the regress logs
(`.build/regress/timers.log`, the GPU dumps beside the clean build);
then check, whose total time readout gives the same distance over a
run.
