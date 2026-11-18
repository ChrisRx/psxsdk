# The image calls and DrawSync(1) take Sony's time on a console

## Why

Spec 49 put each free image call's accesses at Sony's cycles in the emulator, and the console confirmed `DrawSync(1)` now gives Sony's answers (its Results, "On the console"). Two differences showed on the console that the emulator did not:

- `LoadImage`'s call is 9 to 12 eighths over Sony's at every size `loadsync` tried (196 against 186 at 16 by 16, 257 against 245 at 48 by 48). The cost model had it 2 over.
- Polling `DrawSync(1)` to 0 after a large image takes a different number of polls, both libraries on the same console. At 256 by 256: `LoadImage` 170 against Sony's 183, `StoreImage` 236 against 250, `MoveImage` 723 against 603, `ClearImage` on a rectangle 287 against 239, the fill 43 against 36. So the clean poll is longer than Sony's after `LoadImage` and `StoreImage`, and shorter after `MoveImage` and `ClearImage`.

A program that polls `DrawSync(1)` while it waits for an image, or times its frame around one, sees the difference.

## What to do

1. Find from the console where `LoadImage`'s extra time goes, and why the model missed it. Write console probes that split the call into parts (between its accesses, as spec 49's traces do in the emulator), for the spec side to run under `-library sony-ourmon` and `-library clean`. Correct the cost model in `emu/cpu/cost.go` if what the console shows is a rule of the hardware (as spec 48's multiply rule was), with a test of it, and then pace the library to it.
2. Find the length of one `DrawSync(1)` poll on each library after each kind of image call, on the console, and make the clean one Sony's. The answer at each poll must stay Sony's, as spec 49 left it.
3. Keep what specs 45 to 49 fixed: each free image call's accesses in the emulator, `qstate` in both states, the interrupt's length, and the Gs calls.

## Testing it

The emulator with the cost model on (`PROBE_CPUCOST=1`), against Sony's image, and the console, which the spec side runs. Spec 49's checks must still pass: TestImageSync and spec 47's nine (TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll, TestQueueLength) on Sony's discs and the new image, `image/regress.sh`, `go test ./emu/... ./exp/cleanlib/...`, and `archive.sh` against the current tree's summary (99 the same). On the console, `loadsync` and `imgsync` should agree with Sony's call for call.

## Results


First round, 2026-11-12, in the emulator (implementation side), with the scph1001 BIOS. Not yet on a console: what to run is at the end. The candidate image is `.build/console/libps-s50a.exe.gz` (the image whose SHA-1 starts 63dad86c); spec 49's, the starting point, is `libps-s49h.exe.gz`.

### Summary so far

- The emulator's cost model has an image's DMA take the CPU no time, and on a console it does: Sony's own `LoadImage` takes 186, 212, 246 and 245 eighths at 8, 18, 32 and 72 blocks on the console (spec 49's `loadsync`), and 171 at every size under the model. So the model cannot yet say where `LoadImage`'s 9 to 12 eighths go, nor what a poll costs while an image's blocks are going. `probes/dmahold` (new) asks the console for the rule.
- With no DMA going, the model does see the poll's difference: `DrawSync(1)` warm takes Sony's 118 to 134 cycles on both libraries with the costs off, and under the costs 149 (idle), 134 (channel busy) and 152 (GPU drawing) on Sony's against 114, 109 and 120 on s49h. The clean pads were stores and nops, which a console prices at 2 and 1 cycles where the emulator has 2 for each. That is the console's 723 polls against 603 after `MoveImage`, 287 against 239 after `ClearImage` and 43 against 36 after the fill: the model has 576 against 462. Fixed in s50a by making some pads loads.
- After `LoadImage` and `StoreImage` the console has the clean poll longer (170 polls against 183), where the model has it shorter as everywhere else (541 against 449). The blocks are going during those polls, so that is the same missing rule.
- A call's time under the model moves by some 50 cycles with where the probe's own code and the library's fall in the instruction cache, cold and warm alike. `probes/imgparts` (new) splits each call in three on the console, warm and cold, for both libraries.

### What the model has, and has not

`loadsync` under the model with the drawing time on, eighths, against the console's (spec 49, "On the console"):

| | Sony's, model | Sony's, console | s49h, model | s49h, console |
|---|---|---|---|---|
| `LoadImage` 16 by 16, 24 by 24, 32 by 32, 48 by 48 | 171, 171, 171, 171 | 186, 212, 246, 245 | 173, 173, 173, 173 | 196, 223, 255, 257 |
| `MoveImage` 16 by 16, 32 by 32 | 122, 122 | 121, 121 | 117, 117 | 122, 121 |
| `ClearImage`'s fill 64 by 4, 64 by 16 | 147, 147 | 147, 147 | 151, 151 | 150, 152 |

`MoveImage` and the fill, which start no blocks, are the console's within 5 eighths on both libraries. `LoadImage` is not: the console's call grows with the image up to 32 blocks, some 2.5 eighths (20 cycles) a block, and then stops growing. So the blocks hold the CPU up while they go, some 20 of each block's 27 cycles (the emulator's `imageCyclesTenths` has a word at 1.7 cycles), and a call whose blocks outlast it pays for the 32 or so that pass before it returns. The model holds the bus for channel 6 alone (`emu/cpucost.go`). Less the blocks, Sony's call comes to about 166 eighths on the console at every size and the clean one to about 176: the 10 are there with 8 blocks as with 72, so most of them lie outside the blocks' time, and the model does not show them. Whether they are the cache (the calls in `loadsync` come after a `VSync(0)`, so they are partly cold) or the pads' kinds, `imgparts` is to say.

`imgsync`'s polls to 0 at 256 by 256 under the model, against the console's:

| | Sony's, model | Sony's, console | s49h, model | s49h, console |
|---|---|---|---|---|
| `LoadImage` | 449 | 183 | 541 | 170 |
| `MoveImage` | 462 | 603 | 576 | 723 |
| `ClearImage`, a rectangle | 461 | 239 | 575 | 287 |
| `ClearImage`, the fill | 461 | 36 | 575 | 43 |

(The counts themselves differ because the emulator's GPU takes a cycle a pixel for everything, and a console's does not; it is the ratio between the libraries that the model is to have.) Clean over Sony's is 1.25 in the model and 1.20 on the console for the three that start no blocks, and 1.20 in the model against 0.93 on the console for `LoadImage`.

### `DrawSync(1)`, with no blocks going

`probes/pollsync` (new, for the emulator) polls `DrawSync(1)` twelve times in a row behind each kind of image call on 256 by 256, each poll between GP1 markers, for a bus trace. Warm, cycles from the marker before the poll, Sony's, s49h and s50a:

| | costs off: all three | Sony's, costs on | s49h | s50a |
|---|---|---|---|---|
| idle: D2_CHCR, GPUSTAT, the next marker | 68, 84, 118 | 79 to 83, 97 to 101, 145 to 149 | 65, 77, 114 | 85 to 87, 103 to 105, 144 to 151 |
| the channel busy: D2_CHCR, the next marker | 68, 108 | 79 to 83, 130 to 134 | 65, 109 | 85, 135 |
| the GPU drawing: D2_CHCR, GPUSTAT, the next marker | 68, 84, 124 | 79 to 83, 97 to 101, 148 to 152 | 65, 77, 120 | 85 to 87, 103 to 105, 147 to 154 |
| requests waiting (`imgsync`): D2_CHCR twice, the next marker | 76, 102, 138 | 86, 119, 172 | 66, 87, 138 | 87, 121, 172 |

(The ranges are the same call from the probe's twelve call sites, which the compiler unrolled: where each lies in the cache moves a poll by 4 to 9 cycles.)

What changed (`gpu.go`, `fidelity.go`, `nofidelity.go`):

- `loads` and `loadsApart` (new) spend 2 cycles each in the emulator on a load of the stack's top word, or the one above it, into `$zero`: about 6 under a console's costs (spec 48's `padcost48`, "lw zero"), where a store costs 2 and a nop 1. They take no register and no address, so one stands where a nop would.
- `drawPoll`'s pads are loads in part: before the read of D2_CHCR six loads and two stores where there were twelve stores (the function has fewer instructions of its own with the pads as inline assembly, which the compiler does not move its own around), a load and a nop before GPUSTAT where there were two nops, two loads after it, a load and four stores on the busy path, and with requests waiting five loads and six stores before the first read, three and three before the second, one and four after it. With the costs off every access is where it was. Under the costs the tail after GPUSTAT is some 10 cycles short of Sony's (48 or 51), with no instruction left there to make a load of, so the read of D2_CHCR comes 4 to 6 cycles later than Sony's instead and the whole is Sony's within a few cycles.
- The waiting path pads with `loadsApart` and `spaceApart`: with the same loads and stores on both paths the compiler hoisted their common start above the branch, and each count then moved the other path's time.
- `drawPoll` is in `DrawSync`'s section (`//go:section .text.DrawSync`), so the image's packing puts it straight after `DrawSync` and the two never share cache lines. Placed by size, a poll took some 19 cycles more in one layout of three. Whether either shares lines with a program's polling loop still depends on where the program lies.

Polls under the model after this, three probes, clean over Sony's: `pollsync` 0.97 to 1.04 a poll, `imgparts` (24 polls timed as one: 3,719 idle, 3,339 behind `LoadImage` and 3,796 behind `MoveImage` against 3,596, 3,242 and 3,675) 1.03, and `imgsync`'s count of polls to 0 (442 behind `MoveImage` against 462, 431 behind `LoadImage` against 449, 426 with requests waiting against 430) 1.01 to 1.05. On s49h the same three had 0.79, 0.82 and 0.80. The spread between the probes is the cache again. The console had s49h's poll at 0.83 of Sony's where the model had 0.80, so on the console s50a's may come out some 3 to 6% long. The round's figures say which way to trim.

### The probes for the console

**`probes/dmahold`** (`.build/serial/dmahold.elf`, either library, once): cpucost2's frame, machine code the probe writes and runs with interrupts off, six times a row, timed in cycles with root counter 2. A row starts channel 2 on an image of n blocks as `LoadImage` does (or from the GPU, as `StoreImage` does), and times a run of one kind of instruction while the blocks go: nops, loads from RAM, stores to RAM, loads and stores with nops between, reads of GPUSTAT, I_STAT and D2_CHCR, and a loop of a load, an add and a store. The same runs are timed with no transfer. "alone" rows time the transfer itself, by polling D2_CHCR, for 1 to 256 blocks. Its waits give up after a million turns, so it returns whatever the hardware does. Under the model today, as expected of a model with no rule: every run costs the same with a transfer going as without (nop x256 261, lw x64 389, sw x64 134, lw gpustat x32 165), `load alone` is 27 cycles a block (228 for 8, 1,748 for 64) and `store alone` is 18 whatever the size, since the emulator's transfer from the GPU takes no time.

What it should show: how long n blocks take each way; whether cached code with no bus access runs on during a block (nop x256 near 261 with a transfer going) or stops (near 900); and what a load, a store and a read of a port each wait for. From that the model gets its rule (a hold of the bus a block, as `HoldBus` does for channel 6, with the CPU's time between blocks), with a test of it, and the image calls and the poll can then be paced under it.

**`probes/imgparts`** (`.build/serial/imgparts.elf`, both libraries): splits a free image call in three, from outside. The call masks every interrupt for its middle. The probe has root counter 2, on the system clock, interrupt at a count T after it is started just before the call, for every T from 1 to 700 past the call's end, a call for each T, and machine code it puts first in the kernel's interrupt chain (as `irqsplit` does) stamps the counter when the interrupt is taken, with D2_CHCR, D2_MADR, D2_BCR and GPUSTAT. An interrupt due while I_MASK is 0 is taken when the call puts I_MASK back, whatever T was: so the first T taken late is where the call masks ("mask"), the last T taken late is where it unmasks ("last"), and the call's end is timed apart ("warm"). It reads no address of the library's code and prints no stamp but the masked one. It does this warm (each call after the same call) for 24 cases: `LoadImage` of no size, of the CPU's words alone, and of 1 to 128 blocks, `StoreImage` the same, `MoveImage`, and `ClearImage` on a rectangle and a fill. Then cold, as `loadsync` makes its calls (after a `VSync(0)` and a `DrawSync(0)`, a call a frame, T by 4) for seven of them. EDGES lines give each T past the unmasking where D2_MADR is first read changed: the blocks' progress while the call's end runs. POLL lines time 24 `DrawSync(1)` in a row and 16 one by one, with nothing going on and behind each kind of call on 256 by 256. It bounds its waits and takes its chain element out before END. About 2 minutes on a console, half of it the cold sweeps. If no interrupt comes at all it says so for each case and goes on.

Under the model, cycles, the counter's start to the mask, the masked part, and the unmasking to the call's end:

| | Sony's, warm | s50a, warm | Sony's, cold | s50a, cold |
|---|---|---|---|---|
| `LoadImage` 0 by 4 | 295, 370, 113: 778 | 316, 364, 93: 773 | 437, 584, 119: 1,140 | 429, 616, 75: 1,120 |
| `LoadImage` 16 by 1 | 295, 660, 113: 1,068 | 325, 536, 156: 1,017 | | |
| `LoadImage` 16 by 16 | 295, 529, 113: 937 | 334, 416, 150: 900 | 437, 812, 119: 1,368 | 429, 840, 83: 1,352 |
| `LoadImage` 48 by 48 | 295, 553, 113: 961 | 334, 449, 150: 933 | 437, 812, 119: 1,368 | 429, 840, 83: 1,352 |
| `StoreImage` 16 by 16 | 295, 540, 160: 995 | 370, 717, 98: 1,185 | 477, 872, 103: 1,452 | 437, 908, 109: 1,454 |
| `MoveImage` 16 by 16 | 351, 205, 83: 639 | 347, 134, 69: 550 | 553, 304, 115: 972 | 549, 296, 92: 937 |
| `ClearImage`, a rectangle 16 by 16 | 307, 522, 83: 912 | 416, 557, 72: 1,045 | 441, 832, 112: 1,385 | 501, 808, 121: 1,430 |
| `ClearImage`, a fill 64 by 16 | 307, 407, 95: 809 | 429, 463, 87: 979 | 441, 632, 112: 1,185 | 501, 580, 122: 1,203 |

So the model has the clean calls within 4% of Sony's cold, which is how `loadsync` times them, and up to 21% off warm, either way: the clean calls spend their time in other kinds of instruction than Sony's and in more lines of code, and the two cancel cold. A program that loads images in a row meets the warm figures. The same probe built another way had Sony's warm `LoadImage` at 1,025 and the clean one's at 904: 90 cycles' difference from where the probe's own code lay. The console's parts will say how much of this is the model's.

**`imgsync` and `loadsync`** (`.build/serial/imgsync.elf`, `loadsync.elf`) now return after END, where they looped on `VSync(0)`.

### Checks on s50a

- `TestImageSync` and spec 47's nine (`TestDrawWait`, `TestFieldTime`, `TestQueuedImages`, `TestDMALeft`, `TestVSyncCount`, `TestVSyncPhase`, `TestIRQGaps`, `TestVSyncPoll`, `TestQueueLength`) pass on Sony's discs and on s50a's (`.build/s50/tests.sh`; the discs are `.build/s50tNAME-cimg` and `-cimg-clean`).
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before.
- `archive.sh` on the unchanged tree: 99 the same (`.build/s50/summary-base.txt`). Not yet run on s50a.
- `libps_vsync_wait` is where spec 49 left it (8003C070h), and `vblankInterrupt`, `libps_interrupt`, `libps_wait_blanks` and `VSync` had the same addresses in every build of this round (800113F0h, 80046514h, 80022654h, 8003E9B0h). `DrawSync` moves with each build, `drawPoll` behind it (8002331Ch in s50a).
- `go test ./emu/... ./exp/cleanlib/...` pass. The static clean build compiles with and without the fidelity tag (`docs/tutorial/hello`).

### On the console (to do, spec side)

1. `dmahold` (`.build/serial/dmahold.elf`) once, under either library.
2. `imgparts` (`.build/serial/imgparts.elf`) under `-library sony-ourmon`, under `-library clean` on `.build/console/libps-s49h.exe.gz`, and under `-library clean` on `.build/console/libps-s50a.exe.gz`.
3. `imgsync` (`.build/serial/imgsync.elf`) under `-library clean` on s50a: the polls to 0 behind `MoveImage` and `ClearImage` at 64 by 64 and 256 by 256 should be near Sony's 39 and 603, 12 and 239, and 36 for the fill, where s49h had 47 and 723, 15 and 287, and 43. Behind `LoadImage` and `StoreImage` s49h's poll was already longer than Sony's, and s50a's is longer still under the model, so those counts should fall further below Sony's 183 and 250 until the blocks' rule is in the model.

### The console, round 1 (spec side's runs, 2026-11-12, `.build/hardware/spec50/r1`)

`imgsync` on s50a gives Sony's answers. Its polls to 0 at 256 by 256, s50a, Sony's and s49h: `LoadImage` 151, 183, 170; `StoreImage` 220, 250, 236; `MoveImage` 581, 603, 723; `ClearImage` on a rectangle 231, 239, 287; the fill 34, 36, 43. So with no blocks going s50a's poll is 3 to 6% longer than Sony's, where s49h's was 17% shorter, and behind an image's blocks it got longer still, as expected.

`imgparts`, the POLL lines, cycles for 24 polls, Sony's, s49h, s50a: idle 3,552, 2,830, 3,609; behind `MoveImage` 3,630, 3,045, 3,702; behind `LoadImage` 8,564, 9,948, 10,636; behind `StoreImage` 7,423, 8,013, 8,247. One poll behind `LoadImage` is some 315 cycles on Sony's and 400 on s50a, against 140 and 145 with nothing going.

`imgparts`, the calls. The split in three did not come out: of 93 sweeps 15 found a masked part, and those disagree with each other. The probe took the masked part to be the longest run of T with one stamp, within a cycle. In the emulator it is; on the console the stamps of the one moment evidently spread further. The probe now takes the commonest stamp within 12 cycles and prints the spread, for round 2. The calls' times came out, cycles, the least of eight, warm (after the same call) and cold (after a `VSync(0)`):

| `LoadImage` | Sony's warm | s49h | s50a | Sony's cold | s49h | s50a |
|---|---|---|---|---|---|---|
| 0 by 4 | 733 | 778 | 755 | 1,089 | 1,159 | 1,109 |
| 16 by 1, the CPU's words | 1,004 | 996 | 1,009 | 1,414 | 1,505 | 1,465 |
| 1 block | 907 | 939 | 910 | 1,332 | 1,438 | 1,392 |
| 2 | 927 | 935 | 934 | 1,356 | 1,459 | 1,402 |
| 4 | 975 | 1,004 | 987 | 1,402 | 1,494 | 1,452 |
| 8 (16 by 16) | 1,041 | 1,085 | 1,058 | 1,478 | 1,590 | 1,534 |
| 12 | 1,133 | 1,170 | 1,153 | 1,553 | 1,681 | 1,621 |
| 18 (24 by 24) | 1,252 | 1,295 | 1,291 | 1,687 | 1,809 | 1,755 |
| 24 | 1,341 | 1,432 | 1,408 | 1,810 | 1,936 | 1,877 |
| 32 | 1,334 | 1,581 | 1,555 | 1,929 | 2,095 | 2,043 |
| 40 | 1,335 | 1,573 | 1,568 | 1,929 | 2,133 | 2,084 |
| 72 (48 by 48) | 1,355 | 1,598 | 1,527 | 1,921 | 2,144 | 2,085 |
| 128 | 1,341 | 1,610 | 1,576 | 1,922 | 2,128 | 2,083 |

That answers the first question. With no blocks, or one, the clean call is Sony's within 5% warm. Each block of the image then costs both libraries some 20 cycles, for as long as the call is still running: Sony's call is done 24 blocks after it starts them, and the clean one 32. So an image of 32 blocks or more costs the clean call some 240 cycles more than Sony's, 30 eighths, and a smaller one less, by its size. Cold, the clean call is 60 to 110 cycles over Sony's with no blocks at all (the cache: it runs through more lines of code), and s50a, whose `LoadImage` is s49h's instruction for instruction at other addresses, is 40 to 60 under s49h.

`StoreImage` warm, Sony's, s49h, s50a: no size 753, 797, 837; the CPU's words 1,279, 1,133, 1,242; 1 block 1,164, 1,038, 1,209; 8 blocks 1,291, 1,216, 1,392; 32 blocks 1,506, 1,493, 1,710; 128 blocks 1,461, 1,591, 1,748. `MoveImage` 610, 557, 578. `ClearImage` on a rectangle 882, 1,042, 1,048, and the fill 777, 935, 938.

`dmahold`, warm, cycles, alone and with an image of 64 blocks going to the GPU, which takes 1,800 (28.1 a block; 2,210 from the GPU, 34.5 a block):

| run | alone | with the blocks going |
|---|---|---|
| nop x256 | 259 | 269 to 281 |
| lw from RAM x64 | 393 | 1,753 to 1,801 |
| lw from RAM x16 | 100 | 441 to 478 |
| lw and a nop x32 | 262 | 916 to 931 |
| lw and seven nops x16 | 164 | 438 to 477 |
| sw to RAM x64 | 133 | 669 to 706 |
| sw and three nops x32 | 133 | 334 to 368 |
| sw and seven nops x16 | 132 | 144 to 182 |
| lw GPUSTAT x32 | 163 | 473 to 522 |
| lw GPUSTAT and seven nops x16 | 164 | 427 to 478 |
| lw I_STAT x32, lw D2_CHCR x32 | 163 | 483 to 522 |

So code in the cache runs on while the blocks go, and every access to the bus waits: a load from RAM gets through once a block, whether the loads come in a run or seven nops apart, a read of a port twice a block in a run and once seven nops apart, and stores leave the write buffer two or three to a block. With the image coming from the GPU the bus is open longer: two loads a block, three or four stores.

### The model's rule (round 2)

`emu/cpu/cost.go`, `HoldBlocks`: a transfer in blocks holds the bus for the first part of each block and leaves it open for the last 6 cycles. An access (a load, a store leaving the write buffer, a cache miss's fill) may start only while it is open, and waits otherwise; code in the cache runs on. At 6 cycles a second load from RAM never starts in one block and a second read of a port always does. A store has to start 1.4 cycles before the open part ends, which makes it two a block or three by where in a cycle the block starts: the console's 2.6. `emu/gpu_time.go` calls it for an image going to the GPU by channel 2, when the drawing time is on (which is what gives the transfer its length) and the blocks start at once. An image coming from the GPU still takes the emulator no time, so `StoreImage`'s blocks are not in the model.

`dmahold` under it, against the console: nop x256 271 (269 to 281), lw x64 1,746 (1,753 to 1,801), lw x16 462 (441 to 478), lw and a nop x32 897 (916 to 931), lw and seven nops x16 462 (438 to 477), sw x64 as below, lw GPUSTAT x32 462 (473 to 522) and seven nops apart 462 (427 to 478), I_STAT and D2_CHCR 462 (483 to 522). The transfer itself is 27.2 cycles a block in the emulator (`imageCyclesTenths`, which the drawing-time tests are set to) against the console's 28.1, so everything that waits out blocks is 3% short.

**Stores are not settled.** (Settled in round 2 on the console, below: the second rate was wrong and is gone.) `dmahold`'s stores went to a word within 6 KB of the image's own and left the buffer 2.6 to a block. The clean `LoadImage` has 21 stores to a word of the library's, half a megabyte from a program's image, after it starts the blocks, and its call grows by 20 cycles a block up to 32 blocks: with the stores at 2.6 a block the model has it stop at 19, and with them at one a block at 32, with the console's figures (828, 853, 903, 1,004, 1,105, 1,256, 1,340, 1,439 at 1 to 32 blocks under the model, 910 to 1,555 on the console: 611 against 645 from first to last). But a poll of `DrawSync(1)` behind the blocks, with six such stores in runs of two and four, takes 443 cycles on the console, and the model has 449 with the stores at 2.6 a block and 530 with them at one. The model as it stands has a store within 64 KB of the block being moved at 2.6 a block and any other at one (`costBlockNear`), which is right for `LoadImage` and 20% over for that poll. `dmahold` has new rows for round 2: stores 4 bytes to 256 KB from the image's words and to the stack, in a run, in twos and in fours with nops between, and a load and a store in turn.

`TestCPUCostBlocks` (`emu/cpucost_test.go`) holds the rule: cached code runs on, a load from RAM a block (in a run and seven nops apart), two reads of a port a block, stores near the image two or three a block and far from it one, and the bus free once the blocks are done.

### The pacing (round 2)

Four things, each from the console's round 1 and the rule.

**The calls' code lies together.** `LoadImage` in s50a called `imageEnter` from cache lines `imageEnter` itself lay on, in the build after s50a, and every call then ran as if cold: 350 cycles more under the costs. Each build packs the image again, so each build had its own such accidents. s50a's `StoreImage` had one on the console (1,209 warm for a block, s49h's 1,038 with the same instructions), and the model has it (1,185 against 859 once it is gone). Now `DrawSync`, `drawPoll`, the watch's `mark`, `imageEnter`, `LoadImage` and `StoreImage` are one section (`.text.DrawSync`, 3.9 KB, so none shares a line with another but for one where it wraps), and `MoveImage`, `ClearImage` and `packetEnter` another (`.text.MoveImage`). Two functions that do nothing, `drawPlace` and `packetPlace`, lie in them, and their sizes set where the packing puts each. `packetPlace`'s is for the second section, which has to share lines with something in the first, to share them with `LoadImage` and `StoreImage` and not with `DrawSync`, `drawPoll` and `mark`, which a program calls around a `MoveImage` (11 lines of `drawPoll` are left shared with `ClearImage`). `drawPlace`'s is for `TestVSyncPoll`: under the costs `VSync(0)` returned 158 to 167 cycles after a blank's interrupt in one field or more, where Sony's limit is 155, in five of nine sizes tried, and passes in the other four (1, 3, 6 and 16 instructions; it is 1). I looked for what shares lines with the wait's code in the failing layouts and found nothing that tells them from the passing ones, so the test itself is the check: a change to the size of anything in the image wants it run again. `.build/s50/lines.py` lists what shares lines in a built image.

**With the accidents gone the clean calls were cheap.** Their pads were stores, 2 cycles each on a console as in the emulator, where Sony's instructions average more. Under the model, the same probe, warm: `LoadImage` of no size 681 against Sony's 778, of the CPU's words 848 against 1,068, of one block 725 against 959; `StoreImage` 674, 953 and 859 against 801, 1,142 and 995; `ClearImage` 785 against 912 and its fill 623 against 809. (s50a's figures were Sony's on the console because its accidents made up the difference.) So part of each pad is loads now, 2 cycles in the emulator and 6 on a console: in `imageEnter` 3 of the 7 before I_MASK, 4 of the 11 before D2_CHCR and 3 before GPUSTAT; in `LoadImage` 18 of the 19 before the GPU is asked whether it is ready and 6 between the image's four words; in `StoreImage` 16 of 18, 3 with no size and 12 after the CPU's words; in `packetEnter` 2 and 2; in `ClearImage` 1 after the channel starts for a rectangle and 9 for a fill. Inline assembly does not move as the compiler's own stores do, so the stores around each were counted again on `imgsync`'s trace until every access was back where spec 49 left it.

**After the blocks start, fewer accesses to the bus.** `LoadImage`'s pad there was 21 stores, and with the registers it loads on its way out that came to 32 blocks' worth where Sony's has 24. It is 6 stores, 11 nops and two multiplies (`multsWide`: 4 cycles in the emulator, 15 on a console, no access). `StoreImage`'s is 10 stores, 10 nops and a multiply, by the same reasoning: the model has no blocks from the GPU to check it with.

**The poll behind an image's blocks.** `drawPoll`'s way with no request waiting has no stores now: 4 loads, a multiply and 3 nops before D2_CHCR, and with the channel busy a load, a multiply and 2 nops after it.

Under the model with the rule, `imgparts`, cycles, warm:

| `LoadImage` | Sony's, console | Sony's, model | s50a, console | s50c, model |
|---|---|---|---|---|
| 0 by 4 | 733 | 778 | 755 | 751 |
| the CPU's words | 1,004 | 1,068 | 1,009 | 1,028 |
| 1 block | 907 | 959 | 910 | 907 |
| 2 | 927 | 980 | 934 | 920 |
| 4 | 975 | 1,022 | 987 | 985 |
| 8 | 1,041 | 1,102 | 1,058 | 1,061 |
| 12 | 1,133 | 1,184 | 1,153 | 1,140 |
| 18 | 1,252 | 1,279 | 1,291 | 1,267 |
| 24 | 1,341 | 1,355 | 1,408 | 1,300 |
| 32 | 1,334 | 1,355 | 1,555 | 1,300 |
| 72 | 1,355 | 1,379 | 1,527 | 1,345 |

The model has Sony's own call 1 to 6% over the console's, and had s50a's within 2% of the console's where no blocks go (773, 1,017 and 900 for the first three rows, on round 1's probe). So the clean call is set between the two columns of Sony's. It stops growing at 24 blocks, as Sony's does, and grows by 393 cycles to there (Sony's 396 in the model, 434 on the console).

**Cold, the model has `LoadImage` over Sony's still.** After a `VSync(0)`, the least of eight, the clean call and Sony's under the model: no size 1,051 and 1,140, the CPU's words 1,525 and 1,484, one block 1,386 and 1,388, 8 blocks 1,563 and 1,500, 24 and more 1,765 and 1,762. `loadsync`, which times it that way, in eighths, the clean call, Sony's, and Sony's on the console: 199, 190 and 186 at 16 by 16; 218, 210 and 212 at 24 by 24; 225, 220 and 246 at 32 by 32; 225, 220 and 245 at 48 by 48. So at 16 by 16 the model still has the clean call 9 eighths over, the figure this spec began with. Cold less warm is 502 cycles for the clean call at 8 blocks and 398 for Sony's: the clean call runs through some dozen more lines of code, its pads being one instruction after another where Sony's time is evidently spent in fewer. And the model is not to be trusted cold: it has Sony's own call stop growing at 220 where the console has 246, and it had s49h 2 over Sony's where the console had 10. The round's COLDPARTS lines are to say where a cold call's time goes on each library before the pads are made into loops, which is what would take lines out.

The other calls, the same way: `StoreImage` of no size, the CPU's words and a block 801, 1,154 and 990 (Sony's 801, 1,142 and 995 in the model; the console has Sony's 137 and 169 over the model with words and a block, the GPU's own time to answer, which the clean call waits for alike); `MoveImage` 608 to 629 (639, the console 610); `ClearImage` on a rectangle 933 (912, the console 882) and its fill 798 (809, the console 777).

24 polls of `DrawSync(1)`, Sony's and the clean library's under the model, with the console's for Sony's: idle 3,596 and 3,586 (3,552); behind `MoveImage` 3,675 and 3,639 (3,630); behind `LoadImage`, with the blocks going, 9,209 and 8,910 (8,564), where s50a has 12,718 under the model as it stands and 10,636 on the console. `imgsync`'s polls to 0 at 256 by 256 under the model: 461 behind `MoveImage`, `ClearImage` and the fill (Sony's 461 each), and 166 behind `LoadImage` (Sony's 154).

### What changed in round 2

- `emu/cpu/cost.go`, `emu/cpucost.go`, `emu/dma.go`, `emu/gpu_time.go`, `emu/machine.go`: `HoldBlocks` and its use for an image going to the GPU, above, with `TestCPUCostBlocks` in `emu/cpucost_test.go`. With the cost model or the drawing time off nothing changes.
- `exp/cleanlib/fidelity.go`, `nofidelity.go`: `mults` and `multsWide`, pads of a multiply of zero and a read of its result (4 cycles in the emulator, 8 or 15 on a console, no access to the bus; they change HI and LO, which no caller keeps across a call), beside round 1's `loads` and `loadsApart`.
- `exp/cleanlib/gpu.go`, `image.go`, `extra.go`: the sections, `drawPlace` and `packetPlace`, and the pads, above. `ClearImage` writes D2_BCR 4 cycles early now where it was 2 (its nops before the channel's three registers are 2, 0 and 2). Every other access `imgsync` traces is where spec 49 left it, and every call ends on Sony's cycle (`.build/s50/cmp.py` compares the two traces).
- `probes/dmahold`: rows for stores by their distance from the image's words, in a run and in twos and fours, a load and a store in turn, loads from the stack, and multiplies. `probes/imgparts`: the masked part by the commonest stamp, and `const sweep`, which off leaves the call times and POLL lines alone (quick in the emulator).
- The target's `.S` files are not changed.

### Checks on s50c

The image is `.build/console/libps-s50c.exe.gz` (SHA-1 a25dd216). `libps-s50b.exe.gz` is the same pads before `drawPlace` was added, and fails `TestVSyncPoll`: not for use.

- `TestImageSync` and spec 47's nine pass on s50c's discs (`.build/s50/tests-c.log`), and on Sony's discs with the emulator's new rule (`tests-b.log`, where `TestVSyncPoll` on Sony's disc ran out of its 900 seconds with the machine busy and passed run again).
- `go test ./emu/... ./exp/cleanlib/...` pass. The static clean build compiles with and without the fidelity tag.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before.
- `archive.sh`: 99 the same, as on the unchanged tree (`.build/s50/summary-base.txt` and `summary-c.txt`). One line differs: `scee-topten-hover` makes 570 picture changes on both libraries where the clean one made 569, and first differs in frame 605 on both where the clean one had it in 606. It is still counted as differing.
- `pollsync`, warm, costs off: every poll's accesses on Sony's cycles (68, 84, 118; 108 with the channel busy; 124 with the GPU drawing; 76, 102, 138 with requests waiting). Costs on: 138 idle, 141 to 150 with the GPU drawing (Sony's 145 to 149 and 148 to 152), 172 with requests waiting (172).

### Not done, and open

- **Cold `LoadImage`**, above: the model has it 5 to 9 eighths over Sony's in `loadsync` with blocks, and the model's cold figures have not matched the console's so far. Wants the console's COLDPARTS first, then the pads as loops.
- **Stores while blocks go**: the rule has two rates and a guess at where one gives way to the other, above. The clean calls now make few stores there (six in `LoadImage`'s end, ten in `StoreImage`'s, none in the poll's way with nothing waiting), so their own times lean on it little. Sony's side under the model leans on it wherever Sony's stores.
- **`StoreImage`'s blocks** take the emulator no time, so neither its call behind 8 blocks and more (Sony's 1,291 to 1,506 on the console) nor a poll behind them (Sony's 309 cycles) is in the model. The clean pads there were changed by analogy with `LoadImage`'s. The console's round 2 says how near that came.
- **The split in three on the console** is not had yet. If the commonest stamp does not find the masked part either, the stamps' spread on the console wants looking at from the probe's "stamp" pair.
- **`MoveImage` cold** is 809 cycles under the model where Sony's is 972 (953 on the console): the clean call has less code to miss. Not chased.
- **A poll with requests waiting** was not timed behind an image's blocks: its pads still hold stores (`spaceApart`).
- **Layout.** Each build packs the image again, and three things now depend on it that a test or a script checks and nothing enforces: `TestVSyncPoll` (`drawPlace`), what shares lines with the two sections (`lines.py`, `packetPlace`), and `DrawSync`'s section staying under 4 KB (3.9 now).

### On the console, round 2 (to do, spec side)

1. `dmahold` (`.build/serial/dmahold.elf`, rebuilt), once, either library: the new rows are the last 28 lines. The model has stores at +4 bytes to +16 KB at 357 cycles for 32 with the blocks going, at +64 KB 380, at +256 KB and to the stack 872; stores to the stack in twos and in fours 437 for 16; a load and a store in turn 464 for 16; loads from the stack 462 for 16; multiplies 135 for 16, as alone.
2. `imgparts` (`.build/serial/imgparts.elf`, rebuilt) under `-library sony-ourmon` and under `-library clean` on `.build/console/libps-s50c.exe.gz`. Expected on s50c: the POLL lines near Sony's (3,552 idle, 3,630 behind `MoveImage`, 8,564 behind `LoadImage`, where s50a had 3,609, 3,702 and 10,636); `LoadImage` warm near Sony's row for row, and no longer growing past 24 blocks (s50a went on to 1,555); a masked part found in most lines ("mask" not -1), with "samples" near "last" less "mask".
3. `imgsync` (`.build/serial/imgsync.elf`) under `-library clean` on s50c: the polls to 0 at 256 by 256 near Sony's 183, 250, 603, 239 and 36 (s50a: 151, 220, 581, 231, 34).
4. `loadsync` (`.build/serial/loadsync.elf`) under `-library sony-ourmon` and `-library clean` on s50c: the calls, in eighths. The model has the clean `LoadImage` 5 to 9 over Sony's there still (cold), `MoveImage` 4 under and the fill 13 under.

### The console, round 2 (spec side's runs, 2026-11-12, `.build/hardware/spec50/r2`, all on s50c)

**The far-store rule was wrong.** `dmahold`, 32 stores with an image's blocks going, cycles: 331 to 334 whether they go 4 bytes from the image's first word, 1 KB to 256 KB from it, or to the stack. In twos with eight nops between and in fours with sixteen, 148 to 184 for 16 stores (84 alone). A load and a store in turn, 535 for 16 of each (212 alone), and 16 loads from the stack 427 to 474. Multiplies 136 to 145 for 16 (129 alone). So distance has nothing to do with it, and the model's two rates are one again, 2.6 a block (`costBlockNear` and the address `HoldBlocks` took are gone). What made s49h's and s50a's `LoadImage` run on to 32 blocks was not its 21 stores: with the rule as it now is they come to 23 blocks with the call's way out. Those two images' calls ran on cold code every time (the accident of the layout that round 2 found, `LoadImage` and `imageEnter` on each other's lines), and a cache line fetched while the blocks go waits a block's time as a load does: nine lines' worth. s50c, with the accident gone and six stores there, stops at 17 or 18 blocks, where Sony's stops at 24. So round 2 took out stores that were right.

`imgparts`: the masked part is found in every line now, 11 or 12 cycles of spread in the stamp. Warm, cycles, to the mask, masked, and from the unmasking to the end:

| | Sony's | s50c |
|---|---|---|
| `LoadImage` 0 by 4 | 273, 374, 85: 732 | 253, 415, 91: 759 |
| the CPU's words | 270, 659, 75: 1,004 | 253, 705, 75: 1,033 |
| 1 block | 273, 552, 74: 899 | 249, 580, 81: 910 |
| 8 blocks | 271, 704, 79: 1,054 | 251, 735, 81: 1,067 |
| 24 blocks | 1,346 | 1,252 |
| 32, 72, 128 blocks | 1,334, 1,354, 1,351 | 1,248, 1,276, 1,257 |
| `StoreImage` 0 by 4 | 269, 393, 90: 752 | 235, 437, 140: 812 |
| the CPU's words | 270, 819, 184: 1,273 | 264, 791, 137: 1,192 |
| 1 block | 273, 722, 164: 1,159 | 269, 663, 124: 1,056 |
| 8 blocks | 270, 873, 162: 1,305 | 266, 855, 106: 1,227 |
| `MoveImage` | 327, 221, 61: 609 | 329, 179, 131: 639 |
| `ClearImage`, a rectangle | 280, 550, 55: 885 | 334, 487, 123: 944 |
| `ClearImage`, a fill | 280, 440, 57: 777 | 302, 383, 80: 765 |

Cold (after a `VSync(0)`): `LoadImage` of no size 1,087 and 1,054; of the CPU's words 1,423 and 1,521; one block 1,327 and 1,394; 8 blocks 1,476 and 1,532, in parts 421, 972, 85 and 373, 1,104, 56. So the clean call's masked part is 31 cycles over Sony's warm and 132 over cold with a transfer, and with none 41 and 24: the cold excess is in the transfer's code (the wait before the GPU is asked, the image's words, the pad after the blocks start), some dozen lines of it. `loadsync`, eighths, Sony's and s50c: `LoadImage` 187 and 199 at 16 by 16, 213 and 225, 245 and 235, 245 and 234; `MoveImage` 122 and 120; the fill 155 and 146.

24 polls of `DrawSync(1)`, Sony's and s50c: idle 3,552 and 3,274; behind `MoveImage` 3,619 and 3,347; behind `LoadImage` 8,572 and 8,370; behind `StoreImage` 7,320 and 6,405. `imgsync`'s polls to 0 at 256 by 256, s50c and Sony's: `LoadImage` 183 and 183, `StoreImage` 308 and 250, `MoveImage` 648 and 603, `ClearImage` 258 and 239, the fill 38 and 36.

**Why the polls with no blocks going got shorter.** The poll is right behind `LoadImage`'s blocks, which is what round 2 changed it for, and 8% short everywhere else, where the model had it Sony's (3,586 for 3,596). s50a's was 2% long on the console and 3% long in the model. Between the two images the poll's way with nothing waiting lost two loads and two stores and gained a multiply and three nops: 5 cycles less in the model, 14 less on the console. The multiply is what the model prices and the console evidently does not: the pad read its result into `$zero`, and `dmahold`'s multiplies, which cost the model's 8 cycles, read theirs into a register. Behind the blocks the multiply's time is hidden in the wait for the bus, which is why that poll came out right. Not proven: `dmahold` has rows for round 3 with the result read into `$zero`. The pads read it into a register now.

### Round 3 (implementation side, 2026-11-12): s50d

The image is `.build/console/libps-s50d.exe.gz` (SHA-1 507b331b).

**The model.** One rate for stores, above. `TestCPUCostBlocks` has stores to two places 900 KB apart at the same 10 blocks for 30. Left as they are, with the console's figures against them: a load and a store in turn (464 cycles for 16 in the model, 535 on the console), and the transfer's 27.2 cycles a block for the console's 28.1.

**`LoadImage`'s pad after the blocks start** is six turns of a loop of a load (`loadTurns6`), with two nops before it: six loads, each of which waits a block. With the call's way out that should come to 23 blocks or so on the console, for Sony's 24, where s50c's six stores made 17 or 18. `StoreImage`'s is seven turns.

**Loops for the long pads.** `LoadImage`'s wait before the GPU is asked was 18 loads and a store one after another, five cache lines. It is `loadTurns6`, the same 38 cycles in the emulator from two lines: 3 instructions a turn, a load, a branch and a count. The pad after the CPU's words alone is `loadTurns5` where it was 16 stores. With the pad after the blocks start that is some 40 instructions fewer for a cold call to fetch. A turn costs a console about 9 cycles where three loads cost 18, so the loops also took off the 30 or so cycles the clean call's masked part was over Sony's warm.

**The other pads, from the parts.** `imageEnter` has all seven of its pad before I_MASK as loads (the clean call masked 20 cycles before Sony's) and none after (its masked part with no size was 41 over). `sendImage`'s and `readImage`'s pads between their words are loads. `StoreImage` has loads for the whole of its wait and of its pad after the CPU's words: its calls with a transfer were 80 to 100 cycles under Sony's. `packetEnter` has no loads, and `ClearImage` on a rectangle 12 nops and 4 stores after the channel starts where it had 14 stores (it was 59 over), a fill 15 loads. `MoveImage` has 10 nops and 4 stores before I_MASK is put back where it had 14 stores (30 over).

**The multiplies** read their result into a register (`mults`, `multsWide`), and `drawPoll`'s is the wide one. If the console's short poll was the read into `$zero`, the poll with nothing going comes out some 2% over Sony's on s50d; if it was something else, 3% under.

`drawPlace` is 16 instructions: of 1, 0, 3, 6, 16 and 2 only 16 passes `TestVSyncPoll` on this code (the others have one field at 174 cycles, or at 66).

Every access `imgsync` traces is again where spec 49 left it, with `LoadImage`'s first word of the CPU's now on Sony's cycle (it was 2 early), `MoveImage`'s D2_CHCR on it (2 late) and `ClearImage`'s D2_BCR 4 early (2).

Under the model, `imgparts`, warm, with the console's round 2 for Sony's and for s50c, and what that makes of s50d on a console (s50d's model figure and s50c's console figure less its model figure):

| | Sony's, console | s50c, console | s50c, model | s50d, model | s50d, expected |
|---|---|---|---|---|---|
| `LoadImage` 0 by 4 | 732 | 759 | 751 | 746 | 754 |
| the CPU's words | 1,004 | 1,033 | 1,028 | 996 | 1,001 |
| 1 block | 899 | 910 | 907 | 905 | 908 |
| 8 blocks | 1,054 | 1,067 | 1,061 | 1,029 | 1,035 |
| 24 blocks and more | 1,346 | 1,252 | 1,300 | 1,253 | about 1,320 (four blocks more than s50c) |
| `StoreImage` 0 by 4 | 752 | 812 | 801 | 758 | 769 |
| the CPU's words | 1,273 | 1,192 | 1,154 | 1,206 | 1,244 |
| 1 block | 1,159 | 1,056 | 990 | 1,083 | 1,149 |
| `MoveImage` | 609 | 639 | 608 | 594 | 625 |
| `ClearImage`, a rectangle | 885 | 944 | 933 | 872 | 883 |
| `ClearImage`, a fill | 777 | 765 | 798 | 757 | 724 |

The fill is the one that looks set to come out short, 7%: its end got 37 cycles cheaper in the model when the stores before it went (a load after a store waits for it), which I saw too late for this image. Cold under the model, s50d and Sony's: `LoadImage` of no size 1,105 and 1,140, the CPU's words 1,482 and 1,484, a block 1,387 and 1,388, 8 blocks 1,513 and 1,500; s50c had 1,525 and 1,563 for the words and 8 blocks. `loadsync` under the model, eighths, s50d and Sony's: 188 and 190, 208 and 210, 215 and 220, 215 and 220, where s50c had 199, 218, 225 and 225. The console had s50c 12, 12, -10 and -11 against Sony's.

24 polls under the model, s50d and Sony's: idle 3,741 and 3,596, behind `MoveImage` 3,806 and 3,675, behind `LoadImage` 8,583 and 9,209 (s50c's is 8,583 less the multiply too, now that the model has one rate for stores, and the console had s50c's at 8,370 for Sony's 8,572).

### Checks on s50d

- `TestImageSync` and spec 47's nine pass on s50d's discs (`.build/s50/tests-d2.log`). `TestCPUCostBlocks` and the rest of `go test ./emu/... ./exp/cleanlib/...` pass.
- `image/regress.sh`: 38 PASS, 3 TIME, as before.
- `pollsync`, costs off: every poll's accesses on Sony's cycles. Costs on: 145 idle, 148 to 157 with the GPU drawing, 172 with requests waiting.
- `archive.sh`: 99 the same (`.build/s50/summary-d.txt`), the one line that differs from the unchanged tree's being `scee-topten-hover` as on s50c (570 picture changes on both, the first difference in frame 605 on both).
- The static clean build compiles with and without the fidelity tag.

### Open after round 3

- **The fill**, expected 7% short on the console, above. A divide in its pad (38 cycles for two instructions, no access to the bus) would make it up.
- **`LoadImage` past 24 blocks**: six loads may leave it a block short of Sony's 24. A seventh turn does not fit the pad's 21 instructions with the call's accesses where they are.
- **The model under blocks** has Sony's own poll 7% long (9,209 for the console's 8,572) and Sony's `imgsync` polls behind `LoadImage` at 154 for the console's 183, and a load and a store in turn 13% short. Its relative figures held for s50c's poll (3% under Sony's in the model, 2% on the console). `StoreImage`'s blocks are still not in it.
- **Cold calls**: the model has not predicted the console's cold figures to better than 60 cycles in either round. s50d's are set by the reasoning (fewer lines), with the model agreeing.
- **Layout**, as after round 2: `TestVSyncPoll` passes for one size of `drawPlace` in six on this code.
- A poll with requests waiting behind an image's blocks: not timed, and its pads still hold stores.

### On the console, round 3 (to do, spec side)

1. `dmahold` (`.build/serial/dmahold.elf`, rebuilt), once: the last three lines, multiplies with the result read into `$zero` and into a register. The model has 133, 245 and 245 cycles for 16. If the first two come out near 30, the read into `$zero` does not wait, and round 2's short poll is explained.
2. `imgparts` (`.build/serial/imgparts.elf`, as round 2's) under `-library clean` on `.build/console/libps-s50d.exe.gz`. Sony's side is round 2's. Expected: the table above; the POLL lines idle and behind `MoveImage` within 3% of Sony's 3,552 and 3,619 (s50c: 3,274 and 3,347), behind `LoadImage` near s50c's 8,370; `LoadImage` growing to 24 blocks and then flat near 1,320; cold `LoadImage` with a transfer nearer Sony's than s50c's 56 to 98 over.
3. `imgsync` on s50d: polls to 0 at 256 by 256 near Sony's 183, 250, 603, 239, 36 (s50c: 183, 308, 648, 258, 38).
4. `loadsync` on s50d: `LoadImage` near Sony's 187, 213, 245, 245 (s50c: 199, 225, 235, 234).

### The console, round 3 (spec side's runs, 2026-11-12, `.build/hardware/spec50/r3`, on s50d)

**The read into `$zero` was not it.** `dmahold`: 16 multiplies with the result read into `$zero` 129 cycles, with a wide first operand 241 either way. So a multiply costs what the model has wherever its result goes, and why s50c's poll came out 8% short where the model had it Sony's is not explained. s50d's, with the wide multiply, is 3% short, which is what the other reading predicted.

`imgparts`, warm, cycles, Sony's (round 2), s50d, and what round 3 expected of s50d:

| | Sony's | s50d | expected |
|---|---|---|---|
| `LoadImage` 0 by 4 | 732 | 750 | 754 |
| the CPU's words | 1,004 | 989 | 1,001 |
| 1 block | 899 | 890 | 908 |
| 8 blocks | 1,054 | 1,038 | 1,035 |
| 18 blocks | 1,240 | 1,237 | |
| 24 blocks | 1,346 | 1,284 | about 1,320 |
| 32, 72, 128 blocks | 1,334, 1,354, 1,351 | 1,277, 1,291, 1,251 | |
| `StoreImage` 0 by 4 | 752 | 754 | 769 |
| the CPU's words | 1,273 | 1,203 | 1,244 |
| 1 block | 1,159 | 1,102 | 1,149 |
| 8, 32, 128 blocks | 1,305, 1,467, 1,471 | 1,260, 1,422, 1,433 | |
| `MoveImage` | 609 | 607 | 625 |
| `ClearImage`, a rectangle | 885 | 875 | 883 |
| `ClearImage`, a fill | 777 | 718 | 724 |

Cold, Sony's and s50d: `LoadImage` of no size 1,087 and 1,076, the CPU's words 1,423 and 1,438, a block 1,327 and 1,326, 8 blocks 1,476 and 1,461, 24 blocks 1,814 and 1,810, 32 and more 1,927 and 1,840. `loadsync`, eighths: 187 and 186 at 16 by 16, 213 and 212 at 24 by 24, 245 and 228, 245 and 231; `MoveImage` 122 and 117; the fill 155 and 149. So the 9 to 12 eighths this spec began with are gone: the clean `LoadImage` is Sony's within an eighth cold at 16 by 16 and 24 by 24 and within 2% warm up to 18 blocks. What is left of it is past 24 blocks, where the clean call is some 60 cycles short (it is done three blocks sooner than Sony's after starting them), 17 and 14 eighths in `loadsync`'s cold calls.

24 polls of `DrawSync(1)`, Sony's and s50d: idle 3,552 and 3,439 (3% short); behind `MoveImage` 3,630 and 3,502 (3.5%); behind `LoadImage` 8,564 and 8,393 (2%); behind `StoreImage` 7,320 and 6,426 (12%). `imgsync`'s polls to 0 at 256 by 256, s50d and Sony's: `LoadImage` 183 and 183, `MoveImage` 611 and 603, `ClearImage` 243 and 239, the fill 36 and 36, `StoreImage` 310 and 250.

**The poll behind `StoreImage`.** 24% more polls from a poll 12% shorter: so the transfer itself lasts some 8% longer under the clean library's polling than under Sony's. With an image coming from the GPU the bus is open longer in each block than with one going to it (`dmahold`, round 1: two loads from RAM a block against one, three or four stores against two or three, a block every 34.5 cycles against 28.1). Behind `LoadImage` the clean poll waits out 12.4 blocks and Sony's 12.7; behind `StoreImage` 7.8 against 8.8. Every kind of access is cheaper behind `StoreImage`, a load from RAM by half and a store or a port read by a quarter, so Sony's poll must make fewer loads from RAM and more stores or port reads than the clean one, which makes about twelve loads and no store: that mix both waits longer there and holds the transfer up less. To match it the clean poll would trade loads for stores, 2.6 stores for a load to keep its time behind `LoadImage`, and it has nowhere to put them: the path has nine instructions of pad before D2_CHCR and four after, in a call whose cycles in the emulator are Sony's to the cycle, and a store costs a console a third of what a load does with nothing going, so the idle poll would lose what the poll behind `StoreImage` gained. It would take a `drawPoll` with fewer instructions of its own (in assembly, as `GsSortClear` is) to make the room, and a model of blocks from the GPU to set it by. Left as it is and recorded: not a matter of one more round.

### Where it ends: s50d

s50d is the image this spec leaves (`.build/console/libps-s50d.exe.gz`, SHA-1 507b331b), and the tree builds it byte for byte. Its checks are above ("Checks on s50d"), and the console's figures for it are round 3's.

A further image, s50e, was built and dropped. It had four small changes aimed at what round 3 left: a second multiply in the idle poll (3% short), two more loads after `LoadImage`'s blocks start (three blocks short past 24), and two divides each in the fill's pad (7.6% short) and `StoreImage`'s wait (5% short with a transfer). It passed the ten tests. Under the model it did what was meant for the idle poll (4% up on s50d), the fill (85 cycles up) and `StoreImage` (28 up), and it also made the poll behind `LoadImage`'s blocks 7% longer, where s50d's is 2% from Sony's on the console, `ClearImage`'s rectangle 33 cycles longer for no change to it, and `loadsync`'s cold `LoadImage` 7 eighths longer at 16 by 16, where s50d's is Sony's within an eighth on the console. Each of those is the layout or a block's phase moving, not the change itself, and none can be told from the model well enough to send a console image on. s50d's figures are measured. So the four are left as what a next spec could take one at a time, each with its own console run.

### What is left, with the console's figures for s50d

| | Sony's | s50d |
|---|---|---|
| `LoadImage`, warm, to 18 blocks | 732 to 1,240 | within 2.5% |
| `LoadImage`, warm, 24 blocks and more | 1,334 to 1,354 | 1,251 to 1,291: 4 to 7% short |
| `LoadImage`, cold, to 24 blocks | 1,087 to 1,814 | within 1.1% |
| `LoadImage`, cold, 32 blocks and more | 1,927 | 1,840: 4.5% short |
| `StoreImage`, no size | 752 | 754 |
| `StoreImage`, with a transfer | 1,159 to 1,471 | 2.6 to 5.5% short |
| `MoveImage` | 609 | 607 |
| `ClearImage`, a rectangle | 885 | 875 |
| `ClearImage`, a fill | 777 | 718: 7.6% short |
| 24 polls, idle | 3,552 | 3,439: 3.2% short |
| 24 polls behind `MoveImage` | 3,630 | 3,502: 3.5% short |
| 24 polls behind `LoadImage` | 8,564 | 8,393: 2% short |
| 24 polls behind `StoreImage` | 7,320 | 6,426: 12% short |
| polls to 0 at 256 by 256: `LoadImage`, `MoveImage`, `ClearImage`, the fill | 183, 603, 239, 36 | 183, 611, 243, 36 |
| polls to 0 behind `StoreImage` | 250 | 310 |

Where spec 49's image stood on the same console: `LoadImage` 9 to 12 eighths over Sony's at every size; polls to 0 of 170, 236, 723, 287 and 43.

Not done:

- **The poll behind `StoreImage`**, above ("The poll behind `StoreImage`"): it wants a `drawPoll` with room for stores and a model of blocks coming from the GPU.
- **`LoadImage` past 24 blocks**, **the fill**, **`StoreImage` with a transfer** and **the idle poll**: the four changes s50e tried, to be taken one at a time against the console.
- **Why s50c's idle poll was 8% short** where the model had it Sony's. Not the multiply's destination (round 3).
- **The model**: blocks from the GPU are not in it; a load and a store in turn under blocks is 13% short; Sony's own poll behind blocks is 7% long; cold calls are not predicted to better than 60 cycles.
- **Layout**: `TestVSyncPoll` goes by `drawPlace`'s size (one size in six passed on s50d's code, seven in eight on s50e's), and the two sections' lines by `packetPlace`'s. Nothing enforces either but the test and `.build/s50/lines.py`. A way to pin the sections (an alignment the packing honors) would end the lottery.
- A poll with requests waiting behind an image's blocks is not timed.
