# Milestone 16: the C string, memory and number calls

Stage 2 of the resident image (spec 15): original C programs call these entries
at fixed addresses, and the clean image lacks them. The entries here are strlen,
strcpy, strncpy, strcat, strncat, strcmp, strncmp, strchr, strrchr, strpbrk, strspn,
strcspn, strstr, strtok, memchr, bcopy, bzero, bcmp, toupper, tolower, atoi, atol,
strtol, strtoul, qsort, bsearch, setjmp and longjmp. memcpy, memset, memcmp and
memmove are in the image today from `playstation-libc.c`, and differ from Sony's
as listed below.

Everything here was observed by calling Sony's entries from the probes named, on
this repository's emulator and OpenBIOS. A program built by Sony's compiler passes
arguments in a0 to a3 and on the stack, as in spec 15. Each probe's output is
`.build/NAME-cimg/out.tty` after `SIDES=sony exp/cleanlib/image/probe.sh NAME
exp/cleanlib/spec/probes/NAME`.

Sony's versions have several behaviors that differ from standard C, and some
that are plain bugs. They are the behavior: match them. An entry that crashes on
Sony's library need not crash on the clean one.

## General rules

- No entry here calls the kernel: `out.kernel` of `cstr`, `cmem`, `cnum`, `csort`,
  `cbsearch`, `cjmp` and `cneg` holds only the probe's own Write (B0h 35h) and interrupt
  returns (B0h 17h) once the probes start. Implement them all in the library.
- No entry changes s0 to s7, fp or gp (`ccost`'s `kept` lines, one call of each
  entry but bsearch and longjmp, and `cjmp`).
- Bytes are unsigned everywhere: compares, sets and differences.
- A char argument (strchr, strrchr, memchr, memset, toupper, tolower) counts only
  by its low 8 bits. 0x161 is 0x61, -1 is 0xFF, 0x100 is 0.
- A length n is signed, with the exceptions of bsearch and qsort. Zero and
  negative lengths do no work, with the exception of memcmp and bcmp. The value
  returned then is in each entry's rule.
- Address 0 is ordinary memory on this machine, so a null pointer does not fault.
  Some entries check for null and others read through it. The table says which.
  The reads through null show in the probes, which put a known string at address
  0. Copies at any alignment are by bytes, as far as any overlap can show.

### Null pointers

| entry | a null first | a null second | both |
|---|---|---|---|
| strlen | 0 | | |
| strcpy, strcat | returns 0, writes nothing | same | same |
| strncpy, strncat | returns 0, writes nothing, at n of 0, 2 or 3 | same | same |
| strcmp, strncmp | -1, at n of 0 or 3 | 1, at n of 0 or 3 | 0 |
| strchr, strrchr | 0, for any c | | |
| strpbrk, strspn, strcspn, strstr | no check, reads memory at 0 | no check | no check |
| strtok | see strtok | no check, reads the set at 0 | |
| memchr | 0 | | |
| memcpy | returns 0, writes nothing | no check, copies from memory at 0 | returns 0 |
| memmove | no check, writes to memory at 0 | no check | no check |
| bcopy(s, d, n) | no copy, when s is null | no check when d is null: writes to memory at 0 | no copy |
| memset, bzero | returns 0 (memset), writes nothing | | |
| memcmp, bcmp | no check | no check | no check |
| atoi, atol, strtol, strtoul | 0, end pointer not written | | |
| qsort, bsearch | no check | | |

(`cstr`, `cmem`, `cnum`, `cneg`)

## Strings

Probe `cstr`.

**strlen(s)** The count of bytes before the first 0. Null gives 0.

**strcpy(d, s)** Copies s and its terminator, a byte at a time from the start,
writing nothing past the terminator, and returns d. Any alignment. d before s
overlapping works as a forward byte copy does. `strcpy(d, d)` returns d, with the
string unchanged. See Open for d after s.

**strncpy(d, s, n)** Returns d. With n of 0 or negative it writes nothing. Copies
bytes of s up to n; where s ends before n, the rest up to n are 0. A string of n or
longer gets no terminator. Copies forward by bytes: `strncpy(s+1, s, 5)` on
"abcdefgh" leaves "aaaaaagh", as each byte written is read next.

**strcat(d, s)** Returns d. Appends s and its terminator where d's terminator is,
forward by bytes. s inside d past d's terminator works: "abc\0XY" with s at the X
gives "abcXY\0\0".

**strncat(d, s, n)** Returns d. Appends at most n bytes of s and then always a
terminator. With n of 0 or negative it changes nothing visible. It does not pad.

**strcmp(a, b)** Walks both until they differ or end. Returns the unsigned
difference at the first place they differ, a's byte less b's, counting the
terminator as 0: "a","b" gives -1, "a","z" -25, "abc","ab" 99, "\x80","a" 31,
"\xff","\x01" 254, "\xff","" 255, "","\xff" -255. Equal strings give 0.

**strncmp(a, b, n)** Null is checked first, as in the table, even for n of 0. With
n of 0 or negative it then returns 0. Otherwise as strcmp over at most n bytes, and
bytes past a terminator do not count: "a\0b","a\0c",3 gives 0.

**strchr(s, c)** The first place the low byte of c is found, or 0. c of 0 finds the
terminator: `strchr("", 0)` returns s. Null gives 0.

**strrchr(s, c)** The same, for the last place. c of 0 finds the terminator.

**strpbrk(s, set)** The first byte of s found in set, or 0. Neither pointer is
checked. The terminators do not match.

**strspn(s, set)** The length of the start of s made only of bytes in set.
**strcspn(s, set)** The length of the start of s made only of bytes not in set.
An empty set gives 0 and the length of s. Neither pointer is checked.

**strstr(s, sub)** A rule, since Sony's is not a correct search. Start with r the
start of s and i at 0. For each byte c of s in order, not counting the terminator:

- If c equals sub[i] (sub's terminator never equals c, so an empty sub never
  matches): add 1 to i. If sub[i] is now its terminator, return r. If i is now 1,
  set r to the address of c.
- Otherwise set i to 0. The byte c is not tried again as sub's first byte.

At the end of s, return 0. So:

| s | sub | returns |
|---|---|---|
| "hello" | "ll" | s+2 |
| "hello" | "" | 0 |
| "" | "" | 0 |
| "hello" | "o" | s (a one-byte sub returns s, wherever it is found) |
| "hello" | "x" | 0 |
| "hello" | "lo" | 0 (the second l is not retried) |
| "aab" | "ab" | 0 |
| "abcabd" | "abd" | s+3 |
| "aaa" | "aa" | s |

The rule meets all 348 strstr lines of `cstr`, 300 of them random strings over two
letters. Neither pointer is checked.

**strtok(s, set)** It keeps one pointer between calls, 0 at the start of the
program. If s is not null, the pointer becomes s. Then:

1. Skip the bytes in set. If that reaches the terminator, leave the pointer at the
   terminator and return 0.
2. The token starts here. Walk to the next byte in set or the terminator. At a byte
   in set, write 0 over it and leave the pointer just after it. At the terminator,
   leave the pointer at the terminator. Return the token's start.

So later calls on an exhausted string keep returning 0, and "a,b;;c," with ",;"
gives a, b, c and then 0 with the string as "a\0b\0;c\0". The set may change between
calls. An empty set returns the rest of the string as one token. set is not
checked and may be read from address 0. Called with s null and no earlier string
in this program, it returns 0 and writes nothing (`strtok first ever`).

**toupper(c), tolower(c)** The result is by the low 8 bits of c, and is always in
0 to 255. So toupper(-1) is 255 and toupper(0x161) is 0x41. Besides the letters,
Sony's table changes these codes, and all others stay as they are:

- toupper: a to z give A to Z, A5h gives 85h, A6h gives 86h, C7h gives A7h.
- tolower: A to Z give a to z, A4h gives C4h, A5h gives C5h, A6h gives C6h, C7h
  gives E7h.

`cstr` prints both tables whole.

## Memory

Probes `cmem` and `cneg`. Every copy is a forward or backward loop of byte moves:
overlaps show no copying by words at any shift from 1 to 12.

**memcpy(d, s, n)** Returns d. Copies forward by bytes, so with d after s and
overlapping it smears the start of s. With n of 0 or negative it copies nothing and
returns d. With d null it copies nothing and returns 0. s is not checked.

**memmove(d, s, n)** With d before s, as unsigned addresses, it copies forward and
returns d+n. With d the same as s or after s, it copies backward and returns d. So
it is correct on any overlap, and its value is d+n for most calls. With n of 0 or
negative it copies nothing and returns d. No null checks: d null with s anywhere
else is the forward case, so `memmove(0, s, 3)` writes at address 0 and returns 3.

**bcopy(s, d, n)** The source comes first. It returns nothing. Copies forward by
bytes, like memcpy, even with d after s. With n of 0 or negative, or s null, it does
nothing. d is not checked.

**memset(d, c, n)** Writes the low byte of c n times and returns d. With n of 0 or
negative, or d null, it writes nothing and returns 0, not d.

**bzero(d, n)** Writes n zeros. With n of 0 or negative, or d null, nothing.

**memcmp(a, b, n), bcmp(a, b, n)** The same function. Compares a byte, and returns
the unsigned difference a less b if they differ. Otherwise subtracts 1 from n and
goes on if the result, as a signed value, is above 0. So:

- n of 0 or 1 compares one byte. n of 0 is not 0 when the first bytes differ:
  `memcmp("a", "c", 0)` is -2.
- n of -1 compares one byte. n of -2147483648 goes on for 2147483648 bytes at most,
  since it wraps: a difference at byte 5 returns it.
- A 0 byte does not stop it. bcmp returns the same difference, not just 0 or 1.
- No null checks.

**memchr(s, c, n)** The first place in n bytes holding the low byte of c, or 0. A 0
byte does not stop it. With n of 0 or negative, or s null, it returns 0.

### memcpy, memset, memcmp and memmove against playstation-libc.c

`internal/toolchain/_tinygo/targets/playstation-libc.c` differs from Sony's as
follows:

| case | Sony | playstation-libc.c |
|---|---|---|
| memcpy, negative n | no copy, returns d | runs through memory |
| memcpy, d null | no copy, returns 0 | writes at 0, returns d |
| memset, n of 0 | returns 0 | returns d |
| memset, negative n | no write, returns 0 | runs through memory |
| memset, d null | no write, returns 0 | writes at 0 |
| memmove, d before s | returns d+n | returns d |
| memmove, negative n | no copy, returns d | runs through memory |
| memmove, d equal to s | a backward copy over itself | returns at once |
| memcmp, n of 0 | compares one byte | returns 0 |
| memcmp, negative n | compares one byte (see above) | runs on until a difference |

Results, overlap direction and alignment are otherwise the same. memcmp's values
are the same differences.

The compiler calls these names for its own copies and may rely on the C standard's
result (see Open).

## Numbers

Probe `cnum`. strtol, strtoul, atoi and atol take the same steps. The rule below
meets all 1,744 values of `cnum`. Sony's parsing has its own idea of a prefix and
of a plus sign, and does not report overflow.

1. A null string returns 0, without writing the end pointer.
2. Skip bytes 09h to 0Dh and 20h.
3. strtol, atoi and atol: while the byte is `-`, skip it and flip the sign. So
   "--1" is 1 and "---1" is -1. strtoul does not do this, so `-` stops it as a
   non-digit. No call takes `+`, which stops the parse as a non-digit: "+1" gives 0
   with the end pointer at the `+`.
4. The base: if the string goes on with `0x` or `0X`, skip those two and use 16.
   Otherwise with `0b` or `0B`, skip those two and use 2. Either comes before the
   base argument and overrides it: `strtol("0x1f", &e, 2)` is 31, and
   `strtol("0b1", &e, 16)` is 1. Otherwise strtol and strtoul use the base argument
   if it is 2 to 36, and 10 for any other, 0 included. So strtol with base 0 reads
   "077" as 77. atoi and atol use 8 if the string goes on with `0`, else 10, so
   atoi("077") is 63 and atoi("08") is 0.
5. Read digits, 0 to 9 and a to z in either case, until a byte that is not a digit
   or is a digit at or above the base. The value is v = v·base + d, modulo 2^32.
   It wraps with no limit: "4294967296" is 0, "99999999999" is 4876E7FFh.
6. Apply the sign by negating modulo 2^32 (strtol, atoi and atol).
7. If the end pointer is not null, write the address of the first byte not read. It
   is not put back to the start when no digit was read: spaces, signs and a prefix
   stay consumed. " " gives 1, "-" gives 1, "0x" gives 2, "0xg" gives 2, "-0x" gives
   3. Only bytes in step 2 to 5 count, so "+1" and "" give 0.

atoi and atol return the same on every string, and differ only in cost. strtoul
differs from strtol only over a minus sign. `_get_errno` after these calls was not
measured (see Open), but no result here shows a limit being applied.

| string | base | strtol | end | strtoul | end |
|---|---|---|---|---|---|
| "-1" | 10 | FFFFFFFFh | 2 | 0 | 0 |
| "+1" | 10 | 0 | 0 | 0 | 0 |
| "  42" | 10 | 42 | 4 | 42 | 4 |
| "42" | 2 | 0 | 0 | 0 | 0 |
| "42" | 7 | 30 | 2 | | |
| "077" | 0 | 77 | 3 | 77 | 3 |
| "077" | 8 | 63 | 3 | 63 | 3 |
| "0x" | 10 | 0 | 2 | 0 | 2 |
| "0b2" | 10 | 0 | 2 | | |
| "zz" | 36 | 1295 | 2 | 1295 | 2 |
| "12" | 1 | 12 | 2 | | |
| "12" | 37 | 12 | 2 | | |
| "2147483648" | 10 | 80000000h | 10 | 80000000h | 10 |
| "0o7", "0d9", "0t1" | 10 | 0 | 1 | | |

## qsort(base, n, size, cmp)

Probes `csort` and `cqedge`. A C function pointer works, and so does a Go function
taken as a function value (the word after the context word of the value).

The compare is called with two element addresses, and only its sign counts
(`csort` repeats a case with results of ±7FFFFFFFh and gets the same compares). The
sort below gives the exact sequence of compares and the exact final order: it meets
all 76 cases of `csort`, 25 printed compare by compare with the array as it stands,
51 of them with sizes from 1 to 16 bytes, n up to 64 and unaligned bases. Equal keys
are not kept in order. Elements are swapped a byte at a time, and nothing outside
the array changes.

Element indexes here run from 0, in unsigned arithmetic:

```
qsort(base, n, size, cmp):
    if size is 0: return, with no compares
    sort(0, n-1)

sort(lo, hi):
    if lo >= hi: return
    swap elements (lo+hi)/2 and hi
    i = lo
    j = hi-1
    loop:
        while cmp(element i, element hi) < 0: i = i+1
        while j > i and cmp(element j, element hi) > 0: j = j-1
        if i >= j: leave the loop
        swap elements i and j
        i = i+1
        j = j-1
    swap elements i and hi
    sort(lo, i-1)
    sort(i+1, hi)
```

The first inner loop has no bound. It stops at element hi at the latest, which
compares equal to itself, so `cmp(element hi, element hi)` is a real call that shows
in the sequence: "21" gives 0:1 and 1:1. The swap after the loop happens even when
i is hi, which changes nothing but takes time.

Worked example, keys 2 3 1 (n 3). The first swap gives 2 1 3. Compares 0:2, 1:2 and
2:2, leaving i at 2. The left range (0,1) swaps to 1 2 3, compares 0:1 and 1:1. The
result is 1 2 3 with 5 compares.

### Inputs that do not come back

Where the pivot of a range starting at index 0 ends at index 0, `sort(0, i-1)` is
`sort(0, 0xFFFFFFFF)` in unsigned arithmetic, and Sony's qsort then does not come
back: it runs out of stack and faults, or overwrites memory and hangs. That is most
inputs of a few elements, and it depends on the keys. The rule says 12 of the 24
orders of 4 distinct keys come back: `csort` runs those 12 and they do. Of the other
12, the 10 that were run stop as it says (4132 and 4312 were not run). The rule also
named every stop among 2 and 3 keys that was run. The sorted pair "12" stops, since
its first swap puts the larger key at index 0, as do "11" and "312". n of 0 stops in
the same way, and n of -1 had not come back in 600 frames (`cqedge`, with its
constants changed by hand between runs).

The same fault does not arise for a range starting above index 0: `sort(2, 1)` just
returns.

None of this needs matching. In the clean library treat an empty range as empty and
go on, so that an input that stops Sony's sorts by the same rule. An input that
comes back on Sony's must give the same sequence.

## bsearch(key, base, n, size, cmp)

Probes `cbsearch` and `cneg`. The compare is called as cmp(element, key), the
element's address first, and only the result's sign and whether it is 0 count. This
rule meets all 354 searches of `cbsearch`:

```
p = base
loop:
    r = cmp(p, key)
    if n is 0:
        return p if r is 0, else 0
    if r < 0:
        p = p + (n/2 + 1)*size
        n = (n-1)/2
    else:
        n = n/2
```

n is unsigned, so n of -1 runs 33 compares and comes back. Sony's search is wrong
in three ways that must be kept:

- It always makes at least one compare, even with n of 0, and it may compare past
  the end of the array, since p can reach base + n·size. A match there returns that
  address.
- A compare of 0 does not stop it. p stays where it is and n halves down to 0, with
  a compare at each step, so a match takes as many compares as a miss below it.
- It can miss a key that is there: with keys 10, 20 (n 2), key 20 compares element
  0 then element 2 and returns 0. With 10, 20, 30, 40 (n 4), key 20 compares
  elements 0, 3 and 3 and returns 0.

Null pointers are not checked: the key and base are only passed to the compare and
added to. size of 0 walks the same element, with n still halving (`cneg`: 4 compares
at n 8).

## setjmp(buf) and longjmp(buf, v)

Probe `cjmp`. setjmp writes 12 words at buf, and no others:

| word | 0 | 1 | 2 | 3 to 10 | 11 |
|---|---|---|---|---|---|
| holds | ra | sp | fp | s0 to s7 | gp |

It returns 0, and changes only v0 among the registers the probe holds known values
in (at, v1, t0 to t8, s0 to s7, gp, fp).

longjmp loads all 12 from buf and returns to the saved ra, as if setjmp returned
again, with v as the value, unchanged. In particular `longjmp(buf, 0)` makes setjmp
return 0, not 1. -1 and 12345678h come through as they are. It leaves at, v1 and t0
to t8 as it found them, as far as they show (at is the probe's at that point).

A jump out of six nested calls works (`go setjmp` lines).

## Cost

Probe `ccost`, cycles from root counter 2 in system clock mode, read in assembly
right around the call with interrupts off, smallest of 6 runs, less the 10 of the
same reading around an empty function. L is a length and all costs fit exactly
except where said. Costs do not depend on alignment.

| call | cycles |
|---|---|
| strlen | 14 + 10L |
| strcpy | 20 + 10L |
| strncpy, n < L | 16 + 18n |
| strncpy, n > L, L the string's length | 28 + 8L + 10n |
| strcat, lengths D and S | 106 + 18D + 20S |
| strncat, S ≤ n | 32 + 8D + 16S |
| strncat, n < S | 42 + 8D + 16n |
| strcmp, equal for L | 26 + 14L |
| strncmp, stopped by n | 14 at n of 0, else 20 + 20n |
| strncmp, stopped by a terminator at L | 30 + 20L |
| strchr, not found | 22 + 12L |
| strchr, found at L | 16 + 12L |
| strrchr, not found | 32 + 22L |
| strrchr, found as the last byte | 32 at L of 0, else 38 + 8L |
| strpbrk, not found, set of M ≥ 1 | 14 at L of 0, else 12 + L(20 + 18M) |
| strpbrk, one byte, empty set | 30 |
| strspn, all L match at place P in the set | 8 at L of 0, else 10 + L(30 + 14P) |
| strcspn, none of L match, set of M | 8 at L of 0, else 10 + L(24 + 14M) |
| strstr, one-byte sub not found | 14 + 24L |
| strstr, two-byte sub found at L | 68 + 24L |
| strtok, token of L ≥ 1, set of 1, ended by a delimiter | 52 + 34L |
| strtok, L leading delimiters, set of 1, then one byte | 64 + 36L |
| memchr, not found | 8 at n ≤ 0, else 18 + 14n |
| memcpy, bcopy | 10 + 12n |
| memmove, d before s | 10 + 14n |
| memmove, d at or after s | 10 at n of 0, else 18 + 14n |
| memset, bzero | 8 at n ≤ 0, else 10 + 8n |
| memcmp, bcmp, equal | 24 at n of 0, else 8 + 16n |
| atoi, L digits | 120 + 48L |
| atol, L digits | 136 + 48L |
| strtol, L digits | 178 + 48L, base 0 182 + 48L |
| strtoul, L digits | 162 + 48L |
| each leading space, atoi and strtol | 16 |
| toupper, tolower | 18, or 20 if the code changes |
| setjmp | 26 |
| longjmp | about 34 (`cjmp` measures 52 by a longer path) |
| null: strlen, strchr 8; strcpy, memcpy 4; strcmp 12; strcat 28 | |

strcat's and strncat's first figures are measured with one length at 0; the sum is
assumed. strrchr found elsewhere than the last byte was not measured. The `ccost`
table gives lengths 0, 1, 2, 4, 16 and 256.

qsort over n descending keys, with `ccost`'s compare, which takes 20 past an empty
call:

| n | 1 | 2 | 3 | 4 | 5 | 8 | 16 | 32 |
|---|---|---|---|---|---|---|---|---|
| cycles | 236 | 990 | 1270 | 2072 | 3106 | 5086 | 10660 | 23362 |
| compares | 0 | 2 | 3 | 6 | 9 | 18 | 46 | 120 |

236 + 48C + 194R + (88 + 36·size)W fits these up to n of 8 exactly and within 0.3%
above that, C being the compares (the 48 includes the compare's own 20), R the
ranges sort works on (lo < hi), and W the swaps, including the two in sort's first
and last lines. Over n of 8 by size: 3682,
4150, 5086, 6958, 8830, 10702 for sizes 1, 2, 4, 8, 12, 16.

bsearch, with the same compare: 94 at n of 0. Otherwise 96, plus 50 for each step
with r < 0 and 48 for each step with r ≥ 0, a step being each compare made while n
is not 0:

| n | 0 | 1 | 2 | 4 | 8 | 16 | 32 | 64 | 128 |
|---|---|---|---|---|---|---|---|---|---|
| key below all | 94 | 144 | 192 | 240 | 288 | 336 | 384 | 432 | 480 |
| key above all | 94 | 146 | 146 | 196 | 246 | 296 | 346 | 396 | 446 |

## Open

- `strcpy` with d just after s. A forward byte copy never finds a terminator, and a
  run of this call had not ended after 8 minutes when it was stopped, with no output
  to show why. Not worth matching.
- setjmp and longjmp with a null or unaligned buf.
- `_get_errno` after strtol and friends. OpenBIOS has no B0h 54h, so the call halts
  the run; spec 12's harness stands in for it. No result suggests these calls set
  it, but that was not measured.
- What the compiler may do with memset's, memcpy's and memmove's return values.
  The target's C file serves as the compiler's own support for Go code in the image
  as well as these exports. A compiler that knows these names may use the C
  standard's result, which Sony's memset (n of 0) and memmove do not give. Calls the
  compiler itself generates from its memory intrinsics ignore the result, so this
  looks safe, but it was not tested. Should it matter, keep separate functions for
  the exports.
- TinyGo's runtime declares `strlen` as reading memory only through its argument, so
  a Go declaration with a uint32 parameter gets an old result back. The probe uses
  unsafe.Pointer for that entry alone. An export on the clean side should watch for
  the same, in its tests.
- qsort with an inconsistent compare, a size that is negative or larger than 16, and
  bsearch with a negative size.
- strrchr's cost for a last match away from the end.
- Stack use. C programs may run on the scratchpad (spec 15), so keep it small. qsort
  recurses on both sides, by depth of the keys, and Sony's stops being usable at a
  depth the probes did not reach.

## Acceptance

1. Each of these probes, built with `exp/cleanlib/image/probe.sh`, gives the same TTY
   on the clean image as on Sony's, from its first line on: `cstr`, `cmem`, `cnum`,
   `csort`, `cbsearch`, `cjmp`, `cneg`. (Before the probe starts the TTY shows the
   boot, which spec 15 covers.)
2. `cjmp`'s `longjmp cycles` line and `ccost`'s table differ only where this spec
   says a figure is approximate. `ccost` is otherwise a timing check, not a must:
   report its differences.
3. `cqedge` is a Sony's-side record and does not need to match past the line before
   the call that stops it.
4. `exp/cleanlib/image/make.sh` no longer lists these entries in
   `.build/cleanimage/missing.txt`.
5. `../regress.sh` and `image/regress.sh` stay at their baselines.

## What the first runs showed

On the clean image, from outside, with `probe.sh` (2026-11-04).

- `cstr`, `cmem`, `cnum`, `csort`, `cbsearch` and `cneg` are identical to Sony's, and
  `cjmp` apart from its `longjmp cycles` lines, 54 against 52: an entry's jump costs
  an instruction that Sony's does not have, and setjmp is 28 against 26 the same way.
- `cqedge` matches up to the call that stops Sony's: n of 0 returns with no compares
  on the clean image.
- The compiler turns a plain counting loop over bytes into a call of `strlen`, which
  the image does not have (the link fails), and a plain clearing loop into the
  target's `memset`, 44 cycles more. The loops that find a terminator have a store in
  them, which is also Sony's time a byte. memset, bzero and calloc store through
  volatile.
- `ccost`'s `kept` lines differ in one way: the clean entries change at, as the
  compiler's global accesses do, where Sony's do not. The C convention gives at to the
  call. s0 to s7, fp and gp are kept.
- The costs in `ccost` by the byte stay above Sony's where the compiler's loop is longer
  than Sony's: strlen 12 a byte against 10, strcpy 12 against 10, strcmp 20 against 14,
  strchr 16 against 12, memcmp 20 against 16, memmove backward 20 against 14, strrchr 24
  against 22, strncat 12 and 20 against 8 and 16. Calls cost 2 to 26 more at their base.
  qsort, bsearch, strcat and the digits of strtol and friends are paced to the table.
