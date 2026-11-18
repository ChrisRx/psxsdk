# Images queued behind a long drawing start late

## Why

`showcase-gasgar` still flashes on a console under the clean library after specs 39 and 40, and runs smoothly under Sony's. Looking closely at a CRT, the flashing covers the whole screen, 2D sprites and text included. Near the middle of the torus, a few of its far-side triangles show through in a band that moves.

gasgar draws to a 640 by 480 interlaced display with one buffer, and E1 bit 10 is clear, so the GPU only writes the lines of the field not being shown. A frame drawn inside one field is never seen half done. A frame whose drawing runs past the field change finishes on the other field's lines, and leaves what it had drawn, the clear included, on the lines then being shown. That would look like the flashing, and like the torus's far side showing where the drawing had got to. The torus is drawn back to front.

In the emulator, gasgar sends the same GPU words on both libraries, with the same E1 values and display settings. So the question is when the drawing finishes, and on a console the clean library finishes later.

## What the probe shows

`probes/fieldtime` copies gasgar's loop: `VSync(0)`, `ResetGraph(1)`, an optional wait of w ms standing in for the game's work, `GsClearOt`, `GsSortClear`, k semitransparent 640 by 480 boxes with `GsSortBoxFill`, `GsDrawOt`, and then n 16 by 16 `LoadImage` calls behind it. It then spins, reading only GPUSTAT and D2_CHCR, until channel 2 is idle and the GPU ready (bits 26 and 28) for 200 reads in a row. It prints, for 60 frames, the mean time from VSync's return to `GsDrawOt`'s return and to that idle point, in microseconds, and the frames that went past a field (16,683 µs).

On a console (`go run ./tools/hwrun -library sony|clean`, 2026-11-09), w = 0:

| k boxes, n images | Sony's: idle at | clean: idle at |
|---|---|---|
| 0, 0 | 700 | 663 |
| 0, 8 | 947 | 913 |
| 0, 32 | 1,688 | 1,633 |
| 1, 0 | 4,464 | 4,404 |
| 1, 8 | 4,704 | 5,127 |
| 1, 32 | 5,400 | 6,159 |

- Behind the clear alone, which the GPU finishes quickly, both libraries take about 30 µs an image, and the clean one is a little ahead.
- Behind one box, which the GPU is still drawing when channel 2 finishes sending the list, Sony's first 8 images add 240 µs and the clean one's add 723 µs. The next 24 add 696 µs on Sony's (29 an image) and 1,032 on the clean (43 an image).
- With no images, the clean library finishes the box first. The difference comes only from the images queued behind it.

With gasgar's 8 images and the other settings (k = 1, 3, 5 and w = 0, 6, 10 ms), the clean library goes idle 420 to 1,140 µs after Sony's every time. In these runs both libraries are late in the same cases, because the boxes either fit in a field easily or miss it by a lot. gasgar's own frame is presumably close enough to the field change on Sony's that half a millisecond decides it.

`GsDrawOt` itself returns sooner on the clean library (66 against 86 µs).

In the emulator with the drawing time on (`PROBE_DRAWTIME=1`), whose drawing is slower than the console's, the same shape shows:

| k boxes, n images | Sony's | clean |
|---|---|---|
| 0, 0 | 9,545 | 9,542 |
| 0, 8 | 10,212 | 10,126 |
| 0, 32 | 12,230 | 11,877 |
| 1, 0 | 18,622 | 18,620 |
| 1, 8 | 18,916 | 19,500 |
| 1, 32 | 19,276 | 5,725 |

Sony's first 8 images behind the box add 294 µs, and the clean one's add 880. The clean library's 5,725 for one box and 32 images is out of line with everything else. Find out what it is: perhaps the frame's drawing went somewhere other than where the probe expects, or the idle test saw a false idle.

Spec 39 measured Sony's handling on the console. When the drawing's DMA completes, its completion interrupt waits there until the GPU is ready and then sends the image. The clean library was made to do the same (`drawDone`, `imageTurn`, `startHead`, `startImage`). Something on that path, or between one queued image and the next, costs the clean library about half a millisecond more than Sony's for the first image, and about 14 µs more for each one after.

## What to do

- Find where the time goes, between channel 2 finishing the drawing's list and the first queued image's words reaching the GPU, and between one queued image finishing and the next starting. Use the emulator with the drawing time on and `probes/fieldtime`, or a probe of your own.
- Make a queued image start as soon as the GPU can take it, as Sony's does, and the ones after it follow as quickly. Sony's figures are the targets: behind one box, 8 images done within about 300 µs of the box alone in the emulator (240 on the console), and about 30 µs an image after that.
- When the GPU is free and nothing is queued, nothing may change: the same GPU words in the same order, the same times, the same registers left. Every program in the emulator with the switch off takes only that path, and the verify chain checks it. Spec 40's agent checked this with `PROBE_BUS` on 16 probes, which is the way to do it here too.
- `ResetGraph(1)` dropping what's queued, `DrawSync`, and the order of everything stay as specs 39 and 40 left them.

## Testing it

With the drawing time on, `probes/fieldtime` on the clean library must give Sony's times for k = 1 within a few tens of microseconds: the 0, 8 and 32 image lines, and the out-of-line 32 image line explained and fixed. Add a test that runs it with the switch on and checks those, as `TestDrawWait` does for `drawwait`. Before writing the test, cut the probe down if it's too slow in the emulator: the k = 3 and 5 and w = 6 and 10 lines mostly wait out the 25 ms cap there.

Before and after, run the verify chain (copy `.build/verify50/chain.sh` to `.build/verify51`, baseline `.build/carchive/summary-verify50.txt`), the image regression (`image/regress.sh`), `regress.sh`, `terra.sh` and the usual tests. Nothing may get worse.

The console run comes after, on the spec side: `fieldtime` on both libraries, and gasgar.

## Results

Done 2026-11-09 in the emulator. Not tried on a console.

### Where the time went

Traced in the emulator with the drawing time on (`PROBE_BUS` on a copy of the probe cut to one 1-box, 8-image frame), the first queued image reaches the GPU within about 100 cycles of each other on both libraries, once the GPU has finished the box. What differs is everything after it.

- **Sony's handler sends the queued images one after another in the same interrupt.** For each: it reads D2_CHCR for the end of the last one's blocks about 124 cycles after starting them, reads GPUSTAT, starts a watch (timer 1), reads GPUSTAT again about 140 cycles later and sends the next. 508 cycles from one 16 by 16 image's blocks to the next's. Before the last image it clears channel 2's interrupt enable in DICR. No empty packet list, no interrupt between images.
- **The clean library took an interrupt for every image.** `startHead` sent the image, then started its empty packet list, whose completion interrupt sent the next. Each image cost the kernel's way out and back in, about 2,100 cycles apart against Sony's 508. That is the 14 µs or so an image.
- **The vertical blank came between the first image and the second.** The GPU was still drawing the box when the field changed, so the vertical blank was raised while the handler waited in `imageTurn`. Sony's handler, after its last image, reads I_STAT again and takes the vertical blank in the same pass, acknowledging it: the kernel's controller handler never sees that one (Sony's trace reads the pads at one vertical blank of the two in each frame, the clean one's at both). The clean handler returned after its one image, the kernel came straight back in for the vertical blank and ran its controller handler first (about 6,600 cycles of SIO reads), then the way in to the library's handler again (about 2,800), and only then did the next image go. That is the half millisecond on the first image. On a console the vertical blank was presumably in the same place, the box taking most of a field there too.
- **Counting the images** (`imagesWaiting`, for whether an image is the last) ran over the whole queue for each image: 22 cycles for each image behind it, about 680 for the first of 32.

**The 5,725 line** is the probe's clock, not the GPU. The queue held 32 requests, the running one among them, so the 32nd `LoadImage` behind the frame found it full and waited in the call, with interrupts on, until everything had gone out, about 19 ms. `clock.now` counts timer 2's 16 bits on only if it is read at least every 15.48 ms (65,536 ticks of the system clock over eight), and the probe doesn't read it inside the call, so one wrap was lost: 5,725 + 15,480 is 21,205. Sony's queue takes more. With a probe of the queue's length (`probes/qlen`: 80 calls of a 16 by 16 `LoadImage` behind a clear and one box, each call's time and `DrawSync(1)`, switch on), Sony's took 63 images behind the drawing and the 64th waited, where the clean one took 31. It also shows Sony's `DrawSync(1)` counting 1 after the first image and one more for each image, where the clean one counts one more than that (spec 39's 1 against 2).

### What changed

In `gpu.go`, plus the interrupt stack in `kernel.go` and the queue clear in `gs.go`:

- `imageTurn`, which runs from `drawDone` only when the GPU isn't free, now, once the GPU is ready, calls `sendImages`. That sends the image next and each image right behind it in a loop, each through `drawNext` as before (the image, then its empty packet list, which `sendImages` waits for, a few cycles), with Sony's pace around it: a watch and 19 stores before `drawNext`, so that the GPUSTAT read before the image comes 140 cycles after the watch as on Sony's, and 48 after, for the 508. Then, in the handler, `lateInterrupts` runs `libps_interrupt` again before returning, which takes the vertical blank and the DMA raised meanwhile and moves the queue on past the last image. `sendImages` reports whether the GPU is then free, for `drawDone`. Outside the handler (from `pollQueue`, on the program's stack), and with the drive's interrupt raised, `lateInterrupts` does nothing and the handler's own pass takes them as before.
- `imageTurn`'s way out when no image is next is the same code as before. With the loop in `imageTurn` itself, the compiler saved seven registers on the way in and restored them on the way out, some 16 instructions more for every interrupt during a drawing's tail, on a console or with the switch on (`drawwait`'s times moved by up to 7 eighths), so the loop is a call of its own after the wait.
- Whether an image is the last one waiting, for clearing channel 2's interrupt enable before it, is now `queue.head == queue.lastImage`, where `queueImage` records the last image queued, in place of `imagesWaiting`'s count over the queue. A dropped image's index is behind `head`, which only grows, so `ResetGraph` needs no change. Only the image branch of `drawNext` changed: its other paths are the same instructions, register numbers apart.
- A first form sent the images with a copy of `startImage` of its own, to keep `drawNext` out of it. In the static build (`exp/cleanlib/build.sh`) that copy, with `sendImage` and `readImage` inlined into it, put `car`'s program 40 bytes over a 2,048-byte sector, one more sector to load from the disc, and `regress.sh` failed `car` and `car:9410` on a sound tick a frame early. The form kept is 1,016 bytes under (the program before this spec was 1,616 under).
- `queueLen` is 64, as Sony's.
- `ResetGraph(0)` clears the queue's first 2,304 bytes, 32 requests' worth, as it cleared the whole queue before: its time stays Sony's 6,400-byte clear. Clearing the bigger queue moved every probe's `ResetGraph(0)` by 18,432 cycles.
- The interrupt stack is 1,536 bytes, from 1,024, so that the handler running again inside itself leaves the vertical blank's callback at least the room it had.
- The probe's sweep of waits is cut to w = 0 (`waits` at the top of `main.go`): the emulator waits out the 25 ms cap on nearly all of the others. k = 1, 3, 5 with 8 images stay, and the k = 0 and 1 sweeps of 0, 8 and 32 images, so it runs 9 lines. A console run can put 6 and 10 back in `waits`.
- `TestFieldTime` (`image/probetest/fieldtime_test.go`) runs the probe with the switch on (`-fieldtime` names the disc) and checks the k = 1 lines with 0, 8 and 32 images: idle every frame, and the mean within 40 µs of Sony's. It passes on the new image and fails on the one before.

### fieldtime, switch on

Mean µs from VSync's return to the GPU idle, the cut probe:

| k boxes, n images | Sony's | clean before | clean now |
|---|---|---|---|
| 0, 0 | 9,544 | 9,541 | 9,541 |
| 0, 8 | 10,210 | 10,125 | 10,125 |
| 0, 32 | 12,228 | 11,875 | 11,875 |
| 1, 0 | 18,621 | 18,618 | 18,618 |
| 1, 8 | 18,914 | 19,498 | 18,904 |
| 1, 32 | 19,274 | 5,724 (21,204, the clock wrapped) | 19,264 |

k = 3 and 5 with 8 images are never idle within 25 ms on either library, before or after. Behind one box the first 8 images now add 286 µs (Sony's 293) and the next 24 add 360 (Sony's 360), 508 cycles an image as Sony's. The clean library's `GsDrawOt` returns about 150 cycles sooner, so its box ends that much sooner, which is the 10 µs or so it stays ahead.

Behind the clear alone (k = 0) nothing queues, the clear being done before the first `LoadImage`, and the clean library is ahead of Sony's by 85 and 353 µs, as before.

### The free path

Unchanged in the emulator. With the switch off, on the 16 probes spec 40 used (`drawq`, `clearimage`, `storeimage`, `loadleft`, `imgq`, `kanjistream`, `kanjipast`, `kanji`, `fntchars`, `drawwait`, `gscycle`, `dispswap`, `flush`, `fntlayout`, `linestart`, `resetret`), every I/O read and write with its cycle from "Execute !" to the end (`PROBE_BUS`) is the same on the new image as on HEAD's (`.build/verify51/libps-head.exe`), with the DMA addresses written to D2_MADR masked, since the image's data moved. Every TTY is the same, `fntlayout`'s with the packet addresses inside the image masked.

The code, register numbers and addresses masked: `enqueue`, `loadImage`, `StoreImage`, `ClearImage`, `MoveImage`, `GsDrawOt`, `PutDrawEnv`, `DrawSync`, `pollQueue`, `imageEnter`, `imageWait`, `FntFlush`, `FntLoad`, `KanjiFntOpen`, `ResetGraph`, `VSync` and `libps_interrupt` are the same instructions as HEAD's. `drawNext` differs only in its image branch (the last-image test, 12 instructions shorter) and in the queue's mask (3Fh for 1Fh) and offsets. `imageTurn`'s way out when no image is next is the same, so with the switch on `drawwait` prints exactly what HEAD's image does. The registers the image calls leave are the same (`loadleft`).

What moved, off the free path: a queued `LoadImage` or `StoreImage` returns 1 eighth later (`queueImage` records `lastImage`), 116 and 123 in `imgq` with the switch on, against 115 and 122 before and Sony's 96 and 94. `imgq`'s total, the frame and the image behind it to `DrawSync(0)`, is 38,582 to 38,583 eighths, against 38,516 to 38,519 before and Sony's 38,553 to 38,555: the image now goes at Sony's pace after the GPU is ready.

The image ends at 80067348h. The static build of `car`, the closest to a sector, is 130,056 bytes loaded, 1,016 under its 64 sectors.

### Verify chain

`.build/verify51/chain.sh` against `summary-verify50.txt`:

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify50 | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |
| now | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |

No archive line changed.

- `image/regress.sh`: 36 PASS, 5 TIME, the same programs as verify50.
- `regress.sh`: 33 PASS, 8 TIME, the same programs.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- The chain's `go test` list: no failures, `TestYarozeLoadAndRun/ours` included. `./exp/cleanlib/...` passes.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- `TestFieldTime` and `TestQueuedImages` pass. `TestDrawWait` fails its reset lines on the new image and on HEAD's alike: since 5e04f42 the emulator's GP1(00h) and (01h) stop a timed drawing, so after `ResetGraph(1)` the GPU is ready and the second `GsDrawOt` takes 55 eighths, as spec 40 measured on a console, where the test still expects the old model's 30,000 to 45,000. Left as it is.

### What did not come out as the spec says

- **Not every mark inside the chain is Sony's.** Each image's period is Sony's 508 cycles and the watch comes 140 cycles before the GPUSTAT read that precedes the image, but the clean library still starts each image's empty packet list after it and reads D2_CHCR for the blocks 34 cycles after starting them, where Sony's reads it at 124. The totals match: the first image's blocks start 350 cycles after GPUSTAT first shows the GPU ready, Sony's 352.
- **The handler runs again inside itself** after a chain, to take the vertical blank as Sony's does. That puts the kernel's controller handler out of that vertical blank, as on Sony's, so the pads are read once in such a frame, not twice. The handler's stack was made bigger for it.
- **`DrawSync(1)` still counts one more than Sony's** with images queued (spec 39).

### Not checked

- A console run of `fieldtime` and `qlen` on both libraries, and of gasgar.
- Images queued behind a request that is itself waiting, and a `StoreImage` in a chain: only `LoadImage` behind a running frame was timed. `imgq`'s `StoreImage` goes the new way and passes.
- Whether Sony's handler also takes a drive interrupt raised during the chain in the same pass: the clean one leaves that case to the kernel, as before.
