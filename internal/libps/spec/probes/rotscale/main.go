//go:build yaroze

// Command rotscale is a probe for the spec: RotMatrix on angles from a
// fixed pseudo-random sequence, then ScaleMatrix on the result, printing
// both matrices for each case, so builds for Sony's library and the clean
// one can be compared line by line.
package main

import "go.chrisrx.dev/psx/yaroze"

var seed uint32 = 12345

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
	println()
}

func main() {
	var rot yaroze.SVECTOR
	var scale yaroze.VECTOR
	var m yaroze.MATRIX
	for i := range 300 {
		rot.Vx = int16(next(-4096, 4096))
		rot.Vy = int16(next(-4096, 4096))
		rot.Vz = int16(next(-4096, 4096))
		if i%3 == 0 {
			rot.Vx, rot.Vz = 0, 0
		}
		s := next(1024, 16384)
		scale = yaroze.VECTOR{Vx: s, Vy: s, Vz: s}
		if i%2 == 1 {
			scale.Vy = next(1024, 16384)
			scale.Vz = next(1024, 16384)
		}
		yaroze.RotMatrix(&rot, &m)
		show("rot", i, &m)
		yaroze.ScaleMatrix(&m, &scale)
		show("scaled", i, &m)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
