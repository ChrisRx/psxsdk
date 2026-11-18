//go:build yaroze

// Command krom is a probe for spec 18: Krom2RawAdd and Krom2RawAdd2 over every
// 16-bit code, then codes with bits above the 16th. The table is rolled up: each
// line is a run of codes that stay at one distance from the last, as first code,
// last code, first result and the distance. A kernel with another font could give
// other results, so each result that comes back in the program's reach is as the
// kernel made it; run the probe under PSX_BIOS too.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

//export Krom2RawAdd
func krom1(code uint32) uint32

//export Krom2RawAdd2
func krom2(code uint32) uint32

var (
	line  [128]byte
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

func hex(v uint32, digits int) {
	for i := range digits {
		putc("0123456789abcdef"[v>>(uint(digits-1-i)*4)&15])
	}
}

func run(first, last, val, step uint32) {
	hex(first, 4)
	putc(' ')
	hex(last, 4)
	putc(' ')
	hex(val, 8)
	putc(' ')
	hex(step, 8)
	ps("\n")
}

func table(what string, f func(uint32) uint32) {
	ps("table ")
	ps(what)
	ps("\n")
	first, val := uint32(0), f(0)
	prev := val
	step := uint32(0)
	have := false
	for c := uint32(1); c < 65536; c++ {
		v := f(c)
		switch {
		case !have:
			step, have = v-prev, true
		case v-prev != step:
			run(first, c-1, val, step)
			first, val, have = c, v, false
			step = 0
		}
		prev = v
	}
	run(first, 65535, val, step)
}

func high(what string, f func(uint32) uint32) {
	for _, c := range [...]uint32{0x18140, 0x28140, 0x8140_0000, 0xffff8140, 0x1889f, 0xffff889f, 0xffff0000, 0xffffffff, 0x10000} {
		ps(what)
		putc(' ')
		hex(c, 8)
		putc(' ')
		hex(f(c), 8)
		ps("\n")
	}
}

func main() {
	k1 := func(c uint32) uint32 { return krom1(c) }
	k2 := func(c uint32) uint32 { return krom2(c) }
	table("Krom2RawAdd", k1)
	high("Krom2RawAdd", k1)
	table("Krom2RawAdd2", k2)
	high("Krom2RawAdd2", k2)
	end()
}

// libps_biosB calls function fn of the kernel's B0 table.
//
//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

// end prints the line that stops the run, through the kernel's putchar, since
// write would put a "\r" before the "\n" and the harness looks for "END\n".
func end() {
	for _, c := range [...]byte{'E', 'N', 'D', '\n'} {
		libps_biosB(0x3d, uintptr(c), 0, 0, 0)
	}
	for {
	}
}
