//go:build yaroze

// Command gscompose is a probe for spec 19: how a system's Workm is made from its
// parent's Workm and its Coord, and how GsGetLs and GsGetLws make the screen matrix
// from GsWSMATRIX and Workm: rounding, saturation and overflow, on chosen matrices and
// on a random sequence, against CompMatrix, MulMatrix0 and models.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDCNT
var psdCNT uint32

var (
	up, node yaroze.GsCOORDINATE2
	lw, ls   yaroze.MATRIX
	comp     yaroze.MATRIX
	seed     uint32 = 12345
)

func next() uint32 {
	seed = seed*1103515245 + 12345
	return seed >> 7
}

func between(lo, hi int32) int32 { return lo + int32(next()%uint32(hi-lo+1)) }

func show(m *yaroze.MATRIX) {
	for i := range 3 {
		print(" ", m.M[i][0], " ", m.M[i][1], " ", m.M[i][2])
	}
	print(" t ", m.T[0], " ", m.T[1], " ", m.T[2])
}

// chainCompose makes Workm of a child of a system with Workm p, flag old.
func chainCompose(p, q *yaroze.MATRIX, out *yaroze.MATRIX) {
	yaroze.GsInitCoordinate2(nil, &up)
	yaroze.GsInitCoordinate2(&up, &node)
	up.Workm = *p
	up.Flg = psdCNT + 1
	node.Coord = *q
	node.Flg = 0
	yaroze.GsGetLw(&node, out)
}

// exact is the rotation row-by-column model with 64-bit sums, a right shift by 12 and
// wrap to 32 bits.
func exact(p, q *yaroze.MATRIX) [3]int32 {
	var t [3]int32
	for i := range 3 {
		var sum int64
		for k := range 3 {
			sum += int64(p.M[i][k]) * int64(q.T[k])
		}
		t[i] = int32(sum>>12) + p.T[i]
	}
	return t
}

// split is the model that fits: the translation cut to 31 bits, as a low part of 15
// bits and a high part of 16 summed apart, the high part wrapping.
func split(p, q *yaroze.MATRIX) [3]int32 {
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

var cases = [][2]yaroze.MATRIX{
	{
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{1000, 2000, 3000}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{40000, -70000, 100000}},
	},
	{
		{M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}}, T: [3]int32{5, 6, 7}},
		{M: [3][3]int16{{2896, 0, 2896}, {0, 4096, 0}, {-2896, 0, 2896}}, T: [3]int32{1, 2, 3}},
	},
	{
		{M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}}, T: [3]int32{5, 6, 7}},
		{M: [3][3]int16{{1, -1, 3}, {-3, 5, -7}, {4095, -4095, 1}}, T: [3]int32{-1, -2, -3}},
	},
	{
		{M: [3][3]int16{{32767, 32767, 32767}, {-32768, -32768, -32768}, {32767, -32768, 0}}, T: [3]int32{0, 0, 0}},
		{M: [3][3]int16{{32767, 32767, 32767}, {32767, 32767, 32767}, {32767, 32767, 32767}}, T: [3]int32{0, 0, 0}},
	},
	{
		{M: [3][3]int16{{32767, 32767, 32767}, {-32768, -32768, -32768}, {32767, -32768, 1}}, T: [3]int32{0, 0, 1}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{0x7fffffff, 0x7fffffff, -0x80000000}},
	},
	{
		{M: [3][3]int16{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, T: [3]int32{0, 0, 0}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{-1, -4095, -4097}},
	},
	{
		{M: [3][3]int16{{-1, 0, 0}, {0, -1, 0}, {0, 0, -1}}, T: [3]int32{0, 0, 0}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{4095, 4097, 32768}},
	},
	{
		{M: [3][3]int16{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, T: [3]int32{0, 0, 0}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{32767, 32768, 65535}},
	},
	{
		{M: [3][3]int16{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, T: [3]int32{0, 0, 0}},
		{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}, T: [3]int32{65536, 65537, 131071}},
	},
}

type tally struct{ comp, exact, split, mul, ls int }

func randM(m *yaroze.MATRIX, rotRange, tRange int32) {
	for i := range 3 {
		for j := range 3 {
			m.M[i][j] = int16(between(-rotRange, rotRange))
		}
		m.T[i] = between(-tRange, tRange)
	}
}

func score(name string, rotRange, tRange int32, n int) {
	var t tally
	var shown int
	for range n {
		var p, q yaroze.MATRIX
		randM(&p, rotRange, 1000000)
		randM(&q, rotRange, tRange)
		chainCompose(&p, &q, &lw)
		// The same pair through GsGetLs, with GsWSMATRIX as p and Workm as q.
		*yaroze.GsWSMATRIX() = p
		yaroze.GsInitCoordinate2(nil, &node)
		node.Workm = q
		node.Flg = psdCNT + 1
		yaroze.GsGetLs(&node, &ls)
		if ls == lw {
			t.ls++
		}
		var mm yaroze.MATRIX
		yaroze.MulMatrix0(&p, &q, &mm)
		if mm.M == lw.M {
			t.mul++
		}
		cq := q
		yaroze.CompMatrix(&p, &cq, &comp)
		if comp.T == lw.T {
			t.comp++
		}
		if exact(&p, &q) == lw.T {
			t.exact++
		}
		s := split(&p, &q)
		if s == lw.T {
			t.split++
		} else if shown < 3 {
			shown++
			print(name, " split miss p")
			show(&p)
			print(" q")
			show(&q)
			print(" lw")
			show(&lw)
			println(" split", s[0], s[1], s[2])
		}
	}
	println(name, "of", n, "rot=mul0", t.mul, "t=comp", t.comp, "t=exact", t.exact, "t=split", t.split, "ls=lw", t.ls)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	println("Lw from chosen cases")
	for i := range cases {
		p, q := &cases[i][0], &cases[i][1]
		chainCompose(p, q, &lw)
		cq := *q
		yaroze.CompMatrix(p, &cq, &comp)
		print("case ", i, " lw")
		show(&lw)
		println()
		print("  comp")
		show(&comp)
		println()
		e := exact(p, q)
		s := split(p, q)
		println("  exact", e[0], e[1], e[2], "split", s[0], s[1], s[2])
		// ls: GsWSMATRIX is p and Workm is q.
		*yaroze.GsWSMATRIX() = *p
		yaroze.GsInitCoordinate2(nil, &node)
		node.Workm = *q
		node.Flg = psdCNT + 1
		yaroze.GsGetLs(&node, &ls)
		print("  ls")
		show(&ls)
		println()
		node.Flg = psdCNT + 1
		yaroze.GsGetLws(&node, &lw, &ls)
		print("  lws w")
		show(&lw)
		print(" s")
		show(&ls)
		println()
	}
	score("small t", 4096, 30000, 100)
	score("edge t", 4096, 70000, 100)
	score("big t", 4096, 1000000, 100)
	score("huge t", 4096, 0x7fffff00, 100)
	score("big rot", 32767, 30000, 100)
	println("END")
	for {
	}
}
