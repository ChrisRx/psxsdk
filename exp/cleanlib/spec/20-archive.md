# The web archive's programs

The programs in the Net Yaroze web archive were built by other people, in C,
against Sony's resident library. `exp/cleanlib/image/archive.sh` boots each one on
Sony's image and on the clean one and compares the pictures. These programs pass
what our examples never pass: null pointers, odd attribute bits, a scale of 0. This
spec is what they found, and `spec/probes/arcprims` shows most of it on Sony's
library.

## Running it

    exp/cleanlib/image/make.sh
    exp/cleanlib/image/archive.sh                # all of them, about 15 minutes
    exp/cleanlib/image/archive.sh NAME...        # some, prints their lines
    FIRST=1 exp/cleanlib/image/archive.sh NAME... # also where they first differ
    exp/cleanlib/image/probe.sh arcprims exp/cleanlib/spec/probes/arcprims

`.build/carchive/summary.txt` has a line a program, with a word: `same`, `shift` (the
same pictures, on other frames), `differ`, `tty`, `stall`, `wild`, `parked` or
`noboot`. `FIRST=1` writes `.build/carchive/first.txt` and `first/NAME.png`, Sony's
picture on the left: the first different picture, and the first GPU word that
differs since boot, with the words around it. Run with `PSX_BIOS` at the retail BIOS,
as the scripts do. Set it for `probe.sh` too.

`first.txt`, `summary.txt`, `first/` and `test.log` are fine to read. The discs in
`.build/carchive` hold Sony's image: see CLEANROOM.md.

## Address 0 is memory

A C program's null pointer is address 0, which on this machine is RAM: the kernel's
first words. Sony's library reads and writes it as it would any address, and a
program goes on. The clean library must not stop with a panic where it does not.
The same memory can be reached at 80000000h, which is not a nil pointer to Go.

- **GsGetLw, GsGetLs, GsGetLws** with a null system (the manual's `WORLD`, which is
  null): the system at address 0 is used, flag and Workm included, with its words as
  they are. It writes there too. `demos-scee-morph` calls `GsGetLs(WORLD, &m)` on its
  first frame.
- **GsClearOt** on a GsOT with Org null, or any Length: Tag becomes Org +
  4·2^Length - 4 and the table is cleared by the same route as ever, at that address.
  With Org null and Length 0, Tag is null and the terminator's address goes in the
  word at address 0. `scee-topten-roller` and `test-padlib-padlib-sample` do this
  every frame.
- **GsSortClear** with a null `ot`, or a table whose Tag is null: the GsOT at address
  0 is used, and the packet is linked at the Tag it holds, wherever that is.
- **LoadImage** with a null pointer sends the words at address 0.

## LoadImage and StoreImage sizes

The size is kept within video memory first: a negative W or H counts as 0, W over
1024 as 1024, H over 512 as 512. With W·H of 0 or less, LoadImage sends nothing at all
(no 01h, A0h or pixels), returning as usual. `scee-topten-race2` sends a 16 by 1
palette from address 0, and `scee-topten-hover` a rectangle of no width.

## GsGetTimInfo

A TIM with no palette (flag bit 3 clear) leaves Cx, Cy, Cw, Ch and Clut exactly as they
were: programs load the "palette" from whatever the fields last held. Only Pmode and
the pixel fields are written.

## GsSortSprite

Whether the sprite is a GPU sprite (64h) or a quad (2Ch):

- Attribute bit 31 set, or W or H of 0: nothing is sorted.
- Bit 27 (GsROTOFF) set: a sprite, whatever Rotate, Scalex, Scaley and the flips say.
- Otherwise a sprite only when Rotate is 0, Scalex and Scaley are both exactly 4096,
  and bits 22 and 23 (the flips) are clear. **A scale of 0 is a quad**, with all its
  corners on one axis or point, not a sprite at full size.

The packet's texture page holds, for both shapes and for GsSortFastSprite alike:
the low 5 bits of Tpage (the page's place), attribute bits 24-25 as the depth, and
attribute bits 28-29 as the rate. **Tpage's own depth and rate bits are ignored.** So
attribute 1<<24 with a Tpage holding depth 2 gives depth 1, and attribute 0 with
Tpage's depth 1 gives depth 0. Depth 3 stays 3.

The quad:

- Bit 23 (GsHFLIP) swaps u0 and u1, bit 22 (GsVFLIP) swaps v0 and v1, in the corners'
  texture coordinates. The first corner, whose word carries the palette, takes the
  flipped u0, v0; the second, carrying the page, takes u1, v0.
- u1 is U + (W&255) - 1 and v1 is V + (H&255) - 1, each kept to 8 bits. So W of 256
  gives u1 = U - 1.
- The palette is Cy<<6 | (Cx>>4)&63.

The corners are the GTE's result: with the rotation matrix for an angle of Rotate/360
(identity at Rotate 0, not computed), Scalex and Scaley applied as ScaleMatrix applies
them, translation (X, Y, the projection distance h), each corner (dx, dy, 0), with
dx from -Mx to W-Mx and dy from -My to H-My, goes through RotTransPers. The GTE's own
rounding, and its screen offset OFX, OFY, decide the results, so hand calculation
drifts by a pixel in places. Compare `rotate -30` and `rotate depth` in the probe.

GsSortFastSprite ignores Rotate, Scale, Mx, My and the flips, and is always a sprite
(with W or H of 0 or bit 31, nothing). Its E1h is as above.

## GsSortClear

The packet is 02h at (PSDOFSX, PSDOFSY) of the buffer being drawn, sized by the screen
as GsInitGraph was given it (W, H), not by GsDRAWENV's clip. With Isrgb24 set in
GsDISPENV the width is W·3/2.

## PutDrawEnv's fill

With Isbg set, the fill follows ClearImage's rule: W is kept to 0-1023 and H to 0-511,
and it is 02h when X%64 and the kept W%64 are both 0, otherwise 60h. `demos-scee-chap2-3`
has a clip of height 512 and so a fill of 511.

## The GTE without GsInit3D

The GTE is on before main, and after GsInitGraph H is 1000 and the screen offset is
live, even in a program that never calls GsInit3D. A turned sprite or a scale of 0 in
a 2D program relies on this. A quad sprite leaves its rotation and translation in the
GTE, with no restore, including one that is a quad only because of a flip.

## GsSortObject4's attribute

Besides bit 31 (not drawn), GsDIV and the light bits:

- **Bit 30 makes every polygon translucent**: it sets bit 1 of each packet's command
  code, textured or not, gouraud or not (`test-dyddem01` has 28h become 2Ah, `test-l-ctb11`
  38h become 3Ah). It is a set, not a sum: a polygon the TMD already marks translucent
  keeps its code. For a gouraud polygon only the first color word carries the code.
- The light mode: bit 6 set is unlit. Otherwise with bit 5 set the object's own mode
  is bits 3-4, and with bit 5 clear GsSetLightMode's. A mode of 1 is fog, any other is
  normal: bits 3-4 of 3 is not fog.

## Fixed backgrounds

The clean library's model of these was wrong. The window fields of the GsBG are not
used: **X, Y, W, H, R, G, B, Mx, My, Scalex, Scaley and Rotate have no effect.**
Everything is sized and placed by the screen, GsInitGraph's W and H.

With cols = W/16 + 1 and rows = H/16 + 1 taken from the screen:

**GsInitFixBg16(bg, work)** builds two packet lists in work, one for each buffer.
Work's first two words hold the address of list 0 and of list 1, and a list is:

| words | what |
|---|---|
| 2 | link and E5h |
| 5 × cols × rows | a cell, row by row: link, E1h, command, position, texture |
| 2 | link and E5h, the tail |

So list 0 is at work + 8 bytes and list 1 at work + 24 + 20·cols·rows. The cells' tags say
4 words. The command is 7Dh, or 7Fh if bg's attribute has bit 30 at this call, and is
fixed from here on. Its low 24 bits are 0. A cell's position is (16·col, 16·row + i·H)
for list i: the screen's height, not GsDefDispBuff's corners, whatever they are.

**GsSortFixBg16(bg, work, ot, pri)**, with attribute bit 31 clear, fills in a list and
links it:

- The list used is list 1 when PSDIDX is 0, list 0 when it is 1 (in the C program's
  sense: the one the buffer after the coming swap will show). The probe shows y of
  0F0h on the first sort after GsInitGraph.
- With the scroll sx, sy, the head's E5h is (-(sx&15), -(sy&15)), 11 bits each.
- The first column is ((sx&~15)/16) mod Ncellw and the first row likewise, both
  wrapped to be positive, and a row or column stepping past the map's end goes back to
  0.
- A cell of FFFFh sets its tag's size to 0 and leaves its other words alone; the GPU
  skips it. Any other sets the tag's size to 4, its E1h's low 16 bits to (cell's Tpage
  & 1Fh) | attribute bits 24-25 as depth | attribute bits 28-29 as rate, and its
  texture word to the cell's first 4 bytes (U, V, Cba). A cell's Flag is not used.
- The tail's E5h is 0 when GsOFSGTE is in use. With GsOFSGPU it is (POSITION.Offx +
  PSDOFSX[j], POSITION.Offy + PSDOFSY[j]), with j = (PSDIDX + 1) & 1.
- The list's tail links to what ot's entry at (pri + Offset) held, with a size of 1, and
  that entry is set to the list's head with a size of 0. Note the plus: GsSortSprite's
  priority goes the other way.

## Numbers on screen

Many programs print their own frame time on the screen, from VSync(1) or a root
counter: `demos-scee-balls` shows 3000 on Sony's library and 3114 on the clean one. Such
a program can only be `same` when the library's speed matches call for call. That is
the job of spec 19's cost tables, and is not asked for here.
