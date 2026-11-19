# A drawing request that finds the GPU still drawing

## Why

`showcase-gasgar` still flashes on a console under the clean library after spec 39, and runs smoothly under Sony's. Spec 39's change matches Sony's on the console (`imgq`) and didn't help, so it wasn't the cause.

Recording every GPU word gasgar sends in the emulator, for 200 frames on each library, shows the same words in the same order at nearly the same times. With the emulator's drawing time on (`Machine.SetDrawTime`, spec 39), the libraries part ways at the start of each frame. gasgar calls `ResetGraph(1)` at the top of every frame and then `GsDrawOt`, with the GPU still drawing:

- On Sony's library `GsDrawOt` waits inside the call, reading GPUSTAT, until the GPU is ready for a command, and then starts channel 2 on the list itself before returning.
- On the clean one `GsDrawOt` returns at once and leaves the list in the queue. Channel 2 has already finished, so no completion interrupt is coming to start it. The list waits for whatever library call or interrupt next moves the queue on.

gasgar shows one buffer (`GsDefDispBuff(0, 0, 0, 0)`) and `ResetGraph(1)` stops whatever is still drawing. If the frame starts late it can still be drawing when the next frame's `ResetGraph(1)` cuts it off, and what's on the screen is a cleared or half-drawn frame. The emulator's GPU is always ready unless the switch is on, so the emulator never takes this path. This is the most likely cause of the flashing, not a proven one.

## What the probe shows

`probes/drawwait` draws a frame of four 320 by 240 semitransparent boxes (about 7.4 ms on a console), and then a second `GsDrawOt` of the same list in two ways:

- **tail**: once the first list's DMA has ended, with the GPU still drawing the last box.
- **reset**: right after `ResetGraph(1)`, which stops the first list's DMA with the GPU partway through a box.

It prints how long the second call took, GPUSTAT bit 26 and channel 2's busy bit as it returned, `DrawSync(1)`, when channel 2 started if it hadn't by the return, and when everything was done. Times are in eighths of the system clock, from timer 2.

In the emulator with the drawing time on (`PROBE_DRAWTIME=1 PROBE_FRAMES=2000 exp/cleanlib/image/probe.sh drawwait DIR`):

| | Sony's | clean |
|---|---|---|
| tail: the second call took | 9,617 to 11,506 | 41 |
| tail: channel 2 when it returned | busy | idle, not started 60,000 later in 2 of 3 runs |
| tail: everything done at | 76,852 to 76,863 | 108,095 to 127,330 |
| reset: the second call took | 38,386 to 40,275 | 41 |
| reset: channel 2 when it returned | busy | idle |
| reset: everything done at | 76,852 to 76,862 | 98,131 to 98,560 |
| `DrawSync(1)` after the second call | 1 | 2 (tail), 1 (reset) |

The emulator's draw-time model is rough (a cycle per pixel, and `ResetGraph(1)` doesn't end the drawing in progress), so its times are not the console's. The shape is what counts. A console run of the probe is still to come, and will give the real times.

With the drawing time off, both libraries print the same lines.

## What to do

- A drawing request that finds the queue empty and channel 2 idle, but the GPU not yet ready for a command (GPUSTAT bit 26 clear), waits in the call until the GPU is ready and then starts channel 2 on it before returning, as when the GPU was free to begin with. This covers everything that goes through the queue (`GsDrawOt`, `DrawOTag`, `PutDrawEnv`, `FntFlush`, `ClearImage`, `MoveImage` and the rest), not just `GsDrawOt`.
- A request that finds something queued or channel 2 busy goes into the queue as now. The interrupt moves it on.
- When the GPU is free, nothing may change: the same GPU words in the same order, the same times and the same registers left. Every program in the emulator with the switch off takes only that path.
- The image calls (spec 39) keep what they do now. A `LoadImage` or `StoreImage` that finds channel 2 idle and the GPU still drawing already waits for it.
- `DrawSync(1)` after such a call gives 1, with the request running.

## Testing it

With the emulator's drawing time on, `probes/drawwait` on the clean library must print what Sony's does: the second call waits, channel 2 is busy when it returns, and everything is done at Sony's times, within a few hundred eighths. Add a test that runs it with the switch on and checks those, as `TestQueuedImages` does for `imgq`.

Before and after, run the verify chain (copy `.build/verify49/chain.sh` to `.build/verify50`, baseline `.build/carchive/summary-verify49.txt`), the image regression (`image/regress.sh`), `regress.sh`, `terra.sh` and the usual tests. Nothing may get worse.

## Results

Done 2026-11-08 in the emulator. Not tried on a console.

### What changed

- `enqueue` (`gpu.go`) now reads `if queueFree() || drawWait() { drawNext() }`. `drawWait`, not inlined, runs only when the GPU isn't free: if channel 2 is idle and the new request is the only one not yet started (nothing queued, or only a finished list still counted as running, the GPU drawing its last), it spins on GPUSTAT bit 26 and reports true, and `drawNext` retires the finished list and starts the new one before the call returns. Otherwise the request stays queued as before. It covers everything through `enqueue`: `GsDrawOt`, `DrawOTag`, `PutDrawEnv`, `FntFlush`, the kanji flush, `ClearImage` and `MoveImage`. The image calls are as they were.
- `probes/drawwait/main.go` had no `//go:build yaroze` line, so `go test ./exp/cleanlib/...` failed to build it. It has one now, like the other probes.
- `TestDrawWait` (`image/probetest/drawwait_test.go`) runs the probe with the switch on (`-drawwait` names the disc) and checks, for each of the six lines, that the second call waits (5,000 to 20,000 for tail, 30,000 to 45,000 for reset), channel 2 is busy on return, `DrawSync(1)` gives 1, and everything is done between 76,500 and 77,200. It passes on the new image and fails on the one before.

### drawwait, switch on

Eighths of the system clock. Sony's image is unchanged, so its column is the run in the table above.

| | Sony's | clean before | clean now |
|---|---|---|---|
| tail: the second call took | 9,617, 9,617, 11,506 | 41 | 9,633, 9,633, 11,518 |
| tail: channel 2 when it returned | busy | idle, started at 67,798 in 1 of 3 runs | busy |
| tail: everything done at | 76,852 to 76,863 | 108,095 to 127,330 | 76,865 to 76,866 |
| reset: the second call took | 40,275, 38,386, 38,386 | 41 | 40,291, 38,406, 38,406 |
| reset: channel 2 when it returned | busy | idle | busy |
| reset: everything done at | 76,852 to 76,862 | 98,131 to 98,560 | 76,864 to 76,866 |
| `DrawSync(1)` after the second call | 1 | 2 (tail), 1 (reset) | 1 |

The clean calls take 12 to 20 eighths longer than Sony's, and finish within 14 of Sony's. The first call starts at the same time as before (28,835 tail, 62 reset, against Sony's 28,844 and 74), so that small gap was there before this change.

### The free path

Unchanged in the emulator. The first form (`else drawWait()`) cost the free path a jump and its delay slot after the call's last I/O, 4 cycles on every call through the queue. A form that returned through `drawWait` did the same. Making the wait part of the condition gives the free path the same instruction count as before: the branch that skipped `drawNext` now jumps to it instead. The `enqueueNops` pad is unchanged at 13.

Checked with the switch off on 16 probes (`drawq`, `clearimage`, `storeimage`, `loadleft`, `imgq`, `kanjistream`, `kanjipast`, `kanji`, `fntchars`, `drawwait`, `gscycle`, `dispswap`, `flush`, `fntlayout`, `linestart`, `resetret`). Every I/O read and write, with its cycle, from "Execute !" to the end (`PROBE_BUS`) is the same on the new image as on the image before (`.build/verify50/libps-head.exe`). The only exception is the DMA address written to D2_MADR, since the image's data moved by 80h. Every TTY is the same, except `fntlayout`, which prints packet addresses inside the image. With those masked it is the same too. `enqueue`'s code differs from the one before only in register numbers and in those two branches.

One register differs: `enqueue` now returns with 1F801074h in v1 where it had the queue's tail count. `FntFlush` returns straight after `enqueue`, so it now leaves 1F801074h in v1. That matches what Sony's image calls leave in v1 (spec 37). What Sony's `FntFlush` leaves wasn't measured. The other callers set v1 themselves or call on afterward. `loadleft` is the same on both images.

The static build has no pads, so a jump on the free path shows there. The first form moved `TestOffscrScreenshot`, whose screen prints a count of horizontal syncs, from 39 to 40. The final form leaves it at 39, and the test passes.

### Verify chain

`.build/verify50/chain.sh` against `summary-verify49.txt`:

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify49 | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |
| now | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |

No archive line changed.

- `image/regress.sh`: 36 PASS, 5 TIME, the same programs as verify49.
- `regress.sh`: 33 PASS, 8 TIME, the same programs.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- The chain's `go test` list: no failures, `TestYarozeLoadAndRun/ours` included. `./exp/cleanlib/...` passes.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- The image ends at 80066598h.

### Not checked

- A console run of `drawwait` and of gasgar, which will tell whether this was the flashing.
- `ClearImage` and `MoveImage` that wait this way now report that they started at once, so `ClearImage` clips its rectangle as on the free path (spec 21). What Sony's does in that case wasn't measured.
- A request that finds channel 2 idle with a request queued but not started still waits for the next interrupt, as before. The spec leaves that case to the interrupt.

### On a console, 2026-11-08

`drawwait` (`.build/hardware/drawwait-{sony,clean}`) gives the same on both libraries, to a few eighths:

| | Sony's | clean |
|---|---|---|
| tail: second call at, took | 23,345 to 23,694, 6,999 to 7,802 | 23,344 to 23,701, 6,993 to 7,802 |
| tail: all done at | 61,703 to 62,187 | 61,699 to 62,160 |
| reset: GPU ready when the second call came | yes | yes |
| reset: second call took | 83 to 84 | 84 to 85 |
| reset: all done at | 30,747 to 31,229 | 30,753 to 31,220 |

On a console `ResetGraph(1)` leaves the GPU ready at once: it stops the drawing, which the emulator's draw-time model does not. So gasgar's `GsDrawOt` after its `ResetGraph(1)` never takes the path this spec changed, and gasgar still flashes on the clean library as before. The change stays, since it matches Sony's in the tail case.

Also measured on the console, ruling out other suspects:

- `vsynclate` on both libraries: every `VSync` case, interlaced too, ends at the same blank and takes the same lines to within one.
- `probes/callcost`, the calls gasgar spends its frame in (`GsSortObject4` on 200 polygons of seven kinds, `GsSortSprite`, the matrix calls, `GsSortClear`): clean costs what Sony's does to within one eighth on every call. Both are 10 to 25% slower than in the emulator for `GsSortObject4` (for example lg4 7,261 against 5,738).
- Sony's `ResetGraph(1)` empties its drawing queue too, so a queued image is dropped on both.

What the emulator does not model: with the interlaced display and drawing to the displayed field off (E1h bit 10 clear, which gasgar's words have), a console's GPU draws only the lines of the field not shown. The emulator draws every line, so a difference in which field gasgar draws into would not show there.
