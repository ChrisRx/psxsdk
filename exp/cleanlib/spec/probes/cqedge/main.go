//go:build yaroze

// Command cqedge is a probe for spec 16: qsort with too few elements, each case
// printing before its call, since the last does not come back on Sony's library.
// The constant last is that case: 0 stops the run in a fault from running out of
// stack, and -1 (changed by hand) has not come back after 600 frames. With the
// constant keys12 true, the last case is instead an array whose pivot lands at
// index 0, which stops the run in the same way. Compare by hand: this probe is not
// for diffing against the clean image past its first lines.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export qsort
func qsort(base uint32, n, size int32, cmp uint32)

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
	last   = 0
	keys12 = false
)

var ncmp int

func cmp(a, b uint32) int32 {
	ncmp++
	x := volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(a))))
	y := volatile.LoadUint8((*uint8)(unsafe.Pointer(uintptr(b))))
	return int32(x) - int32(y)
}

func cmpAddr() uint32 {
	f := cmp
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

func main() {
	f := cmpAddr()
	if keys12 {
		wb(64, 1)
		wb(68, 2)
		ps("qsort 1 2:\n")
		qsort(at(64), 2, 4, f)
		ps(" came back\n")
	}
	for _, n := range [...]int32{2, 1, last} {
		for i := range 8 {
			wb(64+i*4, byte(8-i))
		}
		ncmp = 0
		ps("qsort n=")
		pd(n)
		ps(":\n")
		qsort(at(64), n, 4, f)
		ps(" compares=")
		pd(int32(ncmp))
		ps(" out=")
		dump(64, 32)
		putc('\n')
	}
	end()
}
