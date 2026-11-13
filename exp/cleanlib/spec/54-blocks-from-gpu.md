# Blocks from the GPU

## Why

Spec 50 left one poll far from Sony's on the console: polling `DrawSync(1)` to 0 behind a `StoreImage` of 256 by 256 takes the clean library 310 polls where Sony's takes 250 (spec 53's image, 1a6df872). Behind `LoadImage`, `MoveImage`, `ClearImage` and the fill the two are within 2%. An image coming from the GPU took the emulator no time, so the model had no blocks to hold the bus with and couldn't show the difference, and spec 50 couldn't pace `drawPoll` against it.

`emu/` is the spec side's, so the model is spec-side work. Pacing `drawPoll` from it would be the implementation side's.

## What to do

1. Give an image coming from the GPU by channel 2 its time under `SetDrawTime`, and have its blocks hold the bus under `SetCPUCost`, fitted to `dmahold`'s console figures (spec 50).
2. Check the model against the console's polls behind `StoreImage`, Sony's and the clean library's.
3. If it holds, write the pacing up for the implementation side.

## Testing it

`dmahold` in the emulator against spec 50's console run. `imgparts` (its POLL lines) and `imgsync` on both libraries under the model against the console. New console probes where the model and the console disagree.

## Results

Done 2026-11-13 (spec side). The model gives the transfer its time, and its bus rule fits `dmahold`, but it gets the two polls the wrong way round, and the console shows why no rule of that kind can fit. `drawPoll` is not paced: the gap stays open.

### The transfer's time

`emu/gpu_time.go`: `timeGPURequest` now times a transfer from the GPU as well as one to it, at `imageFromCyclesHundredths` (2.16 cycles a word, 34.6 a block of 16). The console's `dmahold` has 2,215 cycles for 64 blocks and 8,930 for 256; the emulator has 2,221 and 8,855. The channel stays busy, and the GPU not ready for a command, until the words have gone, as for an image going to it.

### The bus rule

`HoldBlocks` as spec 50 made it, with two holds a block where an image going to the GPU has one, each open to the CPU for its last 4.2 cycles (`imageFromSplit`, `imageFromOpenTenths` in `emu/cpucost.go`). That is the best of one to four holds a block, open from 2.5 to 13 cycles, against `dmahold`'s rows from the GPU:

| run | console | model |
|---|---|---|
| lw from RAM x64 | 1,106 | 1,119 |
| lw from RAM x16 | 320 | 294 |
| lw and a nop x32 | 667 | 569 |
| lw and seven nops x16 | 361 | 294 |
| sw to RAM x64 | 624 | 622 |
| sw and three nops x32 | 376 | 328 |
| sw and seven nops x16 | 219 | 173 |
| lw GPUSTAT x32 | 432 | 569 |
| lw GPUSTAT and seven nops x16 | 358 | 294 |
| lw I_STAT x32, lw D2_CHCR x32 | 432 | 466 |
| the spin loop x32 | 1,210 | 1,104 |

So loads and stores in a run are right, a run of port reads is 8 to 32% long, and an access with nops before the next is 15 to 19% short.

### The polls under it

| | console, Sony's | console, clean | model, Sony's | model, clean |
|---|---|---|---|---|
| `imgparts`, 24 polls behind `StoreImage` | 7,320 | 6,426 | 6,402 | 6,685 |
| `imgparts`, 24 polls behind `LoadImage` | 8,572 | 8,393 | 9,189 | 8,372 |
| `imgsync`, polls to 0 behind `StoreImage` 256 by 256 | 250 | 310 | 305 | 272 |

On the console Sony's poll behind `StoreImage` is 14% longer than the clean one, and in the model 4% shorter. The clean poll's own time is right (6,685 against 6,426), and Sony's is 13% short. The model would steer the clean poll the wrong way.

### Whether the CPU slows the transfer (`dmastretch`)

Spec 50 put the gap down to the transfer lasting longer under the clean poll. **`probes/dmastretch`** (new) starts 64 blocks from the GPU, or to it, runs one kind of access, then polls D2_CHCR to the end. The console (`.build/hardware/spec54/r1`), medians:

- From the GPU, every run ends the transfer between 2,190 and 2,220 cycles, the poll alone 2,220. Loads, stores and port reads in any number do not slow it.
- To the GPU, stores slow it 20 to 50 cycles (1,840 to 1,872 for 1,821 alone), and the spin loop 120.

So the transfer from the GPU takes the same time under either library's poll. The difference is all in what each poll's accesses wait for.

### When an access waits (`dmaphase`)

**`probes/dmaphase`** (new) starts the transfer, runs d nops from the cache (no bus), then one access, and times that against the nops alone, for d from 0 to 70. The accesses are a load from RAM, a read of GPUSTAT, and a store followed by a load. The console, extra cycles for the access at each d (median of six):

- To the GPU it is the model's within 4 cycles everywhere: 27 in the held part of a block, falling to 23 at its end, then 0 in the open part, 28 cycles a block. That checks the probe.
- From the GPU the bus is busy from d 0 to 9, 26 to 35 and 52 to 69, and free from 10 to 25 and 36 to 51. So the spans aren't periodic, and the first blocks come faster, presumably while the GPU's FIFO still holds words. In a busy span a load waits 22 to 32 cycles wherever in the span it arrives, longer than the span itself. A store followed by a load waits 34 to 39 cycles in the first free span, where the load alone waits 2, and 5 in the second.

No rule of the kind `HoldBlocks` has fits that. It holds the bus at fixed times, and an access waits only for the next open part. The console's bus behaves as an arbiter that serves the DMA and the CPU in turns, with a cost that depends on what came before. A model of it would need more console rounds: d past 70, pairs of accesses at set distances, and a store alone.

### What stays

- The transfer's time from the GPU, which a program waiting on `DrawSync(0)` or D2_CHCR behind `StoreImage` now sees: before this, it ended at once.
- The bus rule above. It is better than none for the clean poll's own time and for loads and stores in runs, but it must not be used to pace a poll against Sony's behind `StoreImage`.
- `probes/dmastretch` and `probes/dmaphase`, with the console's output in `.build/hardware/spec54/r1`.

### Checks

- `go test ./emu/...` passes.
- Nine of the ten timing tests pass as they were. `TestImageSync` failed on three `StoreImage` rows on both libraries alike. Its table is Sony's library in the emulator, recorded when a transfer from the GPU ended at once. Sony's side now gives `DrawSync(1)` 118 cycles returning 1 at 16, 64 and 256 square, then 0, 39 and 653 polls to 0, and the clean library gives the same. The table has those now, and the test passes on both.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL, as before.

### Left open

- **The poll behind `StoreImage`**: 310 polls against Sony's 250 on the console. Pacing it needs either a model of the arbiter above or a pace found on the console itself, a round at a time.
