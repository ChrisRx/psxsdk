# Taking out drawPlace and packetPlace

## Why

`drawPlace` and `packetPlace` (`exp/cleanlib/gpu.go`) do nothing. Spec 50 added them to steer the image's packing: `drawPlace`'s size in `.text.DrawSync` was picked so that TestVSyncPoll passes, and `packetPlace`'s in `.text.MoveImage` so that MoveImage, ClearImage and packetEnter share no cache line with DrawSync, drawPoll and the watch's mark. Spec 52 pinned both sections, and the interrupt and VSync paths, at fixed addresses (`image/pins.txt`), so the packing no longer moves them. Both functions are called on `packetRoomWait`'s path, where the queue was full, which costs that path time Sony's library may not spend.

Before the API work starts, the library should not carry code whose only job was to steer a layout that is now fixed.

## What to do

1. Take out `drawPlace`, `packetPlace`, their constants and their calls.
2. Their sections keep their pinned start addresses, but whatever followed them within a section moves up. Find out from the clean `image.elf` (its symbols are the clean library's, CLEANROOM.md) whether anything in `.text.DrawSync` or `.text.MoveImage` moves, and where it then lies in the 4 KB instruction cache against what spec 50 wanted kept apart.
3. Run the ten timing tests and `image/regress.sh`. If TestVSyncPoll or TestImageSync fails, or a call's time moves, find why, and fix it in the library without a function whose only job is its size. Report it if that can't be done.
4. Compare the time of `packetRoomWait`'s path (MoveImage or ClearImage with the queue full, `probes/qlen` and TestQueueLength) with Sony's, before and after. Pace it if it moved away from Sony's.
5. Say what a console run should check.

## Testing it

The ten timing tests (TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll, TestQueueLength, TestImageSync), `image/regress.sh` (38 PASS, 3 TIME, 0 FAIL before), and `imgsync`, `imgparts` and `qlen` under `PROBE_DRAWTIME=1 PROBE_CPUCOST=1` against spec 54's image (1a6df872).

## Results

Done 2026-11-13 (implementation side), as far as the library goes. The two functions are out, every test passes, and no call's accesses moved with the costs off. Under a console's costs three times moved. One is nearer Sony's than before. The other two want a pin each, which is the spec side's, so they are reported here and not fixed.

### What changed

- **`exp/cleanlib/gpu.go`**: `drawPlace`, `packetPlace`, `drawPlaceNops`, `packetPlaceNops` and the two calls in `packetRoomWait` are gone. Nothing else in the library changed.
- **`probes/qfull`** (new): `MoveImage` and `ClearImage` called with the queue full, since `qlen` fills the queue with `LoadImage`, which never reaches `packetRoomWait`. It fills the queue as `qlen` does, makes the one call between GP1 markers for a bus trace, and prints the call's time and the queue's whole time by timers 2 and 0.

The image is `.build/console/libps-s55a.exe.gz`, SHA-1 cd7c1fac, and the tree builds it byte for byte. It ends at 80066288h as before.

### What moved in the pinned sections

From `llvm-nm` on the clean `image.elf`, before (1a6df872) and after. The place in the 4 KB cache is the address's low 12 bits.

| | before | after | in the cache, before | after |
|---|---|---|---|---|
| `.text.DrawSync`: `mark` | 80017AA4 | same | AA4 to AD7 | same |
| `StoreImage` | 80017AD8 | same | AD8 to FCB | same |
| `imageEnter` | 80017FCC | same | FCC to 27B | same |
| `drawPlace` | 8001827C | gone | 27C to 2C3 | |
| `LoadImage` | 800182C4 | 8001827C | 2C4 to 713 | 27C to 6CB |
| `DrawSync` | 80018714 | 800186CC | 714 to 873 | 6CC to 82B |
| `drawPoll` | 80018874 | 8001882C | 874 to 9CF | 82C to 987 |
| `.text.MoveImage`: `ClearImage` | 800139DC | same | 9DC to F2F | same |
| `packetEnter` | 80013F30 | same | F30 to 0EF | same |
| `packetPlace` | 800140F0 | gone | 0F0 to 1D7 | |
| `MoveImage` | 800141D8 | 800140F0 | 1D8 to 6F3 | 0F0 to 60B |

So `LoadImage`, `DrawSync` and `drawPoll` moved up 72 bytes and `MoveImage` 232, each four and a half or fourteen and a half lines: the same code now starts 8 bytes later in its lines. `.text.DrawSync` is 3,812 bytes where it was 3,884, and `.text.MoveImage` 3,120 where it was 3,352. The other sixteen pinned sections are where they were.

Against what spec 50 wanted kept apart (`.build/s55/lines.py` on the `llvm-nm` output):

- `MoveImage`, `ClearImage` and `packetEnter` still share no line with `DrawSync` or `drawPoll`. `DrawSync` and `drawPoll` end at 987 and `ClearImage` starts at 9DC, so 84 bytes are free between them now, and `MoveImage` ends at 60B, 192 bytes before `DrawSync`.
- `ClearImage` shares `mark`'s four lines (AA0 to AD0), before and after. Neither moved, so 1a6df872 had that too.
- **New: `MoveImage` lies on all nine lines of `drawPollMove`** (pinned at 8004114C, 14C to 1CB). `packetPlace`'s 232 bytes were exactly what kept `MoveImage` off them.

What followed the two sections in their gaps moved too, and with the 320 bytes freed the packing gave 95 of 575 functions a new address. Six of them are on the image calls' paths and are not pinned:

| | before | after | shares lines with, after |
|---|---|---|---|
| `queueLoad` | 8003E8AC | 8003F3F0 | `LoadImage`, 15 lines (before: `drawPoll`) |
| `queueStore` | 8003E754 | 8003E8AC | `drawPoll` (before: `DrawSync`) |
| `packetMoved` | 80040604 | 80040DA4 | `ClearImage`, `StoreImage` |
| `packetRoomWait` | 80043E08 | 80043F00 | `ClearImage`, `packetEnter`, `StoreImage` |
| `imageRefused` | 800453C8 | 80044020 | `imageEnter` |
| `imageTurn` | 80048178 | 80048050 | `imageEnter`, `packetEnter` |

### The tests

- The ten timing tests pass (TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll, TestQueueLength and TestImageSync).
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL, as before.

TestImageSync runs with the costs off, so every access `imgsync` traces is on Sony's cycle as before. What follows is under `PROBE_DRAWTIME=1 PROBE_CPUCOST=1`.

### Times that moved under the costs, and why

`imgparts`, warm, least of eight, cycles, and where the call masks:

| | Sony's | before | after |
|---|---|---|---|
| `LoadImage` 0 by 4 | 778 | 772 | 746 |
| the CPU's words (16 by 1) | 1,068 | 1,016 | 996 |
| 1 block (32 by 1) | 959 | 925 | 905 |
| 8 blocks (16 by 16) | 1,102 | 1,049 | 1,029 |
| 24 blocks and more | 1,328 | 1,273 | 1,253 |
| `LoadImage` masks at | 295 | 331 | 293 |
| `LoadImage` 0 by 4, cold | 1,140 | 1,154 | 1,118 |
| `MoveImage` | 639 | 648 | 693 |
| `MoveImage`, cold | 972 | 824 | 854 |
| `MoveImage` masks at | 351 | 422 | 390 |
| `StoreImage`, `ClearImage`, the fill | | no change | |
| 24 polls idle, behind `LoadImage`, `StoreImage`, `MoveImage` | 3,596, 9,209, 6,335, 3,675 | 3,741, 8,583, 6,703, 3,806 | 3,737, 8,556, 6,737, 3,820 |

`imgsync` under the costs, second round, cycles between its markers:

| | Sony's | before | after |
|---|---|---|---|
| `LoadImage` 16 by 16, 0 by 4, 64 by 64 | 1,487, 1,084, 1,728 | 1,587, 1,105, 1,763 | 1,550, 1,045, 1,679 |
| `MoveImage` 16 by 1 | 921 | 769 | 723 |
| `ClearImage`, the fill, `StoreImage` | | no change | |
| `DrawSync(1)` after a call, GPU done | 220 | 244 | 249 |
| `DrawSync(1)` behind `MoveImage` or `ClearImage` 64 by 64 | 222, 245 | 242 | 259 |
| queued behind a drawing: second and third `LoadImage` | 713, 720 | 768, 701 | 951, 870 |
| queued: first `StoreImage` | 796 | 1,030 | 1,077 |
| polls to 0 at 256 by 256: Load, Store, Move, Clear, Fill | 154, 306, 461, 461, 461 | 166, 272, 439, 440, 439 | 167, 272, 439, 439, 439 |
| polls to 0 behind the queued calls | 429 | 422 | 419 |

`qlen`: a queued `LoadImage` takes 94 eighths on Sony's, 104 before and 111 after. The 64th call waits 10,897, 10,451 and 10,415.

Three causes, each read from the clean image's own symbols and disassembly:

1. **`LoadImage` and `spin`** (20 to 38 cycles off the free call). `spin` is pinned at 80043368, lines 360 to 390. Before, those were the lines of `LoadImage` that hold its call of `imageEnter` and what it returns to. `imageEnter` calls `spin`, so each call fetched them again on the way back, before it masked. Now `spin`'s lines fall on `LoadImage`'s branch for a queued request, which a free call skips. That is the 38 cycles the mask moved, to 2 from Sony's, and the 20 to 26 off the warm call. The warm figures after are spec 50's for s50d under the model to the cycle (746, 996, 905, 1,029, 1,253), the image the console measured within 2.5% of Sony's. The image that merged specs 50 and 51 had packed `LoadImage` onto `spin`'s lines, the console's `loadsync` showed it as 2 to 3 eighths at 16 and 24 square (186 and 212 on s50d, 189 and 214 in spec 52's table), and the pins kept it. So this one moved back, and toward Sony's: expect `loadsync`'s `LoadImage` near s50d's 186, 212, 228, 231.
2. **`MoveImage` and `drawPollMove`** (45 cycles on a warm `MoveImage`, 17 on the poll behind it). They share nine lines now, above. `DrawSync(1)` with the GPU busy runs `drawPollMove`, and the next `MoveImage` fetches its first nine lines again, and the other way round. Under `imgsync`, whose calls come after a `VSync(0)`, the same call is 46 cycles shorter than before, so which way a console sees it move depends on what ran in between. Before it was 9 over Sony's warm, and it is 54 over now.
3. **`queueLoad` on `LoadImage`'s lines** (56 cycles a queued call in `qlen`, 170 to 180 in `imgsync`). The packing gave `queueLoad` lines 3F0 to 4D7, inside `LoadImage`. `LoadImage` calls it for every queued request, so each call fetches both again.

The small moves in `DrawSync(1)` (244 to 249) and the polls are the 8 bytes' shift in where `DrawSync`'s and `drawPoll`'s code starts in its lines, and the phase of a block.

### What was tried, and where it stops

Nothing in the library moves `MoveImage` off `drawPollMove`'s lines or `queueLoad` off `LoadImage`'s without a function made for its size or a pin:

- A further image, s55b (`.build/console/libps-s55b.exe.gz`, SHA-1 878eae03, not for use), has `queueLoad` in `.text.DrawSync` (`//go:section`), as spec 50 grouped the others. It passes the ten tests, and a queued `LoadImage` came out at 87 to 88 eighths in `qlen` (Sony's 94) and 810 and 729 cycles in `imgsync` (Sony's 713 and 720): so cause 3 is `queueLoad`'s place and little else. But the compiler put it between `LoadImage` and `DrawSync`, which moved `drawPoll` to 914 to A6F, onto nine lines of `ClearImage`. The order inside a section is the compiler's, and the section has 284 bytes left under 4 KB, so there is no place in it for `queueLoad` (232 bytes) that leaves the rest alone. Dropped, and the tree is s55a's.
- The lines free of `LoadImage`, `imageEnter`, `mark`, `DrawSync`, `drawPoll` and `ClearImage` are 988 to 9DB, 84 bytes. `queueLoad` fits lines 988 to A6F if `ClearImage`'s first lines may be shared, or it is pinned where only `StoreImage`'s lines are under it.
- `drawPollMove` is 128 bytes. Clear of `MoveImage`, `ClearImage`, `packetEnter`, `DrawSync`, `drawPoll`, `mark` and `pollQueue` are lines 60C to 6CB only (the end of `LoadImage`).

Both are stop points of this spec (a new pin, a moved pin), so they are left for the spec side.

### `packetRoomWait`'s path against Sony's

`qfull`, bus traces, both libraries. The call waits while the 63 queued images go out, which on both libraries happens in one chain of interrupts, and then makes its own request with the GPU free: `DrawSync(1)` after it gives 0 on both. The call takes 625,000 to 635,000 cycles, so its end is timed from the last write of I_STAT in the call (the chain's last acknowledge) to the start of the channel for the call's own packet:

| | Sony's | before | after |
|---|---|---|---|
| `MoveImage`, first round, later rounds | 1,321, 1,288 | 1,676, 1,644 | 1,482, 1,459 |
| `ClearImage`, first round, later rounds | 1,469, 1,469 | 2,032, 1,997 | 1,667, 1,632 |
| from that start to the marker after the call, Move and Clear | 327, 393 | 320, 379 to 387 | 314, 387 |

So the two functions cost this path 185 to 365 cycles: 72 nops in 19 lines that are never in the cache after the chain. The path was 355 to 560 cycles over Sony's and is 160 to 200 over now. It moved toward Sony's, so nothing was paced.

How far to trust it: an earlier build of the same probe, with its code elsewhere, had Sony's `ClearImage` at 1,881 for the same span where this one has 1,469, and the clean library's at 1,667 and 1,632 in both. After a chain of 63 interrupts everything is cold, and spec 50 found the model's cold figures no better than 60 cycles. Only the console can say what is left.

The queue's whole time, from `GsDrawOt`'s return to the call's, does not depend on the calls that fill it: Sony's 674,900 cycles (Move rounds) and 675,000 (Clear rounds), before 680,300 and 680,800, after 680,100 and 682,600. So the clean library takes 0.8 to 1.1% longer to send 63 queued images, before and after. The trace shows where: from one image's blocks starting to the next's, Sony's takes 837 cycles and the clean library 918, and the clean library sends a list from the request (D2_MADR in the queue, D2_CHCR 01000401h) after each image's blocks, which Sony's does not. That is not this spec's, and TestQueuedImages passes on it. Why the Clear rounds gained 1,800 cycles after is not found.

### The pins (spec side, 2026-11-13)

`pins.txt` now has:

- **`.text.DrawSync` at 80017AECh and `.text.MoveImage` at 80013AC4h**, 72 and 232 bytes later than before: the sizes of `drawPlace` and `packetPlace`, so that `DrawSync`, `drawPoll`, `LoadImage` and `MoveImage` lie where they did on 1a6df872. The functions ahead of them in each section (`mark`, `StoreImage`, `imageEnter`; `ClearImage`, `packetEnter`) moved down by the same amounts. Each section still ends where it did, so each fills its gap as before. `drawPollMove` stays at 8004114Ch, which is clear of `MoveImage` again.
- **Six more pins**, the unpinned functions on the image paths: `queueLoad`, `queueStore`, `packetMoved`, `packetRoomWait`, `imageTurn` and `imageRefused`. Each is on cache lines clear of the code it runs with (`queueLoad` of `LoadImage`, `imageEnter`, `pollQueue`, `DrawSync`, `drawPoll` and `mark`, and so on), found by trying every place in the gaps after the other pins.

Two layouts on the way were dropped:

- With `.text.MoveImage` where it was and `drawPollMove` moved (s55c, s55d), warm `MoveImage` was 693 under the model and 675 on the console (s55d, ad16fd2c). A watch of every function between two warm `MoveImage` calls in `imgparts` showed what runs there: `mark`, `spin`, `DrawSync`, `drawNext` and `memset`. `DrawSync` had moved onto 4 of `MoveImage`'s lines, which pinning `.text.DrawSync` 72 bytes later undid.
- Placing `drawPollMove` by the same search as the other six put it on 7 of `MoveImage`'s lines once `MoveImage` was back.

The image is `.build/console/libps-s55e.exe.gz`, SHA-1 fb5e592d. It ends at 80066288h. The ten timing tests pass, and `image/regress.sh` gives 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL. Under the model a queued `LoadImage` takes 95 eighths in `qlen` (Sony's 94, 104 before).

### The queue's drain under the model

`qlen`'s time from the first call to the 64th's return (eighths): Sony's 16,837, 1a6df872 17,486, s55e 18,498. Across the layouts tried, it goes with how fast the 63 calls queue, not with where anything lies:

| a queued `LoadImage` | time to the 64th call's return |
|---|---|
| 111 | 17,486 |
| 98 | 18,064 |
| 95 | 18,498 |
| 88 | 18,761 |

The faster the calls queue, the longer the 64th spends in `LoadImage`'s wait for room. So while that wait runs, the interrupt chain that sends the queued images goes slower than it does under the calls. That is the library's wait loop (`spin` and `cdWatch.check` run there, some 1,650 to 2,000 times in `qlen`), not the layout, and is left for a later spec. `qfull` shows it on the console, below.

### On the console (spec side, 2026-11-13, `.build/hardware/spec55`)

`imgparts` on spec 53's image (`base53`) as well, since no console run had timed the warm calls since s50d. Cycles, warm unless marked:

| | Sony's | 1a6df872 | s55d (ad16fd2c) | s55e (fb5e592d) |
|---|---|---|---|---|
| `MoveImage` | 609 | 658 | 675 | 659 |
| `MoveImage` cold | 953 | 818 | 833 | 822 |
| `ClearImage` | 885 | 831 | 844 | 870 |
| `LoadImage` 0x4, 16x16 | 732, 1,054 | 756, 1,038 | 750, 1,041 | 784, 1,056 |
| `StoreImage` 0x4, 16x16 | 752, 1,305 | 723, 1,201 | 746, 1,199 | 759, 1,175 |
| 24 polls: idle, Load, Store, Move | 3,552, 8,572, 7,320, 3,619 | 3,442, 8,402, 6,451, 3,497 | 3,444, 8,396, 6,339, 3,520 | 3,437, 8,391, 6,354, 3,490 |

So s55e's `MoveImage` is 1a6df872's, and the image as a whole is as near Sony's as 1a6df872, a little nearer. The 50 cycles warm `MoveImage` is over Sony's came in between s50d (607) and 1a6df872, not here.

On s55e, against spec 53's runs: `imgsync`'s polls to 0 at 256 by 256 are 183, 308, 611, 243 and 35 (spec 53: 183, 310, 610, 243, 36). `loadsync`'s `LoadImage` at 16, 24, 32 and 48 square is 192, 217, 238 and 240 eighths (Sony's 187, 213, 245, 245, spec 53 188, 214, 231, 234), `MoveImage` 118 and 119, the fill 151 and 150. `kanjisource` gives the same lines.

`qfull`: `sync` is 1 on even rounds and 0 on odd ones on both libraries alike. `last`, a queued `LoadImage`, is 129 to 133 on Sony's and 126 to 128 on s55e (119 to 121 on s55d). `all`, the queue's whole time, is 25,427 to 25,609 on Sony's and 25,961 to 26,223 on s55e, 2% over (s55d 0.5 to 1%): the drain above.

### Left open

- **Warm `MoveImage`**, 50 cycles over Sony's on the console since before this spec: something between s50d and 1a6df872. The watch above lists what runs between two calls.
- **The queue's drain** while `LoadImage` waits for room, above, with the list after each queued image and the chain's 81 cycles an image over Sony's.
- **`queueImageNops`** (4 nops in `queueLoad` and `queueStore`) is a pad "for their size", from before the pins, as its comment says. It is also 8 cycles of a queued call's time. Not touched here: taking it out wants `TestQueuedImages`' times looked at.
- **Unpinned paths**: every function the image calls' paths run is pinned now, but `drawNext` and `memset` run between two `MoveImage` calls and are not. `drawNext` has not moved since 1a6df872.
