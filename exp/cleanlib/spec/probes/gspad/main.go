//go:build yaroze

// Command gspad is a probe for spec 19: the padding halfword of the matrices the
// coordinate calls write, and what they do when an output is an input: the system's
// own Workm or Coord, GsGetLws with its two outputs the same, and CompMatrix and
// MulMatrix0 on their own inputs.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern PSDCNT
var psdCNT uint32

var (
	a, b   yaroze.GsCOORDINATE2
	lw, ls yaroze.MATRIX
	m0, m1 yaroze.MATRIX
	m2     yaroze.MATRIX
	v2     yaroze.GsVIEW2
)

func pad(m *yaroze.MATRIX) int16 { return *(*int16)(unsafe.Add(unsafe.Pointer(m), 18)) }

func plant(m *yaroze.MATRIX, v int16) { *(*int16)(unsafe.Add(unsafe.Pointer(m), 18)) = v }

func show(name string, m *yaroze.MATRIX) {
	print(name)
	for i := range 3 {
		print(" ", m.M[i][0], " ", m.M[i][1], " ", m.M[i][2])
	}
	println(" pad", pad(m), "t", m.T[0], m.T[1], m.T[2])
}

// turned gives a matrix with every element and translation its own.
func turned(m *yaroze.MATRIX, base int16, p int16) {
	for i := range 3 {
		for j := range 3 {
			m.M[i][j] = base + int16(i*3+j)
		}
		m.T[i] = int32(base)*1000 + int32(i)
	}
	plant(m, p)
}

// chain sets a and b up with their own matrices and planted paddings, the flags given.
func chain(fa, fb uint32) {
	yaroze.GsInitCoordinate2(nil, &a)
	yaroze.GsInitCoordinate2(&a, &b)
	turned(&a.Coord, 300, 21)
	turned(&a.Workm, 500, 22)
	turned(&b.Coord, 700, 23)
	turned(&b.Workm, 900, 24)
	a.Flg, b.Flg = fa, fb
	turned(&lw, 1100, 25)
	turned(&ls, 1300, 26)
}

func state() {
	show(" a workm", &a.Workm)
	show(" b workm", &b.Workm)
	show(" b coord", &b.Coord)
	println(" flags a", a.Flg == psdCNT, a.Flg, "b", b.Flg == psdCNT, b.Flg)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	yaroze.GsSwapDispBuff()
	x := psdCNT + 5
	turned(yaroze.GsWSMATRIX(), 1500, 27)

	// Where the padding goes: every flag of a root and a child, each output.
	for _, c := range [...]struct {
		name   string
		fa, fb uint32
	}{{"00", 0, 0}, {"CC", psdCNT, psdCNT}, {"xx", x, x}, {"x0", x, 0}, {"0x", 0, x}} {
		println("flags", c.name)
		chain(c.fa, c.fb)
		yaroze.GsGetLw(&a, &lw)
		show(" lw of root", &lw)
		chain(c.fa, c.fb)
		yaroze.GsGetLw(&b, &lw)
		show(" lw of child", &lw)
		chain(c.fa, c.fb)
		yaroze.GsGetLs(&a, &ls)
		show(" ls of root", &ls)
		chain(c.fa, c.fb)
		yaroze.GsGetLs(&b, &ls)
		show(" ls of child", &ls)
		chain(c.fa, c.fb)
		yaroze.GsGetLws(&b, &lw, &ls)
		show(" lws lw", &lw)
		show(" lws ls", &ls)
		state()
	}

	// The views' padding.
	turned(&v2.View, 1700, 28)
	v2.Super = nil
	yaroze.GsSetView2(&v2)
	show("view world", yaroze.GsWSMATRIX())
	chain(0, 0)
	v2.Super = &b
	yaroze.GsSetView2(&v2)
	show("view chain", yaroze.GsWSMATRIX())

	// An output that is the system's own matrices.
	println("lw is own workm")
	chain(0, 0)
	yaroze.GsGetLw(&b, &b.Workm)
	state()
	chain(x, x)
	yaroze.GsGetLw(&b, &b.Workm)
	state()
	println("ls is own workm")
	chain(0, 0)
	yaroze.GsGetLs(&b, &b.Workm)
	state()
	chain(x, x)
	yaroze.GsGetLs(&b, &b.Workm)
	state()
	println("lw is own coord")
	chain(0, 0)
	yaroze.GsGetLw(&b, &b.Coord)
	state()
	println("ls is own coord")
	chain(0, 0)
	yaroze.GsGetLs(&b, &b.Coord)
	state()
	println("lws, the same output")
	chain(0, 0)
	yaroze.GsGetLws(&b, &lw, &lw)
	show(" out", &lw)
	state()
	println("lws, lw is own workm")
	chain(0, 0)
	yaroze.GsGetLws(&b, &b.Workm, &ls)
	show(" ls", &ls)
	state()
	println("lws, ls is own workm")
	chain(0, 0)
	yaroze.GsGetLws(&b, &lw, &b.Workm)
	show(" lw", &lw)
	state()

	// The matrix calls on their own inputs, with the padding planted.
	fresh := func() {
		m0 = yaroze.MATRIX{M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}}, T: [3]int32{5, 6, 7}}
		m1 = yaroze.MATRIX{M: [3][3]int16{{1, -1, 3}, {-3, 5, -7}, {4095, -4095, 1}}, T: [3]int32{-1000, -2000, -3000}}
		plant(&m0, 31)
		plant(&m1, 32)
		m2 = yaroze.MATRIX{M: [3][3]int16{{9, 9, 9}, {9, 9, 9}, {9, 9, 9}}, T: [3]int32{9, 9, 9}}
		plant(&m2, 33)
	}
	fresh()
	yaroze.MulMatrix0(&m0, &m1, &m2)
	show("MulMatrix0", &m2)
	fresh()
	yaroze.MulMatrix0(&m0, &m1, &m0)
	show("MulMatrix0 on m0", &m0)
	fresh()
	yaroze.MulMatrix0(&m0, &m1, &m1)
	show("MulMatrix0 on m1", &m1)
	fresh()
	yaroze.MulMatrix0(&m0, &m0, &m0)
	show("MulMatrix0 all m0", &m0)
	fresh()
	yaroze.CompMatrix(&m0, &m1, &m2)
	show("CompMatrix", &m2)
	fresh()
	yaroze.CompMatrix(&m0, &m1, &m0)
	show("CompMatrix on m0", &m0)
	fresh()
	yaroze.CompMatrix(&m0, &m1, &m1)
	show("CompMatrix on m1", &m1)
	fresh()
	yaroze.CompMatrix(&m0, &m0, &m0)
	show("CompMatrix all m0", &m0)
	println("END")
	for {
	}
}
