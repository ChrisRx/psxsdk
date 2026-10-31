//go:build yaroze

package libps

import "unsafe"

// libgte beyond the rotation matrices of matrix.go: the matrix products
// and the vector transforms the original ran on the GTE, which these do
// too, so that their results and what they leave in the coprocessor are
// the same; the matrix stack; the rotations about one axis; and the sine
// and square root tables.

//export libps_setLightMatrix
func setLightMatrix(m *MATRIX)

//export libps_setColorMatrix
func setColorMatrix(m *MATRIX)

//export libps_setFarColor
func setFarColor(r, g, b int32)

//export libps_setGeomScreen
func setGeomScreen(h int32)

//export libps_setDQA
func setDQA(v int32)

//export libps_setDQB
func setDQB(v int32)

//export libps_readRotTrans
func readRotTrans(m *MATRIX)

// gteColumn loads column j of m into V0.
func gteColumn(m *MATRIX, j int) {
	gteSetV0Words(uint32(uint16(m.M[0][j]))|uint32(uint16(m.M[1][j]))<<16, uint32(uint16(m.M[2][j])))
}

// mulMatrixInto works out m0's rotation times m1's into out's, with m0
// as the GTE's rotation matrix and m1's columns through it one at a
// time. The last element is written as a whole word, as the original
// did, so the padding after it takes the sign.
func mulMatrixInto(m0, m1, out *MATRIX) {
	setRotMatrix(m0)
	var c [3][3]int32
	for j := range c {
		gteColumn(m1, j)
		gteMVMVA()
		c[j] = [3]int32{gteIR1(), gteIR2(), gteIR3()}
	}
	out.M[0] = [3]int16{int16(c[0][0]), int16(c[1][0]), int16(c[2][0])}
	out.M[1] = [3]int16{int16(c[0][1]), int16(c[1][1]), int16(c[2][1])}
	out.M[2][0], out.M[2][1] = int16(c[0][2]), int16(c[1][2])
	*(*int32)(unsafe.Pointer(&out.M[2][2])) = c[2][2]
}

// MulMatrix multiplies m0's rotation by m1's into m0, and returns m0.
// The GTE's rotation matrix is left as m0 was.
//
// libps 0x8002D2B8, 272 bytes
//
//export MulMatrix
func MulMatrix(m0, m1 *MATRIX) *MATRIX {
	mulMatrixInto(m0, m1, m0)
	return m0
}

// MulMatrix0 multiplies m0's rotation by m1's into m2, and returns m2.
//
// libps 0x8002C9F8, 1000 bytes
//
//export MulMatrix0
func MulMatrix0(m0, m1, m2 *MATRIX) *MATRIX {
	mulMatrixInto(m0, m1, m2)
	return m2
}

// mulMatrix2 multiplies m0's rotation by m1's into m1, and returns m1.
//
// libps 0x8002D3C8, 272 bytes
func mulMatrix2(m0, m1 *MATRIX) *MATRIX {
	mulMatrixInto(m0, m1, m1)
	return m1
}

// CompMatrix composes m0 and m1 into m2: the rotations multiplied, and
// m1's translation, taken as 16 bits a component, turned by m0 and added
// to m0's.
//
// libps 0x8002C898, 352 bytes
//
//export CompMatrix
func CompMatrix(m0, m1, m2 *MATRIX) *MATRIX {
	mulMatrixInto(m0, m1, m2)
	gteSetV0Words(uint32(uint16(m1.T[0]))|uint32(uint16(m1.T[1]))<<16, uint32(m1.T[2]))
	gteMVMVA()
	m2.T = [3]int32{gteMAC1() + m0.T[0], gteMAC2() + m0.T[1], gteMAC3() + m0.T[2]}
	return m2
}

// ApplyMatrix turns v by m's rotation into out, and returns out.
//
// libps 0x8002D4D8, 80 bytes
//
//export ApplyMatrix
func ApplyMatrix(m *MATRIX, v *SVECTOR, out *VECTOR) *VECTOR {
	setRotMatrix(m)
	gteSetV0(unsafe.Pointer(v))
	gteMVMVA()
	out.Vx, out.Vy, out.Vz = gteMAC1(), gteMAC2(), gteMAC3()
	return out
}

// ApplyMatrixSV turns v by m's rotation into out, 16 bits a component,
// and returns out.
//
// libps 0x8002D528, 96 bytes
//
//export ApplyMatrixSV
func ApplyMatrixSV(m *MATRIX, v *SVECTOR, out *SVECTOR) *SVECTOR {
	setRotMatrix(m)
	gteSetV0(unsafe.Pointer(v))
	gteMVMVA()
	out.Vx, out.Vy, out.Vz = int16(gteIR1()), int16(gteIR2()), int16(gteIR3())
	return out
}

// ApplyMatrixLV turns the long vector v by m's rotation into out, and
// returns out. Each component goes through the GTE in two parts, its
// high 17 bits and its low 15, which the matrix turns separately.
//
// libps 0x8002CDE0, 400 bytes
//
//export ApplyMatrixLV
func ApplyMatrixLV(m *MATRIX, v *VECTOR, out *VECTOR) *VECTOR {
	applyMatrixLV(m, (*[3]int32)(unsafe.Pointer(v)), (*[3]int32)(unsafe.Pointer(out)))
	return out
}

func applyMatrixLV(m *MATRIX, v *[3]int32, out *[3]int32) {
	setRotMatrix(m)
	var hi, lo [3]int32
	for i, x := range v {
		if x < 0 {
			hi[i] = -((-x) >> 15)
			lo[i] = -((-x) & 0x7fff)
		} else {
			hi[i] = x >> 15
			lo[i] = x & 0x7fff
		}
	}
	gteSetIR(hi[0], hi[1], hi[2])
	gteMVMVAIR()
	h := [3]int32{gteMAC1(), gteMAC2(), gteMAC3()}
	gteSetIR(lo[0], lo[1], lo[2])
	gteMVMVAIRSF()
	out[0] = gteMAC1() + h[0]<<3
	out[1] = gteMAC2() + h[1]<<3
	out[2] = gteMAC3() + h[2]<<3
}

// TransposeMatrix writes m's rotation transposed into out, and returns
// out.
//
// libps 0x8002DEA8, 80 bytes
//
//export TransposeMatrix
func TransposeMatrix(m, out *MATRIX) *MATRIX {
	for i := range m.M {
		for j := range m.M[i] {
			out.M[j][i] = m.M[i][j]
		}
	}
	return out
}

// The matrix stack: PushMatrix saves the GTE's rotation matrix and
// translation vector, PopMatrix puts them back.
//
// libps 0x800490C4, 640 bytes, and the depth at 0x800490C0
var (
	gteStack    [20]MATRIX
	gteStackTop int
)

// PushMatrix saves the GTE's rotation matrix and translation vector on
// the stack.
//
// libps 0x8002D094, 160 bytes
//
//export PushMatrix
func PushMatrix() {
	if gteStackTop >= len(gteStack) {
		kprintf("Error: Can't push matrix,stack(max 20) is full!\n")
		return
	}
	readRotTrans(&gteStack[gteStackTop])
	gteStackTop++
}

// PopMatrix puts back what PushMatrix last saved.
//
// libps 0x8002D134, 388 bytes
//
//export PopMatrix
func PopMatrix() {
	if gteStackTop <= 0 {
		kprintf("Error: Can't pop matrix,stack is empty!\n")
		return
	}
	gteStackTop--
	m := &gteStack[gteStackTop]
	setRotMatrix(m)
	setTransMatrix(m)
}

// SetLightMatrix loads the light direction matrix.
//
// libps 0x8002D718, 48 bytes
//
//export SetLightMatrix
func SetLightMatrix(m *MATRIX) { setLightMatrix(m) }

// SetColorMatrix loads the light color matrix.
//
// libps 0x8002D748, 48 bytes
//
//export SetColorMatrix
func SetColorMatrix(m *MATRIX) { setColorMatrix(m) }

// SetBackColor sets the ambient color, each component up to 255.
//
// libps 0x8002D918, 32 bytes
//
//export SetBackColor
func SetBackColor(r, g, b int32) { setBackColor(r, g, b) }

// SetFarColor sets the fog's color, each component up to 255.
//
// libps 0x8002D938, 32 bytes
//
//export SetFarColor
func SetFarColor(r, g, b int32) { setFarColor(r, g, b) }

// SetGeomOffset sets the screen offset the projection adds.
//
// libps 0x8002D958, 32 bytes
//
//export SetGeomOffset
func SetGeomOffset(x, y int32) { setGeomOffset(x, y) }

// SetGeomScreen sets the projection distance.
//
// libps 0x8002D978, 16 bytes
//
//export SetGeomScreen
func SetGeomScreen(h int32) { setGeomScreen(h) }

// RotMatrixX turns m's rotation about the x axis by r, in 4096ths of a
// turn, and returns m.
//
// libps 0x8002E188, 416 bytes
//
//export RotMatrixX
func RotMatrixX(r int32, m *MATRIX) *MATRIX {
	s, c := sinCos(r)
	m1, m2 := m.M[1], m.M[2]
	for j := range m1 {
		a, b := int32(m1[j]), int32(m2[j])
		m.M[1][j] = int16((c*a - s*b) >> 12)
		m.M[2][j] = int16((s*a + c*b) >> 12)
	}
	return m
}

// RotMatrixY turns m's rotation about the y axis by r, in 4096ths of a
// turn, and returns m. The turn goes the other way from the x and z
// ones, as libgte had it.
//
// libps 0x8002E328, 416 bytes
//
//export RotMatrixY
func RotMatrixY(r int32, m *MATRIX) *MATRIX {
	s, c := sinCos(r)
	s = -s
	m0, m2 := m.M[0], m.M[2]
	for j := range m0 {
		a, b := int32(m0[j]), int32(m2[j])
		m.M[0][j] = int16((c*a - s*b) >> 12)
		m.M[2][j] = int16((s*a + c*b) >> 12)
	}
	return m
}

// RotMatrixZ turns m's rotation about the z axis by r, in 4096ths of a
// turn, and returns m.
//
// libps 0x8002E4C8, 408 bytes
//
//export RotMatrixZ
func RotMatrixZ(r int32, m *MATRIX) *MATRIX {
	s, c := sinCos(r)
	m0, m1 := m.M[0], m.M[1]
	for j := range m0 {
		a, b := int32(m0[j]), int32(m1[j])
		m.M[0][j] = int16((c*a - s*b) >> 12)
		m.M[1][j] = int16((s*a + c*b) >> 12)
	}
	return m
}

// ScaleMatrixL scales m's rows by v, in 4.12, and returns m. The last
// element is written as a whole word, as ScaleMatrix does.
//
// libps 0x8002CF70, 292 bytes
//
//export ScaleMatrixL
func ScaleMatrixL(m *MATRIX, v *VECTOR) *MATRIX {
	s := [3]int32{v.Vx, v.Vy, v.Vz}
	for i := range m.M {
		for j := range m.M[i] {
			if i == 2 && j == 2 {
				*(*int32)(unsafe.Pointer(&m.M[2][2])) = mul(int32(m.M[2][2]), s[2])
				break
			}
			m.M[i][j] = int16(mul(int32(m.M[i][j]), s[i]))
		}
	}
	return m
}

// rsin is the sine of an angle in 4096ths of a turn, in 4.12: the
// quarter-turn table folded over the other quarters, and a negative angle
// the negative of its opposite.
//
// libps 0x8002BDA8, 60 bytes, and 0x8002BDE4, 144 bytes
func rsin(a int32) int32 {
	if a < 0 {
		return -rsinPositive(uint32(-a) & 0xfff)
	}
	return rsinPositive(uint32(a) & 0xfff)
}

func rsinPositive(a uint32) int32 {
	switch {
	case a <= 1024:
		return int32(sinTable[a])
	case a <= 2048:
		return int32(sinTable[2048-a])
	case a <= 3072:
		return -int32(sinTable[a-2048])
	}
	return -int32(sinTable[4096-a])
}

// rcos is the cosine of an angle in 4096ths of a turn, in 4.12.
//
// libps 0x8002BE74, 160 bytes
func rcos(a int32) int32 {
	if a < 0 {
		a = -a
	}
	n := uint32(a) & 0xfff
	switch {
	case n <= 1024:
		return int32(sinTable[1024-n])
	case n <= 2048:
		return -int32(sinTable[n-1024])
	case n <= 3072:
		return -int32(sinTable[3072-n])
	}
	return int32(sinTable[n-3072])
}

// SquareRoot0 is the square root of a, to the nearest whole number below:
// a's leading byte looked up in the table, scaled by half its leading
// zeros.
//
// libps 0x8002C448, 144 bytes
//
//export SquareRoot0
func SquareRoot0(a int32) int32 {
	lz := gteLZCR(a)
	if lz == 32 {
		return 0
	}
	even := lz &^ 1
	top := a
	if even >= 24 {
		top <<= uint(even - 24)
	} else {
		top >>= uint(24 - even)
	}
	root := uint32(int32(sqrtTable[top-64])) << uint((31-even)>>1)
	return int32(root >> 12)
}

// GteMIMefunc adds n differences to n vertices, each scaled by p in
// 4.12 through the GTE: the vertex animation of a model blended towards
// another shape.
//
// libps 0x8002C778, 128 bytes
//
//export gteMIMefunc
func GteMIMefunc(otp, dfp *SVECTOR, n, p int32) {
	gteSetIR0(p)
	out := unsafe.Slice(otp, n)
	diff := unsafe.Slice(dfp, n)
	for i := range diff {
		d := &diff[i]
		gteSetIR(int32(d.Vx), int32(d.Vy), int32(d.Vz))
		gteGPF()
		o := &out[i]
		o.Vx += int16(gteIR1())
		o.Vy += int16(gteIR2())
		o.Vz += int16(gteIR3())
	}
}
