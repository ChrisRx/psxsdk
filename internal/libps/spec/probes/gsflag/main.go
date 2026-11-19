//go:build yaroze

// Command gsflag is a probe for spec 19: every combination of Flg values over chains
// of 1 to 4 systems, for GsGetLw, GsGetLs and GsGetLws. A flag is 0, C (PSDCNT) or x
// (any other value). Each Workm is planted with a half-scale rotation and a large
// translation so that a kept matrix and a parent's use of it both show. A line gives
// the flags before, the flags after, each Workm's translation x and rotation (0,0),
// and the matrix the call stored. Then depth 3 again with x large, negative, and equal
// to C in its low half, and with PSDCNT at 0, where a flag of 0 is the current value.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

var (
	chain [5]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
	ls    yaroze.MATRIX
)

func letter(f, x uint32) string {
	switch f {
	case psdCNT:
		return "C"
	case 0:
		return "0"
	case x:
		return "x"
	}
	return "?"
}

// setup makes a chain of d with the flags given by code, base 3: digit 0 is 0, 1 is
// C and 2 is x.
func setup(d int, code int, x uint32) *yaroze.GsCOORDINATE2 {
	for i := 0; i < d; i++ {
		var up *yaroze.GsCOORDINATE2
		if i > 0 {
			up = &chain[i-1]
		}
		yaroze.GsInitCoordinate2(up, &chain[i])
		p := int32(2)
		for range i {
			p *= 10
		}
		chain[i].Coord.T = [3]int32{p, 0, 0}
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
			M: [3][3]int16{{2048, 0, 0}, {0, 2048, 0}, {0, 0, 2048}},
			T: [3]int32{-100000 * int32(i+1), 0, 0},
		}
	}
	return &chain[d-1]
}

func line(d int, code int, x uint32) {
	for i := 0; i < d; i++ {
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
	for i := 0; i < d; i++ {
		print(letter(chain[i].Flg, x))
	}
	print(" w")
	for i := 0; i < d; i++ {
		print(" ", chain[i].Workm.T[0], "/", chain[i].Workm.M[0][0])
	}
	print(" lw ", lw.T[0], "/", lw.M[0][0])
}

func run(x uint32) {
	println("cnt", psdCNT, "x", x)
	n := 1
	for d := 1; d <= 4; d++ {
		n *= 3
		for code := 0; code < n; code++ {
			c := setup(d, code, x)
			lw = yaroze.MATRIX{}
			yaroze.GsGetLw(c, &lw)
			print("d", d, " ")
			line(d, code, x)
			println()
		}
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	for range 5 {
		yaroze.GsSwapDispBuff()
	}
	// The world-to-screen matrix, so that ls differs from lw.
	ws := yaroze.GsWSMATRIX()
	*ws = yaroze.MATRIX{
		M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}},
		T: [3]int32{7, 0, 0},
	}
	run(psdCNT + 1)

	println("GsGetLws")
	n := 27
	for code := 0; code < n; code++ {
		c := setup(3, code, psdCNT+1)
		lw, ls = yaroze.MATRIX{}, yaroze.MATRIX{}
		yaroze.GsGetLws(c, &lw, &ls)
		line(3, code, psdCNT+1)
		println(" ls", ls.T[0], "/", ls.M[0][0])
	}
	println("GsGetLs")
	for code := 0; code < n; code++ {
		c := setup(3, code, psdCNT+1)
		lw, ls = yaroze.MATRIX{}, yaroze.MATRIX{}
		yaroze.GsGetLs(c, &ls)
		line(3, code, psdCNT+1)
		println(" ls", ls.T[0], "/", ls.M[0][0])
	}
	// Depth 3 with other values of x, and with C as 0.
	for _, x := range [...]uint32{0x80000000, 0xffffffff, psdCNT + 0x10000} {
		println("cnt", psdCNT, "x", x)
		for code := 0; code < 27; code++ {
			c := setup(3, code, x)
			lw = yaroze.MATRIX{}
			yaroze.GsGetLw(c, &lw)
			line(3, code, x)
			println()
		}
	}
	psdCNT = 0
	println("cnt 0")
	for code := 0; code < 27; code++ {
		c := setup(3, code, 5)
		lw = yaroze.MATRIX{}
		yaroze.GsGetLw(c, &lw)
		line(3, code, 5)
		println()
	}
	println("END")
	for {
	}
}
