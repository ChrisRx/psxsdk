# Accuracy: what's left in Go

After spec 30 the image has 97 of 140 programs `same` (98 in truth: `scee-demos-allother-asc2sjis` is a character the emulator's capture of printed text doubled). Of the rest, 14 differ only in figures they print, 11 aren't the library's doing, and these 8 are off in time: `scee-topten-cardem`, `event-event001-rave096`, `test-demo-fm`, `pssoft-koro-koro`, `scee-topten-adven`, `pssoft-airbob-2-airbob`, `test-pd961207`, `test-ftptest`.

All of this concerns the `fidelity` build (spec 30): what you add to match Sony's goes through its operations, so that static programs don't pay for it. The aim is to match Sony's as closely as black-box observation allows, and assembly is open everywhere.

## Work, in this order

**Cut polygons.** `libps_gso_div` is the last of GsSortObject4 in Go. By cycles it is +12.8% sorted at level 1, +5.7% at level 2, +2.5% at level 3, +45% on a near-drop, -22.9% off-screen, and Sony's lights a cut polygon before dropping it, which the clean library doesn't yet (spec 25). Write it in assembly, in the same way as the uncut loops: Sony's GTE order and spacing from `PROBE_BUS`, at Sony's cycles per path, for each kind and level. `test-dungeon-dungeon`, `test-dyddem01` and `test-dydref01` use it. 250 packet differences under cuts are top bytes the GPU ignores: probe whether they can be matched too.

**printf and sprintf conversions.** printf's `%d` is about +480 and sprintf's about +180 with +68 fixed. Assembly or restructuring as needed, by conversion, and match the rest of `cformat.go`'s paths you can measure. `event-event001-rave096` reaches 99.6% of Sony's output.

**sin and cos.** Sony's varies by about 10,000 cycles with the argument. Sum matches on `event-event002-r3`, and `test-demo-fm` is 2,000 a call short. The clean ones are paced by range. Work out what the time follows by a finer probe: Sony's soft float costs differ with operand bits, so the time likely follows the series' intermediate values. Writing the same computation with the soft float's real paths, in place of a pad, is the way to get it right for every argument. Do the same for Sqrt, which spreads widely by argument (215,050 and 267,508 at 3.5), and tan, asin and the like, not measured since `dFloat` changed.

**The rest of the math and sort:** GsSortFixBg16 (15,909 and 15,576), FntFlush with text (+36 to +40), RotMatrixY (+4 at some angles), memmove, GsSetRefView2's twisted path (1,853 and 1,861), `__addsf3` (+50 to +190 a sum), InitHeap (14 short), `_err_math` (+10 to +18), Krom2RawAdd2 (+218 an ASCII glyph), SetVideoMode (+24 to +32), puts (+14 a character).

**FntPrint's unknown letters** (spec 25): `%u`, `%o`, `%ld` and `%-5d` take time and flush as if the word were a width: 278, 654, 4,428 and 17,664 cycles for 0, 12, 123 and 1234. Probe and match.

**The interrupt's kernel-side entry** differs per interrupt: +408, +206 and -1,158 in different programs (spec 25). Probe by `TestArchiveBus`, and find what it follows.

**`goapi.go`**: `exp/ps` no longer uses it, since it sits on `yaroze`. Delete it.

## Acceptance

Time by cycles only. Say what each of the 8 programs shows and what stops any that isn't `same`.

- `FIRST=1 exp/cleanlib/image/archive.sh` on the 8 programs, then the full `exp/cleanlib/image/archive.sh`, against `.build/carchive/summary-verify35.txt`: name anything that got worse.
- `ARCHIVE_TEXT` on `demos-scee-balls demos-scee-morph event-event001-hhpd0002 scei-sample-graphics-bmirror scei-sample-graphics-cube test-bomb96-bomb test-dungeon-dungeon test-dyddem01 test-dydref01 test-dydtst02 test-gr-01 test-wnfs-t2-wnfs-t2 test-wstg-tv3`: still no difference.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`, `exp/cleanlib/image/regress.sh`: 33 PASS, 8 TIME, 0 FAIL. `exp/cleanlib/image/terra.sh`: all pass.
- `task --force build:discs`, then `go test ./tests`: static programs still work, and goldens differ only in printed readouts, named for the spec side.
- The image ends below 80068000h. It ends at 80066288h now, with only 7.5 KB to spare, and more assembly may not fit: say so early if not, since the spec side lays it out.
- `costs.txt` refreshed, with before and after.

## What the work on this spec found

Done: cut polygons, printf and sprintf conversions, soft float pacing with sin and cos, and deleting `goapi.go`. Still open: the rest of the math and sort, FntPrint's unknown letters, and the interrupt's kernel-side entry. The probes are `probes/acc*`.

GsSortObject4 under cuts now matches Sony's in 1,760 probe cases by GTE trace and cycles, and 1,056 more by cycles, with packets the same word for word. Levels 4 to 7, unlit double-sided `ft3`, own-mode attributes and n_prim 0 aren't checked on the cycle-exact side, and grad polygons don't match in scratch.

Corrections to earlier specs, from probes:

- Spec 25 and `tmd.go` said scratch isn't written. Sony's work area under cuts is the scratch argument: level, screen size, palette and page, first color, root + 4000h past NCLIP only, then 6-word cells and frames of 22 words for triangles and 35 for quads. Quad cells go ab, ca, be, ec, center. A level 6 or 7 quad overruns a 1 KB scratchpad, on static builds too.
- Spec 04: in fog, gouraud corners under cuts get their own RTPS and IR0, a fogged gouraud quad's entry is AVSZ3 of its first three points, and a flat quad's center z is (ab + ec)>>1.

Soft float time follows the operands: a sum is a base by signs and order, 24 cycles per exponent place, 60 for a carry and about 81 per place of renormalizing. A product is 3,832 plus 4 per set mantissa bit, and a quotient 264 per set quotient bit. Sin keeps pads fitted on program sums (6,080 at 0, 7,300 below pi/2, 3,200 above), since a real program's arguments sit about 3,200 above the arithmetic's sum for a reason not found. Sqrt is still off by up to about 30,000 per argument, and sqrt(1) by 68,000. Tan, asin and the like weren't measured, and got slower with the paced arithmetic.

`demos-scee-chap5-1` moved from `same` to `differ`, on one digit of the `GetRCnt(1)` value it prints: what it sends the GPU apart from glyphs is the same. `scee-demos-allother-asc2sjis` returning to `same` doesn't mean the emulator's doubled character is gone.

Fidelity builds grew: `tunnel-clean`'s `MAIN.EXE` is 196,608 bytes and at 204,800 it fails on spec 25's disc-layout trap, which is why `gso_sled` exists. Other programs near that size can hit it. The instruction score didn't rise with this work.
