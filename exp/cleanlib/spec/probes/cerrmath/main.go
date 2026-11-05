//go:build yaroze

// Command cerrmath is a probe for spec 17: _err_math and _dbl_shift called with
// arguments in a0 to a3, what comes back in v0 and v1, and math_errno around them.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export _err_math
func errMath(a0, a1, a2, a3 uint32) uint64

//export _dbl_shift
func dblShift5(a0, a1, a2, a3, a4, a5 uint32) uint64

//export sqrt
func csqrt(lo, hi uint32) uint64

//export log
func clog(lo, hi uint32) uint64

//export log10
func clog10(lo, hi uint32) uint64

//export exp
func cexp(lo, hi uint32) uint64

//export asin
func casin(lo, hi uint32) uint64

//export acos
func cacos(lo, hi uint32) uint64

//export sinh
func csinh(lo, hi uint32) uint64

//export pow
func cpow(a0, a1, a2, a3 uint32) uint64

//export atan2
func catan2(a0, a1, a2, a3 uint32) uint64

//export fmod
func cfmod(a0, a1, a2, a3 uint32) uint64

//export ldexp
func cldexp(a0, a1, n uint32) uint64

//export __divdf3
func divdf3(a0, a1, a2, a3 uint32) uint64

//export __muldf3
func muldf3(a0, a1, a2, a3 uint32) uint64

//export __adddf3
func adddf3(a0, a1, a2, a3 uint32) uint64

//export __fixdfsi
func fixdfsi(lo, hi uint32) int32

//export atof
func catof(s *byte) uint64

//go:extern math_errno
var mathErrno uint32

var (
	sink  uint64
	isink int32
)

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

func hex32(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
	putc(' ')
}

func hex64(v uint64) {
	hex32(uint32(v))
	hex32(uint32(v >> 32))
}

func volatileErrno() uint32 {
	return *(*uint32)(unsafe.Pointer(&mathErrno))
}

func em(a0, a1, a2, a3 uint32) {
	ps("_err_math ")
	hex32(a0)
	hex32(a1)
	hex32(a2)
	hex32(a3)
	r := errMath(a0, a1, a2, a3)
	ps("-> ")
	hex64(r)
	ps("errno ")
	hex32(volatileErrno())
	putc('\n')
}

var buf [8]uint32

// dp calls _dbl_shift with buf's index 2 as a0 and prints what comes back.
func dp(a1, a2, a3, a4, a5 uint32) {
	for i := range buf {
		buf[i] = 0x11111111 * uint32(i+1)
	}
	ps("_dbl_shift a1 ")
	hex32(a1)
	ps("a2 ")
	hex32(a2)
	ps("a3 ")
	hex32(a3)
	ps("s0 ")
	hex32(a4)
	ps("s1 ")
	hex32(a5)
	r := dblShift5(uint32(uintptr(unsafe.Pointer(&buf[2]))), a1, a2, a3, a4, a5)
	ps("-> ")
	hex32(uint32(r) - uint32(uintptr(unsafe.Pointer(&buf[2]))))
	hex32(uint32(r >> 32))
	ps("buf ")
	for _, v := range buf {
		hex32(v)
	}
	putc('\n')
}

// The doubles' high words; every low word but xmax's and x3rd's is 0.
const (
	x1   = 0x3ff00000
	xm1  = 0xbff00000
	x2   = 0x40000000
	x1k  = 0x408f4000
	xmx  = 0x7fefffff
	xmn  = 0x00100000
	x8m  = 0xc0200000
	x3rd = 0x3fd55555
)

var big = [...]byte{'1', 'e', '9', '9', '9', 0}

func after(name string) {
	ps(name)
	ps(" errno ")
	hex32(volatileErrno())
	putc('\n')
	*(*uint32)(unsafe.Pointer(&mathErrno)) = 0
}

func main() {
	ps("errno at start ")
	hex32(volatileErrno())
	putc('\n')
	for _, v := range [...]uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 16, 32, 33, 34, 35, 36, 0x121, 0x10021, 0xffffffff} {
		em(v, 0x11111111, 0x22222222, 0x33333333)
		em(v, 0, 0, 0)
	}
	*(*uint32)(unsafe.Pointer(&mathErrno)) = 0
	sink = csqrt(0, xm1)
	after("sqrt(-1)")
	sink = clog(0, xm1)
	after("log(-1)")
	sink = clog(0, 0)
	after("log(0)")
	sink = clog10(0, xm1)
	after("log10(-1)")
	sink = cexp(0, x1k)
	after("exp(1000)")
	sink = casin(0, x2)
	after("asin(2)")
	sink = cacos(0, x2)
	after("acos(2)")
	sink = csinh(0, x1k)
	after("sinh(1000)")
	sink = cpow(0, 0, 0, xm1)
	after("pow(0,-1)")
	sink = cpow(0, x8m, 0x55555555, x3rd)
	after("pow(-8,1/3)")
	sink = catan2(0, 0, 0, 0)
	after("atan2(0,0)")
	sink = cfmod(0, x1, 0, 0)
	after("fmod(1,0)")
	sink = cldexp(0, x1, 5000)
	after("ldexp(1,5000)")
	sink = catof(&big[0])
	after("atof(1e999)")
	sink = divdf3(0, x1, 0, 0)
	after("1/0")
	sink = muldf3(0xffffffff, xmx, 0xffffffff, xmx)
	after("max*max")
	sink = muldf3(0, xmn, 0, xmn)
	after("min*min")
	sink = adddf3(0xffffffff, xmx, 0xffffffff, xmx)
	after("max+max")
	isink = fixdfsi(0xffffffff, xmx)
	after("(int)max")
	// Word by word: all four bytes of the word are its value.
	*(*uint32)(unsafe.Pointer(&mathErrno)) = 0x12345678
	after("written back")
	const x, y, z, w, v = 0x89abcdef, 0x01234567, 0x76543210, 0xfedcba98, 0x13579bdf
	dp(0, 0, 0, 0, 0)
	dp(x, 0, 0, 0, 0)
	dp(0, y, 0, 0, 0)
	dp(0, 0, z, 0, 0)
	dp(0, 0, 0, w, 0)
	dp(0, 0, 0, 0, v)
	dp(x, y, z, w, v)
	for _, n := range [...]uint32{0, 1, 2, 3, 4, 8, 16, 31, 32, 33, 52, 63, 64, 65, 66, 100, 1000} {
		dp(x, y, z, n, v)
		// A top bit: is the shift logical?
		dp(x, y, 0xf6543210, n, v)
	}
	for _, n := range [...]uint32{0x80000000, 0xffffffff, 0xffffffc0, 0xfffffffe} {
		dp(x, y, 0xf6543210, n, v)
	}
	dp(x, 0xffffffff, 0xffffffff, 1, v)
	dp(x, 0xffffffff, 0xffffffff, 63, v)
	dp(x, 0xffffffff, 0xffffffff, 64, v)
	ps("END\n")
}
