# VSync on an interlaced display returns once the field has changed

## Why

`showcase-gasgar` flashes on a console under the clean library and not under Sony's. Cut down to its display alone, it still flashes: `probes/teal` draws gasgar's 640 by 480 interlaced single-buffer display with nothing but a teal clear each frame. On a console under the clean library, the clear only ever reaches one field's lines. The other field keeps what was in video memory before, the monitor's old screens, for the whole run, and the CRT alternates between the two. Under Sony's library the screen is solid teal. At 640 by 240, not interlaced (`probes/tealni`), the clean library is steady too.

With an interlaced 480-line display and drawing to the displayed area off (E1 bit 10 clear, as gasgar and `teal` have it), the GPU draws only one field's lines at a time. GPUSTAT bit 31 says which: through the picture, the field not being shown, and outside the picture the even lines. A frame drawn at the same point after the vertical blank every frame, with the same bit 31 every time, reaches the same lines every time.

The monitor and the takeover were ruled out: Sony's library run under the clean monitor (`hwrun -library clean -image` Sony's image) behaves as under Sony's monitor, and `teal` is solid teal there.

## What the probes show

`probes/statlines` records, on a console, GPUSTAT bits 31 and 13 and the vertical blank count (`VSync(-1)`) against the lines timer 1 counts, over three fields of a 640 by 480 interlaced display. Counting from each vertical blank interrupt, with `GsInitGraph`'s display range:

- Bit 13 toggles 8 lines after it.
- After the odd field, bit 31 falls from 1 to 0 as the picture ends, 8 lines after it.
- After the even field, bit 31 rises from 0 to 1 as the odd picture starts, 23 lines after it.

Sony's `VSync(0)` on that display returns right as bit 31 changes from the value it had when the blank was counted: 8 lines after the blank that follows the odd field, and 23 lines after the one that follows the even field. Its frames therefore start either just after the odd picture ends or just after it starts, and their drawing alternates between the fields' lines. On a 640 by 240 display it returns as the blank is counted.

`probes/vsyncphase` calls `VSync(0)` as an odd picture ends and counts the lines to its return. On a console:

| | 480 interlaced | past the blank | 240 |
|---|---|---|---|
| Sony's, Sony's monitor | 273 to 274 | every frame | 262 |
| Sony's, clean monitor | 277 | every frame | 262 |
| clean | 262 | never | 262 |

A controller plugged in or not makes no difference (273 and 274).

`probes/tealwait` is `teal` with the program itself waiting 12 lines after each `VSync(0)`. On a console under the clean library its screen is solid teal from the start: the old screens are cleared.

The emulator now has GPUSTAT's display mode bits (14, 16 to 22, from GP1(08h)) and bits 31 and 13 placed as `statlines` measured them (commit b226b89). With it, `vsyncphase` in the emulator gives:

| | 480 interlaced | past the blank | 240 |
|---|---|---|---|
| Sony's | 278 | every frame | 262 |
| clean | 262 | never | 262 |

(`PSX_BIOS=$HOME/.mednafen/firmware/scph1001.bin exp/cleanlib/image/probe.sh vsyncphase exp/cleanlib/spec/probes/vsyncphase`, with the lines in `.build/vsyncphase-cimg{,-clean}/out.tty`.)

A second, smaller difference shows on the console only, and is not for this spec. Under the clean library the vertical blank count comes about 11 lines late every other field (count intervals 274 and 251 against Sony's 263 and 262). The clean library counts the blank in its handler after the kernel's own handlers have run. Leave it as it is: it is noted for a later spec.

## What to do

- On a display whose GPUSTAT bit 19 shows 480 lines, `VSync(0)` returns only once GPUSTAT bit 31 differs from what it read when the blank it waited for was counted, as Sony's is seen to. On any other display, nothing changes.
- Find out with probes, in the emulator against Sony's library, which other modes do the same (`VSync(n)` for n of 2 or more, and `VSync(1)`), and what each returns, and match them.
- `VSync`'s return value, the time it records for `VSync(1)`, and everything else it does stay as they are, unless a probe shows Sony's differ on such a display.
- When the display isn't 480 lines, nothing may change: the same instructions on that path, the same times.

## Testing it

`probes/vsyncphase` in the emulator must give the clean library Sony's figures: the same lines, past the blank in every frame, and 262 at 240 lines. Add a test that runs it and checks those, as `TestDrawWait` does for `drawwait`. Check `fieldphase` too, which prints, for each frame, bits 31 and 13 when `VSync` returned and when the clear was done.

The emulator change moved the archive: 480-line programs on Sony's library now wait in `VSync` there and the clean library's don't. Take a new baseline before changing anything: copy `.build/verify51/chain.sh` to `.build/verify52`, run it on the image in the working tree as it is, and keep that summary. Then run it again after. Programs that differ only because of this should come back to what they were before b226b89, or closer, and nothing may get worse. Also run the image regression (`image/regress.sh`), `regress.sh`, `terra.sh` and the usual tests.

The console runs come after, on the spec side: `vsyncphase`, `teal`, `tealbox`, `buildup` and gasgar.

## Results

Done 2026-11-09 in the emulator. Not tried on a console.

### Which modes wait, and on what

`probes/vsyncmodes` is `vsynclate`'s table on gasgar's 640 by 480 interlaced display: `VSync(n)` for n of 0 to 3, called d lines after a `VSync(0)` (d from 0 to 560), each case twice so that the `VSync(0)` before it ends after each kind of field. Each line gives the return, the blanks to the call and to the return, the lines the call took, GPUSTAT bits 31 and 13 when the `VSync(0)` returned and when the call returned, and what a `VSync(1)` right after returns. Then the same at 640 by 240, not interlaced.

Under Sony's library in the emulator:

- Modes 0, 2 and 3 all wait for the field. Mode 1 doesn't wait, and a negative mode doesn't either.
- The return value is as before: the lines from the last waiting `VSync`'s return to the call.
- The time a waiting call records for `VSync(1)` is its return, after the field wait: a `VSync(1)` straight after returns 0 in every case.
- At 240 lines nothing differs from the clean library.

What the wait compares against isn't quite what this spec guessed. With the blanks named A (after the odd field, bit 31 falls 8 lines after it) and B (after the even field, bit 31 rises 23 lines after it):

- `VSync(0)` called at A + 7, just before bit 31 falls, returns at B + 7, the end of the interrupt, with bit 31 still 0: no wait, though bit 31 was 0 when B was counted. Called at A + 8, just after the fall, it returns at B + 23 (`vsyncphase`'s 278).
- `VSync(3)` called at A + 8 returns at B' + 7 (788 lines), with bit 31 still 0.

So the bit 31 that the wait compares against isn't the one at the blank it waited for. It is bit 31 at the call, or, when blanks are counted before the last one, bit 31 at the one before the last. `VSync` returns once the last blank is counted and bit 31 differs from that value: at once if it already does, else when the field changes, 8 or 23 lines in. That fits all 72 lines of the 480-line table: n from 0 to 3, d from 0 to 560.

### What changed

- `VSync`'s assembly (`playstation-yaroze-clean.S`) reads GPUSTAT for every mode. For modes other than 1 and the negative ones it goes to `libps_vsync_field` when bit 19 shows 480 lines, else to `libps_vsync_wait` as before.
- `vsyncField` (`sync.go`, exported as `libps_vsync_field`, not inlined) is `vsyncWait` with the field wait. It works out the target blank in the same way. `waitField` waits for the count, taking bit 31 at the call and again at each blank counted before the target, and then waits until bit 31 differs. It also stops at the next blank in case bit 31 never changes. It then records `vsync.last` and the line counter, and returns the same lines as `vsyncWait`.
- `vsyncWait` is unchanged, the same instructions as before, register numbers and one data offset apart.
- `probes/vsyncmodes` is new.
- `TestVSyncPhase` (`image/probetest/vsyncphase_test.go`) runs `vsyncphase` (`-vsyncphase` names the clean disc). It checks the 480-line line for 278 lines least and most, past the blank in every frame, and the 240-line line for 262. It passes on the new image and on Sony's disc, and fails on the image before.

### vsyncphase and fieldphase

In the emulator:

| | Sony's | clean before | clean now |
|---|---|---|---|
| `vsyncphase` 480 interlaced, lines least, most, mean | 278, 278, 278 | 262, 262, 262 | 278, 278, 278 |
| past the blank | 240 of 240 | 0 of 240 | 240 of 240 |
| 240, lines | 262, 262, 262 | 262, 262, 262 | 262, 262, 262 |

`fieldphase`, frames by GPUSTAT bits 31 and 13 when `VSync` returned and when the clear was done:

| | Sony's | clean before | clean now |
|---|---|---|---|
| returned 0 1, done | 0 1: 300 | 0 0: 300 | 0 1: 300 |
| returned 1 0, done | 1 0: 300 | 0 1: 300 | 1 0: 300 |
| clear done mean, µs | 503 | 500 | 500 |
| blank's end not seen within 3 ms | 300 of 600 | 0 | 300 of 600 |

Each frame's clear now ends in the same field as on Sony's: alternately inside the even and the odd picture, where before it ended outside the picture every frame, so the GPU drew the even rows every frame.

`vsyncmodes`, `vsynclate` and `vsyncone` now print the same as Sony's, line for line.

### The path for displays that aren't 480 lines

The test costs the waiting modes five instructions (`lw`, `li`, `beq`, `sll`, `bltz` with its delay slot, in place of the old `li`, `beq` and their slot). That is just as long as one pass of `vsyncWait`'s wait loop (five instructions, 10 cycles in the emulator). So the interrupt lands at the same point in the loop, and the wait ends on the same cycle as before. Mode 1 now reads GPUSTAT too, so its `NOPS` pad is cut from 13 to 9, with a `lui` again, and it takes the same instruction count to its read of the line counter. A negative mode takes the same count as before, five instructions to the Go.

Checked as in specs 40 and 41: the 16 probes with the switch off (`drawq`, `clearimage`, `storeimage`, `loadleft`, `imgq`, `kanjistream`, `kanjipast`, `kanji`, `fntchars`, `drawwait`, `gscycle`, `dispswap`, `flush`, `fntlayout`, `linestart`, `resetret`), every I/O read and write with its cycle from "Execute !" to the end (`PROBE_BUS`), against the image before (`.build/verify52/libps-head.exe`). Each waiting `VSync` now has one GPUSTAT read more, and its read of the line counter at the call comes 10 cycles later. With the GPUSTAT reads left out, every other access is at the same cycle as before, in every probe. Every TTY is the same, `fntlayout`'s with the image's packet addresses masked.

Where it isn't the same:

- **The line count at the call is read 10 cycles later.** A waiting `VSync` returns the lines since the last return, read at the call. Taking that read 10 cycles later moves it to the next line about once in 215 calls (10 of 2,152 cycles a line), and `VSync` then returns one more.
- **Mode 1 reads GPUSTAT.** That is an I/O read where there were `nop`s. The emulator charges only the fetch, so its time there is unchanged. On a console the read may take a few cycles more than the `nop`s it replaced. Not measured.
- **Builds without `LIBPS_FIDELITY`.** These have no `NOPS` pad to cut, so there mode 1 has 4 instructions more than before. The image and `build.sh`'s target have the pad.

A form that tested bit 19 in `vsyncWait` itself, after the read of the line counter, cost the waiting path 6 or 7 instructions. That shifted the loop's phase, so the return after the wait moved by −8 to +8 cycles from call to call (`imgq`'s readouts moved by an eighth). The test went into the assembly for that reason. The image ends at 80067488h.

### Where the return lands, to the cycle

A bus trace of `vsyncphase` on both libraries (`PROBE_BUS`) shows what is left. Each side returns the same number of cycles after the GPUSTAT read that sees bit 31 change: the line counter is read 24 cycles after it, and the probe's next read 44 after. What differs is how often each side polls GPUSTAT: every 12 cycles on Sony's, every 18 on the clean side, whose loop also checks the blank count. So the clean return lands from 6 cycles before Sony's to 14 after, varying from frame to frame. A program that times its frame in lines (`VSync(1)`) can then read one line apart now and then, as `demos-scee-morph` does below.

### Verify chain

`.build/verify53/chain.sh`, with the new baseline `.build/verify52` (this working tree before the change, on the emulator since b226b89) and with `verify51` (before b226b89):

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify51 | 93 | 20 | 14 | 9 | 1 | 1 | 1 | 1 |
| verify52, baseline | 94 | 20 | 14 | 8 | 2 | 1 | 0 | 1 |
| now | 94 | 20 | 14 | 8 | 2 | 1 | 0 | 1 |

Against verify52 one archive line changed: `demos-scee-morph`, still "differ", now from change 366 (frame 372) where it was change 19 (frame 25). That difference is its "HSYNC" readout, 65 on Sony's and 66 on clean: a `VSync(1)` count one line apart, the race above. Nothing got worse.

Against verify51, the 15 lines b226b89 moved are the same now as on the baseline, morph apart. All but morph and `event-event003-theme-krwire` moved on both sides alike (the change counts changed on Sony's as on clean). The emulator change moved them, not `VSync`: `demos-scee-chap3-1`, `chap3-2`, `hijump`, `scee-topten-adven`, `rave096`, `kyuri`, `showcase-sf`, `hhpd0002`, `bmirror`, `test-dydtst02` and `test-pd961207` moved that way, and `test-dungeon-dungeon` and `test-negimu02` went from differ and shift to same. `event-event003-theme-krwire` went from stall to wild on clean (e406e414) with b226b89 and stays so.

- `image/regress.sh`: 36 PASS, 5 TIME (baseline 35 and 6). `morph` went from TIME to PASS. `flame` stays TIME, as on the baseline (PASS on verify51): its first frame's readout is 1940 lines on Sony's and 1941 on clean. That frame comes before any waiting `VSync` (it times the start-up), and the head image gives the same 1940 and 1941, so it isn't this change. Every later frame reads 168 on both.
- `regress.sh`: 33 PASS, 8 TIME, the same programs as verify52 and verify51.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- The chain's `go test` list, run without `PSX_BIOS`: no failures, `TestYarozeLoadAndRun/ours` included. `./exp/cleanlib/...` passes.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.
- `TestFieldTime` passes on the new image, as on the head image. `fieldtime` is 480 lines interlaced, but its numbers don't move: they are the same on both clean images, and on Sony's within 11 µs, as in spec 41. `TestDrawWait` and `TestQueuedImages` weren't run: their probes are 240 lines, and their bus traces show only the shifted read of the line counter.
- The image ends at 80067488h.

### What did not come out as the spec says

- **The bit 31 that the wait compares against** is the value at the call, or at the blank before the last when more than one is counted in the call. It is not the value when the blank waited for is counted. `vsyncphase` can't tell these apart; `vsyncmodes` can.
- **The non-480 path isn't identical.** A waiting `VSync` reads GPUSTAT and takes its line count 10 cycles later at the call, but returns on the same cycle. Mode 1 reads GPUSTAT in place of `nop`s. See above.
- **Not every return is on Sony's cycle** on a 480-line display: they differ by −6 to +14 cycles, so a line count is off by one now and then (morph's HSYNC at frame 372).
- **On a console** the clean library counts every other blank about 11 lines late (this spec's second difference, left alone). After the odd field that is past the bit 31 change at 8 lines. A `VSync(0)` whose value at the call is still that of the odd picture then finds bit 31 already changed when it sees the count, and returns there, 3 lines later than Sony's. It doesn't wait a further field. Not measured.
- **The wait for bit 31 has a bound** that Sony's may not have: it ends at the next blank should bit 31 not change. On an interlaced 480-line display it always changes within 23 lines, so the bound is never reached in these probes.
- With `PROBE_BUS` set, the run goes on past "END", and Sony's prints "End of Main" when the probe's `main` returns where the clean library prints nothing. Seen in `vsyncphase`. It isn't from this change, and it is outside this spec.

### Not checked

- The console runs: `vsyncphase`, `vsyncmodes`, `teal`, `tealbox`, `buildup` and gasgar.
- `VSync(n)` for n above 3 on a 480-line display. The rule carries on from 2 and 3, but the probe stops at 3.

### On the console (spec side)

Run with `hwrun -library clean`, results in `.build/hardware/spec42`:

- `vsyncphase`: 274 lines at 480 interlaced, past the blank in all 240 frames, and 262 at 240 lines. Sony's gave 273 to 274.
- `vsyncmodes`: ran to the end. It hasn't been run on Sony's library on the console to compare.
- `teal`: solid teal, twice.
- `tealbox`: solid, the box moves, and all three parts reach END.
- gasgar: no flashing. The far side of the torus no longer shows through. Music, motion, particles and the spinning ball look as under Sony's library and in the emulator.
- `buildup`: hung once in stage 2 and once in stage 4, with the monitor not answering. This is the hang seen before this spec. Its ring, now in the scratchpad, shows the second hang inside `VSync(0)` at frame 2533, while frame 2532's drawing list was still being sent (D2_CHCR 01000401, GPUSTAT 404E2200) with its 8 images queued behind it.

A reset of the console clears RAM from at least 800A0000h up. The scratchpad survives it.
