# Milestone 8: the rest of the graphics, matrix and CD calls

Status: implemented. Every probe below gives the same TTY and GPU stream on
the clean library as on Sony's.

The clean library lacks 70 of the 179 calls the `yaroze` package
declares. This milestone covers the graphics, matrix and CD ones, the
calls games use most:

- Matrices and the GTE: ApplyMatrixSV, CompMatrix, RotMatrixX,
  ScaleMatrixL, TransposeMatrix.
- Gs: GsSetClip2D, GsSetDrawBuffClip, GsSetDrawBuffOffset, GsSetOrign,
  GsSortGLine, GsSortOt, GsGetWorkBase.
- Display and system: PutDispEnv, SetDispMask, VSyncCallback,
  FlushCashe.
- CD: CdSearchFile, CdControlB, CdPosToInt.

None of the examples calls them, so the work is probes, not TestClean.

## Sources

- The Library Reference (`docs/manuals/030-1997-libraryref.pdf`) for
  what each call does, and the `yaroze` package's comments
  (`yaroze/libps.txt`, `yaroze/zz_generated*.go`) for the signatures.
- Black-box probes on Sony's library, one per group, written for this
  milestone in `exp/cleanlib/spec/probes/NAME/`. Each probe calls the
  functions over a fixed set of inputs, including edge cases (zero,
  negative, overflow, large angles, rectangles past the screen) and
  prints the results through the TTY. For drawing calls, each case goes
  between GP1 marker words (10000100h plus the case), so the GPU stream
  shows what each call sends (see `spec/probes/clearimage` and
  `.build/gpuprobe`). Build each probe for Sony's library with `go run
  ./cmd/yaroze disc --opt=2 --elf DIR/prog.elf -o DIR/NAME.bin SRC` and
  for the clean one with `exp/cleanlib/build.sh NAME-clean SRC`, and
  compare. The TTY comes from `.build/voiceprobe` (TestDump, OUT.tty),
  the GPU stream from `.build/gpuprobe` (TestGPU).

## What is known already

- CompMatrix and the other matrix calls work in 4.12 fixed point, as
  the existing MulMatrix and ApplyMatrix do (spec 03 and 04). Probe the
  rounding of each: shifts round down, as RotMatrix's do (spec 04,
  "RotMatrix rounding"), unless the probe shows otherwise.
- RotMatrixX turns a matrix about x as RotMatrixY and RotMatrixZ do
  about theirs (spec 05).
- CdSearchFile looks a file up in the disc's directory as CdReadFile
  does (spec 05, CD), and fills in its CdlFILE: position and size.
  CdPosToInt is the reverse of CdIntToPos. CdControlB is CdControl
  waiting for the command's completion (the second response for the
  two-response commands, spec 05).
- PutDispEnv writes the display registers GsSwapDispBuff writes (spec
  05, "Display"). SetDispMask writes GP1 03h.
- VSyncCallback installs a function called at each vertical blank from
  the library's interrupt handler, and with 0 removes it.
- The timing rules of spec 08 apply to the new calls too: queued
  drawing calls do the queue's entry and exit, and drive calls do the
  timeout checks.

## Done when

- Each probe gives the same TTY and GPU stream on the clean library as
  on Sony's, case for case.
- Every rule found is written here, under a heading for its call.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, square, terra, timers.

## Findings (milestone 8)

Probes, each under `spec/probes/`, built for Sony's library and the
clean one as above:

- `matrix`: the five matrix calls over edge cases and a pseudo-random
  sequence (TTY).
- `gsclip`: GsSetOrign, GsSetDrawBuffOffset, GsSetClip2D and
  GsSetDrawBuffClip, with GsSwapDispBuff, GsSetOffset, GsSetClip and
  GsDefDispBuff between, in the GsOFSGPU and then the GsOFSGTE mode
  (the GTE's OFX and OFY read back); then GsSortGLine, GsSortOt,
  GsGetWorkBase and GsSortClear (TTY and GPU stream).
- `ofsgte`: GsSortObject4 and the 2D sorts in the GsOFSGPU and then the
  GsOFSGTE mode, in each buffer and with three origins (TTY, packets in
  drawing order, and the GPU stream around each GsDrawOt); then the
  subdivision sweeps of spec 04 in the GsOFSGTE mode.
- `dispsys`: PutDispEnv over widths, heights, depths, positions and
  screen areas, in NTSC and PAL, with swaps between; SetDispMask;
  VSyncCallback; FlushCache around code changed in memory.
- `dispswap`: when PutDispEnv and GsSwapDispBuff write the mode, each
  test from GsInitGraph, across the four orders of NTSC and PAL.
- `cdsearch`: CdPosToInt and CdIntToPos; CdSearchFile over a disc with
  nested directories, a listing of more than a sector, a directory of 45
  directories, and names that are not there or are written otherwise;
  CdControlB against CdControl over commands with one and two responses
  and refused ones. File positions are printed against where mkdisc put
  each file, since the executables differ in size.
- `cdlimits` (2026-11-03): CdSearchFile over a disc laid out to find
  the tables' limits: 300 directories under one (the path table takes
  two sectors), listings whose first sector holds 40 and 42 records,
  names of 15 to 40 bytes, and how much of a name the match compares;
  and CdControlB over SeekL, SeekP and ReadN to a position past the
  disc's end and into an audio track, against CdControl, with GetlocP
  after each. `mkfiles` writes `files.go` (its comment gives the disc's
  layout) and the audio track, so the disc is built with `--audio
  exp/cleanlib/spec/probes/cdlimits/files/track.raw`. The last cases
  crash Sony's library (see "CdSearchFile"), so they come after
  everything else. Sony's TTY is at `.build/cdlimits-sony/out.tty`.
- `sortclear` (2026-11-03): GsSortClear around GsDefDispBuff and
  GsSwapDispBuff calls, the table printed from its Tag after each step
  (TTY).
- `cdmore` (2026-11-03): CdControl and CdControlB over the commands that
  take a position (SeekL, SeekP, Play, ReadN and ReadS), a refused Setloc
  on its own and before a SeekL, a SeekL past the disc's end, and Play
  with no parameter and with a track number, each call followed by
  GetlocP or Getstat and a Pause where it plays or reads; the kernel
  events the library delivers are counted through two events of class
  F0000003h (20h and 40h) the probe opens with callbacks, and the
  commands each call sends were traced with `.build/iotrace` (TestIO
  with CALLS=CdControl,CdControlB, the cases between GP1 marker words).
  Then CdSearchFile over a listing whose first sector holds 59 records,
  as many as a sector can show: `mkfiles` writes `files.go` and the audio
  track, and the disc is built without the XA area of each record, with
  `tools/mkdisc --no-xa --audio
  exp/cleanlib/spec/probes/cdmore/files/track.raw` over the executable
  (`yaroze disc` has no such flag; the probe's header comment has the
  commands). Each call starts at a vertical blank, so that a short call
  cannot straddle one and the frame counts compare. Sony's TTY is at
  `.build/cdmore-sony/out.tty`.

### The angle tables (RotMatrixX and the others)

The opposite of a negative angle is taken in 32 bits: -80000000h is its
own opposite and its sine 0, its cosine 4096. (The clean library
recursed forever on it.)

### RotMatrixX

m becomes the turn about X times m: for each column j, with s and c the
sine and cosine of r, row 1 becomes (c·m1j − s·m2j) >> 12 and row 2
(s·m1j + c·m2j) >> 12, in 32 bits, cut to 16. Row 0 and the translation
are left alone. It returns m.

### ScaleMatrixL

Row i of the rotation is scaled by v's element i: (m·v) >> 12 in 32
bits (the product wraps), cut to 16 bits. The translation is left alone.
It returns m.

### TransposeMatrix

m1's element (i, j) becomes m0's (j, i), one at a time in row order, so
with m1 the same matrix as m0 the lower triangle is copied over the
upper and the result is symmetric. m1's translation is left alone. It
returns m1.

### ApplyMatrixSV

The GTE's MVMVA of v0 by m's rotation (loaded, and left loaded), each
element shifted right by 12 and saturated to 16 bits. v1's Pad is left
alone; v1 may be v0. It returns v1.

### CompMatrix

m2's rotation is m0's times m1's as MulMatrix0 makes it (saturated to 16
bits). Its translation is, for each row i, the sum over k of m0's
element (i, k) times m1's translation element k cut to 16 bits, shifted
right by 12, plus m0's translation element i, in 32 bits. m2 may be m0
or m1. It returns m2.

### GsSetOrign

Sets POSITION to x and y cut to 16 bits, and nothing else: no packet,
and the GTE's offset is untouched until the next GsSwapDispBuff or
GsSetDrawBuffOffset. GsInitGraph sets POSITION to 0, 0; GsInit3D to the
middle of the screen.

### GsSetDrawBuffOffset

What a swap does for the offset, for the buffer drawn into: in the
GsOFSGPU mode GsDRAWENV's offset becomes the buffer's corner plus
POSITION and the environment is installed (one packet); in the GsOFSGTE
mode the GTE's OFX and OFY become the other buffer's corner plus
POSITION, in 16.16, and nothing is sent.

### GsSetClip2D and GsSetDrawBuffClip

GsSetClip2D keeps a copy of the rectangle and sends nothing. GsInitGraph
sets it to 0, 0, w, h. From then on every buffer drawn into takes it,
relative to its corner: GsSwapDispBuff, GsDefDispBuff and
GsSetDrawBuffClip set GsDRAWENV's clip to the buffer's corner plus its
X and Y, with its W and H, and install the environment (the E3h and E4h
words clamp to video memory as PutDrawEnv's always have). A zero or
negative size is kept as given. GsSetDrawBuffClip sends that one
packet, in both modes.

### GsInitGraph (found on the way)

Its first PutDrawEnv keeps the clip GsDRAWENV had (the previous
GsInitGraph's, or zero at start), with the offset and the rest reset.
In the GsOFSGPU mode, with the GTE on, it sets the GTE's OFX and OFY to
0. It sets GsDISPENV's Pad0 to 1 in PAL (see PutDispEnv).

### GsSetOffset in the GsOFSGTE mode (found on the way)

It sets the GTE's OFX and OFY to the other buffer's corner plus x, y,
and sends nothing; GsDRAWENV is untouched. (In the GsOFSGPU mode it is
as before.)

### The sorts in the GsOFSGTE mode

Probe `spec/probes/ofsgte`, Sony's library, 320 by 240, GsInit3D,
GsSetOrign then GsSetDrawBuffOffset for each origin. With the drawing
environment's offset at 0 (above), the sorts put the offset into the
packets themselves:

- GsSortObject4's vertices carry the GTE's OFX and OFY, as the GTE adds
  them: the flat quad whose corner is (-40, -30) in the GsOFSGPU mode
  comes out at (120, 330) with OFX 160 and OFY 360 (cases 0 and 6). The
  rest of the packet is the same.
- GsSortSprite (with and without GsROTOFF), GsSortFastSprite,
  GsSortBoxFill, GsSortLine and GsSortGLine add OFX >> 16 and OFY >> 16
  to every coordinate word: the sprite at (10, 20) with Mx 3, My 2 is
  (7, 18) in the GsOFSGPU mode and (167, 378) with OFX 160, OFY 360; the
  box's corner, both ends of each line likewise (cases 6 to 11 against
  0 to 5). Every other word is unchanged. With the origin 0, 0 and the
  offset 0, 0 the packets equal the GsOFSGPU mode's (case 11 against 5).
- Which corner: while GsGetActiveBuff returned 0 the offset was buffer
  1's corner (0, 240) plus the origin, and the GsOFSGPU mode's drawing
  environment in the same case clipped to y 240 to 479: both modes draw
  into the buffer at 240 then.
- GsSortClear's fill is the same 02h fill of the buffer's rectangle in
  both modes, and GsDrawOt sends nothing else in the GsOFSGTE mode: the
  DMA mode word, the fill, the list (the stream at each draw marker).
- GsSetOrign then GsSetDrawBuffOffset sends an environment in the
  GsOFSGPU mode and nothing in the GsOFSGTE mode, as above.

The reference library matches in every case: the packets in the TTY, and
the GPU stream from the first marker to the last. Before the first marker
Sony's build writes one more group than the reference's (1:03000001,
0004829c, 1:00000000, 1:10000007) and its TTY has one more ResetGraph
line; the cause was not looked into. Where the screen rectangle
sits for subdivision in this mode is in spec 04, "GsOFSGTE".

### GsSortGLine

When the attribute's bit 31 is clear, a 6-word packet from the packet
area: the link, the E1h mode word GsSortLine sends (the rate, bits 28
and 29, at bit 5, and dithering on), then 50h (52h with bit 30, semi-
transparent) with R0, G0, B0, the first point, R1, G1, B1 with a zero
top byte, the second point. A hidden line takes no packet.

### Depths past the table (GsSortGLine and the other sorts)

No sort clamps its depth to the table: a depth past the table's last
entry writes the word that many entries past Org, wherever that is.
GsSortLine, GsSortBoxFill and GsSortSprite do the same (the clean
library clamped them; the probe sorts each past its table's end).

### GsSortOt

Links all of ot_src into ot_dest at the entry ot_src.Point, which is not
clamped to ot_dest's length (a Point past it writes past Org). The entry
then leads to ot_src.Tag (the Tag as it is, even if the program moved
it); the last element of ot_src's entry 0 chain, the one that led to the
table's end packet (entry 0 itself when nothing is sorted there), leads
to what the entry led to. Links keep their top bytes. It takes no packet
and returns ot_dest.

### GsGetWorkBase

Returns GsOUT_PACKET_P, where the next packet goes.

### GsSortClear (found on the way)

The spec 01 description (an 8-word packet at the work base, linked in
front of Tag) is not what Sony's library does. It takes nothing from the
packet area and leaves Tag alone. Its packet is the library's own, one
for each buffer (by the buffer drawn into): the link and the 02h fill
of GsDRAWENV's clip, 4 words in all. It is put first in the entry Tag
points to, as a sort into that entry: a primitive sorted into that entry
afterwards is drawn before the fill. A second GsSortClear in the same
buffer relinks the same packet (into the same table, the packet links to
itself and the GPU loops; into another, the first table loses what
followed the fill).

Around GsDefDispBuff (`sortclear`): the packet's words are written when
GsSortClear is called, from GsDRAWENV's clip then, and a GsDefDispBuff
after it changes the clip but not the packet. After GsDefDispBuff(0, 0,
0, 240) and a GsSortClear, GsDefDispBuff(0, 240, 0, 0) sets the clip to
(0, 240, 320, 240) and the offset to (0, 240) while the sorted packet
still fills (0, 0), 320 by 240 (case 1); drawn, the fill lands there,
not in the buffer now drawn into (case 7). A GsSortClear after the
GsDefDispBuff rewrites the same buffer's packet with the new clip, (0,
240) in case 2 and (64, 16) after GsDefDispBuff(64, 16, 64, 256) in case
3, and relinks it (into the same table, it then links to itself, as
above). GsDefDispBuff leaves the active buffer alone: with buffer 1
active after a GsSwapDispBuff, GsDefDispBuff(0, 0, 0, 240) keeps it
active, sets the clip to its rectangle, (0, 240), and the GsSortClear
after writes buffer 1's packet (case 8). The packet is the active
buffer's: GsSortClear, GsSwapDispBuff, GsSortClear into the same table
links both packets, the later one first (case 5). The reference gives
the same TTY in every case.

### PutDispEnv and GsSwapDispBuff's display writes

The library keeps a copy of the last DISPENV put, which ResetGraph(0)
forgets. A put of env (PutDispEnv, and GsSwapDispBuff with GsDISPENV
after setting its Disp X and Y to the buffer shown) writes:

1. GP1 05h, the start, always: X in bits 0 to 9, Y in bits 10 to 19
   (both cut to 10 bits).
2. If env equals the copy byte for byte, nothing more.
3. Otherwise env's Pad0 becomes the video mode (1 for PAL, 0 for NTSC;
   GsInitGraph's put sets GsDISPENV's so), then: if Screen differs from
   the copy's, GP1 06h and 07h, and 08h only if anything else differs
   too (Disp, Isinter, Isrgb24, Pad0 as now set, Pad1); if Screen is
   the same, 08h.
4. The copy becomes env.

So a SetVideoMode is only seen through Pad0: a put of the same env,
unchanged since its last put, writes no mode. GsSwapDispBuff then
writes GP1 03000000h (SetDispMask(1)). PutDispEnv leaves GsDISPENV
alone and returns env.

The mode (08h): the width takes 0 up to 280, 1 up to 352, 40h up to 400,
2 up to 560, 3 past that (signed: a negative width is 0). A height past
a field, 256 lines on NTSC and 288 on PAL, adds 24h (interlaced, 480
lines) whatever Isinter says. PAL adds 08h, Isrgb24 (any non-zero) 10h,
Isinter (any non-zero) 20h.

The screen area (06h, 07h), in a frame 256 units by 240 lines, a zero
width being 256 and a zero height 240:

- x0 = 608 + 10·X and x1 = x0 + 10·W, from the raw x0; then x0 is
  kept from 500 to 3290 and x1 from 580 to 3290; then if W is under 8,
  x1 = x0 + 80 (which may pass 3290).
- y0 = 16 + Y on NTSC (19 + Y on PAL) and y1 = y0 + H; then y0 is kept
  from 16 to last − 2 and y1 from 18 to last, last being 258 on NTSC
  and 312 on PAL; then if H is under 2, y1 = y0 + 2.

### SetDispMask

GP1 03000001h (off) for a mask of 0, 03000000h (on) for anything else.

### VSyncCallback

Calls the function once at each vertical blank, from the library's
interrupt handler (none in a critical section), until called with 0.
The binding returns nothing (the Library Reference gives int; not
probed).

### FlushCache

Sony's library exports FlushCache, as the Library Reference spells it;
the `yaroze` binding's FlushCashe, after the header, does not link
against it. The clean library exports both, each the kernel's
FlushCache (A0 44h). The emulator does not model the instruction cache,
so the probe shows only that both libraries run the changed code.

### CdPosToInt and CdIntToPos

CdPosToInt reads each byte as high digit times 10 plus low digit,
valid BCD or not, and returns (minutes·60 + seconds)·75 + sectors −
150; Track is ignored. CdIntToPos converts i + 150 by truncating
division in 32 bits (i + 150 may wrap), each part to BCD as tens·16 +
units cut to a byte (so a negative part is not OR-ed); Track is left
alone; it returns p.

### CdSearchFile

- Directories come from the ISO 9660 path table, read once (through the
  primary volume descriptor) at the first call, into a table of 128
  entries in path table order, the root included. A directory whose
  path table entry is among the first 128 is found, wherever its record
  lies in its parent's listing (45 in one directory, two sectors of it,
  were all found, `cdsearch`); one whose entry is the 129th or later is
  not, nor is anything under it (`cdlimits`, "case dirs": with entry
  128 the 69th of Q's 300 directories, Q/000 to Q/068 are found and
  Q/069 to Q/299 not; an earlier layout of the same disc put entry 128
  at another byte offset with the same result, and the path table's
  first sector held more than 160 entries in both, so the limit is a
  count of entries). Directory names of 16 and 24 bytes are found, by
  their full names only ("case names-dir").
- A file comes from a table of the records of the first sector of its
  directory's listing only; a file in a later sector is not found. The
  table is read again only when the directory differs from the last
  searched. Its entries are CdlFILEs 24 bytes apart ("case stride"), in
  record order, "." and ".." (the records named 00h and 01h) included,
  and each holds the position (Track 0), the length and the name. Every
  record of a first sector is found: 40 records in one listing and 42
  in another ("case files" and "case records": F's _A and G's _C, the
  last records of the first sectors, are found; _B and _D, the first of
  the second sectors, are not), and 59 in a listing written without the
  XA area, where a record is 33 bytes plus its identifier padded even
  (`cdmore`, "case table": R's "." and ".." and its 57 directories of
  one and two characters, 2046 bytes, are found as entries 0 to 58 of
  the table, 24 bytes apart; _K, _L and _M, in the second sector, are
  not, and their files are found through the path table). That is as
  many records as a sector holds with the 37 identifier characters ISO
  9660 allows (mkdisc writes no others), so the table holds at least 59
  entries and no such disc can show whether it is larger. Nor can a
  name's copy be made to run far past the 59th entry: a 59th record
  leaves room for a name of 5 bytes at most, and a 58th for 39, whose
  copy ends at the 59th entry's end; a writer using any byte as a name
  could fit 60 records, or a 59th of 43 bytes whose copy reaches the
  61st entry. The clean library's table of 64 is never filled or
  overrun either way.
- The name is copied into the entry's 16 name bytes as a C string,
  whole, with its NUL, however long it is: a 16-byte identifier fills
  the 16 bytes and the NUL and anything longer run on into the next
  entry, which the next record's copy then overwrites ("case names":
  the entry of D234567890.BIN;1 holds all 16 bytes, that of
  J2345678901234567890.BIN;1 its first 16; "case long": after a listing
  whose last record is the 32-byte Z2345678901234567890123456.BIN;1 the
  entry after it holds that name's bytes 16 to 31 and its NUL, which the
  listing read next, N's, shows in its third record's name as C;1, a
  NUL, IN;1 and a NUL at byte 8). Bytes past the NUL keep what an earlier
  listing's entry left there. Only the entries this listing filled are
  searched.
- The path must start with a backslash; each part up to a backslash is a
  directory, matched exactly (case included) under the one before, the
  root's name being empty (so "\\ROOT.BIN;1" finds the root's file).
  The last part is matched against the table as C's strncmp with 12
  does: byte for byte, case included, stopping at a NUL on either side,
  and no byte past the twelfth is looked at. So a part shorter than 12
  bytes must be the whole name, version included ("\DATA\ONE.BIN" and
  "\DATA\ONE.BIN;2" find nothing for ONE.BIN;1), and a part of 12 bytes
  or more finds any name with the same first 12 bytes: "\L\B23456789.BI",
  "\L\B23456789.BIN", "\L\B23456789.BIx" and "\L\B23456789.BIN;x" all
  find B23456789.BIN;1, "\L\B23456789.Bx" (11 bytes, the last wrong)
  does not, and "\L\H23456789012" finds H2345678901234.BIN;1 while
  "\L\H2345678901" does not ("case compare"). "\DATA\SUB" finds the
  directory's own record, "\DIR\." its "." record, an empty last part
  nothing.
- A path part of 32 bytes or more breaks the library, on the disc or
  not, file part or directory part: a part of 32 bytes made the call
  return the entry before the right one ("case long", the search for
  the 32-byte identifier gives delta -1 and A's name), and a part of 33
  or more ends in an address error (the kernel's a0 40h message; badv
  and a saved register hold the part's last bytes). Inferred: each part
  is copied into a 32-byte buffer of the library's and the bytes past it
  land on a saved register. Up to 31 bytes a part is searched as any
  other (28, 30 and 31 found). The clean library must not break; what it
  returns for such a part is its own.
- Found, it copies the entry (position, size and the 16 name bytes) to
  fp and returns the entry itself, not fp. Not found, it leaves fp alone
  and returns 0; a failed read returns -1.
- The reference (exp/libps) copies the names as Sony's does since
  2026-11-03 (whole, with the NUL, over the next entry; its probe output
  has Sony's name bytes in every case) and differs only on path parts of
  32 bytes and more: it finds the right entry at 32 (Sony's returns the
  one before) and the 33- to 40-byte identifiers (Sony's breaks). Its
  128 directories, 42 records, 24-byte stride and 12-byte comparison are
  Sony's, case for case.

### CdControlB, and CdControl's result

CdControlB is CdControl (with its events) followed by a timed wait for
the second response of a command that answers twice. Both copy the
whole 8-byte response buffer to result, the bytes past the response
zero (the clean CdControl copied only the response). Sony's library
sends Test (19h) without its parameter byte, which the drive refuses
with error 20h; the clean library does the same.

For SeekL, SeekP, ReadN, Play and ReadS the parameter is the CdlLOC to
go to: the library sends Setloc with its three bytes, waits for the
response, then sends the command without parameters (`cdlimits`, "case
control", traced with iotrace: a SeekL is Setloc, SeekL, three
responses; `cdmore`: Play and ReadS the same, through CdControl and
CdControlB alike, and the drive then plays or reads from the position).
Play's parameter is a position whatever it holds: CdControl(Play, {02})
sends Setloc 02:00:00 and Play, not Play with a track byte, and
GetlocP then shows the drive far past the disc's end. With a nil
parameter Play is sent alone (`cdmore`, "play-nil"). A Setloc the
program sent before is superseded: CdControlB(Setloc, 70:00:00) then
CdControlB(SeekL, 00:02:16) leaves the drive at 00:02:16 by GetlocP.

The kernel events each call delivers are in spec 05 ("The library
delivers kernel events"): in short, 20h once the program's command has
fully answered, nothing for the Setloc sent before it, 40h then 20h when
the drive refuses the command or ends it with an error, 40h alone when
it refuses that Setloc, and 40h at each sector the drive delivers after
a ReadN or ReadS the program sent (`cdmore`, every case).

The errors, on the emulator's drive (which answers a SeekL past the
disc's end with 06h 10h and into an audio track with 06h 04h as its
second response, and refuses a Setloc with a byte that is not BCD with
03h 10h):

- A two-response command whose second response is an error: CdControlB
  returns 0 and result holds the error response, 06h then 10h for the
  SeekL to 70:00:00 and 06h then 04h for the SeekL into the audio
  track, the bytes past it zero. The wait ends at that response (the
  call takes under a frame, as a good seek does). GetlocP after it
  shows the position unchanged, and Getstat gives 02h, the drive's
  status with no error bit. The general rule, which the clean library
  follows (2026-11-03): CdControlB's result holds a two-response
  command's second response whether or not it is an error (the Pause
  after a CdControlB(ReadN) in the probe's "control" case answers 22h
  first and 02h second, and result shows 02h).
- Sony's library does not wait on the drive's status busy bit (1800h
  bit 7) before sending a command (spec 08's trace of CdControl shows no
  1800h read before it). The emulator keeps that bit set after a SeekP
  until the next command's first response, so a library that waits on
  it hangs at the Getstat after a far SeekP; the clean library waited
  and does not since 2026-11-03.
- CdControl with the same SeekL returns 1 with the first response (02h)
  in result, before the error, which nothing then reports (Getstat 02h
  and GetlocP unchanged 60 frames on).
- A refused command, the first response an error: the library sends it
  again, four times in all, then returns 0 with the error response (03h
  then 10h) in result. With a position parameter it is the Setloc that
  is refused, and the command itself is never sent: CdControlB(SeekL,
  AAh:00:00) gives 0 with 03h 10h after four Setlocs. CdControl does
  the same: CdControl(Setloc, AAh 00 00) and CdControl(SeekL, AAh:00:00)
  each write Setloc to the drive four times, some 54,000 cycles apart
  (the drive's refusal takes 52,000), and return 0 with 03h 10h
  (`cdmore`, traced; the clean CdControl sent a refused command once
  before 2026-11-03).
- CdControlB(ReadN, pos) returns 1 after ReadN's first response (two
  responses, the Setloc's and its own), position past the end or not:
  a read's errors come with the data, after it has returned (Getstat
  right after gives 22h, reading).
- The emulator does not refuse a SeekP past the end (1, 02h, and
  GetlocP 69:59:50); nothing of that is the library's.

The reference gives the same returns, results and positions in every
case; its disc's audio track starts elsewhere, so the GetlocP bytes of
the audio cases differ by the layout alone. The clean library gives the
same TTY as Sony's on `cdmore`, every line (positions, frames and event
counts), since 2026-11-03.

## Left open

- CdSearchFile's file table is known to hold at least 59 entries, every
  record a sector can show, and nothing on a disc can tell whether it is
  larger or reach past its 59th entry (see "CdSearchFile"); the clean
  library's 64 is its own choice. CdReadFile's use of the tables is
  answered in spec 14 (Sony's goes through them, and the clean one does
  too, ignoring case with the version optional). The tables' sizes, long
  names, the comparison and the two-response errors are answered above
  from `cdlimits`; CdControl's retries, Play and ReadS, and the events
  from `cdmore`; GsSortClear between GsDefDispBuff calls from
  `sortclear`.
