//go:build yaroze

package cleanlib

import (
	"device"
	"unsafe"
)

// Matrices: rotations from angles, translations, and products on the GTE.
// An angle is in 4096ths of a turn, and a matrix element in 4.12 fixed
// point, 4096 being 1.

// SVECTOR is a vector of 16-bit elements: a vertex, a normal or angles.
type SVECTOR struct {
	Vx, Vy, Vz, Pad int16
}

// VECTOR is a vector of 32-bit elements.
type VECTOR struct {
	Vx, Vy, Vz, Pad int32
}

// sin returns the sine of the angle a in 4.12 fixed point, from the table
// of the first quarter turn, which the other quarters fold onto. As the
// reference library's does, the second and fourth quarters read the table
// one step early: the sine of 1024+d is the table's entry 1023-d, not
// 1024-d, so that the cosine of a small angle d reads as the sine of
// 1023-d. The streams show it: cos(48) is 4084, the sine of 975, where the
// true value rounds to 4085. A negative angle is the sine of its opposite,
// negated, as flame's rotations by -64 show.
//
// The opposite is taken in 32 bits, so the most negative angle is its
// own opposite, the sine of 80000000h, 0 (probe spec/probes/matrix).
func sin(a int32) int32 {
	if a < 0 {
		return -sinMag(uint32(-a))
	}
	return sinMag(uint32(a))
}

// sinMag is the sine of the non-negative angle a.
//
//go:inline
func sinMag(a uint32) int32 {
	i := a & 0x3ff
	if a&0x400 != 0 {
		i = 0x3ff - i
	}
	if a&0x800 != 0 {
		return -int32(sineTable[i])
	}
	return int32(sineTable[i])
}

// cos returns the cosine of the angle a in 4.12 fixed point, the sine of
// a quarter turn more. A negative angle's is its opposite's, as player 2's
// turns in split show: cos(-224) is cos(224), 3854.
func cos(a int32) int32 {
	m := uint32(a)
	if a < 0 {
		m = uint32(-a)
	}
	return sinMag(m + 1024)
}

// sinCos returns sin(a) and cos(a) from the table's two entries for the angle
// within its quarter, each the other's in the next quarter (see sin).
//
//go:inline
func sinCos(a int32) (s, c int32) {
	m := uint32(a)
	if a < 0 {
		m = uint32(-a)
	}
	i := m & 0x3ff
	s, c = int32(sineTable[i]), int32(sineTable[0x3ff-i])
	if m&0x400 != 0 {
		s, c = c, -s
	}
	if m&0x800 != 0 {
		s, c = -s, -c
	}
	if a < 0 {
		s = -s
	}
	return s, c
}

// mulShift returns a times b, shifted right by 12, as an element of 4.12. It
// is a block of its own since the compiler, left to it, keeps products in the
// HI and LO registers across the sine's branches and so saves them on the stack,
// which a C program may keep small. Nothing around it may multiply: the
// compiler does not know that this changes those registers.
//
//go:inline
func mulShift(a, b int32) int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	mult {a}, {b}
	mflo {}
	sra {}, {}, 12
	nop
	.set pop`, map[string]interface{}{"a": a, "b": b}))
}

// RotMatrix makes the rotation by the angles in r in m, about Z first,
// then Y, then X, and returns m. The translation is left alone.
//
//export RotMatrix
func RotMatrix(r *SVECTOR, m *MATRIX) *MATRIX {
	sx, cx := sinCos(int32(r.Vx))
	sy, cy := sinCos(int32(r.Vy))
	sz, cz := sinCos(int32(r.Vz))
	// m is X times Y times Z, each product of two elements back to 4.12
	// by an arithmetic shift, which rounds down. The sign goes on the
	// first factor before the shift, and the products of three take Y
	// with Z first, then X.
	a := mulShift(-sy, cz)
	b := mulShift(-sy, sz)
	m.M[0][0] = int16(mulShift(cy, cz))
	m.M[0][1] = int16(mulShift(-cy, sz))
	m.M[0][2] = int16(sy)
	m.M[1][0] = int16(mulShift(cx, sz) - mulShift(sx, a))
	m.M[1][1] = int16(mulShift(cx, cz) + mulShift(sx, b))
	m.M[1][2] = int16(mulShift(-sx, cy))
	m.M[2][0] = int16(mulShift(sx, sz) + mulShift(cx, a))
	m.M[2][1] = int16(mulShift(sx, cz) - mulShift(cx, b))
	m.M[2][2] = int16(mulShift(cx, cy))
	return m
}

// TransMatrix sets the translation of m to v and returns m.
//
//export TransMatrix
func TransMatrix(m *MATRIX, v *VECTOR) *MATRIX {
	m.T = [3]int32{v.Vx, v.Vy, v.Vz}
	return m
}

// ScaleMatrix scales the rotation of m by v, 4096 being 1: each column j
// by v's element j, so that the scaling comes before m's rotation. Each
// element is shifted back to 4.12, rounding down. It returns m.
//
//export ScaleMatrix
func ScaleMatrix(m *MATRIX, v *VECTOR) *MATRIX {
	s := [3]int32{v.Vx, v.Vy, v.Vz}
	for i := range 3 {
		for j := range 3 {
			m.M[i][j] = int16(int32(m.M[i][j]) * s[j] >> 12)
		}
	}
	return m
}

// MulMatrix0 sets the rotation of m2 to that of m0 times that of m1 on the
// GTE, which it leaves holding m0's, and returns m2. Each element is the
// sum of three products shifted right by 12. The padding halfword becomes 0,
// and the translations are left alone. m2 may be m0 or m1.
//
//export MulMatrix0
func MulMatrix0(m0, m1, m2 *MATRIX) *MATRIX {
	mulRotation(m0, m1, m2)
	return m2
}

// mulRotation is MulMatrix0's product. a goes into the GTE first, so out may
// be a or b. It is inlined, as C programs may give these calls little stack.
//
//go:inline
func mulRotation(a, b, out *MATRIX) {
	setRotation(a)
	mulLoaded(b, out)
}

// mulLoaded is mulRotation with a's rotation already in the GTE. Each column of
// b is read before the same column of out is written.
//
//go:inline
func mulLoaded(b, out *MATRIX) {
	for j := range 3 {
		out.M[0][j], out.M[1][j], out.M[2][j] = rotate(b.M[0][j], b.M[1][j], b.M[2][j])
	}
	*(*uint16)(unsafe.Add(unsafe.Pointer(out), 18)) = 0
}

// MulMatrix makes m0's rotation m0 times m1 on the GTE, which it leaves holding
// m0's old rotation, and returns m0. m0 may be m1.
//
//export MulMatrix
func MulMatrix(m0, m1 *MATRIX) *MATRIX { return MulMatrix0(m0, m1, m0) }

// compose makes out the transformation b and then a: the rotation a.M times
// b.M as MulMatrix0 makes it, and the translation turnLong's result plus a.T,
// all of it wrapping. The GTE's rotation matrix is left holding a's, and the
// rest of it as that last multiply leaves it. Out may be a or b.
//
//go:inline
func compose(a, b, out *MATRIX) {
	setRotation(a)
	tx, ty, tz := turnLong(b.T[0], b.T[1], b.T[2])
	out.T[0], out.T[1], out.T[2] = tx+a.T[0], ty+a.T[1], tz+a.T[2]
	mulLoaded(b, out)
}

// turnLong multiplies the vector x, y, z by the GTE's rotation matrix as the
// coordinate calls do it (spec 19, "Composing"), in two parts: the low 15 bits
// of each element, whose sums are shifted right by 12, and the 16 bits above
// them, whose sums wrap at 32 bits before they go up by 3. Bit 31 plays no
// part. Under 2^30 it is the exact sum over 4096, rounded down.
//
//go:inline
func turnLong(x, y, z int32) (int32, int32, int32) {
	mvmva(int16(x&0x7fff), int16(y&0x7fff), int16(z&0x7fff))
	lo1, lo2, lo3 := mac1(), mac2(), mac3()
	mvmvaWhole(int16(x>>15), int16(y>>15), int16(z>>15))
	return lo1 + mac1()<<3, lo2 + mac2()<<3, lo3 + mac3()<<3
}

// ApplyMatrixLV multiplies the 32-bit vector v0 by the rotation of m on the GTE,
// which it leaves holding m's, into v1, as the coordinate calls multiply (turnLong),
// and returns v1.
//
//export ApplyMatrixLV
func ApplyMatrixLV(m *MATRIX, v0, v1 *VECTOR) *VECTOR {
	setRotation(m)
	v1.Vx, v1.Vy, v1.Vz = turnLong(v0.Vx, v0.Vy, v0.Vz)
	return v1
}

// RotMatrixX turns m about the X axis by r, 4096 to a turn: m becomes the
// turn times m, rows 1 and 2 mixed as RotMatrixY and RotMatrixZ mix
// theirs. It returns m.
//
//export RotMatrixX
func RotMatrixX(r int32, m *MATRIX) *MATRIX {
	s, c := sinCos(r)
	for j := range 3 {
		a, b := int32(m.M[1][j]), int32(m.M[2][j])
		m.M[1][j] = int16((c*a - s*b) >> 12)
		m.M[2][j] = int16((s*a + c*b) >> 12)
	}
	return m
}

// ScaleMatrixL scales the rotation of m by v, 4096 being 1: each row i by
// v's element i, so that the scaling comes after m's rotation. Each
// element is the 32-bit product shifted back to 4.12, rounding down, and
// cut to 16 bits. It returns m.
//
//export ScaleMatrixL
func ScaleMatrixL(m *MATRIX, v *VECTOR) *MATRIX {
	s := [3]int32{v.Vx, v.Vy, v.Vz}
	for i := range 3 {
		for j := range 3 {
			m.M[i][j] = int16(int32(m.M[i][j]) * s[i] >> 12)
		}
	}
	return m
}

// TransposeMatrix writes the transpose of m0's rotation to m1, element by
// element in row order, so that with m1 the same matrix as m0 the lower
// triangle is copied onto the upper and the result is symmetric. The
// translation of m1 is left alone. It returns m1.
//
//export TransposeMatrix
func TransposeMatrix(m0, m1 *MATRIX) *MATRIX {
	for i := range 3 {
		for j := range 3 {
			m1.M[i][j] = m0.M[j][i]
		}
	}
	return m1
}

// ApplyMatrixSV multiplies v0 by the rotation of m on the GTE, which it
// leaves holding m's, into v1: each element shifted right by 12 and
// saturated to 16 bits. v1's Pad is left alone, and v1 may be v0. It
// returns v1, typed as a VECTOR.
//
//export ApplyMatrixSV
func ApplyMatrixSV(m *MATRIX, v0, v1 *SVECTOR) *VECTOR {
	setRotation(m)
	mvmvaAt(uintptr(unsafe.Pointer(v0)))
	v1.Vx, v1.Vy, v1.Vz = ir1(), ir2(), ir3()
	return (*VECTOR)(unsafe.Pointer(v1))
}

// CompMatrix composes m0 and m1 into m2, the transformation m1 and then
// m0: the rotation m0's times m1's, as MulMatrix0 makes it, and the
// translation m0's rotation times m1's, each element of which is cut to
// 16 bits, the GTE's vector, shifted right by 12, plus m0's. m2 may be m0 or
// m1. The GTE is left holding m0's rotation and that translation's last
// multiply. It returns m2.
//
//export CompMatrix
func CompMatrix(m0, m1, m2 *MATRIX) *MATRIX {
	x, y, z := int16(m1.T[0]), int16(m1.T[1]), int16(m1.T[2])
	tx, ty, tz := m0.T[0], m0.T[1], m0.T[2]
	mulRotation(m0, m1, m2)
	mvmva(x, y, z)
	m2.T[0] = mac1() + tx
	m2.T[1] = mac2() + ty
	m2.T[2] = mac3() + tz
	return m2
}
