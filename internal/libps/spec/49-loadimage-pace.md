# A free LoadImage starts its blocks where Sony's does

## Why

Spec 45 left this open (its Results, part 2, "Not done here"). A `LoadImage` that finds the GPU free takes 85 eighths on both libraries, but the two spend that time in a different order:

- Sony's starts the image's blocks 92 cycles before it returns.
- The clean one starts them 264 cycles before it returns: it spends its pace after starting the transfer, where Sony's spends it before.

With the drawing time on, the blocks take the emulator about 220 cycles (since c31746e). So they are still going when Sony's call returns, and a `DrawSync(1)` straight after gives 1, with channel 2 busy. On the clean library they are done, and it gives 0. With the drawing time off the blocks take no time, and both give 0.

A program that polls `DrawSync(1)` after loading an image, or starts drawing straight after one, sees the GPU in a different state. The same order likely holds for the other image calls that start a transfer at once (`StoreImage`, `MoveImage`, `ClearImage`); that is to be checked, not assumed.

## What to do

1. Find, from a bus trace (`PROBE_BUS`), the cycle of each I/O access a free `LoadImage` makes on both libraries, from its entry to its return: the GPU's and DMA's registers and anything else it touches. Do this for the sizes spec 47's `qstate` table uses (16 by 16, 16 by 1, 20 by 2, 0 by 4) and a large one.
2. Move the clean `LoadImage`'s time so its accesses fall at Sony's cycles, within a few cycles, and its total stays Sony's. Moving the pace moves every `LoadImage`'s I/O, so check the queued path (`queueLoad`) and the handler's image chain keep their times, as spec 47 left them, in both queue states.
3. Check `DrawSync(1)` straight after a free `LoadImage` gives Sony's answer with the drawing time on.
4. Do the same check on `StoreImage`, `MoveImage` and `ClearImage` when free. Fix each one whose order differs, or, if one is much larger, say what it would take and leave it for its own spec. `MoveImage` is 22 cycles over Sony's and `ClearImage`'s fill 74 over at the one size spec 47 measured, and fixing their order may settle those too.

Keep what specs 45, 47 and 48 fixed: `qstate`'s figures in both states, the interrupt's length, and `GsSortClear` and the other Gs calls.

## Testing it

In the emulator against Sony's image, with the drawing time on, and the cost model on (`PROBE_CPUCOST=1`) where console time matters. A new test, failing before and passing after, for `DrawSync(1)` after a free `LoadImage`. Spec 47's checks must still pass: TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll and TestQueueLength, on Sony's discs and the new image, plus `image/regress.sh`, `go test ./exp/cleanlib/...`, and `archive.sh` against the current tree's summary.

A console run confirms it: the spec side runs a probe for `DrawSync(1)` after a free `LoadImage` under `-library sony-ourmon` and `-library clean`.

## Results

Done 2026-11-12 in the emulator (implementation side), with the scph1001 BIOS, and on the console by the spec side (below). The candidate image is `.build/console/libps-s49h.exe.gz` (the image whose SHA-1 starts ac82982d), and the tree's image before any change is `.build/console/libps-s49-base.exe.gz`.

### Summary

- A free `LoadImage` now makes every access at Sony's cycle, or within 4 of it, at all six sizes tried, and takes Sony's time. Its blocks start 108 cycles before the marker after the call, Sony's 108, where they started 272 before. `DrawSync(1)` straight after it gives 1 for an image of whole blocks, as on Sony's.
- `StoreImage`, `MoveImage` and `ClearImage` had the same fault, each its own size of it, and are fixed the same way. `MoveImage` and `ClearImage`'s fill take Sony's time now (22 and 74 cycles over before).
- `DrawSync(1)` itself took about twice Sony's time (248 cycles against 128 on the probe) and read other registers. It now reads what Sony's reads, at Sony's cycles. That was half of the wrong answer after `MoveImage` and `ClearImage`: the clean call came back so late that the GPU had finished.
- Found on the way and fixed: `MoveImage` of no width or no height returns -1 on Sony's without a look at the GPU; an image call that goes into the queue took 128 cycles more than Sony's; an image of no size made behind a drawing is queued and counted on Sony's, and was not; `GsDrawOt` and `PutDrawEnv` started the channel 32 and 140 cycles sooner before their return than Sony's.
- `qstate` is as spec 47 left it or nearer Sony's, in both states. The archive has 99 the same, where it had 98.

### What was measured

`probes/imgsync` (new) makes each image call with the GPU free, after a `VSync(0)` and a `DrawSync(0)`, between GP1 marker words, then `DrawSync(1)` at once and a marker again, and then polls `DrawSync(1)` to 0. It does this for `LoadImage`, `StoreImage`, `MoveImage`, `ClearImage` on a rectangle and `ClearImage` on a fill, each at 16 by 16, 16 by 1, 20 by 2, 0 by 4, 64 by 64 and 256 by 256, twice over. A second part makes the calls behind a drawing that still holds the channel (a clear and a box over the screen), and a third makes `MoveImage` and `ClearImage` on sizes and places of each kind and prints what they return. Run with the drawing time on and a bus trace:

	PROBE_DRAWTIME=1 PROBE_BUS=start exp/cleanlib/image/probe.sh s49imgsync exp/cleanlib/spec/probes/imgsync

Cycles below are from the marker before the call, the probe's own instructions included, the same in both rounds. The scripts that read the traces are in `.build/s49/`.

**A free `LoadImage`, 16 by 16.**

| access | Sony's | before | now |
|---|---|---|---|
| the watch: GPUSTAT, timer 1 | 132, 134 | 132, 136 | 132, 134 |
| I_MASK read, written 0 | 216, 218 | 154, 156 | 216, 218 |
| D2_CHCR read | 264 | 158 | 264 |
| GPUSTAT read | 290 | 160 | 290 |
| the transfer's watch: GPUSTAT, timer 1 | 352, 354 | 238, 242 | 352, 354 |
| GPUSTAT read, the GPU ready | 494 | 266 | 494 |
| GP1 04000000h | 510 | 352 | 510 |
| GP0 01000000h, A0000000h, place, size | 518, 528, 538, 548 | 358, 362, 370, 378 | 518, 528, 538, 548 |
| GP1 04000002h | 566 | 402 | 566 |
| D2_MADR, D2_BCR | 574, 584 | 408, 420 | 574, 588 |
| D2_CHCR 01000201h, the blocks start | 592 | 428 | 592 |
| I_MASK read, written back | 644, 646 | 460, 466 | 644, 646 |
| I_MASK read again | none | 668 | none |
| the marker after the call | 700 | 700 | 700 |

So Sony's spends its time before the transfer and between its accesses, and 108 cycles from the blocks' start to the marker. The clean call ran through its accesses and then spun for 122 cycles and more.

**By size**, on Sony's: the entry is the same to the transfer's watch at 352. An image of no size makes no read of GPUSTAT at 494 and no transfer, reads I_MASK at 534 and returns at 590. The CPU's words go to GP0 from 568, 16 cycles apart; with no blocks I_MASK is read 60 cycles after the last of them (16 by 1: 740, return 796), and with blocks GP1 04000002h comes 16 after the last word (20 by 2: 632, the blocks at 658, return 766). 64 by 64 and 256 by 256 are 16 by 16 to the cycle.

Now: the same, with these differences left. The first of the CPU's words comes 2 cycles early (566), the rest on Sony's cycles. With words and blocks, GP1 04000002h and the blocks come 4 late (636, 662); I_MASK and the return are Sony's. D2_BCR is written 4 late, between D2_MADR and D2_CHCR, which are on Sony's cycles. An image of no width reaches its first watch 2 cycles early and returns on Sony's cycle.

**`StoreImage`** is `LoadImage` with the GPU's answer waited for. Sony's 16 by 16: the entry as `LoadImage`'s, the transfer's watch at 350, GPUSTAT at 492, GP1 04000000h at 508, GP0 at 516, 524, 534 and 544, GPUSTAT read at 552 for the pixels to be ready, GP1 04000003h at 580, D2_MADR, D2_BCR and D2_CHCR at 588, 598 and 606, I_MASK at 656 and the return at 712. The CPU's words are read from 578, 16 apart. The clean one before started its first watch at 142 and the blocks at 434, 278 cycles before the return. Now it is Sony's but for the transfer's watch 2 late (352), the second of the CPU's words on 2 early, and D2_BCR 4 late. In the emulator a `StoreImage`'s blocks take no time, so `DrawSync(1)` after it gave 0 on both libraries before as well.

**`MoveImage`**, on Sony's (the third part's cycles): the watch at 174, I_MASK at 258 and 260, D2_CHCR read at 306, GPUSTAT at 332, GP1 04000002h at 356, D2_MADR, D2_BCR and D2_CHCR 01000401h at 364, 372 and 380, I_MASK at 414 and 416, the return at 464. Before: the watch at 126, the channel's start at 384, the return at 486. Now: GP1 04000002h 2 late (358), D2_BCR 4 early and D2_CHCR 2 late (382), everything else and the return on Sony's cycles. A rectangle of no width returns -1 after 102 cycles and one of no height after 114, with no access at all; a negative width or height is sent as it is and returns 0. The clean one before sent every size and returned 0.

**`ClearImage`**, on Sony's: the watch at 154, I_MASK at 238 and 240, D2_CHCR at 286, GPUSTAT at 312. Then Sony's builds its packet. For a fill the channel starts 156 cycles after that read of GPUSTAT (GP1 04000002h, with D2_MADR, D2_BCR and D2_CHCR 8, 16 and 24 after it). For a rectangle it asks the GPU for the drawing area and offset first: GP1 10000003h 176 cycles after the read with x off a multiple of 64, 186 with x on one and the width not, each answer read from GPUREAD 8 cycles after it is asked, 10000004h and 10000005h 34 and 36 cycles after the question before, and the channel 42 after the last. A width over 1,023 adds 6, a height over 511 adds 4, and a negative width takes 18 off. I_MASK is read 48 cycles after the channel starts and written 2 later, and the call returns 54 after that: 738 and 728 for the two rectangles, 596 for a fill. The clean one before built its packet first (the watch at 202 or 188) and paced a fill at its end: the same 738 for a rectangle, with the channel started 18 cycles sooner before the return, and 670 for a fill, with it started 120 sooner. Now every access is on Sony's cycle but D2_BCR, 2 early. The sizes' times: a width over 1,023 or a height over 511 is 2 over in all, a fill of no width or no height 2 under, and a negative width 4 over.

**`DrawSync(1)`**, from the marker before it. Sony's reads D2_CHCR at 76. With the channel busy it reads nothing more and returns at 118. Otherwise it reads GPUSTAT at 92 and returns at 128 with the GPU ready and at 134 with it drawing. With requests waiting behind the one running it reads D2_CHCR at 76 and again at 102 and returns the count at 138. The clean one before read DICR at 68, D2_CHCR at 196 and GPUSTAT at 208, and D2_CHCR and GPUSTAT once more with the GPU drawing: 232 to 270 cycles, and 202 with requests waiting. Now it is Sony's in each case, the second read with requests waiting 2 late (104).

**All thirty cases.** Each cell: the call from marker to marker, the transfer's start before the second marker, the `DrawSync(1)` after it, what that gave, and the polls after it to a 0.

| call | Sony's | before | now |
|---|---|---|---|
| `LoadImage` 16 by 16 | 700, 108, 118, 1, 0 | 700, 272, 248, 0, 0 | 700, 108, 118, 1, 0 |
| `LoadImage` 16 by 1 | 796, none, 128, 0, 0 | 796, none, 248, 0, 0 | 796, none, 128, 0, 0 |
| `LoadImage` 20 by 2 | 766, 108, 128, 0, 0 | 764, 264, 248, 0, 0 | 766, 104, 128, 0, 0 |
| `LoadImage` 0 by 4 | 590, none, 128, 0, 0 | 592, none, 248, 0, 0 | 590, none, 128, 0, 0 |
| `LoadImage` 64 by 64 | 700, 108, 118, 1, 30 | 700, 272, 232, 1, 13 | 700, 108, 118, 1, 30 |
| `LoadImage` 256 by 256 | 700, 108, 118, 1, 513 | 700, 272, 232, 1, 248 | 700, 108, 118, 1, 513 |
| `StoreImage` 16 by 16 | 712, 106, 128, 0, 0 | 712, 278, 248, 0, 0 | 712, 106, 128, 0, 0 |
| `StoreImage` 16 by 1 | 808, none, 128, 0, 0 | 808, none, 248, 0, 0 | 808, none, 128, 0, 0 |
| `StoreImage` 20 by 2 | 778, 106, 128, 0, 0 | 778, 268, 248, 0, 0 | 778, 104, 128, 0, 0 |
| `StoreImage` 0 by 4 | 586, none, 128, 0, 0 | 586, none, 248, 0, 0 | 586, none, 128, 0, 0 |
| `StoreImage` 64 by 64 | 712, 106, 128, 0, 0 | 712, 278, 248, 0, 0 | 712, 106, 128, 0, 0 |
| `StoreImage` 256 by 256 | 712, 106, 128, 0, 0 | 712, 278, 248, 0, 0 | 712, 106, 128, 0, 0 |
| `MoveImage` 16 by 16 | 472, 92, 134, 1, 0 | 494, 110, 270, 0, 0 | 472, 90, 134, 1, 0 |
| `MoveImage` 16 by 1 | 472, 92, 128, 0, 0 | 494, 110, 258, 0, 0 | 472, 90, 128, 0, 0 |
| `MoveImage` 20 by 2 | 472, 92, 128, 0, 0 | 494, 110, 258, 0, 0 | 472, 90, 128, 0, 0 |
| `MoveImage` 0 by 4 | 110, none, 128, 0, 0 | 494, 110, 258, 0, 0 | 110, none, 128, 0, 0 |
| `MoveImage` 64 by 64 | 472, 92, 134, 1, 31 | 494, 110, 266, 1, 14 | 472, 90, 134, 1, 31 |
| `MoveImage` 256 by 256 | 472, 92, 134, 1, 526 | 494, 110, 266, 1, 254 | 472, 90, 134, 1, 526 |
| `ClearImage`, a rectangle 16 by 16 | 732, 108, 134, 1, 0 | 742, 126, 270, 0, 0 | 732, 108, 134, 1, 0 |
| `ClearImage`, a rectangle 16 by 1 | 732, 108, 128, 0, 0 | 742, 126, 258, 0, 0 | 732, 108, 128, 0, 0 |
| `ClearImage`, a rectangle 20 by 2 | 732, 108, 128, 0, 0 | 742, 126, 258, 0, 0 | 732, 108, 128, 0, 0 |
| `ClearImage`, a rectangle 0 by 4 | 732, 108, 128, 0, 0 | 740, 126, 258, 0, 0 | 730, 108, 128, 0, 0 |
| `ClearImage`, a rectangle 64 by 64 | 732, 108, 134, 1, 31 | 742, 126, 266, 1, 14 | 732, 108, 134, 1, 31 |
| `ClearImage`, a rectangle 256 by 256 | 732, 108, 134, 1, 526 | 742, 126, 266, 1, 254 | 732, 108, 134, 1, 526 |
| `ClearImage`, a fill 16 by 16 | 594, 104, 134, 1, 6 | 668, 224, 266, 1, 2 | 594, 104, 134, 1, 6 |
| `ClearImage`, a fill 16 by 1 | 594, 104, 128, 0, 0 | 668, 224, 258, 0, 0 | 594, 104, 128, 0, 0 |
| `ClearImage`, a fill 20 by 2 | 594, 104, 128, 0, 0 | 668, 224, 258, 0, 0 | 594, 104, 128, 0, 0 |
| `ClearImage`, a fill 0 by 4 | 594, 104, 134, 1, 0 | 668, 224, 258, 0, 0 | 594, 104, 134, 1, 0 |
| `ClearImage`, a fill 64 by 64 | 594, 104, 134, 1, 31 | 668, 224, 266, 1, 14 | 594, 104, 134, 1, 31 |
| `ClearImage`, a fill 256 by 256 | 594, 104, 134, 1, 526 | 668, 224, 266, 1, 254 | 594, 104, 134, 1, 526 |

(The rectangles are at x 641, the fills on a width of 64 at least; a rectangle at 640 takes Sony's 10 cycles more, above.) The polls to 0 are the same as Sony's in every case now: a poll costs what Sony's costs, where it cost about twice that.

**Behind a drawing.** Sony's, with the channel still busy and nothing waiting yet: the entry to the read of D2_CHCR, then DICR read 64 cycles later and written 16 after that with the channel's completion made to interrupt, I_MASK put back 216 after that, D2_CHCR read 20 cycles later, and the return. With requests waiting already it reads neither D2_CHCR nor DICR: I_MASK is put back 292 cycles after it was masked (406 in `MoveImage`), and D2_CHCR read 20 after that. Marker to marker:

| call behind a drawing | Sony's | before | now |
|---|---|---|---|
| `LoadImage`, the first | 646 | 724 | 646 |
| `LoadImage` or `StoreImage` behind another | 596 | 724, 786 | 596 |
| `MoveImage` | 756 | 428 | 756 |
| `ClearImage`, a rectangle | 634 | 562 | 634 |
| `ClearImage`, a fill | 658 | 638 | 658 |
| `DrawSync(1)`, eight requests waiting | 138 | 202 | 138 |

Now the accesses are Sony's too: I_MASK is put back within 4 cycles of Sony's (an image's read on Sony's cycle and its write 4 late) and the read of D2_CHCR after it is on Sony's cycle.

An image of no size made behind a drawing goes into Sony's queue as any other: the same accesses, and `DrawSync(1)` counts it. When its turn comes Sony's handler sends nothing for it. The clean library returned at once and queued nothing, so `DrawSync(1)` gave 1 where Sony's gave 1, 2 and 3 after three of them. `probes/imgzero` (new) prints DICR, D2_CHCR and `DrawSync(1)` after each of three such calls behind a drawing, after the `DrawSync(0)` that ends it and around a second frame: the same lines on both libraries now. `probes/imgpace` (spec 45) has the same from a program's side: a queued `LoadImage` from one read of the clock to the next took 772 cycles and takes Sony's 644, and a poll of `DrawSync(1)` took 224 and takes Sony's 160, so a frame's polls come to Sony's 1,789 where they were 1,259.

### What changed

**`LoadImage` and `StoreImage`** (`image.go`, `extra.go`, `gpu.go`). The pace at the call's end is gone. The time is spent between the accesses, in stores (`space`), turns of `spin` where a pad is long, and nops where a store would cost 4 cycles, each count set on `imgsync`:

- The transfer (the place and size as the GPU takes them, the words the CPU moves, D2_BCR's word, where the blocks start) is worked out first, before the call's watch (`imageTransfer`): Sony's has 140 cycles between the transfer's watch and its read of GPUSTAT and sends the image 16 after it, which leaves no room to work it out there.
- `imageEnter` makes the entry's accesses at Sony's cycles. It no longer loops: with the channel done and the GPU still drawing it reads GPUSTAT until the GPU is ready, with I_MASK 0, as Sony's is seen to (`gpuWait`), where it put I_MASK back and waited in `drain`. It returns how it found the GPU (`enterIdle`, `enterWaited`, `enterLater`); `queue.later` and `imageLater` are gone.
- `sendImage` and `readImage` take the transfer and pad between their accesses. Each kind of image (blocks, the CPU's words, both) pads to I_MASK with a value of its own: with the same value the compiler merged the pads' ends, and a change to one moved another.
- I_MASK is read and then written with the mask the call found, in the next instruction, as Sony's is (`imageLeave`); the clean call ORed the two, 4 cycles later. The four image calls leave that mask in a0 (`imageCallLeaves`) and no longer read I_MASK a second time, which Sony's does not.
- `LoadImage` is the whole call now; the font's and the kanji's images go through it (`loadImage` is a wrapper).

**The watch** (`drawNow` in `gpu.go`): the drawing calls' watches read GPUSTAT and timer 1 in consecutive instructions, 2 cycles apart, as Sony's does; as Go the second came 4 after. The drive's watches keep `cdNow` as it was. A first try changed `cdNow` itself, and the drive's wait loops then ran a few cycles longer a pass: the start-up took 6 cycles more and `CdSearchFile` 160, and `pssoft-kyuri-2-kyuri` in the archive, which reads an address that follows its timing, read another.

**An image that goes into the queue.** With requests waiting, `imageEnter` reads neither D2_CHCR nor GPUSTAT. With none waiting and the channel busy it sets DICR itself, at Sony's cycles. `queueImage` no longer touches DICR or D2_CHCR and always queues, an image of no size too, whose turn sends nothing (`startImage`). The check it made, that the channel had not finished before its completion was made to interrupt, is made where Sony's reads D2_CHCR, 20 cycles after I_MASK is back (`imageQueued`): should the channel be idle there, the queue is moved on from the call (`pollQueue`). An image the queue refuses (pixels on the stack) turns the interrupt off again if the call turned it on (`imageRefused`).

**`DrawSync`** (`gpu.go`). Any mode but 0 goes to `drawPoll`, a function that calls nothing: the count of requests waiting, and with none, D2_CHCR and GPUSTAT as Sony's reads them. With requests waiting and the channel seen idle, which the channel's interrupt should not leave for a call to find, it moves the queue on as every `DrawSync(1)` did before (`drawPollMove`). A request that has gone out with none behind it is left for the next call that looks at the queue to retire, as the handler leaves it (spec 47). Mode 0 makes its first look at the queue in `DrawSync` and waits in `drainRest`: with the wait's loop inline, `DrawSync` saved eight registers, and mode 1 restored them on its way out, 10 cycles more than Sony's whole way out. Mode 0 takes its 216 on `qstate` as before, with stores before its watch.

**`MoveImage` and `ClearImage`** no longer go through `enqueue`. `MoveImage` makes its packet in the queue's next request before it masks the interrupts, checks after masking that the queue's tail is where it was (a callback may have queued a request: `packetMoved`), and has the entry's accesses written out in it: Sony's starts the channel 24 cycles after the read of GPUSTAT, less than a function's return takes. It returns -1 for no width or no height. `ClearImage` enters through `packetEnter`, the same accesses, and builds its packet after the read of GPUSTAT, where Sony's does, with pads for Sony's time by kind and size. Its test for a multiple of 64 is on the low six bits. Both end as the image calls do (`imageLeave`, `imageQueued`).

**`enqueue`**, now `GsDrawOt`'s, `PutDrawEnv`'s and `FntFlush`'s alone. With `MoveImage` and `ClearImage` gone from it the compiler dropped what only they used and it came out 22 cycles shorter, so its pads had to be set again, and they were set to Sony's order: 15 stores between its watch and I_MASK, 5 nops before I_MASK is put back, none after. From its watch to the channel's start is now Sony's 206 cycles. Its callers make up the rest: `GsDrawOt` has 6 nops before the call, `PutDrawEnv` spins before its request is queued as well as after, and a flush has 6 nops after its list is queued. On `qstate`:

| | Sony's | before | now |
|---|---|---|---|
| `GsDrawOt`: watch, channel's start, I_MASK back, end | 98, 304, 340, 400 | 92, 272, 296, 400 | 98, 304, 330, 400 |
| `PutDrawEnv`: the same | 526, 732, 768, 1,952 | 310, 592, 616, 1,952 | 424, 732, 758, 1,952 |
| `FntFlush(0)`, empty: the same | 208, 414, 450, 536 | 222, 402, 426, 536 | 216, 422, 448, 536 |

Inside it the reads are not at Sony's cycles: I_MASK is written 60 cycles after the watch where Sony's is 86 after, and D2_CHCR and GPUSTAT are read 32 and 36 after that where Sony's are 46 and 72. `PutDrawEnv` copies its packet with I_MASK 0, which Sony's does not, so its watch comes 102 cycles before Sony's for the channel to start with Sony's. The channel's address is written as it is, top bits and all, as Sony's writes it (80090B30h for a table at that address, where the clean one wrote 00090B30h); the channel uses the low 24 bits.

**The image chain** (`sendImages`): `sendImagesSpace` is 38 (37), so that one queued image's blocks to the next's are Sony's 966 cycles with `sendImage`'s new pads, the address unmasked and `startImage`'s test for an image of no size.

**The image's layout.** `queueLoad` and `queueStore` start with four nops (`queueImageNops`), for their size and nothing else. Without them each came to 216 bytes and the packing gave one of them the place `libps_vsync_wait` had (8003C070h) and put that on other cache lines. Under the console's costs `VSync(0)` at 240 lines then returned 158 to 165 cycles after the interrupt in many fields, where it returns within 155, and the interrupt ran some 30 cycles longer (`TestVSyncPoll`). With the nops `libps_vsync_wait` is where it was. `vblankInterrupt`, `libps_interrupt`, `libps_wait_blanks` and `VSync` did not move in any candidate.

### `qstate`, both states

Marker to marker, the same in both states on both libraries:

| call | Sony's | before | now |
|---|---|---|---|
| `GsDrawOt` | 400 | 400 | 400 |
| `DrawSync(0)` | 216 | 216 | 216 |
| `FntFlush(0)`, empty | 536 | 536 | 536 |
| `FntFlush(-1)`, empty | 548 | 534 | 534 |
| `FntFlush(0)`, "hi" | 816 | 862 | 862 |
| `FntFlush(-1)`, "hi" | 828 | 860 | 860 |
| `PutDrawEnv` | 1,952 | 1,952 | 1,952 |
| `GsSwapDispBuff`, the second state | 4,780 | 4,782 | 4,782 |
| `LoadImage` 16 by 16, 16 by 1 | 702, 798 | 702, 798 | 702, 798 |
| `LoadImage` 20 by 2, 0 by 4 | 768, 592 | 766, 594 | 768, 592 |
| `StoreImage` 16 by 16, 16 by 1, 20 by 2, 0 by 4 | 714, 810, 780, 588 | 714, 810, 780, 588 | 714, 810, 780, 588 |
| `MoveImage` | 474 | 496 | 474 |
| `ClearImage`, a rectangle | 744 | 744 | 744 |
| `ClearImage`, a fill (64 wide) | 602 | 676 | 602 |

The four lines that are not Sony's are the ones spec 47 left ("Not done"), unchanged.

### On a console's costs

`imgsync` under the cost model (`PROBE_CPUCOST=1`), a call from marker to marker with the GPU free, at 16 by 16 and 64 by 64, Sony's and now: `LoadImage` 1,379 and 1,430, 1,294 and 1,353; `StoreImage` 1,409 and 1,433, 1,396 and 1,400; `MoveImage` 935 and 958, 893 and 920; `ClearImage` 1,413 and 1,472, 1,388 and 1,398 for a rectangle and 1,155 and 1,163, 1,137 and 1,163 for a fill. Each case is made once after a `VSync(0)`, so these are cold calls, cache misses and all: the clean ones are 0 to 5% over. The blocks start 261 cycles before the marker on Sony's and 259 on the clean library. `DrawSync(1)` cold is 193 to 231 on Sony's and 217 to 252; polled in a row with requests waiting it is 172 against 144.

The calls that go into the queue, under the model: an image behind another 713 to 720 on Sony's and 676 to 778. The first of each kind in the probe is far over, 1,064 against 1,349 for the first image, 985 against 1,347 for `MoveImage`, 766 against 1,212 for `ClearImage`'s rectangle and 807 against 970 for its fill: those run code no call has run yet, and the clean path has more of it to miss in the cache than Sony's. Not chased here: it wants the console's own figures first.

`probes/loadsync` (new, for a console) makes each call with the GPU free, waits d turns of a small loop (17 cycles a turn under the model) and reads `DrawSync(1)`, for d from 0 to 63, and prints the first d that gives 0, with the call's and `DrawSync(1)`'s time in eighths. In the emulator with the model and the drawing time on:

| | Sony's | before | now |
|---|---|---|---|
| `LoadImage` 24 by 24, first d that gives 0 | 4 | 0 | 4 |
| `LoadImage` 32 by 32 | 27 | 5 | 26 |
| `MoveImage` 32 by 32 | 38 | 25 | 38 |
| `ClearImage`, a fill 64 by 16 | 36 | 14 | 35 |
| `LoadImage`, the call, eighths | 171 | 156 | 173 |
| `MoveImage`, the call | 122 | 112 | 117 |
| `ClearImage`'s fill, the call | 147 | 147 | 151 |
| `DrawSync(1)` | 25 to 28 | 42 to 54 | 26 to 28 |

Under the model a 16 by 16 image's blocks are done before either library's `DrawSync(1)` reads the channel, so the answer there is 0 on both. What a console's blocks take is for the console to say.

### Not done, and found in passing

- **`FntFlush`'s watch** comes 8 cycles after Sony's (216 against 208) with nothing to draw, so its channel starts 8 late; the calls' totals are as they were. With text the flush is 46 cycles over as before (spec 47), and all of it now lies before the watch.
- **`enqueue`'s reads** of I_MASK, D2_CHCR and GPUSTAT are not at Sony's cycles, above. Its queued path (a list behind a drawing) still reads D2_CHCR and GPUSTAT where Sony's image calls read neither; Sony's `GsDrawOt` behind a drawing was not traced.
- **A size out of range**: 2 or 4 cycles off in `ClearImage`, above; `LoadImage` and `StoreImage` reach their first watch 2 cycles early for a width or height of 0 or less and 2 late for one past video memory, since the clean clamp is a branch.
- **Tests that go by a few cycles of the start-up.** `TestIRQGaps` goes by where a first `VSync(0)` leaves a program, to the cycle, after a `GsInitGraph` and a `GsDefDispBuff` whose times are not Sony's (spec 47), and `TestVSyncPoll` under the console's costs by where `VSync`'s code lies in the cache, above. Each failed on an earlier candidate and passes on this one. The first can fail on a change of a few cycles there that is no fault; the second is a real cost on a console, and a reason to look at the packing when the image's functions change size.
- **`GsSetDrawBuffClip`** is 22 cycles under Sony's (1,978 against 2,000 in `pssoft-kyuri-2-kyuri`), and `GsDefDispBuff` 88 under, as before.
- **`DrawSync(0)` while it waits** reads D2_CHCR and GPUSTAT every 162 cycles on Sony's (114 and 48 apart) and every 44 on the clean library, so the clean call can return up to 120 cycles sooner after the drawing ends. Seen in `imgzero`'s trace, not changed.
- **A queued image's empty packet list** is still started after each image (spec 45): in `imgzero` the clean library starts the channel 10 times where Sony's starts it 7. Behind `imgsync`'s drawing the eight queued requests take the clean library 518 polls of `DrawSync(1)` to drain where Sony's takes 516.
- **A callback that queues a request** between `MoveImage`'s making its packet and its masking the interrupts is handled (`packetMoved`). One that fills the queue there is not, as it was not in `enqueue`, which waits for room before it masks.

### Testing

On the candidate (`.build/console/libps-s49h.exe.gz`), scph1001 BIOS:

- `TestImageSync` (new, `image/probetest/imgsync_test.go`, `-imgsync` names the disc) runs `imgsync` with the drawing time on and checks, from outside, each of the thirty cases in both rounds against Sony's: the call's length within 4 cycles, the transfer's start before the return within 4, `DrawSync(1)`'s length within 2, what it gave, and the polls to 0 within 2 in 100; then what `MoveImage` returns by size and `DrawSync(1)`'s count of 8 behind the drawing. It passes on Sony's disc and on the candidate's, and fails on the image before (`LoadImage` 16 by 16: the start 272 before the return, `DrawSync(1)` 248 long and giving 0).
- `TestDrawWait`, `TestFieldTime`, `TestQueuedImages`, `TestDMALeft`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps`, `TestVSyncPoll` and `TestQueueLength` pass on Sony's discs and on the candidate's, each built with `probe.sh` (`.build/s49NAME-cimg` and `-cimg-clean`; `.build/s49/tests.sh` runs the ten).
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before.
- `go test ./exp/cleanlib/...` passes. The static clean build compiles with and without the fidelity tag (`docs/tutorial/hello`).
- `probes/imgpace` with the drawing time on: one queued image's blocks to the next's 966 cycles (Sony's 966), 5,638 with a vertical blank between (5,634); 8 and 32 images done at 78,557 and 81,448 eighths (Sony's 78,514 and 81,402; before 78,560 and 81,454).
- `probes/imgzero`, `loadleft`, `clearimage`, `storeimage` and `drawq`: the same TTY on both libraries.
- `probes/synccount` with the drawing time on, Sony's, before and now, in eighths: `DrawSync(1)` first read after the frame's last call at 96, 109 and 96 with no image behind the frame, 183, 201 and 183 with one, 335, 385 and 335 with three, and at 108, 124 and 108 after a free `LoadImage`. The frame and its images are done at 19,248, 19,245 and 19,249 with none, 19,352, 19,419 and 19,430 with one, and 19,641, 19,665 and 19,668 with three: the chain's end stays behind Sony's (spec 45, "Not done"), 3 to 11 eighths further than it was.
- The full `archive.sh`, against the tree before (`.build/s49/summary-base.txt`, and `summary-h.txt` for the candidate):

  | | same | differ | still | shift | wild | tty | stall | noboot |
  |---|---|---|---|---|---|---|---|---|
  | before | 98 | 19 | 13 | 7 | 0 | 1 | 1 | 1 |
  | now | 99 | 19 | 13 | 6 | 0 | 1 | 1 | 1 |

  Two lines changed. `scee-topten-grav` is now the same; it slipped a frame at change 244 of 245 (frame 402 on Sony's, 403 on the clean library). `test-zero-zero` first shifts at change 562 (frame 944), from 561 (frame 932): the program times itself with the root counters, and where it first slips a frame moves with any change of timing (spec 47).

The tree is HEAD with the changes to `gpu.go`, `image.go`, `extra.go`, `gs.go`, `font.go`, `kanji.go`, `fidelity.go` and `nofidelity.go`, three new probes (`imgsync`, `imgzero`, `loadsync`), the test and this file. Nothing is committed or staged. The target's `.S` files are not changed.

### On the console

Run by the spec side on 2026-11-12, Sony's under `-library sony-ourmon`, the clean images under `-library clean`. Outputs are in `.build/hardware/spec49/r1` and `rerun7`.

`loadsync`, in eighths, with the first turn that `DrawSync(1)` gives 0 (-1 for never):

| case | Sony's: first 0, call, `DrawSync(1)` | s49h | s49-base |
|---|---|---|---|
| `LoadImage` 16 by 16 | 0, 186, 29 | 0, 196, 27 | 0, 176, 46 |
| `LoadImage` 24 by 24 | 0, 212, 29 | 0, 223, 27 | 0, 195, 46 |
| `LoadImage` 32 by 32 | 0, 246, 28 | 0, 255, 26 | 0, 231, 46 |
| `LoadImage` 48 by 48 | 24, 245, 82 | 22, 257, 88 | 0, 276, 100 |
| `MoveImage` 16 by 16 | 16, 121, 29 | 16, 122, 28 | 5, 114, 55 |
| `MoveImage` 32 by 32 | -1, 121, 29 | -1, 121, 28 | -1, 113, 54 |
| `ClearImage` fill 64 by 4 | 0, 147, 29 | 0, 150, 26 | 0, 148, 48 |
| `ClearImage` fill 64 by 16 | 0, 147, 28 | 0, 152, 25 | 0, 148, 49 |

The fault is gone: the candidate's first 0 is Sony's within two turns where the image before was far off, and `DrawSync(1)` takes Sony's time where it took near twice that. The model was wrong about `LoadImage`'s call: it is 9 to 12 eighths over Sony's on the console at every size, where the model had it 2 over. `MoveImage` is within 1, the fill 3 to 5 over.

`imgsync`: every `DrawSync(1)` straight after a call gives Sony's answer. The polls to 0 at 64 by 64 and 256 by 256 differ: `LoadImage` 6 and 170 against Sony's 7 and 183, `StoreImage` 9 and 236 against 11 and 250, `MoveImage` 47 and 723 against 39 and 603, `ClearImage` on a rectangle 15 and 287 against 12 and 239, and the fill at 256 by 256 43 against 36. Both ran on the same console, so the clean poll's length differs from Sony's: longer after `LoadImage` and `StoreImage`, shorter after `MoveImage` and `ClearImage`.

The spec 45 set on s49h: `vsyncphase` 274 lines past the blank (one frame 275) and 262 at 240 lines, `qlen` returns, `rerun` seven runs to END. gasgar was not run.

Left for spec 50: `LoadImage`'s 9 to 12 eighths on the console, and the poll's length after each image call.
