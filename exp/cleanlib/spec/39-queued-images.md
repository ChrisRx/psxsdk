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

Done 2026-11-08, with one requirement not met: the calls' times on the free path are the same in total but not inside, and that moved one test. Not tried on a console.

### What changed

- `LoadImage` and `StoreImage` (`loadImage` in `gpu.go`, `StoreImage` in `extra.go`) go into the queue when `imageEnter` finds the GPU busy and the drawing's end will start them: channel 2 still sending, or a request waiting (`imageMayQueue`). The call returns at once, with the pace it always had. A queued image is a request with `restore` imageLoad or imageStore, holding the rectangle, the pixels and an empty packet list.
- While an image waits, channel 2's completion interrupts the CPU (DICR bit 18), as Sony's handler is seen to. `drawDone`, from that interrupt or any other, waits for GPUSTAT bit 26 if the channel has finished and an image is next (`imageTurn`), then `startHead` sends it (`startImage`): the CPU words then the blocks, as on the free path, then waits for the blocks and starts the empty list, which moves the queue on as any request does. `StoreImage` writes the caller's buffer then. `DrawSync(0)` waits for all of it.
- Not queued, and waiting as before: an image whose pixels are on the stack or in the scratchpad (`onStack`), since they would be gone by its turn. The font's palette and the kanji's glyphs are there, so `FntLoad`, `KanjiFntOpen` and the kanji flush still wait. Also a full queue, and a channel already finished with nothing waiting, since no completion would come.
- `ResetGraph(1)` drops queued images with the rest. The interrupt then stays enabled until the next image starts, acknowledged in `drawDone`.
- Emulator: `Machine.SetDrawTime` (`emu/gpu_time.go`), off by default. With it on, each drawn pixel costs a cycle, GPUSTAT bit 26 is clear while the GPU draws, a packet list's DMA is held while the GPU is busy, and the DMA ends when the list's last command reaches the GPU. `PROBE_DRAWTIME=1` turns it on in `probetest`'s `TestDump`. `TestDrawTime` in `emu` tests it.

### The test

`TestQueuedImages` (`image/probetest/imgq_test.go`) runs `probes/imgq` with the switch on (`-imgq` names the disc). Eighths of the system clock, as in the console table:

| | Sony's image | clean before | clean now |
|---|---|---|---|
| the frame alone, to `DrawSync(0)` | 38,451 | 38,447 | 38,450 |
| `LoadImage` just after `GsDrawOt` | 96 | 36,581 to 38,463 | 115 |
| `StoreImage` just after `GsDrawOt` | 94 | 38,471 | 122 |
| `DrawSync(1)` before, then after | 1, then 1 | 1, then 0 | 1, then 2 |
| `StoreImage`'s buffer when it returns | 128 of 128 untouched | written | 128 of 128 untouched |
| the image under the boxes, and what `StoreImage` read | 128 of 128 | 128 of 128 | 128 of 128 |

So the emulator with the switch gives both console columns from both images, and the clean one now orders as Sony's does. No A0h or C0h word reaches the GPU while it draws. The frame is 38,400 here against the console's 31,087: a cycle a pixel is rough. Traced cycle by cycle in `watch`, the drawing's DMA ends at 3/4 of the drawing, the CPU goes into the interrupt there, waits, and sends the image as the GPU frees, as on the console. The probe's own timer readings in `watch` put that gap about 9,600 eighths early in the emulator, on either image. Not looked into.

### What did not come out as asked

- **`DrawSync(1)` gives 2** with a frame drawing and an image behind it, where Sony's gives 1: it counts the image. Leaving images out of the count is a small change in `DrawSync`, but every form of it that was tried moved the compiled `DrawSync(0)` path by 2 cycles, so it was left out. The test expects 2.
- **The free path's times inside the calls moved.** The queue check needs a branch in `loadImage`, `StoreImage` and `startHead`, and the compiler laid the functions out again around it. The totals were put back with the pads (`loadImagePad` 158 to 134, the block, no-block and no-size stores, `storeImagePad` 242 to 230, `clearImagePad` 50 to 26 with `clearImageSpace`). `FntLoad` and `KanjiFntOpen` no longer inline `loadImage`, which cost them 226 and 146 cycles, put back in their waits (`fntLoadImageTime`, `kanjiFntOpenPad` and its nops, a refused open apart). Inside the calls, the first GPU word of `LoadImage` comes 12 cycles later, of `StoreImage` 10, `ClearImage`'s GP1 10000003h 12, `FntLoad`'s and `KanjiFntOpen`'s palette up to about 200.
- **That moved one test.** An interrupt landing in a call whose inside moved takes a cycle or two more or less to enter. In `TestYarozeLoadAndRun/ours`, Sony's sample on the clean monitor, `KanjiFntOpen` takes 91,892 cycles against 91,894 and `FntLoad` 4,684 against 4,686, a vertical blank having landed in each. The vertical blank waits after them come out a few cycles off, and 300 frames on the ball is a step further. The golden was left as it was.

### Checked

Every GPU word and its cycle, against the image before (`.build/verify49/libps-base.exe`), on `probes/imgq`, `drawq`, `storeimage`, `clearimage`, `loadleft`, `kanjistream`, `kanjipast`, `kanji` and `fntchars`, and a scratch probe of image sizes 0, negative and over the limits: the same words, every call's time marker to marker the same (`LoadImage` and `StoreImage` at all 17 sizes and the odd ones, `ClearImage` both ways, `MoveImage`, `GsDrawOt`, `DrawSync`, `PutDrawEnv`, `GsSwapDispBuff`, `FntFlush`, `FntLoad`, `KanjiFntOpen` refused or not, the kanji flushes), every TTY the same. The registers the image calls leave (`loadleft`) are the same.

The verify chain, `.build/verify49/chain.sh`, against `summary-verify48.txt`:

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify48 | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |
| now | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |

No archive line changed.

- `exp/cleanlib/image/regress.sh`: 36 PASS, 5 TIME, the same programs as verify48.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS, 8 TIME, the same programs.
- `exp/cleanlib/image/terra.sh`: all pass, 150 of 150 pictures, 1970 and 2115 changes on each.
- `go test` of `./tests`, `./internal/...`, `./emu/...`, `./serial/...`, `./format/...` and the rest of the chain's list: one failure, `TestYarozeLoadAndRun/ours`, above. `./exp/cleanlib/...` passes.
- The probes `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- The image ends at 80066518h. `link/libps.exe.gz` and `cmd/yaroze/wasm/sysroot.tar.gz` are rebuilt by the chain.

### Not checked

- A console run of gasgar or `probes/imgq`, which is what tells whether this was the flashing.
- Whether Sony's drops a queued image at `ResetGraph(1)`.
- `StoreImage` and `LoadImage` queued behind a request that is itself waiting: the image goes at a later interrupt, the vertical blank's at worst. Only an image behind a running frame was tried.
- The time a queued call takes to return on Sony's console (155 and 150 eighths) is not matched: the clean one returns with the free path's pace.
