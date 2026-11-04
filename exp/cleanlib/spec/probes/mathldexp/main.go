//go:build yaroze

// Command mathldexp is a sixth probe for spec 12: ldexp over a grid of
// exponent fields and powers, the powers at and around every 1024
// boundary from 4096 down to -8192, further out at the 2048 and 4096
// boundaries to the ends of the int range, at the values that put the
// field plus the power on the edges of the exponent range, around the
// anomaly at 2^-1022 with -1075, and over dense scans of two ranges below
// -1023. It follows the rules of spec/probes/math: no floating point of
// its own (inputs from bits), nothing allocated, the call through a
// function value, interrupts off for the math, printing every 20 calls.
// The power -2147483648 is never called (Sony's never returns).
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

func hex32(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

func bits(b uint64) float64 { return math.Float64frombits(b) }

type entry struct {
	x, r uint64
	n    uint32
}

var (
	lg [20]entry
	n  int
)

func record(x uint64, p int32, r float64) {
	l := &lg[n]
	l.x, l.n, l.r = x, uint32(p), math.Float64bits(r)
	n++
	if n == len(lg) {
		flush()
	}
}

func flush() {
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &lg[i]
		ps("ldexp ")
		hex64(l.x)
		ps(" ")
		hex32(l.n)
		ps(" = ")
		hex64(l.r)
		ps("\n")
	}
	n = 0
	yaroze.EnterCriticalSection()
}

type fn struct {
	f func(float64, int32) float64
}

// The call goes through a function value so that the compiler cannot
// work it out itself (LLVM knows ldexp).
var fns = [...]fn{{func(x float64, n int32) float64 { return yaroze.Ldexp(x, n) }}}

func call(f *fn, x uint64, p int32) {
	if p == -2147483648 {
		return
	}
	record(x, p, f.f(bits(x), p))
}

// xs are the arguments: the fraction 0x23456789abcde at eleven exponent
// fields, both signs; the two zeros; the powers of two 2^-1022, 2^-1021
// and 2^-1020 (a zero fraction at the bottom fields, where the anomaly
// was seen), the smallest denormal pattern, one, and the infinity
// pattern, all both signs.
var xs = [...]uint64{
	0x00023456789abcde, 0x80023456789abcde, 0x00123456789abcde, 0x80123456789abcde,
	0x00223456789abcde, 0x80223456789abcde, 0x20023456789abcde, 0xa0023456789abcde,
	0x3fe23456789abcde, 0xbfe23456789abcde, 0x3ff23456789abcde, 0xbff23456789abcde,
	0x40023456789abcde, 0xc0023456789abcde, 0x60023456789abcde, 0xe0023456789abcde,
	0x7fd23456789abcde, 0xffd23456789abcde, 0x7fe23456789abcde, 0xffe23456789abcde,
	0x7ff23456789abcde, 0xfff23456789abcde,
	0x0000000000000000, 0x8000000000000000,
	0x0010000000000000, 0x8010000000000000, 0x0020000000000000, 0x8020000000000000,
	0x0030000000000000, 0x8030000000000000,
	0x0000000000000001, 0x8000000000000001, 0x3ff0000000000000, 0xbff0000000000000,
	0x7ff0000000000000, 0xfff0000000000000,
}

// far are the powers beyond the 1024 boundaries, to the ends of the int
// range, with the two that exp hands ldexp for -1e6 and -6.4e7.
var far = [...]int32{
	-16384, -32768, -65536, -100000, -1442695, -92332570, -1 << 20, -1 << 24, -1 << 30, -2147483647,
	65536, 1 << 30, 2147483647,
}

// targets are the values of field plus power to hit for each argument.
var targets = [...]int32{0, 1, 1023, 1024, 2046, 2047, 2048, -1, -1024, -1025, -2048, -2049, -3072, -3073, -4096}

func runGrid(f *fn) {
	for _, x := range xs {
		field := int32(x >> 52 & 0x7ff)
		for k := int32(4); k >= -8; k-- {
			for d := int32(-3); d <= 3; d++ {
				call(f, x, k*1024+d)
			}
		}
		for _, p := range far {
			call(f, x, p)
		}
		for p := int32(-1073); p >= -1077; p-- {
			call(f, x, p)
		}
		for _, t := range targets {
			call(f, x, t-field)
		}
		flush()
	}
}

// scan calls ldexp on x for every power from lo down to hi.
func scan(f *fn, x uint64, lo, hi int32) {
	for p := lo; p >= hi; p-- {
		call(f, x, p)
	}
	flush()
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

func main() {
	yaroze.ResetGraph(0)
	ps("mathldexp probe\n")
	yaroze.EnterCriticalSection()
	f := &fns[0]
	start("grid")
	runGrid(f)
	start("scan")
	for _, x := range [...]uint64{0x3ff23456789abcde, 0x00023456789abcde, 0x0010000000000000} {
		scan(f, x, -1000, -1100)
		scan(f, x, -2030, -2130)
	}
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathldexp probe done\n")
	for {
		yaroze.VSync(0)
	}
}
