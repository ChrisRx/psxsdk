//go:build yaroze

// Command matrix is a probe for the spec: ApplyMatrixSV, CompMatrix,
// RotMatrixX, ScaleMatrixL and TransposeMatrix over fixed edge cases and
// a pseudo-random sequence, printing every output (and whether each
// returns its output argument), so builds for Sony's library and the
// clean one can be compared line by line.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var seed uint32 = 4242

func next(lo, hi int32) int32 {
	seed = seed*1103515245 + 12345
	return lo + int32(seed>>8)%(hi-lo)
}

func show(what string, i int, m *yaroze.MATRIX) {
	print(what, " ", i, ":")
	for r := range 3 {
		for c := range 3 {
			print(" ", m.M[r][c])
		}
	}
	print(" |")
	for r := range 3 {
		print(" ", m.T[r])
	}
	println()
}

// edges are matrix elements the random cases mix in.
var edges = []int16{0, 1, -1, 4096, -4096, 32767, -32768, 16384, -16384, 2048, 4095}

// randMatrix fills m with elements from edges or random ones, and a
// translation of n bits.
func randMatrix(m *yaroze.MATRIX, bits uint) {
	for r := range 3 {
		for c := range 3 {
			if next(0, 4) == 0 {
				m.M[r][c] = edges[next(0, int32(len(edges)))]
			} else {
				m.M[r][c] = int16(next(-32768, 32768))
			}
		}
		if bits >= 32 {
			m.T[r] = int32(next(-32768, 32768))<<16 | next(0, 65536)
		} else {
			lim := int32(1) << (bits - 1)
			m.T[r] = next(-lim, lim)
		}
	}
}

func randSV(v *yaroze.SVECTOR) {
	for _, p := range []*int16{&v.Vx, &v.Vy, &v.Vz} {
		if next(0, 4) == 0 {
			*p = edges[next(0, int32(len(edges)))]
		} else {
			*p = int16(next(-32768, 32768))
		}
	}
	v.Pad = int16(next(-32768, 32768))
}

// ret is "arg" when a call returned its output argument p, and the
// value it returned otherwise.
func ret(r, p unsafe.Pointer) string {
	if r == p {
		return "arg"
	}
	const hex = "0123456789abcdef"
	var b [8]byte
	for i := range b {
		b[i] = hex[uintptr(r)>>(28-4*i)&15]
	}
	return string(b[:])
}

func identity() yaroze.MATRIX {
	return yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}}
}

func main() {
	var m, m1, m2 yaroze.MATRIX

	// RotMatrixX over angles, on the identity and on random matrices.
	angles := []int32{0, 1, -1, 512, 1024, 2048, 3072, 4095, 4096, 4097, -4096, -4097, 8192, 10000, -10000, 65536, 1 << 20, -(1 << 20), 0x7fffffff, -0x80000000, 300, -300, 1023, 1025, 2047, 2049}
	for i, a := range angles {
		m = identity()
		m.T = [3]int32{1, 2, 3}
		r := yaroze.RotMatrixX(a, &m)
		print("rotx-id ", a, " ret ", ret(unsafe.Pointer(r), unsafe.Pointer(&m)), " ")
		show("", i, &m)
	}
	for i := range 80 {
		randMatrix(&m, 32)
		a := next(-20000, 20000)
		if i < len(angles) {
			a = angles[i]
		}
		yaroze.RotMatrixX(a, &m)
		print("rotx ", a, " ")
		show("", i, &m)
	}

	// ScaleMatrixL on random matrices and scales.
	scales := []int32{0, 1, -1, 4096, -4096, 8192, 1 << 16, 1 << 20, -(1 << 20), 0x7fffffff, -0x80000000, 2048, 4095}
	for i := range 80 {
		randMatrix(&m, 32)
		var v yaroze.VECTOR
		for _, p := range []*int32{&v.Vx, &v.Vy, &v.Vz} {
			if next(0, 3) == 0 {
				*p = scales[next(0, int32(len(scales)))]
			} else {
				*p = next(-40000, 40000)
			}
		}
		v.Pad = 12345
		r := yaroze.ScaleMatrixL(&m, &v)
		print("scalel ", v.Vx, " ", v.Vy, " ", v.Vz, " ret ", ret(unsafe.Pointer(r), unsafe.Pointer(&m)), " ")
		show("", i, &m)
	}

	// TransposeMatrix, into another matrix and in place.
	for i := range 20 {
		randMatrix(&m, 32)
		m1 = yaroze.MATRIX{}
		m1.T = [3]int32{7, 8, 9}
		r := yaroze.TransposeMatrix(&m, &m1)
		print("transpose ret ", ret(unsafe.Pointer(r), unsafe.Pointer(&m1)), " ")
		show("", i, &m1)
		if i%4 == 0 {
			yaroze.TransposeMatrix(&m, &m)
			show("transpose-self", i, &m)
		}
	}

	// ApplyMatrixSV.
	for i := range 80 {
		randMatrix(&m, 32)
		if i%3 == 0 {
			for r := range 3 {
				for c := range 3 {
					m.M[r][c] = int16(next(-4096, 4097))
				}
			}
		}
		var v0, v1 yaroze.SVECTOR
		randSV(&v0)
		v1.Pad = 0x5555
		r := yaroze.ApplyMatrixSV(&m, &v0, &v1)
		println("applysv", i, ":", v0.Vx, v0.Vy, v0.Vz, "->", v1.Vx, v1.Vy, v1.Vz, v1.Pad, "ret", ret(unsafe.Pointer(r), unsafe.Pointer(&v1)))
		if i%10 == 0 {
			// In place.
			yaroze.ApplyMatrixSV(&m, &v0, &v0)
			println("applysv-self", i, ":", v0.Vx, v0.Vy, v0.Vz, v0.Pad)
		}
	}

	// CompMatrix: translations of 16 bits and of more.
	for i := range 80 {
		bits := uint(16)
		if i%4 == 3 {
			bits = 32
		} else if i%4 == 2 {
			bits = 20
		}
		randMatrix(&m, 32)
		randMatrix(&m1, bits)
		if i%3 == 0 {
			for r := range 3 {
				for c := range 3 {
					m.M[r][c] = int16(next(-4096, 4097))
					m1.M[r][c] = int16(next(-4096, 4097))
				}
			}
		}
		if i == 0 {
			m, m1 = identity(), identity()
		}
		m2 = yaroze.MATRIX{}
		r := yaroze.CompMatrix(&m, &m1, &m2)
		print("comp ret ", ret(unsafe.Pointer(r), unsafe.Pointer(&m2)), " ")
		show("", i, &m2)
		if i%8 == 1 {
			save := m1
			yaroze.CompMatrix(&m, &m1, &m1)
			show("comp-m2=m1", i, &m1)
			m1 = save
			yaroze.CompMatrix(&m, &m1, &m)
			show("comp-m2=m0", i, &m)
		}
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
