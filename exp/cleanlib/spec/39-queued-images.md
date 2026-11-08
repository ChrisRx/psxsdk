# LoadImage and StoreImage behind the drawing

## Why

On a console, `showcase-gasgar` on the clean library flashes so badly it's painful to watch, and on Sony's it runs smoothly (spec 35, 2026-11-08). In the emulator the two draw it the same for 1,200 frames.

gasgar runs 640 by 480 interlaced with one buffer (`GsInitGraph(640, 480, 1, 0)`, `GsDefDispBuff(0, 0, 0, 0)`). Every other frame it calls `GsDrawOt` and then `LoadImage` eight times, for its animated sprites. On Sony's library each of those returns at once. On the clean one the first waits until the whole frame has been drawn, which is the CPU held for the frame's drawing time on just the frames it animates. The clean library says so in `imageEnter` (`gpu.go`): "Sony's library would queue the image behind the drawing; the GPU in the emulator is always free by then, so the clean library waits for the queue instead". That holds only in the emulator, whose GPU takes no time.

This is the most likely cause of the flashing, not a proven one. A console run after the change will tell.

## What a console shows

Measured with `probes/imgq` on a console under each library, through `hwrun`. Its frame is four 320 by 240 semitransparent boxes, which take about 7.4 ms to draw. Times are in eighths of the system clock, from timer 2 in mode 200h, counted from just before `GsDrawOt`.

| | Sony's | clean |
|---|---|---|
| the frame alone, to `DrawSync(0)` returning | 31,087 | 31,088 |
| `LoadImage` (16 by 16) called just after `GsDrawOt` returns after | 155 | 31,145 |
| `StoreImage` (16 by 16) called just after `GsDrawOt` returns after | 150 | 30,688 to 31,169 |
| `DrawSync(1)` before the image call, then after it | 1, then 1 | 1, then 0 |
| `StoreImage`'s buffer when it returns | not yet written (128 of 128 words untouched) | written |
| the image's pixels afterward, under the boxes | the image's, 128 of 128 | the same |
| what `StoreImage` read | the finished picture, 128 of 128 | the same |

So on Sony's, both calls go into the queue behind the drawing and return at once. The transfer happens after the drawing: the image isn't drawn over, and `StoreImage` reads the finished picture. `DrawSync(1)` gives 1 both before and after the call.

**What starts a queued image.** `watch` in the probe sets off the same frame and a 64 by 64 `LoadImage` behind it, then spins for about 31 ms without calling the library, logging GPUSTAT bit 26 (ready for a command), D2_CHCR bit 24 (busy), and any gap in its own sampling, which is the CPU away in an interrupt. On Sony's, in every run:

- 245: the drawing's DMA starts.
- About 22,900: the DMA ends. The GPU paces it, so it takes most of the drawing time, but the GPU is still drawing the last box.
- From that moment the CPU is away for about 8,320 eighths, until about 31,200, when the GPU is ready again. That's the drawing's completion interrupt, which waits there until the GPU can take the image and then sends it. No separate DMA for the image shows after it.
- No call is needed. A `DrawSync(1)`, `VSync(0)`, `GsGetActiveBuff` or `GsClearOt` made later changed nothing, since the image had gone already.

So the CPU is held on Sony's too, but only from the end of the drawing's DMA to the end of the drawing, and in the interrupt, not from the call. With gasgar's frame that's a short wait at the end of the drawing instead of a wait for all of it right after `GsDrawOt`.

Also seen: Sony's vertical blank interrupt takes about 3,460 eighths each time.

The emulator can't show any of this. Its GPU is always ready and its DMA runs a whole list as CHCR is written, so in the emulator a queued image always goes at once.

## What to do

- `LoadImage` and `StoreImage` go into the drawing queue (spec 06) behind whatever is queued or running, and return at once, as `MoveImage` and `ClearImage` already do. When the GPU is free and nothing is ahead of them, which in the emulator is always, nothing about them may change: the same GPU words in the same order, the same times, the same registers left (spec 08 and 37 have the measured shape of each).
- A queued image starts from the drawing's completion interrupt. If the GPU is still drawing then, the handler waits for it, as Sony's does, and then sends the image: the CPU words then the DMA blocks, as `loadImage` does now.
- `DrawSync(1)` gives what the table shows: 1 with a frame drawing and an image queued behind it. `DrawSync(0)` waits for the image too.
- `StoreImage` writes the caller's buffer only when its turn comes, and `DrawSync(0)` returns after it's written.
- What `imageEnter` reports as `idle`, which decides whether the rectangle is clipped (spec 21), must stay what it is now for the case it covers.
- `ResetGraph(1)` and `ResetGraph(3)` empty the queue in the clean library (`gs.go`). Queued images go with it. gasgar calls `ResetGraph(1)` at the start of every frame. Whether Sony's drops a queued image there wasn't measured.

## Testing it

The emulator's GPU never makes an image wait, so the queued path needs a way to make it wait in a test. Give the emulator a switch, off unless a test turns it on, under which the GPU stays busy for a time after drawing: GPUSTAT bit 26 clear, and a list DMA held until the GPU is free. A rough cost is enough, for example a fixed time per drawn pixel. With it on, a test like `probes/imgq` must show the order the console does: the call returns at once, the image goes after the drawing and isn't drawn over, `StoreImage` reads the finished picture, and `DrawSync(1)` gives 1 then 1. With it off, nothing changes for any program.

Before and after, run the verify chain (`.build/verify48/chain.sh`, baseline `.build/carchive/summary-verify48.txt`) and the image regression. Nothing may get worse. The usual tests must pass.

## Results

(To be filled in.)
