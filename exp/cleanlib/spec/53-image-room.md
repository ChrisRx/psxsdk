# Room in the image

## Why

After spec 52 the clean image ended at 80067FA8h, 88 bytes below 80068000h, where a Net Yaroze's monitor starts (spec 29). Every accuracy fix still to come needs room, and so does anything the API work adds to the resident image. With 88 bytes, the next change of any size would stop the build.

A third of the image's `.bss` was the Go runtime's heap: `gen.py` reserved 32 KB for it as `libps_image_heap`. The image uses TinyGo's leaking collector, a bump allocator. Its pointer, `runtime.heapptr`, is written once, by `libps_image_start`, and read by nothing (`llvm-objdump` of the clean `image.elf`), and `runtime.alloc` isn't linked in. The library allocates nothing, so the heap was never used.

`gen.py` is the spec side's (CLEANROOM.md), so this is spec-side work.

## What to do

1. Shrink the heap to what the runtime needs to start.
2. Make an allocation stop the build. With no heap, the first allocation at run time would stop the console with no message.
3. Show that nothing else changes.

## Testing it

The ten timing tests on the new image, `image/regress.sh`, `archive.sh`, and the console's `imgsync`, `loadsync` and `kanjisource`.

## Results

Done 2026-11-13 (spec side).

### What changed

- **`gen.py`**: the heap is `HEAP = 8` bytes, where it was 0x8000.
- **`make.sh`**: the build stops if `runtime.alloc` is in the image. TinyGo marks `alloc` `//go:noinline`, so it is linked in exactly when something allocates.
- `gc: none` was tried first, since it would turn any allocation into a link error by itself. This TinyGo's runtime names `alloc` in `chan.go` whatever the collector, so it doesn't compile.

The image (`.build/console/libps-s53a.exe.gz`, SHA-1 1a6df872) ends at 80066288h. That is the end of Sony's fixed variables `PSDCNT` and `PSDIDX` (800661E0h to 800661E8h) and 8 bytes of `.got`. It can't end lower, because everything packed now fits in the gaps below them. The room is:

- 7,544 bytes between 80066288h and 80068000h, up from 88;
- most of the rest of the 32 KB as free space in the gaps. A new section takes it if it fits a gap, and goes at the end if not.

The pinned sections (spec 52) are where they were.

### Checks

- The ten timing tests pass: TestDrawWait, TestFieldTime, TestQueuedImages, TestDMALeft, TestVSyncCount, TestVSyncPhase, TestIRQGaps, TestVSyncPoll, TestQueueLength and TestImageSync.
- `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), 0 FAIL, as before.
- `archive.sh`: 99 the same, 19 differ, 13 still, 6 shift, 1 noboot, 1 tty and 1 wild. Against the summary from before spec 50, two lines changed:
  - `scee-topten-grav` went from shift to same. That is spec 50's 99.
  - `event-event003-theme-krwire` went from stall to wild (2760F894h on clean). It does the same on spec 52's image with the 32 KB heap, so specs 50 to 52 moved it, not this one. Spec 38 found the program at fault on both sides, entering the library with sp 0, and spec 43 saw it flip between stall and wild with layout before.
- The console (`.build/hardware/spec53`, clean library), against spec 52's pinned image:

| | Sony's | spec 52 (8a61c8ce) | spec 53 (1a6df872) |
|---|---|---|---|
| `imgsync` 256 by 256 polls: Load, Store, Move, Clear, Fill | 183, 250, 603, 239, 36 | 183, 307, 611, 243, 36 | 183, 310, 610, 243, 36 |
| `loadsync` LoadImage 16, 24, 32, 48 square | 187, 213, 245, 245 | 189, 214, 231, 234 | 188, 214, 231, 234 |
| `loadsync` MoveImage 16, 32 square; fill | 122, 120; 155 | 119, 118; 151 | 118, 118; 151 |

  `kanjisource` gives the same lines as spec 51's candidate.
