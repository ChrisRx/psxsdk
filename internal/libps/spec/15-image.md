# Milestone 15: the resident image

So far the clean library has run linked into each program, through the
`playstation-yaroze-clean` target. This milestone lays it out as the resident
library's image: at its old place below 0x80090000, with a jump at each entry
point, so that a program built for Sony's library runs on it unchanged, whether
built in this repository or by Sony's SDK years ago. Such a program cannot be
rebuilt, so what it calls and when is fixed.

## The tooling

`exp/cleanlib/image` holds it, and it is the spec side's:

- `make.sh` builds `.build/cleanimage/libps.exe`, in about ten seconds.
  `.build/cleanimage/missing.txt` lists the entry points the library lacks.
  Each of those parks the program in a loop, with DEAD0000h in t8 and the low
  half of the entry's address in t9.
- `main.go` is the image's program. Its `setup` runs when a program's start-up
  calls the image's init routine, after the Go runtime has started.
- `regress.sh [NAME...]` builds each example for Sony's library twice, on
  Sony's image (`.build/NAME-cimg`) and on the clean one
  (`.build/NAME-cimg-clean`), and runs TestClean on the pair with
  `PSX_CLEAN_REF=cimg`. The program is byte for byte the same on both sides, so
  every difference is the library's. It prints a line per run, as
  `../regress.sh` does, with logs in `.build/cimg-regress/`.
- `probe.sh NAME DIR` builds a probe for Sony's library and runs the one program
  on both images, writing `.build/NAME-cimg/out.tty` and `.kernel` and
  `.build/NAME-cimg-clean/out.tty` and `.kernel`. It stops at a line `END` on the
  TTY. `SIDES=sony` runs Sony's side alone.
- An entry that takes C's variable arguments is exported as `libps_va_NAME`, with
  one parameter, a pointer to the argument words: `args[0]` is what a0 held, and
  on up through a3 and the caller's stack arguments, in the o32 layout (a double
  takes two words, starting at an even index).
- An entry whose name the Go runtime takes (malloc, free, calloc, realloc) or the
  compiler treats specially (strlen) is exported as `libps_c_NAME`. A probe reaches
  such an entry by its address, as `spec/probes/heap` does.

The image has no C library of its own, no libm and no compiler-rt. All it holds
is the clean library, the target's `playstation-libc.c`, and the jumps.

## Stage 1: the init routine and SsSetTickMode

### Baseline

Before this stage, 8 of the 41 runs reach all 300 swaps: breakout, car, car:9410,
clone, flame, invaders, morph and offscr. Where their first difference is shown,
it is in the setup: the clean side writes some 530 more SPU words. dino, dungeon,
oden and tunnel stop early, for reasons not looked into. The rest make no swap:
17 of the 27 programs call SsSetTickMode, which hello's `Screen` reaches through
SetVideoMode, and that parks them.

### The init routine

A program built for the resident library calls the init routine, from a stub
that then jumps to the program's entry. TestClean starts recording at that
entry. So all the work of "The library's start-up" in spec 07, steps 1 to 6,
lies before the recording on Sony's side, and it has to on the clean image's.

Add to `exp/cleanlib` an exported `Startup()`, not a C export, which does those
six steps in spec 07's order: ResetCallback, SetDispMask(0), ResetGraph(0) with
everything its first call does, the pads, the drive, the sound system and the
card driver. It also calls SsSetTickMode(5), which is how Sony's routine leaves
the mode (observed: a0 5 on an entry to SsSetTickMode made from inside the
routine), and that must come before the first sequence opens. Have
`exp/cleanlib/image/main.go`'s `setup` call it: that is the one file of the
tooling to edit.

As spec 07 says, nothing is started twice. A program's own `ResetGraph(0)` after
Startup does what any later one does. The linked-in `playstation-yaroze-clean`
target does not call Startup, and keeps starting things at the first ResetGraph,
so `../regress.sh` must stay at its baseline of 33 PASS, 0 FAIL, 8 TIME.

The routine returns to its caller with the callee-saved registers, sp and gp as
it found them. The program's start-up has yet to run.

### SsSetTickMode

`void SsSetTickMode(long mode)`, a C export. Probe `spec/probes/tickmode`, on
Sony's library: its output is `.build/tickmode-sony/out.tty`, from
`.build/voiceprobe` with FRAMES=6000. The probe declares the entry itself, since
the yaroze package has no binding for it.

The mode sets the rate a sequence's tempo is worked out against, in ticks a
second. It does not change how often the tick runs: that stays one tick a
vertical blank in every mode (the dump's 18Ch writes). Sequence ONE, a note at
tick 48 at its header's tempo, first changes a voice at these frames from
SsSeqPlay:

| rate | 240 | 120 | 60 | 50 | 30 | 6 |
|---|---|---|---|---|---|---|
| frame | 61 | 31 | 16 | 14 | 9 | 3 |

The rate for each mode, with V being 60 under NTSC and 50 under PAL:

| mode | rate |
|---|---|
| 0, 5 | V |
| 1 | 60 |
| 2 | 240 |
| 3 | 120 |
| 4 | 50 |
| 6 and up | the mode |
| negative, bit 12 clear | 60 |

- With bit 12 set (1000h), the rest of the mode's low 12 bits are the mode as
  above: 1000h and 1005h give V, 1001h gives 60, 101Eh gives 30, 103Ch gives 60.
- -1 has that bit set and so gives a rate of 4095: the note does not come in 300
  frames, for the reason below (a tempo under a tenth of a tick a blank).
- A negative mode with bit 12 clear gives 60 under NTSC and under PAL, whatever
  the mode before (probe `spec/probes/tickrate`, -8192). A mode over 4095 with
  bit 12 clear is the mode: 8192 and 65536 play as the rates they say. 0x3005
  has bit 12 set and gives V.
- SetVideoMode sets the rate back to V for the new mode, whatever the mode was:
  `SsSetTickMode(2), SetVideoMode(1), SsSeqOpen` plays at frame 14, and at 16
  with SetVideoMode(0) (`tickrate`). On the clean image a program's SetVideoMode
  reaches the image's export and then SsSetTickMode(5) (seen on the entries, a0
  and ra, in the program's code), so V is the program's mode and the question in
  Open does not arise: all of `tickmode`'s PAL lines match on the image too.
- A tempo under a tenth of a tick a blank (res·bpm·10 below rate·60) plays by the
  inverse, as spec 10 has it: rate·60·100/(res·bpm·10) tenths a blank, rounded
  down, against any rate. But the sequence goes no further after a blank that
  plays nothing. It advances again only for a new tempo or a new start. So at 213
  tenths (rate 4095, 120 beats) a note at tick 21 plays at frame 2, one at tick 42
  at frame 3, and one at tick 43 never; at 375 (rate 60, tempo 1), ticks 37, 75
  and 76; at 1500 (rate 240, tempo 1), ticks 150, 300 and 301; at 3413 (rate
  65536), ticks 341, 682 and 683. A blank that plays something earns the next: at
  375, a note at tick 38 is followed by its note off 48 ticks on and the end,
  over 860 tenths in three blanks, while the note at tick 37 is left playing, its
  note off at 850 being past the second blank's 750 with nothing played in that
  blank. This corrects spec 10's "(2) 187, (3) 125" as a rate that goes on: ONE
  does not play at either tempo, as 187 and 125 stop short of tick 48. The tempo's
  rate is otherwise the old one, rounded, for res·bpm·10 from rate·60 up.
- The rate is read when it is needed, not stored in a sequence. SsSeqOpen reads
  it, and so does SsSetTempo. A sequence already open keeps the timing it has
  until then: `open, then mode N` plays at frame 16 under NTSC and 14 under PAL
  for every mode, and `open at 5, mode N, SsSetTempo 120` plays at the new mode's
  frame.
- V comes from the video mode when SsSetTickMode is called. Under PAL, modes 0
  and 5 give 50; under NTSC, 60.

### Acceptance

1. The `tickmode` probe's lines, built with `../build.sh` (PAD_TO_REF=0), equal
   Sony's in `.build/tickmode-sony/out.tty`.
2. `../regress.sh` stays at 33 PASS, 0 FAIL, 8 TIME.
3. `image/regress.sh` improves as far as it can. Report every run's line, and
   for each run that still fails, the first difference and what you found.
   Timing differences that remain are worth reporting and not worth padding yet:
   say which they are.

### What the first runs showed (2026-11-04)

Measured on the clean image from outside, with a temporary test in `tests/`.

- dino, dungeon, oden and tunnel stopped early, and the others would have, on
  "fatal error: out of memory" (the TTY). The image has its own Go heap of 32 KB
  and the runtime's allocator never frees. GsSetLightMatrix's local matrix went
  on the heap, 32 bytes a call: 1,025 calls filled it. Linked in, the same leak
  had the program's whole heap to fill and never showed. Nothing in the library
  may allocate after the start: `llvm-objdump -d image.elf` finds no call of
  `runtime.alloc` now. The CD calls' seven small allocations went too.
- The library loaded the GTE's control registers through gp and left the value
  there, so a program's gp did not survive GsInit3D or GsSetLightMatrix: tunnel
  had gp 00001000h. Each now puts gp back. $at cannot hold it meanwhile, since
  the compiler allocates $at for the operands.
- gp at a VSyncCallback function came from the exit buffer, which held 0.
  VSyncCallback now puts its caller's gp there (dispsys on the image: 8009B680h
  at the call and in the function).
- At the program's entry the registers s0 to s7, gp, sp, fp, ra and I_MASK are
  the same on both images (hello), and the entry itself comes within 76 cycles of
  Sony's (breakout +76, hello -30).

What is left, none of it the program's output apart from the font:

- Sony's font is not the clean font, as CLEANROOM.md requires. With the GPU
  streams equal, the last picture differs only in text, foreground against
  background, and on hello video memory differs only in the text and the font's
  own rectangle (x 960 to 991, y 256 to 287). oden, tunnel and quest load the font
  too and pass, their last pictures showing none of the glyphs that differ.
- Sony's library, at about the program's first VSync (spec 08), calls B0h 56h
  GetC0Table and has the kernel print "Found C0 patch" with interrupts off, then
  A0h 44h FlushCache: 456,996 cycles on breakout from the first ReadN's point of
  view, which the clean image neither takes nor prints. Every program that loads
  from the disc after that reaches its CD reads and uploads that much sooner, and
  the times from then on match (the first ReadN to its Pause 6,962,714 against
  6,962,614 on breakout). That is the frame edge at the setup's SPU difference:
  SsVabTransfer one frame early. With the idle ticks and the FIFO words removed,
  breakout's, invaders', dogtale's, check's and clone's SPU streams are equal
  throughout.
- The readouts of a frame's time differ (check, dungeon, flame, morph, offscr,
  timers) and square's loading bar: the words that differ are the digits' or the
  bar's.
- terra: three key ons (frame 786 of the dump) write pitch 0001h on Sony's where
  the clean library writes 0. These are the effects spec 05 and `stepPitch` call
  keyed on from an overwritten header, a negative table index, which the spec
  leaves open. The reference writes 0 too.

## Stage 2: the C library

The entries left in `.build/cleanimage/missing.txt`, and the C forms of FntPrint
and KanjiFntPrint. The specs are `16-cstring.md`, `17-cformat.md` and
`18-clib-rest.md`, each with its Acceptance. Original programs call these, so they
are written for C callers, under the rules below. A scan of direct calls in 165
programs found printf in 85 and malloc in 41.

Where the code goes:

- In `exp/cleanlib`, as Go, with `//export NAME`.
- `//export libps_c_NAME` for a name the Go runtime takes (malloc, free, calloc,
  realloc) or the compiler treats specially (strlen). The image's entry jumps there.
  memcpy, memset, memcmp and memmove are this case too: the compiler calls the
  target's own, in `playstation-libc.c`, and leaves them as they are, while the
  entries follow spec 16.
- `//export libps_va_NAME` for variable arguments (see "The tooling"). The linked-in
  `//export FntPrint` and `KanjiFntPrint` stay for the Go programs on that target.
- `exp/cleanlib/image/clib.S` for what Go cannot do, as setjmp and longjmp. Each
  `.globl` there is an entry.
- `math_errno` is a C variable: `yaroze/` style `//go:extern`, with its storage already
  in `playstation-yaroze-libps-data.c`. The image lays it out at its address.

Spec 12's math calls gain what spec 17 says about math_errno and `_err_math`. The
putchar export returns the kernel write's result (spec 18).

Nothing here may allocate from the Go heap, as stage 1 says: `llvm-objdump -d` of
`.build/cleanimage/image.elf` shows no call of `runtime.alloc`, and the symbol is gone.
A Go function compiled with nil checks stops where C reads address 0, so take pointers
as `uintptr` or otherwise reach the behavior the specs give for a null. Stack use stays
at or under the figures in spec 17's Stack section, and small elsewhere.

Sony-only crashes and hangs are not matched: the specs name them (qsort's are in spec
16). Do what they say there, or the sensible nearest thing if they say nothing, and
report it.

Costs: the specs give cycles. Pace to them where a loop's shape gives it cheaply, in
the way the rest of this library is paced, and report what stays different. No program
is known to depend on these yet, so a cost is not worth an unreadable function.

### Acceptance

1. Each spec's Acceptance, by `probe.sh`, reporting every difference left.
2. `missing.txt` is empty, and the image still fits below 0x80090000.
3. `../regress.sh` stays at 33 PASS, 0 FAIL, 8 TIME.
4. `image/regress.sh`: 7 PASS, 10 TIME, 24 FAIL before this stage, with the 24 being
   the font's pixels and the setup's frame edge. No run gets worse.

## Rules for code a C program calls

These apply to every export from now on, since programs built by Sony's compiler
call them. The Go programs in this repository do not break these rules, so the
linked-in target has not shown them.

- A C program uses gp for its small data. Nothing the library does may change
  gp on return, or across an interrupt.
- A C program may keep its stack in the 1 KB scratchpad, so keep exports' stack
  use small: no large local arrays.
- C allows a null pointer where Go stops with a fault. See Open.

## Open

- A program for Sony's library carries Sony's linked members in its own code.
  SetVideoMode's is settled above (it reaches the image's export). The others, if
  any keep their own copy of a state the image also keeps, are not measured.
- A delta time of 4800 ticks plays at the right frame at a rate that reaches it
  (`tickrate`, 151 at rate 6), so the total is not 16 bits short there. No
  larger delta was tried.
- What a stalled sequence (a tempo under a tenth of a tick) does on SsSeqPause or
  SsSeqReplay was not measured, and an accelerando or a tempo event while stalled
  neither: the clean library's accelerando goes on from where it stands.

## Baselines on the image

`image/regress.sh` runs on the retail BIOS, with Sony's image carrying the clean font, as
`image/sonyfont.py` builds it. Its baseline is 34 PASS, 0 FAIL, 7 TIME. `image/archive.sh` and
`image/terra.sh` compare original programs: specs 20 to 23 cover them. `make.sh` learns section
sizes from a link that reads the compile cache, since a package compiled just now can differ in
size from the same one read back, and it removes the old image first, so a failed link leaves
none behind.

## Comparing with Sony's, once its bytes are out of the repository

Nothing in the repository holds Sony's code after step 3, so comparing needs two things kept outside it:

- A private copy of Sony's image. `PSX_SONY_LIBPS` names it, a PSX-EXE gzipped or not, and `image/regress.sh`, `image/archive.sh`,
  `image/probe.sh`, `image/terra.sh` and `tools/libpsdis` read it, by default from `$HOME/src/ChrisRx/psxsdk-sony/libps.exe.gz`.
  Without it those scripts stop and say so, and everything else still works.
- The local git branch `sony-reference`, the tree before Sony's code was removed. A worktree of it builds the reference discs
  (`exp/cleanlib/refs.sh`) and runs anything that needs Sony's linked-in members or the decompiled library.

A probe's two sides are then the same program, with the clean start-up and members, on Sony's image and on the clean one, which is
what compares the library. Sony's start-up and members can only be run from the branch.
