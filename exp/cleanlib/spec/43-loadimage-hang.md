# A console hang inside LoadImage

## Why

On a console, `probes/buildup` hangs under the clean library and not under Sony's. It has hung in its stage 1, 2 and 4, after 20 to 45 seconds; `probes/tealbox` has hung at its end the same way. When it hangs the picture stops, and the clean monitor no longer answers the serial port, so the console takes the reset button. Under Sony's library `buildup` runs through all its stages. The emulator has never hung on it.

## What the probes show

`buildup` draws 640 by 480 interlaced from one buffer. Each frame: `VSync(0)`, `ResetGraph(1)`, eight sprites and (from stage 3) a torus sorted, `GsDrawOt`, then in stage 2 and after, every other frame, eight `LoadImage` calls of 16 by 16 (the sprites' texture, at 704,0 to 735,63), then `FntPrint` and `FntFlush(-1)`. It marks each call in a ring in the scratchpad before the call, and after `VSync`, `GsDrawOt`, `LoadImage` and `FntFlush` return. A `VSyncCallback` counts the blanks taken. Each mark keeps GPUSTAT, D2_CHCR, I_STAT, I_MASK, DICR, the line counter, the callback count and D2_MADR. The ring survives the reset; read back after a hang in stage 2 (frame 1250):

| mark | call | GPUSTAT | D2_CHCR | DICR | callbacks | D2_MADR |
|---|---|---|---|---|---|---|
| 18100 | before `VSync` | D64E030B | 00000401 | 00900000 | 1251 | FFFFFF |
| 18101 | `VSync` returned | 524E201F | 01000401 | 00900000 | 1252 | 048198 |
| 18102 | before `ResetGraph` | 404E201F | 01000401 | 00900000 | 1252 | 048168 |
| 18103 | before the sorting | 564E201F | 00000000 | 00900000 | 1252 | 048158 |
| 18112 | before `GsDrawOt` | 564E201F | 00000000 | 00900000 | 1252 | 048158 |
| 18113 | `GsDrawOt` returned | 524E201F | 01000401 | 00900000 | 1252 | 0964FC |
| 18114 | before `LoadImage` 1 | 524E201F | 01000401 | 00900000 | 1252 | 096230 |
| 18115 | `LoadImage` 1 returned | 524E201F | 01000401 | 00940000 | 1252 | 095EB0 |
| 18117 | `LoadImage` 2 returned | 524E201F | 01000401 | 00940000 | 1252 | 0959D4 |
| 18119 | `LoadImage` 3 returned | 404E230B | 01000401 | 00940000 | 1252 | 096674 |
| 18121 | `LoadImage` 4 returned | 404E230B | 01000401 | 00940000 | 1252 | 09662C |
| 18122 | before `LoadImage` 5 | 404E230B | 01000401 | 00940000 | 1252 | 0965FC |

- Mark 18122 is the last: `LoadImage` 5 never returned.
- I_STAT reads 0 and I_MASK 0Dh (vertical blank, CD-ROM, DMA) at every mark.
- The callback count is 1252 at the last mark and 1252 in the ring's header, which the callback itself keeps: no vertical blank was taken after that `LoadImage` began. Either interrupts stayed off from then on, or the CPU stayed inside an interrupt handler.
- D2_MADR was still moving at the last marks. It went down through the drawing list (0964FC to 09576C by `LoadImage` 3), then to 096674 and down again: the channel was going through a list then.

An earlier hang, in stage 4 (frame 2533), had its last mark before a `VSync(0)`, with D2_CHCR 01000401 and GPUSTAT 404E2200, after eight `LoadImage` calls that frame. The mark before the `ResetGraph(1)` that follows never came, so that hang was inside `VSync(0)`, or in an interrupt handler taken during it.

`probes/readybusy` measured on a console, for `LoadImage` with the GPU idle:

- While channel 2 sends an image, GPUSTAT bit 26 (ready for a command) and bit 27 are 0, and bit 28 (ready for a DMA block) is 0. Bit 28 comes back about one sample after the channel ends, and bit 26 one or two samples after that: 16 cycles after the channel, by root counter 2.
- The channel takes about 1.7 cycles a word: 16 by 16, done 24 cycles after the call returns (1,440 cycles from the call); 64 by 64, 4,440; 256 by 64, 14,700; 256 by 128, 28,800. The call itself takes about 2,130 cycles once there is a block to send.

`probes/imgrace` does what stage 2 does with one large box in place of the sprites and no text, and sweeps the time between `GsDrawOt` and the eight `LoadImage` calls frame by frame across the whole drawing. It runs its 3000 frames on a console without a hang. So the hang needs more than images queued behind a drawing: `buildup` has, besides, sprites drawn with the texture the images replace, `FntFlush` after the images, and `ResetGraph(1)` at the top of each frame stopping whatever is still going out (mark 18102 to 18103: the channel busy before, stopped after).

The emulator now has an image's DMA take time under `SetDrawTime` (commit c31746e): the channel stays busy and bit 26 clear for 1.7 cycles a word. With it (`PROBE_DRAWTIME=1`), `buildup` and `imgrace` still run to END on both libraries in the emulator.

## What to do

- Find how the clean library can stay in `LoadImage`, or in what it calls, or in an interrupt handler taken during it, with no vertical blank taken, on a console. Every wait in the image and queue paths that runs with interrupts off or inside a handler is a suspect: what it waits for, what makes that come, and whether anything the program can do (`ResetGraph(1)` while a transfer or a queued image is going out, `FntFlush` behind queued images, a drawing list ending while an image is queued) can keep it from coming.
- Make the hang impossible, keeping what Sony's library does where a probe can measure it. Where the clean library waits for something with interrupts off that Sony's may not, say so in the Results.
- Try to make it happen in the emulator, with `PROBE_DRAWTIME=1` and probes of your own (cut down from `buildup`, or new). If the emulator lacks something a console does that the hang needs, say what, and the spec side will look at adding it.

## Testing it

The spec side runs `buildup` on the console after, until it has run through all its stages several times, and `tealbox`, `teal`, `vsyncphase` and gasgar again. In the emulator: a probe of your own that hangs before and not after, if you find one, made a test as `TestVSyncPhase` is; the verify chain against a new baseline (`.build/verify53/chain.sh` copied to `.build/verify54`, run before the change); `image/regress.sh`, `regress.sh`, `terra.sh` and the usual tests; `TestFieldTime` and `TestQueuedImages` with draw time on.

## Results

Done 2026-11-09 in the emulator. Not tried on a console. The hang was not reproduced: the emulator can't get into the state that, by the best reading of the ring, the console does. What was found is a difference from Sony's library that puts the console in reach of that state on every frame of `buildup`, and it is fixed.

### What the clean library did that Sony's doesn't

`probes/fontwake` (new) is `buildup`'s frame cut down to a clear, k semitransparent 640 by 480 boxes and a line of text: `VSync(0)`, `ResetGraph(1)`, `GsDrawOt`, `FntPrint`, `FntFlush(-1)`, with the drawing time on. It reads D2_CHCR and D2_MADR when `VSync` returns, DICR after `GsDrawOt` and after `FntFlush`, and in one part spins after `FntFlush`, reading only the hardware, to see when channel 2 goes idle and when the CPU is away in an interrupt.

- **Sony's** turns on channel 2's completion interrupt (DICR bit 18, 00940000h) when `FntFlush`'s list goes into the queue behind a drawing still going out. When the drawing's list ends, its handler waits there for the GPU, with interrupts off (145 lines behind one box, until the GPU has drawn it), then starts the text's list, and takes the vertical blank raised meanwhile in the same pass. The text is out before the next `VSync(0)` returns. The interrupt is off again by the next frame's `GsDrawOt`, and left on when `ResetGraph(1)` drops a list still queued.
- **The clean library** turned that interrupt on only for images (spec 39). A packet list queued behind a drawing waited for whatever interrupt next found the GPU free: the vertical blank at best, after the frame. Behind one box, with the program going straight on to `VSync(0)` as `buildup` does, the text's list was still waiting when `VSync` returned, and `ResetGraph(1)` dropped it, in every frame. With nothing to draw but the clear, the list went out in the vertical blank, and was running when `VSync(0)` returned.

The console ring shows exactly that: at mark 18100, before `VSync`, channel 2 and the GPU are idle with frame 1249's text still queued (MADR FFFFFFh). It starts in the vertical blank (18101: channel 2 busy, MADR 048198h, inside the clean library's font packets, `cleanlib.font` at 80047B28h to 8004C0D8h in that image), and `ResetGraph(1)` stops it partway through the list (18102 to 18103: MADR 048168h, then 048158h, not the list's end). On Sony's, `buildup`'s text has gone out long before.

### The likely cause of the hang

So on the clean library, `ResetGraph(1)` stops a list partway on nearly every frame of `buildup`, and on Sony's never in these frames. Stopped between a command's words, a list can leave the GPU partway through a command. If a console's GPU keeps that through GP1(02h) and GP1(01h), or channel 2 delivers a word or two after it was stopped, the next frame's words go to it out of step. Then a list can end with the GPU still waiting for words of a command that never come, and GPUSTAT bit 26 never comes back. Every wait for the GPU then waits forever:

- in the handler, `imageTurn` waits for bit 26 before the images queued behind the drawing (the stage 2 hang, inside `LoadImage` 5 as channel 2 reached the drawing list's last packet at 0965FCh, `packets[0]`, the first sprite's; and the stage 4 hang inside `VSync(0)`, with 8 images queued behind a list still going out);
- with no images (stage 1), `drawWait` in `GsDrawOt` or `FntFlush`, with I_MASK 0.

That is not proven: it rests on what a console does with a list stopped partway, which nothing here has measured. Sony's library would hang the same way in the same state, since none of these waits gives up on Sony's either:

- `probes/listwait` (new) holds the GPU busy by writing large rectangles to GP0 while the handler is due to send a list queued behind a drawing. Sony's handler waits it out with interrupts off: 5,001 lines for 20 rectangles, and 29,376 (wrapped four times, so about 291,000, 18 seconds) for 1,200.
- `probes/gpustuck` (new) does the same before `DrawSync(0)`, `GsDrawOt` and `LoadImage`. Sony's `GsDrawOt` and `LoadImage` wait with interrupts masked (one blank counted in 48,570 lines), and with 1,200 rectangles both waited the whole 18 seconds. Only `DrawSync(0)` gives up: after about 240 blanks (63,400 lines, 4 seconds) it printed `GPU timeout:que=0,stat=184e2000,chcr=00000401,madr=80063e70,func=(80029e50)(80063e70,00000000)` and the GPU was ready after it.

So what keeps Sony's out of it is that its lists aren't stopped partway, not a limit on its waits.

### What the emulator lacks for it

- Channel 2 sending a packet list a word at a time, paced by the GPU's FIFO, so that clearing D2_CHCR's busy bit can stop it between a command's words. The emulator sends a whole list at once, or holds it whole while the GPU draws (`transferGPUTimed`), and stopping it drops the rest at a packet boundary.
- A command partway received: GPUSTAT bit 26 clear until its last word comes. The emulator's bit 26 follows only the drawing time.
- What a console's GPU does with a command partway received at GP1(01h), and whether channel 2 sends any word after its busy bit is cleared. The emulator ends the command. A console probe that stops a list of long commands partway, writes GP1(02h) and GP1(01h) as `ResetGraph(1)` does, and then reads GPUSTAT, and reads it again after one more GP0 word, would settle it.

### What changed

In `gpu.go`:

- `drawWait`, which runs in `enqueue` only when the GPU isn't free, turns on channel 2's completion interrupt (DICR bit 18) when the request is left queued behind a list still going out, as Sony's does. If the channel finished between its check and that, the request is taken as before when it is the only one waiting (wait for the GPU in the call, spec 40).
- `imageTurn` goes on when any request is next after the one that finished, not only an image: it waits for the GPU and calls `sendImages`, which sends the images as before and, for a packet list next (before the images, after them, or alone), calls `listTurn`. That turns the interrupt off when nothing waits behind the list, as `startImage` does before the last image, waits for the GPU and starts the list. `lateInterrupts` then takes the vertical blank raised meanwhile, as after images.
- `drawDone`, `drawNext`, `startHead`, `enqueue`'s free path and the handler's usual path are the same code as before. A program whose requests never wait behind a running one never turns the interrupt on and never takes the new path.

The image ends at 80067538h.

### fontwake

The summary lines (`FWAKE`), with the drawing time on. Per part and k, of 11 frames: channel 2 idle when `VSync` returned, with D2_MADR in the library (the text's list the last sent), DICR bit 18 on after `GsDrawOt` and after `FntFlush`, frames with the CPU away over 20,000 ticks of timer 2 (a handler waiting for the GPU), and the line, from `VSync`'s return, at which channel 2 went idle with the text sent.

| part, k | Sony's | clean before | clean now |
|---|---|---|---|
| spin, 0 | idle 11 font 11, irq 0 0, away 0, 143 to 262 | the same | the same |
| spin, 1 | idle 11 font 11, irq 0 11, away 11, 288 | idle 11 font 11, irq 0 0, away 0, never idle with the text sent by line 300 | as Sony's |
| spin, 2 | idle 11 font 11, irq 0 11, away 11 | idle 11 font 11, irq 0 0, away 0 | as Sony's |
| straight on, 0 | idle 11 font 11, irq 0 0 | the same | the same |
| straight on, 1 | idle 10 font 10, irq 1 11 | idle 11 font 0, irq 0 0 | as Sony's |
| straight on, 2 | idle 0 font 0, irq 11 11 | idle 0 font 0, irq 0 0 | as Sony's |

The whole TTY is now the same as Sony's, 43 lines. In the detail (an earlier form of the probe printed each frame's events), Sony's handler kept the CPU 39,125 ticks behind one box and the clean one 39,127, and behind two 38,479 against 38,486. `listwait` now prints Sony's lines too: 5,001 and 29,376 lines away, DICR 00940000h after `FntFlush`. Before, the clean library took no interrupt there.

`TestFontWake` (`image/probetest/fontwake_test.go`, `-fontwake` names the clean disc) runs it with the drawing time on and checks the six lines against Sony's. It passes on the new image and fails on the one before.

No probe hangs before and not after: the hang needs the console state above.

### The free path, and the other draw-time probes

With the drawing time off, nothing waits behind a running request, so the new code is never reached. Checked as in specs 40 to 42: the 16 probes (`drawq`, `clearimage`, `storeimage`, `loadleft`, `imgq`, `kanjistream`, `kanjipast`, `kanji`, `fntchars`, `drawwait`, `gscycle`, `dispswap`, `flush`, `fntlayout`, `linestart`, `resetret`), every I/O read and write with its cycle from "Execute !" to the end (`PROBE_BUS`), on the image before (`.build/verify54/libps-base.exe`) and the new one: the same in every probe, with the addresses written to D2_MADR masked. Every TTY is the same, `fntlayout`'s with the image's packet addresses masked.

With the drawing time on:

- `drawwait` and `vsyncphase` print the same as before. `TestVSyncPhase` and `TestQueuedImages` pass.
- `imgq`: its readouts move by one eighth in places (`STORE` total 38,584 against 38,583, some `WATCH` gaps): the handler's test for what is next is a few instructions longer on the queued-image path.
- `fieldtime` prints exactly what the image before does, every line. `TestFieldTime` fails on both, on its 1 box, 32 images line: 19,444 µs against the test's 19,276 (±40). That comes from c31746e, the image's DMA taking time, after spec 41 set those numbers: Sony's image in today's emulator gives 18,999 and 19,684 for 8 and 32 images behind one box, where the clean one gives 18,949 and 19,444. Each 16 by 16 image now takes 698 cycles from one to the next on the clean side, the 218 of its DMA included. Not looked into further; the test's numbers want measuring again on Sony's.
- `TestDrawWait` fails its reset lines on both images, as spec 41 found.
- `buildup` runs to END on the new image, stage by stage, as on the one before.

The handler now waits for the GPU with interrupts off, before a list queued behind a drawing, where it didn't before. Sony's does the same (`listwait`), as long as the GPU takes.

In passing (`gpustuck`), not changed: the clean `LoadImage` waits for a busy GPU with interrupts on (184 blanks counted in 48,563 lines) where Sony's masks them (one), and Sony's `DrawSync(0)` gives up after about 4 seconds with its message and a GPU reset, where the clean one waits on.

### Verify chain

`.build/verify55/chain.sh` (verify53's, renamed) against the baseline `.build/verify54`, the same chain run on the working tree before the change:

| | same | differ | still | shift | wild | tty | stall | noboot |
|---|---|---|---|---|---|---|---|---|
| verify54 (before) | 94 | 20 | 14 | 8 | 2 | 1 | 0 | 1 |
| verify55 (now) | 94 | 20 | 14 | 8 | 1 | 1 | 1 | 1 |

One archive line changed: `event-event003-theme-krwire`, from `wild` on clean (E406E414h) to `stall`, which is what it was from verify22 to verify51. Spec 38 found that program jumping into its library with sp 0 on both sides, at an address set by where the last interrupt struck in each library's code, so where it ends depends on the library's layout and time; the new image is 176 bytes longer. It went from `stall` to `wild` with b226b89 the same way. Nothing got worse.

- `image/regress.sh`: 36 PASS, 5 TIME, the same programs as verify54.
- `regress.sh`: 33 PASS, 8 TIME, the same programs.
- `terra.sh`: 150 of 150 pictures, 1970 and 2115 changes on each side.
- The chain's `go test` list: no failures. `./exp/cleanlib/...` passes.
- `linestart`, `initkeep`, `timnull`, `loadleft`, `revctrl` and `resetret` give the same TTY on both images.

### What did not come out as the spec says

- **The hang was not reproduced**, and no probe hangs before and not after. `fontwake` shows the difference that, by the reading above, leads to it, and `TestFontWake` checks it.
- **The cause is a candidate, not proven.** It needs a console to say what a list stopped partway does to the GPU. If the hang comes back after this change, the next suspect is any other way a list goes out late enough for `ResetGraph(1)` to stop it, and the waits named above.
- **No wait was given a limit.** Sony's waits for the GPU have none, in the handler or with interrupts masked (`listwait`, `gpustuck`), so a limit would differ from it where a probe can see. With a GPU that never comes ready, the clean library still hangs where Sony's would.
- **`tealbox` isn't explained.** It queues nothing behind its drawing, and its hang was at its end, after END (spec 42's console run: all three parts reach END). Nothing here touches what happens when a program returns.

### Not checked

- The console runs: `buildup` through all its stages several times, `tealbox`, `teal`, `vsyncphase`, gasgar, and `fontwake` and `listwait` on both libraries to compare with the emulator.
- Requests other than `FntFlush`'s queued behind a drawing (`GsDrawOt` behind `GsDrawOt`, `PutDrawEnv`, `MoveImage`, `ClearImage`): they take the same path, but only the text's list was compared with Sony's.
- `DrawSync(1)` and `FntFlush`'s wait for its earlier packets call `pollQueue`, which can now wait for the GPU with interrupts off before starting a list next, as it already did for an image. Whether Sony's `DrawSync(1)` does wasn't measured.

### On the console (spec side)

- **`buildup`, 20 runs on the clean library after the change:** 18 reached END through all six stages. The two that didn't were the fifth run after a library upload (runs 5 and 10, with an upload after run 5's hang), both in the first `VSync(0)` of frame 0. Before the change one run in one or two hung, in stages 1, 2 and 4, partway through the frames; none of those came back.
- **The frame 0 hang is not the clean library's.** On Sony's library, `buildup` hung the same way on its fifth run in a row (frame 0, blank 14,486, runs 1 to 4 to END). A copy with a tenth of the frames hung on its fifth run too, at blank 1,535, so it follows the count of runs since the library was loaded, not the time. In it, one vertical blank is taken after setup and then none. The callback's record of that last blank (the scratchpad's last 16 bytes, added to `buildup` for this) shows nothing pending: GPUSTAT 564E0000, D2_CHCR 00000201, DICR 00940000, I_STAT 0, I_MASK 0Dh. Something each run leaves behind runs out on the fifth, on both libraries and both monitors. Not yet found.
- `tealbox` and `vsyncphase` reached END with the same output as in spec 42 (`vsyncphase`'s most moves from 274 to 275 lines). `teal` counted frames normally to frame 6,600, where the 120-second watch stopped it, short of its 7,200. gasgar ran its two minutes.
- Not run yet: `fontwake` and `listwait` on both libraries.
