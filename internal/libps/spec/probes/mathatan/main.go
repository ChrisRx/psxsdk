//go:build yaroze

// Command mathatan is a fourth probe for spec 12: atan's continued
// fraction worked out step by step on the library's own helpers
// (__muldf3, __divdf3, __adddf3, __floatsidf), so that the order of the
// operations inside each term can be scored against the library's atan
// in one run, and the intermediate values compared between the two
// libraries line by line. Three groups: "cand" prints, for each argument
// of the tiny grid and of atan's open cases, atan(x) and x over the
// fraction's denominator formed with four candidate orderings of the
// term k*k*x*x/d; "tr" prints the fraction term by term for the arguments
// asin and atan2 hand atan in their open cases; "chain" prints the
// intermediate values of asin(0.6), of the tiny asin cases and of the
// atan2 cases; "mul" calls __muldf3 directly on products whose exact
// fraction below the last place is a chosen excess above the half
// (mulpairs.go), which shows which low partial products the library's
// product drops. It follows the rules of spec/probes/math: no floating
// point of its own (every operation is a named call of a helper), nothing
// allocated, inputs through volatile memory, interrupts off while
// computing.
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

//export __floatsidf
func floatsidf(a int32) float64

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
var cur64 uint64

func d(b uint64) float64 {
	volatile.StoreUint64(&cur64, b)
	return math.Float64frombits(volatile.LoadUint64(&cur64))
}

func bits(x float64) uint64 { return math.Float64bits(x) }

// The library's math calls, through function values.
var (
	atanF  = func(x float64) float64 { return yaroze.Atan(x) }
	asinF  = func(x float64) float64 { return yaroze.Asin(x) }
	sqrtF  = func(x float64) float64 { return yaroze.Sqrt(x) }
	hypotF = func(x, y float64) float64 { return yaroze.Hypot(x, y) }
	atan2F = func(y, x float64) float64 { return yaroze.Atan2(y, x) }
)

// Results are logged and printed in batches, with interrupts let in only
// to print. An entry is a tag, an integer and up to five values.
type entry struct {
	tag string
	k   int32
	n   int
	v   [5]uint64
}

var (
	lg [24]entry
	n  int
)

// record logs a tag, k (printed when not negative) and the first cnt of
// the five values. No slice is made: the target's allocator never frees.
func record(tag string, k int32, cnt int, a, b, c, d, e uint64) {
	l := &lg[n]
	l.tag, l.k, l.n = tag, k, cnt
	l.v = [5]uint64{a, b, c, d, e}
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

// The fraction's denominator, d starting at 39 and then for k from 19
// down d = (2k-1) + t, with the term t formed four ways.

// denomA: t = ((k*k) * z) / d, z = x*x, the spec's first form.
func denomA(x float64) float64 {
	z := muldf3(x, x)
	dd := floatsidf(39)
	for k := int32(19); k >= 1; k-- {
		dd = adddf3(floatsidf(2*k-1), divdf3(muldf3(floatsidf(k*k), z), dd))
	}
	return dd
}

// denomC1: t = (((k*k) * x) * x) / d, C's left to right k*k*x*x/d.
func denomC1(x float64) float64 {
	dd := floatsidf(39)
	for k := int32(19); k >= 1; k-- {
		dd = adddf3(floatsidf(2*k-1), divdf3(muldf3(muldf3(floatsidf(k*k), x), x), dd))
	}
	return dd
}

// denomF: u = k*x, t = (u*u) / d.
func denomF(x float64) float64 {
	dd := floatsidf(39)
	for k := int32(19); k >= 1; k-- {
		u := muldf3(floatsidf(k), x)
		dd = adddf3(floatsidf(2*k-1), divdf3(muldf3(u, u), dd))
	}
	return dd
}

// denomB: t = ((z * k) * k) / d.
func denomB(x float64) float64 {
	z := muldf3(x, x)
	dd := floatsidf(39)
	for k := int32(19); k >= 1; k-- {
		kk := floatsidf(k)
		dd = adddf3(floatsidf(2*k-1), divdf3(muldf3(muldf3(z, kk), kk), dd))
	}
	return dd
}

// fields and mants are the tiny grid of spec/probes/mathtiny.
var fields = [...]int{
	0, 1, 2, 3, 5, 8, 12, 16, 20, 24, 26, 30, 40, 60, 100, 200, 300, 400, 480, 500, 505, 508, 510, 511, 512,
	513, 515, 520, 540, 600, 700, 800, 900, 950, 1000, 1010, 1015, 1020,
}

var mants = [...]uint64{0, 1 << 51, 1 << 50, 0x23456789abcde, 0xfffffffffffff}

// extra are the arguments of atan's open cases: the main probe's three,
// the denormal quotients asin and atan2 hand atan in theirs, and two near
// 0.75 from asin(0.6).
var extra = [...]uint64{
	0x0000000000000001, 0x000fffffffffffff, 0x0010000000000000,
	0x0003ffffffffff80, 0x0003ffffffff8000, 0x0003ffffffffffc7, 0x0003ffffffffff9d,
	0x0003ffffffffffe0, 0x0003fffffffddb89, 0x0003ffffffff8d2f, 0x0004000000000000,
	0x0003ffffffff0000, 0x0003fffffffffff0, 0x0003ffffffffffff, 0x0003ffff00000000,
	0x3fe7ffffffffffff, 0x3fe8000000000000, 0x3fe7fffffffffffe,
}

// cand prints atan(x), then x over the denominators A, C1, F and B.
func cand(x float64) {
	record("cand", -1, 5, bits(x), bits(atanF(x)),
		bits(divdf3(x, denomA(x))), bits(divdf3(x, denomC1(x))), bits(divdf3(x, denomF(x))))
	record("candB", -1, 2, bits(x), bits(divdf3(x, denomB(x))), 0, 0, 0)
}

func runCand() {
	start("cand")
	for _, e := range fields {
		for _, m := range mants {
			cand(d(uint64(e)<<52 | m))
		}
	}
	for _, b := range extra {
		cand(d(b))
	}
	flush()
}

// trace prints the C1 fraction term by term for x: the numerator, the
// term and d after each k, then x/d and atan(x).
func trace(x float64) {
	record("tr", -1, 2, bits(x), bits(atanF(x)), 0, 0, 0)
	dd := floatsidf(39)
	for k := int32(19); k >= 1; k-- {
		num := muldf3(muldf3(floatsidf(k*k), x), x)
		t := divdf3(num, dd)
		dd = adddf3(floatsidf(2*k-1), t)
		record("tr", k, 3, bits(num), bits(t), bits(dd), 0, 0)
	}
	record("tr", -1, 2, bits(x), bits(divdf3(x, dd)), 0, 0, 0)
}

func runTrace() {
	start("trace")
	for _, b := range extra[3:] {
		trace(d(b))
	}
	flush()
}

// chain prints asin's intermediate values for x: x*x, 1 - x*x, its
// sqrt, x over that, atan of that and asin(x) itself.
func chain(x float64) {
	z := muldf3(x, x)
	w := subdf3(floatsidf(1), z)
	s := sqrtF(w)
	q := divdf3(x, s)
	record("asin", -1, 5, bits(x), bits(z), bits(w), bits(s), bits(q))
	record("asin2", -1, 3, bits(x), bits(atanF(q)), bits(asinF(x)), 0, 0)
}

var asinArgs = [...]uint64{
	0x3fe3333333333333, 0x0140000000000000, 0x0148000000000000, 0x01423456789abcde,
	0x014fffffffffffff, 0x0100000000000000, 0x0180000000000000, 0x80eeeb016c91a352,
	0x0100e4cfa021829c,
}

var atan2Args = [...][2]uint64{
	{0xc020000000000000, 0x7e37e43c8800759c},
	{0x4024000000000000, 0x7e37e43c8800759c},
	{0x01a56e1fc2f8f359, 0x3fb999999999999a},
}

func runChain() {
	start("chain")
	for _, b := range asinArgs {
		chain(d(b))
	}
	for _, p := range atan2Args {
		y, x := d(p[0]), d(p[1])
		h := hypotF(x, y)
		q := divdf3(y, h)
		record("atan2", -1, 5, bits(y), bits(x), bits(h), bits(q), bits(atan2F(y, x)))
		chain(q)
	}
	flush()
}

// runMul calls __muldf3 on the pairs of mulpairs.go.
func runMul() {
	start("mul")
	for _, p := range mulPairs {
		a, b := d(p[0]), d(p[1])
		record("mul", -1, 3, p[0], p[1], bits(muldf3(a, b)), 0, 0)
	}
	for _, p := range mulPairs2 {
		a, b := d(p[0]), d(p[1])
		record("mul", -1, 3, p[0], p[1], bits(muldf3(a, b)), 0, 0)
	}
	flush()
}

func main() {
	yaroze.ResetGraph(0)
	ps("mathatan probe\n")
	yaroze.EnterCriticalSection()
	runCand()
	runTrace()
	runChain()
	runMul()
	flush()
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathatan probe done\n")
	for {
		yaroze.VSync(0)
	}
}
