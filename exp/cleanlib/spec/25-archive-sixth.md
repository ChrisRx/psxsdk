# The web archive's programs: sixth round

After spec 24, `exp/cleanlib/image/archive.sh` has 97 of 140 programs `same`. Of the rest, 13 differ only in
the figures they print, 11 are not the library's doing, one runs wild on both, and 9 are off in time:
`scee-topten-cardem`, `event-event001-rave096`, `test-demo-fm`, `pssoft-koro-koro`, `scee-topten-adven`,
`pssoft-airbob-2-airbob`, `test-gr-01`, `test-pd961207` and `test-ftptest`. Spec 24's rule holds: accuracy comes
first inside the library, and assembly is open wherever it gets closer.

## Never time with a root counter

**This emulator's root counters count steps, not cycles.** `emu/machine.go`: "steps counts the steps run, which is
the timers' clock." Frames and `TotalCycles` go by what things really cost, GTE waits included. So on any path that
waits for the GTE, a root counter says something else. An 11-polygon lit 28h call, flag 0, attribute 0, on spec 24's
`gsocost` probe:

| | TotalCycles | root counter 2 |
|---|---|---|
| Sony's | 2,316 | 2,066 |
| clean | 2,098 | 2,178 |

The counter shows the clean library about 5% slow, and the truth is about 10% quick. That is spec 24's unexplained
gap: there is no special path on real models. `test-gr-01`'s first object takes 33,895 cycles on Sony's and 30,452
on the clean library, the same per polygon as the probe. Sony's is about 184 a polygon, the clean library's 164.

**Time only by `TotalCycles`**: `cyc` in `PROBE_WATCH`'s lines, `TestArchiveCalls`, `TestArchiveCosts`. Any earlier
figure measured by a root counter on a path with GTE instructions is wrong, in specs too. That is every per-path
figure for the GsSortObject4 loops, and possibly the matrix calls, GsGetLs and the like. Take none on trust.

## A trace from outside

`ARCHIVE_BUS="NAME:FROM:TO"`, `TestArchiveBus`, writes `calls/NAME-sony.bus` and `NAME-clean.bus`, and `PROBE_BUS=TEXT`
on `probe.sh`'s test writes `out.bus` once the TTY holds TEXT. Each has a line for each I/O register access and each
GTE instruction, with the cycle:

    io r 1f801074 4 0000000d cyc 590501248
    gte 4a486012 cyc 590822026

A GTE line is what the GTE sees: a command whole, and for a move, load or store only the operation and GTE register.
`emu/cpu/cpu.go` says what a command costs: the GTE runs while the CPU goes on, and holds the CPU at its next GTE
instruction. So how far apart these lines are decides what a path costs, and you can read that from outside. In
`test-gr-01`'s frame 12, the median cycles from a command to the next GTE instruction, Sony's/clean:

| command | Sony's | clean |
|---|---|---|
| RTPS | 0 | 16 |
| RTPT | 18 | 24 |
| NCLIP | 0 | 4 |
| AVSZ4 | 0 | 6 |
| AVSZ3 | 0 | 0 |
| MVMVA | 12 | 0 |
| NCCS | 32 | 18 |
| NCCT | 42 | 40 |

**Use this in place of any tracer of your own.** A tool must not read the instruction at the PC on Sony's side, even
to decide what to log: `CLEANROOM.md` now says so. Spec 24's `.build/p24/iotrace` did that, so don't use it again.

## Work

**GTE paths first.** Measure every kind of polygon under each light mode again in cycles, and match both the cycles
per path and the spacing on the trace: GsSortObject4's loops, then the matrix calls, GsGetLs, GsGetLw, GsGetLws,
GsSetLightMatrix, GsSetRefView2, gteMIMefunc and anything else with GTE instructions. Pads tuned to a counter on those
paths will now be wrong. Spec 24's assembly is a place to start, and rewriting it to fit is expected.

**Cut polygons in assembly.** `libps_gso_div` and `libps_gso_grad` are in Go. Spec 24 measured DIV1 about 34% over and
DIV2 about 15%, by the counter, so measure again. `test-dungeon-dungeon`, `test-dyddem01` and `test-dydref01` use them.

**Then the rest**, in order of total in `costs.txt`, by cycles:

| call | Sony/clean | note |
|---|---|---|
| FntFlush | 2,721/2,898 | about 180 over, on the queue's path. `scee-topten-cardem` spends 21,000 more by frame 60 |
| FntPrint | 907/998 | |
| GsLinkObject4 | 470/542 | exact on a probe by the counter: measure by cycles |
| printf | 19,560/20,188 | about 90 a digit over. `event-event001-rave096` reaches 98% of Sony's output |
| sprintf | 646/820 | |
| GsSortFixBg16 | 15,576/15,909 | |
| GsSortSprite | 246/254 | a quad with a turn is +8 to +10 |
| RotMatrix | 243/250 | |

**Sin and cos last.** Sony's varies by about 10,000 cycles with the argument, in no pattern found, and `test-demo-fm`
is 2,000 a call short. Do this if you get to it, and a table is fine if you find what it follows.

**The 9 programs.** Say what each shows after the work, and what stops any that isn't `same`.

## Acceptance

- `FIRST=1 exp/cleanlib/image/archive.sh scee-topten-cardem event-event001-rave096 test-demo-fm pssoft-koro-koro
  scee-topten-adven pssoft-airbob-2-airbob test-gr-01 test-pd961207 test-ftptest`.
- `ARCHIVE_TEXT` on `demos-scee-balls demos-scee-chap5-1 demos-scee-morph event-event001-hhpd0002
  scei-sample-graphics-bmirror scei-sample-graphics-cube test-bomb96-bomb test-dungeon-dungeon test-dyddem01
  test-dydref01 test-dydtst02 test-wnfs-t2-wnfs-t2 test-wstg-tv3`: still no difference.
- `exp/cleanlib/regress.sh` without `PSX_BIOS` in the environment, and `exp/cleanlib/image/regress.sh`: FAIL stays 0,
  and a new TIME has to be explained. Baselines: 33 PASS, 8 TIME, both. `exp/cleanlib/image/terra.sh`: all pass.
- The full `exp/cleanlib/image/archive.sh`, with counts before and after, and name anything that got worse. Before:
  97 same, 28 differ, 9 noboot, 3 shift, 1 wild, 1 tty, 1 stall.
- `costs.txt` refreshed, with before and after for the table above and for GsSortObject4.

## What the work on this spec found

Probes of Sony's library by the agent that did the work. Where these disagree with the sections above or with
earlier specs, these are right.

**Cut polygons** (spec 24): Sony's lights a cut polygon before dropping it. The order is H read (near), then OFX, then
OFY. The clean library has not been changed to match. By cycles, summed over the probe's kinds, clean is +12.8% sorted at
level 1, +5.7% at level 2, +2.5% at level 3, with near-drop +45%, off-screen -22.9% and turned away -2.6% at level 1. All
250 packet differences under cuts are top bytes the GPU ignores.

**Composing** (spec 19) splits by magnitude with the element's sign, high part first, on IR. Results are the same. Each
negative element costs 5 more instructions and each negative high-part sum 3 more.

**gteMIMefunc** uses GPF, not GPL. **GsLinkObject4**: a new run by mode costs 3 instructions less than one by flag alone,
n_prim of 0 writes 0 to the first header's low half, and v0 comes back 0. **FntFlush's "queue path"** was the clean
library's own wait, which Sony's never makes. **printf** takes 570 cycles for a byte at or above 80h.

**FntPrint's unknown letters** (spec 17: `%u`, `%o`, `%ld`, `%-5d`): time and flush on Sony's vary with the argument, 278,
654, 4,428 and 17,664 cycles for 0, 12, 123 and 1234, as if the word were taken as a width. Not changed.

**Not done.** Cut polygons stay in Go, about 16KB of assembly. So do printf and sprintf conversions (`%d` about +480 and
+180), GsSortFixBg16, sin and cos, and memmove. FntFlush with text is +36 to +40. Every entry costs the image stub's `j`
and `nop`, 4 cycles, so rand, TransMatrix, bcopy, memcpy, memset and bzero stay +2 and TransposeMatrix and GsGetActiveBuff +4.

**Not explained.** The kernel-side interrupt entry differs per interrupt, +408, +206 and -1,158 in different programs.
`scee-topten-adven`'s 12-frame gap at its frame 694. RotMatrixY is +4 at some angles.

**A trap in the disc layout.** Linked-in tunnel and scene first failed because a larger MAIN.EXE moved their files on disc,
which shifted when a sound tick fell. `build.sh`'s padding only protects builds smaller than the reference.

Not verified: FntPrint's cut and refused paths, W or H of 0 on the sprite quad, MulMatrix's own entry, twisted GsSetRefView2
gaps, fog and off rows beyond those named, anything under cuts past level 3, and the effect of `dFloat` on unpaced math.
