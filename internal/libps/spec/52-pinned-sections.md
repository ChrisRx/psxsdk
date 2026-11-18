# The image's timing doesn't move when its size does

## Why

Spec 50 found that TestVSyncPoll, run under a console's costs, passes for one size of `drawPlace` in six. `drawPlace` does nothing, so its size can only move code. `gen.py` packs the image's sections into the gaps between the resident entry points, largest first, each into the first gap it fits. A change of size anywhere moves most of what is packed after it, and with it which lines of the 4 KB instruction cache the interrupt, VSync and the image calls share. A program that waits on VSync then sees a different time, and any later change to the library (the API work above all) could break TestVSyncPoll, or move a timing the console has confirmed, with nothing to show why.

`gen.py` holds the resident address table, so this is the spec side's work, not a clean-room agent's (CLEANROOM.md).

## What to do

1. Let `gen.py` place a list of sections at fixed addresses before it packs the rest. The gaps between the entry points never move, so a pinned section stays where it is whatever the rest of the image does.
2. Pin the sections on the interrupt, VSync and image call paths at the addresses where the image that passed specs 50 and 51 has them.
3. Show that TestVSyncPoll holds when the rest of the packing moves, and that it doesn't without the pins.

## Testing it

`GEN_SHIFT=N` reserves N bytes at the start of each gap of 1 KB or more, which moves what the packing puts there as a change of size would. `GEN_PINS=/dev/null` turns the pins off. Both are for testing only. TestVSyncPoll runs at several shifts with and without pins. Then the ten timing tests, `image/regress.sh` and the console's `imgsync`, `loadsync` and `kanjisource` run on the pinned image.

## Results

Done 2026-11-12 (spec side).

### What changed

- **`exp/cleanlib/image/pins.txt`** is new. It lists 18 sections, each with its address:
  - the interrupt and VSync: `vblankInterrupt`, `libps_interrupt`, `libps_intr_entry`, `tickExitPace`, `libps_vsync_field`, `libps_vsync_wait`, `libps_wait_blanks`, `spin`, `VSync` and `VSyncCallback`;
  - the image calls: DrawSync's section (with `drawPoll`, `LoadImage` and `StoreImage`), MoveImage's section (with `ClearImage`), `drawPollMove`, and `cdWatch.check` and `cdWatch.begin`;
  - the TTY, which a program prints through between the waits it times: `putchar`, `ttyPut` and `libps_biosB`.
- **`gen.py`** cuts each pinned section out of its gap before packing, and stops the build with a message if one no longer fits.
- **`CLEANROOM.md`** says agents may read `pins.txt`, and that a build stopped by a pin is reported to the spec side, not fixed by editing the file.

The pinned image is `.build/console/libps-s52a.exe.gz`, SHA-1 8a61c8ce. It differs from the merged spec 50 and 51 image (9dc3438b) only in where the packing put the unpinned sections.

### Finding what to pin

The first list had the 11 sections spec 50 grouped or named. Shifting one gap moved none of them, so every build passed, pinned or not: that test showed nothing. Shifting every gap of 1 KB or more moved four of them, and TestVSyncPoll gave:

| Shift (bytes) | No pins | 11 pins |
|---|---|---|
| 16 | fail | fail |
| 32 | fail | pass |
| 48 | fail | pass |
| 64 | pass | fail |
| 96 | fail | pass |

To find what else mattered, the vsyncpoll probe ran in the emulator with `PROBE_WATCH` on every function of the clean `image.elf`. Each hit after the image's entry was counted by the input section it falls in (`map2.txt`). Hits before the entry are the BIOS's shell, which runs from RAM near 80030000 while the disc boots. Seven sections that ran were not pinned: `libps_intr_entry` and `tickExitPace` once per interrupt (396 and 387 times), `spin` (3,263), `libps_wait_blanks` (144), and the TTY's `putchar`, `ttyPut` and `libps_biosB`. With them pinned too, at the addresses the packing had already given them, the image is byte for byte the same as with 11 pins, and:

| Shift (bytes) | 18 pins |
|---|---|
| 16 | pass |
| 32 | pass |
| 48 | pass |
| 64 | pass |
| 96 | pass |
| 128 | pass |

### Checks on the pinned image

- The ten timing tests pass: TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll, TestQueueLength and TestImageSync.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL, as before.
- The console (`.build/hardware/spec52`, clean library), against the merged image's runs (`.build/hardware/spec5051`) and Sony's:

| | Sony's | merged (9dc3438b) | pinned (8a61c8ce) |
|---|---|---|---|
| `imgsync` 256 by 256 polls: Load, Store, Move, Clear, Fill | 183, 250, 603, 239, 36 | 183, 308, 611, 243, 36 | 183, 307, 611, 243, 36 |
| `loadsync` LoadImage 16, 24, 32, 48 square | 187, 213, 245, 245 | 189, 214, 232, 232 | 189, 214, 231, 234 |
| `loadsync` MoveImage 16, 32 square; fill | 122, 120; 155 | 118, 118; 151 | 119, 118; 151 |

  `kanjisource` gives the same lines as spec 51's candidate.

### Left open

- `cdWatch.begin` fills its gap exactly, so it can't grow without a new place. `gen.py` prints the least room after a pinned section on each build.
- The pins hold what the vsyncpoll probe runs. A path no test covers (sound, the CD, the pads under a program's own loop) is still packed by size, and its time can still move with the image's size.
- `drawPlace` and `packetPlace` exist only to steer the packing. With the pins they move nothing that matters, but they still take time where they're called, so removing them is library work with its own pacing, for a later spec.
- The image has to end below 80068000h, where a Net Yaroze's monitor starts (spec 29). The pinned image ends at 80067FA8h, 88 bytes short of it: pinning costs the packing 16 bytes. The shifted builds end past it, but they are for the emulator only.
