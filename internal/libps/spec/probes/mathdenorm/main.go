//go:build yaroze

// Command mathdenorm is a seventh probe for spec 12, for the two open
// lines: hypot of the smallest denormal with itself and sin of the
// negative field-1 pattern with every fraction bit set. Four groups.
// "grid" calls __muldf3, __adddf3, __subdf3, __divdf3 and __ltdf2 on
// operands whose exponent field is 0 (the denormal patterns), against
// normal numbers of several sizes and against each other, both operand
// orders, and __adddf3 and __subdf3 over a scan of the exponent
// difference against one huge operand (the garbage hypot makes); "hyp"
// works hypot's step on the library's own helpers for the open pair and
// its neighbors, printing every intermediate value of each step and the
// final value of six orderings of p += 2*s*p beside hypot itself; "hypot"
// calls hypot on more tiny pairs; "sin" calls sin on pairs of arguments
// of both signs chosen on the host so that a reduction on the signed
// argument and one on its magnitude round apart, and prints the two
// reductions worked on the helpers, with the quotient and the zero cases
// of the open line. It follows the rules of spec/probes/math: no
// floating point of its own (every operation is a named call of a
// helper), nothing allocated, inputs through volatile memory, interrupts
// off while computing.
package main

import (
	"math"
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

//export __adddf3
func adddf3(a, b float64) float64

//export __subdf3
func subdf3(a, b float64) float64

//export __muldf3
func muldf3(a, b float64) float64

//export __divdf3
func divdf3(a, b float64) float64

//export __ltdf2
func ltdf2(a, b float64) int32

//export __floatsidf
func floatsidf(a int32) float64

//export __fixdfsi
func fixdfsi(a float64) int32

//export __mulsf3
func mulsf3(a, b float32) float32

//export __divsf3
func divsf3(a, b float32) float32

//export __addsf3
func addsf3(a, b float32) float32

//export __truncdfsf2
func truncdfsf2(a float64) float32

//export __extendsfdf2
func extendsfdf2(a float32) float64

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

const hexDigits = "0123456789abcdef"

var (
	line  [256]byte
	nline int
)

func putc(c byte) {
	if nline < len(line) {
		line[nline] = c
		nline++
	}
	if c == '\n' {
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
	}
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int64) {
	if v < 0 {
		putc('-')
		v = -v
	}
	var d [20]byte
	k := 0
	for {
		d[k] = byte('0' + v%10)
		k++
		v /= 10
		if v == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hex64(v uint64) {
	for i := range 16 {
		putc(hexDigits[v>>(60-4*i)&15])
	}
}

// The inputs go through memory so that the compiler cannot fold a call.
var (
	cur64 uint64
	cur32 uint32
)

func d(b uint64) float64 {
	volatile.StoreUint64(&cur64, b)
	return math.Float64frombits(volatile.LoadUint64(&cur64))
}

func f(b uint32) float32 {
	volatile.StoreUint32(&cur32, b)
	return math.Float32frombits(volatile.LoadUint32(&cur32))
}

func bits(x float64) uint64  { return math.Float64bits(x) }
func sbits(x float32) uint64 { return uint64(math.Float32bits(x)) }

// The library's math calls, through function values.
var (
	hypotF = func(x, y float64) float64 { return yaroze.Hypot(x, y) }
	sinF   = func(x float64) float64 { return yaroze.Sin(x) }
	tanF   = func(x float64) float64 { return yaroze.Tan(x) }
)

// Results are logged and printed in batches, with interrupts let in only
// to print. An entry is a tag, an integer and up to ten values.
type entry struct {
	tag string
	k   int32
	n   int
	v   [10]uint64
}

var (
	lg [16]entry
	n  int
)

// record logs a tag, k (printed when not negative) and the first cnt of
// the values. No slice is made: the target's allocator never frees.
func record(tag string, k int32, cnt int, v [10]uint64) {
	l := &lg[n]
	l.tag, l.k, l.n = tag, k, cnt
	l.v = v
	n++
	if n == len(lg) {
		flush()
	}
}

func flush() {
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &lg[i]
		ps(l.tag)
		if l.k >= 0 {
			ps(" k=")
			pi(int64(l.k))
		}
		for j := range l.n {
			ps(" ")
			hex64(l.v[j])
		}
		ps("\n")
	}
	n = 0
	yaroze.EnterCriticalSection()
}

var group uint32

func start(name string) {
	flush()
	group++
	mark(group)
	yaroze.ExitCriticalSection()
	ps("== ")
	ps(name)
	ps("\n")
	yaroze.EnterCriticalSection()
}

// fracs are the fractions of the field-0 operands of the grid.
var fracs = [...]uint64{
	1, 2, 3, 0x4000000000000, 0x8000000000000, 0x8000000000001, 0xfffffffffffff, 0x23456789abcde,
}

// normals are the other operands: 1, 2, 0.5, 3, 1.6, the single 0.4 and
// 0.2 and 2/17 widened (hypot's step), 2^-1022, 2^-1000, 2^1000, and the
// garbage hypot's first step makes, -1.6 times 2^1023.
var normals = [...]uint64{
	0x3ff0000000000000, 0x4000000000000000, 0x3fe0000000000000, 0x4008000000000000,
	0x3ff999999999999a, 0x3fd99999a0000000, 0x3fc99999a0000000, 0x3fbe1e1e20000000,
	0x0010000000000000, 0x0170000000000000, 0x7e70000000000000, 0xffe99999a0000002,
}

// ops prints, for a and b, a*b, b*a, a+b, b+a, a-b, b-a, a/b, b/a.
func ops(tag string, a, b uint64) {
	x, y := d(a), d(b)
	record(tag, -1, 10, [10]uint64{a, b,
		bits(muldf3(x, y)), bits(muldf3(y, x)), bits(adddf3(x, y)), bits(adddf3(y, x)),
		bits(subdf3(x, y)), bits(subdf3(y, x)), bits(divdf3(x, y)), bits(divdf3(y, x))})
}

func runGrid() {
	start("grid")
	for _, fr := range fracs {
		for _, b := range normals {
			ops("g", fr, b)
		}
	}
	for _, fr := range fracs {
		for _, fr2 := range fracs {
			ops("gg", fr, fr2)
		}
		ops("gn", fr|1<<63, 0x3fd99999a0000000)
		ops("gn", fr, 0xbfd99999a0000000)
	}
	// Comparisons of field-0 patterns: less than, both orders, and against
	// zero and the smallest normal.
	for _, fr := range fracs {
		x := d(fr)
		record("lt", -1, 7, [10]uint64{fr,
			uint64(uint32(ltdf2(x, d(0)))), uint64(uint32(ltdf2(d(0), x))),
			uint64(uint32(ltdf2(x, d(0x0010000000000000)))), uint64(uint32(ltdf2(d(0x0010000000000000), x))),
			uint64(uint32(ltdf2(x, d(1)))), uint64(uint32(ltdf2(d(1), x)))})
	}
	// The exponent difference scanned: 1.0 times 2^(e-1023) for every field
	// e from 0 to 70 against the garbage and its positive twin.
	for e := uint64(0); e <= 70; e++ {
		a := e << 52
		for _, b := range [...]uint64{0xffe99999a0000002, 0x7fe99999a0000002, 0x7fe0000000000000} {
			x, y := d(a), d(b)
			record("scan", int32(e), 7, [10]uint64{a, b,
				bits(adddf3(x, y)), bits(adddf3(y, x)), bits(subdf3(x, y)), bits(subdf3(y, x)), bits(muldf3(x, y))})
		}
	}
	// The same scan with a set fraction, for the shifted bits' value.
	for e := uint64(0); e <= 70; e += 2 {
		a := e<<52 | 0xfffffffffffff
		for _, b := range [...]uint64{0xffe99999a0000002, 0x7fe0000000000000} {
			x, y := d(a), d(b)
			record("scanf", int32(e), 6, [10]uint64{a, b,
				bits(adddf3(x, y)), bits(adddf3(y, x)), bits(subdf3(x, y)), bits(subdf3(y, x))})
		}
	}
	flush()
}

// pairs are hypot's arguments: the open pair and its neighbors.
var pairs = [...][2]uint64{
	{0x0000000000000001, 0x0000000000000001}, {0x0000000000000001, 0x0000000000000002},
	{0x0000000000000002, 0x0000000000000001}, {0x0000000000000002, 0x0000000000000002},
	{0x0000000000000003, 0x0000000000000003}, {0x0008000000000000, 0x0008000000000000},
	{0x000fffffffffffff, 0x000fffffffffffff}, {0x0000000000000001, 0x0010000000000000},
	{0x0010000000000000, 0x0000000000000001}, {0x0010000000000000, 0x0010000000000000},
	{0x0008000000000000, 0x0000000000000001}, {0x0000000000000001, 0x0008000000000000},
	{0x0004000000000000, 0x0004000000000000}, {0x0000000000000003, 0x0000000000000001},
	{0x0000000000000001, 0x0000000000000003}, {0x0020000000000000, 0x0020000000000000},
	{0x0010000000000000, 0x0008000000000000}, {0x0010000000000000, 0x0020000000000000},
	{0x0030000000000000, 0x0030000000000000}, {0x8000000000000001, 0x8000000000000001},
	{0x0000000000000001, 0x8000000000000001}, {0x0000000000000001, 0x0000000000000000},
	{0x0000000000000000, 0x0000000000000001}, {0x00023456789abcde, 0x00023456789abcde},
	{0x0000000000000001, 0x3ff0000000000000}, {0x0000000000000011, 0x0000000000000011},
	{0x0000000000000100, 0x0000000000000100}, {0x0000000000010000, 0x0000000000010000},
	{0x0000000100000000, 0x0000000100000000}, {0x0000010000000000, 0x0000010000000000},
}

const (
	twoS  = 0x40000000
	fourS = 0x40800000
	oneS  = 0x3f800000
	twoD  = 0x4000000000000000
	halfD = 0x3fe0000000000000
	piD   = 0x400921fb54442d18
)

// hypTrace works hypot's three steps on the helpers for x and y: the
// swap through a float when p < q, then for each step the quotient, its
// narrowing, the square, 4+r, s, s widened and 2s (a single product,
// widened), the products (2s)*p, p*(2s), s*p, 2*(s*p), the sums for p
// and the two orders of s*q. The step goes on with p + (2s)*p and s*q.
// Six orderings of the step are carried along on their own and their
// final values printed beside hypot(x, y).
func hypTrace(xb, yb uint64) {
	x, y := d(xb), d(yb)
	record("hypot", -1, 3, [10]uint64{xb, yb, bits(hypotF(x, y))})
	p, q := x, y
	if ltdf2(p, d(0)) != 0 {
		p = subdf3(d(0), p)
	}
	if ltdf2(q, d(0)) != 0 {
		q = subdf3(d(0), q)
	}
	if ltdf2(p, q) != 0 {
		p, q = q, extendsfdf2(truncdfsf2(p))
	}
	record("pq", -1, 2, [10]uint64{bits(p), bits(q)})
	// The six orderings' own p and q: 0 p+(2s)*p, 1 p+p*(2s), 2 p+2*(s*p),
	// 3 (p+s*p)+s*p, 4 p*(1+2s), 5 (2s)*p+p; q = s*q for 0 to 4 and q*s
	// for 5.
	var vp, vq [6]float64
	for i := range vp {
		vp[i], vq[i] = p, q
	}
	for i := range 3 {
		t64 := divdf3(q, p)
		t := truncdfsf2(t64)
		r := mulsf3(t, t)
		fr := addsf3(f(fourS), r)
		s := divsf3(r, fr)
		sd := extendsfdf2(s)
		s2 := extendsfdf2(mulsf3(f(twoS), s))
		record("st", int32(i+1), 7, [10]uint64{bits(t64), sbits(t), sbits(r), sbits(fr), sbits(s), bits(sd), bits(s2)})
		a := muldf3(s2, p)
		b := muldf3(p, s2)
		c := muldf3(sd, p)
		e := muldf3(d(twoD), c)
		record("pr", int32(i+1), 4, [10]uint64{bits(a), bits(b), bits(c), bits(e)})
		record("sum", int32(i+1), 6, [10]uint64{
			bits(adddf3(p, a)), bits(adddf3(a, p)), bits(adddf3(p, e)), bits(adddf3(adddf3(p, c), c)),
			bits(muldf3(sd, q)), bits(muldf3(q, sd))})
		p = adddf3(p, a)
		q = muldf3(sd, q)
		for v := range vp {
			t64 := divdf3(vq[v], vp[v])
			t := truncdfsf2(t64)
			r := mulsf3(t, t)
			s := divsf3(r, addsf3(f(fourS), r))
			sd := extendsfdf2(s)
			s2 := extendsfdf2(mulsf3(f(twoS), s))
			pp := vp[v]
			switch v {
			case 0:
				vp[v] = adddf3(pp, muldf3(s2, pp))
			case 1:
				vp[v] = adddf3(pp, muldf3(pp, s2))
			case 2:
				vp[v] = adddf3(pp, muldf3(d(twoD), muldf3(sd, pp)))
			case 3:
				vp[v] = adddf3(adddf3(pp, muldf3(sd, pp)), muldf3(sd, pp))
			case 4:
				vp[v] = muldf3(pp, extendsfdf2(addsf3(f(oneS), mulsf3(f(twoS), s))))
			case 5:
				vp[v] = adddf3(muldf3(s2, pp), pp)
			}
			if v == 5 {
				vq[v] = muldf3(vq[v], sd)
			} else {
				vq[v] = muldf3(sd, vq[v])
			}
		}
	}
	record("fin", -1, 6, [10]uint64{bits(vp[0]), bits(vp[1]), bits(vp[2]), bits(vp[3]), bits(vp[4]), bits(vp[5])})
}

func runHyp() {
	start("hyp")
	for _, p := range pairs[:13] {
		hypTrace(p[0], p[1])
	}
	flush()
}

func runHypot() {
	start("hypot")
	for _, p := range pairs {
		x, y := d(p[0]), d(p[1])
		record("hypot", -1, 3, [10]uint64{p[0], p[1], bits(hypotF(x, y))})
	}
	flush()
}

// sinArgs are negative arguments chosen on the host, by a simulation of
// the library's arithmetic, so that a reduction by the nearest multiple
// of pi on the signed argument and one on its magnitude round f apart:
// the addend N*C2 loses bits when it is lined up, and the library's sum
// rounds a negative addend's lost bits up and a positive one's down. All
// are above -pi*2^31, below which Sony's sin never returns.
var sinArgs = [...]uint64{
	0xc072f4ab875a76eb, 0xc072f3396f94542e, 0xc07322a617f3f8b0, 0xc072f30831582a61,
	0xc07320d5d9d17e01, 0xc072f3da4b6d598f, 0xc07321d6ea39536c, 0xc0732246cffbffbb,
	0xc072fb2d60154255, 0xc072f43b015115e4, 0xc0731c4be4e76af3, 0xc072f637512fda24,
	0xc072f2ba20e35063, 0xc0732314fb187811, 0xc072f60c1c25cd51, 0xc072fb3b928ade05,
	// The open line and its positive twin, and the field-0 all-ones pair.
	0x801fffffffffffff, 0x001fffffffffffff, 0x800fffffffffffff, 0x000fffffffffffff,
	// Ordinary values whose results are known, as controls.
	0xc000000000000000, 0x4000000000000000, 0xc024000000000000, 0x4024000000000000,
}

const (
	sinC1 uint64 = 0x4009220000000000 // 3.1416015625
	sinC2 uint64 = 0xbee2aeef4b9ee59e // -8.908910206761537356617e-6
)

// reduce is Cody and Waite's reduction on the helpers: N the nearest
// integer to x/pi (rounded away from zero on the signed x when signed is
// set, else on |x| with N not negative), f = (x - N*C1) - N*C2.
func reduce(x float64, signed bool) (float64, int32) {
	a := x
	if !signed && ltdf2(x, d(0)) != 0 {
		a = subdf3(d(0), x)
	}
	y := divdf3(a, d(piD))
	var nn int32
	if signed && ltdf2(a, d(0)) != 0 {
		nn = fixdfsi(subdf3(y, d(halfD)))
	} else {
		nn = fixdfsi(adddf3(y, d(halfD)))
	}
	xn := floatsidf(nn)
	return subdf3(subdf3(a, muldf3(xn, d(sinC1))), muldf3(xn, d(sinC2))), nn
}

func runSin() {
	start("sin")
	for _, b := range sinArgs {
		x := d(b)
		xneg := d(b ^ 1<<63)
		fs, ns := reduce(x, true)
		fa, na := reduce(x, false)
		record("sin", -1, 3, [10]uint64{b, bits(sinF(x)), bits(sinF(xneg))})
		record("red", -1, 5, [10]uint64{b, uint64(uint32(ns)), bits(fs), uint64(uint32(na)), bits(fa)})
		record("half", -1, 4, [10]uint64{b, bits(divdf3(fs, d(twoD))), bits(divdf3(fa, d(twoD))), bits(muldf3(fs, d(halfD)))})
	}
	// tan at the zeros and at the open line's f, and the quotient of the
	// open line's reduction by 2 in both signs.
	for _, b := range [...]uint64{0, 1 << 63, 0x801fffffffffffff, 0x001fffffffffffff, 0x800fffffffffffff, 0x000fffffffffffff} {
		x := d(b)
		record("tan", -1, 4, [10]uint64{b, bits(tanF(x)), bits(divdf3(x, d(twoD))), bits(muldf3(x, d(halfD)))})
	}
	flush()
}

func main() {
	yaroze.ResetGraph(0)
	ps("mathdenorm probe\n")
	yaroze.EnterCriticalSection()
	runGrid()
	runHyp()
	runHypot()
	runSin()
	flush()
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathdenorm probe done\n")
	for {
		yaroze.VSync(0)
	}
}
