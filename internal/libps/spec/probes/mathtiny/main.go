//go:build yaroze

// Command mathtiny is a second probe for spec 12: the math calls over a
// grid of tiny arguments, where the library's arithmetic underflows and
// its results turn to garbage whose pattern shows the evaluation form
// (which products are taken, in what order), and over dense scans of a
// few ordinary ranges. It follows the rules of spec/probes/math: no
// floating point of its own, nothing allocated, calls through function
// values, interrupts off for the math, printing every 20 calls.
package main

import (
	"math"
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

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

func hex64(v uint64) {
	for i := range 16 {
		putc(hexDigits[v>>(60-4*i)&15])
	}
}

func bits(b uint64) float64 { return math.Float64frombits(b) }

type entry struct {
	a, b, r uint64
}

var (
	lg [20]entry
	n  int
)

func record(name string, a, b, r float64) {
	l := &lg[n]
	l.a, l.b, l.r = math.Float64bits(a), math.Float64bits(b), math.Float64bits(r)
	n++
	if n == len(lg) {
		flush(name)
	}
}

func flush(name string) {
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &lg[i]
		ps(name)
		ps(" ")
		hex64(l.a)
		ps(" ")
		hex64(l.b)
		ps(" = ")
		hex64(l.r)
		ps("\n")
	}
	n = 0
	yaroze.EnterCriticalSection()
}

// fields are the exponent fields of the tiny grid, from the denormals up
// to a few past the point where a square no longer underflows, then the
// rest of the range below one in big steps.
var fields = [...]int{
	0, 1, 2, 3, 5, 8, 12, 16, 20, 24, 26, 30, 40, 60, 100, 200, 300, 400, 480, 500, 505, 508, 510, 511, 512,
	513, 515, 520, 540, 600, 700, 800, 900, 950, 1000, 1010, 1015, 1020,
}

// mants are the fraction bits of the grid.
var mants = [...]uint64{0, 1 << 51, 1 << 50, 0x23456789abcde, 0xfffffffffffff}

type fn1 struct {
	name string
	f    func(float64) float64
	neg  bool
}

var fns1 = [...]fn1{
	{"exp", func(x float64) float64 { return yaroze.Exp(x) }, true},
	{"sin", func(x float64) float64 { return yaroze.Sin(x) }, true},
	{"cos", func(x float64) float64 { return yaroze.Cos(x) }, true},
	{"tan", func(x float64) float64 { return yaroze.Tan(x) }, true},
	{"atan", func(x float64) float64 { return yaroze.Atan(x) }, true},
	{"asin", func(x float64) float64 { return yaroze.Asin(x) }, true},
	{"sinh", func(x float64) float64 { return yaroze.Sinh(x) }, true},
	{"cosh", func(x float64) float64 { return yaroze.Cosh(x) }, true},
	{"tanh", func(x float64) float64 { return yaroze.Tanh(x) }, true},
	{"log", func(x float64) float64 { return yaroze.Log(x) }, false},
}

var group uint32

func start(name string) {
	group++
	mark(group)
	yaroze.ExitCriticalSection()
	ps("== ")
	ps(name)
	ps("\n")
	yaroze.EnterCriticalSection()
}

func runTiny(f *fn1) {
	start(f.name)
	for _, e := range fields {
		for _, m := range mants {
			x := bits(uint64(e)<<52 | m)
			record(f.name, x, 0, f.f(x))
			if f.neg {
				x = bits(1<<63 | uint64(e)<<52 | m)
				record(f.name, x, 0, f.f(x))
			}
		}
	}
	flush(f.name)
}

// scan calls f over count values from lo up by step, both made from
// bits so that the probe does no arithmetic of its own: lo and step are
// bit patterns added as integers, which walks the mantissa evenly.
func scan(f *fn1, lo uint64, step uint64, count int) {
	start(f.name)
	b := lo
	for range count {
		x := bits(b)
		record(f.name, x, 0, f.f(x))
		b += step
	}
	flush(f.name)
}

type fn2 struct {
	name string
	f    func(float64, float64) float64
}

var fns2 = [...]fn2{
	{"atan2", func(x, y float64) float64 { return yaroze.Atan2(x, y) }},
	{"hypot", func(x, y float64) float64 { return yaroze.Hypot(x, y) }},
	{"pow", func(x, y float64) float64 { return yaroze.Pow(x, y) }},
}

// pairs are the two-argument calls' tiny and structured inputs.
var pairs = [...][2]uint64{
	{0x0010000000000000, 0x3ff0000000000000}, {0x3ff0000000000000, 0x0010000000000000},
	{0x0010000000000000, 0x0010000000000000}, {0x0000000000000001, 0x3ff0000000000000},
	{0x01a56e1fc2f8f359, 0x3ff0000000000000},
	{0x3ff0000000000000, 0x01a56e1fc2f8f359}, {0x1000000000000000, 0x3ff0000000000000},
	{0x3ff0000000000000, 0x1000000000000000}, {0x2000000000000000, 0x3ff0000000000000},
	{0x3ff0000000000000, 0x2000000000000000}, {0x4000000000000000, 0x4000000000000000},
	{0x4000000000000000, 0x3fd0000000000000}, {0x4008000000000000, 0x4000000000000000},
	{0x4024000000000000, 0x3fd0000000000000}, {0x3fe0000000000000, 0x3fe0000000000000},
	{0x4000000000000000, 0x3fe0000000000000}, {0x4010000000000000, 0x3fe0000000000000},
	{0x4022000000000000, 0x3fe0000000000000}, {0x4000000000000000, 0x4008000000000000},
	{0x4000000000000000, 0x3ff8000000000000}, {0x3ff0000000000000, 0x3ff0000000000000},
	{0x3ff0000000000000, 0x4000000000000000}, {0x4000000000000000, 0xbff0000000000000},
	{0x4000000000000000, 0xc000000000000000}, {0x4024000000000000, 0xbff0000000000000},
	{0x4024000000000000, 0x4000000000000000}, {0x4024000000000000, 0x4008000000000000},
	{0x3ff8000000000000, 0x4000000000000000}, {0x4008000000000000, 0x4008000000000000},
	{0x3ff0000000000001, 0x4000000000000000}, {0x3fefffffffffffff, 0x4000000000000000},
	{0x4000000000000000, 0x3ff0000000000001}, {0x40c3880000000000, 0x3fe0000000000000},
	{0x4059000000000000, 0x4000000000000000}, {0x4059000000000000, 0xc000000000000000},
}

func runPairs(f *fn2) {
	start(f.name)
	for _, p := range pairs {
		x, y := bits(p[0]), bits(p[1])
		record(f.name, x, y, f.f(x, y))
	}
	flush(f.name)
}

func main() {
	yaroze.ResetGraph(0)
	ps("mathtiny probe\n")
	yaroze.EnterCriticalSection()
	for i := range fns1 {
		runTiny(&fns1[i])
	}
	// Dense scans: sin over [0.5, 1.6) in 120 steps, atan over [0.85, 1.05)
	// in 100 steps, tan over [0.7, 0.9) in 60, exp over [0.3, 0.4) in 40,
	// log over [0.7, 1.4) in 80, asin over [0.4, 0.95) in 60.
	scan(&fns1[1], 0x3fe0000000000000, 0x0000133333333333, 120)
	scan(&fns1[4], 0x3feb333333333333, 0x0000083126e978d5, 100)
	scan(&fns1[3], 0x3fe6666666666666, 0x0000111111111111, 60)
	scan(&fns1[0], 0x3fd3333333333333, 0x0000199999999999, 40)
	scan(&fns1[9], 0x3fe6666666666666, 0x0000199999999999, 80)
	scan(&fns1[5], 0x3fd999999999999a, 0x0000277777777777, 60)
	for i := range fns2 {
		runPairs(&fns2[i])
	}
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathtiny probe done\n")
	for {
		yaroze.VSync(0)
	}
}
