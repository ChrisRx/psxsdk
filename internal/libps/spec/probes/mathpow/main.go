//go:build yaroze

// Command mathpow is a fifth probe for spec 12, for pow's open cases and
// log's two. Four groups: "tie" calls exp on arguments within a few ulps
// of (k + 1/2) ln 2 (ties.go), where the two ways of forming exp's N,
// (int)(x * (1/ln 2) + 1/2) and (int)(x / ln 2 + 1/2), round to different
// integers; "pw" prints, for each open pow case and some controls, pow(x,
// y) beside the chain exp(y * log(x)) worked out on the library's own log,
// product and exp, with x * (1/ln 2) and x / ln 2 for the product; "log"
// calls log and frexp over a grid of the exponent fields at both ends of
// the range with many mantissas; "form" works n ln 2 + 2 sum(f), log's
// own form, on the library's helpers for chosen n and f, to find which
// pair gives log(NaN) and log(2^-1074). It follows the rules of
// spec/probes/math: no floating point of its own (every operation is a
// named call of a helper), nothing allocated, inputs through volatile
// memory, interrupts off while computing.
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

//export __fixdfsi
func fixdfsi(a float64) int32

//export __eqdf2
func eqdf2(a, b float64) int32

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
	expF   = func(x float64) float64 { return yaroze.Exp(x) }
	logF   = func(x float64) float64 { return yaroze.Log(x) }
	powF   = func(x, y float64) float64 { return yaroze.Pow(x, y) }
	frexpF = func(x float64, e *int32) float64 { return yaroze.Frexp(x, e) }
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

// record logs a tag, k (printed when not -1) and the first cnt of the
// five values. No slice is made: the target's allocator never frees.
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
		if l.k != -1 {
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

// Constants as bits: ln 2, 1/ln 2 and 1/2 as the clean library has them.
const (
	ln2    = 0x3fe62e42fefa39ef
	invLn2 = 0x3ff71547652b82fe
	half   = 0x3fe0000000000000
	one    = 0x3ff0000000000000
	two    = 0x4000000000000000
)

// runTie: exp over the tie arguments.
func runTie() {
	start("tie")
	for _, b := range expTies {
		x := d(b)
		record("tie", -1, 2, b, bits(expF(x)), 0, 0, 0)
	}
	flush()
}

// powCases are pow's open cases (a base of 2 or 1/2 with a half-integer
// power, NaN and 2^-1074 as bases) and controls that match.
var powCases = [...][2]uint64{
	{0x4000000000000000, 0x3ff8000000000000}, // 2^1.5, open
	{0x4000000000000000, 0xbff8000000000000}, // 2^-1.5, open
	{0x4000000000000000, 0x400c000000000000}, // 2^3.5, open
	{0x4000000000000000, 0xc00c000000000000}, // 2^-3.5, open
	{0x4000000000000000, 0x401a000000000000}, // 2^6.5, open
	{0x4000000000000000, 0xc01a000000000000}, // 2^-6.5, open
	{0x4000000000000000, 0x4030800000000000}, // 2^16.5, open
	{0x4000000000000000, 0xc030800000000000}, // 2^-16.5, open
	{0x4000000000000000, 0x4031800000000000}, // 2^17.5, open
	{0x4000000000000000, 0xc031800000000000}, // 2^-17.5, open
	{0x4000000000000000, 0x403d800000000000}, // 2^29.5, open
	{0x4000000000000000, 0xc03d800000000000}, // 2^-29.5, open
	{0x3fe0000000000000, 0x3ff8000000000000}, // 0.5^1.5, open
	{0x4000000000000000, 0x4004000000000000}, // 2^2.5, matches
	{0x4000000000000000, 0x4012000000000000}, // 2^4.5, matches
	{0x4000000000000000, 0x3fe0000000000000}, // 2^0.5, matches
	{0x4000000000000000, 0x3fd0000000000000}, // 2^0.25, matches
	{0x4024000000000000, 0x3fe0000000000000}, // 10^0.5, matches
	{0x4008000000000000, 0x3fd5555555555555}, // 3^(1/3), matches
	{0x4000000000000000, 0x3fb999999999999a}, // 2^0.1, matches
	{0x7ff8000000000000, 0x3fe0000000000000}, // NaN^0.5, open through log
	{0x0000000000000001, 0x3fe0000000000000}, // (2^-1074)^0.5, open through log
	{0x0000000000000001, 0x3ff8000000000000}, // (2^-1074)^1.5, open through log
}

// runPow prints each case's pow(x, y), log(x) and t = y * log(x) on one
// line, and exp(t), t * (1/ln 2) and t / ln 2 on the next.
func runPow() {
	start("pw")
	for _, c := range powCases {
		x, y := d(c[0]), d(c[1])
		p := powF(x, y)
		l := logF(x)
		t := muldf3(y, l)
		record("pw", -1, 5, c[0], c[1], bits(p), bits(l), bits(t))
		record("pw2", -1, 5, c[0], c[1], bits(expF(t)), bits(muldf3(t, d(invLn2))), bits(divdf3(t, d(ln2))))
	}
	flush()
}

// The log grid: exponent fields at both ends of the range and a few in
// the middle, with mantissas (fraction fields) placed around the
// doubling threshold sqrt(1/2) and at the ends.
var logFields = [...]int{0, 1, 2, 3, 100, 1021, 1022, 1023, 1024, 1025, 2000, 2044, 2045, 2046, 2047}

var logMants = [...]uint64{
	0, 1, 2, 3, 0x100, 1 << 20, 1 << 32, 1 << 40, 1 << 48, 1 << 49, 1 << 50,
	0x2000000000001, 0x3ffffffffffff, 0x4000000000000, 0x4000000000001, 0x5000000000000,
	0x6000000000000, 0x6a09e667f3bcc, 0x6a09e667f3bcd, 0x6a09e667f3bce, 0x7000000000000,
	0x8000000000000, 0x8000000000001, 0x9000000000000, 0xa000000000000, 0xc000000000000,
	0xe000000000000, 0xfffffffffffff,
}

var frexpExp [2]int32

// runLog prints log(x), frexp's mantissa and (as k) its exponent for the
// grid.
func runLog() {
	start("log")
	for _, e := range logFields {
		for _, m := range logMants {
			b := uint64(e)<<52 | m
			x := d(b)
			l := logF(x)
			frexpExp[0] = -0x55555556
			f := frexpF(x, &frexpExp[0])
			record("log", frexpExp[0], 3, b, bits(l), bits(f), 0, 0)
		}
	}
	flush()
}

// form is log's form n ln 2 + 2 sum(f), the series in z = (f-1)/(f+1)
// summed until a term no longer changes the sum, on the library's
// helpers.
func form(n int32, f float64) float64 {
	z := divdf3(subdf3(f, d(one)), adddf3(f, d(one)))
	w := muldf3(z, z)
	sum := z
	t := z
	for k := int32(3); k < 200; k += 2 {
		t = muldf3(t, w)
		next := adddf3(sum, divdf3(t, floatsidf(k)))
		if eqdf2(next, sum) == 0 {
			break
		}
		sum = next
	}
	return adddf3(muldf3(floatsidf(n), d(ln2)), muldf3(d(two), sum))
}

// formCases are (n, f) pairs for log(NaN) and log(2^-1074): the split
// the other inputs follow and its neighbors.
var formCases = [...]struct {
	n int32
	f uint64
}{
	{1025, 0x3fe8000000000000}, {1024, 0x3ff8000000000000}, {1025, 0x3ff8000000000000},
	{1026, 0x3fe8000000000000}, {1024, 0x4008000000000000},
	{-1023, 0x3ff0000000000001}, {-1022, 0x3fe0000000000001}, {-1021, 0x3fd0000000000001},
	{-1024, 0x4000000000000001},
	{1024, 0x3fe8000000000000}, {1025, 0x3fe4000000000000}, {1024, 0x3ff4000000000000},
	{-1022, 0x3fe0000000000000}, {-1021, 0x3fe0000000000000}, {-1022, 0x3ff0000000000000},
}

func runForm() {
	start("form")
	for _, c := range formCases {
		record("form", c.n, 2, c.f, bits(form(c.n, d(c.f))), 0, 0, 0)
	}
	flush()
}

func main() {
	yaroze.ResetGraph(0)
	ps("mathpow probe\n")
	yaroze.EnterCriticalSection()
	runTie()
	runPow()
	runLog()
	runForm()
	flush()
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathpow probe done\n")
	for {
		yaroze.VSync(0)
	}
}
