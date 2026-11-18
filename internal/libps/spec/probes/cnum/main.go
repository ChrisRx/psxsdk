//go:build yaroze

// Command cnum is a probe for spec 16: atoi, atol, strtol and strtoul over a
// fixed list of strings and every base of interest, printing each value in hex
// and the end pointer as an offset, with a guard word around it. A value prints
// as VALUE/END, the end pointer being an offset from the string, or - if it was not
// written.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export atoi
func atoi(s uint32) int32

//export atol
func atol(s uint32) int32

//export strtol
func strtol(s, end uint32, base int32) int32

//export strtoul
func strtoul(s, end uint32, base int32) uint32

// The probe prints a line at a time with Write, which is much faster than
// println. Results that point into the arena print as a+N, so the output is the
// same whichever image runs.
var (
	line  [512]byte
	nline int
	arena [1024]uint32
)

const hexDigits = "0123456789abcdef"

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

func pd(v int32) {
	u := uint32(v)
	if v < 0 {
		putc('-')
		u = -u
	}
	var d [10]byte
	k := 0
	for {
		d[k] = byte('0' + u%10)
		k++
		u /= 10
		if u == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hb(b byte) {
	putc(hexDigits[b>>4])
	putc(hexDigits[b&15])
}

func hx(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

func base() uint32 { return uint32(uintptr(unsafe.Pointer(&arena[0]))) }

func at(off int) uint32 { return base() + uint32(off) }

// rb and wb read and write the arena. They are volatile since the compiler sees
// only an integer go to the C call and may otherwise treat the arena as untouched
// by it (a store just before strlen was dropped without this).
func rb(off int) byte {
	return volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(base()) + uintptr(off))))
}

func wb(off int, v byte) {
	volatile.StoreUint8((*uint8)(unsafe.Pointer(uintptr(base())+uintptr(off))), v)
}

// fill sets the first n bytes of the arena to the guard byte.
func fill(n int) {
	for i := range n {
		wb(i, 0xee)
	}
}

// put places s and a terminator in the arena and returns its address.
func put(off int, s string) uint32 {
	for i := 0; i < len(s); i++ {
		wb(off+i, s[i])
	}
	wb(off+len(s), 0)
	return at(off)
}

// nb reads and sb writes memory at address 0, through its cached mirror.
func nb(i int) byte { return volatile.LoadUint8((*uint8)(yaroze.Pointer(0x80000000 + uintptr(i)))) }

func sb(i int, v byte) { volatile.StoreUint8((*uint8)(yaroze.Pointer(0x80000000+uintptr(i))), v) }

// setNull puts a known string at address 0, so that a call that reads through a
// null pointer can be told from one that checks for it.
func setNull(s string) {
	for i := range 16 {
		sb(i, 0xdd)
	}
	for i := 0; i < len(s); i++ {
		sb(i, s[i])
	}
	sb(len(s), 0)
}

// pr prints a result: 0, an offset in the arena, an offset from address 0, or
// the value. Anything inside the library prints as lib.
func pr(v uint32) {
	b := base()
	switch {
	case v == 0:
		putc('0')
	case v >= b && v < b+4096:
		ps("a+")
		pd(int32(v - b))
	case v >= 0x80010000 && v < 0x80090000:
		ps("lib")
	case v < 16:
		ps("null+")
		pd(int32(v))
	default:
		hx(v)
	}
}

// dump prints n bytes of the arena from off.
func dump(off, n int) {
	for i := range n {
		hb(rb(off + i))
	}
}

func dumpNull() {
	for i := 0; i < 8; i++ {
		hb(nb(i))
	}
}

func end() {
	ps("END\n")
	for {
		yaroze.VSync(0)
	}
}

const (
	endp = 16
	srcA = 64
)

func wrw(off int, v uint32) {
	volatile.StoreUint32((*uint32)(unsafe.Pointer(uintptr(base())+uintptr(off))), v)
}

func rdw(off int) uint32 {
	return volatile.LoadUint32((*uint32)(unsafe.Pointer(uintptr(base()) + uintptr(off))))
}

func quote(s string) {
	putc('"')
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			ps("\\x")
			hb(s[i])
		} else {
			putc(s[i])
		}
	}
	putc('"')
}

var strs = [...]string{
	"", " ", "0", "1", "9", "-1", "+1", "-0", "42", "  42", "\t\n\v\f\r 42", "\xa042", "42abc", "abc", "z", "Z", "zz", "Zz",
	"0x1f", "0X1F", "0x", "0xg", "x1", "-0x1f", "+0x1f", " 0x1f", "077", "078", "08", "09", "00", "0b1", "101", "102", "12", "19", "1a", "1g", "7f", "G",
	"2147483647", "2147483648", "4294967295", "4294967296", "99999999999", "-2147483648", "-2147483649", "-4294967295", "-4294967296",
	"7fffffff", "80000000", "ffffffff", "100000000", "1ffffffff", "0xffffffff", "0x100000000", "37777777777", "40000000000",
	"zik0zj", "zik0zk", "1z141z3", "1z141z4", "-", "+", "+-1", "-+1", "--1", "++1", "- 1", "+ 1", "1 2", "1-2", "1+2", "1.5", "1e3",
	"\x80", "\xff", "0\x80", "\x20\x09x",
	"0B1", "0b2", "0b", "0B", "0X", "0b101", "0o7", "0O7", "0d9", "0D9", "0t1", "---1", "-077", "-0b1", "-0x", "0x-1", "0b-1", "0 1",
	"-\t1", "\v1", "00x1", "0xX1", "0x0x1", "0b0b1", "0x1G", "0xfF", "0b12", "0b102", "-08", "-0",
}

var bases = [...]int32{0, 2, 8, 10, 16, 36, -1, 1, 37, 100, 7}

// ubases are strtoul's, which differs from strtol only over a minus sign.
var ubases = [...]int32{0, 8, 16, 36}

// one prints a value and where parsing stopped, compactly, since the harness
// slows as the TTY grows.
func one(ret uint32, off int) {
	putc(' ')
	hx(ret)
	putc('/')
	e := rdw(endp)
	switch {
	case e == 0xdeadbeef:
		putc('-')
	case e >= at(off) && e <= at(off+64):
		pd(int32(e - at(off)))
	default:
		pr(e)
	}
	if rdw(endp-4) != 0xeeeeeeee || rdw(endp+4) != 0xeeeeeeee {
		ps(" GUARD")
	}
}

func main() {
	wrw(endp-4, 0xeeeeeeee)
	wrw(endp+4, 0xeeeeeeee)
	for _, s := range strs {
		put(srcA, s)
		ps("ato ")
		quote(s)
		ps(" atoi=")
		hx(uint32(atoi(at(srcA))))
		ps(" atol=")
		hx(uint32(atol(at(srcA))))
		putc('\n')
	}
	ps("bases")
	for _, b := range bases {
		putc(' ')
		pd(b)
	}
	putc('\n')
	for _, s := range strs {
		put(srcA, s)
		ps("l ")
		quote(s)
		for _, b := range bases {
			wrw(endp, 0xdeadbeef)
			one(uint32(strtol(at(srcA), at(endp), b)), srcA)
		}
		putc('\n')
		ps("u ")
		quote(s)
		for _, b := range ubases {
			wrw(endp, 0xdeadbeef)
			one(strtoul(at(srcA), at(endp), b), srcA)
		}
		putc('\n')
	}
	// A null end pointer; an unaligned one; a null string, with a known string
	// at address 0.
	put(srcA, "123x")
	ps("strtol end=null ")
	hx(uint32(strtol(at(srcA), 0, 10)))
	putc('\n')
	ps("strtoul end=null ")
	hx(strtoul(at(srcA), 0, 10))
	putc('\n')
	setNull("77x")
	wrw(endp, 0xdeadbeef)
	ps("strtol null")
	one(uint32(strtol(0, at(endp), 10)), srcA)
	putc('\n')
	wrw(endp, 0xdeadbeef)
	ps("strtoul null")
	one(strtoul(0, at(endp), 10), srcA)
	putc('\n')
	ps("atoi null ")
	hx(uint32(atoi(0)))
	ps(" atol null ")
	hx(uint32(atol(0)))
	putc('\n')
	setNull("")
	wrw(endp, 0xdeadbeef)
	ps("strtol null empty")
	one(uint32(strtol(0, at(endp), 10)), srcA)
	putc('\n')
	end()
}
