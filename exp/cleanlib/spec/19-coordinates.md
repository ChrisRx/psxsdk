# Milestone 19: coordinate systems and stack use

Team Fatal's original Terra Incognita keeps its stack in the 1 KB scratchpad. On Sony's
image it runs 4,500 frames with a lowest sp of 1F800284h. On the clean image it runs off
the bottom of the scratchpad at frame 321, in GsGetLw's walk of a coordinate chain. The
clean GsGetLw, GsGetLs and GsGetLws take stack for every level and ignore Flg. This spec
gives what they do on Sony's library, what the rest of a frame's calls take in stack, and
what that means for the clean ones. All of it is from calling Sony's entries in the
emulator.

Each probe is in `spec/probes/NAME` and runs with
`PROBE_FRAMES=2000 exp/cleanlib/image/probe.sh NAME exp/cleanlib/spec/probes/NAME`.
`SIDES=sony` runs Sony's side alone. Its output is `.build/NAME-cimg/out.tty`.

## The rule in short

Flg is compared with the frame counter PSDCNT, written C below. It is not tested for
being set.

- 0 means the program changed Coord: work this system out.
- C means it was worked out in this frame. Its Workm is right and its children have to be
  worked out, since it may have changed.
- Any other value means it was worked out in an earlier frame and nothing has changed
  since. Its Workm is right and its children can keep theirs.

A call on a system whose flag is C only hands back Workm. Everything else depends on the
chain above it, as given below.

The results in the first version of `gschain` came from its own set-up, not from the rule
being odd. It never calls GsInitGraph, so PSDCNT is 0, which makes a flag of 0 current,
and GsIDMATRIX is all zeros, so each system has a zero rotation. Its stack lines for a flag
of 0 are the case where nothing is worked out, as is its 168 for GsSetView2 on a chain. Use
`gsstack`'s figures instead.

## The counter

`gscnt`, `gsinit`.

| call | PSDCNT | PSDIDX |
|---|---|---|
| start of the program, ResetGraph | 0 | 0 |
| GsInitGraph, every call | 1 | 0 |
| GsSwapDispBuff | adds 1, FFFFFFFFh goes to 0 and 0 to 1 | swaps 0 and 1 |
| GsInit3D, GsDefDispBuff, ResetGraph, VSync, DrawSync, GsSetDrawBuffOffset | no change | no change |

The comparison takes all 32 bits and the sign does not matter: flags of 80000000h,
FFFFFFFFh and C+10000h behave as any other value (`gsflag`). A program may write PSDCNT,
and the calls then compare with what it wrote. At C = 0 a flag of 0 is current, and the
rule below holds with that: nothing treats 0 first.

## WORLD and GsIDMATRIX

- WORLD is a null Super. A system with a null Super is a root.
- GsInitGraph sets GsIDMATRIX to the identity, rotation 4096 on the diagonal, translation
  0. Before it, GsIDMATRIX is all zeros. ResetGraph does not set it (`gsinit`).

## GsInitCoordinate2

`gsinit`. `GsInitCoordinate2(super, base)`:

- base.Coord becomes a copy of GsIDMATRIX, all 32 bytes, with the padding halfword and
  the translation. A program that changes GsIDMATRIX gets its own matrix here. Before
  GsInitGraph it gets zeros.
- base.Flg becomes 0. base.Super becomes super.
- When super is not null, super.Sub becomes base. A later child takes Sub over, and the
  earlier child is not touched.
- base.Workm, base.Param and base.Sub keep what they held.
- No GTE register changes, and it costs 58 cycles with a null super and 70 with a parent.

The calls below never read or write Sub or Param: a chain whose Subs point at the wrong
systems comes out right and keeps them (`gsinit`).

## GsGetLw, GsGetLs, GsGetLws

Let s be the system asked for, C be PSDCNT, and "root" be a system whose Super is null.
The calls act as if these steps were followed:

1. If s is not a root and s.Flg is C, nothing is written, and the result is s.Workm.
2. Otherwise, going up from s through Super, the top t is the first system above s whose
   Flg is C, or the root, whichever comes first. (s itself is the top when it is a root.)
3. At t:
   - t is not a root (so its flag is C): t is not written. The systems below it are
     worked out.
   - t is a root with Flg 0 or C: t.Workm becomes a copy of t.Coord, 32 bytes, and t.Flg
     becomes C. The systems below it are worked out.
   - t is a root with any other flag: t.Workm and t.Flg stay. The systems below it are
     worked out only from the first one with Flg 0.
4. Going down from below t to s, each system n in turn: when it is to be worked out,
   n.Workm becomes the parent's Workm composed with n.Coord (below), and n.Flg becomes C.
   Otherwise n stays as it was. A flag of 0 starts the working out here if it has not
   started, and it goes on to s.
5. The result is s.Workm, whatever it holds: a planted one comes back unchanged. GsGetLw
   copies it to its output. GsGetLs does not write a local-to-world output. GsGetLws writes
   both.

The screen matrix comes after all this, as GsWSMATRIX composed with s.Workm. It is not kept
anywhere, and it does not touch flags.

All three calls do the same to flags and Workm, on all 27 patterns of a chain of three
(`gsflag`'s GsGetLws and GsGetLs parts against its depth 3 lines). `gsflag` covers chains
of 1 to 4. A model of the steps above gives every one of its 282 lines.

For a chain of one, s is a root: flag 0 and flag C both make Workm a copy of Coord with flag
C, and any other flag keeps Workm and hands it back. So a root's Workm is written, and read
when its flag is an old frame's.

For a chain of three, flags given root first, x being a value other than 0 and C, and "any"
meaning all three:

| flags | what happens | flags after |
|---|---|---|
| any, any, C | nothing | the same |
| any, C, 0 or x | node 2 from node 1's Workm | any, C, C |
| x, 0, 0 or x | node 1 from the root's Workm, and node 2 | x, C, C |
| 0 or C, 0, 0 or x | root from Coord, nodes 1 and 2 | C, C, C |
| 0 or C, x, 0 or x | root from Coord, nodes 1 and 2 | C, C, C |
| x, x, 0 | node 2 from node 1's Workm | x, x, C |
| x, x, x | nothing, the result is node 2's Workm | x, x, x |

What a program sees (`gsinit`, its last lines):

- Clearing a parent's Flg after a child was worked out in this frame does not reach the
  child in this frame: the child's flag C hands back its old Workm. The next frame's call on
  the child, whose flag is then an old value, sees the parent's 0 and works both out.
- From the second frame on, a chain nobody touched costs one walk and no multiplication.

## Composing

`gscompose`, `gstrap`. Composing a with b, a being the parent's Workm or GsWSMATRIX and b
being Coord or Workm:

- **Rotation.** a.M times b.M exactly as MulMatrix0 makes it, saturating to 16 bits: 500
  random pairs, up to full-range elements.
- **Translation.** For each row i, in 32-bit arithmetic that wraps:

      lo = sum over k of a.M[i][k] * (b.T[k] & 7FFFh), in 64 bits
      hi = sum over k of a.M[i][k] * int16(b.T[k] >> 15), wrapping to 32 bits
      T[i] = int32(lo >> 12) + (hi << 3) + a.T[i]

  It gives every line of 800 random pairs with full-range matrices and translations. For a
  translation under 2^30 with hi not wrapping, it is the floor of the exact sum over 4096
  plus a.T[i]. Bit 31 of b.T plays no part. This is not CompMatrix, which cuts b.T to 16
  bits (spec 09).
- **Overflow.** None of these sums stops the machine. 7FFFFFFFh + 1 wraps to 80000000h in
  GsGetLw, GsGetLs and GsSetView2 (`gstrap` cases 0, 1, 2, 4, 5, 6, 7).
- **CompMatrix does stop.** 7FFFFFFFh + 1 in its last add takes the overflow exception,
  cause 30h, and the kernel halts (`gstrap` case 3). Spec 15 says such Sony-only crashes are
  not matched.
- **Padding.** A composed Workm and the copy GsGetLw gives have 0 in the padding halfword
  (planted values of 21 to 25 in each input did not appear). A root's Workm has Coord's.

## GsSetView2, GsSetRefView2

`gsview`.

GsSetView2 with a null Super copies View to GsWSMATRIX, and touches nothing else.

With a Super, it works out Super by the steps above: flags and Workm come out as for GsGetLw
on all 27 patterns of a chain of three. A root flagged x hands back its kept Workm: -9 from
a planted 9, against -5 with flag 0 from a Coord of 5. Then, with W being Super's Workm:

    inverse.M = W.M transposed
    inverse.T = -(W.M transposed, times W.T, each row floored)
    GsWSMATRIX.M = MulMatrix0(View.M, inverse.M)
    GsWSMATRIX.T = View.T + (View.M times inverse.T, as in Composing)

The negation is of the floored value: negating W.T first gives the wrong result on 79 of
80 random cases, against none. Tested with translations to 1,000,000 only.

GsSetRefView2 works its matrix out for WORLD (spec 04) and goes on as GsSetView2 with that
matrix and its Super: 40 of 40 random views came out the same as the two calls made apart.
A null Super gives GsSetView2's copy. It returns 0, with two cases that spec 04 lacks:

- **The two points the same.** It returns 1 and does not touch the chain. GsWSMATRIX
  becomes GsIDMATRIX2's rotation with the twist applied as usual, and GsIDMATRIX2's
  translation. It does not work out a view: the viewpoint does not show in the translation.
- **Straight up or down** (dx and dz both 0, the points apart). It is a view like any other
  and returns 0, not an error. cy is 4096, sy is 0, cx is 0, and sx is dy·4096/l, so
  looking from (1, 2, 3) to (1, 200, 3) on an identity GsIDMATRIX2, with l from spec 04's
  square root (197 here), makes rows
  (4096, 0, 0), (0, 0, -4116), (0, 4116, 0) and translation (-1, 3, -3) before the chain.

Neither changes PSDCNT, and neither writes into the chain other than through the steps above.

GsSortObject4 reads nothing of the system either: with Flg 0 and a planted Workm, both are
the same after it (`gsstack`).

## GTE registers

`gsregs`, which fills the registers with a pattern, makes the call and lists the changes.
d is a data register and c a control one.

| call | changes |
|---|---|
| GsInitCoordinate2, GsSetView2 with null Super | none |
| GsGetLw when nothing composes: a root, s flag C, all x | none |
| GsGetLw with a composition | see below |
| GsGetLs, GsGetLws, always | see below |
| GsSetView2 with a Super, on any flags | see below |

After a composition of a with b:

- R (c0 to c4) holds a's rotation.
- V0 (d0, d1) holds b's third rotation column.
- IR1 to IR3 (d9 to d11) hold the result's third column, saturated as MulMatrix0's is, and
  MAC1 to MAC3 (d25 to d27) the same unsaturated. IRGB and ORGB (d28, d29) follow IR.
- FLAG (c31) is as that multiply leaves it: 81C00000h on a saturated one, cleared on the
  plain one after it.
- TR (c5 to c7) and everything else stay.

So the last thing composed decides. GsGetLs and GsGetLws end on GsWSMATRIX with Workm, even
when nothing else was worked out. GsSetView2 with a Super ends on View with the inverse.
GsSetRefView2 changes the same and LZCS and LZCR (d30, d31), from its square root. A program
calling GsGetLw with a current flag keeps the rotation it had loaded, and the clean library
must keep it too.

## Long chains and cycles

`gslong`, `gscycle`.

- Every depth to 647 comes out right: at 600, lw is 600 and all flags are C.
- 648 is wrong in this build: nodes 13 and 14, 634 and 633 from the target, keep their old
  flags, and the result is garbage (680701850). Nothing need match this. No program is known
  to come near it. An earlier form of the probe went on through depths of 1,000, 2,000 and
  4,000: the node 634 from the target went wrong each time, and the program did not reach
  its end.
- The stack does not grow with depth (below). The cost is linear: see Cost.
- A vertical blank callback that checks every Super and Sub saw an unbroken chain on all 28
  blanks while 30 calls ran at depth 600. So a link is never changed even for a time.

A cycle with no flag C in it does not return. A system that is its own parent with flag 0
ends in a reserved-instruction exception inside the program's own code, so it writes over
memory it does not own. Two systems in a cycle with flags 0 or x printed nothing more in
500 frames. A cycle that the first step or a flag C leaves does return: a system its own
parent with flag C gives its Workm, and two with the parent at flag C gives the child from
the parent's Workm. None of this needs matching.

## Stack

Measured as in `gschain`: the stack below the probe's sp is filled with a pattern, the call
runs inside a closure, and the figure is how far down the pattern changed, from that sp.
The highest of two runs. The figure includes the probe's frame of 24 bytes for the closure,
and its scan stops at sp-8, so 8 means the call itself wrote nothing, and a direct call of
GsGetLw at depth 16 shows 64 where this table shows 88. The method is the same on both sides.
A figure does not see a frame that is reserved and never written.

A spin of over a frame with nothing called shows 8, with nothing playing, with a sequence
playing and with keys on, on both images. So interrupts do not use the program's stack.

Figures from `gsstack` and `ssstack`, Sony's against the clean image as of this spec. **Bold**
is where the clean library takes more.

| call | Sony's | clean |
|---|---|---|
| GsGetLw, nothing composed, any depth | 40 | **76 to 1,792** |
| GsGetLw, composing, any depth | 88 | **224 to 1,792** |
| GsGetLs, any depth, any flags | 88 | **176 to 1,856** |
| GsGetLws, any depth, any flags | 96 | **184 to 1,864** |
| GsInitCoordinate2 | 8 | 8 |
| GsSetView2, null Super | 40 | **64** |
| GsSetView2, depth 1 | 168 | **216** |
| GsSetView2, depth 2, 4 and 16 | 200 | **328, 552, 1,896** |
| GsSetRefView2, null Super | 288 | 240 |
| GsSetRefView2, depth 1, 2, 4, 16 | 288 | **392, 504, 728, 2,072** |
| GsSetLsMatrix | 32 | 32 |
| GsSetLightMatrix | 72 | 68 |
| GsSetFlatLight | 208 | 64 |
| GsSetAmbient | 32 | 8 |
| GsSetLightMode | 32 | 8 |
| RotMatrix | 8 | **40** |
| TransMatrix | 8 | 8 |
| ScaleMatrix | 8 | 8 |
| MulMatrix0 | 8 | **36** |
| CompMatrix | 8 | **124** |
| ApplyMatrix | 8 | **40** |
| ApplyMatrixLV | 8 | **36** |
| GsSortObject4, no division | 88 | **112** |
| GsSortObject4, GsDIV1 flat or gouraud quad | 216 | 216, 204 |
| GsSortObject4, GsDIV1 triangle | 200 to 224 | 196 to 204 |
| GsSortObject4, GsDIV2, quad | 216 | **304**, 284 |
| GsSortObject4, GsDIV2, triangle | 200 to 224 | **268 to 284** |
| GsSortObject4, GsDIV3 | 200 to 224 | **340 to 392** |
| GsSortObject4, GsDIV4 | 200 to 224 | **412 to 480** |
| GsSortObject4, GsDIV5 | 200 to 224 | **484 to 568** |
| GsSortSprite, turned or GsROTOFF | 224 | 96 |
| GsSortFastSprite | 8 | **28** |
| GsSortBoxFill | 72 | 8 |
| GsSortLine | 72 | 8 |
| GsClearOt | 152 | 28 |
| GsDrawOt, idle or behind a draw | 152 | 108 |
| GsSwapDispBuff | 192 | 132 |
| VSync(0) | 72 | 52 |
| VSync(1), VSync(-1) | 40 | **52** |
| DrawSync(0) | 112 | 56 |
| GetPadBuf (the Go PadRead's library call) | 12 | **80** |
| SsSeqPlay | 56 | 40 |
| SsSeqStop | 56 | 32 |
| SsSeqClose | 56 | 40 |
| SsSetMVol | 72 | 36 |
| SsUtKeyOn | 104 | **112** |
| SsUtAllKeyOff, keys on or not | 8 | **52** |
| FntPrint, text, `%d %x` and `%20d` | 64, 68, 68 | 64, 64, 64 |
| FntFlush | 232 | 140 |

PadRead is the yaroze package's Go function, reading the buffers GetPadBuf found, and not
a library entry.

### GsSortObject4's scratch argument

Sony's library uses the memory its fourth argument points at, and the clean one does not,
which is why its stack grows instead. Bytes from the argument up to the last one changed
(`gsstack`), the same with fog, on a floor running past the camera and on an upright quad:

| kind | no division | GsDIV1 | GsDIV2 | GsDIV3 | GsDIV4 | GsDIV5 |
|---|---|---|---|---|---|---|
| quad, gouraud or flat textured | 0 | 256 | 400 | 540 | 680 | 820 |
| triangle, flat, flat textured or gouraud | 0 | 180 | 272 | 360 | 448 | 536 |

A program with both its stack and this work space in the scratchpad has fixed both figures
in advance. The clean library must write no more of the work space than these, and take no
more stack than the table above at any division.

### Rules for the clean library

- GsGetLw, GsGetLs, GsGetLws, GsSetView2 and GsSetRefView2 at or under Sony's figures, at any
  depth to 647. No recursion, and no table on the stack.
- Super and Sub seen from an interrupt stay as they are.
- GsSortObject4 at or under 224 at any division, with the work space above.
- Every other bold line at or under Sony's figure where that is cheap to do. The ones an
  original program calls deep in a frame come first: CompMatrix, MulMatrix0, RotMatrix,
  ApplyMatrix, ApplyMatrixLV, GsSortFastSprite, SsUtKeyOn, SsUtAllKeyOff.

## Cost

`gscost`: root counter 2 in system clock mode, the minimum of six runs with an empty call of
2 taken off, with interrupts off. The matrices have a real rotation. Clean's figures are
from the clean image as of this spec, which works everything out every time.

| case | Sony's | clean |
|---|---|---|
| GsGetLw, depth 1, flag 0 | 152 | 220 |
| GsGetLw, depth 1, flag C | 146 | 218 |
| GsGetLw, depth 1, other flag | 124 | 218 |
| GsGetLw, all 0, depth 2, 3, 4, 8, 16, 64 | 560, 962, 1,364, 2,972, 6,188, 25,484 | 798, 1,376, 1,954, 4,266, 8,890, 36,634 |
| GsGetLw, target flag C, depth 2 and up | 110 | 796 and up |
| GsGetLw, all other flag, depth 2, 3, 4, 8, 16, 64 | 158, 192, 226, 362, 634, 2,266 | as all 0 |
| GsGetLw, target 0 under a parent at C, depth 3 and up | 518 | as all 0 |
| GsGetLw, target 0 under a parent at C, depth 2 (root) | 554 | 796 |
| GsGetLw, target 0, parents other flag, depth 2, 3, 4, 8, 16 | 534, 568, 602, 738, 1,010 | as all 0 |
| GsGetLs, flag C, depth 2 and up | 432 | as GsGetLw plus about 450 |
| GsGetLws, flag C, depth 2 and up | 468 | as GsGetLw plus about 500 |
| CompMatrix | 186 | 530 |
| GsSetView2, null Super | 106 | 118 |
| GsSetView2, flags 0, depth 1, 2, 4, 8 | 816, 1,214, 2,028, 3,636 | 862, 1,440, 2,596, 4,908 |
| GsSetRefView2, null Super | 1,416 | 4,766 |
| GsSetRefView2, flags 0, depth 1, 2, 4, 8 | 2,126, 2,524, 3,338, 4,946 | 5,510, 6,088, 7,244, 9,556 |

So on Sony's a level worked out is about 402 cycles, a level walked and left is 34, and a
GsGetLs or GsGetLws adds about 322 and 358 to the GsGetLw it holds. Pace to these where the
shape of the loop gives them cheaply, as elsewhere in this library.

## Corrections to other documents

- **yaroze/libps.txt, `_GsCOORDINATE2`, `.flg`, GsGetLw.** "Flg is nonzero while Workm is up to
  date" is wrong as a test: 0 means work it out, PSDCNT means worked out this frame, and any
  other value means still good from an earlier frame. "A parent that has changed brings its
  children with it" holds only within the steps above. A child flagged C this frame does not
  follow a parent cleared afterwards.
- **yaroze/libps.txt, `.workm`.** "Except for a system whose parent is the world" is wrong:
  a root's Workm is written from Coord and handed back.
- **yaroze/libps.txt, GsInitCoordinate2.** "Sets a coordinate system to the identity" holds
  only because GsIDMATRIX is the identity: it copies that variable, and before GsInitGraph
  gets zeros.
- **Spec 03, "Coordinate systems".** "The result must be the same either way" is wrong: a stale
  Workm comes back as it is.
- **Spec 04, "Coordinate helpers" and GsSetRefView2.** GsSetRefView2 returns 1 only for the
  two points the same, with GsWSMATRIX as above. A view straight up or down returns 0 with a
  real matrix. The twist is applied on these paths too.
- **Spec 09, CompMatrix.** "In 32 bits" holds except that the last add takes an overflow
  exception. The chain's own composition is not CompMatrix: it does not cut translations to
  16 bits.
- **The clean library as it stands**, for the implementer: localToWorld sets Flg to 1 and
  reads no flags, composition has no 31-bit cut, GsInitCoordinate2 does not read GsIDMATRIX,
  and GsSetRefView2 returns 1 straight up and down and leaves GsWSMATRIX for two equal points.

## Open

- A call whose output is the system's own Workm, and GsGetLws with the two outputs the same.
- GsGetLw called from an interrupt during another GsGetLw.
- The padding of a composed matrix being 0 in every case: seen on one chain.
- GsSetView2's inverse with translations over 1,000,000, where the 31-bit cut should apply
  to both steps.
- What GsSetRefView2 leaves in V0 and IR: the listing in `gsregs` gives one case on each side,
  and they differ in those registers. No program is known to read them, so it is not asked
  for here.
- GsSortObject4's stack and work space on lit gouraud polygons, on lines and sprites in a TMD,
  and on an object of many primitives.
- Stack of SsSeqPlay with other sequences, and of the Ss calls not listed.
- Whether 648 is the first failing depth in a program laid out differently. It is not worth
  more.

## Acceptance

By `probe.sh`, on both images, reporting each difference left.

1. `gsflag`: the TTY identical.
2. `gscnt` and `gsinit`: the TTY identical, on these calls' lines.
3. `gscompose`: the TTY identical. `gstrap` with `which` from 0 to 8, set in its source: on
   the clean image every case prints "no stop", and each line of cases 0 to 2 and 4 to 8 is
   Sony's. Case 3 is the crash that is not matched.
4. `gsview`: the TTY identical. Random lines, `same points`, `view` and `straight up` lines
   included.
5. `gsregs`: the same registers listed as Sony's on every GsGetLw, GsGetLs, GsGetLws,
   GsInitCoordinate2 and GsSetView2 line.
6. `gslong`: the same up to and including depth 600, and the callback's line "with a broken
   chain 0". The stack lines at or under Sony's 64.
7. `gsstack` and `ssstack`: each figure in "Rules for the clean library" at or under Sony's,
   the GsSortObject4 lines' scratch figures at or under Sony's, with report of any other
   bold line that stays over.
8. `gscost`: report the differences. None of it fails a run yet.
9. Terra Incognita's original: `exp/cleanlib/image/terra.sh` builds it on both images and
   compares them for 4,500 frames, with the stack's lowest point on each. Its stack stays
   inside the scratchpad on the clean image, and report every second whose picture differs.
10. `../regress.sh` stays at its baseline and `image/regress.sh` gets no worse. The examples
    that use GsGetLw, GsGetLs and GsGetLws are where a wrong flag rule would show.

## What the first runs showed

Measured on the clean image with `probe.sh`, on the sources as they stand. New probes:
`gsrz`, `gspad`, `gspsd` and `crand`.

### Corrections to this spec and others

- **The twist, spec 04.** The angle in 4096ths of a turn is `(-Rz)/360`, rounded toward zero,
  with the negation wrapping: -80000000h comes out as 7FFFFFFFh does. So Rz is in 4096ths of a
  degree (`gsrz`).
- **The twist's sine and cosine.** They are the table's own in every quarter, which is not
  RotMatrix's `sin` and `cos`: those read the second and fourth quarters one step early.
  cos(512) is 2896 here and 2892 there.
- **GsSetRefView2's order, spec 04.** The rotation is built left to right, each step rounded down:
  GsIDMATRIX2 times the twist, times the turn about x, times the turn about y. All 200 of `gsrz`'s
  random views with a twist fit that, and all 100 without one on a changed GsIDMATRIX2. With the
  twist times (x times y), as spec 04's rows would have it, 21 of the 200 fit. An identity
  GsIDMATRIX2 and no twist hide the difference.
- **Padding.** "The copy GsGetLw gives has 0" holds only for a composed Workm: GsGetLw copies all
  32 bytes, so it gives a root's 21 and a kept Workm's 24. GsGetLs's output, a GsGetLws's ls and
  GsWSMATRIX from a chain have 0. GsSetView2 with a null Super copies View's padding. MulMatrix0
  and CompMatrix also write 0 there, on any output (`gspad`).
- **CompMatrix's registers.** Its translation is on the GTE, after the rotation: V0 holds m1.T cut
  to 16 bits, and IR and MAC that multiply's result (`gsregs`). The clean one now does this.
- **PSDIDX, PSDOFSX, PSDOFSY.** These are the library's state, not a copy of it. PSDIDX is the
  buffer drawn into, and GsGetActiveBuff and the next GsSwapDispBuff go by what a program wrote
  there. PSDOFSX and PSDOFSY each hold two halfwords, buffer 0's and buffer 1's x or y, and the next
  swap draws with what a program wrote. All are 0 after GsInitGraph, so before GsDefDispBuff both
  buffers are at (0, 0) (`gspsd`). On the image these three symbols did not exist before, since
  nothing read them: the compiler dropped the stores, and programs read 0.
- **rand and srand, spec 02.** Sony's library does not call the kernel's A0h 2Fh and 30h, and `crand`
  and Terra show no such call. Its rand is next = next * 1103515245 + 12345, returning bits 16 to
  30, from a state of 0, and srand sets the state as given. OpenBIOS's sequence is the same, so
  every probe and regression run missed this. A retail kernel's differs, and that was Terra's
  scenery: its 72 rands at load scale and turn the objects.

### Open questions answered

An output that is the system's own Workm comes out as if there were no overlap, and so does
GsGetLws with its two outputs the same or either one the Workm: the clean library's lines match. An
output that is the system's own Coord does not match: Sony's Workm comes out as the parent's Workm
composed with itself, as if Coord had taken the parent's Workm first. Nothing is known to
do this, and it is not matched. Those are the only lines of `gspad` that differ.

### How the chain is walked

No table and no recursion. The walk up finds the highest system to work out. Each system below that
is about to get a new Workm, so until then its Workm holds the address of the system under it. The
walk has no depth limit: `gslong` reaches its END with no failing depth, where Sony's fails at 648.
Workm holds an address for a moment during a call, so GsGetLw from an interrupt on a chain another
GsGetLw is working out could go wrong. That case stays open, on both sides.

### Terra Incognita

The stack stays inside the scratchpad for all 4,500 frames, and the program's own lowest sp is
Sony's. Before spec 01's VSync rule the pictures were the same but landed earlier: with no input,
all 1,970 picture changes in 4,500 frames came in the same order, the clean side 1 frame early from
frame 295 and 2 frames early from 824, so 19 of TestTerra's 150 pictures matched.

Both slips were one rule, in VSync, and not the loads before it. Each came at the program's first
VSync(2) after a long stretch of work: the first call, and call 287. With n blanks already past,
Sony's waited for the next blank (0.39 and 0.70 of a frame) and the clean one returned at once, so
the clean side ran ahead of the blank and later calls rounded the lead up to whole frames. The work
itself was not shorter: from call 286 to 287 it is 0.38 of a frame longer on the clean side. Spec 01
now has the rule, from `vsynclate`.

Measure these on the retail BIOS, as `terra.sh` does. Under OpenBIOS, Sony's image prints "Found C0
patch" inside the program's GsInitGraph, 82 characters at about 4,300 cycles each, which is most of
the 696,006 cycles that call takes there against 114,580 on retail.

TestTerraOrder and TestTerraOrderPressed compare every picture change, and TestTerraCosts totals the
cycles from each jump out of the program into the library, by entry. Per-call costs still differ
over Terra's first 824 frames, by entry, Sony's against clean:

| entry | Sony | clean |
|---|---|---|
| GsSetLightMatrix | 361 | 264 |
| GsGetLws | 948 | 870 |
| RotMatrix | 242 | 399 |
| ApplyMatrix | 46 | 177 |
| ApplyMatrixSV | 52 | 108 |
| GsClearOt | 272 | 320 |
