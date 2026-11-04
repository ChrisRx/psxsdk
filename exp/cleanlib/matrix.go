//go:build yaroze

package cleanlib

import "unsafe"

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

// RotMatrix makes the rotation by the angles in r in m, about Z first,
// then Y, then X, and returns m. The translation is left alone.
//
//export RotMatrix
func RotMatrix(r *SVECTOR, m *MATRIX) *MATRIX {
	sx, cx := sin(int32(r.Vx)), cos(int32(r.Vx))
	sy, cy := sin(int32(r.Vy)), cos(int32(r.Vy))
	sz, cz := sin(int32(r.Vz)), cos(int32(r.Vz))
	// m is X times Y times Z, each product of two elements back to 4.12
	// by an arithmetic shift, which rounds down. The sign goes on the
	// first factor before the shift, and the products of three take Y
	// with Z first, then X.
	a := -sy * cz >> 12
	b := -sy * sz >> 12
	m.M[0][0] = int16(cy * cz >> 12)
	m.M[0][1] = int16(-cy * sz >> 12)
	m.M[0][2] = int16(sy)
	m.M[1][0] = int16(cx*sz>>12 - sx*a>>12)
	m.M[1][1] = int16(cx*cz>>12 + sx*b>>12)
	m.M[1][2] = int16(-sx * cy >> 12)
	m.M[2][0] = int16(sx*sz>>12 + cx*a>>12)
	m.M[2][1] = int16(sx*cz>>12 - cx*b>>12)
	m.M[2][2] = int16(cx * cy >> 12)
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
// sum of three products shifted right by 12. The translations are left
// alone. m2 may be m1.
//
//export MulMatrix0
func MulMatrix0(m0, m1, m2 *MATRIX) *MATRIX {
	SetRotMatrix(m0)
	var r [3][3]int16
	for j := range 3 {
		r[0][j], r[1][j], r[2][j] = rotate(m1.M[0][j], m1.M[1][j], m1.M[2][j])
	}
	m2.M = r
	return m2
}

// compose returns the transformation b and then a: the rotation a.M times
// b.M, and the translation a.M times b.T plus a.T. The GTE's rotation
// matrix is left holding a's.
func compose(a, b *MATRIX) MATRIX {
	var m MATRIX
	MulMatrix0(a, b, &m)
	t := applyLong(a, b.T)
	m.T = [3]int32{t[0] + a.T[0], t[1] + a.T[1], t[2] + a.T[2]}
	return m
}

// applyLong multiplies the 32-bit vector v by the rotation of m, each
// element a sum of three products shifted right by 12.
func applyLong(m *MATRIX, v [3]int32) [3]int32 {
	var r [3]int32
	for i := range 3 {
		var sum int64
		for k := range 3 {
			sum += int64(m.M[i][k]) * int64(v[k])
		}
		r[i] = int32(sum >> 12)
	}
	return r
}

// ApplyMatrixLV multiplies the 32-bit vector v0 by the rotation of m into
// v1, each element a sum of three products shifted right by 12, and
// returns v1.
//
//export ApplyMatrixLV
func ApplyMatrixLV(m *MATRIX, v0, v1 *VECTOR) *VECTOR {
	r := applyLong(m, [3]int32{v0.Vx, v0.Vy, v0.Vz})
	v1.Vx, v1.Vy, v1.Vz = r[0], r[1], r[2]
	return v1
}

// RotMatrixX turns m about the X axis by r, 4096 to a turn: m becomes the
// turn times m, rows 1 and 2 mixed as RotMatrixY and RotMatrixZ mix
// theirs. It returns m.
//
//export RotMatrixX
func RotMatrixX(r int32, m *MATRIX) *MATRIX {
	s, c := sin(r), cos(r)
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
	SetRotMatrix(m)
	v1.Vx, v1.Vy, v1.Vz = rotate(v0.Vx, v0.Vy, v0.Vz)
	return (*VECTOR)(unsafe.Pointer(v1))
}

// CompMatrix composes m0 and m1 into m2, the transformation m1 and then
// m0: the rotation m0's times m1's, as MulMatrix0 makes it, and the
// translation m0's rotation times m1's, each element of which is cut to
// 16 bits, shifted right by 12, plus m0's. m2 may be m0 or m1. The GTE
// is left holding m0's rotation. It returns m2.
//
//export CompMatrix
func CompMatrix(m0, m1, m2 *MATRIX) *MATRIX {
	var t [3]int32
	for i := range 3 {
		var sum int64
		for k := range 3 {
			sum += int64(m0.M[i][k]) * int64(int16(m1.T[k]))
		}
		t[i] = int32(sum>>12) + m0.T[i]
	}
	MulMatrix0(m0, m1, m2)
	m2.T = t
	return m2
}
