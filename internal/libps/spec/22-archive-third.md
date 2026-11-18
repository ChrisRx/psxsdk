# The web archive's programs: third round

After specs 20 and 21, `exp/cleanlib/image/archive.sh` has 81 of 140 programs `same`. This
spec holds what the remaining ones showed: two behaviors, then the calls where the clean
library takes longer than Sony's. Spec 20 tells how to run things and spec 21 the timing
tools.

## GsScaleScreen does not build up

GsScaleScreen(scale) starts from the world-to-screen matrix as the last GsSetRefView2 or
GsSetView2 left it, not from the matrix as it stands now. Calling it twice with 0.8 gives
0.8, not 0.64. `test-dydref01` calls it every frame, and on the clean library its
rotation matrix and translation shrink away: -927 for -1450 in the last element after two.

Each row of the rotation and each element of the translation is multiplied by that row's
element of scale, in 4.12. The rotation goes as ScaleMatrix does it, the translation as
(s·t)/4096 rounded towards zero in 32 bits. It takes 289 cycles.

## GsSetFlatLight and GsSetLightMode

**GsSetFlatLight(id, lt)**

- A direction of no length returns -1 and changes nothing, whatever id is. The length is the
  square root, SquareRoot0's way, of Vx·Vx + Vy·Vy + Vz·Vz, in 32 bits: it wraps.
- id outside 0 to 2 returns 0, not 1, and sets no light. The color matrix still goes to the
  GTE.
- A row is (-Vx<<12)/d and so on, in 32 bits, rounded towards zero, kept to 16 bits.

**GsSetLightMode(mode)**: a mode outside 0 to 3 prints `not supported light mode %d\n` with
the mode by the kernel's printf and changes nothing.

## Polygon forms in GsSortObject4

Found by the last round's black-box probe, over 16 kinds, each with and without the
translucent bit, under 4 attributes. Check each against Sony's library yourself with a probe
of your own under `.build`, since this says what, not every field of how.

- A lit 20h with flag bit 2 (a color to each corner) is sorted as a 30h packet. The clean
  library skips it.
- With attribute bit 6 (unlit), a textured gouraud polygon of 34h or 3Ch gives a flat
  textured 25h or 2Dh packet, with the first color word as its color. The clean library
  gives a gouraud one. `test-pd961207` shows it at word 254: 25808080 against 34808080.

Also look at `event-event003-free-mai`, word 3056 (38736475 on Sony's, 28675453 on the clean
library), and `test-dydtst02`, word 209 (28 packets of other colors and corners), and
`test-mot001-mot001`, word 97 (Sony's sends a 20h first, the clean library a 28h, and
368 more words in all). Find out by a probe if they are more of the same. If one is not,
describe it in your report.

## Calls where the clean library takes longer

Calls sum to Sony's time only where the clean call is no slower. These stop programs reaching
frames on time, delay their first picture, and change every frame time a program prints on
the screen: `demos-scee-chap1-1` is now one frame late only because of its 8 printf calls at
18,085 cycles each against Sony's 15,846.

From `.build/carchive/costs.txt`, cycles a call, biggest total first:

| call | Sony | clean |
|---|---|---|
| GsSortObject4 | 4,885 | 7,134 |
| GsSortFastSprite | 95 | 140 |
| GsSetRefView2 | 1,850 | 5,173 |
| CdPlay | 4,813 | 57,011 |
| FntPrint | 920 | 1,243 |
| ApplyMatrix | 47 | 167 |
| GsSetFlatLight | 463 | 2,158 |
| GsSortFixBg16 | 15,576 | 19,194 |
| KanjiFntPrint | 616 | 1,782 |
| GsLinkObject4 | 461 | 653 (the probe's long models match: small ones are over) |
| printf | 19,560 | 22,473 |
| GsSortSprite | 243 | 264 |
| FntFlush | 2,915 | 3,113 |
| ApplyMatrixSV, ApplyMatrixLV | 53, 125 | 111, 188 |
| RotMatrix | 242 | 392 |
| RotMatrixX, Y, Z | 178 | 330 |
| GsSortBoxFill | 173 | 187 |
| `__divsf3` | 439 | 4,903 |
| gteMIMefunc | 5,468 | 6,399 |
| GsSwapDispBuff | 4,602 | 4,721 |
| rand | 24 | 28 |
| GsSortClear | 162 | 229 |
| GsClearOt | 272 | 318 |
| LoadImage | 747 | 826 |
| TransMatrix | 18 | 30 |
| PushMatrix, PopMatrix | 62 | 137 |
| sprintf | 646 | 796 |
| StartRCnt, GetRCnt, ResetRCnt | 26 | 95 |
| GsSortLine | 170 | 194 |

Sony's library is C and hand-written assembly. Some of these can be matched in plain Go by
leaving out work: a square root by the GTE's leading zeros and a table in GsSetFlatLight in
place of 64-bit arithmetic, no spills around a GTE sequence, and no stack copy of a matrix.
Where Go can't, a short routine in `image/clib.S`-style assembly is allowed for the calls
that matter most: GsSortObject4's inner loops, ApplyMatrix and the like, RotMatrix, and
GsSortFastSprite. **Readability counts: weigh each gain. Do not take a clever trick for a few
cycles on a call nobody makes often.** Take them in order of total, stop when gains get small,
and say where you stopped.

Timing on the linked-in target is compared with the reference library, so these changes can
move `exp/cleanlib/regress.sh` too. A TIME there is acceptable where the reason is clear from
this work. A FAIL is not.

## What the work on this spec found

Probes of Sony's library by the agent that did the work. Where these disagree with the
sections above, these are right.

**GsScaleScreen**: the rotation scales by column, as ScaleMatrix does, not by row. The
translation goes by row.

**GsSetRefView2**: when it returns 1, for two equal points, the matrix GsScaleScreen starts
from does not change. While any element of either point has a magnitude of 32768 or more, both
points shift right together, then subtract. Spec 04 left this open. About 130 probe views match.

**GsSetFlatLight**: a negative wrapped sum reads the sine table at 970 + (shifted v), which
looks like the table before the square roots. All 86 probe lights match. GsSetRefView2 uses the
same 32-bit root.

**GsSetLightMatrix** keeps the GTE's rotation. `event-event003-free-mai` and
`test-mot001-mot001` load the draw matrix before calling it, which is what their first
differences were, not the polygon forms above.

**Polygon forms**:

- Lit or unlit comes from mode bit 0. The flag's own bit 0 is ignored.
- Flag bit 2 matters only on 20h and 30h. Those packets take no translucency and are never cut.
- 34h and 3Ch under a GsDIV stay gouraud.
- Flag bit 1, double-sided, is ignored, as the member FAQ's question 14 says, except on a
  24h or 25h being cut.

**CdPlay(3)** does not wait for Getstat, 974 cycles on Sony's. After a data end the call has
no Getstat of its own, against spec 05.

**FntPrint from C**: 36 cycles a character on the probe, not spec 08's 50.

**GsSetLightMode** writes GsLIGHT_MODE, not GsLMODE. The clean library still writes GsLMODE.

Still open: flat untextured quads, 28h and 29h, at GsDIV level 2 have one center point 1 pixel
off in x, unexplained. The ignored top bytes of gouraud pieces differ, as `div.go` notes. The
high halves of later piece corners' texture words may come from the scratch area on Sony's: the
clean library zeroes them, and the probe's work area was all zeros.

Three pads sit under Sony's per-unit figures so that the archive's means line up: FntFlush's
glyph by 14 cycles, FntPrint's C character at 32 against 36, GsLinkObject4's primitive at 56
against 62. GsSortObject4 remains about 13% over Sony's. What is left there is load-delay nops
the compiler places and FLAG reads through `$gp`, about 16 loops in assembly to remove, which
was judged not to be a short routine. No assembly was used.

Costs after this work, Sony/clean in cycles a call, against the table above:

| call | after |
|---|---|
| GsSortObject4 | 4,185/4,745 |
| GsSortFastSprite | 95/94 |
| GsSetRefView2 | 1,888/1,878 |
| CdPlay | 4,813/4,787 |
| FntPrint | 945/960 |
| ApplyMatrix | 47/64 |
| GsSetFlatLight | 463/459 |
| GsSortFixBg16 | 15,576/17,697 |
| KanjiFntPrint | 616/738 |
| GsLinkObject4 | 470/540 |
| printf | 19,560/21,302 |
| RotMatrix | 243/327 |
| `__divsf3` | 439/358 |

Not improved: gteMIMefunc 5,468/6,422 and sprintf 646/820 are slightly worse. rand, TransMatrix,
PushMatrix and PopMatrix did not change.
