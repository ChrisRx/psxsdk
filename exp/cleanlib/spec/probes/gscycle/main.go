//go:build yaroze

// Command gscycle is a probe for spec 19: a chain whose parents form a cycle. A cycle
// the walk never leaves hangs the call, so the const which picks the case and "out"
// on the TTY says the call returned.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

const which = 0

var (
	a, b yaroze.GsCOORDINATE2
	lw   yaroze.MATRIX
)

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	yaroze.GsInitCoordinate2(nil, &a)
	yaroze.GsInitCoordinate2(&a, &b)
	a.Coord.T = [3]int32{1, 10, 100}
	b.Coord.T = [3]int32{2, 20, 200}
	println("case", which)
	switch which {
	case 0: // itself, flag 0
		a.Super = &a
		yaroze.GsGetLw(&a, &lw)
	case 1: // two, flags 0
		a.Super = &b
		yaroze.GsGetLw(&b, &lw)
	case 2: // itself, flag C: no walk
		a.Super = &a
		a.Flg = psdCNT
		yaroze.GsGetLw(&a, &lw)
	case 3: // two, the parent flag C: a walk of one
		a.Super = &b
		a.Flg = psdCNT
		yaroze.GsGetLw(&b, &lw)
	case 4: // two, flags x
		a.Super = &b
		a.Flg = psdCNT + 1
		b.Flg = psdCNT + 1
		yaroze.GsGetLw(&b, &lw)
	}
	println("out", "lw", lw.T[0], lw.T[1], lw.T[2], "flags", a.Flg, b.Flg, "cnt", psdCNT)
	println("END")
	for {
	}
}
