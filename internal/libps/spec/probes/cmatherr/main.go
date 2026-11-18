//go:build yaroze

// Command cmatherr is a probe for spec 17: math_errno after each math call of spec
// 12 and each floating point helper, over the edges. math_errno is set to 0 before
// each call and printed after it, with the result's bits. Calls spec 12 lists as
// never returning are skipped. With interrupts off, as spec 12's probes run. The
// harness's PROBE_WATCH on _err_math shows each call of it, and PROBE_KERNEL the
// events.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern math_errno
var mathErrno uint32

//export sqrt
func csqrt(lo, hi uint32) uint64

//export log
func clog(lo, hi uint32) uint64

//export log10
func clog10(lo, hi uint32) uint64

//export exp
func cexp(lo, hi uint32) uint64

//export sin
func csin(lo, hi uint32) uint64

//export cos
func ccos(lo, hi uint32) uint64

//export tan
func ctan(lo, hi uint32) uint64

//export asin
func casin(lo, hi uint32) uint64

//export acos
func cacos(lo, hi uint32) uint64

//export atan
func catan(lo, hi uint32) uint64

//export sinh
func csinh(lo, hi uint32) uint64

//export cosh
func ccosh(lo, hi uint32) uint64

//export tanh
func ctanh(lo, hi uint32) uint64

//export floor
func cfloor(lo, hi uint32) uint64

//export ceil
func cceil(lo, hi uint32) uint64

//export pow
func cpow(a0, a1, a2, a3 uint32) uint64

//export atan2
func catan2(a0, a1, a2, a3 uint32) uint64

//export fmod
func cfmod(a0, a1, a2, a3 uint32) uint64

//export hypot
func chypot(a0, a1, a2, a3 uint32) uint64

//export ldexp
func cldexp(a0, a1, n uint32) uint64

//export frexp
func cfrexp(a0, a1 uint32, e *int32) uint64

//export modf
func cmodf(a0, a1 uint32, ip *uint64) uint64

//export __adddf3
func addD(a0, a1, a2, a3 uint32) uint64

//export __subdf3
func subD(a0, a1, a2, a3 uint32) uint64

//export __muldf3
func mulD(a0, a1, a2, a3 uint32) uint64

//export __divdf3
func divD(a0, a1, a2, a3 uint32) uint64

//export __negdf2
func negD(lo, hi uint32) uint64

//export __ltdf2
func ltD(a0, a1, a2, a3 uint32) int32

//export __fixdfsi
func fixD(lo, hi uint32) int32

//export __floatsidf
func floatD(a uint32) uint64

//export __truncdfsf2
func truncD(lo, hi uint32) uint32

//export __extendsfdf2
func extendS(a uint32) uint64

//export __addsf3
func addS(a, b uint32) uint32

//export __mulsf3
func mulS(a, b uint32) uint32

//export __divsf3
func divS(a, b uint32) uint32

//export atof
func catof(s *byte) uint64

//export strtod
func cstrtod(s *byte, end *uint32) uint64

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

func errno() uint32      { return *(*uint32)(unsafe.Pointer(&mathErrno)) }
func clear()             { *(*uint32)(unsafe.Pointer(&mathErrno)) = 0 }
func lo(v uint64) uint32 { return uint32(v) }
func hi(v uint64) uint32 { return uint32(v >> 32) }

// Lines wait in memory, so that the prints' interrupts stay out of the math.
type entry struct {
	name string
	a, b uint64
	r    uint64
	e    uint32
}

var (
	log [256]entry
	n   int
)

func put(name string, a, b, r uint64) {
	log[n] = entry{name, a, b, r, errno()}
	n++
	if n == len(log) {
		drain()
	}
}

func drain() {
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &log[i]
		ps(l.name)
		putc(' ')
		hex64(l.a)
		putc(' ')
		hex64(l.b)
		ps(" = ")
		hex64(l.r)
		ps(" errno ")
		hex32(l.e)
		putc('\n')
	}
	n = 0
	yaroze.EnterCriticalSection()
}

const big = 0x41c0000000000000 // 2^29: past this the calls below can hang

var edges = [...]uint64{
	0, 1 << 63, 0x3fe0000000000000, 0xbfe0000000000000, 0x3ff0000000000000, 0xbff0000000000000,
	0x4000000000000000, 0xc000000000000000, 0x400921fb54442d18, 0x3ff921fb54442d18, 0xbff921fb54442d18,
	0x4024000000000000, 0xc024000000000000, 0x4059000000000000,
	0x0000000000000001, 0x8000000000000001, 0x000fffffffffffff, 0x0010000000000000, 0x8010000000000000,
	0x7fefffffffffffff, 0xffefffffffffffff, 0x7ff0000000000000, 0xfff0000000000000,
	0x7ff8000000000000, 0xfff8000000000000, 0x7ff0000000000001,
	0x3ff0000000000001, 0xbff0000000000001, 0x3fefffffffffffff, 0xbfefffffffffffff,
	0x412e848000000000, 0xc12e848000000000, 0x4341c37937e08000, 0x7e37e43c8800759c, 0xfe37e43c8800759c,
	0x01a56e1fc2f8f359, 0x81a56e1fc2f8f359, 0x3e45798ee2308c3a,
	0x4085e00000000000, 0x4086280000000000, 0x40862e4000000000, 0x4086340000000000, 0x4090000000000000,
	0xc085e00000000000, 0xc086300000000000, 0xc0874a0000000000, 0xc0874c0000000000, 0xc090000000000000,
}

type fn1 struct {
	name string
	f    func(lo, hi uint32) uint64
	// skip says the call hangs.
	skip func(x uint64) bool
}

func isNan(x uint64) bool { return x&^(1<<63) > 0x7ff0000000000000 }
func isBig(x uint64) bool { return x&^(1<<63) >= big }
func neg(x uint64) bool   { return x>>63 != 0 }

var fns1 = [...]fn1{
	{"sqrt", func(lo, hi uint32) uint64 { return csqrt(lo, hi) }, nil},
	{"log", func(lo, hi uint32) uint64 { return clog(lo, hi) }, nil},
	{"log10", func(lo, hi uint32) uint64 { return clog10(lo, hi) }, nil},
	{"exp", func(lo, hi uint32) uint64 { return cexp(lo, hi) }, func(x uint64) bool { return neg(x) && (isBig(x) || isNan(x)) }},
	{"sin", func(lo, hi uint32) uint64 { return csin(lo, hi) }, func(x uint64) bool { return neg(x) && isBig(x) }},
	{"cos", func(lo, hi uint32) uint64 { return ccos(lo, hi) }, func(x uint64) bool { return isBig(x) || isNan(x) }},
	{"tan", func(lo, hi uint32) uint64 { return ctan(lo, hi) }, func(x uint64) bool { return neg(x) && isBig(x) }},
	{"asin", func(lo, hi uint32) uint64 { return casin(lo, hi) }, nil},
	{"acos", func(lo, hi uint32) uint64 { return cacos(lo, hi) }, nil},
	{"atan", func(lo, hi uint32) uint64 { return catan(lo, hi) }, nil},
	{"sinh", func(lo, hi uint32) uint64 { return csinh(lo, hi) }, func(x uint64) bool { return neg(x) && (isBig(x) || isNan(x)) }},
	{"cosh", func(lo, hi uint32) uint64 { return ccosh(lo, hi) }, func(x uint64) bool { return neg(x) && (isBig(x) || isNan(x)) }},
	{"tanh", func(lo, hi uint32) uint64 { return ctanh(lo, hi) }, func(x uint64) bool { return isBig(x) || isNan(x) }},
	{"floor", func(lo, hi uint32) uint64 { return cfloor(lo, hi) }, nil},
	{"ceil", func(lo, hi uint32) uint64 { return cceil(lo, hi) }, nil},
	{"neg", func(lo, hi uint32) uint64 { return negD(lo, hi) }, nil},
}

var pair = [...]uint64{
	0, 1 << 63, 0x3ff0000000000000, 0xbff0000000000000, 0x4000000000000000, 0xc000000000000000,
	0x3fe0000000000000, 0x4008000000000000, 0x3fd5555555555555, 0x4024000000000000, 0xc020000000000000,
	0x7fefffffffffffff, 0xffefffffffffffff, 0x0010000000000000, 0x7e37e43c8800759c, 0x01a56e1fc2f8f359,
	0x4090000000000000, 0xc090000000000000,
}

var pairInf = [...]uint64{0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0x0000000000000001}

type fn2 struct {
	name string
	f    func(a0, a1, a2, a3 uint32) uint64
}

var fns2 = [...]fn2{
	{"add", func(a, b, c, d uint32) uint64 { return addD(a, b, c, d) }},
	{"sub", func(a, b, c, d uint32) uint64 { return subD(a, b, c, d) }},
	{"mul", func(a, b, c, d uint32) uint64 { return mulD(a, b, c, d) }},
	{"div", func(a, b, c, d uint32) uint64 { return divD(a, b, c, d) }},
	{"atan2", func(a, b, c, d uint32) uint64 { return catan2(a, b, c, d) }},
	{"fmod", func(a, b, c, d uint32) uint64 { return cfmod(a, b, c, d) }},
	{"hypot", func(a, b, c, d uint32) uint64 { return chypot(a, b, c, d) }},
}

var (
	cbuf [64]byte
	fbs  = [...]uint32{
		0, 0x80000000, 0x3f800000, 0xbf800000, 0x7f7fffff, 0xff7fffff, 0x00800000, 0x00000001, 0x7f800000, 0xff800000, 0x7fc00000, 0x40000000, 0x3f000000,
	}
	strs = [...]string{
		"1", "1e10", "1e255", "1e256", "1e-255", "1e-256", "1e308", "1e309", "1e400", "1e999", "1e-308", "1e-320", "1e-400", "1e-999",
		"0e999", "0e-999", "0", "", "abc", "1e4294967297", "1e2147483648", "99999999999999999999", "1.7976931348623157e308", "1.8e308", "4.9e-324",
	}
)

func main() {
	yaroze.ResetGraph(0)
	yaroze.EnterCriticalSection()
	for i := range fns1 {
		f := &fns1[i]
		for _, x := range edges {
			if f.skip != nil && f.skip(x) {
				continue
			}
			clear()
			r := f.f(lo(x), hi(x))
			put(f.name, x, 0, r)
		}
	}
	drain()
	for i := range fns2 {
		f := &fns2[i]
		for _, x := range pair {
			for _, y := range pair {
				clear()
				r := f.f(lo(x), hi(x), lo(y), hi(y))
				put(f.name, x, y, r)
			}
			for _, y := range pairInf {
				clear()
				r := f.f(lo(x), hi(x), lo(y), hi(y))
				put(f.name, x, y, r)
				clear()
				r = f.f(lo(y), hi(y), lo(x), hi(x))
				put(f.name, y, x, r)
			}
		}
	}
	drain()
	// pow, over powers that return.
	for _, x := range pair {
		for _, y := range [...]uint64{
			0, 0x3ff0000000000000, 0xbff0000000000000, 0x4000000000000000, 0xc000000000000000, 0x3fe0000000000000, 0xbfe0000000000000,
			0x4008000000000000, 0x3fd5555555555555, 0x408f400000000000, 0xc08f400000000000,
		} {
			clear()
			r := cpow(lo(x), hi(x), lo(y), hi(y))
			put("pow", x, y, r)
		}
	}
	drain()
	// ldexp, frexp, modf.
	for _, x := range pair {
		for _, m := range [...]uint32{0, 1, 0xffffffff, 2000, 0xfffff830, 5000, 0xffffec78} {
			clear()
			r := cldexp(lo(x), hi(x), m)
			put("ldexp", x, uint64(m), r)
		}
		var e int32
		clear()
		r := cfrexp(lo(x), hi(x), &e)
		put("frexp", x, 0, r)
		var ip uint64
		clear()
		r = cmodf(lo(x), hi(x), &ip)
		put("modf", x, ip, r)
	}
	for _, x := range pairInf {
		var e int32
		clear()
		r := cfrexp(lo(x), hi(x), &e)
		put("frexp", x, 0, r)
		var ip uint64
		clear()
		r = cmodf(lo(x), hi(x), &ip)
		put("modf", x, ip, r)
	}
	drain()
	// Conversions and single precision.
	for _, x := range edges {
		clear()
		put("fix", x, 0, uint64(uint32(fixD(lo(x), hi(x)))))
		clear()
		put("trunc", x, 0, uint64(truncD(lo(x), hi(x))))
	}
	for _, x := range [...]uint64{
		0x41dfffffffc00000, 0x41e0000000000000, 0xc1e0000000000000, 0xc1e0000000200000, 0x41dfffffffe00000, 0xc1dfffffffe00000,
		0x47efffffe0000000, 0x47efffffe8000000, 0x47efffffefffffff, 0x47f0000000000000, 0x36a0000000000000, 0x3690000000000000, 0x3810000000000000,
	} {
		clear()
		put("fix", x, 0, uint64(uint32(fixD(lo(x), hi(x)))))
		clear()
		put("trunc", x, 0, uint64(truncD(lo(x), hi(x))))
	}
	for _, a := range [...]uint32{0, 1, 0xffffffff, 0x7fffffff} {
		clear()
		put("float", uint64(a), 0, floatD(a))
	}
	drain()
	for _, a := range fbs {
		clear()
		put("extend", uint64(a), 0, extendS(a))
		for _, b := range fbs {
			clear()
			put("adds", uint64(a), uint64(b), uint64(addS(a, b)))
			clear()
			put("muls", uint64(a), uint64(b), uint64(mulS(a, b)))
			clear()
			put("divs", uint64(a), uint64(b), uint64(divS(a, b)))
		}
	}
	drain()
	// Comparisons, on a sample.
	for _, x := range pairInf {
		clear()
		put("lt", x, 0x3ff0000000000000, uint64(uint32(ltD(lo(x), hi(x), 0, 0x3ff00000))))
	}
	// Parsing.
	for i := range strs {
		k := copy(cbuf[:len(cbuf)-1], strs[i])
		cbuf[k] = 0
		clear()
		put("atof", uint64(i), 0, catof(&cbuf[0]))
		var end uint32
		clear()
		put("strtod", uint64(i), 0, cstrtod(&cbuf[0], &end))
	}
	drain()
	// Every exponent field through log and log10: only the changes print.
	for _, m := range [...]uint64{0, 0x5a5a5a5a5a5a5} {
		last := uint32(0xffffffff)
		for e := uint64(0); e < 2048; e++ {
			x := e<<52 | m
			clear()
			r := clog(lo(x), hi(x))
			if errno() != last {
				last = errno()
				put("logsweep", x, 0, r)
			}
		}
		last = 0xffffffff
		for e := uint64(0); e < 2048; e++ {
			x := e<<52 | m
			clear()
			r := clog10(lo(x), hi(x))
			if errno() != last {
				last = errno()
				put("log10sweep", x, 0, r)
			}
		}
	}
	drain()
	yaroze.ExitCriticalSection()
	ps("END\n")
}
