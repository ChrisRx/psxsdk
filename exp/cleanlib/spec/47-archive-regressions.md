# Two archive programs that spec 45 made worse

## Why

Spec 45's verify chain (its Results, "Verify chain") ran the web archive's original programs on both libraries before and after its four parts. Two got worse, and neither was looked into:

- `scee-topten-cardem` first differs from Sony's at picture change 66 (frame 75). Before, it held until change 167 (frame 413).
- `test-gr-01` makes 360 picture changes. Sony's makes 375, and the clean library made 364 before.

Each is a place where a real program now behaves less like it does on Sony's library. Spec 45's library work went in as one commit, f807cb8 ("blank timing, queue wait and what a run leaves"). The verify58 baseline was the tree just before it, bbba586. Specs 45 (part 1) and 46 have changed the library since.

## What to do

For each program:

1. Run it on the current tree with exp/cleanlib/image/archive.sh, giving its name, and with FIRST=1 for where it first differs and the GPU words there. Say whether it is still worse than at bbba586.
2. Find which change made it worse. f807cb8 holds several parts (the blank timing, the queue wait with `imageBlocksWait` and `imageBlank`, what a run leaves in DPCR and D2_CHCR), so bisect within it as well as across commits: rebuild the image with one part at a time backed out.
3. Find what the program does there that the change affects: which library calls, and what it waits on.
4. Fix the library so the program behaves as it does on Sony's, without undoing what spec 45's part fixed. Its tests (TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase and the others in Results) must still pass on Sony's discs and the new image.

If one program's difference turns out not to be the library's, as `car`'s load size was in spec 45, say what it is and show it.

## Testing it

The archive runs under the emulator against Sony's image with the clean font, as archive.sh does. Compare `first.txt` and the summary line against the baseline at bbba586. A console run can confirm a fix: the spec side runs it, with `-library sony-ourmon` for Sony's side.

## Results

Done 2026-11-11 in the emulator, with the scph1001 BIOS. Not tried on a console.

### Summary

| | verify58 (before f807cb8) | verify59 (f807cb8) | HEAD (ff1ed9f) | with this spec's changes |
|---|---|---|---|---|
| `scee-topten-cardem`, first difference | change 167, frame 413 | change 66, frame 75 | change 50, frame 59 | change 167, frame 413 |
| `test-gr-01`, picture changes, Sony's / clean | 375 / 364 | 375 / 360 | 375 / 364 | 375 / 364 |

The first three columns were reproduced here with today's emulator by building each library (`bbba586`, `f807cb8`, `bc7e097`, HEAD) into the image, so the emulator's changes since play no part in these two lines.

Both programs are back where they were before spec 45, and neither is the same as on Sony's. What is left in each is older than spec 45 and is listed under "Not done".

### `scee-topten-cardem`

**What the program does.** It never waits for the vertical blank. Its loop is `GsSwapDispBuff`, `FntFlush(0)` with nothing printed, `GsClearOt` twice, 265 `GsSortFastSprite`, `GsSortClear`, `GsDrawOt` twice, `DrawSync(0)` and `VSync(1)`, about 97,450 cycles a turn, six turns to a PAL frame, and what a turn draws depends on how many turns have run. So the picture at each blank is the one whose swap came last before it, and the clean library shows a different picture as soon as a swap falls on the other side of a blank than Sony's does. Every cycle a call takes more or less than Sony's adds up from the program's start. "First differs at change N" is where the sum first carries a swap across a blank. It is not a measure of how close the library is: an error can cancel another, and did.

`TestArchiveCalls` gives each call's time (to the next call) on both images. In the loop:

| cycles | Sony's | bbba586 | f807cb8 | HEAD | now |
|---|---|---|---|---|---|
| `FntFlush(0)`, empty | 530 | 534 | 534 | 534 | 530 |
| `GsSwapDispBuff`, by buffer | 4,776 | 4,780 / 4,772 | the same | the same | the same |
| `GsDrawOt`, the first after a blank | 390 | 384 | 384 | 384 | 390 |
| `GsDrawOt`, any other | 390 | 390 | 390 | 390 | 390 |
| `DrawSync(0)` | 198 | 198 | 200 | 200 | 198 |
| a vertical blank interrupt, in `GsSortFastSprite` | 15,544 | 15,532 | 15,550 | 15,554 | 15,544 |
| sum over a frame, against Sony's | | +6 | +36 | +40 | 0 |

(The interrupt is the call it lands in less the call's 306. Before the program's sound starts it is 15,112 on Sony's, and the clean one differs from it by the same cycles in each column.)

**Which change did it.** Two things in f807cb8, both on this loop:

- The interrupt handler. Part 4 moved the vertical blank's work out of `libps_interrupt` into `vblankInterrupt` and gave the handler pads at its start and end to "keep its time", and part 1 added its pads before the acknowledgment and the count. They kept the handler's time with the drawing queue empty, which is what `vsynccount` and the later probes measure. This program's blanks find the queue in another state: the last request (`GsSwapDispBuff`'s second `PutDrawEnv`, or `FntFlush`'s list) has gone out, and no call has looked at the queue since. There the clean handler retired the request (`drawDone` calling `drawNext`), a path the pads were not measured on: 12 cycles under Sony's before f807cb8, 6 over after it, 10 over from bc7e097 on. Sony's handler takes 15,544 in that state, the same as with the queue empty (spec 46's figure for `vsyncpoll`).
- `DrawSync(1)`'s count (part 2). `queued() - queue.running` returned from a branch of its own made the compiler put that return just before the function's end, and mode 0's way out now jumps over it: 2 cycles more in every `DrawSync(0)`. Spec 45 said the mode 0 path was "the same instructions as before, jump targets apart". One of them is a new jump.

Checked by backing parts out of f807cb8 (`TestArchive` on the two programs each time):

| library | cardem | gr-01 |
|---|---|---|
| bbba586 | change 167, frame 413 | 364 |
| f807cb8 | change 66, frame 75 | 360 |
| f807cb8, `DrawSync(1)` counting as before | change 66, frame 75 | 360 |
| f807cb8, that and the handler 18 cycles shorter (`interruptSpaceIn` 0, `interruptSpaceOut` 1) | change 167, frame 413 | 366 |
| f807cb8, the blank acknowledged and counted early again (`vblankAckSpace` and `vblankCountSpace` 0, the tick's `space(24)`) | change 66, frame 75 | 362 |
| bc7e097 | change 50, frame 59 | 364 |

So for cardem it is the handler's length in that state, with `DrawSync(0)`'s 2 cycles adding to it. `imageBlocksWait`, `imageBlank`, DPCR and D2_CHCR play no part: the program queues no image behind a drawing and never calls `ResetGraph(1)`.

The baseline's frame 413 was two errors cancelling: the handler 12 cycles short and `GsDrawOt` 6 short after each blank against `FntFlush` 4 long six times a frame, +6 a frame in all.

### `test-gr-01`

**What the program does.** It waits with `VSync(0)` every turn and both libraries run it at one turn a frame for all 1,200 frames (1,187 `GsSwapDispBuff` on each, in the same frames). The pictures differ only in two numbers it prints, "CPU" and "GPU": what `VSync(1)` returns before and after `DrawSync(0)`, the lines since the last `VSync(0)` returned. They sit near a line's edge (113 or 114 lines a turn), so a few hundred cycles decide which is printed, and "picture changes" counts how often the printed numbers change from one frame to the next. 125 of the 2,376 `VSync(1)` results in 1,200 frames differ by one line between the libraries.

**Which change did it.** The 360 was the handler's timing in f807cb8 again, through when `VSync(0)` returns: every variant of the handler in the table above gives another count (360, 362, 366). bc7e097 ("VSync reads and waits at Sony's cycles") put the wait's end at Sony's cycle and the count went back to 364. It is 364 at HEAD and with this spec's changes: no longer worse than the baseline.

The count is not a measure either. Sony's 375 against 364 is the jitter of a line count, and it would take the same cycles in every frame to match. What keeps it apart is under "Not done".

### The fix

`probes/qstate` (new, `spec/probes/qstate`) makes each call that goes through the drawing queue in two states, between GP1 markers: after a `DrawSync(0)` (state 0), and after a `PutDrawEnv` that nothing has looked at since (state 1, the state cardem's blanks and its `FntFlush` find). Build it with `PROBE_BUS=start exp/cleanlib/image/probe.sh NAME exp/cleanlib/spec/probes/qstate`. The marker before case c in state s of round r is r*128 + (c*2+s)*2. Cycles from marker to marker, the probe's own overhead included, the same in each round:

| call | Sony's, state 0 and 1 | before, state 0 / 1 | now, state 0 and 1 |
|---|---|---|---|
| `GsDrawOt` | 400 | 394 / 400 | 400 |
| `DrawSync(0)` | 216 | 210 / 216 | 216 |
| `FntFlush(0)`, empty | 536 | 534 / 540 | 536 |
| `FntFlush(-1)`, empty | 548 | 532 / 538 | 534 |
| `FntFlush(0)`, "hi" | 816 | 856 / 862 | 862 |
| `FntFlush(-1)`, "hi" | 828 | 854 / 860 | 860 |
| `PutDrawEnv` | 1,952 | 1,950 / 1,956 | 1,952 |
| `GsSwapDispBuff` (the states alternate the buffer too) | 4,780 | 4,786 / 4,788 | 4,786 / 4,782 |
| `LoadImage` 16 by 16 | 702 | 704 / 710 | 702 |
| `LoadImage` 16 by 1 | 798 | 798 / 804 | 798 |
| `LoadImage` 20 by 2 | 768 | 766 / 772 | 766 |
| `LoadImage` 0 by 4 | 592 | 578 / 584 | 594 |
| `StoreImage` 16 by 16 | 714 | 744 / 750 | 714 |
| `StoreImage` 16 by 1 | 810 | 844 / 850 | 810 |
| `StoreImage` 20 by 2 | 780 | 810 / 816 | 780 |
| `StoreImage` 0 by 4 | 588 | not measured | 588 |
| `MoveImage` | 474 | 490 / 496 | 496 |
| `ClearImage`, a rectangle | 744 | 774 / 780 | 744 |
| `ClearImage`, a fill (64 wide) | 602 | 670 / 676 | 676 |

"Before" is HEAD with the first two changes below, which is when the probe was written: HEAD's own `DrawSync(0)` is 2 more in both states. Sony's calls take the same time in both states, every one. The clean library took 6 cycles more in state 1: `drawNext` retired the request that was running behind a branch. The pads of spec 08 were set on `drawq` and `fntchars`, which make some calls right after a `DrawSync(0)` and others right after another queued call, so each pad had one state or the other built in.

What changed (`gpu.go`, `kernel.go`, `gs.go`, `extra.go`):

- **The handler leaves a finished request alone** (`handlerDrawDone`, in place of `drawDone` in `libps_interrupt`). With no request waiting behind the one running, the handler does nothing to the queue and spends the same nops as with the queue empty (21 where `drawDone` has 24: `requestNext` takes the other 6 cycles). The next call that looks at the queue retires the request, as it does when no interrupt came between: `enqueue`, `drain`, `imageEnter` and `pollQueue` each do already, since the channel's completion does not interrupt with nothing waiting. With a request waiting it does what `drawDone` does. `pollQueue` (`DrawSync(1)`, a full queue, the font's wait) still uses `drawDone`. The interrupt is 15,544 in cardem's state, Sony's, and `GsDrawOt` after a blank 390.
- **`DrawSync`'s other modes** compute the count before the test (`n == 0 && !gpuIdle()` returns 1, else `n`), so that mode 0's `return 0` falls into the function's end: 198 in the loop, Sony's. `DrawSync(1)` returning 0 or a count is 2 cycles longer than at HEAD and 2 shorter with the GPU busy. Its time against Sony's is not known to me: earlier call lists in the main checkout (`.build/carchive/calls`, `scee-topten-second` and `event-dengeki001-nanatan-nanatan`, of an older image) have a turn of a `DrawSync(1)` poll at 114 and 124 cycles on Sony's and 228 and 248 on the clean library. Not looked into.
- **`drawNext` retires without a branch**: `queue.head += queue.running`, with stores to `queue.running` that bring it to the time the branch's retiring path took (three `spaceAt`, the compiler dropping the plain store before them as dead; a build without the fidelity tag has the plain store alone). State 1's times are what they were and state 0's are 6 longer.
- **The pads that had state 0 built in** come down by what the probe shows: `PutDrawEnv` by 4 (`putDrawEnvNops`), `LoadImage` by 8 (`loadImageNops`) with its size pads set again on the four sizes above (`loadImageBlockSpace` 7, `loadImageWholeSpace` 1, `loadImageNoBlockSpace` 9, `loadImageNoSizeSpace` 11), `StoreImage` by 36 (`storeImageTurns`, `storeImageNoBlockSpace` 6, `storeImageNoSizeSpace` 0), `ClearImage`'s rectangle by 34 (its pace and `clearImageSpace` are gone), `FntFlush`'s empty flush by 4 (`fntNoGlyphSpace` 4, from 6). `GsSwapDispBuff` has one nop more (`swapDispBuffNops` 3), which leaves it where it was in both programs. The calls an image call makes when it goes into the queue instead (`queueLoad`, `queueStore`) keep their pads.

`TestQueuedImages`, `TestQueueLength`, `TestFieldTime` and `TestDrawWait` pass, below, so the queue's behavior with the drawing time on is as it was.

### Not done, and found in passing

All of these are older than spec 45. Each is a time that is not Sony's, with the figure.

- **cardem's start-up.** By its first swap the clean library is some 250 cycles behind Sony's (136 before this spec, when `LoadImage` and `DrawSync` errors cancelled part of it): `init` +230 over its 4.5 million cycles, `InitHeap` -14, `ExitCriticalSection` +18, `SetVideoMode` +14, `SsSetTickMode` +10, `ResetGraph(0)` -22, `SsVabTransfer` +64, `GsInitGraph` -28, `GsDefDispBuff` -84, `GsInit3D` +164, `FntLoad` -168, `FntOpen` -132, `GsGetTimInfo` +8 each of 24 (it has no pad to take from), `GsMapModelingData` +38 once, `GsSetLightMode` -20. `test-gr-01` has the same figures for the calls it shares (`init` +248, `GsInit3D` +166, `FntLoad` -168, `FntOpen` -132, `GsDefDispBuff` -80, `GsInitGraph` -10), so they are the calls' own, not where an interrupt fell. That offset, constant now that the loop takes Sony's time, is what still carries a swap across the blank at frame 413. A spec for the start-up calls' times would be the next step for this program.
- **`GsSwapDispBuff` by buffer.** 4,780 and 4,772 on a PAL display 320 wide where Sony's takes 4,776 for both, and 4,774 and 4,770 against 4,768 on an NTSC one 256 wide (4,786 and 4,782 against 4,780 at 320 wide): the clean one's time depends on which buffer it shows, by 8 on PAL and 4 on NTSC, and Sony's does not. It sums to nothing over two turns in cardem.
- **`FntFlush` with an id that is not open.** Sony's takes 12 cycles more for `FntFlush(-1)` than for `FntFlush(0)`, the clean one 2 less: 14 short on an empty flush. Not changed, because:
- **`FntFlush` with text** is 46 cycles over Sony's for an open id whatever the text's length (`test-gr-01`'s 13 lines and the probe's "hi" alike), and 32 over for -1, where the 14 above hide part of it. The path with glyphs has no pad to take 46 from (`fntGlyphsSpace` is 0), so it wants the loop's set-up looked at. Giving -1 its 14 cycles first would have made the commonest call, `FntFlush(-1)` with text, worse.
- **`MoveImage`** 22 over (it has no pad), **`ClearImage`'s fill** 74 over at 64 by 16. Spec 08 had the fill at Sony's 586 on `drawq`; one size is not enough to set its pad again.
- **`LoadImage`** 20 by 2 is 2 under and 0 by 4 is 2 over: a store more or less moves each by 4 or 6 where it falls.
- **`test-gr-01`'s first reading.** Its first `VSync(1)`, before any `VSync(0)`, returns 3,144 lines on Sony's and 3,146 on the clean library (the "CPU" and "GPU" of its first picture, change 2 at frame 15). The program's start-up is about 300 cycles behind by then, not two lines, so the count starts two lines apart: where the line counter starts, or what Sony's takes as the last wait's return before there has been one.
- **`test-gr-01`'s sequence.** In about one frame in eight the clean `VSync(0)` returns 1,800 to 5,300 cycles sooner than Sony's: the program plays a sequence, and the clean tick's time for its events is not Sony's. The rest of its frames run about 300 cycles behind.
- **A system call** (`ExitCriticalSection`) takes 18 cycles more on the clean library in both programs: the handler's way through with nothing raised.
- `malloc` is 22 over and `GsMapModelingData` 6 over on a model already mapped (114 over on its first call), `GsSetRefView2` 6 under, `RotMatrix` 2 under, `calloc` 2 over (`test-gr-01`, every turn).

### Testing

On the image with these changes (`.build/console/libps-s47a.exe.gz`, gzip of the image whose SHA-1 starts 69948ab0), scph1001 BIOS:

- `go test ./emu/...` and `./exp/cleanlib/...` pass.
- `exp/cleanlib/image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before.
- `TestDrawWait`, `TestFieldTime`, `TestQueuedImages`, `TestDMALeft`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps`, `TestVSyncPoll` and `TestQueueLength` pass on Sony's discs and on the new image's, each built with `probe.sh` (`.build/s47NAME-cimg` and `-cimg-clean`).
- `archive.sh` on the two programs: the summary table above.
- The full `archive.sh`, against HEAD's image run the same day on the same emulator (`.build/s47/summary-head.txt`, and `.build/s47/v/summary.txt` for this image):

  | | same | differ | still | shift | wild | tty | stall | noboot |
  |---|---|---|---|---|---|---|---|---|
  | verify59 | 96 | 19 | 14 | 7 | 1 | 1 | 1 | 1 |
  | HEAD | 97 | 19 | 13 | 7 | 1 | 1 | 1 | 1 |
  | now | 98 | 19 | 13 | 7 | 0 | 1 | 1 | 1 |

  Four lines differ from HEAD's. `scee-topten-cardem`: change 167, frame 413, from change 50, frame 59. `pssoft-kyuri-2-kyuri` is now the same: the wild address it reads, which follows its timing, is Sony's 5430EFCCh (5430EFC8h at HEAD). `test-pd961207` first shifts at change 272 (frame 530), from 171 (frame 353). `test-zero-zero` first shifts at change 561 (frame 932), from 562 (frame 944): one change sooner, and where verify58 had it. That program times itself with the root counters (`GetRCnt` and `ResetRCnt` in loops), and its calls differ between the libraries from its start (`CdSearchFile` alone is 256 cycles shorter on the clean library), so where it first slips a frame moves with any change of timing. Not looked into further.

  Against verify59, the lines that differ beside those are the emulator's and the later specs' doing, the same at HEAD: `pssoft-madoka-madoka` (still to same), `pssoft-hasire86-2-mh` and `showcase-funky` (the same number of changes on both sides, another number than before), `event-event001-rave096` (its TTY 9 bytes longer than Sony's, from 20), and `test-gr-01` at 364.
- `probes/qstate`: the table above.

The tree is HEAD with the changes to `gpu.go`, `kernel.go`, `gs.go` and `extra.go`, the new probe and this file. Nothing is committed or staged. The target's `.S` files are not changed. The bisecting scripts and their outputs are in `.build/s47/`.

### A console run

None is needed to accept this: Sony's side of every figure above is Sony's own code run in the emulator with the costs off, and the changes are to cycles the costs-off emulator counts exactly. One would be worth making before the image is used on a console for timing: 227 of the image's 933 symbols moved (`drawNext` among them, to 800274E4h; `libps_interrupt`, `VSync`, `libps_wait_blanks` and the sound tick did not), and a console charges for where code lies. The spec 45 set on `.build/console/libps-s47a.exe.gz` with `-library clean`: `vsyncphase` (274 lines past the blank, 262 at 240 lines), `qlen` (every call returns), `rerun` (seven runs to END), and `irqsplit` against spec 46's figures (the handler's parts should be within its noise, since the handler's code is where it was and its queue check is the same length).
