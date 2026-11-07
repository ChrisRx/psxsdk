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

// RotMatrix makes the rotation by the angles in r in m, about Z first,
// then Y, then X, and returns m. The translation is left alone. m is X times Y
// times Z, each product of two elements back to 4.12 by an arithmetic shift, which
// rounds down. The sign goes on the first factor before the shift, and the products
// of three take Y with Z first, then X. It is in assembly, with no branch on an
// angle: every call takes the same time on Sony's.
//
//export RotMatrix
func RotMatrix(r *SVECTOR, m *MATRIX) *MATRIX

// TransMatrix sets the translation of m to v and returns m.
//
//export TransMatrix
func TransMatrix(m *MATRIX, v *VECTOR) *MATRIX

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
	// The last element goes as a word, which carries into the padding (spec 32).
	*(*int32)(unsafe.Pointer(&m.M[2][2])) = int32(m.M[2][2])
	space(scaleSpace - 1)
	return m
}

// MulMatrix0 sets the rotation of m2 to that of m0 times that of m1 on the
// GTE, which it leaves holding m0's, and returns m2. Each element is the
// sum of three products shifted right by 12. The last goes as a word, so that
// the padding halfword takes its sign (spec 32), and the translations are left alone. m2 may be m0 or m1. In assembly.
//
//export MulMatrix0
func MulMatrix0(m0, m1, m2 *MATRIX) *MATRIX

// MulMatrix makes m0's rotation m0 times m1 on the GTE, which it leaves holding
// m0's old rotation, and returns m0. m0 may be m1. In assembly.
//
//export MulMatrix
func MulMatrix(m0, m1 *MATRIX) *MATRIX

// compose makes out the transformation b and then a: the rotation a.M times
// b.M as MulMatrix0 makes it, and the translation b.T turned as ApplyMatrixLV
// turns it, plus a.T, all of it wrapping. The GTE's rotation matrix is left holding
// a's, and the rest of it as that last multiply leaves it. Out may be a or b. In assembly.
//
//export libps_compose
func compose(a, b, out *MATRIX)

// ApplyMatrixLV multiplies the 32-bit vector v0 by the rotation of m on the GTE,
// which it leaves holding m's, into v1, as the coordinate calls multiply (spec 19,
// "Composing"), and returns v1: in two parts, the magnitude of each element above
// its low 15 bits, whose sums wrap at 32 bits before they go up by 3, and those low
// bits, whose sums are shifted right by 12, each part with the element's sign. It comes
// to the exact sum over 4096, rounded down, under 2^30. In assembly.
//
//export ApplyMatrixLV
func ApplyMatrixLV(m *MATRIX, v0, v1 *VECTOR) *VECTOR

// RotMatrixX turns m about the X axis by r, 4096 to a turn: m becomes the
// turn times m, rows 1 and 2 mixed as RotMatrixY and RotMatrixZ mix
// theirs, (c*a - s*b) >> 12 and (s*a + c*b) >> 12. It returns m. In assembly, as are
// RotMatrixY and RotMatrixZ.
//
//export RotMatrixX
func RotMatrixX(r int32, m *MATRIX) *MATRIX

// scaleSpace is what ScaleMatrix and ScaleMatrixL spend past their work, for Sony's
// time.
const scaleSpace = 2

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
	space(scaleSpace)
	return m
}

// TransposeMatrix writes the transpose of m0's rotation to m1, element by
// element in row order, so that with m1 the same matrix as m0 the lower
// triangle is copied onto the upper and the result is symmetric. The
// translation of m1 is left alone. It returns m1. In assembly.
//
//export TransposeMatrix
func TransposeMatrix(m0, m1 *MATRIX) *MATRIX

// ApplyMatrixSV multiplies v0 by the rotation of m on the GTE, which it
// leaves holding m's, into v1: each element shifted right by 12 and
// saturated to 16 bits. v1's Pad is left alone, and v1 may be v0. It
// returns v1, typed as a VECTOR. In assembly.
//
//export ApplyMatrixSV
func ApplyMatrixSV(m *MATRIX, v0, v1 *SVECTOR) *VECTOR

// CompMatrix composes m0 and m1 into m2, the transformation m1 and then
// m0: the rotation m0's times m1's, as MulMatrix0 makes it, and the
// translation m0's rotation times m1's, each element of which is cut to
// 16 bits, the GTE's vector, shifted right by 12, plus m0's. m2 may be m0 or
// m1. The GTE is left holding m0's rotation and that translation's last
// multiply. It returns m2. In assembly.
//
//export CompMatrix
func CompMatrix(m0, m1, m2 *MATRIX) *MATRIX
