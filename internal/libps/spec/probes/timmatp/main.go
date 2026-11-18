//go:build yaroze

// Command matp prints the halves after each matrix's last element as the matrix calls leave them.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func pad(m *yaroze.MATRIX) *uint16 { return (*uint16)(unsafe.Add(unsafe.Pointer(m), 18)) }

func show(s string, m *yaroze.MATRIX) {
	print(s, " ", m.M[2][2], " ", *pad(m), "\n")
}

func fresh() *yaroze.MATRIX {
	m := &yaroze.MATRIX{}
	*pad(m) = 0x1234
	return m
}

func main() {
	yaroze.ResetGraph(0)
	for _, a := range []int16{0, 100, 1024, 2000, 3000} {
		v := yaroze.SVECTOR{Vx: a, Vy: a / 2, Vz: a / 3}
		m := fresh()
		yaroze.RotMatrix(&v, m)
		show("RotMatrix", m)
		m = fresh()
		m.M[2][2] = 4096
		yaroze.RotMatrixX(int32(a), m)
		show("RotMatrixX", m)
		m = fresh()
		m.M[2][2] = 4096
		yaroze.RotMatrixY(int32(a), m)
		show("RotMatrixY", m)
		m = fresh()
		m.M[2][2] = 4096
		yaroze.RotMatrixZ(int32(a), m)
		show("RotMatrixZ", m)
		m = fresh()
		k := yaroze.MATRIX{}
		k.M[2][2] = a - 1000
		k.M[0][0] = 4096
		m2 := fresh()
		m2.M[2][2] = 2048
		yaroze.MulMatrix0(&k, m2, m)
		show("MulMatrix0", m)
		m = fresh()
		m.M[2][2] = a - 1000
		yaroze.TransposeMatrix(m, m2)
		show("Transpose", m2)
		m = fresh()
		m.M[2][2] = a - 1000
		vv := yaroze.VECTOR{Vx: 4096, Vy: 4096, Vz: 8192}
		yaroze.ScaleMatrix(m, &vv)
		show("Scale", m)
	}
	print("END\n")
	for {
		yaroze.VSync(0)
	}
}
