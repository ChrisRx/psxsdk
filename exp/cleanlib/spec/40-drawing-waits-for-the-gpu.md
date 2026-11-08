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

(To be filled in.)
