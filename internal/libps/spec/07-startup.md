# Milestone 6: the start-up

Status: implemented in playstation-yaroze-clean.S. TestCleanStart
matches the timing below to the cycle: 58 cycles from InitHeap's entry
to the system call and 486 more to main, with the clean InitHeap. The
clean executable is 488 bytes smaller than with Sony's start-up, which
moved invaders' load by a sector and made it pass. When `_fbss` equals
`_end` nothing is written.

A program built against the clean library still starts in Sony's
start-up routine (the crt0 from libps.a). It is the last of Sony's code
in a clean build: no other member of Sony's libraries is linked. This
milestone replaces it.

## Where it goes

The build already has the hooks:

- `exp/cleanlib/build.sh` builds with the target
  `playstation-yaroze-clean` (`internal/toolchain/_tinygo/targets/
  playstation-yaroze-clean.json`). It inherits `playstation-yaroze-go`,
  adds the `cleanlib` build tag and defines `LIBPS_CLEAN` for C and
  assembly.
- Under `LIBPS_CLEAN` the target's first assembly file includes
  `playstation-yaroze-clean.S` in place of Sony's start-up, so the new
  start-up keeps the first place on the link line. That file does not
  exist yet. Write it in `internal/toolchain/_tinygo/targets/`. Its
  `_start` must be in `.text` (not a `.text.*` section), so that the link
  script puts it at the load address, 0x80010000.

The C `main` it calls is the TinyGo runtime's (`runtime_yaroze.go`),
which never returns. `InitHeap` is the clean library's own.

## What it does

Observed with `TestCleanStart` (below) on a program built for Sony's
resident library (`.build/hello-sony`), on the reference build and on the
clean build with Sony's start-up. All three behave the same way.

At entry the BIOS has set sp and fp to 0x801ff000, gp to 0, and
interrupts off (SR 40000000h for a program loaded at 0x80010000).
Then, in order:

1. gp is set to `_gp`, the link script's symbol.
2. The words from `_fbss` up to, but not including, `_end` are set to 0,
   in ascending order. Nothing below `_fbss` or at or above `_end` is
   touched.
3. sp is left as the BIOS set it. fp is set to sp.
4. `InitHeap(_end, sp - 0x10000 - _end)` is called: the heap is
   everything from the end of the image to 64KB below the stack (for
   hello-ref, InitHeap(8001c52c, 001d2ad4)).
5. Interrupts are enabled with ExitCriticalSection (system call 2,
   a0 = 2). SR is 40000401h after it.
6. `main` is called.

Registers at main: gp = `_gp`, sp unchanged, fp = sp, a0 = 2 (the
system call's argument), and v0, v1, a1, a2, a3 and s0 to s7 as they
were at entry (the BIOS's values, kept across InitHeap).

If main returns (it does not in a Go program), the conventional ending
is EnterCriticalSection (system call 1) and a return to the address ra
held at entry. This was not observed.

## Timing

The time to main is what TestClean's boot timing depends on, so match
it to the cycle:

- InitHeap is reached 146 cycles after entry plus 10 cycles per word
  cleared (hello-ref: 7297 words, +73116; the clean build with Sony's
  start-up: 11249 words, +112636).
- With the reference's InitHeap and ExitCriticalSection, main followed
  at +73640 for hello-ref.

Interrupts are off for the whole clearing, so no interrupt changes the
count.

## Boot timing, measured (not the start-up's)

The start-up is not why boot timing differs between a clean build and
its reference: both used the same one. Two things outside the library
decide when main starts:

- The executable's size. The BIOS loads it sector by sector, so a
  bigger executable starts later, at another point of the video frame.
  build.sh pads the clean executable to the reference's size (unless
  `PAD_TO_REF=0`), and then entry falls on the same cycle in both.
- The size of `.bss`, at 10 cycles a word. The clean builds' `.bss`
  differs from the references' both ways by up to some 1,700 words
  (hello 785 fewer, square 764 more, scene 1,726 fewer on 2026-11-02),
  so main would come up to about 17,000 cycles earlier or later; TestClean
  evens this out by lengthening the smaller build's clear (`readBSS`).

With the padding, tunnel passes TestClean and dino's sound writes match.
invaders still differs: the reference's GsInitGraph (its first GP1 05h)
comes about 170,000 cycles later after main than the clean one's,
which is the ResetGraph wait in spec 05. That is the library's, not the
start-up's.

## Tests

`TestCleanStart` with `PSX_START=NAME` (a build in `.build/NAME`) logs
the registers at entry and at main, the kernel calls, system calls and
the InitHeap call on the way with their cycles, and which words of RAM
between `_fbss` and 64KB below the stack changed. Run it on
`hello-ref` and on the clean build and compare. Then every example in
`regress.sh` must give the same result as before, since nothing after
main changes.

## The library's start-up (2026-11-02)

What the start-up above does not cover is the library's own start: the
work Sony's resident library did before the member's program ran. A
program built for Sony's library in this repository runs it the same
way: the executable carries a stub that calls the resident library's
initialization routine and then jumps to the program's entry (the crt0
above), so the routine's work lies before `main`, and before anything
TestClean records. The reference library and the clean library have no
routine before the program, so the program's first `ResetGraph(0)` or
`ResetGraph(3)` does the same work, once, right after its GPU reset.
Sony's routine, traced from its first instruction (iotrace with
`START=80010030` on a Sony build), does this, in this order, with the
cycles from its start:

1. ResetCallback's kernel calls (spec 05, "found on the way"), from
   about +8,500: the library's handler is on the kernel's exit from
   then on (its first ReturnFromException at about +231,000, the first
   vertical blank).
2. SetDispMask(0) (GP1 03000001h at +12,900), then ResetGraph(0): its
   "ResetGraph:jtb=...,env=..." line (spec 05, about 164,000 cycles of
   printing), A0h 49h GPU_cw at +177,000, and the GPU reset.
3. B0h 12h InitPad with the two 34-byte buffers at +232,000; the kernel
   prints "PS-X Control PAD Driver". Sony's routine never calls
   StartPad: the ports are read every vertical blank once the card
   driver starts (step 6, InitCard(1) means "with the pads"). The
   reference calls StartPad and ChangeClearPad(0) here instead, and the
   kernel's work each frame is the same either way: 40 accesses of the
   serial port, 5,782 cycles, measured on terra against Sony's run.
4. The drive's reset, from +424,000: the CD paragraph of spec 05
   (Getstat, Init, Demute and the five SPU words at +647,700).
5. The sound system's initialization, from +647,800: the register
   writes of spec 05's "Initialization" (531 more), the SPU event
   opened (B0h 08h, class F0000009h, +743,700) and enabled (B0h 0Ch),
   ChangeClearPad(1) then (0) with ChangeClearRCnt. The tick runs at
   every vertical blank from here: 24 envelope reads and the six words
   (188, 18a, 18c, 18e, 198, 19a), every frame, whether or not a
   sequence is open. Whether the tick is enabled at the end of this step
   or at the end of the whole routine cannot be told apart: in Sony's
   run and in every reference build no vertical blank falls between this
   step's last ChangeClearPad(0) and StartCard, and from StartCard until
   the card driver's ChangeClearPad(0) the kernel acknowledges the blank
   itself, so no tick comes then in either. That is why the setup
   section of a run's SPU stream usually has no tick at all.
6. The card driver, as spec 11's "Starting the card driver" lists it:
   the eight OpenEvents from +777,000, InitCard(1) at +800,200,
   StartCard at +807,300 (the kernel's first serial port access follows
   at once), _bu_init at +808,700, the EnableEvents. The kernel's card
   work goes on to about +2,500,000.

The whole routine takes about 2.5 million cycles, 4.4 frames, before
the program's entry. The TTY shows its order: "ResetGraph:" first, "PS-X
Control PAD Driver" second, "addr=..." (the CD system's, spec 05) third,
then the program's own lines. In a run of a Sony build these lines come
before the program and TestClean's recording; in the reference's and
the clean library's they are the first ResetGraph's, and so is the
start-up's work: the setup section of TestClean's SPU stream (before the
first swap) holds the five words, the 531 and the ticks until the swap,
and the program's first frames come some 2.5 million cycles later than
on a Sony build. The reference's own `ResetGraph:` line is printed as
before; InitPad's message follows it, then the drive's.

A program that calls one of these systems before any ResetGraph starts
that system there, as the reference does: GetPadBuf starts the pads
(after ResetCallback, which puts the library's handler in place), a
sound or CD call runs the drive's reset and the sound initialization, a
card call starts the card driver (and the pads first, spec 11). The
first ResetGraph then skips what is already started. Nothing is ever
started twice.

This replaces the lazy starts the earlier specs described (the pads at
GetPadBuf, the sound system at the first sound or CD call, the card
driver at the first card call); specs 02, 05, 11 and 14 point here. The
reason: Sony's library runs with all of it started, and the per-frame
cost of the pad polling, the card driver and the tick (about 11,000
cycles a frame together) moved a program's frames against Sony's when
the clean library started them later. Whether all of it is worth doing
for a program that uses none of it is a question for after the library
matches.

Done when: for every run in `regress.sh`, the clean library's kernel
calls, TTY lines and the SPU words of TestClean's setup section match
the reference's, and the runs that passed before pass again.
