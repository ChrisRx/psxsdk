//go:build yaroze

// Command gsrz is a probe for spec 19: how GsSetRefView2 takes its twist, Rz. On the
// two points the same its matrix is GsIDMATRIX2's with the twist alone, which shows the
// angle's units and rounding; then random views with a twist, with GsIDMATRIX2 as it
// is and changed, and random views without one on the changed GsIDMATRIX2.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern GsIDMATRIX2
var idm2 yaroze.MATRIX

var (
	rv   yaroze.GsRVIEW2
	seed uint32 = 777
)

func next(lo, hi int32) int32 {
	seed = seed*1103515245 + 12345
	return lo + int32((seed>>7)%uint32(hi-lo+1))
}

func show(m *yaroze.MATRIX) {
	for i := range 3 {
		print(" ", m.M[i][0], " ", m.M[i][1], " ", m.M[i][2])
	}
	print(" t ", m.T[0], " ", m.T[1], " ", m.T[2])
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	for _, rz := range [...]int32{
		0, 1, 89, 90, 179, 180, 181, 359, 360, 361, 539, 540, 719, 720, 721, 3600, 4096, 45 * 4096, 90 * 4096,
		180 * 4096, 270 * 4096, 360 * 4096, 450 * 4096, 720 * 4096, 1 << 24, 1 << 28, 0x7fffffff,
		-1, -89, -90, -179, -180, -181, -359, -360, -361, -539, -540, -719, -720, -721, -3600, -4096, -45 * 4096,
		-90 * 4096, -180 * 4096, -360 * 4096, -450 * 4096, -(1 << 24), -(1 << 28), -0x80000000,
	} {
		rv = yaroze.GsRVIEW2{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 2, Vrz: 3, Rz: rz}
		r := yaroze.GsSetRefView2(&rv)
		print("rz ", rz, " ret ", r, " ws")
		show(yaroze.GsWSMATRIX())
		println()
	}
	for i := range 200 {
		if i == 100 {
			idm2.M = [3][3]int16{{4096, 0, 0}, {0, 2048, 0}, {0, 0, 4096}}
			idm2.T = [3]int32{5, 6, 7}
		}
		rv = yaroze.GsRVIEW2{
			Vpx: next(-3000, 3000), Vpy: next(-3000, 3000), Vpz: next(-3000, 3000),
			Vrx: next(-3000, 3000), Vry: next(-3000, 3000), Vrz: next(-3000, 3000),
			Rz: next(-4096*720, 4096*720),
		}
		r := yaroze.GsSetRefView2(&rv)
		print("view rz ", rv.Rz, " ret ", r, " ws")
		show(yaroze.GsWSMATRIX())
		println()
	}
	// No twist, with GsIDMATRIX2 still changed: the order of its product with the turns.
	for range 100 {
		rv = yaroze.GsRVIEW2{
			Vpx: next(-3000, 3000), Vpy: next(-3000, 3000), Vpz: next(-3000, 3000),
			Vrx: next(-3000, 3000), Vry: next(-3000, 3000), Vrz: next(-3000, 3000),
		}
		r := yaroze.GsSetRefView2(&rv)
		print("plain view ret ", r, " ws")
		show(yaroze.GsWSMATRIX())
		println()
	}
	println("END")
	for {
	}
}
