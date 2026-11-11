# Four differences left open by specs 39 to 44

## Why

Each of the last specs noted a difference from Sony's library and left it for later. They are small, but each is a place where a program can see the clean library behave otherwise, and two tests fail on both images. Take them in this order. Each part stands alone: finish, check and write up one before the next.

## 1. The vertical blank is counted late every other field

On a console, at 640 by 480 interlaced, the clean library's count of vertical blanks (what `VSync(-1)` returns, and what a waiting `VSync` waits on) moves about 11 lines late every other field: the intervals between counts are 274 and 251 lines, against Sony's 263 and 262 (spec 42, "A second, smaller difference"). Spec 42 found that the clean library counts the blank in its handler after the kernel's own handlers have run, the pad driver's among them. With a controller plugged in or not, Sony's is the same.

Find, with probes in the emulator, where in the vertical blank interrupt each library counts the blank, in cycles from the interrupt, with the pad driver running as the boot disc leaves it. If the emulator can't show 11 lines, say what it lacks (for instance the pad driver's time on a console, or the controller's replies). Make the clean library count when Sony's does. Spec 42's note on the 480-line wait says what a late count does to `VSync(0)`.

## 2. The request queue's length

`probes/qlen` (spec 41) queues 80 `LoadImage` calls of 16 by 16 behind a clear and one box, timing each call and reading `DrawSync(1)`. Sony's library took 63 images behind the drawing and the 64th waited in the call. The clean one took 31. `DrawSync(1)` on Sony's counts 1 after the first image and one more for each image after; the clean one counts one more than that (spec 39's 1 against 2).

Make the clean queue hold what Sony's holds, and `DrawSync(1)` count what Sony's counts, checked with `qlen` on both images. `qlen` is slow in the emulator; cut it down if needed, keeping what it measures.

## 3. What a program leaves when it returns

After `probes/rerun` returns, Sony's library leaves DPCR at 3B3B3B33h and D2_CHCR at 00000401h, and the clean one 08089899h and 0, with or without `ResetGraph(3)` (spec 44, "Found in passing"). The next program on the same load starts with those.

Find which calls set DPCR and D2_CHCR on each side (start-up, `ResetGraph(0)`, `(1)`, `(3)`, the last transfer), and make the clean library leave what Sony's leaves. Spec 44's `TestRerun` can read them after each run.

## 4. Two tests that fail on both images

- **`TestDrawWait`'s reset lines** expect the second `GsDrawOt` after `ResetGraph(1)` to take 30,000 to 45,000, the old emulator's model. Since 5e04f42 the emulator stops a timed drawing at GP1(00h) and (01h), as spec 40 measured on a console, and it takes 55 eighths. Set the test's expectations from Sony's image in today's emulator.
- **`TestFieldTime`'s 1 box, 32 images line** expects 19,276 µs ±40. Since c31746e (an image's DMA takes time) Sony's image gives 18,999 and 19,684 µs for 8 and 32 images behind one box, and the clean one 18,949 and 19,444. Each 16 by 16 image takes 698 cycles from one to the next on the clean side, 218 of them its DMA. That is a real difference of 240 µs over 32 images: find it and make the clean side take Sony's time, then set the test's numbers from Sony's image.

## Testing it

A test per part where a probe can show it, as `TestFontWake` and `TestFifthRun` do, failing before and passing after. The verify chain against a new baseline (`.build/verify57/chain.sh` copied to `.build/verify58`, run before the first change); `image/regress.sh`, `regress.sh`, `terra.sh`, `go test ./exp/cleanlib/...`, and `TestDrawWait`, `TestFieldTime`, `TestQueuedImages`, `TestVSyncPhase`, `TestFontWake` and `TestFifthRun`. The spec side runs `vsynclate`, `vsyncphase`, `qlen`, `rerun` and gasgar on the console after.

## Results

Done 2026-11-10 in the emulator. Not tried on a console.

### 1. Where the vertical blank is counted

`probes/vsynccount` (new) sets up gasgar's 640 by 480 interlaced display, with the controller driver running as the library's start leaves it on the probe disc, installs a `VSyncCallback` that adds 1 to a scratchpad word and keeps what `VSync(-1)` gives, and then reads `VSync(-1)` in a loop, writing each new value to the scratchpad. `TestVSyncCount` (new, `image/probetest/vsynccount_test.go`) runs it and times 24 vertical blank interrupts from outside, in cycles from the step I_STAT bit 0 is seen set: when the CPU takes the interrupt, the controller driver's accesses to the controller port, the write to I_STAT that clears bit 0, the change of the library's count (the one word of RAM that holds what `VSync(-1)` returned), the callback's write and the program's.

In the emulator, with the scph1001 BIOS:

| cycles from I_STAT bit 0 | Sony's | clean before | clean now |
|---|---|---|---|
| interrupt taken | 0 | 0 | 0 |
| controller port, first and last access (41 accesses) | 1,088 to 7,656 | 1,088 to 7,656 | 1,088 to 7,656 |
| the library's read of I_STAT | 10,208 | 10,188 | 10,188 |
| blank acknowledged in I_STAT | 10,246 | 10,206 | 10,246 |
| blank counted | 10,298 | 10,218 | 10,298 |
| callback's write | 10,326 | 10,244 | 10,326 |
| first envelope read of the sound tick | 10,568 | 10,568 | 10,568 |
| program sees the new count | 15,150 to 15,610 | 15,140 to 15,610 | 15,150 to 15,610 |

The same in every field, after the odd field and after the even. Both libraries count the blank in the handler the kernel goes to at the end of the interrupt (`SetCustomExitFromException`), after the kernel's own handlers, the controller driver's among them, and before the callback: the callback's `VSync(-1)` gives the new count on both. Sony's adds nothing to the kernel's interrupt chains: after its start-up they hold the kernel's own handlers only, the controller driver's in chain 2 (`TestRerun`'s dump). A program sees the count when the interrupt ends, on both libraries at the same cycles. `vsynccount` prints the lines between the counts it saw: 263 after each field on both libraries.

So the emulator does not show the 11 lines. The order is the same on both libraries, and nothing in either handler differs between the fields. What the emulator lacks to show it is not clear from here. Its controller exchange takes 6,568 cycles (about 3 lines) at every blank, the same on both libraries; a console's driver may take much longer, but it is the kernel's on both libraries and runs before either library's handler. Whatever makes the clean library's count late after one field of two on a console is outside what these probes see in the emulator: the boot disc's monitor between the library's load and the program, the clean library's start-up running at the program's first `ResetGraph(0)` where Sony's ran at boot (spec 44), the access card in slot 1, or the controller's replies. A run of `vsynccount` on the console would tell whether the late count is in the interrupt (the callback's word would come late with it) or in when the program gets to see it.

What did differ in the emulator, the clean library acknowledging and counting the blank 40 and 80 cycles before Sony's, is changed: `libps_interrupt` spends 17 stores before the acknowledgment and 21 between it and the count (`vblankAckSpace`, `vblankCountSpace` in `kernel.go`), so the acknowledgment, the count and the callback come at Sony's cycles (the callback 4 cycles early), and `soundTick` takes the 84 cycles back before its first envelope read (`tickReadsPad` 278 to 194, `space(1)` to `space(2)`), so the sound tick and the end of the interrupt stay where they were.

`TestVSyncCount` checks, for each blank, that the interrupt is taken at once, the controller driver runs before the count, one write acknowledges the blank, at 10,244 to 10,248, the count at 10,296 to 10,300 and the callback at 10,320 to 10,334. It passes on Sony's disc and on the new image, and fails on the image before. Build the discs with `exp/cleanlib/image/probe.sh vsynccount exp/cleanlib/spec/probes/vsynccount` and pass `-vsynccount` with either cue sheet.

Found in passing, not changed: Sony's `VSync(-1)` reads timer 1 (the line counter) where the clean one reads nothing, 6 cycles longer.

`image/regress.sh` after this part: 37 PASS, 4 TIME, where the baseline had 36 and 5: `flame` went from TIME to PASS (its first frame's readout, 1,940 lines on Sony's and 1,941 on the clean before, is now the same). `regress.sh`: 33 PASS, 8 TIME, the same programs. `TestVSyncPhase` passes.

### 2. The request queue's length and `DrawSync(1)`

`qlen` was run with the drawing time on, on Sony's image and the clean one as it stood (spec 41 had made the queue 64 long already; the clean library's 31 in this spec's table is from before spec 41):

| | Sony's | clean before | clean now |
|---|---|---|---|
| calls that return at once behind the drawing | 63 | 63 | 63 |
| 64th call's time, eighths | 13,430 | 9,977 | 9,928 |
| `DrawSync(1)` after the nth image queued | n | n + 1 | n |
| `DrawSync(1)` after the 64th call and each after it | 1 | 0 | 0 |

`qlen` takes about six seconds of the emulator's own time, so it was not cut down.

`DrawSync(1)` now returns the requests waiting behind the one running, and with none waiting, 1 while channel 2 or the GPU is busy, as before. Sony's counts the images behind the drawing and not the drawing; `probes/synccount` (new) follows `DrawSync(1)` through a frame with 0, 1 and 3 images behind it and shows the same on both libraries now: 1 while the frame draws with none, then the number of images, then 0 once they have gone. The compiled `DrawSync(0)` path is the same instructions as before, jump targets apart; mode 1 has two more (a load of `queue.running` and its delay slot).

`TestQueueLength` (new, `image/probetest/qlen_test.go`) runs `qlen`'s first run with the drawing time on (`-qlen` names the disc) and checks that the first 63 calls return within 200 eighths with `DrawSync(1)` giving 1 to 63, and that the 64th waits. It passes on Sony's disc and the new image and fails on the image before. `TestQueuedImages` expected the clean library's 2 after the image call; it now expects Sony's 1, and passes.

Not done here:

- **What `DrawSync(1)` gives after a `LoadImage` that found the GPU free**, 1 on Sony's and 0 on the clean library, with the drawing time on. Both calls take 85 eighths, but Sony's starts the image's blocks 92 cycles before it returns and the clean one 264 before: the clean call spends its pace after starting them, Sony's before. Since c31746e the blocks take the emulator about 220 cycles, so they are still going when Sony's call returns and `DrawSync(1)` sees channel 2 busy. With the drawing time off the blocks take no time and both give 0. Moving the clean `LoadImage`'s pace before the transfer would move every `LoadImage`'s I/O, so it was left for a spec of its own.
- **The 64th call's wait.** Sony's waits 3,500 eighths longer. Most of that is the time each queued image takes (part 4); the rest is a vertical blank that came between the 63rd image and the 64th on the clean library, whose handler, the sound tick with it, ran there.

### 3. What a program leaves in DPCR and D2_CHCR

`probes/dmaleft` (new) prints DPCR and D2_CHCR, with D4_CHCR and D6_CHCR, after each call that may set them: at entry, `ResetGraph(0)`, `GsInitGraph`, `GsClearOt`, `GsDrawOt`, `DrawSync(0)`, `LoadImage`, `StoreImage`, `VSync(0)`, `ResetGraph(1)` idle, after a frame and on a running frame, `ResetGraph(3)`, `ResetGraph(0)` again, and a read of a sector of the disc. A bus trace from "Execute !" shows the writes.

Sony's:

- Its start, before `SetCustomExitFromException`, masks I_MASK, clears I_STAT and writes DPCR 33333333h: every channel's priority 3, every channel off. The kernel had left 00009099h.
- `ResetGraph(0)` writes D2_CHCR 401h and enables channel 2 in DPCR (33333B33h), and the sound's start enables channel 4 (333B3B33h), as the clean library does. `GsClearOt` enables channel 6 (3B3B3B33h). A read of the disc enables channel 3 (3B3BBB33h).
- `ResetGraph(1)` and `(3)` mask I_MASK, write D2_CHCR 401h, enable channel 2 in DPCR, write GP1 02000000h and 01000000h, and put I_MASK back.

The clean library before: no write of DPCR at its start, so the kernel's channels 0, 1 and 3 stayed on with their priorities (08089899h at the end); channel 3 enabled at the drive's start, not at a read; and `ResetGraph(1)` and `(3)` wrote D2_CHCR 0, with the CPU's interrupts off rather than I_MASK, and did not touch DPCR. A transfer itself leaves what it leaves on both: 401h after a list, 201h after a `LoadImage`, 200h after a `StoreImage`.

What changed:

- `libBegin` (`kernel.go`) writes DPCR 33333333h first.
- `ResetGraph`'s modes 1 and 3 (`gs.go`) do what Sony's do, in the same order, with stores spacing the accesses to Sony's cycles (`resetSpaceIn` and on): from the read of I_MASK, the write of D2_CHCR at 40 cycles (Sony's 40), DPCR's read and write at 46 and 52 (48, 54), GP1's at 62 and 70 (62, 70), I_MASK's at 86 and 92 (86, 88), and the caller's next access at 164 (164). The call starts its first access at Sony's cycle after the caller's last.
- The drive's start (`cd.go`) no longer enables channel 3 in DPCR: `cdTransfer` does, at each read, as Sony's is seen to. Four stores take the time the enable took.

`dmaleft` now prints the same on both libraries, line for line. After `rerun`, run twice on one boot (`TestRerun`, `PROBE_SR=401`):

| | DPCR | D2_CHCR | DICR |
|---|---|---|---|
| Sony's, init routine on the first run | 3B3B3B33h | 401h | 00900000h |
| Sony's, init routine never | 3B333B33h | 401h | 0C000000h |
| clean, init routine on the first run | 3B3B3B33h | 401h | 00900000h |
| clean, init routine never (a console) | 3B3B3B33h | 401h | 00900000h |

The same after each run. On a console Sony's library has had its init routine run at boot, the first row, and the clean library's never runs, the last; those two now agree. (The clean library does its init routine's work at the first `ResetGraph(0)` when the routine has not run, spec 15, which Sony's does not, hence the second row.)

`TestDMALeft` (new, `image/probetest/dmaleft_test.go`) runs `dmaleft` (`-dmaleft` names the disc) and checks every line against Sony's. It passes on Sony's disc and the new image and fails on the image before.

Found in passing, not changed: after `ResetGraph(3)`'s last access, Sony's takes about 1,060 cycles before the caller's next access, the clean one about 120.

`image/regress.sh` after parts 2 and 3: 38 PASS, 3 TIME: `dungeon` went from TIME to PASS. `regress.sh`: 33 PASS, 8 TIME, the same programs as the baseline.

### 4. The two tests

**`TestDrawWait`'s reset lines.** Sony's image in today's emulator, `drawwait` with the drawing time on:

| | Sony's | clean |
|---|---|---|
| tail: second `GsDrawOt` takes | 9,617, 9,617, 11,506 | 9,633, 9,633, 11,520 |
| tail: all done at | 76,852 to 76,863 | 76,864 to 76,865 |
| reset: second `GsDrawOt` takes | 55 | 54 |
| reset: all done at | 38,520, 39,631, 38,523 | 38,524, 39,609, 38,523 |

The test now checks the reset lines for a call of 40 to 80 eighths and everything done at 38,400 to 39,800, the tail lines as before. Channel 2 busy on return and `DrawSync(1)` 1 hold on both. It passes on Sony's disc and on the clean image before and after this spec; the library was not changed for it.

**`TestFieldTime`'s 32 images.** A bus trace of a cut copy of `fieldtime` (`probes/imgpace`, new: one box and 8 or 32 images behind it, then `DrawSync(1)` polled to 0, with "trace" before the last frame) shows where the time goes. With the image's DMA taking time, Sony's handler, after starting an image's blocks, reads D2_CHCR at 124 cycles and finds the channel still sending. It then goes through its interrupt sources: it puts I_MASK back, reads DICR three times, reads I_STAT and I_MASK at 290, acknowledges the channel's completion in I_STAT at 384 and in DICR at 504, reads D2_CHCR at 528 and finds it done, and only then goes on to the next image: 966 cycles from one image's blocks to the next's. The clean library read D2_CHCR in a tight loop, went on as soon as the blocks were done, about 224 cycles in, and took 698 cycles an image. That is the 240 µs over 24 images.

Sony's handler also takes a vertical blank there: in the traced frame, one raised during the drawing is taken between the first image and the second, its sound tick included, 5,634 cycles from one image to the next. The clean library took it after the last image. With Sony's last image, whose completion does not interrupt, the handler reads I_STAT twice more and D2_CHCR at 606, and returns.

What changed (`gpu.go`):

- `startImage` waits for a queued image's blocks through `imageBlocksWait`, which makes Sony's accesses at Sony's cycles, from the image's start: D2_CHCR at 124 (Sony's 124), I_MASK read and written back at 146 and 150 (148, 150), DICR at 208, 226 and 238 (the same), I_STAT and I_MASK at 290 and 294 (290, 292), and with the completion raised, its acknowledgment in I_STAT at 384 (384), DICR read at 418 and 498 (418, 498) and written at 506 (504), and D2_CHCR at 530 (528). With the channel done at the first read it returns there. The pads between are 2-cycle stores (`blocksSpace...`) and, for the long ones, turns of a loop of stores (`spaceRun` in `fidelity.go`, about 8 cycles a turn, its count read from memory so that it is not unrolled; `blocksRuns`), which keeps the inlined code smaller than stores alone would. It is inlined into `drawNext`: as a call it made `drawNext` save registers and slowed the free path's `GsDrawOt` and `LoadImage` by 3 eighths. It lives in `fidelity.go`: a build without the fidelity tag (`nofidelity.go`) waits in a tight loop, as before this spec.
- The last image, behind which nothing waits, is still waited for in a tight loop. Sony's waits for it until 606 cycles and returns, but the clean library still has the image's empty packet list and the handler's second pass to make after it, some 800 cycles, so a Sony-paced wait there put the end of the chain 1,200 cycles behind Sony's instead of 400.
- `sendImages` takes a vertical blank raised meanwhile after each image (`imageBlank` in `fidelity.go`: `vblankInterrupt`, with `blocksSpaceBlank` after it), as Sony's handler takes it between two images; the kernel's handlers don't see it. It does this from `pollQueue` too, on the program's stack with interrupts off, since the chain runs there when a program polls `DrawSync(1)`, where Sony's runs in its handler. `sendImagesMarkSpace` and `sendImagesSpace` are 5 and 37 (19 and 48 before), so that one image to the next is Sony's 966 cycles, with the watch 112 cycles before the GPUSTAT read where Sony's is 140 before it. A build without the fidelity tag leaves the blank to the interrupt after the chain, as before: taken there, it made `offscr`'s static build print 40 lines a frame where its golden (`TestOffscrScreenshot`) has 39.
- The vertical blank's work moved out of `libps_interrupt` into `vblankInterrupt` (`kernel.go`), for `sendImages` to share. The compiler doesn't inline it, so `libps_interrupt` saves five registers fewer, and it spends five stores at its start and five at its end (`interruptSpaceIn`, `interruptSpaceOut`) to keep its time; part 1's spaces were set again for the call (`vblankAckSpace` 3, `vblankCountSpace` 19), with `TestVSyncCount` passing.

In the emulator with the drawing time on:

| | Sony's | clean before | clean now |
|---|---|---|---|
| `imgpace`, one image's blocks to the next's, cycles | 966 | 698 | 966 |
| `imgpace`, a vertical blank between two images | 5,634 | taken after the last | 5,636 |
| `imgpace`, 8 and 32 images done, eighths | 78,514, 81,405 | 78,309, 80,384 | 78,494, 81,392 |
| `fieldtime`, 1 box: 0, 8 and 32 images, µs | 18,621, 18,999, 19,684 | 18,620, 18,951, 19,446 | 18,619, 19,007, 19,692 |
| `imgq`, image and `StoreImage` totals, eighths | 38,550, 38,555 | 38,606, 38,584 | 38,604, 38,582 |
| `qlen`, the 64th call, eighths | 13,430 | 9,928 | 12,008 |

`imgq`'s image is a last image, whose wait is as before. The `qlen` call that waits is now 1,400 eighths short of Sony's, where it was 3,500: the vertical blank comes elsewhere in the chain on each library.

`TestFieldTime` now expects Sony's 18,621, 18,999 and 19,684 µs within 40. It passes on Sony's disc and on the new image, and fails on the image before (its 8- and 32-image lines). The free path is unchanged: `drawwait`, `imgq`'s frame alone, and `GsDrawOt`'s and a free `LoadImage`'s times are the same as on the image before this part.

Not done:

- The clean library still starts an empty packet list after each queued image and makes a second pass of its handler after the last, where Sony's does neither; the chain's end stays about 400 cycles behind Sony's.
- Sony's handler takes the vertical blank right after its read of I_STAT, before the image's completion; the clean one after the image's empty list. The time from one image to the next is the same either way.

### Verify chain

`.build/verify58/chain.sh` (verify57's, renamed) run on the working tree before the first change as the baseline, and `.build/verify59/chain.sh` after all four parts. Its sysroot, image, discs and `go test` stages were run again (`.build/verify59/part2`) after `imageBlank` moved into `fidelity.go`, which changes only builds without the fidelity tag: the image is the same, byte for byte.

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify58 (before) | 94 | 20 | 14 | 8 | 1 | 1 | 1 | 1 |
| verify59 (now) | 96 | 19 | 14 | 7 | 1 | 1 | 1 | 1 |

Seven archive lines changed. `demos-scee-chap5-1` (differed at change 2) and `test-bomb96-bomb` (a shift) are now the same. `test-pd961207` and `test-zero-zero` still shift, but first at change 272 and 562, where they did at 112 and 561. `event-event001-rave096`'s TTY is 20 bytes longer than Sony's where it was 54. Two are worse: `scee-topten-cardem` first differs at change 66 (frame 75), where it did at change 167 (frame 413), and `test-gr-01` makes 360 picture changes where Sony's makes 375 and the clean library made 364. Neither was looked into.

- `image/regress.sh`: 38 PASS, 3 TIME, where verify58 had 36 and 5: `flame` and `dungeon` now pass. The three left are `check`, `offscr` and `terra`, as before.
- `regress.sh`: 31 PASS, 8 TIME, 2 FAIL, where verify58 had 33 and 8. The TIME programs are the same. `car` and `car:9410` fail, below.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- `go test ./exp/cleanlib/...` passes. `TestDrawWait`, `TestFieldTime`, `TestQueuedImages`, `TestVSyncPhase`, `TestFontWake`, `TestFifthRun`, `TestVSyncCount`, `TestQueueLength` and `TestDMALeft` pass on Sony's discs and on the new image, with the scph1001 BIOS (`PSX_BIOS`); on OpenBIOS, `TestFontWake` and `TestVSyncCount` fail on both, as they are written for the retail BIOS.

**`car` in `regress.sh`.** Its setup writes one more sound tick on the clean side (2,680 SPU words against 2,674): its first swap comes in the frame after the reference's. That is not the library's doing. The program's load is now 131,720 bytes, over the 64 sectors (131,072) it fitted in before by 56 bytes, so its `MAIN.EXE` is a sector longer, the BIOS loads it later against the vertical blank, and every wait for a blank in the start-up comes a frame later from the program's entry (the start-up from the entry to the first swap took 5,472,354 cycles against 5,132,544). The same build with `imageBlocksWait` as a tight loop (64 sectors) passes, and that build with 720 bytes of unused data added (66 sectors) fails the same way. `imageBlocksWait`, inlined into `drawNext`, adds some 680 bytes, the `spaceRun` loops in it included; nothing in this spec could take that much back. `regress.sh` compares against `.build/car-ref`, a build of a different size, and does not even out the load; `image/regress.sh`, where both sides load the same program, passes `car`.

**`TestOffscrScreenshot`** (the chain's `go test`) failed on the first run of verify59: `offscr`'s static build, without the fidelity tag, printed 40 lines a frame where the golden has 39. It was `sendImages` taking the vertical blank between images (`imageBlank`); with that left to the fidelity build, the test passes again, and the chain's `go test` list, run again, has no failures.

### On the console (spec side)

Run 2026-11-10 on the console with the new image (`.build/hardware/images/libps-spec45.exe.gz`, the image `46035eb1` unzipped) under the clean monitor, against Sony's runs from earlier sessions. Results in `.build/hardware/spec45-*` and `spec45b-*`.

- `vsyncphase`: 274 lines from the blank's start to the return, past the blank in all 240 frames, and 262 at 240 lines. The same as Sony's.
- `vsynccount`: 262 and 263 lines between counts, by parity, the same as Sony's in the emulator. The old image (`dd5c83f`) gives the same.
- `qlen`: every call returns, the queue fills and drains as in the emulator.
- `rerun`: seven runs in a row, each to END.
- gasgar: no flashing, the same as before (watched by the user).

Part 1 is not settled on the console:

- `statlines`: the clean library counts the blank 11 lines late every other field, at 480 lines and at 240 (lines 525 and 263 where Sony's counts at 514 and 252, `statlines-sony3` in spec 35). The old image gives the same lines.
- `vsynclate` against a real run on Sony's library (`vsynclate-sony2`): the 240 line table matches, and in the interlaced one the clean library's `VSync` returns 12 lines sooner than Sony's in 37 of 96 cases, with the same blank counts. The earlier figure here (36 of 96, a field apart) compared the clean library with itself from before spec 42, see spec 35, 2026-11-10 evening.
- `vsynccount` does not show it: it installs a `VSyncCallback` and reads `VSync(-1)` in a loop, where `statlines` reads GPUSTAT and timer 1 after a `VSync(0)` and has no callback. `TestVSyncCount` therefore passes without covering what the console shows. A later spec: why the clean library's count comes 11 lines late every other field.
- `irqgaps` (2026-11-10, `.build/hardware/irqgaps-sony` and `irqgaps-clean`) polls the system clock, the line count and `VSync(-1)` after `VSync(0)` and reports each stretch over 400 cycles where something else had the CPU. On a console, on both libraries, one such stretch of about 23,900 cycles (11 lines) starts at each blank. The count goes up either before it (seen on a poll, "outside a gap") or inside it:

  | | 480 lines | 240 lines |
  |---|---|---|
  | Sony's, `irqgaps` | before, 3 of 3 | inside, 3 of 3 |
  | Sony's, `statlines-sony3` | before, 3 of 3 | before, 2 of 2 |
  | clean | before 1 of 3 | before 2 of 3 |

  So the late count is the count landing after that 11-line stretch. Sony's lands before it at 480 lines every time seen, and at 240 lines it varies from run to run. In the emulator the stretch is about 15,200 cycles and starts 1 line after the blank. Sony's image counts before it in all 6 fields, and the clean one after it in 3 of 6 (`probes/irqgaps` through `probe.sh`), so the emulator shows the difference, smaller.

### 1, again: the stretch at the blank, and when a program sees the count

Done 2026-11-10 in the emulator, with the console's CPU costs off (the default) and on (`SetCPUCost`, `PROBE_CPUCOST`). Not tried on a console.

**The stretch is the vertical blank interrupt itself**, the kernel's handlers (the controller driver's among them) and the library's, sound tick included. A bus trace of `irqgaps` from outside, timed from I_STAT bit 0, shows the interrupt taken at once and the program's next read of timer 2 some 15,200 cycles later, with nothing else between: about 15,240 with the costs off, 20,800 (Sony's) and 21,100 (clean) with them on, and about 23,900 on a console. Both libraries change the count at the same cycle inside it (10,298, part 1 above), so neither program can see the new count during the stretch.

**"Counted before the stretch" is where the poll was when the interrupt came.** `irqgaps` and `statlines` read the line counter and then call `VSync(-1)`. When the interrupt is taken after the program's read of the line counter and before `VSync(-1)` reads the count, the program sees the new count with the line from before the interrupt: "before the stretch", 11 lines early on a console. When it is taken after the read of the count, the next poll sees it, with the line from after. In `irqgaps` with the costs off, Sony's six interrupts were all taken 2 to 44 cycles after the program's read of timer 2, inside that window. So which poll sees the count depends on two things the library sets:

- **Where in `VSync(-1)` the count is read.** `probes/vsyncpoll` (new) polls `VSync(-1)` across the blank, a few cycles later each field, and `TestVSyncPoll` (new) times each interrupt taken in a call from outside, in cycles from the program's read of timer 2 before the call:

  | | Sony's | clean before | clean now |
  |---|---|---|---|
  | GPUSTAT read, timer 1 read, costs off | 22, 24 | none | 22, 24 |
  | interrupt in the result if taken by, not if from, costs off | 38, 42 | 40, 42 | 38, 42 |
  | the next read of timer 2, costs off | 58 | 52 | 58 |
  | GPUSTAT read, timer 1 read, costs on | 29, 34 | none | 29, 34 |
  | interrupt in the result if taken by, not if from, costs on | 49, 56 | 28, 37 | 50, 56 |
  | the next read of timer 2, costs on | 77 to 93 | 45 to 48 | 80 to 94 |

- **Where `VSync(0)` leaves the program**, which sets where the poll loop is when the next blank comes. From the end of the interrupt (the CPU's interrupts enabled again) at 240 lines, and from GPUSTAT bit 31's change at 480, to `VSync`'s read of timer 1 on its way out, over 48 fields of `vsyncpoll`'s second half:

  | | Sony's | clean before | clean now |
  |---|---|---|---|
  | 240 lines, costs off | 38 to 60 | 22 to 30 | 38 to 60 |
  | 480 lines, costs off | 24 to 34 | 22 to 26, 38 to 42 | 26 to 36 |
  | 240 lines, costs on | 80 to 139 | 44 to 54 | 77 to 147 |
  | 480 lines, costs on | 46 to 51 | 33 to 39 | 46 to 51 |

  Sony's pass of the wait for a blank is about 12 instructions (24 cycles) and some 60 cycles with the costs on, where the clean one's was 5; its wait for bit 31 about 6.

**The 12-line `VSync` return in `vsynclate` is a separate thing**, the bound on the wait for bit 31. On a 480-line display `VSync(0)` waits for the count and then for bit 31 to differ from its value at the call. With the costs on, the interrupt takes about 9.7 lines, past the field's change at 8 lines (on a console 11), so when the count is seen bit 31 can be back to its value at the call. Sony's `VSync` then waits on to the next field change, 23 lines into the blank after. The clean one ended its wait at the next blank's count (spec 42's "bound that Sony's may not have"), 12 to 14 lines sooner. With the costs on the emulator shows it: `vsynclate`'s interlaced table, `n 0 d 262`, took 540 lines on Sony's and 526 on the clean library (537 and 525 on a console), and 77 of its 96 lines differed. With the costs off the interrupt ends before 8 lines and the tables were the same.

What changed:

- `VSync(-1)` is in the target's assembly (`playstation-yaroze-clean.S`): once the library has started it reads GPUSTAT and timer 1 and then the count, at Sony's cycles, with loads from RAM (`VSYNC_RAMLOAD`) where a nop would be too quick under a console's costs. Before the start it goes to the Go, which starts the library. The count and the started flag moved to `libps_vsync_blanks` and `libps_lib_started` in the assembly's data, for it to read (`vsyncBlanks`, `libStarted` in the Go).
- `waitBlanks` (`VSync` at 240 lines) is in the assembly too, `libps_wait_blanks`: a pass of loads, the count, two loads and the test, and nops on the way out, with Sony's pass and return. `waitField`'s wait for the count paces each pass the same way (`blanksPaced`).
- `waitField`'s wait for bit 31 has no bound now, a store a pass and one on the way out. On a 480-line display bit 31 changes every field.
- `libps_interrupt` spends one more store at its end (`interruptSpaceOut` 6), so that the interrupt takes Sony's 15,238 to 15,240 cycles.
- `loadSpace` (`fidelity.go`) pads with loads, 2 cycles each with the costs off and about 6 with them on.

In the emulator now:

- `irqgaps`, costs off: the count is seen before the stretch in all six fields, as on Sony's; the clean library before saw three of six after it.
- `irqgaps`, costs on: the three fields at 480 lines match Sony's. At 240 lines they do not: under the cost model the clean interrupt takes about 300 cycles longer than Sony's (21,100 against 20,800), which moves the poll's phase. On the console both took about 23,900, so this was not chased.
- `vsynclate`, costs off and on: both tables the same as Sony's, the 96 interlaced lines included.
- `vsyncpoll`: the second half (`WAIT`) prints the same lines as Sony's. The first half's per-field lines are near Sony's, shifted by about one step of its delay, from the few cycles the clean interrupt still differs by.

Tests:

- `TestIRQGaps` (new, `image/probetest/irqgaps_test.go`) runs `irqgaps` (`-irqgaps` names the disc) and checks that each count is seen on a poll before the stretch, in all six fields. It passes on Sony's disc and the new image and fails on the image before (three of six).
- `TestVSyncPoll` (new, `image/probetest/vsyncpoll_test.go`) runs `vsyncpoll` (`-vsyncpoll`) with the costs off and on and checks the accesses and ranges in the tables above. It passes on Sony's disc and the new image; the image before has no reads in `VSync(-1)` and returns from `VSync(0)` outside the ranges.
- `TestVSyncCount`, `TestVSyncPhase`, `TestDrawWait` and `TestFieldTime` pass on the new image. `go test ./exp/cleanlib/...` passes. `image/regress.sh`: 38 PASS, 3 TIME (`check`, `offscr`, `terra`), as before.
- A static build through `cmd/yaroze` needed the embedded sysroot rebuilt for the new assembly symbols: `task compiler:overlay`, then `task yaroze`, which refreshed `cmd/yaroze/wasm/sysroot.tar.gz`.

On the console (spec side, 2026-11-11), the new image (`.build/hardware/images/libps-spec45b.exe.gz`) under the clean monitor and Sony's library under its own, in `.build/hardware/spec45c`:

- `statlines`: at 480 lines the clean library now counts at 251, 514 and 776, Sony's lines in `statlines-sony3`; it was 11 lines late every other field. At 240 lines it counted at 263, 515 and 778 where Sony's counted at 252 and 515, so two of three fields are still seen after the stretch.
- `irqgaps`: at 240 lines the same as `irqgaps-sony`, the count seen inside the stretch in all three fields. At 480 lines the count was seen before the stretch in one field of three, where Sony's was in all three.
- `vsynclate`: the same as `vsynclate-sony2` but for one line, `n 0 d 2000`, where that run of Sony's returned 1983 and every other run on either library returns 2000 or 2001. The 37 interlaced lines 12 sooner are gone.
- `vsyncpoll`: the `WAIT` lines are the same, every `VSync(0)` returning on line 0. The shortest `VSync(-1)` took 79 cycles on Sony's and 76 on the clean library, at both displays. At 240 lines 24 calls held the interrupt on both, 13 of them counting it on Sony's and 14 on the clean one. At 480 lines Sony's had 28 calls hold the interrupt and 23 count it, the clean library 22 and 11. That 480-line difference, where the poll meets the interrupt, is what `irqgaps` still shows at 480 lines; the emulator does not reproduce it.
