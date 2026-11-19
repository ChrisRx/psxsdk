//go:build yaroze

// Command refview is a probe for the spec: GsSetRefView2 on views from a
// fixed pseudo-random sequence, printing GsWSMATRIX after each, so builds
// for Sony's library and the clean one can be compared line by line.
package main

import "go.chrisrx.dev/psx/yaroze"

var seed uint32 = 777

func next(lo, hi int32) int32 {
	seed = seed*1103515245 + 12345
	return lo + int32(seed>>8)%(hi-lo)
}

var view yaroze.GsRVIEW2

func main() {
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsInit3D()
	for i := range 400 {
		r := int32(30000)
		if i%4 == 1 {
			r = 2000
		}
		if i%4 == 2 {
			r = 300
		}
		view = yaroze.GsRVIEW2{
			Vpx: next(-r, r), Vpy: next(-r, r), Vpz: next(-r, r),
			Vrx: next(-r, r), Vry: next(-r, r), Vrz: next(-r, r),
			Super: yaroze.WORLD,
		}
		if i%4 == 3 {
			view.Vry = view.Vpy + next(-50, 50)
		}
		yaroze.GsSetRefView2(&view)
		m := yaroze.GsWSMATRIX()
		print("view ", i, ":")
		for r := range 3 {
			for c := range 3 {
				print(" ", m.M[r][c])
			}
		}
		println(" t", m.T[0], m.T[1], m.T[2])
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
