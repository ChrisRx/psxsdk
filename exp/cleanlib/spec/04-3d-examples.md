# Milestone 4: the other 3D examples

After flame, the 3D examples that play no sound, roughly in the order
that each adds little to the one before. Their reference discs are built
(`.build/NAME-ref`). For each:

    exp/cleanlib/build.sh NAME-clean examples/NAME
    PSX_CLEAN=NAME PSX_CLEAN_FRAMES=300 go test ./tests -run 'TestClean$' -v

and once with input that moves the camera or model (read the example's
package comment for its controls and pick buttons, in the controller's
bit order from `emu/pad.go`). As before, on-screen readouts of frame
time may differ; every other word must match.

| Example  | Adds (beyond flame)                                                        |
|----------|----------------------------------------------------------------------------|
| morph    | textured TMD primitives                                                    |
| car      | lighting: GsSetAmbient GsSetFlatLight GsSetLightMatrix GsSetLightMode, GsGetLw, GsSetRefView2 |
| oden     | fog: GsSetFogParam; ScaleMatrix                                            |
| split    | GsSetClip GsSetOffset (two views on one screen)                            |
| dungeon  | ApplyMatrixLV GsGetLws GsSortLine; fog                                     |
| offscr   | GsSortFastSprite MoveImage PutDrawEnv (drawing off screen, then using it)  |
| scene    | GetClut GsInitFixBg16 GsSortFixBg16 memcpy memmove                         |
| jumpcube | as scene, with sprites and boxes                                           |
| quest    | as scene                                                                   |

A link error names any export still missing. No resident address is
defined in a clean build: the CD entry points the resident library has
beyond symusr.o are in `playstation-yaroze-cd.S`, which defines nothing
under `LIBPS_GO`, so they fail at link time too.

## Notes

**TMD primitives.** The File Formats manual lists every primitive kind:
flat or gouraud, triangle or quad, textured or not, lit or not (the
light-off flag), single or double sided, gradation. Implement each kind
the examples use; note in a comment which kinds remain. For lit
primitives the GTE computes the color from the normal (NCCS/NCCT or
NCDS/NCDT with fog, NCS/NCT for flat colors as the manual's light mode
says); the packet's color words come from the GTE's color FIFO.

**Lighting state.** GsSetFlatLight(id, light) sets light id's direction
(normalized to 4.12, as the manual says) and color. GsSetLightMatrix(m)
loads the GTE's light matrix as the light directions times m's rotation,
and the light color matrix from the colors. GsSetAmbient(r, g, b) sets
the background color (4.12 values). GsSetLightMode(mode) chooses normal
lighting, fog, or both. GsSetFogParam(fog) sets the depth cue (the GTE's
DQA and DQB from dqa and dqb, and the far color). The library's light
variables in `playstation-yaroze-libps-data.c` (GsLMODE, GsLIGNR,
GsLIOFF and the rest) are the state programs may read or write directly:
keep them as the source of truth.

**Coordinate helpers.** GsGetLw gives the local-to-world matrix, GsGetLws
both local-to-world and local-to-screen. GsSetRefView2 is GsSetView2
with the view's coordinate system taken into account (the manual).
ApplyMatrixLV multiplies a long vector by a matrix through the GTE.

**Screen.** GsSetClip(rect) sets the drawing area of the current buffer
and GsSetOffset(x, y) its drawing offset, both taking effect at the next
GsDrawOt (as the stream shows). PutDrawEnv sends a full drawing
environment; MoveImage copies within VRAM (GP0 80h).

**Fixed backgrounds.** GsInitFixBg16(bg, work) builds, in the work area,
the sprite packets a GsBG of 16 by 16 cells needs to fill its window, and
GsSortFixBg16(bg, work, ot, pri) updates them from the map, scroll and
color and links them in. The manual gives the work area's size. GsBG,
GsMAP and GsCELL are declared in `yaroze/`.

**Sprites.** GsSortFastSprite is GsSortSprite without rotation or scaling.
If an example rotates or scales sprites, implement that now: the GTE's
or Go's arithmetic must give the reference's vertices.

## Findings (milestone 4)

split, jumpcube and quest pass TestClean with and without input. car
and scene match word for word, font included. morph, offscr and flame
differ only in their timing readouts. oden and dungeon differ only in
subdivided polygons (below).

Corrections and additions to the specs:

- **Sines.** The second and fourth quarters read the table one entry
  early: sin(1024+d) = T[1023-d]. A negative angle gives -sin(-a), and
  cos(-a) = cos(a).
- **RotMatrix rounding.** With s and c the sines and cosines of the
  three angles, every >> an arithmetic shift (rounding down), and
  a = -sy·cz>>12 and b = -sy·sz>>12:
  - m00 = cy·cz>>12, m01 = -cy·sz>>12, m02 = sy
  - m10 = cx·sz>>12 - (sx·a>>12)
  - m11 = cx·cz>>12 + (sx·b>>12)
  - m12 = -sx·cy>>12
  - m20 = sx·sz>>12 + (cx·a>>12)
  - m21 = sx·cz>>12 - (cx·b>>12)
  - m22 = cx·cy>>12

  The sign goes on the first factor before the shift (-sy·cz>>12, not
  -(sy·cz>>12)), and the products of three are Y with Z first, then X.
  Probe `spec/probes/rotscale` on Sony's library: 300 angle triples
  from a fixed sequence, all nine elements match. An earlier form of
  this rule, with -sx·sy first, matched morph, car, split and flame but
  is 1 or 2 off on about half of general triples (terra's lighting).
  ScaleMatrix multiplies column j by scale j: m[i][j]·s[j]>>12 (the same
  probe, all cases).
- **Square root.** The library's square root shifts its input v right
  by an even number of bits s into 64 to 255 (m = v>>s, or left for a
  small v), takes the table value t = floor(sqrt(m)·512), the square
  root in 9 fraction bits, and returns (t<<(s/2))>>9: 999 for 1,000,000,
  142 for 20,400. A table in 12 fraction bits gives the same results
  for small inputs but not for large ones, where s/2 brings the low
  bits out (probe `spec/probes/refview` on Sony's library: 400 random
  views, the 360 whose squared lengths are below 2^31 all match with 9
  bits, 70 differ by 1 with 12). GsSetRefView2 and GsSetFlatLight use
  it. A squared length of 2^31 or more (points over about 46,000
  apart) gives values not explained yet.
- **GsSetRefView2.** Two turns built from ratios, with h and l from the
  square root above: cy = dz·4096/h, sy = dx·4096/h, cx = h·4096/l,
  sx = dy·4096/l. The rows are (cy, 0, -sy), (-sx·sy>>12, cx,
  -sx·cy>>12), (cx·sy>>12, sx, cx·cy>>12), then GsIDMATRIX2, then the
  translation (-WS·vp)>>12. The twist (Rz) is not checked.
- **GsIDMATRIX2** is diag(4096, 4096·4h/(3w), 4096): y is halved at
  640 by 240.
- **GsInit3D** sets DQA = -4194 and DQB = 0x1400000. GsSetProjection
  leaves them.
- **GsSetFlatLight.** The light row is normalize(-v) by the square root,
  rounded toward zero. The color is c·4096/255 rounded down (D0h gives
  D0Dh, FFh gives 1000h).
- **GsSetAmbient** keeps only multiples of 16 (r>>4<<4).
- **GsSetLightMatrix** is GsLIGHTWSMATRIX times m, through the GTE.
- **GsSortObject4.**
  - A polygon is dropped when GTE FLAG bit 31 is set after RTPT or
    RTPS.
  - Textured unlit primitives use the mode with bit 0 cleared as the
    code (2Dh gives 2Ch).
  - Textured lit primitives light 808080h and use the mode as the code.
  - GsDOFF hides the object.
- **LoadImage.** The CPU writes the leftover words from the start of the
  image, then DMA sends the whole blocks.
- **ResetGraph(1)** writes GP1 02h, then 01h.
- **VSync's return value** is the lines from the previous VSync's return
  to this call, before it waits (offscr shows 45, not 263).
- **GsSortFastSprite** ignores Mx and My.
- **GsSortSprite, rotated or scaled.** A textured quad (2Ch) with no E1h
  word. Each corner is X + ((dx·cos - dy·sin)>>12), with dx = (corner -
  Mx)·Scalex>>12. Corners use W and H, texture coordinates W-1 and H-1.
  The angle is Rotate/360. Checked against jumpcube.
- **GsSortFixBg16.**
  - E5h with (-(scrollx&15), -(scrolly&15)), then one E1h and 7Dh sprite
    per cell, row by row, then E5h back.
  - The window is W/16+1 by H/16+1 cells.
  - Positions are for the buffer after the coming swap, plus POSITION
    and the window's X and Y.
  - The 7Dh sprite has color 0 at brightness 128,128,128 (only that case
    seen).
- **Library variables.** TinyGo ignores `//go:extern` on a variable inside
  a `var (...)` group, so each one is declared on its own.

### Subdivision (GsDIV)

Probed on Sony's library with `spec/probes/div` (one primitive at a
time, the stream recorded and the ordering table walked after the sort).
GsDIV1 cuts each polygon once, into 4 pieces; GsDIV2 cuts each of those
again, into 16. Already known from oden and dungeon: pieces are cut in
model space with halved values, a quad's center is the sum of its four
corners over four, triangle pieces are [Pi, mid(Pi, next), mid(Pi, prev)]
and the middle [m01, m12, m20], and quad corners go round 0, 1, 3, 2,
each quad piece being [vi, mid(vi, next), mid(vi, prev), center].

**Order.** Within one ordering table entry the last packet linked is
drawn first. Recursion goes depth first: each piece is cut all the way
down before the next piece is started.

- Quads, at every level: pieces are made and linked in corner order,
  v0, v1, v2, v3.
- Triangles, above the last level: the pieces are cut in the order P0,
  P1, P2, middle.
- Triangles, at the last level: the four leaves are linked in the order
  P1, middle, P0, P2. So GsDIV1 links (P1, middle, P0, P2), and GsDIV2
  links the leaves of P0's piece in that order, then P1's, then P2's,
  then the middle's.

**Depth.** Every piece goes into the same ordering table entry: the
whole polygon's, from its original corners as without subdivision
(AVSZ3 or AVSZ4 then the shift). A floor running past the camera keeps
the entry the undivided polygon would have had.

**What is dropped.**

- A polygon facing away (NCLIP) is dropped whole, with or without
  subdivision.
- The GTE error check (FLAG bit 31) does not apply to subdivided
  polygons or their pieces. A floor whose near corners are behind the
  camera, which the library drops undivided, is kept and cut, and the
  corners behind the camera come out as the GTE gives them (saturated,
  such as (1023, 199) with h 300).
- Before a polygon or a piece is cut further, it is dropped, with
  everything it would have become, when all its corners are off one side
  of the screen rectangle: every x < -w/2, every x > w/2, every y < -h/2
  or every y > h/2. w and h are GsInitGraph's width and height (checked
  at 320 by 240 and 640 by 480), in GTE screen coordinates with GsOFSGPU.
  A corner exactly on w/2 or h/2 is inside. GsSetClip does not change
  the rectangle. The last level's pieces (the leaves) are never checked:
  a leaf wholly off screen is still linked.
- Leaves are never dropped for facing away either: zero-area leaves of a
  floor through the camera are kept.

**Contents of a piece.**

- Colors and texture coordinates of midpoints and centers are the
  averages of the parent piece's values, rounded down per channel or
  coordinate: mids (a+b)/2, a quad's center (a+b+c+d)/4. Each level
  works from the level above's rounded values.
- A lit flat primitive is lit once, from its normal; every piece has the
  same color. CBA and TSB are copied.
- The packet code is the same as without subdivision.

Gouraud-lit primitives (dungeon's fogged 3Ch walls): each corner is
lit once from its normal, then the pieces' colors are averaged as for
unlit ones. Only the first color word of a piece carries the code; the
words for corners 1 to 3 have 00 in the top byte on the first sort
after boot. On later sorts (seen 2026-11-03 rerunning the div probe's
cases a second time) Sony's GsDIV1 pieces of a gouraud 39h carry 39h in
the top byte of all four color words, while GsDIV2 pieces keep 00 in
corners 1 to 3. Inference: Sony keeps the points' color words in a work
area, writes a midpoint's RGB into the low 24 bits only, and a point's
word gets the code byte once it has been a piece's first corner, which
level-1 points are only from GsDIV2 on. The GPU ignores the byte; the
clean library writes 00 always, and TestClean's streams are unaffected.

Status: implemented in div.go. All 366 cases of `spec/probes/div` match
Sony's stream word for word; oden passes TestClean and dungeon matches
apart from its own timing readout.

#### Levels above GsDIV2

Probed on Sony's library with `spec/probes/divdeep` (one primitive at a
time, the ordering table walked after the sort and the packets printed
over the TTY; a flat quad or triangle facing the camera at 300 with h
300, so one model unit is a pixel, and its pieces' corners can be
predicted). The attribute's field (bits 9 to 11) is the number of cuts:
level n cuts every polygon n times by the rules above, applied again at
each level, into 4^n pieces. GsDIV3 gives 64, GsDIV4 256 and GsDIV5
1024 (cases 1 to 9); the field's values 6 and 7 are not refused and give
4096 and 16384 (cases 168 and 169).

A model of the DIV1 and DIV2 rules applied recursively (quad pieces
[vi, mid(vi, next), mid(vi, prev), center] in corner order at every
level; triangles cut P0, P1, P2, middle above the last level and their
leaves linked P1, middle, P0, P2) predicts every piece of levels 3, 4
and 5: the corners of the flat quad's and triangle's pieces at level 3 (2Ch,
39h, 20h, 24h), level 4 (2Ch, 39h, 20h) and level 5 (2Ch, 20h), and at
level 3 the uv words of 2Ch and 24h and the color words of 39h. The
script did not compare the lit color words of 2Ch, 24h and 20h. Halving rounds down, as an arithmetic shift: mid =
(a + b) >> 1, center = (a + b + c + d) >> 2. Corners at y -30 give -15,
then -8 (not -7), then -4; with truncation toward zero half the level
3 pieces are off by one (cases 1 to 9, checked by a script).

**Cost.** Every piece takes a packet of the undivided primitive's size
from the work area (2Ch: 40 bytes, 39h: 36, 24h: 32, 20h: 20), 4^n of
them, nothing more (the work base after each sort, cases 10 to 21).
Time, as the emulator counts CPU cycles between GP1 marker words around
one GsSortObject4 of one polygon, Sony's library: 2Ch 501, 1451, 3879,
13591, 67101 and 222493 cycles for levels 0 to 5; 20h 462, 1134, 2593,
8429, 46435 and 139811; 20h at 6 and 7, 513317 and 2051317. Roughly
four times per level from level 2 on (emulator cycles, not hardware).
The reference library (`--libps go`) matches Sony's packets and entries
in every case but the field's value 7, where it stops with an index out
of range.

#### Depth 0 and an entry past the table (with and without subdivision)

- A polygon whose average depth is 0 is kept, in entry 0. With h 4 the
  2 by 2 quad at 3 has OTZ (3·4)/16 = 0 and is linked at entry 0; the
  triangle at 3 and at 4 (OTZ 0 by ZSF3) too; undivided, GsDIV1 and
  GsDIV2 alike, every piece in entry 0 (cases 124 to 147). (The clean
  library dropped a depth of 0.)
- An entry past the table is written where it falls, as the 2D sorts do
  (spec 09, "Depths past the table"): the flat quad at 1024 has OTZ 256;
  into a table of 2 entries at shift 7 (entry 2, the first past), 8
  (entry 1, the last) and 0 (entry 256), the polygon or all its pieces
  hang off the word that many entries past Org, with nothing dropped or
  clamped (cases 55 to 75). (The clean library dropped them.)

#### The near drop under subdivision

Undivided, a polygon is dropped on GTE FLAG bit 31 as before, and in the
emulator that bit comes with the perspective division's overflow (bit
17, h >= 2·sz): the flat quad is dropped at 149 and 150 and kept at 151
with h 300, dropped at 100 and kept at 101 with h 200, dropped at 150
and kept at 151 with h 301 (cases 76 to 79, 148 to 153). The
emulator's mask for bit 31 is bits 30 to 23 and 18 to 13, which includes
bit 17. All of this is the emulator's GTE; no hardware was run. Probe
`spec/probes/divnear` repeats the one case each, in the emulator: Sony's
library drops the undivided flat quad at 149 and at 150 (sz exactly
h >> 1) and keeps it at 151 with h 300, and under GsDIV1 drops it at
149 and keeps it at 150 (cases 0 to 5). The clean library does the same.

Subdivided, the GTE's errors do not drop a polygon (above), but another
check does: before a polygon or a piece is cut, it is dropped, with all
it would become, when every corner's depth sz is below h >> 1. Observed:

- The flat quad at 149 is dropped and at 150 kept, under GsDIV1 and
  GsDIV2, h 300 (cases 92 to 95, 108 to 111); at 99 dropped and 100
  kept with h 200 (158 to 160); at 149 dropped and 150 kept with h 301
  (161 to 163), so the half is h >> 1, not 2·sz < h. The 2 by 2 quad
  at 1 is dropped and at 2 kept with h 4 (164, 165).
- The quad with one, two or three corners at 100 and the rest at 200 is
  kept at the top level (cases 101 to 103): every corner, not any. So
  is the triangle with two corners at 100 (166); the triangle with all
  three at 100 is dropped (167).
- The three-near-corner quad under GsDIV2 keeps 12 of 16 pieces (case
  119): the piece at the three near corners has depths 100, 100, 100
  and 125 (its center's) and is dropped with its four leaves. Only the
  count is observed; which piece goes, and these depths, are inferred
  from the model and were not printed. Under GsDIV1 all four leaves are kept (103):
  the leaves are not checked, as for the screen rectangle.
- The floor past the camera adds nothing new: at -200 it keeps 4 of 4
  under GsDIV1 and 8 of 16 under GsDIV2 (cases 104, 120), and at -300 and beyond
  nothing (105 to 107, 121 to 123). Undivided it is dropped at all four
  distances (88 to 91), which fits the FLAG check. Inference, from depths
  and y values worked out by hand and not printed: under GsDIV2 at -200
  the two near pieces go by the screen rectangle (all their corners at y
  199), and at -300 and beyond the far corners' y pass 120 too, so the
  rectangle drops the whole polygon.
- A corner behind the camera counts by its saturated depth, 0. Probe
  `spec/probes/divnear`: the flat quad at 200 with h 300 and one to four
  corners moved to depth -100 (sz saturated to 0), to depth 0 and to
  depth 1. A moved corner projects to about twice its model coordinates
  ((-80, -60) for corner 0), so the screen rectangle never drops these
  polygons and only the depth check can. The three depths behave alike.
  Undivided the quad is dropped (FLAG bit 31). Under GsDIV1 it is kept
  whole, all four pieces, with one, two or three corners moved and
  dropped with four (cases 10 to 13, 22 to 25, 34 to 37). Under GsDIV2
  two moved corners keep 8 of 16 pieces, three keep 4 and four none (15
  to 17, 27 to 29, 39 to 41); one corner at depth -100 keeps 12, the
  piece at that corner having depths 0, 50, 50 and 125 (case 14), while
  one corner at depth 0 or 1 keeps all 16, that piece's center being at
  150 (cases 26, 38). The triangle with one, two or three corners at
  depth -100 keeps 4, 4 and none under GsDIV1 and 12, 4 and none under
  GsDIV2 (45 to 50). The entry comes from the saturated depths too:
  (0 + 3·200)/16 = 37, 25 and 12 for one, two and three moved corners of
  the quad, 33 and 16 for the triangle.
- The half is h >> 1 even when that is 0: with h 1 the 800 by 600 quad
  at 20 with all four corners at depth -100 is kept, 4 pieces under
  GsDIV1 and 16 under GsDIV2, in entry 0, its corners at (-800, -600) to
  (799, 599); with h 2 (h >> 1 = 1) it is dropped (cases 51 to 62). So a
  corner at or behind the camera is below the half for every h of 2 or
  more and nothing drops it when h is 1.

Whether the library compares sz itself or something equal to it is an
inference; what is observed is the drop at sz < h >> 1 for every corner.
The clean library matches every one of these cases (the divnear probe's
cases 0 to 62, TTY and GPU words).

#### GsOFSGTE

Probe `spec/probes/ofsgte` (the same flat quad moved out of each edge
of the screen under GsDIV2, in the GsOFSGTE mode, in each buffer and
with POSITION at 160, 120, at 0, 0 and at 100, 50, so the GTE's OFX and
OFY were 160/360, 160/120, 0/240, 0/0, 100/290 and 100/50). The pieces
dropped fit the GsOFSGPU rule above at every position tried: the far
half goes at center x 161 and all of it at 201, and at y 121 and 151.
This probe does not run the same sweep in the GsOFSGPU mode; the
comparison is with that rule. Coverage (cases 12 to 552): +x at all three
positions, +y at 160, 120 and 100, 50, and -x and -y only at 160, 120 in
steps of 5, so those two edges are placed within 5 (x between -160 and
-165, half; y between -120 and -125, half). So the rectangle is
w/2 and h/2 around the GTE's offset, not around 0 in the GTE's output,
and POSITION does not move it (inference: the check is on the
coordinates before the offset, or the offset is taken off; the result
is the same). What the sorts put in their packets in this mode is in
spec 09, "The sorts in the GsOFSGTE mode".

#### Lines and sprites in a TMD

GsSortObject4 does not draw a TMD's lines (40h, 50h) or 3D sprites (64h,
6Ch, 74h, 7Ch) and does not get past one. It prints `GPU CODE 40H not
assigned.` once, with the primitive's mode in hex, then `non supported
code 40 800a8df8`, the mode and the primitive's address, over and over,
and never returns: the primitive pointer does not advance. Nothing is
sent to the GPU. The GsDIV field changes nothing: undivided, GsDIV1 and
GsDIV2 behave the same, as do the line with the light-off flag clear and
set and every sprite size. Probe `spec/probes/divnear`, cases 63 on: each
kind at each level in its own disc, since each one hangs. The probe
prints the address of its word `terminal` at the start, and a Sony build
with `--load FILE@ADDR` of a 4-byte file holding the case's index at
that address runs that case alone after the others; the word's default,
FFFFFFFFh, runs all of them, which the clean build does.

The clean library skips these primitives and returns, sorting nothing
(its cases 63 to 86). This is the one difference from Sony's library in
the probe; a sort that never returns is not worth matching.

The reference library prints the two messages once each and returns, so a
program that sorts such a TMD sees them on the TTY but does not stop.

### Not implemented

Lit gradation primitives (flag bit 2). Lines (40h, 50h) and 3D sprites
in TMDs are skipped, where Sony's library hangs (above). Fog is
implemented but not checked against a reference.
