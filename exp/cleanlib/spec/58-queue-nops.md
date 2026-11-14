# Taking out queueImageNops

## Why

`queueLoad` and `queueStore` (`exp/cleanlib/gpu.go`) start with four nops, `queueImageNops`, for their size and nothing else. Spec 49 added them because without them the image's packing gave one of the two functions the place of `libps_vsync_wait`, which moved that onto other cache lines and made `VSync(0)` at 240 lines return late under a console's costs. Since spec 52 `libps_vsync_wait` is pinned, and since spec 55 so are `queueLoad` (80010B20) and `queueStore` (800102D0) (`image/pins.txt`). The nops now only cost every queued `LoadImage` and `StoreImage` 8 cycles that Sony's library may not spend.

Like spec 55's `drawPlace` and `packetPlace`, the library should not carry code whose only job was to steer a layout that is now fixed, before the API work starts.

## What to do

1. Take out `queueImageNops` and its two calls.
2. The two sections keep their pinned start addresses and get 16 bytes shorter, so the gaps after them grow, and the packing may place other sections differently. Find out from the clean `image.elf` what moved, and whether anything on the interrupt, VSync or image call paths now shares cache lines it didn't (spec 56 is the latest look at those lines).
3. Run the ten timing tests, the `GEN_SHIFT` sweep for `TestVSyncPoll` and `image/regress.sh`. If a test fails or a call's time moves, find why, and fix it in the library without code whose only job is its size or its place. If the fix wants a new pin or a moved one, report it instead: pins are the spec side's.
4. Compare a queued `LoadImage` and `StoreImage` with Sony's, before and after: `TestQueuedImages` with the costs off, and `probes/qfull` and `probes/qlen` under `PROBE_DRAWTIME=1 PROBE_CPUCOST=1`. If the 8 cycles moved the clean call away from Sony's, pace it with something that does a job (a wait, a check), or report that nothing does.
5. Say what a console run should check. The console is not available until the spec side says so.

## Testing it

The ten timing tests, `TestVSyncPoll` with `GEN_SHIFT` 16, 32, 48, 64, 96 and 128, `image/regress.sh` (38 PASS, 3 TIME, 0 FAIL before), and `qfull`, `qlen`, `imgq` and `imgsync` on both libraries, against spec 57's image, s57c (5b9ee0f1).

## Results

Done 2026-11-13 (implementation side) and 2026-11-14 (the spec side's choice). `queueImageNops` is out with nothing in its place, and nothing breaks without it: the ten tests, the shift sweep and `image/regress.sh` pass. **The image taken is s58a**, `.build/console/libps-s58a.exe.gz`, SHA-1 32e80acc, and the tree builds it byte for byte.

The 8 cycles were not spare: with the costs off Sony's queued `LoadImage` and `StoreImage` take them too, and the clean calls are now 8 short of Sony's. They are left out on purpose (spec side). They are about 1% of the queued call, and the console's VSync interrupt alone varies by about 45 cycles a field, so no program can observe them. A candidate that kept them as a pace, s58c, was measured and not taken. No console run is wanted for this spec.

### What changed

- **`exp/cleanlib/gpu.go`**: `queueImageNops` and its two calls are gone, with nothing in their place.

Nothing else changed: no pin, no test, nothing under `emu/`.

The images, each `gzip -9` of `libps.exe`, all ending at 80066288h:

| | | SHA-1 |
|---|---|---|
| **s58a** | `.build/console/libps-s58a.exe.gz` | **32e80acc** (32e80acce7767bb930d23b91f5bb4e050bb02f38): the nops out, nothing in their place. The tree's, and the image taken |
| s58b | `.build/console/libps-s58b.exe.gz` | 19ce71b6: the 8 cycles as four stores in `LoadImage` and `StoreImage`. Not for use, below |
| s58c | `.build/console/libps-s58c.exe.gz` | 55606482: the 8 cycles as four loads where the nops were (`loads(queueImageLoads)` at the start of `queueLoad` and `queueStore`, 8 cycles in the emulator and some 30 under a console's costs). Measured below, not taken |

### What moved with the nops out (s58a)

From `llvm-nm` on the clean `image.elf`, s57c against s58a (`.build/s58/nmdiff.py`, `nmdata.py`, `lines.py`). The place in the 4 KB cache is the address's low 12 bits.

| | s57c | s58a | in the cache, s57c | s58a |
|---|---|---|---|---|
| `queueLoad` | 80010B20, E8h bytes | 80010B20, D8h | B20 to C07, 15 lines | B20 to BF7, 14 lines |
| `queueStore` | 800102D0, E8h | 800102D0, D8h | 2D0 to 3B7, 15 lines | 2D0 to 3A7, 14 lines |

- The other 23 pinned sections are where they were, and the functions inside `.text.DrawSync` and `.text.MoveImage` too. `queueLoad` and `queueStore` share lines with the same pinned functions as before (`queueLoad` with `ClearImage`, `StoreImage`, `drawNext` and `vblankInterrupt`; `queueStore` with `LoadImage`, `MoveImage`, `putchar`, `ttyPut` and `vblankInterrupt`), one line fewer each.
- The 16 bytes freed after each changed what the packing put there (`strstr` at 80010BF8, a `gso_` label at 800103A8), and with that 56 more functions of 573 have a new address. None of them is on a path: `vsyncpoll`, `imgsync` and `qlen` ran under the costs with `PROBE_WATCH` on all 58 (`.build/s58/watch.sh`), and after the program is entered only `libps_image_start`, `libps_image_setup`, `InitHeap` (twice) and `spuCount` (14 times) run, all before the program's first line. So no function on the interrupt, VSync or image call paths shares a cache line it didn't.
- **The data moved**: 81 of the image's 380 other symbols, among them the VSync count `vsync` (80010CAC to 8001511C), `spuCounter`, `tickReads`, `gs`, `gteOn`, `pads` and the fields of `dispLast`. That is what spec 56 found still moving under a shift (a store to another 1 KB row of RAM costs more), and it shows in `TestVSyncPoll`, below.

s58c has none of this. Its `image.elf` has every symbol at s57c's address with s57c's size, and `libps.exe` differs from s57c's in 16 bytes, the eight instructions.

### A queued call against Sony's, costs off

`qlen`'s bus trace, a warm queued `LoadImage`, cycles after the program's read of timer 2 before the call:

| | Sony's | s57c | s58a | s58c |
|---|---|---|---|---|
| I_MASK read, written 0 | 214, 216 | 214, 216 | 214, 216 | 214, 216 |
| I_MASK read, put back | 508, 510 | 508, 514 | 500, 506 | 508, 514 |
| D2_CHCR read | 530 | 530 | 522 | 530 |
| the timer read after the call | 592 | 592 | 584 | 592 |

`imgsync`'s queued part, marker to marker, cycles:

| | Sony's | s57c | s58a | s58c |
|---|---|---|---|---|
| `LoadImage`, first, second, third | 646, 596, 600 | 644, 596, 600 | 636, 588, 592 | 644, 596, 600 |
| `StoreImage`, first, second | 596, 596 | 596, 596 | 588, 588 | 596, 596 |
| `MoveImage`, `ClearImage`, the fill, `DrawSync(1)` | 756, 634, 658, 138 | the same | the same | the same |

`imgq` (`TestQueuedImages`' probe) has `loadimage` 96 and `storeimage` 94 eighths on Sony's, s57c and s58c, and 95 and 93 on s58a. `qlen` has every queued call at 74 eighths on Sony's, s57c and s58c and 73 on s58a, and `qfull`'s `last` the same.

So the nops were the last 8 cycles of spec 49's pace for the queued call, whatever their comment said, and Sony's library does spend them. None of the ten tests times a queued `LoadImage` or `StoreImage` to the cycle (`TestQueuedImages` allows 1 to 400 eighths, `TestQueueLength` 0 to 200, and `TestImageSync` checks the free calls), which is why s58a passes.

### The same under the costs

`PROBE_DRAWTIME=1 PROBE_CPUCOST=1`. `qlen`, calls 2 to 62 of each run, cycles from the bus trace:

| | Sony's | s57c | s58a | s58c |
|---|---|---|---|---|
| a queued `LoadImage` | 751 | 740 (736 to 743) | 736 (732 to 739) | 766 (762 to 769) |
| I_MASK written 0 | 260 | 274 | 274 | 274 |
| I_MASK read to put it back | 572 | 554 | 550 | 580 |
| D2_CHCR read | 620 | 594 | 590 | 620 |
| from there to the timer read | 131 | 146 | 146 | 146 |
| the 64th call, eighths | 10,897 | 13,081 | 13,113 | 12,877 |

`imgsync`'s queued part, cycles, both rounds alike but where two figures are given:

| | Sony's | s57c | s58a | s58c |
|---|---|---|---|---|
| `LoadImage`, first, second, third | 1,045, 713, 720 | 1,345, 776, 699 | 1,336, 772, 695 | 1,368, 802, 725 |
| `StoreImage`, first, second | 796, 721 | 1,078, 700 | 1,069, 696 | 1,101, 721 |
| `MoveImage`, `ClearImage`, the fill | 985, 815 and 850, 847 | 1,337, 1,223, 919 and 915 | the same | the same |

`qfull` and `imgq`, eighths:

| | Sony's | s57c | s58a | s58c |
|---|---|---|---|---|
| `qfull`: `last`, a queued `LoadImage` | 88 to 89 | 83 to 84 | 83 to 84 | 87 to 88 |
| `qfull`: `all`, the queue's whole time, less Sony's least | 0 to 95 | 1,905 to 2,101 | 1,959 to 2,017 | 1,910 to 2,012 |
| `imgq`: `loadimage`, two runs | 162, 151 | 198, 187 | 197, 186 | 201, 190 |
| `imgq`: `storeimage`, two runs | 144, 154 | 186, 190 | 185, 189 | 189, 193 |

Read together:

- Four nops cost 4 cycles under the costs, so taking them out (s58a) moves the warm queued call 4 further under Sony's: 15 under in `qlen` where s57c is 11 under, 25 under in `imgsync` where it is 21.
- Four loads cost 30. On s58c the warm queued call is 15 over Sony's in `qlen`, 5 over and on it in `imgsync` (725 against 720, 721 against 721), and 1 eighth under in `qfull` where s57c is 5 under. Inside the call, D2_CHCR is read on Sony's cycle (620) and I_MASK 8 after Sony's, where s57c read them 26 and 18 early. The 15 left in `qlen` is from D2_CHCR to the return, 146 cycles against Sony's 131 on all three images. Before it, I_MASK is written 0 14 cycles after Sony's (274 against 260), so with D2_CHCR on Sony's cycle the stretch between the two is 14 shorter than Sony's. Neither end is the queue functions'.
- The first queued call behind a drawing (`imgq`, `imgsync`'s first and second `LoadImage` and first `StoreImage`) is cold and 60 to 300 cycles over Sony's on s57c. The loads add their 26 there too. That gap is not this spec's, and spec 50 found the model's cold figures no better than 60 cycles.
- On the console the warm queued call is under Sony's: `qfull`'s `last` was 122 to 123 on s56b against Sony's 130 (spec 56). So the console wants the call longer, as the loads make it, not shorter.
- The queue's whole time (`qfull`'s `all`, some 84,300 eighths on Sony's) is 2.3 to 2.5% over Sony's under the model on s57c, s58a and s58c alike. `MoveImage`, `ClearImage`, the fill, `DrawSync(1)` and the free calls did not move on either image.

Three loads and a nop would come to some 23 cycles where four loads are 30: nearer in `qlen` (8 over), further in `imgsync` and `qfull`. Not built: the console's figure says more, not less.

### The checks

| | s58a | s58c |
|---|---|---|
| the ten timing tests | pass | pass |
| `TestVSyncPoll`, `GEN_SHIFT` 16, 32, 48, 64, 96, 128 | pass at each | pass at each |
| `image/regress.sh` | 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL | the same |
| the image after the sweep, built without a shift | 32e80acc | 55606482 |

`TestVSyncPoll`'s 240-line returns after the interrupt, costs on, 95 waits a run (`.build/s58/sweep.sh`, `rets.sh`; the logs are `.build/s58/a/vpoll-N.log` and `.build/s58/c/vpoll-N.log`):

| shift | s58c: returns | first wait after the printing | s58a: returns | first wait after the printing |
|---|---|---|---|---|
| 0 | 85 to 133 | in the range | 85 to 133 | 142 |
| 16 | 85 to 133 | in the range | 85 to 133 | 151 |
| 32 | 85 to 133 | 105 | 85 to 133 | 151 |
| 48 | 85 to 133 | 141 | 85 to 133 | 111 |
| 64 | 85 to 133 | 111 | 85 to 133 | 111 |
| 96 | 85 to 133 | 145 | 85 to 133 | 111 |
| 128 | 85 to 133 | 145 | 85 to 133 | 150 |

s58c's column is spec 57's for s57c, wait for wait. On s58a the pass is the same 85 to 133, and the first wait after the printing lands on other phases: the data's move does what a shift does. It reaches 151 against the window's 155, where s57c and s58c reach 145. With the costs off all fourteen runs have the same 95 returns, 38 to 60.

### The two that were not taken

- **s58c**, four loads where the nops were. It has Sony's times with the costs off and is nearer Sony's than s57c under the costs, above, and it moves nothing in the image, since the loads are the nops' size in the nops' place. That is a pad in the two functions again, for 8 cycles no program can observe (above), so the spec side took s58a.
- **s58b**, the 8 cycles as four more stores in the callers' own pads (`loadImageQueuedSpace` and `storeImageQueuedSpace` 10 for 6). `StoreImage` and `LoadImage` grow 16 bytes each, and they lie in the pinned `.text.DrawSync`: `imageEnter` and `LoadImage` move 16 bytes later and `DrawSync` and `drawPoll` 32, two cache lines: `DrawSync` to 734 to 893 and `drawPoll` to 894 to 9EF, which shares 990 to 9EF with `drawNext` where it shared 990 to 9CF. 105 functions and 85 data symbols have a new address, `vsync` among them as on s58a. Keeping those where the console confirmed them wants `.text.DrawSync`'s pin moved, as spec 55's did, which is the spec side's. Built for its layout only, no probe run on it.

### What s58a costs

- The queued `LoadImage` and `StoreImage` are 8 cycles short of Sony's with the costs off (584 against 592), where s57c was on Sony's cycle at every access but the write that puts I_MASK back, and 4 further under Sony's under the costs. Accepted, above.
- The data the interrupt path stores to is at new addresses, and `TestVSyncPoll`'s first wait after the printing has 4 cycles of room at two shifts where s57c has 10.

### The console

No console run is wanted for this spec (spec side, 2026-11-14).

### Left open

- **The warm queued call on the console**, 7 eighths under Sony's on s56b (`qfull`'s `last`, 122 to 123 against 130, spec 56), and half an eighth more by the model now. The model has the call's end after the read of D2_CHCR going the other way (146 cycles against 131), so the console and the model disagree on this call by some 70 cycles, and only the console's figures should pace it.
- **The first queued call behind a drawing**, cold: 300 cycles over Sony's for `LoadImage` and 280 for `StoreImage` under the model (`imgsync`, `imgq`), on s57c as on s58a. No console figure for it yet: `imgq` on the console would give one.
- **The queue's drain under the model.** `qlen`'s 64th call waits 13,081 eighths on s57c against Sony's 10,897, where spec 55 had 10,451 and 10,415 before its pins, and the 64 calls together take 18,993 against 16,852. Spec 55 left the drain open, and the model's figure has grown since. `qfull`'s `all` is 2.3 to 2.5% over under the model and was 1.2 to 1.6% on the console (spec 56).
- **I_MASK put back 6 cycles after its read** on the queued path with the costs off (508 and 514), where Sony's is 2 (508 and 510). On s57c too, and not looked at here.
- The scratch tools are in `.build/s58` of this worktree: `probes.sh` and `probesc.sh` (the four probes, both sides or the clean one), `sum.sh`, `qcall.py`, `qmean.py` and `qmarks.py` (the tables above from the TTY and bus traces), `nmdiff.py`, `nmdata.py`, `lines.py`, `moved.py` and `watch.sh` (the layout), `sweep.sh` and `rets.sh` (the shifts). The outputs are in `base`, `a` and `c` there.
