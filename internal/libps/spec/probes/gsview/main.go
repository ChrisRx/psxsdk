//go:build yaroze

// Command gsview is a probe for spec 19: how GsSetView2 and GsSetRefView2 use a
// coordinate chain. For every flag combination over a chain of three, it prints the
// flags and each Workm's translation after the call and the world-to-screen matrix;
// then random systems, to find how the view is combined with the system's inverse.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

//go:extern GsIDMATRIX2
var idm2 yaroze.MATRIX

var (
	chain [3]yaroze.GsCOORDINATE2
	rv    yaroze.GsRVIEW2
	v2    yaroze.GsVIEW2
	seed  uint32 = 4242
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

func setup(code int, x uint32) *yaroze.GsCOORDINATE2 {
	var up *yaroze.GsCOORDINATE2
	for i := range 3 {
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord.T = [3]int32{2 * int32(i+1), 0, 0}
		switch code % 3 {
		case 0:
			chain[i].Flg = 0
		case 1:
			chain[i].Flg = psdCNT
		case 2:
			chain[i].Flg = x
		}
		code /= 3
		chain[i].Workm = yaroze.MATRIX{
			M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}},
			T: [3]int32{1000 * int32(i+1), 0, 0},
		}
		up = &chain[i]
	}
	return up
}

func line(code int, x uint32, r int32) {
	for range 3 {
		switch code % 3 {
		case 0:
			print("0")
		case 1:
			print("C")
		case 2:
			print("x")
		}
		code /= 3
	}
	print(" -> ")
	for i := range 3 {
		switch chain[i].Flg {
		case psdCNT:
			print("C")
		case 0:
			print("0")
		case x:
			print("x")
		default:
			print("?")
		}
	}
	print(" w")
	for i := range 3 {
		print(" ", chain[i].Workm.T[0])
	}
	ws := yaroze.GsWSMATRIX()
	println(" ws", ws.T[0], ws.M[0][0], "ret", r)
}

// randRot gives a matrix whose elements are not those of a rotation, so that the
// transpose shows, or a real turn when turn is set.
func randM(m *yaroze.MATRIX, turn bool, tr int32) {
	if turn {
		a := yaroze.SVECTOR{Vx: int16(next(0, 4095)), Vy: int16(next(0, 4095)), Vz: int16(next(0, 4095))}
		yaroze.RotMatrix(&a, m)
	} else {
		for i := range 3 {
			for j := range 3 {
				m.M[i][j] = int16(next(-4096, 4096))
			}
		}
	}
	for i := range 3 {
		m.T[i] = next(-tr, tr)
	}
}

func random(what string, turn bool, tr int32, n int) {
	for range n {
		yaroze.GsInitCoordinate2(nil, &chain[0])
		randM(&chain[0].Coord, turn, tr)
		chain[0].Flg = 0
		randM(&v2.View, turn, tr)
		v2.Super = &chain[0]
		r := yaroze.GsSetView2(&v2)
		print(what, " v")
		show(&v2.View)
		print(" w")
		show(&chain[0].Workm)
		print(" ws")
		show(yaroze.GsWSMATRIX())
		println(" r", r)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	for range 3 {
		yaroze.GsSwapDispBuff()
	}
	x := psdCNT + 1
	println("cnt", psdCNT)
	println("GsSetView2")
	v2.View = yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{7, 0, 0}}
	for code := 0; code < 27; code++ {
		v2.Super = setup(code, x)
		r := yaroze.GsSetView2(&v2)
		line(code, x, r)
	}
	println("GsSetRefView2")
	rv = yaroze.GsRVIEW2{Vpx: 0, Vpy: 0, Vpz: -1000, Vrx: 0, Vry: 1, Vrz: 0}
	for code := 0; code < 27; code++ {
		rv.Super = setup(code, x)
		r := yaroze.GsSetRefView2(&rv)
		line(code, x, r)
	}
	println("cnt", psdCNT)

	// World views: GsSetView2 copies, and whether View's parts all go.
	v2.Super = nil
	randM(&v2.View, false, 100000)
	println("world", yaroze.GsSetView2(&v2))
	show(&v2.View)
	show(yaroze.GsWSMATRIX())
	println()

	random("turn", true, 30000, 30)
	random("any", false, 30000, 30)
	random("big", true, 1000000, 20)

	// GsSetRefView2 with Super against GsSetRefView2 with WORLD and the result as a
	// GsSetView2 with the same Super.
	same, diff := 0, 0
	for range 40 {
		yaroze.GsInitCoordinate2(nil, &chain[0])
		randM(&chain[0].Coord, true, 30000)
		rv = yaroze.GsRVIEW2{
			Vpx: next(-3000, 3000), Vpy: next(-3000, 3000), Vpz: next(-3000, 3000),
			Vrx: next(-3000, 3000), Vry: next(-3000, 3000), Vrz: next(-3000, 3000),
			Rz: next(-4096*90, 4096*90),
		}
		rv.Super = nil
		yaroze.GsSetRefView2(&rv)
		v2.View = *yaroze.GsWSMATRIX()
		v2.Super = &chain[0]
		chain[0].Flg = 0
		yaroze.GsSetView2(&v2)
		a := *yaroze.GsWSMATRIX()
		rv.Super = &chain[0]
		chain[0].Flg = 0
		yaroze.GsSetRefView2(&rv)
		if a == *yaroze.GsWSMATRIX() {
			same++
		} else {
			diff++
			if diff < 4 {
				print("ref differs a")
				show(&a)
				print(" b")
				show(yaroze.GsWSMATRIX())
				println()
			}
		}
	}
	println("refview with Super", same, "same as view,", diff, "differ")
	// The two points the same, with a chain: does the chain get worked out?
	rv = yaroze.GsRVIEW2{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 2, Vrz: 3}
	rv.Super = setup(0, x)
	*yaroze.GsWSMATRIX() = yaroze.MATRIX{T: [3]int32{55, 55, 55}}
	r := yaroze.GsSetRefView2(&rv)
	line(0, x, r)
	print("same points ws")
	show(yaroze.GsWSMATRIX())
	println()
	// Straight up: the line of sight's ground length is 0.
	rv = yaroze.GsRVIEW2{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 200, Vrz: 3}
	rv.Super = setup(0, x)
	*yaroze.GsWSMATRIX() = yaroze.MATRIX{T: [3]int32{55, 55, 55}}
	r = yaroze.GsSetRefView2(&rv)
	line(0, x, r)
	print("straight up ws")
	show(yaroze.GsWSMATRIX())
	println()
	// The same, with GsIDMATRIX2 changed and a twist, world and straight down.
	saved := idm2
	idm2.M = [3][3]int16{{2048, 0, 0}, {0, 1024, 0}, {0, 0, 512}}
	idm2.T = [3]int32{31, 32, 33}
	for _, c := range [...]yaroze.GsRVIEW2{
		{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 2, Vrz: 3},
		{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 2, Vrz: 3, Rz: 4096 * 90},
		{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 200, Vrz: 3},
		{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: -200, Vrz: 3},
		{Vpx: 1, Vpy: 2, Vpz: 3, Vrx: 1, Vry: 200, Vrz: 3, Rz: 4096 * 90},
	} {
		rv = c
		*yaroze.GsWSMATRIX() = yaroze.MATRIX{T: [3]int32{55, 55, 55}}
		r = yaroze.GsSetRefView2(&rv)
		print("view ", c.Vry, " rz ", c.Rz, " ret ", r, " ws")
		show(yaroze.GsWSMATRIX())
		println()
	}
	idm2 = saved
	// A view against a chain that ends high up, for the walk: the view with a chain
	// of one whose flag is x uses Workm, and one of flag 0 uses Coord.
	v2.View = yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}}
	yaroze.GsInitCoordinate2(nil, &chain[0])
	chain[0].Coord.T = [3]int32{5, 5, 5}
	chain[0].Workm = yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{9, 9, 9}}
	chain[0].Flg = x
	v2.Super = &chain[0]
	yaroze.GsSetView2(&v2)
	println("view root x ws", yaroze.GsWSMATRIX().T[0], "flg x", chain[0].Flg == x)
	chain[0].Flg = 0
	yaroze.GsSetView2(&v2)
	println("view root 0 ws", yaroze.GsWSMATRIX().T[0], "flg C", chain[0].Flg == psdCNT)
	println("END")
	for {
	}
}
