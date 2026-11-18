//go:build yaroze

// Command gstrap is a probe for spec 19: which sums in composing matrices stop the
// machine on signed overflow and which wrap. A stop ends the run, so the const which
// picks the case, and a run per case is needed; the last line of the TTY says what
// happened. Case 8 scores random large rotations and translations against the model
// of spec 19.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

const which = 0

var (
	up, node yaroze.GsCOORDINATE2
	lw       yaroze.MATRIX
	v2       yaroze.GsVIEW2
	seed     uint32 = 99
)

func next() uint32 {
	seed = seed*1103515245 + 12345
	return seed>>16 | seed<<16
}

func diag(v int16, t ...int32) yaroze.MATRIX {
	return yaroze.MATRIX{
		M: [3][3]int16{{v, 0, 0}, {0, v, 0}, {0, 0, v}},
		T: [3]int32{t[0], t[1], t[2]},
	}
}

func chain(p, q *yaroze.MATRIX, out *yaroze.MATRIX) {
	yaroze.GsInitCoordinate2(nil, &up)
	yaroze.GsInitCoordinate2(&up, &node)
	up.Workm = *p
	up.Flg = psdCNT + 1
	node.Coord = *q
	node.Flg = 0
	yaroze.GsGetLw(&node, out)
}

// model is spec 19's sum for one row, with the final add wrapping.
func model(p, q *yaroze.MATRIX) [3]int32 {
	var t [3]int32
	for i := range 3 {
		var lo int64
		var hi int32
		for k := range 3 {
			lo += int64(p.M[i][k]) * int64(q.T[k]&0x7fff)
			hi += int32(p.M[i][k]) * int32(int16(q.T[k]>>15))
		}
		t[i] = int32(lo>>12) + hi<<3 + p.T[i]
	}
	return t
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	println("case", which)
	switch which {
	case 0: // the parent's translation plus the product, GsGetLw
		p, q := diag(4096, 0x7fffffff, 0, 0), diag(4096, 1, 0, 0)
		chain(&p, &q, &lw)
	case 1: // the same downward
		p, q := diag(4096, -0x80000000, 0, 0), diag(4096, -1, 0, 0)
		chain(&p, &q, &lw)
	case 2: // GsWSMATRIX's translation plus the product, GsGetLs
		*yaroze.GsWSMATRIX() = diag(4096, 0x7fffffff, 0, 0)
		yaroze.GsInitCoordinate2(nil, &node)
		node.Workm = diag(4096, 1, 0, 0)
		node.Flg = psdCNT + 1
		yaroze.GsGetLs(&node, &lw)
	case 3: // CompMatrix
		p, q := diag(4096, 0x7fffffff, 0, 0), diag(4096, 1, 0, 0)
		yaroze.CompMatrix(&p, &q, &lw)
	case 4: // the low part plus the high part, with a parent translation of 0
		p, q := diag(32767, 0, 0, 0), diag(4096, 0x10007fff, 0, 0)
		chain(&p, &q, &lw)
	case 5: // the view's translation plus the product, GsSetView2
		yaroze.GsInitCoordinate2(nil, &node)
		node.Coord = diag(4096, -1, 0, 0)
		v2.View = diag(4096, 0x7fffffff, 0, 0)
		v2.Super = &node
		yaroze.GsSetView2(&v2)
	case 6: // the high part alone wraps, the rest no trap
		p, q := diag(32767, 0, 0, 0), diag(4096, 0x3fffffff, 0, 0)
		chain(&p, &q, &lw)
	case 7: // the sum of the products of the high part wraps
		p := yaroze.MATRIX{M: [3][3]int16{{32767, 32767, 32767}, {0, 0, 0}, {0, 0, 0}}}
		q := diag(4096, 0x3fff8000, 0x3fff8000, 0x3fff8000)
		chain(&p, &q, &lw)
	case 8: // big rotations and huge translations, against the model
		same, shown := 0, 0
		for range 300 {
			var p, q yaroze.MATRIX
			for i := range 3 {
				for j := range 3 {
					p.M[i][j] = int16(next())
					q.M[i][j] = int16(next())
				}
				p.T[i] = int32(next() >> 20)
				q.T[i] = int32(next())
			}
			chain(&p, &q, &lw)
			if model(&p, &q) == lw.T {
				same++
			} else if shown < 3 {
				shown++
				m := model(&p, &q)
				println("miss", p.M[0][0], p.M[0][1], p.M[0][2], "t", q.T[0], q.T[1], q.T[2], "pt", p.T[0], "got", lw.T[0], lw.T[1], lw.T[2], "model", m[0], m[1], m[2])
			}
		}
		println("model", same, "of 300")
	}
	println("lw", lw.T[0], lw.T[1], lw.T[2], "ws", yaroze.GsWSMATRIX().T[0])
	println("no stop")
	println("END")
	for {
	}
}
