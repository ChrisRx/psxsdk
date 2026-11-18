//go:build yaroze

// Command gscnt is a probe for spec 19: what PSDCNT and PSDIDX hold after each
// start-up call and across swaps, and which values of Flg make GsGetLw work a chain
// out, with PSDCNT at various values. Each flag is printed against PSDCNT.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

//go:extern PSDIDX
var psdIDX int32

var (
	chain [8]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
)

func counters(what string) {
	println(what, "cnt", psdCNT, "idx", psdIDX)
}

// build makes a chain of d with the flags given, and each Workm planted with -1 in
// translation and a zero rotation.
func build(d int, flags ...uint32) *yaroze.GsCOORDINATE2 {
	for i := 0; i < d; i++ {
		var up *yaroze.GsCOORDINATE2
		if i > 0 {
			up = &chain[i-1]
		}
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord.T = [3]int32{int32(i + 1), int32(10 * (i + 1)), int32(100 * (i + 1))}
		chain[i].Flg = flags[i]
		chain[i].Workm = yaroze.MATRIX{T: [3]int32{-1, -1, -1}}
	}
	return &chain[d-1]
}

func state(d int) {
	for i := 0; i < d; i++ {
		println("  node", i, "flg", chain[i].Flg, "work", chain[i].Workm.M[0][0], chain[i].Workm.T[0], chain[i].Workm.T[1], chain[i].Workm.T[2])
	}
	println("  lw", lw.M[0][0], lw.T[0], lw.T[1], lw.T[2])
}

func patterns(a, b uint32) {
	println("patterns, a", a, "b", b, "cnt", psdCNT)
	for pat := uint32(0); pat < 8; pat++ {
		fl := [3]uint32{}
		for i := range 3 {
			if pat>>i&1 != 0 {
				fl[i] = b
			} else {
				fl[i] = a
			}
		}
		c := build(3, fl[0], fl[1], fl[2])
		lw = yaroze.MATRIX{}
		yaroze.GsGetLw(c, &lw)
		println("pat", pat, "flags", fl[0], fl[1], fl[2])
		state(3)
	}
}

func main() {
	counters("start")
	yaroze.ResetGraph(0)
	counters("ResetGraph")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	counters("GsInitGraph")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	counters("GsDefDispBuff")
	yaroze.GsInit3D()
	counters("GsInit3D")
	patterns(psdCNT, psdCNT+1)
	for i := range 5 {
		yaroze.VSync(0)
		counters("VSync")
		yaroze.GsSwapDispBuff()
		counters("swap")
		_ = i
	}
	patterns(psdCNT, 0)
	patterns(psdCNT, psdCNT+1)
	patterns(psdCNT, psdCNT-1)
	psdCNT = 100
	counters("set")
	patterns(100, 0)
	patterns(100, 99)
	yaroze.GsSwapDispBuff()
	counters("swap after set")
	psdCNT = 0xffffffff
	yaroze.GsSwapDispBuff()
	counters("swap at -1")
	yaroze.GsSwapDispBuff()
	counters("swap at 0")
	println("END")
	for {
	}
}
