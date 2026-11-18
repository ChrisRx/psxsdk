# The fifth run after a library load hangs

## Why

On a console, a program run again and again on one load of the clean library hangs on its fifth run, in its first `VSync(0)`. Under Sony's library the same program runs on. Net Yaroze programs were made to be run again: Sony's samples end with `ResetGraph(3)` and return to the monitor, and the next build is uploaded and run without a reset. Under the clean library the fifth run takes the reset button.

## What the probes show

`probes/rerun` is `probes/buildup` with a tenth of its frames (`stageFrames` 60, about six seconds a run) and `ResetGraph(3)` before it returns. `hwrun` runs it again and again on one load of the library. It sets up 640 by 480 interlaced (`ResetGraph(0)`, `GsInitGraph`, `GsDefDispBuff`, `GsInit3D`, `FntLoad`, `FntOpen`, a `LoadImage` and `DrawSync(0)`), installs a `VSyncCallback`, and each frame calls `VSync(0)`, `ResetGraph(1)`, `GsDrawOt`, `LoadImage` and `FntFlush(-1)`. At the end it calls `VSyncCallback(0)` and `ResetGraph(3)`, and returns.

| library | monitor | ends with | runs to END | then |
|---|---|---|---|---|
| Sony's | Sony's | return | 4 | hangs on run 5 |
| Sony's | Sony's | `ResetGraph(3)`, return | 7 of 7 | |
| Sony's | clean | `ResetGraph(3)`, return | 7 of 7 | |
| clean | clean | return | 4 | hangs on run 5 |
| clean | clean | `ResetGraph(3)`, return | 4 | hangs on run 5 |

Sony's library under the clean monitor (`hwrun -library clean -image` Sony's image) runs on as under its own, so the monitor is not what differs.

The full-length `buildup` (no `ResetGraph(3)`) hangs on the fifth run as well, on both libraries, so it is the count of runs, not the time: blank 14,490 at full length, 1,535 at a tenth. After an upload of the library the count starts again (runs 6 to 9 of a loop reached END, run 10 hung).

In the hang, the program has printed its setup and frame 0, and is in its first `VSync(0)`. The `VSyncCallback` counts blanks in the scratchpad: one blank is taken after setup, and then none, ever. At that one blank nothing is pending: GPUSTAT 564E0000, D2_CHCR 00000201, DICR 00940000, I_STAT 0, I_MASK 0Dh. The serial port's break sometimes brings the monitor back and sometimes not.

The pad driver's banner (`PS-X Control PAD Driver Ver 3.0`, `addr=...`) prints on the first run after a load and on no later one, on both libraries.

## What to do

- Find what a program's run takes that is not given back when it returns, so that by the fifth run something runs out (an event, a handler slot, a callback chain, a kernel table entry, anything the library sets up at its start or in `ResetGraph`, `VSyncCallback`, `GsInitGraph`, the pad driver, `FntOpen`), and what Sony's `ResetGraph(3)` puts back that the clean one does not. Measure Sony's side with probes: what each run leaves in the kernel's tables and the library's state, with and without `ResetGraph(3)`.
- Make the clean library do what Sony's does, so that a program ending with `ResetGraph(3)` can be run any number of times, and one that doesn't hangs where Sony's would, if a probe can tell.
- Try to make it happen in the emulator: several programs, or one several times, on one load of the library, as `hwrun` does on the console. If the emulator or the probe harness can't run programs in a row without a new boot, say what it would need, and the spec side will look at adding it.

## Testing it

The spec side runs the short `buildup` with `ResetGraph(3)` seven times in a row on the console on the clean library, and without it on both. In the emulator: a test that runs a program five or more times on one load, if one can be made; the verify chain against a new baseline (`.build/verify55/chain.sh` copied to `.build/verify56`, run before the change); `image/regress.sh`, `regress.sh`, `terra.sh` and the usual tests.

## Results

Done 2026-11-10 in the emulator. Not tried on a console. The clean library's hang is found, reproduced in the emulator and fixed. It has nothing to do with `ResetGraph(3)`. Sony's library's hang without `ResetGraph(3)` was not reproduced and is not explained.

### What each run took

The clean library's `FntLoad` took 32 bytes of the image's Go heap for the font's palette every call, and never gave them back. `loadImage` keeps the address of an image it leaves queued behind a drawing (spec 39), so the palette, a local array, went to the heap. The image's heap start is set in one place: the image's init routine (80010030h), which a disc's executable calls from the stub `link.PatchExecutable` puts before the program. On a console that has the clean library put in place through the monitor, as `hwrun` does, nothing calls the init routine: the take-over's loader jumps to the monitor, and programs uploaded through the monitor start at their own entry. So the heap pointer is the 0 the image was loaded with, and each run's `FntLoad` took the next 32 bytes from address 0 up, cleared them and wrote the palette there, FFFF0000h then seven 0s:

| run | palette at | what is there |
|---|---|---|
| 1 | 00h | no vector |
| 2 | 20h | no vector |
| 3 | 40h | the debug exception vector |
| 4 | 60h | no vector |
| 5 | 80h | the exception vector |
| 6 | A0h | the kernel's A-function stub |

The fifth run's `FntLoad`, in its setup, writes over the exception vector, and the next interrupt goes to FFFF0000h, a reserved instruction, whose exception goes to the same place. The count starts again at an upload of the library, which loads the heap pointer's 0 again. `ResetGraph(3)` doesn't touch it, so with or without it the clean library hangs on the fifth run, as the console showed.

`runtime.alloc` was called from one place in the whole image, this one. The init routine's runtime start does nothing else: in the image, `libps_image_start` is one store of the heap's start (`initRand` and `initAll` compile to nothing).

On a console the hang came after one more vertical blank. In the emulator the CPU is at 80000080h from the first interrupt after the palette is written. A likely reason for the difference is the instruction cache: the vector's lines, fetched at every interrupt, can stay cached for a while after the data writes under them. Not measured.

### The emulator

`TestRerun` (new, `image/probetest/rerun_test.go`) runs a probe's program again and again on one boot. When the program returns to what started it, its memory goes back to what was loaded, the registers to what it was entered with, and it is entered again, at the disc's stub (the init routine every run), at the ELF's own entry after the first run (`PROBE_ELFENTRY=1`, the init routine once), or at the ELF's entry from the first run (`PROBE_ELFENTRY=all`, the init routine never, as on the console). `PROBE_SR=401` turns the interrupts on at each entry, as the monitor's start-up has when it starts a program. After each run it writes the kernel's events in use, its four interrupt chains, some device registers and the words at 100h to 15Fh, and with `PROBE_KRAM` the RAM below the program (kernel and library) for diffs. `PROBE_CALLS` logs the kernel calls at each run's start and end.

With the scph1001 BIOS:

| library | init routine | runs to END | then |
|---|---|---|---|
| Sony's, `rerun` | every run | 1 | run 2 hangs in the init routine's drive reset: Sony's init routine starts the pads, drive, sound and card driver again, opening 9 more events, the last two failing |
| Sony's, `rerun` | first run | 7 of 7 | |
| Sony's, `rerunbare` | first run | 7 of 7 | |
| clean, `rerun` | every run | 6 of 6 | |
| clean, `rerun` | first run | 7 of 7 | |
| clean before, `rerun` | never | 4 | run 5 hangs in its first `VSync(0)`, PC at 80000080h |
| clean now, `rerun` | never | 7 of 7 | |
| clean now, `rerunbare` | never | 7 of 7 | |

`probes/rerunbare` (new) is `rerun` without the `ResetGraph(3)`.

The "never" row is the console's case, and it hangs where the console did: run 5 prints `setup done`, `STAGE 0` and `frame 0 vsync`, and does not return from the first `VSync(0)`. The RAM below 100h after four runs holds the four palettes, at 00h, 20h, 40h and 60h.

In the other rows, on both libraries, the kernel's state doesn't build up from run to run: the same 9 events (the sound system's and the card driver's 8), the same handlers in the same chains, the same device registers, and the same kernel RAM. Below the program, from the second run on, Sony's library changes five words a run, which look like counts, with and without `ResetGraph(3)`. The clean library changes its counters (the vertical blank count, the line counter at `VSync`, the font's and the queue's state) and its heap: with the init routine run, the heap pointer moves on 32 bytes a run, inside the image's 32 KB heap, and the palette lands there. That leak was harmless there for a while, but a program calling `FntLoad` 1,024 times on one load would have run the heap out.

`TestFifthRun` (new) is the console's case as a test: `rerun` seven times with the init routine never run, every run returning and printing END, and the RAM below 100h after each run the same as after the first. Build the disc with `SIDES=clean exp/cleanlib/image/probe.sh rerun exp/cleanlib/spec/probes/rerun` and pass `-rerun=$PWD/.build/rerun-cimg-clean/rerun-cimg-clean.cue`; it takes two minutes. On the image before the change it fails: 4 runs, and the RAM below 100h differs after each run. On the new one it passes. `TestImageNoHeap` (new) checks that the image make.sh last built has no `runtime.alloc`.

A hung run whose CPU loops through the exception vector stops the emulator's frame count, so `TestRerun` also ends a run after as many instructions as its frames could hold.

### What changed

In `font.go`, the palette is a package variable, `fontClut`, initialized in the image's data, and `FntLoad` sends it from there. `FntLoad` spends the time the allocation took, 388 cycles, in its spin and two more nops, so that it takes as long as before: 4,694 cycles from its entry to its return at `rerun`'s setup, on the image before and the new one. The image has no heap allocation left and ends at 800673A8h, 400 bytes shorter.

### Sony's ResetGraph(3)

Nothing in the emulator shows what Sony's `ResetGraph(3)` gives back. Run after run, without it as with it, Sony's library leaves the kernel's tables, the device registers read and the RAM below the program as the run before did, with the init routine run once as the boot disc runs it. At the end of a run its `ResetGraph(3)` makes no kernel call but its line's printing and the A0h 49h (`GPU_cw`) that every `ResetGraph` makes, as the clean one does: no event closed, no handler taken out (`PROBE_CALLS=0`). So the console's fifth-run hang on Sony's library without `ResetGraph(3)` comes from something the emulator run doesn't have: the boot disc's monitor between runs, or something of the console's. The console table has Sony's library under the clean monitor only with `ResetGraph(3)`. Running `rerunbare` on Sony's library under the clean monitor would tell whether the boot disc's monitor is needed for it.

The clean library now runs `rerunbare` any number of times, where Sony's hangs on a console. It was not made to hang the same way: no probe here can tell what Sony's does.

What the emulator would need to show it: running programs in a row through a monitor over the emulated serial port, the boot disc's or the clean one, uploading and starting each as `hwrun` does, so that whatever the monitor does between runs happens too. `TestRerun` puts the program's memory and registers back itself and runs no monitor.

### Found in passing, not changed

- After a run, Sony's library leaves DPCR at 3B3B3B33h and D2_CHCR at 00000401h, the clean one 08089899h and 0, with or without `ResetGraph(3)` on Sony's side. Not looked into.
- On the console the clean library's init routine never runs, so its `Startup` (spec 15) doesn't either: the library starts at the program's first `ResetGraph`, as the linked-in target does. Nothing found depends on it.
- `FntLoad` takes 4,694 cycles on the clean image and 4,862 on Sony's at `rerun`'s setup. Not changed here.

### Verify chain

`.build/verify56/chain.sh` (verify55's, renamed) run on the working tree before the change as the baseline, and `.build/verify57/chain.sh` after it:

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify56 (before) | 94 | 20 | 14 | 8 | 1 | 1 | 1 | 1 |
| verify57 (now) | 94 | 20 | 14 | 8 | 1 | 1 | 1 | 1 |

No archive line changed, from verify55 to verify56 or from verify56 to verify57.

- `image/regress.sh`: 36 PASS, 5 TIME, the same programs as verify56.
- `regress.sh`: 33 PASS, 8 TIME, the same programs.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- The chain's `go test` list: no failures. `go test ./exp/cleanlib/...` passes.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- `TestFifthRun` passes on the new image and fails on the one before; `TestImageNoHeap` passes.

### What did not come out as the spec says

- **`ResetGraph(3)` was not the difference.** The clean library hung on the fifth run because of where its heap starts on a console, with or without `ResetGraph(3)`. Sony's `ResetGraph(3)` gives back nothing the emulator can see.
- **Sony's hang without `ResetGraph(3)` is not explained,** and the clean library doesn't copy it: `rerunbare` now runs on under the clean library. See above for the run that would narrow it down.
- **The emulator doesn't run a monitor between runs.** `TestRerun` puts the program back itself, with the interrupts on as the monitor leaves them. The console's flow (the library put in place without its init routine, programs entered at their own start-up) is there, and is enough to show the clean library's hang.

### Not checked

- The console runs: `rerun` seven times or more in a row on the clean library, and `rerunbare` on both libraries.
- Whether the console's one more vertical blank comes from the instruction cache.
- On a console, what the init routine's `Startup` does (spec 15) is left to the program's first `ResetGraph`, as before. That was not compared with Sony's there.

### On the console (spec side)

On one upload of the new clean library, `rerun` and then `rerunbare` ran 7 times each in a row, all 14 to END. Before the change both hung on the fifth run. Afterward the exception vector at 80h read as the kernel's jump; FFFF0000h palettes from the runs before the change were still at 20h and 40h, since a reset doesn't clear RAM that low.

Sony's library under the clean monitor (`hwrun -library clean -image` Sony's image) ran `rerunbare` 7 times in a row, all to END. So Sony's library hangs on the fifth run without `ResetGraph(3)` only under the boot disc's monitor: it is something of the boot disc's monitor, or of the two together, and not of Sony's library alone. Nothing for the clean library to match. Not looked into further.
