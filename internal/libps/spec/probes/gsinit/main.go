//go:build yaroze

// Command gsinit is a probe for spec 19: what sets PSDCNT and GsIDMATRIX, what
// GsInitCoordinate2 does to each field of its system and of the parent, whether the
// library follows Sub, and what a system does before GsInitGraph has run.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern PSDCNT
var psdCNT uint32

//go:extern PSDIDX
var psdIDX int32

//go:extern GsIDMATRIX
var idm yaroze.MATRIX

var (
	a, b, c yaroze.GsCOORDINATE2
	param   yaroze.GsCOORD2PARAM
	lw      yaroze.MATRIX
)

func counters(what string) { println(what, "cnt", psdCNT, "idx", psdIDX) }

func show(m *yaroze.MATRIX) {
	for i := range 3 {
		print(" ", m.M[i][0], " ", m.M[i][1], " ", m.M[i][2])
	}
	print(" pad ", *(*int16)(unsafe.Add(unsafe.Pointer(m), 18)), " t ", m.T[0], " ", m.T[1], " ", m.T[2])
}

func ptr(p unsafe.Pointer) string {
	switch p {
	case nil:
		return "nil"
	case unsafe.Pointer(&a):
		return "a"
	case unsafe.Pointer(&b):
		return "b"
	case unsafe.Pointer(&c):
		return "c"
	case unsafe.Pointer(&param):
		return "param"
	}
	return "other"
}

func node(name string, n *yaroze.GsCOORDINATE2) {
	print(name, " flg ", n.Flg, " param ", ptr(unsafe.Pointer(n.Param)), " super ", ptr(unsafe.Pointer(n.Super)), " sub ", ptr(unsafe.Pointer(n.Sub)), " coord")
	show(&n.Coord)
	print(" workm")
	show(&n.Workm)
	println()
}

func plant(n *yaroze.GsCOORDINATE2) {
	n.Flg = 77
	n.Coord = yaroze.MATRIX{M: [3][3]int16{{9, 9, 9}, {9, 9, 9}, {9, 9, 9}}, T: [3]int32{9, 9, 9}}
	*(*int16)(unsafe.Add(unsafe.Pointer(&n.Coord), 18)) = 9
	n.Workm = yaroze.MATRIX{M: [3][3]int16{{8, 8, 8}, {8, 8, 8}, {8, 8, 8}}, T: [3]int32{8, 8, 8}}
	*(*int16)(unsafe.Add(unsafe.Pointer(&n.Workm), 18)) = 8
	n.Param = &param
	n.Super = &c
	n.Sub = &c
}

func main() {
	counters("start")
	print("idm before GsInitGraph")
	show(&idm)
	println()
	plant(&a)
	plant(&b)
	yaroze.GsInitCoordinate2(nil, &a)
	node("before GsInitGraph, nil super: a", &a)
	yaroze.ResetGraph(0)
	counters("ResetGraph")
	print("idm after ResetGraph")
	show(&idm)
	println()
	// A chain before GsInitGraph, when PSDCNT is 0 so that a fresh flag reads as
	// current, and with what the identity is then.
	yaroze.GsInitCoordinate2(nil, &a)
	yaroze.GsInitCoordinate2(&a, &b)
	a.Coord.T = [3]int32{1, 10, 100}
	b.Coord.T = [3]int32{2, 20, 200}
	a.Workm = yaroze.MATRIX{T: [3]int32{-1, -1, -1}}
	b.Workm = yaroze.MATRIX{T: [3]int32{-2, -2, -2}}
	lw = yaroze.MATRIX{}
	yaroze.GsGetLw(&b, &lw)
	print("chain at cnt 0, flags 0 0: lw")
	show(&lw)
	println(" flags", a.Flg, b.Flg)

	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	counters("GsInitGraph")
	print("idm after GsInitGraph")
	show(&idm)
	println()
	yaroze.GsInit3D()
	counters("GsInit3D")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	counters("GsDefDispBuff")
	yaroze.GsSwapDispBuff()
	counters("swap")
	psdCNT = 100
	yaroze.GsInit3D()
	counters("GsInit3D at 100")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	counters("GsDefDispBuff at 100")
	yaroze.ResetGraph(0)
	counters("ResetGraph(0) at 100")
	yaroze.ResetGraph(1)
	counters("ResetGraph(1) at 100")
	yaroze.VSync(0)
	counters("VSync at 100")
	yaroze.GsSetDrawBuffOffset()
	counters("GsSetDrawBuffOffset at 100")
	yaroze.DrawSync(0)
	counters("DrawSync at 100")
	psdIDX = 0
	yaroze.GsSwapDispBuff()
	counters("swap from 100, idx 0")
	yaroze.GsSwapDispBuff()
	counters("swap")
	psdCNT = 0x7fffffff
	yaroze.GsSwapDispBuff()
	counters("swap from 7fffffff")
	psdCNT = 0xffffffff
	yaroze.GsSwapDispBuff()
	counters("swap from ffffffff")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	counters("second GsInitGraph, from 1")
	psdCNT = 100
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGTE, 1, 0)
	counters("third GsInitGraph, from 100")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()

	// GsInitCoordinate2's fields.
	plant(&a)
	plant(&b)
	plant(&c)
	yaroze.GsInitCoordinate2(nil, &a)
	node("nil super: a", &a)
	node("c", &c)
	plant(&a)
	yaroze.GsInitCoordinate2(&b, &a)
	node("super b: a", &a)
	node("b", &b)
	// A second child: the parent's Sub moves, and the first child is not touched.
	plant(&c)
	yaroze.GsInitCoordinate2(&b, &c)
	node("second child c", &c)
	node("b", &b)
	node("a", &a)

	// The identity comes from GsIDMATRIX, or not.
	idm0 := idm
	idm = yaroze.MATRIX{M: [3][3]int16{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, T: [3]int32{11, 12, 13}}
	*(*int16)(unsafe.Add(unsafe.Pointer(&idm), 18)) = 14
	plant(&a)
	yaroze.GsInitCoordinate2(nil, &a)
	node("with idm changed: a", &a)
	idm = idm0
	print("idm back")
	show(&idm)
	println()

	// Sub: a chain where each Sub points elsewhere, and a child reached with a Sub
	// set to the wrong system.
	yaroze.GsInitCoordinate2(nil, &a)
	yaroze.GsInitCoordinate2(&a, &b)
	yaroze.GsInitCoordinate2(&b, &c)
	a.Coord.T = [3]int32{1, 10, 100}
	b.Coord.T = [3]int32{2, 20, 200}
	c.Coord.T = [3]int32{4, 40, 400}
	a.Sub = &c
	b.Sub = nil
	c.Sub = &a
	yaroze.GsGetLw(&c, &lw)
	node("after Lw with wrong Subs: a", &a)
	node("b", &b)
	node("c", &c)
	print("lw")
	show(&lw)
	println()

	// The next call after a change in a system at each place, in the same frame, as
	// a program would make them, and in the next.
	a.Coord.T[0] += 1000
	a.Flg = 0
	yaroze.GsGetLw(&c, &lw)
	println("same frame, a moved, flag 0, target C: lw", lw.T[0], "flags", a.Flg, b.Flg, c.Flg)
	yaroze.GsSwapDispBuff()
	yaroze.GsGetLw(&c, &lw)
	println("next frame, nothing moved: lw", lw.T[0], "flags", a.Flg-psdCNT, b.Flg-psdCNT, c.Flg-psdCNT, "lag")
	a.Flg = 0
	yaroze.GsGetLw(&c, &lw)
	println("a moved: lw", lw.T[0], "flags", a.Flg-psdCNT, b.Flg-psdCNT, c.Flg-psdCNT, "lag")
	yaroze.GsSwapDispBuff()
	b.Coord.T[0] += 1000
	b.Flg = 0
	yaroze.GsGetLw(&a, &lw)
	println("b moved, asked of a: lw", lw.T[0], "flags", a.Flg-psdCNT, b.Flg-psdCNT, c.Flg-psdCNT, "lag")
	yaroze.GsGetLw(&c, &lw)
	println("then of c: lw", lw.T[0], "flags", a.Flg-psdCNT, b.Flg-psdCNT, c.Flg-psdCNT, "lag")
	// Does the padding halfword go with the matrices?
	yaroze.GsInitCoordinate2(nil, &a)
	yaroze.GsInitCoordinate2(&a, &b)
	*(*int16)(unsafe.Add(unsafe.Pointer(&a.Coord), 18)) = 21
	*(*int16)(unsafe.Add(unsafe.Pointer(&a.Workm), 18)) = 22
	*(*int16)(unsafe.Add(unsafe.Pointer(&b.Coord), 18)) = 23
	*(*int16)(unsafe.Add(unsafe.Pointer(&b.Workm), 18)) = 24
	*(*int16)(unsafe.Add(unsafe.Pointer(&lw), 18)) = 25
	a.Flg, b.Flg = 0, 0
	yaroze.GsGetLw(&b, &lw)
	node("pads: a", &a)
	node("b", &b)
	print("lw")
	show(&lw)
	println()
	println("END")
	for {
	}
}
