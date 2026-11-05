//go:build yaroze

package cleanlib

import "unsafe"

// The 3D side of the Gs library: coordinate systems, the view, and the
// matrices the GTE transforms models with.

// GsCOORD2PARAM is a coordinate system given as a scale, a rotation and a
// translation, for programs that keep one beside the matrix.
type GsCOORD2PARAM struct {
	Scale  VECTOR
	Rotate SVECTOR
	Trans  VECTOR
}

// GsCOORDINATE2 is a coordinate system: its matrix in its parent's
// coordinates, its parent, or nil for the world, and Workm, its
// transformation to the world as it was last worked out. Flg is compared
// with the frame counter, PSDCNT: 0 means the program changed Coord, the
// counter means Workm was worked out in this frame, and anything else in an
// earlier frame, with nothing changed since (localToWorld).
type GsCOORDINATE2 struct {
	Flg   uint32
	Coord MATRIX
	Workm MATRIX
	Param *GsCOORD2PARAM
	Super *GsCOORDINATE2
	Sub   *GsCOORDINATE2
}

// GsVIEW2 is a view given as the matrix from Super's coordinates to the
// screen's.
type GsVIEW2 struct {
	View  MATRIX
	Super *GsCOORDINATE2
}

//go:extern GsWSMATRIX
var gsWSMATRIX MATRIX

// GsInit3D sets up the 3D side after GsInitGraph: it turns the GTE on,
// moves the screen origin to the middle of the screen, and sets the GTE's
// screen offset, projection distance, depth scales and depth cue.
//
//export GsInit3D
func GsInit3D() {
	gteEnable()
	position = GsPOSITION{Offx: gs.width / 2, Offy: gs.height / 2}
	if gs.ofsGPU {
		setOFX(0)
		setOFY(0)
	}
	gs.offset()
	setH(1000)
	// The default depth cue, for that distance: DQA -4194304/1000, so
	// that the fade is complete at a depth of 65536, and DQB 1.25 in
	// 8.24.
	setDQ(-4194, 0x1400000)
	// AVSZ3 and AVSZ4 average the depths and scale them to the 14 bits
	// of depth an ordering table is sorted by: a twelfth and a sixteenth
	// of 4096.
	setZSF3(4096 / 12)
	setZSF4(4096 / 16)
}

// GsSetProjection sets the projection distance h, the GTE's H register.
//
//export GsSetProjection
func GsSetProjection(h int32) {
	setH(h)
	space(1)
}

// GsSetView2 makes the view the world-to-screen matrix, GsWSMATRIX: a copy
// of the view for a view in world coordinates, and otherwise the view after
// the transformation from the world to Super's coordinates, with Super's
// chain brought up to date first. It returns 0.
//
//export GsSetView2
func GsSetView2(pv *GsVIEW2) int32

// wsUnscaled is GsWSMATRIX as the last GsSetRefView2 or GsSetView2 that worked out
// a view left it, what GsScaleScreen scales.
//
//go:extern libps_ws_unscaled
var wsUnscaled MATRIX

// viewUnder makes view, in super's coordinates, the world-to-screen matrix, with
// super's chain brought up to date first: view after the inverse of super's Workm,
// the rotation transposed and the translation turned by it and negated, each
// row's result negated since the rows round down. In assembly.
//
//export libps_view
func viewUnder(view *MATRIX, super *GsCOORDINATE2)

// GsInitCoordinate2 makes base a coordinate system under super, or under
// the world when super is nil, its matrix a copy of GsIDMATRIX, and makes
// base super's Sub. It leaves base's Workm, Param and Sub.
//
//export GsInitCoordinate2
func GsInitCoordinate2(super, base *GsCOORDINATE2) {
	base.Coord = gsIDMATRIX
	base.Flg = 0
	base.Super = super
	if super != nil {
		super.Sub = base
	}
}

// localToWorld brings c's Workm up to date with what its chain's flags say,
// and returns it, without recursion or a table, since a program's stack may be
// the 1 KB scratchpad, and with no link in the chain changed even for a time.
// With C the frame counter (spec 19, "GsGetLw, GsGetLs, GsGetLws"):
//
//   - A system below the root flagged C is current, and nothing is done.
//   - Otherwise the top is the first system above c flagged C, or the root.
//     A root flagged 0 or C takes its Coord as its Workm and is flagged C.
//     From a top that is flagged C, each system below it down to c is worked
//     out from its parent's Workm and its Coord, and flagged C. From a root
//     flagged anything else that only starts at the highest system flagged 0,
//     if there is one: the systems above it are as good as they were.
//
// The walk up finds the highest system to work out, and each below that is
// about to get a new Workm, so until then its Workm holds the way back down:
// the address of the system under it. In assembly, with GsGetLs and the rest.
//
// A null system, the manual's WORLD, is the one at address 0, with its words as
// they are (spec 20).

// GsGetLs stores in m the transformation from coord's coordinates to the
// screen: GsWSMATRIX after coord's Workm, once localToWorld has seen to it.
// Nothing keeps this matrix, so it is worked out at every call. In assembly.
//
//export GsGetLs
func GsGetLs(coord *GsCOORDINATE2, m *MATRIX)

// GsSetLsMatrix loads m into the GTE's rotation and translation, for
// GsSortObject4 to transform with. In assembly.
//
//export GsSetLsMatrix
func GsSetLsMatrix(m *MATRIX)

// GsGetLw stores in m a copy of coord's Workm, the transformation from its
// coordinates to the world's, once localToWorld has seen to it. In assembly.
//
//export GsGetLw
func GsGetLw(coord *GsCOORDINATE2, m *MATRIX)

// GsGetLws stores in lw and ls what GsGetLw and GsGetLs store. In assembly.
//
//export GsGetLws
func GsGetLws(coord *GsCOORDINATE2, lw, ls *MATRIX)

// GsRVIEW2 is a view given as a viewpoint, a point it looks at, and a
// twist about the line between them, in Super's coordinates.
type GsRVIEW2 struct {
	Vpx, Vpy, Vpz int32
	Vrx, Vry, Vrz int32
	Rz            int32
	Super         *GsCOORDINATE2
}

// GsSetRefView2 works out the world-to-screen matrix, GsWSMATRIX, for a
// view from the viewpoint towards the reference point. With either point's
// elements past 16 bits both points are shifted right together until none is. With d
// the line of sight then, l its length and h the length of its projection on the
// ground, both from sqrt0 and in 32 bits, which a sight over 46,340 long passes, the
// rotation is made up in steps, each a product rounded down:
// GsIDMATRIX2, which squeezes y for a screen whose pixels are not square,
// times the twist, times a turn about x and then one about y, from their
// cosines and sines as ratios rather than angles: about x by cx = h*4096/l and
// sx = dy*4096/l, and about y by cy = dz*4096/h and sy = dx*4096/h. With
// sqrt0's error the rows can be a little longer than 4096, as the reference's
// are. The translation, WS times the viewpoint's opposite as the coordinate calls
// multiply it (turnLong), moves the viewpoint to the origin. A view in a coordinate system other than the
// world's is turned into the world's through it, as GsSetView2 does it. It
// returns 0.
//
// Looking straight up or down is a view like any other, with h 0, cy 4096 and
// sy 0. With the two points the same it returns 1 without a look at the chain:
// GsWSMATRIX is GsIDMATRIX2 times the twist, with GsIDMATRIX2's translation, and
// GsScaleScreen goes on from the last view worked out.
//
// The twist is in 4096ths of a degree: the angle, in 4096ths of a turn, is the
// opposite of Rz over 360, rounded toward zero (probe spec/probes/gsrz).
//
//export GsSetRefView2
func GsSetRefView2(pv *GsRVIEW2) int32 {
	// A view in the world's coordinates is made in place.
	m, turn := &gsWSMATRIX, &refTurn
	if pv.Super != nil {
		m = &refView
	}
	*m = gsIDMATRIX2
	if pv.Rz != 0 {
		rotZ(-pv.Rz/360, turn)
		MulMatrix0(m, turn, m)
		delay(refAfterTwist)
	} else {
		delay(refNoTwist)
	}
	// Points too far out for 16 bits are brought in together.
	k := uint32(0)
	for w := magnitude(pv.Vpx) | magnitude(pv.Vpy) | magnitude(pv.Vpz) | magnitude(pv.Vrx) | magnitude(pv.Vry) | magnitude(pv.Vrz); w >= 1<<15; w >>= 1 {
		k++
	}
	k &= 31
	dx, dy, dz := pv.Vrx>>k-pv.Vpx>>k, pv.Vry>>k-pv.Vpy>>k, pv.Vrz>>k-pv.Vpz>>k
	l := sqrt0(dx*dx + dy*dy + dz*dz)
	if l == 0 {
		gsWSMATRIX = *m
		return 1
	}
	delay(refSquares)
	h := sqrt0(dx*dx + dz*dz)
	cx, sx := int16(h*4096/l), int16(dy*4096/l)
	turn.M = [3][3]int16{{4096, 0, 0}, {0, cx, -sx}, {0, sx, cx}}
	MulMatrix0(m, turn, m)
	delay(refTurnTwo)
	cy, sy := int16(4096), int16(0)
	if h != 0 {
		cy, sy = int16(dz*4096/h), int16(dx*4096/h)
	}
	turn.M = [3][3]int16{{cy, 0, -sy}, {0, 4096, 0}, {sy, 0, cy}}
	MulMatrix0(m, turn, m)
	refSpot = VECTOR{Vx: -pv.Vpx, Vy: -pv.Vpy, Vz: -pv.Vpz}
	ApplyMatrixLV(m, &refSpot, (*VECTOR)(unsafe.Pointer(&m.T)))
	if pv.Super == nil {
		wsUnscaled = gsWSMATRIX
		return 0
	}
	viewUnder(m, pv.Super)
	return 0
}

// What GsSetRefView2 spends past its work, in nops, for Sony's times from each thing the
// GTE sees to the next.
const (
	refAfterTwist = 83
	refNoTwist    = 124
	refSquares    = 5
	refTurnTwo    = 19
)

// delay spends n instructions and the call. In assembly, for what a Go call needs to
// spend exactly.
//
//export libps_delay
func delay(n uint32)

// magnitude returns the size of v, as an unsigned value for the most negative.
//
//go:inline
func magnitude(v int32) uint32 {
	if v < 0 {
		return uint32(-v)
	}
	return uint32(v)
}

// refView and refTurn are GsSetRefView2's matrices, not locals, which the
// compiler would put on the heap.
var refView, refTurn MATRIX

// refSpot is GsSetRefView2's vector to the viewpoint.
var refSpot VECTOR

// rotZ makes m the rotation by a about z. Its sine and cosine are true to the
// table in every quarter, unlike RotMatrix's (sin).
func rotZ(a int32, m *MATRIX) {
	mag := uint32(a)
	if a < 0 {
		mag = uint32(-a)
	}
	s, c := int16(trueSine(mag)), int16(trueSine(mag+1024))
	if a < 0 {
		s = -s
	}
	m.M = [3][3]int16{{c, -s, 0}, {s, c, 0}, {0, 0, 4096}}
}

// trueSine returns the sine of the non-negative angle a: the second and fourth
// quarters read the table from its other end.
func trueSine(a uint32) int32 {
	i := a & 0x3ff
	if a&0x400 != 0 {
		i = 1024 - i
	}
	v := int32(sineTable[i])
	if a&0x800 != 0 {
		return -v
	}
	return v
}
