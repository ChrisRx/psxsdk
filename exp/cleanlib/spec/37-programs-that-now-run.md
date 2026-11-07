# Programs that now run

Until now, 42 of the archive's programs never loaded from their test discs, on either library, because of a bug in the disc build. They counted as `same` with one picture. That is fixed, and `.build/carchive/summary-verify47.txt` is the new baseline: 86 `same`, 27 `differ`, 15 `still`, 9 `shift`, 1 `wild`, 1 `tty`, 1 `stall`. `still` means one picture on each side, which says nothing about agreement.

Ten programs now show a difference for the first time. Nothing has looked at them.

| Program | Result | At |
|---|---|---|
| `test-mimet-mimet` | `shift` | 647 picture changes on Sony's, 1 on clean |
| `demos-scea-ricochet` | `differ` | change 209, frame 230, 760 and 762 changes |
| `event-dengeki001-pokecon` | `differ` | change 6, frame 303 |
| `event-event001-tantank` | `differ` | change 3, frame 360 and 364, 23 and 24 changes |
| `event-event002-wstgnbds` | `differ` | change 1, frame 12 |
| `test-senpon` | `differ` | change 1, frame 123 |
| `event-event002-yakata-p` | `shift` | change 2, frame 75 and 73 |
| `pssoft-yakata-yakata` | `shift` | change 2, frame 59 and 57 |
| `test-negimu02` | `shift` | change 1, frame 15 and 14 |
| `test-rog00` | `shift` | change 3, frame 16 and 14 |

## Work

1. Start with `test-mimet-mimet`, since clean appears to stop where Sony's goes on. Find the first thing that differs and why.
2. Go through the rest in the table's order. For each, find the first thing that differs in behavior: a return value, what the hardware is told, or what's drawn. Use `TestDump`, `PROBE_WATCH`, `TestArchiveBus` and `TestArchiveCalls`. Several may share a cause, so look for that before fixing one at a time.
3. For each cause that's a difference in behavior, write a probe that reproduces it on both libraries with no program involved, calling the library by name, and keep it in `probes/`. Correct the spec and the library for every case the probe covers. Don't fit anything to one program.
4. If what you find is only time, say so and stop on that program: don't tune cycles. Spec 34 and spec 35 explain why that waits for hardware data.

Look through the 15 `still` programs briefly and say which show the same single picture because they wait for input and which look stuck, with what's on the screen and where the PC is. Don't fix them under this spec. Name anything else you find and don't take it on.

## Acceptance

Results first, then time by cycles.

- `exp/cleanlib/image/probe.sh` on each new probe: the TTY of the two sides is identical. The existing probes for what you change keep matching.
- `FIRST=1 exp/cleanlib/image/archive.sh` on the programs named above, then the full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify47.txt`: name anything that got worse.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 34 PASS and 7 TIME. `exp/cleanlib/image/terra.sh`: all pass. `go test ./tests` passes.
- The image ends below 80068000h, now 80066288h.

## What the work on this spec found

Five causes, all differences in behavior, cleared five of the ten programs: `test-mimet-mimet`, `demos-scea-ricochet`, `event-dengeki001-pokecon`, `test-senpon` and `test-rog00` are `same` now. The other five are time, and none was tuned. The library changed: `exp/cleanlib/gs.go`, `sync.go`, `kernel.go`, `image.go`, `gpu.go`, `extra.go` and `reverb.go`.

### The ten programs

| Program | First thing that differs | Kind | Now |
|---|---|---|---|
| `test-mimet-mimet` | `GsGetTimInfo(tim, 0)` at frame 6: Sony's writes the fields at address 0, clean panics on the nil | behavior | `same`, 647/647 |
| `demos-scea-ricochet` | `VSync(1)`, the seed of `srand`, at frame 6, 1515 on Sony's and 1671 on clean at the same cycle: timer 1's start | behavior | `same`, 760/760 |
| `event-dengeki001-pokecon` | `srand`'s argument at frame 6, 0Dh on Sony's and F0h on clean, which is what `LoadImage` left in a0 on each | behavior | `same`, 28/28 |
| `event-event001-tantank` | A `VSync(0)` at frame 12 lands after the blank on Sony's and before it on clean, with Sony's about 6,170 cycles later by then | time | `shift`, 23/23, change 1, frame 15 and 14 |
| `event-event002-wstgnbds` | The readout "VSYNC = 32" against 33, `VSync(0)`'s return, 20h and 21h from frame 9 | time | `differ`, change 1, frame 12 |
| `test-senpon` | The first `VSync(0)` it prints, 7A98h on Sony's and 7AE8h on clean: timer 1's start | behavior | `same`, 3/3 |
| `event-event002-yakata-p` | `atof`, about twice as long on Sony's, with the same results | time | `shift`, change 2, frame 75 and 73 |
| `pssoft-yakata-yakata` | The same, and it has the `SsUtSetReverbType` and `ResetGraph` differences below, now fixed | time | `shift`, change 2, frame 59 and 57 |
| `test-negimu02` | As tantank, frame 13, Sony's about 11,200 cycles later | time | `shift`, change 1, frame 15 and 14 |
| `test-rog00` | The second `GsInitGraph` zeroes the buffers' corners on clean, so every swap draws buffer 0 | behavior | `same`, 5/5 |

So ricochet's and senpon's cause is shared. The three fixes for mimet, ricochet and rog00 went in together, with senpon clearing at the same time, so which cleared senpon is from its figures and not from trying them one by one.

### The rules that were wrong

**Timer 1's start (`probes/linestart`).** The library's start writes the mode 0107h and nothing else: the counter holds at 0 until the next vertical blank, then runs free, and the mode reads 0106h from there. Spec 08 had the write. Clean then wrote 0100h and set the counter to 0, so it ran from the start, up to a frame ahead. The gap depends on where the blank fell: 156 lines in ricochet, 80 in senpon. It reaches the first `VSync(0)` and `VSync(1)` and every `GetRCnt(RCntCNT1)`. A later `ResetGraph` or `GsInitGraph` doesn't restart it. Spec 02 is corrected.

**`GsInitGraph` and the corners (`probes/initkeep`).** Spec 19 said PSDOFSX and PSDOFSY are 0 after `GsInitGraph`. That was the first call on variables that start at 0. It doesn't touch them, even as the first call after `GsDefDispBuff`. PSDIDX does go to 0, each time. Spec 19 is corrected.

**Null in `GsGetTimInfo` (`probes/timnull`).** With info null the fields go to address 0 on, with the palette or without. With the TIM null it reads from address 0, and the addresses it stores count from 0: pixels at 10h with no palette, and with one the palette at 10h and the pixels at 3Ch. Both null works too. Spec 02 is corrected.

**What the image calls leave (`probes/loadleft`).** `LoadImage`, `StoreImage`, `MoveImage` and `ClearImage` return with the interrupt mask in a0 and 1F801074h in v1, for every size tried, and for all seven masks tried on `LoadImage`. That pokecon's `srand` takes a0 is from the values, F0h and 0Dh, matching on each side. Its code wasn't read. Spec 04 is corrected. On Sony's, a1 to a3 hold other things, and these are not matched: for `LoadImage` and `StoreImage` a1 is the height, a2 is 800482EAh and a3 the pixels; for `MoveImage` 0, 14h and 0; for `ClearImage` E4FFFFFFh, 8006059Ch and 03FFFFFFh. The probe doesn't print them, since no program was seen to use them. Other calls weren't tried.

**`SsUtSetReverbType` (`probes/revctrl`).** Seen in yakata's SPU writes, and worse than it showed there:

- With the reverb on, it clears SPUCNT bit 7 first and sets it again after the 1A2 base, for any type but 0. Yakata shows this. Clean left the bit on.
- Type 0 always writes SPUCNT, on or off, and leaves the bit clear.
- A negative type also clears the work area in sound memory, from the preset's base to the top, by DMA, a kilobyte at a time, before the base goes in. Clean didn't. The probe marks sound memory and reads it back: below the base stays and the rest reads 0 on both sides now, for -1, -3 and -9. Positive types leave it. Spec 23 had only what these return.

The SPU writes in the probe's bus trace are now the same in order and value, 45,226 on each side, leaving out the sound tick's six zero writes, which fall at different places. Sony's waits for each block with DICR and the interrupt registers where clean polls, and the time isn't matched: on Sony's a clear takes 288,000 to 514,000 cycles, and clean wasn't measured. No program in the table uses a negative type. Spec 05 is corrected.

**`ResetGraph`'s result and levels (`probes/resetret`).** Found by comparing results: `ResetGraph(0)` returns 3 and clean returned 0. Only the level's low three bits count: 8, 16, 256, 65536 and -8 act as 0, 11 and 259 as 3, and every other level as 1. Clean did nothing for those. The GP1 and GP0 register writes over levels 5 to 259 are the same now, 128 on each side. Levels 2, 4 and -1 come before the trace starts, so those are from the print and the result only. Spec 05 is corrected.

### Time, where it stops

In all of these, the arguments and results of calls that have a result agree as far as the two sides' calls stay in step, except `FntFlush` and `KanjiFntFlush`, which return an address in each library's own memory, and `ResetGraph`, now fixed. `VSync` and `GetRCnt` return time, so aren't counted here. In tantank and negimu the SPU and GPU writes up to the shift are the same but for where the per-frame zero writes fall.

- **tantank and negimu.** The late cycles build at the CD calls, with `CdSearchFile` to `printf` about +190 a call on Sony's and `CdReadFile` to `CdReadSync` about +960. In negimu `SsUtSetReverbType` is 2,378 the other way. That moves one `VSync(0)` across a blank, a shift of one frame. Tantank's "76,800 pixels" at frame 360 is gone: it came from the wrong line count, and none of its 23 pictures differs now.
- **wstgnbds.** Everything sent to the GPU apart from glyphs is the same, 3,419,828 words on each side. Clean is about 1,060 cycles late over a frame, mostly -856 between its two `GsSortFixBg16` calls, so the program's readout of `VSync(0)` shows one line more.
- **The two yakatas.** `atof` takes 45,582 on Sony's where clean takes 22,668 and 72,974 where clean takes 31,440, with the same results. Over 37 calls that is 1,311,430 cycles, about two frames, with clean ahead. The picture is the same when it comes.

### The 15 `still` programs

All 15 show a black picture at frame 1200 on both sides, 0 pixels differing. None is a difference between the libraries. Addresses below are PCs at frame 1200.

| Program | What it does | Reading | PC, Sony's and clean |
|---|---|---|---|
| `pssoft-madoka-madoka` | Prints its controls, then sorts and swaps every frame to frame 1200 | Runs. It may want input other than Start and Cross, but why the picture is black wasn't found | 8003cbe0, 8003e948 |
| `event-event003-theme-tm-tm`, `-tmnew-tm`, `-tmnew2-tm-c` | "Cannot load \DATA\SOUND\STD0.VB", "End of Main" | Ended: the data is in an archive not unpacked (`resource.lzh` beside it in `tm-tm`; the other two weren't listed) | a0 on both, except `tm-tm` a0, bfc0d958 |
| `pssoft-terrai-n`, `-p` | `CdSearchFile("\DATA\SOUND\GOGO.SEQ;1")` over and over, 1,268,533 calls on Sony's and 2,324,367 on clean | Stuck: the disc has no such file | 8003b214, 8003ed60 and 8003ed44 |
| `showcase-layd100` | "(SEQ)SsVabTransfer failed (-1)", then no more calls | Stuck in the program's own loop, same on both | 800e03d0, 800e0410 |
| `scee-demos-allother-asc2sjis` | Prints its table to the TTY, "End of Main" | Finished: it draws nothing | a0 on both |
| `test-timview100` | "This TIM formar NON-support", "End of Main" | Ended | bfc0d954 on both |
| `tool-memcard-2`, `tool-rest002-2-memcard` | "End of Main" after 6 calls | Ended, with nothing asked of the pad | a0, bfc0d958, and bfc0d954, bfc0d958 |
| `pssoft-rakugaki-rakugaki`, `pssoft-vtrclp-test-4-02` | `init`, then no library call in 1,200 frames | Look stuck. What they wait on wasn't found | bfc0d8e4, bfc0d8e8, and a0, bfc0d8e8 |
| `test-spso` | No library call at all after "Execute !" | Looks stuck, or never started | bfc0d8e8 on both |
| `pssoft-escape` | No library call at all | Never starts: see below | bfc0d8e4 on both |

None of them is shown to wait for the pad in the run's Start and Cross presses, unless madoka does.

### Other things found, not taken on

- **`pssoft-escape` doesn't build.** `eco2exe` panics in `format/psx`'s `memory.read`, slice bounds out of range [65536:0]. Its data reaches the top of memory, and with f60e194's whole number of sectors the end comes to 2 MB, which the mask makes 0. `archive.sh` prints "eco2exe failed" and goes on, so the run tests the disc from before that fix, from 13:30, which holds a text size the BIOS won't load, and an old image. So this program's `still` says nothing, in `summary-verify47.txt` too.
- `archive.sh` doesn't stop or mark a program for a failed build, which is how the above stayed hidden.
- `CdSearchFile` on a missing file takes about half as long on clean, going by terrai's counts of calls. Time, so left.
- `atof`, above, and the CD calls' pacing are the largest time differences among these programs.
- The CD's register writes differ at the start of these programs: 1,149 on Sony's and 1,095 on clean to 1F801800h to 1F801803h, in tantank to frame 14. What was read of the differences is index selects at 1F801800h and the order at the interrupt registers. Not all of them were read.
- `event-event001-rave096` stays `tty`, now 1078906 and 1078958 bytes, and `pssoft-kyuri-2-kyuri` `wild`, with clean's PC at 5430f014, from 5430f010. Not looked into.

### Probes

| Probe | Result |
|---|---|
| `linestart` | identical, 42 lines |
| `initkeep` | identical, 61 lines |
| `timnull` | identical, 40 lines |
| `loadleft` | identical, 54 lines, printing v0, v1 and a0 only |
| `revctrl` | identical, 76 lines, and the SPU writes as above |
| `resetret` | identical, 98 lines, and the GPU writes as above |

`linestart` prints the counter exactly only at a blank. Read just after the library's start, the two sides are a line apart by phase (1115 and 1116).

Existing probes for what changed, all rerun on the final image: `gspsd`, `gsinit`, `gsclip`, `sortclear`, `storeimage`, `clearimage`, `dispsys`, `dispswap`, `sndcalls`, `seqnoise`, `vsyncone`, `resetwait`, `timbytes`, `flush`, `readsync`, `isbg` and `vabrefuse` are identical. `vsynclate` differs in one line, "End of Main" after `END`, which only Sony's run caught before the tool stopped. It isn't a result of the probe, whose lines are all the same.

`TestArchiveCalls` now records a0 in what a call leaves, and `TestArchiveGP0` writes a program's GPU words a frame at a time.

### Acceptance

`FIRST=1 exp/cleanlib/image/archive.sh` on the ten, on the final image, gives the table above. The full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify47.txt`, with its copy in `summary-agent37.txt`:

| | same | differ | shift | still | tty | stall | wild |
|---|---|---|---|---|---|---|---|
| Before | 86 | 27 | 9 | 15 | 1 | 1 | 1 |
| Now | 93 | 20 | 9 | 15 | 1 | 1 | 1 |

Nothing got worse. Besides the ten, `demos-scee-morph` and `scei-sample-graphics-cube` went from `differ` to `same`, `test-bomb96-bomb` from `differ` to `shift`, and `event-event001-hhpd0002` first differs at change 10, frame 19, from change 2, frame 11. `event-event001-rave096`'s and `pssoft-kyuri-2-kyuri`'s lines moved, as above. Which fix moved these four wasn't looked into.

- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME, as asked.
- `exp/cleanlib/image/regress.sh`: 36 PASS and 5 TIME, not the 34 and 7 asked for. The two extra passes are better, not worse: spec 35 lists `flame` and `timers` in that TIME set, and both pass now, having read their lines from the corrected counter. No baseline run of this list was made today, so that they are these two is from spec 35. TIME: `check`, `dungeon`, `morph`, `offscr`, `terra`.
- `exp/cleanlib/image/terra.sh`: all pass, TestTerra, TestTerraStack, TestTerraOrder, TestTerraOrderPressed and TestTerraCosts.
- `go test ./tests` passes, with `PSX_SONY_BIOS` set and `PSX_BIOS` not.
- The image ends at 80066288h.

The sysroot is refreshed: `task compiler:overlay`, `task --force playground:sysroot`, `task yaroze` and `task image`, all exit 0.

### Not checked

- `pssoft-escape` wasn't compared on this library at all.
- The reverb clear's time on clean, and a clear in a running program: no program here was seen to use a negative type.
- Whether other calls that go through the interrupt mask, such as `DrawOTag` or `PutDrawEnv`, leave it in a0 too.
- `ResetGraph` levels 2, 4 and -1 against the GPU's writes, and the queue emptying for levels other than 1, which the probe can't see.
- The effect on the static build in itself: the same Go serves both, and `regress.sh` ran on it, but the new probes ran on the image only.
- Why madoka's picture is black, what rakugaki, vtrclp and spso wait on, and whether the other two theme programs' data is in the same place.
- Other programs' `srand` calls: whether more take an argument from a register.
