# MoveImage's cache lines

## Why

On the console a warm `MoveImage` took the clean library 658 cycles on spec 53's image (1a6df872) and 659 on spec 55's, where Sony's takes 609 and spec 50's image (507b331b) took 607. The loss came in with the image that merged specs 50 and 51, before spec 55.

Pins are the spec side's (`image/pins.txt`), so this is spec-side work.

## What was found

From `llvm-nm` on the clean `image.elf`. The place in the 4 KB cache is the address's low 12 bits.

- `MoveImage` is at 800141D8, 51Ch bytes, on cache 1D8 to 6F3.
- `spin`, which `MoveImage` calls on every call, was pinned at 80043368, on cache 368 to 393: four of `MoveImage`'s lines. Each call refetched them.
- `drawNext`, which runs between two image calls, was packed at 8002A1B8, on cache 1B8 to 95B, over all of `MoveImage`.

## What changed

`image/pins.txt` only:

- `spin` moves to 8001AD5C (cache D5C to D87), its address in spec 50's image.
- `drawNext` is pinned at 80016990 (cache 990 to 133), clear of `MoveImage` (1D8 to 6F3) and `DrawSync` (714 to 873). It shares 990 to 9CF with the end of `drawPoll`.

The image is `.build/console/libps-s56b.exe.gz`, d0751133. It ends at 80066288h as before.

## Results

Done 2026-11-13 (spec side). The console (`.build/hardware/spec56/b`), cycles:

| | Sony's | spec 55 | spec 56 |
|---|---|---|---|
| warm `MoveImage` | 609 | 659 | 609 |
| `LoadImage` 0 by 4 | 732 | 784 | 740 |
| `LoadImage` 16 by 16 | 1,054 | 1,056 | 1,027 |
| `StoreImage` 0 by 4 | 752 | | 775 |
| `StoreImage` 16 by 16 | 1,305 | | 1,218 |
| `ClearImage` | 885 | 870 | 877 |

`qfull`: the whole queue takes 25,741 to 26,025 cycles against Sony's 25,427 to 25,609, 1.2 to 1.6% over where spec 55 was 2%. The last call takes 122 to 123 against Sony's 130. `imgsync` (polls 184, 311, 611, 243, 36), `loadsync` and `kanjisource` are as on spec 55's image.

### Checks

- The ten timing tests pass.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL.

### Left open

- **TestVSyncPoll under `GEN_SHIFT`.** Spec 52 had it pass with the packing shifted 16 to 128 bytes. On this image it fails at 32 and 64 and passes at 16, 48, 96 and 128. Spec 55's committed pins already fail at 32 (and pass at 64), so it predates this spec. The failure is `VSync(0)` returning 174 cycles after the interrupt, against 80 to 155. Every function the VSync path runs often is pinned (watch trace), and pinning `libps_syscall`, `spuCount`, `enqueue` and `DRAWENV.put` as well did not help. What still moves is the library's data: the emulator charges a store to another 1 KB DRAM row more than one to the same row, as the console does, so the shift changes what the interrupt path's stores cost. Spec 57 pins that data.
