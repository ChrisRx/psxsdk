//go:build yaroze

package cleanlib

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
// transformation to the world as GsGetLs last worked it out, which Flg
// says is current.
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
}

// GsSetView2 makes the view the world-to-screen matrix, GsWSMATRIX: the
// view itself for a view in world coordinates, and otherwise the view
// after the transformation from the world to Super's coordinates. It
// returns 0.
//
//export GsSetView2
func GsSetView2(pv *GsVIEW2) int32 {
	if pv.Super == nil {
		gsWSMATRIX = pv.View
		return 0
	}
	lw := localToWorld(pv.Super)
	inv := invert(&lw)
	gsWSMATRIX = compose(&pv.View, &inv)
	return 0
}

// invert returns the inverse of a rotation and translation: the
// transposed rotation, and the translation turned back by it.
func invert(m *MATRIX) MATRIX {
	var r MATRIX
	for i := range 3 {
		for j := range 3 {
			r.M[i][j] = m.M[j][i]
		}
	}
	t := applyLong(&r, m.T)
	r.T = [3]int32{-t[0], -t[1], -t[2]}
	return r
}

// GsInitCoordinate2 makes base a coordinate system under super, or under
// the world when super is nil, with the identity as its matrix.
//
//export GsInitCoordinate2
func GsInitCoordinate2(super, base *GsCOORDINATE2) {
	base.Coord = identity()
	base.Flg = 0
	base.Super = super
	if super != nil {
		super.Sub = base
	}
}

// localToWorld returns the transformation from c's coordinates to the
// world's, the chain of matrices from the world down to c, keeping it in
// each system's Workm.
func localToWorld(c *GsCOORDINATE2) MATRIX {
	if c.Super == nil {
		c.Workm = c.Coord
	} else {
		up := localToWorld(c.Super)
		c.Workm = compose(&up, &c.Coord)
	}
	c.Flg = 1
	return c.Workm
}

// GsGetLs stores in m the transformation from coord's coordinates to the
// screen: the world-to-screen matrix after the chain of coordinate
// systems from the world down to coord.
//
//export GsGetLs
func GsGetLs(coord *GsCOORDINATE2, m *MATRIX) {
	lw := localToWorld(coord)
	*m = compose(&gsWSMATRIX, &lw)
}

// GsSetLsMatrix loads m into the GTE's rotation and translation, for
// GsSortObject4 to transform with.
//
//export GsSetLsMatrix
func GsSetLsMatrix(m *MATRIX) {
	SetRotMatrix(m)
	SetTransMatrix(m)
}

// GsGetLw stores in m the transformation from coord's coordinates to the
// world's.
//
//export GsGetLw
func GsGetLw(coord *GsCOORDINATE2, m *MATRIX) {
	*m = localToWorld(coord)
}

// GsGetLws stores in lw the transformation from coord's coordinates to the
// world's, and in ls the one to the screen's.
//
//export GsGetLws
func GsGetLws(coord *GsCOORDINATE2, lw, ls *MATRIX) {
	w := localToWorld(coord)
	*lw = w
	*ls = compose(&gsWSMATRIX, &w)
}

// GsRVIEW2 is a view given as a viewpoint, a point it looks at, and a
// twist about the line between them, in Super's coordinates.
type GsRVIEW2 struct {
	Vpx, Vpy, Vpz int32
	Vrx, Vry, Vrz int32
	Rz            int32
	Super         *GsCOORDINATE2
}

// GsSetRefView2 works out the world-to-screen matrix, GsWSMATRIX, for a
// view from the viewpoint towards the reference point, twisted by Rz
// about the line of sight. With d the line of sight, l its length and h
// the length of its projection on the ground, both from sqrt0, it is a
// turn about y and then one about x, from their cosines and sines as
// ratios rather than angles: about y by cy = dz*4096/h and sy =
// dx*4096/h, which brings d into the y-z plane, and about x by cx =
// h*4096/l and sx = dy*4096/l, which brings it onto z. Their product's
// elements are each rounded down from 8.24. With sqrt0's error the rows
// can be a little longer than 4096, as the reference's are. Then comes
// the twist, and then GsIDMATRIX2, which squeezes y for a screen whose
// pixels are not square. The translation, -WS times the viewpoint,
// rounded down, moves the viewpoint to the origin. A view in a coordinate
// system other than the world's is turned into the world's through it.
// It returns 0, or 1 when the two points are the same.
//
//export GsSetRefView2
func GsSetRefView2(pv *GsRVIEW2) int32 {
	dx, dy, dz := int64(pv.Vrx-pv.Vpx), int64(pv.Vry-pv.Vpy), int64(pv.Vrz-pv.Vpz)
	l := int64(sqrt0(dx*dx + dy*dy + dz*dz))
	h := int64(sqrt0(dx*dx + dz*dz))
	if l == 0 || h == 0 {
		return 1
	}
	cy, sy := int32(div64(dz*4096, h)), int32(div64(dx*4096, h))
	cx, sx := int32(div64(h*4096, l)), int32(div64(dy*4096, l))
	var m MATRIX
	m.M[0] = [3]int16{int16(cy), 0, int16(-sy)}
	m.M[1] = [3]int16{int16(-sx * sy >> 12), int16(cx), int16(-sx * cy >> 12)}
	m.M[2] = [3]int16{int16(cx * sy >> 12), int16(sx), int16(cx * cy >> 12)}
	if pv.Rz != 0 {
		var rz MATRIX
		rotZ(-pv.Rz, &rz)
		MulMatrix0(&rz, &m, &m)
	}
	MulMatrix0(&gsIDMATRIX2, &m, &m)
	vp := [3]int64{int64(pv.Vpx), int64(pv.Vpy), int64(pv.Vpz)}
	for i := range 3 {
		var t int64
		for j := range 3 {
			t += int64(m.M[i][j]) * vp[j]
		}
		m.T[i] = int32(-t >> 12)
	}
	view := GsVIEW2{View: m, Super: pv.Super}
	GsSetView2(&view)
	return 0
}

// rotZ makes m the rotation by a about z.
func rotZ(a int32, m *MATRIX) {
	s, c := int16(sin(a)), int16(cos(a))
	m.M = [3][3]int16{{c, -s, 0}, {s, c, 0}, {0, 0, 4096}}
}
