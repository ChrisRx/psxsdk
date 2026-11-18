# Milestone 3: 3D, starting with examples/flame

`examples/flame` is a single model built in memory: a flat sheet of
gouraud-shaded quads with lighting off, recolored every frame and drawn
through the 3D library at 640 by 480 interlaced. It is the smallest 3D
program and the first step to the TMD examples (morph, car, dungeon and
the rest).

## Acceptance

    exp/cleanlib/build.sh flame-clean examples/flame
    PSX_CLEAN=flame go test ./tests -run 'TestClean$' -v
    PSX_CLEAN=flame PSX_CLEAN_PAD=0410 go test ./tests -run 'TestClean$' -v

The reference disc `.build/flame-ref` is built. 0410 holds up and L1, which
tilt and roll the sheet, so the second run exercises the rotation path.

flame prints the horizontal lines a frame takes (VSync's count) in the
top left: like timers' readout this measures CPU time and may differ.
Every other word must match. If the count's digits are the only
difference, say so and count it a pass.

## Functions

Beyond milestones 1 and 2 flame needs about:

    ResetGraph GsInit3D GsSetProjection GsSetView2
    GsInitCoordinate2 GsGetLs GsSetLsMatrix
    GsMapModelingData GsLinkObject4 GsSortObject4
    RotMatrix TransMatrix MulMatrix0

The Library Reference describes each, and the File Formats manual
(`docs/manuals/040-1997-fileformat.pdf`) the TMD format: the header, the
object table, and each primitive's packet (mode, flag, lengths, then its
colors, normals and vertex indices). `yaroze/` declares the structures
(GsDOBJ2, GsCOORDINATE2, GsVIEW2, MATRIX, SVECTOR, VECTOR).

What is not obvious from those:

**The GTE.** These functions are the GTE's work, and their results must
equal the reference's to the bit, so do the arithmetic on the GTE
itself (rotation and translation registers, MVMVA, RTPS/RTPT, NCLIP,
AVSZ3/AVSZ4) rather than in Go integer math where the manual says a
function uses it, and match its truncation where you must compute in Go.
`yaroze/gte_yaroze.go` and `internal/toolchain/_tinygo/targets/playstation-yaroze-gte.S`
show how to reach the GTE from TinyGo (inline asm via `device.AsmFull`,
`.word` encodings for the cop2 instructions; read the comments there on
which registers are safe). The GTE must be enabled in the CPU's status
register (bit 30) before use; GsInit3D does that.

**Sines.** RotMatrix and its kin take angles in 4096ths of a turn. The
sine of angle a in 4.12 fixed point is `floor(4096*sin(2*pi*a/4096) + 0.5)`
for the quarter turn 0 to 1024, and the other quarters fold onto it by
symmetry. Generate the table with a `gen/` command like `gen/font`.

**Matrices.** RotMatrix builds the rotation from a vector of three angles
as the manual says (check the order of the axes against the stream: a
wrong order shows as different vertex positions once the sheet tilts).
MulMatrix0(m0, m1, m2) sets m2 = m0 * m1 through the GTE, each element a
sum of three products of 4.12 values shifted right by 12. TransMatrix
sets a matrix's translation.

**Coordinate systems.** GsInitCoordinate2(super, base) makes `base` a
coordinate system under `super`, or under the world when super is nil,
with an identity matrix. GsGetLs(coord, m) gives the local-to-screen
matrix: the world-to-screen matrix GsSetView2 made times the chain of
coordinate matrices from the world down to coord (cache flags in the
struct are for skipping recomputation; the result must be the same
either way). GsSetLsMatrix loads a matrix into the GTE's rotation and
translation registers. GsSetView2(view) computes the world-to-screen
matrix from the viewpoint, the reference point and the roll, as the
manual describes. GsSetProjection(h) sets the projection distance (the
GTE's H register).

**Models.** GsMapModelingData(tmd) turns the TMD's object table offsets
into addresses in place and marks the TMD as mapped (its flags word),
so a second call does nothing. GsLinkObject4(tmd, obj, n) points obj at
object n of the mapped TMD (the address of its object table entry).
GsSortObject4(obj, ot, shift, scratch) transforms every primitive of the
object with the matrix in the GTE, drops what is behind the screen or
faces away when single sided (NCLIP), and builds a GPU packet for each
in the work area, linked into the ordering table at depth `average z >>
shift`. For this milestone implement the primitive kinds flame uses
(gouraud quads, lighting off) and, if they come cheap, flat and
gouraud triangles and quads without texture; list what you left out.
The packet words and the order they are linked in are in the reference
stream.

**ResetGraph(mode)**: 0 resets the GPU completely, 1 cancels the current
drawing and clears the queue, 3 resets but keeps the drawing and display
environments, as the manual says. Compare the words it writes with the
reference stream.

## Findings (milestone 3 done)

flame matches but for its lines-per-frame readout (the clean library
takes 240 lines a frame against the reference's 190; both keep one
frame per loop). Corrections to this spec:

- GsInitGraph interlaces any display over 256 lines, even without the
  GsINTER bit.
- GP1 08h follows a swap's 05h only when the display start changed from
  the last one written (GsInitGraph's own write counts).
- GsInit3D sends one drawing environment with only the offset changed,
  and sets OFX = OFY = 0, H = 1000.
- ZSF3 = 4096/12 and ZSF4 = 4096/16: OTZ is a 14-bit average depth over
  4. Quads use AVSZ4 over all four corners.
- ResetGraph(0) writes the same reset sequence as GsInitGraph.
- RotMatrix builds Rx·Ry·Rz, each product shifted right by 12;
  MulMatrix0 is MVMVA (sf=1, no translation) on each column of m1. Not
  yet checked with mixed rotations.
- GsSetView2 takes a ready matrix (the viewpoint form is
  GsSetRefView2); with a WORLD super GsWSMATRIX is the view's matrix.
- GsSortObject4 handles unlit, untextured flat and gouraud triangles and
  quads; the TMD's mode byte is the GPU command (top byte of the first
  color word). Lit, textured, lines and sprites are skipped for now.
