//go:build yaroze

package libps

//go:generate go run ./gen/sintab

import "unsafe"

// Matrices, as libgte built them without the GTE: 4.12 fixed point, a
// product shifted back by 12.

// mul is the product of two 4.12 numbers in 4.12: the low word of the
// product, as the console's multiply gives it, shifted.
func mul(a, b int32) int32 { return int32(uint32(a)*uint32(b)) >> 12 }

// sinCos is the sine and cosine of an angle in 4096ths of a turn, in
// 4.12, as libgte's rotation table held them: the quarter turn of sines
// folded onto the circle, with the second and fourth quarters a step
// off, which the table had. A negative angle gives the sine negated, and
// is not wrapped, as the original did not.
func sinCos(i int32) (s, c int32) {
	neg := i < 0
	if neg {
		i = -i
		if i < 0 {
			i &= 0xfff
		}
	} else {
		i &= 0xfff
	}
	s = tableSin(i)
	c = tableSin(i + 1024)
	if neg {
		s = -s
	}
	return s, c
}

// tableSin is the rotation table's sine of an angle in 4096ths of a
// turn, from the quarter turn of sines.
func tableSin(a int32) int32 {
	switch a &= 0xfff; {
	case a < 1024:
		return int32(sinTable[a])
	case a < 2048:
		return int32(sinTable[2047-a])
	case a < 3072:
		return -int32(sinTable[a-2048])
	}
	return -int32(sinTable[4095-a])
}

// RotMatrix sets m's rotation to the angles in r, each in 4096ths of a
// turn, applied about Z, then Y, then X, and returns m.
//
// libps 0x8002DEF8, 656 bytes
//
//export RotMatrix
func RotMatrix(r *SVECTOR, m *MATRIX) *MATRIX {
	sx, cx := sinCos(int32(r.Vx))
	sy, cy := sinCos(int32(r.Vy))
	sz, cz := sinCos(int32(r.Vz))
	// A negated product is negated before the shift, as the original
	// did, which rounds the other way from negating the shifted one.
	m.M[0][2] = int16(sy)
	m.M[1][2] = int16(mul(-cy, sx))
	m.M[2][2] = int16(mul(cy, cx))
	m.M[0][0] = int16(mul(cz, cy))
	m.M[0][1] = int16(mul(-sz, cy))
	czsy := mul(cz, -sy)
	m.M[1][0] = int16(mul(sz, cx) - mul(czsy, sx))
	m.M[2][0] = int16(mul(sz, sx) + mul(czsy, cx))
	szsy := mul(sz, -sy)
	m.M[1][1] = int16(mul(cz, cx) + mul(szsy, sx))
	m.M[2][1] = int16(mul(cz, sx) - mul(szsy, cx))
	return m
}

// ScaleMatrix scales m's columns by v, in 4.12, and returns m. The last
// element is written as a whole word, as the original did, so the padding
// after it takes the product's upper half.
//
// libps 0x8002D5B8, 304 bytes
//
//export ScaleMatrix
func ScaleMatrix(m *MATRIX, v *VECTOR) *MATRIX {
	s := [3]int32{v.Vx, v.Vy, v.Vz}
	for i := range m.M {
		for j := range m.M[i] {
			if i == 2 && j == 2 {
				*(*int32)(unsafe.Pointer(&m.M[2][2])) = mul(int32(m.M[2][2]), s[2])
				break
			}
			m.M[i][j] = int16(mul(int32(m.M[i][j]), s[j]))
		}
	}
	return m
}

// TransMatrix sets m's translation to v and returns m.
//
// libps 0x8002D588, 48 bytes
//
//export TransMatrix
func TransMatrix(m *MATRIX, v *VECTOR) *MATRIX {
	m.T = [3]int32{v.Vx, v.Vy, v.Vz}
	return m
}
