//go:build yaroze

// Command cmatherr2 is a probe for spec 17: where math_errno changes along a sweep.
// ldexp over powers, and frexp, log and log10 over every exponent field, printing only the fields where the value after the call
// changes. Interrupts are off for the calls, as in spec 12's probes.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern math_errno
var mathErrno uint32

//export ldexp
func cldexp(a0, a1, n uint32) uint64

//export frexp
func cfrexp(a0, a1 uint32, e *int32) uint64

//export log
func clog(lo, hi uint32) uint64

//export log10
func clog10(lo, hi uint32) uint64

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
}

func errno() uint32 { return *(*uint32)(unsafe.Pointer(&mathErrno)) }
func clear()        { *(*uint32)(unsafe.Pointer(&mathErrno)) = 0 }

func show(name string, x uint32, e uint32) {
	yaroze.ExitCriticalSection()
	ps(name)
	putc(' ')
	hex32(x)
	ps(" errno ")
	hex32(e)
	putc('\n')
	yaroze.EnterCriticalSection()
}

// sweep calls f over every high word of exponent field 0 to 2047 and prints each
// field where errno differs from the last.
func sweep(name string, m uint32, f func(hi uint32)) {
	last := uint32(0xffffffff)
	for e := uint32(0); e < 2048; e++ {
		clear()
		f(e<<20 | m)
		if errno() != last {
			last = errno()
			show(name, e<<20|m, last)
		}
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.EnterCriticalSection()
	// Powers of ldexp on 1: the positive and negative, printing each transition.
	for _, sign := range [...]int32{1, -1} {
		last := uint32(0xffffffff)
		for n := int32(0); n < 1100; n++ {
			clear()
			cldexp(0, 0x3ff00000, uint32(n*sign))
			if errno() != last {
				last = errno()
				show("ldexp 1 power", uint32(n*sign), last)
			}
		}
	}
	for _, m := range [...]uint32{0, 0x5a5a5} {
		sweep("frexp", m, func(hi uint32) {
			var e int32
			cfrexp(0, hi, &e)
		})
		sweep("log", m, func(hi uint32) { clog(0, hi) })
		sweep("log10", m, func(hi uint32) { clog10(0, hi) })
	}
	yaroze.ExitCriticalSection()
	ps("END\n")
}
