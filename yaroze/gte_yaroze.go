//go:build yaroze

package yaroze

import (
	"device"
	"unsafe"
)

// The GTE, the console's geometry coprocessor, for a program that transforms
// its own vertices rather than leaving every model to GsSortObject4. The
// resident library sets the coprocessor up and keeps its matrices: GsSetLsMatrix
// loads the rotation and translation of the coordinate system being drawn,
// GsSetLightMatrix the light directions, GsSetFlatLight and GsSetAmbient the
// light colors, GsSetProjection the projection distance and GsInitGraph the
// screen offset. These are the commands a program then issues against them,
// each a few instructions of the coprocessor's own in the pattern of the
// SDK's inline macros: load the arguments into the coprocessor, run one
// command, store the results. They are inline assembly, so that a loop
// over a model's polygons pays no call for each; a command's inputs and
// outputs stay in the coprocessor between them, which the sequence of a
// polygon relies on: RotTransPers3 leaves three screen points and depths for
// NormalClip and AverageZ3 to work on. The assembler takes the
// coprocessor's loads, stores and data moves by name, but not its control
// moves or commands, which are written as words: a command is 0x4a000000
// with its function word, as the SDK's macros give it.
//
// The three that move the control registers, SetRotMatrix, SetTransMatrix
// and ReadFlag, are functions the playstation-yaroze target defines in
// assembly, since inline assembly cannot name the register it is given a
// value in, which the control moves' encoding needs.
//
// A block given a register says .set noat: the compiler may give it $at,
// the register the assembler keeps for expanding its macros, and the
// assembler warns at each use of it unless told the block expands none,
// which none of these do.
//
// A screen point is packed as the GPU takes it, x in the low half and y in
// the high; a color as the GPU takes it, red in the low byte, then green and
// blue, and the GPU command code in the top byte, which the lighting
// commands copy from what SetRGBC gave them. The flag is the coprocessor's
// FLAG register: negative when a result overflowed, which a program treats
// as a polygon to leave out.
//
// The stubs for every function here are in gte_stub.go, with the same
// comments.

// RotTransPers3 transforms three vertices to the screen with RTPT, stores
// their screen points in sxy, and returns the flag. Their depths remain in
// the coprocessor for AverageZ3, and the points for NormalClip.
//
//go:inline
func RotTransPers3(v0, v1, v2 *SVECTOR, sxy *[3]uint32) int32 {
	loadV3(v0, v1, v2)
	device.AsmFull(`nop
	nop
	.word 0x4a280030`, nil)
	sxy[0] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$12
	nop
	.set pop`, nil))
	sxy[1] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$13
	nop
	.set pop`, nil))
	sxy[2] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$14
	nop
	.set pop`, nil))
	return ReadFlag()
}

// loadV3 loads three vectors into V0, V1 and V2, and loadV0 one into V0.
//
//go:inline
func loadV3(v0, v1, v2 *SVECTOR) {
	a, b := words(v0)
	c, d := words(v1)
	e, f := words(v2)
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	mtc2 {c}, $$2
	mtc2 {d}, $$3
	mtc2 {e}, $$4
	mtc2 {f}, $$5
	.set pop`, map[string]interface{}{"a": a, "b": b, "c": c, "d": d, "e": e, "f": f})
}

//go:inline
func loadV0(v *SVECTOR) {
	a, b := words(v)
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$0
	mtc2 {b}, $$1
	.set pop`, map[string]interface{}{"a": a, "b": b})
}

// words is a vector as the two words the coprocessor takes: x and y, then
// z.
//
//go:inline
func words(v *SVECTOR) (uint32, uint32) {
	p := unsafe.Pointer(v)
	return *(*uint32)(p), *(*uint32)(unsafe.Add(p, 4))
}

// RotTransPers transforms one vertex to the screen with RTPS, stores its
// screen point in sxy, and returns the flag. The point and its depth push
// the coprocessor's three-deep queues, so after RotTransPers3 it makes the
// fourth corner of a quad, and AverageZ4 sees all four depths.
//
//go:inline
func RotTransPers(v *SVECTOR, sxy *uint32) int32 {
	loadV0(v)
	*sxy = uint32(device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4a180001
	mfc2 {}, $$14
	nop
	.set pop`, nil))
	return ReadFlag()
}

// NormalClip is NCLIP on the last three screen points: the signed area they
// enclose, positive when they turn clockwise on the screen, which is a
// polygon facing the viewer, and negative or zero for one to leave out.
//
//go:inline
func NormalClip() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b400006
	mfc2 {}, $$24
	nop
	.set pop`, nil))
}

// AverageZ3 is AVSZ3: the average of the last three depths, scaled by the
// ordering table factor the library set, as the depth to sort at.
//
//go:inline
func AverageZ3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b58002d
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

// AverageZ4 is AVSZ4: the average of the last four depths, scaled, as the
// depth to sort at.
//
//go:inline
func AverageZ4() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	.word 0x4b68002e
	mfc2 {}, $$7
	nop
	.set pop`, nil))
}

// ReadSZ3 returns the depth of the last vertex transformed, before it is
// averaged, for a program that keeps transformed vertices to reuse.
//
//go:inline
func ReadSZ3() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$19
	nop
	.set pop`, nil))
}

// NormalColorCol3 lights three normals with NCCT, the color each gives under
// the lights set, times the color SetRGBC gave, into rgb with its command
// code.
//
//go:inline
func NormalColorCol3(n0, n1, n2 *SVECTOR, rgb *[3]uint32) {
	loadV3(n0, n1, n2)
	device.AsmFull(`nop
	nop
	.word 0x4b18043f`, nil)
	rgb[0] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$20
	nop
	.set pop`, nil))
	rgb[1] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$21
	nop
	.set pop`, nil))
	rgb[2] = uint32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$22
	nop
	.set pop`, nil))
}

// NormalColorCol lights one normal with NCCS and returns its color with the
// command code.
//
//go:inline
func NormalColorCol(n *SVECTOR) uint32 {
	loadV0(n)
	return uint32(device.AsmFull(`.set push
	.set noat
	nop
	nop
	.word 0x4b08041b
	mfc2 {}, $$22
	nop
	.set pop`, nil))
}

// NormalColor lights one normal with NCS, leaving out the color matrix: the
// result is the light falling on it, IR1 to IR3, which RotColor can turn
// into a color under any color later, for a program that lights its
// normals once and draws them many times.
//
//go:inline
func NormalColor(n *SVECTOR, ir *[3]int16) {
	loadV0(n)
	device.AsmFull(`nop
	nop
	.word 0x4ac8041e`, nil)
	ir[0] = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$9
	nop
	.set pop`, nil))
	ir[1] = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$10
	nop
	.set pop`, nil))
	ir[2] = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$11
	nop
	.set pop`, nil))
}

// RotColor multiplies the vector ir, as NormalColor left it, by the rotation
// matrix, clamped at zero, into out. With the rotation matrix set to a
// color on its diagonal, SetRotMatrix with a MATRIX whose M[0][0], M[1][1]
// and M[2][2] are red, green and blue, it colors a lit normal.
//
//go:inline
func RotColor(ir *[3]int16, out *[3]int32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$9
	mtc2 {b}, $$10
	mtc2 {c}, $$11
	nop
	nop
	.word 0x4a49e012
	.set pop`, map[string]interface{}{"a": int32(ir[0]), "b": int32(ir[1]), "c": int32(ir[2])})
	out[0] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$9
	nop
	.set pop`, nil))
	out[1] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$10
	nop
	.set pop`, nil))
	out[2] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$11
	nop
	.set pop`, nil))
}

// RotTrans rotates and translates one vector into out, without projecting
// it, and returns the flag.
//
//go:inline
func RotTrans(v *SVECTOR, out *VECTOR) int32 {
	loadV0(v)
	device.AsmFull(`nop
	nop
	.word 0x4a480012`, nil)
	out.Vx = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$25
	nop
	.set pop`, nil))
	out.Vy = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$26
	nop
	.set pop`, nil))
	out.Vz = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$27
	nop
	.set pop`, nil))
	return ReadFlag()
}

// SetRGBC sets the color the lighting commands multiply by, with the GPU
// command code in the top byte, which they carry into every color they
// produce: 0x34808080 for a gouraud textured triangle at full brightness.
//
//go:inline
func SetRGBC(c uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {c}, $$6
	.set pop`, map[string]interface{}{"c": c})
}

// SetRotMatrix loads the rotation matrix from m. GsSetLsMatrix does this
// too, with the translation; a program that sets a color matrix for
// RotColor sets the real one again afterwards.
//
//export SetRotMatrix
func SetRotMatrix(m *MATRIX)

// SetTransMatrix loads the translation vector from m.
//
//export SetTransMatrix
func SetTransMatrix(m *MATRIX)

// LoadScreen3 loads three screen points and depths that an earlier
// RotTransPers produced, so that NormalClip and AverageZ3 can run on
// vertices transformed once and shared between polygons.
//
//go:inline
func LoadScreen3(sxy, sz *[3]uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$12
	mtc2 {b}, $$13
	mtc2 {c}, $$14
	mtc2 {d}, $$17
	mtc2 {e}, $$18
	mtc2 {f}, $$19
	.set pop`, map[string]interface{}{"a": sxy[0], "b": sxy[1], "c": sxy[2], "d": sz[0], "e": sz[1], "f": sz[2]})
}

// LoadScreen4 loads three screen points and four depths, for NormalClip and
// AverageZ4 on a quad of vertices transformed earlier.
//
//go:inline
func LoadScreen4(sxy *[3]uint32, sz *[4]uint32) {
	device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$12
	mtc2 {b}, $$13
	mtc2 {c}, $$14
	mtc2 {d}, $$16
	mtc2 {e}, $$17
	mtc2 {f}, $$18
	mtc2 {g}, $$19
	.set pop`, map[string]interface{}{"a": sxy[0], "b": sxy[1], "c": sxy[2], "d": sz[0], "e": sz[1], "f": sz[2], "g": sz[3]})
}

// ReadSZ4 stores the depths of the last four vertices transformed, which
// RotTransPers3 and then RotTransPers leave as a quad's four corners, for
// a program that sorts by the farthest rather than the average.
//
//go:inline
func ReadSZ4(sz *[4]int32) {
	sz[0] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$16
	nop
	.set pop`, nil))
	sz[1] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$17
	nop
	.set pop`, nil))
	sz[2] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$18
	nop
	.set pop`, nil))
	sz[3] = int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$19
	nop
	.set pop`, nil))
}

// ReadMAC0 returns MAC0, which after RotTransPers holds the depth cue the
// point was given, and after NormalClip the area.
//
//go:inline
func ReadMAC0() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$24
	nop
	.set pop`, nil))
}

// ReadFlag returns the flag register as the last command left it.
//
//export ReadFlag
func ReadFlag() int32
