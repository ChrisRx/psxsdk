# SsUtKeyOn's choice of voice

In `pssoft-airbob-2-airbob` at frame 600, `SsUtKeyOn` returns 7 on the clean library and 0 on Sony's, with the same arguments. It returns the voice it took, so the two are playing the sound on different voices, which is a difference in behavior and not just time. Spec 05 has the rule for choosing a voice, from `probes/voices` (69 cases).

## Work

1. Find what differs before that call. Compare both sides' calls in the program that touch voices (`SsUtKeyOn`, `SsUtKeyOff`, `SsUtAllKeyOff`, `SsUtReverbOn` and so on, the sequencer calls, and whatever else it uses) and what the SPU is told, from the start of the program up to the first return value that differs. Use `TestDump` with `PROBE_WATCH` and `TestArchiveBus`.

   It may be that the state the choice depends on has already diverged, or that the choice is wrong from equal state. Say which. If it depends on time, such as an envelope read from the SPU at a moment that differs between the two sides, say so, because that may be a phase difference and not a bug in the choice.
2. Write a probe that reproduces it on both libraries with no program involved, calling the library by name, and keep it in `probes/`. If it's a phase difference and a probe can't reproduce it on equal state, say that and stop. Don't fit anything to this program.
3. Correct the rule in spec 05 and the library to match, if the rule is what's wrong, for every case the probe covers. `probes/voices` must keep matching on all its cases.
4. Say what `pssoft-airbob-2-airbob` shows now. Getting it to `same` isn't the aim: what stops it may still be spec 34's time difference.

Look for other programs this affects: run the same comparison of `SsUtKeyOn`'s return values on the archive's other programs that call it, and say how many differ and at which call. Don't take on a different bug you find there. Name it.

## Acceptance

Results first, then time by cycles.

- `exp/cleanlib/image/probe.sh` on `probes/voices` and the new probe: the TTY of the two sides is identical.
- `FIRST=1 exp/cleanlib/image/archive.sh` on `pssoft-airbob-2-airbob`, then the full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify44.txt`: name anything that got worse.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 34 PASS and 7 TIME. `exp/cleanlib/image/terra.sh`: all pass. `go test ./tests` passes.
- The image ends below 80068000h, now 80066288h.

If the library doesn't change, say so and skip the full runs.

## What the work on this spec found

The choice of voice was never wrong. Frame 600's 7 follows from a wrong sample address at frame 354, and spec 05's rule for that address came from a mistake in a probe. It isn't time and it isn't phase. The library changed: `exp/cleanlib/vab.go`, and spec 05 is corrected.

### What first differs

`pssoft-airbob-2-airbob` makes 14 sound calls up to frame 1064: `SsVabTransfer` and `SsSetMVol` at frame 53, `SsUtAllKeyOff` at 97, then `SsUtKeyOn` at frames 354 to 360 (tone 8 at several notes, voices 0 to 6 on both sides) and at 600 and 840. Arguments and all earlier results are the same.

`TestArchiveBus` over frames 353 to 602, the SPU's writes tick by tick. The first difference is the first key on's start address, written at the tick after frame 354's call, about 246 frames before the different return value:

| | Sony's | clean |
|---|---|---|
| Voice 0's writes | 006=0f02 | 006=ffff |
| The other six voices' | 0f02 | ffff |
| Envelopes 200 ticks into the trace | 0000 for all seven | 0a1f to 0b23 |
| Key on at frame 600 | voice 0, 006=0958 | voice 7, 006=ffff |

In the first 20 ticks of the trace, that register is the only difference in the writes, and the envelopes read the same. So the state had diverged already. On Sony's the voices go quiet by themselves, and the 16 zero reads free them. On clean they play from the top of sound memory and, going by their envelopes, find no end: they fall on the sustain rate and are still sounding at frame 600. The choice from that state is right on both sides, and `allocVoice` didn't change.

### The rule that was wrong

Airbob's bank (`DATA/SOUND/SE.VH`) has 1 program and 14 VAGs, and zeros in the entries' bytes 12 to 15. Spec 05 said that halves from 2p, p the program count, hold ffffh whatever the header holds, so VAG 3 and up played from ffffh.

The start address is half h of the header, bytes 12 to 15 of entry h/2, for all 256 halves, read at the key on. No count bounds it. Half 256 and up gives 0, as before. This holds for SsUtKeyOn and a sequence's note on alike.

`probes/vagtable` shows it. Each bank is built in memory, with every half marked A000h + h before the transfer, so what the transfer wrote and what's left can be told apart:

- The transfer writes vs + 1 halves and leaves the rest: "bank 1 14" leaves 241 of the 256 marks, "bank 3 4" 251.
- `SsUtKeyOn` at VAG v reads half v - 1. On 1 program and 14 VAGs, v 3 gives 0208h and v 15 the end, 022ch. v 16 gives its mark a00fh, v 254 a0fdh, v 256 a0ffh, and v 257 0000h. With half v - 1 marked 4000h + v - 1 after the transfer, each gives the new mark, so all of them are read from the header at the key on. On 3 programs and 4 VAGs v 7 gives its mark a006h, where the old rule said ffffh.
- A sequence's note on gives the same addresses, tried on those two banks with v from 1 to 17, unmarked.
- The counts don't matter afterward: 8 programs and 1 VAG written over 1 and 14, or the reverse, change nothing.
- Two banks open at once each read their own header.
- `voice` cases reproduce the program with nothing else involved: seven key ons of VAG 9, one a frame, on a bank of 1 program and 14 VAGs whose samples end, then a key on 20 or 240 frames later. Before the fix it returned 0 on Sony's and 7 on clean, with clean's envelopes at 01d0 to 02d3 after 240 frames. VAG 1, in range under the old rule, gave 0 on both.

**How spec 05 went wrong.** `probes/seqexpr`'s `far` marks wrote eight bytes from entry 2's byte 12: `set(entry2+12, 0x44, 0x04, 0x55, 0x05, 0x66, 0x06, 0x77, 0x07)`. Only the first four are halves. The other four land in entry 3's bytes 0 to 3, so halves 6 and 7 were never marked, and mkvab's file holds ffffh there. Every "ffffh" in those cases is the file's, and the program count had no part in cases 69 to 72. The probe is left as it is, since its output still matches. Spec 05's text now says this.

The probe found one more difference, which the acceptance needs: `SsVabTransfer` refuses some headers, and clean took them. `probes/vabrefuse` has it. A VAG count of 0 or above 255, or sizes that come to 0, returns -1 with nothing written and no sound memory taken. Program count 0 with VAGs is accepted, and so is a VAG count of 255. Sizes were only tried all 0.

### Now

- `probes/vagtable`: TTY identical, 320 lines. `probes/vabrefuse`: identical, 48 lines. `probes/voices`: identical, 3,254 lines, all 69 cases, and spec 05's three "quiet after" counts that differed by a frame or two now match too. It needs `PROBE_FRAMES` of about 14,500 to reach its end, since it never prints `END`.
- `vabhdr`, `vabsize` and `seqnoise` are identical. `seqexpr` differs only in the tempo 0 case at the end, as spec 05 says.
- `seqhdr` differs in one line, `koff 59 right`: env 8000 on Sony's, 4000 on clean. It is the same on a disc built without this change, so it was already there. Not taken on.
- `pssoft-airbob-2-airbob` is still `differ` at change 201, frame 1068 on both sides, 37,919 pixels, exactly as before. All its `SsUtKeyOn` results now match, 12 of 12 to frame 1200, and the SPU's writes and envelope reads over frames 353 to 602 are the same line for line, without the cycle counts. What stops it is spec 34's phase.

### Other programs

`TestArchiveCalls` at 1,200 frames on all 140 programs. 9 don't boot on either side, so can't be compared: `demos-scea-ricochet`, `demos-scee-tunelfin`, `event-event001-tantank`, `event-event003-free-hijump-hijump`, `pssoft-tantank2-2`, `scee-demos-all3d2-follow`, `showcase-gasgar`, `showcase-t-tank2f`, `test-kbo3d06-3d`. Of the other 131, 18 call `SsUtKeyOn`.

| | Differ | Where |
|---|---|---|
| Before | 2 of 18 | `pssoft-airbob-2-airbob`: 1 result, call 8, frame 600. `event-dengeki001-nanatan-nanatan`: 11 of 28 results, from call 6 at frame 601 (2 on Sony's, 3 on clean) |
| After | 0 of 18 | |

`event-dengeki001-nanatan-nanatan` was `same` in pictures before and is still. That its cause was the same is from the fix clearing it. Its header wasn't read and its SPU wasn't traced.

### Acceptance

Against `.build/carchive/summary-verify44.txt`, the counts are unchanged: 102 same, 4 shift, 22 differ, 1 tty, 1 stall, 1 wild, 9 noboot. One line moved: `test-gr-01` went from 375/362 changes to 375/364, still first differing at change 2, frame 15. Nothing got worse. It doesn't call `SsUtKeyOn`, and why its count moved wasn't looked into.

`exp/cleanlib/regress.sh` without `PSX_BIOS`: 33 PASS and 8 TIME. `exp/cleanlib/image/regress.sh`: 34 PASS and 7 TIME. `exp/cleanlib/image/terra.sh`: all pass. `go test ./tests` passes. The image ends at 80066288h.

### Not checked

- A VAG count of 255 has 256 values to write, and the clean transfer's loop stops at half 254. Whether Sony's writes half 255 wasn't looked at.
- `SsVabTransfer`'s sizes with the first 0 and a later one set, and whether the VAG count limits which sizes are summed.
- Halves at negative indices behave as they did. The `vagtable` probe doesn't try them, and `seqexpr`'s cases still pass.
