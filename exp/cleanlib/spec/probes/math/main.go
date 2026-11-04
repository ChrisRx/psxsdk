//go:build yaroze

// Command math is a probe for spec 12: the library's software floating
// point helpers, the double precision math calls, atof and strtod, and
// _get_errno, over fixed inputs, each result printed as the hex digits of
// its bits. Results are logged in memory, with _get_errno read right
// after each call, and printed after each group. The last group makes
// failing and succeeding file calls on the memory cards the harness puts
// in, printing _get_errno after each. Built for the resident library it
// shows Sony's behavior. Spec 12 says how to run it and what it must
// avoid doing (floating point of its own, allocation, calls the compiler
// can work out).
package main

import (
	"math"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export _get_errno
func getErrnoLib() int32

// withErrno reads _get_errno after each call. The kernel here
// (OpenBIOS) has no B0h 54h, so a harness must stand in for it.
const withErrno = true

func getErrno() int32 {
	if withErrno {
		return getErrnoLib()
	}
	return 0
}

// The library's software floating point, which the compiler calls for
// every float64 and float32 operation, called directly here so that the
// results, comparisons' included, come back raw.

//export ldexp
func ldexpRaw(x float64, n int32) float64

//export __adddf3
func adddf3(a, b float64) float64

//export __subdf3
func subdf3(a, b float64) float64

//export __muldf3
func muldf3(a, b float64) float64

//export __divdf3
func divdf3(a, b float64) float64

//export __negdf2
func negdf2(a float64) float64

//export __eqdf2
func eqdf2(a, b float64) int32

//export __nedf2
func nedf2(a, b float64) int32

//export __ltdf2
func ltdf2(a, b float64) int32

//export __ledf2
func ledf2(a, b float64) int32

//export __gtdf2
func gtdf2(a, b float64) int32

//export __gedf2
func gedf2(a, b float64) int32

//export __fixdfsi
func fixdfsi(a float64) int32

//export __floatsidf
func floatsidf(a int32) float64

//export __addsf3
func addsf3(a, b float32) float32

//export __mulsf3
func mulsf3(a, b float32) float32

//export __divsf3
func divsf3(a, b float32) float32

//export __extendsfdf2
func extendsfdf2(a float32) float64

//export __truncdfsf2
func truncdfsf2(a float64) float32

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

const hexDigits = "0123456789abcdef"

// line is the line being printed, written to the TTY (file 1) whole,
// which is much faster in the emulator than a character at a time.
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

func putInt(v int64) {
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

// ps and pi print a string and an integer. Nothing here allocates: the
// target's allocator never frees, and a heap grown up into the memory
// Sony's library works in breaks it (sqrt never returned, a long way
// into the probe, while every print boxed its arguments).
func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int64) { putInt(v) }

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

// seed is the probe's pseudo-random sequence, a 32-bit xorshift.
var seed uint32 = 0x12345678

func rnd() uint32 {
	seed ^= seed << 13
	seed ^= seed >> 17
	seed ^= seed << 5
	return seed
}

func mant() uint64 { return uint64(rnd()&0xfffff)<<32 | uint64(rnd()) }

// ranged returns a random double with an exponent from lo to hi (powers
// of two), and a random sign unless pos.
func ranged(lo, hi int, pos bool) float64 {
	e := lo + int(rnd()%uint32(hi-lo+1))
	b := uint64(e+1023)<<52 | mant()
	if !pos && rnd()&1 != 0 {
		b |= 1 << 63
	}
	return math.Float64frombits(b)
}

// wide returns a double with any bits but NaN's.
func wide() float64 {
	for {
		b := uint64(rnd())<<32 | uint64(rnd())
		if b>>52&0x7ff != 0x7ff {
			return math.Float64frombits(b)
		}
	}
}

func bits(b uint64) float64 { return math.Float64frombits(b) }

func neg(x float64) float64 { return bits(math.Float64bits(x) ^ 1<<63) }

// fromInt returns v times 2 to the -shift, exactly, built from its bits
// so that the probe does no floating point of its own.
func fromInt(v int64, shift int) float64 {
	if v == 0 {
		return 0
	}
	var sign uint64
	if v < 0 {
		sign, v = 1<<63, -v
	}
	m := uint64(v)
	e := 52
	for m >= 1<<53 {
		m >>= 1
		e++
	}
	for m < 1<<52 {
		m <<= 1
		e--
	}
	return bits(sign | uint64(e-shift+1023)<<52 | m&(1<<52-1))
}

// edges are the simple inputs and the edges every one-argument call gets.
var edges = [...]float64{
	0, bits(1 << 63), 0.5, -0.5, 1, -1, 2, -2, math.Pi / 4, -math.Pi / 4,
	math.Pi / 2, -math.Pi / 2, math.Pi, -math.Pi, 2 * math.Pi, 10, -10, 100, 1e10, -1e10,
	bits(1), bits(0x8000000000000001), bits(0x000fffffffffffff), bits(0x0010000000000000),
	bits(0x8010000000000000), bits(0x7fefffffffffffff), bits(0xffefffffffffffff),
	math.Inf(1), math.Inf(-1),
	bits(0x7ff8000000000000), bits(0xfff8000000000000), bits(0x7ff0000000000001), bits(0x7ff4000000000000),
	bits(0x3ff0000000000001), bits(0xbff0000000000001), bits(0x3fefffffffffffff), bits(0xbfefffffffffffff),
	1e6, -1e6, 1e15, 1e300, -1e300, 1e-300, 1e-20, 1e-8, 3, 0.25, 0.75, 1.5, 4, 1e5,
	700, 709, 709.78, 710, -700, -708, -709, -745, -746, 20, 22, -22, 30, 40, 1e22, 1e100,
	7.0, 0.1, 0.2, 0.3, 1.0 / 3, math.E, math.Ln2, 1 - 1e-9, 1 + 1e-9, 4503599627370496.5, 4503599627370497,
	2.5, -2.5, 3.5, -3.5, 0.49999999999999994, -0.49999999999999994, 9007199254740993, 1e16,
	0.999, 0.9999999, -0.999, 0.6, -0.6, 0.4, 0.43, 0.45,
}

type entry struct {
	a, b, r uint64
	e       int32
	x       [4]uint32
}

var (
	lg [400]entry
	n  int
)

func record(a, b, r float64, e int32) *entry {
	l := &lg[n]
	l.a, l.b, l.r, l.e = math.Float64bits(a), math.Float64bits(b), math.Float64bits(r), e
	n++
	return l
}

var group uint32

// startAt skips the groups before it, to find hangs faster.
const startAt = 1

func start(name string) bool {
	group++
	mark(group)
	n = 0
	// Each group has its own sequence, so that groups can be skipped.
	seed = 0x12345678
	for i := 0; i < len(name); i++ {
		seed = (seed ^ uint32(name[i])) * 16777619
	}
	if group < startAt {
		return false
	}
	ps("== ")
	pi(int64(group))
	ps(" ")
	ps(name)
	ps(" errno ")
	pi(int64(getErrno()))
	ps("\n")
	return true
}

func flush(name string, two bool, extra int) {
	// Printing goes through the kernel, which lets interrupts back in:
	// the calls are made with them off, and printed after.
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &lg[i]
		ps(name)
		ps(" ")
		hex64(l.a)
		if two {
			ps(" ")
			hex64(l.b)
		}
		ps(" = ")
		hex64(l.r)
		ps(" ")
		pi(int64(l.e))
		for j := range extra {
			ps(" ")
			hex32(l.x[j])
		}
		ps("\n")
	}
	n = 0
	yaroze.EnterCriticalSection()
}

// verbose prints each call's arguments before it is made, to find the
// calls that never return.
const verbose = false

// hangs are the calls that never return on Sony's library, skipped: the
// result is printed as deaddeaddeaddead.
var hangs = [...]struct {
	name string
	a, b uint64
}{
	{"sin", 0xfff0000000000000, 0},
	{"float", 0x0000000080000000, 0},
	{"pow", 0x3fb999999999999a, 0x7fefffffffffffff},
	{"pow", 0x3fb999999999999a, 0x7e37e43c8800759c},
	{"pow", 0x3fb999999999999a, 0x7ff8000000000000},
	{"pow", 0x3fb999999999999a, 0x7ff0000000000000},
	{"pow", 0x3fb999999999999a, 0x8000000000000000},
	{"pow", 0xc090cc0000000000, 0x8000000000000000},
	{"pow", 0x4090000000000000, 0xfe37e43c8800759c},
	{"pow", 0x4090000000000000, 0xfff0000000000000},
	{"pow", 0x4090000000000000, 0x8000000000000000},
	{"pow", 0x7fefffffffffffff, 0xfe37e43c8800759c},
	{"pow", 0x7fefffffffffffff, 0xfff0000000000000},
	{"pow", 0x7fefffffffffffff, 0x8000000000000000},
	{"pow", 0x0000000000000001, 0x7fefffffffffffff},
	{"pow", 0x0000000000000001, 0x7e37e43c8800759c},
	{"pow", 0x0000000000000001, 0x7ff8000000000000},
	{"pow", 0x0000000000000001, 0x7ff0000000000000},
	{"pow", 0x0000000000000001, 0x8000000000000000},
	{"pow", 0x01a56e1fc2f8f359, 0x7fefffffffffffff},
	{"pow", 0x01a56e1fc2f8f359, 0x7e37e43c8800759c},
	{"pow", 0x01a56e1fc2f8f359, 0x7ff8000000000000},
	{"pow", 0x01a56e1fc2f8f359, 0x7ff0000000000000},
	{"pow", 0x01a56e1fc2f8f359, 0x8000000000000000},
	{"pow", 0xfe37e43c8800759c, 0x8000000000000000},
	{"pow", 0x7e37e43c8800759c, 0xfe37e43c8800759c},
	{"pow", 0x7e37e43c8800759c, 0xfff0000000000000},
	{"pow", 0x7e37e43c8800759c, 0x8000000000000000},
	{"pow", 0x7ff8000000000000, 0xfe37e43c8800759c},
	{"pow", 0x7ff8000000000000, 0xfff0000000000000},
	{"pow", 0x7ff8000000000000, 0x8000000000000000},
	{"pow", 0xfff0000000000000, 0x8000000000000000},
	{"pow", 0x7ff0000000000000, 0xfe37e43c8800759c},
	{"pow", 0x7ff0000000000000, 0xfff0000000000000},
	{"pow", 0x7ff0000000000000, 0x8000000000000000},
	{"pow", 0x3ff8000000000000, 0x0000000000000001},
	{"pow", 0x3ff8000000000000, 0xfe37e43c8800759c},
	{"pow", 0x3ff8000000000000, 0xfff0000000000000},
	{"pow", 0x3ff8000000000000, 0x8000000000000000},
	{"pow", 0x4024000000000000, 0xfe37e43c8800759c},
	{"pow", 0x4024000000000000, 0xfff0000000000000},
	{"pow", 0x4024000000000000, 0x8000000000000000},
	{"pow", 0x3fd5555555555555, 0x7fefffffffffffff},
	{"pow", 0x3fd5555555555555, 0x7e37e43c8800759c},
	{"pow", 0x3fd5555555555555, 0x7ff8000000000000},
	{"pow", 0x3fd5555555555555, 0x7ff0000000000000},
	{"pow", 0x3fd5555555555555, 0x8000000000000000},
	{"pow", 0xc020000000000000, 0x8000000000000000},
	{"pow", 0xc008000000000000, 0x8000000000000000},
	{"pow", 0x4008000000000000, 0xfe37e43c8800759c},
	{"pow", 0x4008000000000000, 0xfff0000000000000},
	{"pow", 0x4008000000000000, 0x8000000000000000},
	{"pow", 0xbfe0000000000000, 0x8000000000000000},
	{"pow", 0x3fe0000000000000, 0x7fefffffffffffff},
	{"pow", 0x3fe0000000000000, 0x0000000000000001},
	{"pow", 0x3fe0000000000000, 0x7e37e43c8800759c},
	{"pow", 0x3fe0000000000000, 0x7ff8000000000000},
	{"pow", 0x3fe0000000000000, 0x7ff0000000000000},
	{"pow", 0x3fe0000000000000, 0x8000000000000000},
	{"pow", 0xc000000000000000, 0x8000000000000000},
	{"pow", 0x4000000000000000, 0x0000000000000001},
	{"pow", 0x4000000000000000, 0xfe37e43c8800759c},
	{"pow", 0x4000000000000000, 0xfff0000000000000},
	{"pow", 0x4000000000000000, 0x8000000000000000},
	{"pow", 0xbff0000000000000, 0x8000000000000000},
	{"pow", 0x3ff0000000000000, 0x8000000000000000},
}

const hung = 0xdeaddeaddeaddead

func hangs1(name string, a, b float64) bool {
	ab, bb := math.Float64bits(a), math.Float64bits(b)
	// The tests are on the bits, so that the probe itself does no
	// floating point: in a build for Sony's library that would run on
	// Sony's arithmetic.
	mag, neg := ab&^(1<<63), ab>>63 != 0
	nan := mag > 0x7ff0000000000000
	switch name {
	case "cos":
		// Never returns for a magnitude past pi times 2^31, nor for NaN.
		if nan || mag >= math.Float64bits(6.7465e9) {
			return true
		}
	case "sin", "tan":
		// Never return for a negative argument below -pi times 2^31.
		if neg && mag >= math.Float64bits(6.7465e9) {
			return true
		}
	case "tanh":
		if nan || mag >= math.Float64bits(7e8) {
			return true
		}
	case "ldexp":
		// Never returns for a power of -2^31.
		if bb == 0x80000000 {
			return true
		}
	case "exp", "cosh", "sinh":
		// Never return for a large negative argument.
		if neg && mag >= math.Float64bits(1.4e9) {
			return true
		}
	}
	for i := range hangs {
		h := &hangs[i]
		if h.name == name && h.a == ab && h.b == bb {
			return true
		}
	}
	if verbose {
		ps("? ")
		ps(name)
		ps(" ")
		hex64(ab)
		ps(" ")
		hex64(bb)
		ps("\n")
	}
	return false
}

// cur is the argument of the call in progress, for a harness to read
// when a call never returns.
var cur uint64

func call1(f *fn1, x float64) float64 {
	if hangs1(f.name, x, 0) {
		return bits(hung)
	}
	volatile.StoreUint64(&cur, math.Float64bits(x))
	return f.f(x)
}

func call2(f *fn2, x, y float64) float64 {
	if hangs1(f.name, x, y) {
		return bits(hung)
	}
	return f.f(x, y)
}

type fn1 struct {
	name   string
	f      func(float64) float64
	lo, hi int
	pos    bool
}

var fns1 = [...]fn1{
	{"acos", func(x float64) float64 { return yaroze.Acos(x) }, -30, -1, false},
	{"asin", func(x float64) float64 { return yaroze.Asin(x) }, -30, -1, false},
	{"atan", func(x float64) float64 { return yaroze.Atan(x) }, -30, 60, false},
	{"ceil", func(x float64) float64 { return yaroze.Ceil(x) }, -10, 60, false},
	{"cos", func(x float64) float64 { return yaroze.Cos(x) }, -30, 20, false},
	{"cosh", func(x float64) float64 { return yaroze.Cosh(x) }, -30, 9, false},
	{"exp", func(x float64) float64 { return yaroze.Exp(x) }, -30, 9, false},
	{"floor", func(x float64) float64 { return yaroze.Floor(x) }, -10, 60, false},
	{"log", func(x float64) float64 { return yaroze.Log(x) }, -1022, 1023, true},
	{"log10", func(x float64) float64 { return yaroze.Log10(x) }, -1022, 1023, true},
	{"sin", func(x float64) float64 { return yaroze.Sin(x) }, -30, 20, false},
	{"sinh", func(x float64) float64 { return yaroze.Sinh(x) }, -30, 9, false},
	{"sqrt", func(x float64) float64 { return yaroze.Sqrt(x) }, -1022, 1023, true},
	{"tan", func(x float64) float64 { return yaroze.Tan(x) }, -30, 20, false},
	{"tanh", func(x float64) float64 { return yaroze.Tanh(x) }, -30, 9, false},
}

func run1(f *fn1) {
	if !start(f.name) {
		return
	}
	for _, x := range edges {
		record(x, 0, call1(f, x), getErrno())
	}
	flush(f.name, false, 0)
	for range 15 {
		for range 20 {
			x := ranged(f.lo, f.hi, f.pos)
			record(x, 0, call1(f, x), getErrno())
		}
		flush(f.name, false, 0)
	}
	// The ordinary range near 1, where the logs and powers work hardest.
	for range 100 {
		x := ranged(-2, 1, f.pos)
		record(x, 0, call1(f, x), getErrno())
	}
	for range 60 {
		x := wide()
		record(x, 0, call1(f, x), getErrno())
	}
	flush(f.name, false, 0)
}

type fn2 struct {
	name   string
	f      func(float64, float64) float64
	lo, hi int
}

var fns2 = [...]fn2{
	{"atan2", func(x, y float64) float64 { return yaroze.Atan2(x, y) }, -20, 20},
	{"fmod", func(x, y float64) float64 { return yaroze.Fmod(x, y) }, -10, 60},
	{"hypot", func(x, y float64) float64 { return yaroze.Hypot(x, y) }, -500, 500},
	{"pow", func(x, y float64) float64 { return yaroze.Pow(x, y) }, -8, 8},
}

var edges2 = [...]float64{
	0, bits(1 << 63), 1, -1, 2, -2, 0.5, -0.5, 3, -3, -8, 1.0 / 3, 10, 1.5,
	math.Inf(1), math.Inf(-1), bits(0x7ff8000000000000), 1e300, -1e300, 1e-300, bits(1),
	bits(0x7fefffffffffffff), 1024, -1075, 0.1,
}

func run2(f *fn2) {
	if !start(f.name) {
		return
	}
	for _, x := range edges2 {
		for _, y := range edges2 {
			record(x, y, call2(f, x, y), getErrno())
			if n == len(lg) {
				flush(f.name, true, 0)
			}
		}
	}
	flush(f.name, true, 0)
	for range 3 {
		for range 100 {
			x := ranged(f.lo, f.hi, false)
			y := ranged(f.lo, f.hi, false)
			if f.name == "pow" {
				x = ranged(-8, 8, true)
				y = ranged(-6, 5, false)
			}
			record(x, y, call2(f, x, y), getErrno())
		}
		flush(f.name, true, 0)
	}
	if f.name == "pow" {
		// Negative bases with integer powers, and powers of ten and two.
		for i := range 100 {
			x := neg(ranged(-4, 4, true))
			if i < 20 {
				x = fromInt(-int64(i)-1, 0)
			}
			y := fromInt(int64(rnd()%61)-30, 0)
			record(x, y, call2(f, x, y), getErrno())
		}
		for i := range 60 {
			y := fromInt(int64(i)-30, 0)
			h := fromInt(2*(int64(i)-30)+1, 1)
			record(10, y, call2(f, 10, y), getErrno())
			record(2, h, call2(f, 2, h), getErrno())
		}
		flush(f.name, true, 0)
	}
	for range 60 {
		x, y := wide(), wide()
		if f.name == "pow" {
			// Most wide powers never return: keep them moderate.
			x, y = ranged(-60, 60, true), ranged(-30, 4, false)
		}
		record(x, y, call2(f, x, y), getErrno())
	}
	flush(f.name, true, 0)
}

func runSplit() {
	if !start("frexp") {
		return
	}
	var ex [4]int32
	for _, x := range edges {
		for i := range ex {
			ex[i] = -0x55555556
		}
		r := bits(hung)
		if !hangs1("frexp", x, 0) {
			r = yaroze.Frexp(x, &ex[1])
		}
		l := record(x, 0, r, getErrno())
		for i := range ex {
			l.x[i] = uint32(ex[i])
		}
	}
	flush("frexp", false, 4)
	for range 100 {
		x := wide()
		r := bits(hung)
		if !hangs1("frexp", x, 0) {
			r = yaroze.Frexp(x, &ex[1])
		}
		l := record(x, 0, r, getErrno())
		l.x[1] = uint32(ex[1])
	}
	flush("frexp", false, 4)

}

func runModf() {
	if !start("modf") {
		return
	}
	var ip [3]float64
	for _, x := range edges {
		for i := range ip {
			ip[i] = bits(0xaaaaaaaaaaaaaaaa)
		}
		r := bits(hung)
		if !hangs1("modf", x, 0) {
			r = yaroze.Modf(x, &ip[1])
		}
		l := record(x, 0, r, getErrno())
		for i := range ip {
			b := math.Float64bits(ip[i])
			l.x[i] = uint32(b >> 32)
			if i == 1 {
				l.x[3] = uint32(b)
			}
		}
	}
	flush("modf", false, 4)
	for range 100 {
		x := ranged(-10, 60, false)
		r := bits(hung)
		if !hangs1("modf", x, 0) {
			r = yaroze.Modf(x, &ip[1])
		}
		l := record(x, 0, r, getErrno())
		b := math.Float64bits(ip[1])
		l.x[0], l.x[1] = uint32(b>>32), uint32(b)
	}
	flush("modf", false, 2)

}

func runLdexp() {
	if !start("ldexp") {
		return
	}
	var exps = [...]int32{0, 1, -1, 10, -10, 52, -52, 1023, -1022, 1024, -1074, -1075, 2000, -2000, 0x7fffffff, -0x80000000}
	for _, x := range edges {
		for _, e := range exps {
			r := bits(hung)
			if !hangs1("ldexp", x, bits(uint64(uint32(e)))) {
				r = yaroze.Ldexp(x, e)
			}
			l := record(x, 0, r, getErrno())
			l.x[0] = uint32(e)
			if n == len(lg) {
				flush("ldexp", false, 1)
			}
		}
	}
	flush("ldexp", false, 1)
	for range 200 {
		x := wide()
		e := int32(rnd()%2400) - 1200
		r := bits(hung)
		if !hangs1("ldexp", x, bits(uint64(uint32(e)))) {
			r = yaroze.Ldexp(x, e)
		}
		l := record(x, 0, r, getErrno())
		l.x[0] = uint32(e)
	}
	flush("ldexp", false, 1)
}

var strs = [...]string{
	"0", "1", "-1", "+1", "  12.5", "\t\n\v\f\r 3", "1e10", "1E-10", "1.5e+3", "-.5", ".5", "5.", ".", "e5",
	"1e", "1e+", "1e-", "1ex", "0x10", "0X1p3", "0x", "inf", "INF", "-Inf", "infinity", "nan", "NaN", "-nan",
	"nan(123)", "1e309", "-1e309", "1e-400", "-1e-400", "1e-310", "2.2250738585072014e-308",
	"2.2250738585072011e-308", "4.9e-324", "5e-324", "2e-324", "3e-324", "1.7976931348623157e308",
	"1.7976931348623158e308", "1.7976931348623159e308", "123456789012345678901234567890",
	"0.000000000000000000000000001234", "3.14159265358979323846", "0.1", "0.2", "0.3", "1.1",
	"9007199254740993", "9007199254740995", "", "   ", "abc", "-", "+", "+-1", "--1", "1.2.3", "12abc", "1,5",
	"00012", "1e0010", "1e99999", "1e-99999", "0.1e1", "2.4703282292062327e-324", "2.4703282292062328e-324",
	"1.00000000000000011102230246251565404236316680908203125",
	"1.00000000000000011102230246251565404236316680908203126",
	"0.30000000000000004", "123.456", "6.02214076e23", "1e23", "8.98846567431158e307", "1e22", "1e-5",
	"100000000000000000000000", "0.0000001", "1.5e-45", "3.4028235e38", "65535", "4294967296", "-0",
	"-0.0e5", "0e999999", "000000000000000000000000000000000001", "1.000000000000000000000000000001",
	"12345678901234567", "123456789012345678", "0.1234567890123456789", "7e-1", "1 2", "1\t", "+.e1",
	"  -  1", "1e+0", "1E", "99999999999999999999e-20", "1e-307", "1e-308", "1e-320", "1e308", "2e308",
	"4.940656458412465441765687928682213723651e-324", "inF", "nAn", "i", "in", "na",
}

var sbuf [80]byte

func cstr(s string) *byte {
	k := copy(sbuf[:79], s)
	sbuf[k] = 0
	return &sbuf[0]
}

func showStr(s string) {
	ps("\"")
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c < 0x7f {
			putc(c)
		} else {
			ps("\\")
			pi(int64(c))
		}
	}
	ps("\"")
}

func parse(s string) {
	if verbose {
		ps("? strtod ")
		showStr(s)
		ps("\n")
	}
	p := cstr(s)
	end := (*byte)(unsafe.Pointer(uintptr(0xdeadbeef)))
	r := yaroze.Strtod(p, &end)
	e := getErrno()
	r2 := yaroze.Strtod(p, nil)
	e2 := getErrno()
	r3 := yaroze.Atof(p)
	e3 := getErrno()
	ps("strtod ")
	showStr(s)
	ps(" = ")
	hex64(math.Float64bits(r))
	ps(" end ")
	pi(int64(int32(uintptr(unsafe.Pointer(end)) - uintptr(unsafe.Pointer(p)))))
	ps(" ")
	pi(int64(e))
	ps(" nil ")
	hex64(math.Float64bits(r2))
	ps(" ")
	pi(int64(e2))
	ps(" atof ")
	hex64(math.Float64bits(r3))
	ps(" ")
	pi(int64(e3))
	ps("\n")
}

var rbuf [48]byte

// randomNumber makes a decimal string of random digits, point and
// exponent.
func randomNumber() string {
	k := 0
	if rnd()&3 == 0 {
		rbuf[k] = '-'
		k++
	}
	digits := 1 + int(rnd()%20)
	point := int(rnd() % uint32(digits+1))
	for i := range digits {
		if i == point && rnd()&1 == 0 {
			rbuf[k] = '.'
			k++
		}
		rbuf[k] = byte('0' + rnd()%10)
		k++
	}
	if rnd()&1 == 0 {
		rbuf[k] = 'e'
		k++
		ex := int(rnd()%700) - 350
		if ex < 0 {
			rbuf[k] = '-'
			k++
			ex = -ex
		}
		var d [3]byte
		nd := 0
		for {
			d[nd] = byte('0' + ex%10)
			nd++
			ex /= 10
			if ex == 0 {
				break
			}
		}
		for i := nd - 1; i >= 0; i-- {
			rbuf[k] = d[i]
			k++
		}
	}
	return unsafe.String(&rbuf[0], k)
}

func runParse() {
	if !start("strtod") {
		return
	}
	for _, s := range strs {
		parse(s)
	}
	for range 200 {
		parse(randomNumber())
	}
}

var pathBuf [32]byte

func path(s string) *byte {
	k := copy(pathBuf[:31], s)
	pathBuf[k] = 0
	return &pathBuf[0]
}

func say(what string, r int32) {
	ps(what)
	ps(" = ")
	pi(int64(r))
	ps(" errno ")
	pi(int64(getErrno()))
	ps("\n")
}

func runFiles() {
	if !start("files") {
		return
	}
	say("lseek 99", yaroze.Lseek(99, 0, 0))
	say("read 99", yaroze.Read(99, &sbuf[0], 128))
	say("write 99", yaroze.Write(99, &sbuf[0], 128))
	say("close 99", yaroze.Close(99))
	say("close -1", yaroze.Close(-1))
	say("sqrt -1 after", int32(math.Float64bits(yaroze.Sqrt(-1))>>32))
	say("open bu00:MISSING", yaroze.Open(path("bu00:MISSING"), yaroze.O_RDONLY))
	say("open bu10:MISSING", yaroze.Open(path("bu10:MISSING"), yaroze.O_RDONLY))
	say("open bu20:X", yaroze.Open(path("bu20:X"), yaroze.O_RDONLY))
	say("open xyz:X", yaroze.Open(path("xyz:X"), yaroze.O_RDONLY))
	say("delete bu00:MISSING", yaroze.Delete(path("bu00:MISSING")))
	say("rename bu00:MISSING", yaroze.Rename(path("bu00:MISSING"), path("bu00:OTHER")))
	var dir yaroze.DIRENTRY
	say("firstfile bu00:MISSING", int32(uintptr(unsafe.Pointer(yaroze.Firstfile(path("bu00:MISSING"), &dir)))))
	say("format bu20:", yaroze.Format(path("bu20:")))
	say("format bu00:", yaroze.Format(path("bu00:")))
	fd := yaroze.Open(path("bu00:FILE"), yaroze.O_CREAT|1<<16)
	say("open create bu00:FILE", fd)
	say("close", yaroze.Close(fd))
	say("open create bu00:FILE again", yaroze.Open(path("bu00:FILE"), yaroze.O_CREAT|1<<16))
	fd = yaroze.Open(path("bu00:FILE"), yaroze.O_RDWR)
	say("open bu00:FILE", fd)
	say("write 128", yaroze.Write(fd, &sbuf[0], 128))
	say("write 100", yaroze.Write(fd, &sbuf[0], 100))
	say("lseek 0", yaroze.Lseek(fd, 0, 0))
	say("read 128", yaroze.Read(fd, &sbuf[0], 128))
	say("lseek 9000", yaroze.Lseek(fd, 9000, 0))
	say("write past", yaroze.Write(fd, &sbuf[0], 128))
	say("close", yaroze.Close(fd))
	say("close again", yaroze.Close(fd))
	// Fill the card: 15 blocks in all.
	var name = [...]byte{'b', 'u', '0', '0', ':', 'F', 'A', 0}
	for i := range 16 {
		name[6] = byte('A' + i)
		fd := yaroze.Open(&name[0], yaroze.O_CREAT|1<<16)
		ps("create ")
		ps(unsafe.String(&name[5], 2))
		ps(" = ")
		pi(int64(fd))
		ps(" errno ")
		pi(int64(getErrno()))
		ps("\n")
		if fd >= 0 {
			yaroze.Close(fd)
		}
	}
	say("open create big", yaroze.Open(path("bu00:BIG"), yaroze.O_CREAT|3<<16))
	say("delete bu00:FILE", yaroze.Delete(path("bu00:FILE")))
	say("delete bu00:FILE again", yaroze.Delete(path("bu00:FILE")))
	say("rename FA FB", yaroze.Rename(path("bu00:FA"), path("bu00:FB")))
	say("open create bu00:NEW 2", yaroze.Open(path("bu00:NEW"), yaroze.O_CREAT|2<<16))
	say("lseek 99", yaroze.Lseek(99, 0, 0))
	say("TestCard 0", yaroze.TestCard(0))
	say("lseek 99", yaroze.Lseek(99, 0, 0))
}

// primEdges are the operands every arithmetic primitive gets in pairs.
var primEdges = [...]uint64{
	0, 1 << 63, 0x3ff0000000000000, 0xbff0000000000000, 0x3ff8000000000000, 0x4000000000000000,
	0x4008000000000000, 0x3fb999999999999a, 0x3fd5555555555555, 0x400921fb54442d18,
	0x3ff0000000000001, 0x3fefffffffffffff, 0x7fe0000000000000, 0x7fefffffffffffff,
	0xffefffffffffffff, 0x0010000000000000, 0x0000000000000001, 0x000fffffffffffff,
	0x0008000000000000, 0x8000000000000001, 0x7ff0000000000000, 0xfff0000000000000,
	0x7ff8000000000000, 0xfff8000000000000, 0x7ff0000000000001, 0x7e37e43c8800759c,
	0x01a56e1fc2f8f359, 0x0170000000000000, 0x7e70000000000000, 0x2270000000000000,
	0x5d70000000000000, 0x5ff0000000000000, 0x1ff0000000000000, 0x4340000000000000,
	0x3ca0000000000000, 0xc340000000000000,
}

type prim struct {
	name string
	f    func(a, b float64) float64
}

var prims = [...]prim{
	{"add", func(a, b float64) float64 { return adddf3(a, b) }},
	{"sub", func(a, b float64) float64 { return subdf3(a, b) }},
	{"mul", func(a, b float64) float64 { return muldf3(a, b) }},
	{"div", func(a, b float64) float64 { return divdf3(a, b) }},
}

// near returns a double with random mantissa and an exponent within a
// few of e, and a random sign.
func near(e int) uint64 {
	e += int(rnd()%9) - 4
	return uint64(e&0x7ff)<<52 | mant() | uint64(rnd()&1)<<63
}

func runPrim(p *prim) {
	if !start(p.name) {
		return
	}
	for _, a := range primEdges {
		for _, b := range primEdges {
			record(bits(a), bits(b), primCall(p, bits(a), bits(b)), 0)
			if n == len(lg) {
				flush(p.name, true, 0)
			}
		}
	}
	flush(p.name, true, 0)
	// Rounding: random operands near each other's size, and with the
	// low bits set, which sticky bits and ties need.
	for range 300 {
		a := near(1023)
		b := near(1023)
		if rnd()&3 == 0 {
			b = a&^0xffff | uint64(rnd()&0xffff)
		}
		if rnd()&3 == 0 {
			b = b&^(0x7ff<<52) | uint64(1023+int(rnd()%60)-30)<<52
		}
		record(bits(a), bits(b), primCall(p, bits(a), bits(b)), 0)
	}
	flush(p.name, true, 0)
	// Overflow and underflow: exponents near the ends.
	for range 200 {
		var a, b uint64
		switch rnd() % 4 {
		case 0:
			a, b = near(1000), near(1000)
		case 1:
			a, b = near(30), near(30)
		case 2:
			a, b = near(1000), near(30)
		default:
			a, b = near(4), near(1020)
		}
		record(bits(a), bits(b), primCall(p, bits(a), bits(b)), 0)
	}
	flush(p.name, true, 0)
}

func primCall(p *prim, a, b float64) float64 {
	if hangs1(p.name, a, b) {
		return bits(hung)
	}
	return p.f(a, b)
}

func runCompare() {
	if !start("cmp") {
		return
	}
	for _, a := range primEdges {
		for _, b := range primEdges {
			x, y := bits(a), bits(b)
			ps("cmp ")
			hex64(a)
			ps(" ")
			hex64(b)
			ps(" eq ")
			pi(int64(eqdf2(x, y)))
			ps(" ne ")
			pi(int64(nedf2(x, y)))
			ps(" lt ")
			pi(int64(ltdf2(x, y)))
			ps(" le ")
			pi(int64(ledf2(x, y)))
			ps(" gt ")
			pi(int64(gtdf2(x, y)))
			ps(" ge ")
			pi(int64(gedf2(x, y)))
			ps("\n")
		}
	}
	for range 100 {
		a := near(1023)
		b := a ^ uint64(rnd()&0xff)
		if rnd()&1 == 0 {
			b ^= 1 << 63
		}
		x, y := bits(a), bits(b)
		ps("cmp ")
		hex64(a)
		ps(" ")
		hex64(b)
		ps(" eq ")
		pi(int64(eqdf2(x, y)))
		ps(" ne ")
		pi(int64(nedf2(x, y)))
		ps(" lt ")
		pi(int64(ltdf2(x, y)))
		ps(" le ")
		pi(int64(ledf2(x, y)))
		ps(" gt ")
		pi(int64(gtdf2(x, y)))
		ps(" ge ")
		pi(int64(gedf2(x, y)))
		ps("\n")
	}
}

var fixInputs = [...]uint64{
	0, 1 << 63, 0x3fe0000000000000, 0xbfe0000000000000, 0x3fefffffffffffff, 0x3ff0000000000000,
	0xbff0000000000000, 0x3ff8000000000000, 0xbff8000000000000, 0x4004000000000000, 0xc004000000000000,
	0x41dfffffffc00000, 0xc1e0000000000000, 0x41dfffffffffffff, 0x41e0000000000000, 0xc1e0000000200000,
	0x41efffffffe00000, 0x41f0000000000000, 0x4330000000000000, 0x4340000000000000, 0x7fefffffffffffff,
	0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0x0000000000000001, 0x0010000000000000,
	0x3cb0000000000000, 0x41d0000000000000, 0xc1d0000000000000, 0x4320000000000001, 0x3fe0000000000001,
	0x41cfffffffffffff, 0xc1dfffffffc00000, 0xc1dfffffffffffff,
}

var floatInputs = [...]int32{
	0, 1, -1, 2, -2, 3, 7, 100, -100, 1 << 20, 0x3fffffff, 0x40000000, -0x40000000, -0x3fffffff,
	0x7ffffffe, 0x7fffffff, -0x7fffffff, -0x80000000, 0x12345678, -0x12345678, 0x1fffffff,
}

func runConvert() {
	if !start("conv") {
		return
	}
	for _, a := range fixInputs {
		if !hangs1("fix", bits(a), 0) {
			r := fixdfsi(bits(a))
			ps("fix ")
			hex64(a)
			ps(" = ")
			pi(int64(r))
			ps("\n")
		}
	}
	for range 100 {
		a := near(1023 + 20)
		r := fixdfsi(bits(a))
		ps("fix ")
		hex64(a)
		ps(" = ")
		pi(int64(r))
		ps("\n")
	}
	for _, i := range floatInputs {
		if !hangs1("float", bits(uint64(uint32(i))), 0) {
			r := floatsidf(i)
			ps("float ")
			pi(int64(i))
			ps(" = ")
			hex64(math.Float64bits(r))
			ps("\n")
		}
	}
	for range 100 {
		i := int32(rnd() >> (rnd() % 32))
		if rnd()&1 == 0 {
			i = -i
		}
		if !hangs1("float", bits(uint64(uint32(i))), 0) {
			r := floatsidf(i)
			ps("float ")
			pi(int64(i))
			ps(" = ")
			hex64(math.Float64bits(r))
			ps("\n")
		}
	}
	for _, a := range primEdges {
		ps("neg ")
		hex64(a)
		ps(" = ")
		hex64(math.Float64bits(negdf2(bits(a))))
		ps(" trunc ")
		hex32(math.Float32bits(truncdfsf2(bits(a))))
		ps("\n")
	}
	for range 100 {
		a := near(1023 + int(rnd()%300) - 150)
		ps("trunc ")
		hex64(a)
		ps(" = ")
		hex32(math.Float32bits(truncdfsf2(bits(a))))
		ps("\n")
	}
}

var sfEdges = [...]uint32{
	0, 0x80000000, 0x3f800000, 0xbf800000, 0x3fc00000, 0x40000000, 0x3dcccccd, 0x40490fdb,
	0x3f800001, 0x3f7fffff, 0x7f000000, 0x7f7fffff, 0x00800000, 0x00000001, 0x007fffff, 0x7f800000,
	0xff800000, 0x7fc00000, 0x7f800001, 0x1f800000, 0x5f800000,
}

func runSingle() {
	if !start("single") {
		return
	}
	for _, a := range sfEdges {
		x := math.Float32frombits(a)
		ps("ext ")
		hex32(a)
		ps(" = ")
		hex64(math.Float64bits(extendsfdf2(x)))
		ps("\n")
		for _, b := range sfEdges {
			y := math.Float32frombits(b)
			ps("sf ")
			hex32(a)
			ps(" ")
			hex32(b)
			ps(" add ")
			hex32(math.Float32bits(addsf3(x, y)))
			ps(" mul ")
			hex32(math.Float32bits(mulsf3(x, y)))
			ps(" div ")
			hex32(math.Float32bits(divsf3(x, y)))
			ps("\n")
		}
	}
	for range 200 {
		a := uint32(near(1023)>>32)&0x807fffff | uint32(127+int(rnd()%40)-20)<<23
		b := uint32(near(1023)>>32)&0x807fffff | uint32(127+int(rnd()%40)-20)<<23
		x, y := math.Float32frombits(a), math.Float32frombits(b)
		ps("sf ")
		hex32(a)
		ps(" ")
		hex32(b)
		ps(" add ")
		hex32(math.Float32bits(addsf3(x, y)))
		ps(" mul ")
		hex32(math.Float32bits(mulsf3(x, y)))
		ps(" div ")
		hex32(math.Float32bits(divsf3(x, y)))
		ps("\n")
	}
}

func main() {
	yaroze.ResetGraph(0)
	ps("math probe errno ")
	pi(int64(getErrno()))
	ps("\n")
	yaroze.TestCard(0)
	ps("after TestCard errno ")
	pi(int64(getErrno()))
	ps("\n")
	// An interrupt landing at some points of Sony's arithmetic sends it
	// into a loop it never leaves (seen in sqrt's random cases, never
	// with interrupts off), so the math runs with them off. flush lets
	// them in to print.
	yaroze.EnterCriticalSection()
	for i := range prims {
		runPrim(&prims[i])
	}
	runCompare()
	runConvert()
	runSingle()
	runAddRound()
	runAddGuard()
	runMulRound()
	runDivRound()
	runFixMore()
	for i := range fns1 {
		run1(&fns1[i])
	}
	for i := range fns2 {
		run2(&fns2[i])
	}
	runSplit()
	runModf()
	runLdexp()
	runLdexpGrid()
	runParse()
	yaroze.ExitCriticalSection()
	runFiles()
	group++
	mark(group)
	ps("math probe done\n")
	for {
		yaroze.VSync(0)
	}
}
