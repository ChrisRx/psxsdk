# Three programs that go wrong

Three archive programs behave differently on the two libraries, in ways that aren't time. The baseline is `.build/carchive/summary-verify48.txt`.

## `showcase-gasgar`

From its test disc it's `same` for 1,200 frames, 115 picture changes on each side. Loaded through the monitor, as on the real Net Yaroze, it differs. In the emulator, with `TestYarozeArchiveJob` and `PSX_HW_PROGRAM=showcase-gasgar`, Sony's library keeps it running after 20 s. On clean it prints `ResetGraph` a second time, and then the monitor's greeting and prompt, so the program returned or jumped back to the monitor. A real console does the same on clean (spec 35, Results).

The two ways of loading differ in what's in memory around the program: the monitor at 80068000h, its heap below 80090000h, the stack the monitor gives, and the data files written before `go`. Find which of those clean depends on and Sony's doesn't, and why the program leaves.

The rehearsal needs `PSX_HW_ARCHIVE=$PWD/.build/carchive/programs.tsv`, `PSX_SONY_BIOS` and `PSX_YAROZE_BOOT`. Spec 35 has them, and `go test ./tests -run TestYarozeArchiveJob` runs it. A run takes several minutes.

## `pssoft-kyuri-2-kyuri`

`wild` on both sides at 1,200 frames: the PC leaves RAM at 5430efd0h on Sony's and 5430f010h on clean, 40h apart. Find where the two runs first part, and whether the 40h comes from the library.

## `event-event003-theme-krwire`

`stall`: Sony's shows 10 frames and clean 1,200. Sony's stops early and clean doesn't. Find what Sony's stops on and why clean goes past it.

## Work

For each, find the first difference in behavior: a return value, what the hardware is told, or what's drawn. Use `TestDump`, `PROBE_WATCH`, `TestArchiveBus`, `TestArchiveCalls` and `TestArchiveGP0`. Then, as in spec 37: for each cause that's a difference in behavior, write a probe that reproduces it on both libraries with no program involved, calling the library by name, and keep it in `probes/`. Correct the spec and the library for every case the probe covers. Don't fit anything to one program. If what you find is only time, say so and stop on that program.

A program can also be at fault on both sides, as when it reads memory nothing set. Say so when that's what you find, and stop on it.

## Acceptance

Results first, then time by cycles.

- `exp/cleanlib/image/probe.sh` on each new probe: the TTY of the two sides is identical. The existing probes for what you change keep matching.
- `FIRST=1 exp/cleanlib/image/archive.sh` on the three, then the full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify48.txt`: name anything that got worse.
- For gasgar, the rehearsal above on both libraries.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 36 PASS and 5 TIME. `exp/cleanlib/image/terra.sh`: all pass. `go test ./tests` passes.
- The image ends below 80068000h, now 80066288h.

## What the work on this spec found

None of the three is a difference in the library's behavior. Two are the programs' own faults, the same on both libraries, and the third's 44h is time. The library didn't change, no spec's rules changed, and no probe was written, since there was no behavior to reproduce.

### `event-event003-theme-krwire`: the program's fault on both sides

`TestArchiveCalls` gives the same five calls on both sides, with the same arguments: `init`, `InitHeap`, `ExitCriticalSection`, `GetPadBuf`, then `CdSearchFile` at frame 6 for `\DATA\SOUND\STD0.VH;1`, which returns 0 on both. The theme event's programs expect their sound data on the event's own disc. Like the `tm` programs in spec 37, it isn't in the archive copy, and the test disc doesn't have it. The search takes to frame 10 on both sides.

Then the program jumps into the BIOS at BFC03D60h, on both sides, clean 710 cycles later. That routine loads ra, sp, fp and gp from memory nothing set: all 0. It returns to address 0, where the kernel's first words add C80h to k0 and jump there. k0 is what the last interrupt left, an address in each library's own code: 8003C0B8h on Sony's, 8004104Ch on clean. So Sony's goes on at 8003CD38h and clean's at 80041CCCh, each in the middle of its own library with sp 0.

From there each runs wild in its own way. Sony's ends at 80000080h with sp D0h, where the emulator stops advancing: the cycle count stays at 615,801,934 and no frame completes, which is what `stall` counts as 10 frames. Clean's goes through low memory and keeps taking interrupts, so frames go on to 1,200. Where each lands depends only on where the last interrupt struck in each library, which is layout and time. That the emulator makes no progress at 80000080h wasn't looked into.

### `pssoft-kyuri-2-kyuri`: the program's exit, then time

The 17 calls agree in entry and argument on both sides, and so do the results of all that return one. Only the `v0` of void calls (`init`, `InitHeap`, `ExitCriticalSection`, `GetPadBuf`, `GsInitGraph`, `GsDefDispBuff`, `GsSetClip2D`, `GsSetDrawBuffClip`) and the registers left differ, and the program doesn't use them. It searches for a file on the Net Yaroze CD, which the test disc isn't, prints "Please Insert NetYAROZE CD-ROM!" and "Thank you for your playing!", calls `VSync(0)` at frame 10 and exits. The exit goes to the same BIOS routine as krwire's, which loads ra = 54202020h, text from the printed lines ("   T"), the same on both sides. The PC then runs on through unmapped memory from 54202020h at 2 cycles an instruction.

The two runs never part in behavior. They part in time at the first call. Clean is 126 cycles later at `InitHeap` and 518 later at `VSync(0)`. `VSync(0)` returns after the frame 11 blank, which both see at cycle 627,304,212. From there to the return, Sony's takes 15,176 cycles and clean 15,142. So clean reaches BFC03D60h 34 cycles sooner, 17 instructions, and at the frame where `TestArchive` samples the PC it is 44h further on: 5430F014h against 5430EFD0h. That is the whole gap, and it comes from the library's time from the blank's interrupt to `VSync`'s return. Spec 37's 40h was the same thing at 32 cycles. Under spec 34 and spec 35 that waits for hardware data and wasn't tuned.

### `showcase-gasgar`: the program reads its stack before it sets it

What the console showed (the coordinator's observation, the clean library loaded through the clean monitor): the TV stayed black the whole time, and the program drew no picture before it went back to the monitor. It printed the pad driver line, then `ResetGraph` twice, then the monitor's greeting, as in the emulator. So it leaves during its start-up, before its first displayed frame.

`TestYarozeArchiveJob` with `PSX_HW_PROGRAM=showcase-gasgar` reproduces both sides as spec 35 says. Sony's prints `ResetGraph` once and is still running after 20 s. Clean prints the pad driver line, `ResetGraph` twice and the greeting, with the PC at 4480h, in the kernel, under the monitor's `>>`.

A trace of the program's library calls through the monitor, Sony's against clean's, with the same TSV upload:

- Clean's calls are the disc run's, up to `SsSeqPlay`, `SsUtSetReverbType`, `SsUtSetReverbDepth`, `SsUtReverbOn` and the first `SsSeqSetAccelerando`, its first pass through the main loop. Then, within 32 cycles, it calls `SsSeqStop`, `SsVabClose` twice, `SsUtAllKeyOff`, `ResetGraph(3)` (the second `ResetGraph` line) and `EnterCriticalSection`, and returns to the monitor. That is the program's own exit path. Sony's goes on to a second `SsSeqSetAccelerando` and draws.
- Through Sony's monitor the program takes the same exit, but only at frame 121, right after the test's first press of Start and Cross. So it's the program's quit, which on clean it takes with no press.
- The pad buffers read the same on both sides at every call checked (`ffff4100 ffffffff`, no buttons), so it isn't the pads.
- Clean's `GsInitGraph` there takes 76 frames, where Sony's takes part of one. `serial.TakeOver`'s loader jumps to the clean monitor without running the image's `init`, so the library's start-up waits for the program's first `ResetGraph`, as in a program built for the clean library alone. The card driver's `_bu_init` reads the boot card's directory there, two frames a sector. Sony's library ran its start-up at boot, before Sony's monitor. This isn't why the program leaves: with `init` run through the monitor (`epc` 80010030h, `go`) before the upload, the start-up comes before the program, `GsInitGraph` is quick, and the program still leaves on its first pass. Whether `TakeOver` should run `init` is for the serial tools, not this spec.
- Not the heap: `InitHeap` gets a negative size on all three runs, from the program's own sp less 10000h (801EF000h on the disc, 801EFFF0h through the clean monitor, 801EFF28h through Sony's), and nothing allocates.
- Not the bss: SIOCONS-style uploads don't clear it, and through the clean monitor 7,165 of its 13,251 words hold what `TakeOver` copied below 801F0000h, where Sony's are all 0. Uploading zeros over 801E7000h to 801F3F0Ch changes nothing: it still leaves at the same call.
- **The stack.** The host sets sp to 801FFFF0h on both. Sony's monitor starts the program with sp at 801FFF28h, as its first calls show, and clean's at 801FFFF0h, as asked. The clean monitor runs with its own stack in the same 801FFFxxh area (its kernel calls show sp 801FFF30h to 801FFFD8h), so the program's first frames sit on what the monitor left there. Set to 801FF000h instead, the TSV's upload unchanged, the program runs on clean through the monitor to frame 110, the end of the trace, still in its main loop with no exit. Its pictures weren't looked at. Uploading zeros over 801FE000h to 801FFFF0h, to narrow it down, stops the clean monitor answering, so which word the program reads wasn't found.

So the program reads stack memory it never set, and the quit test depends on what is there. On the disc and through Sony's monitor that memory is harmless. Through the clean monitor it holds the monitor's own leftovers. The library is the same in all three cases, and on the disc gasgar is `same` on it for 1,200 frames. This is the program's fault, shown up by where the clean monitor puts its stack and the program's, and is the monitor's to work around if anyone wants to: giving the program Sony's sp, or keeping the monitor's stack away from it.

### Other things found, not taken on

- The clean monitor isn't started with the library's start-up, as above, so the first program through it pays for the start-up inside its first `ResetGraph`: 76 frames for `_bu_init` on the emulated boot card. On a console the card is the user's. It also prints the pad driver line in the program's output, which Sony's printed at boot.
- `SsSeqSetAccelerando`'s `v0`, void, is 8004EB34h then 6Eh on Sony's and 8004CEBCh on clean; many void calls' `v0` differ, as before. Nothing seen uses them.
- The emulator stops making progress with the PC at 80000080h in krwire's Sony run (cycle count fixed, no frames).

### Acceptance

- No new probe, so `probe.sh` has nothing to run, and with the library unchanged no existing probe was rerun.
- `FIRST=1 exp/cleanlib/image/archive.sh` on the three, on an image rebuilt with `make.sh` at the start: the same three lines as `summary-verify48.txt`, krwire `stall` 10 against 1,200 frames, kyuri `wild` 5430EFD0h against 5430F014h, gasgar `same` 115/115. `first.txt` finds no different picture in any. The full `archive.sh` wasn't run, since nothing it builds changed.
- The rehearsal for gasgar, `TestYarozeArchiveJob`, on both libraries, before anything else: passes, with the outputs above.
- The image ends at 80066288h.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 36 PASS and 5 TIME (`check`, `dungeon`, `morph`, `offscr`, `terra`), as after spec 37. `exp/cleanlib/image/terra.sh`: all pass.
- `go test ./tests` passes, with `PSX_SONY_BIOS` set and `PSX_BIOS` and `PSX_YAROZE_BOOT` not, in 103 s.
- No sysroot refresh, since the library didn't change.

### Not checked

- Which stack word gasgar reads. Clearing the stack area through the monitor stops the monitor, and RAM reads can't be watched from outside without reading the program.
- Whether gasgar draws correctly through the clean monitor with its stack moved, past frame 110.
- What the BIOS routine at BFC03D60h is by name. Only what it was seen to do is given above.
- Gasgar through the monitor on a console with its stack moved.

### The fix, in the serial tools

The clean monitor now keeps its own stack in its space below 80090000h, and `go` zeroes the 16 KB below the program's stack pointer before starting it. Moving the monitor's stack alone, or starting the program C8h lower as Sony's monitor does, still left gasgar quitting: the leftovers were also the boot disc's monitor's and `TakeOver`'s loader's. With the area zeroed, the rehearsal shows gasgar still running after 20 s, with one `ResetGraph`, as on Sony's.
