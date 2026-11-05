//go:build yaroze

// Command cneg is a probe for spec 16: negative and otherwise odd lengths, on
// calls that would run through all of memory if they read a negative length as
// unsigned. The order puts the reads first and the writes after. Each line prints
// before its call and after it, so a run that stops shows which call stopped it.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export memcpy
func memcpy(d, s uint32, n int32) uint32

//export memmove
func memmove(d, s uint32, n int32) uint32

//export memset
func memset(d uint32, c, n int32) uint32

//export memcmp
func memcmp(a, b uint32, n int32) int32

//export memchr
func memchr(s uint32, c, n int32) uint32

//export bcopy
func bcopy(s, d uint32, n int32)

//export bzero
func bzero(d uint32, n int32)

//export bcmp
func bcmp(a, b uint32, n int32) int32

//export strncpy
func strncpy(d, s uint32, n int32) uint32

//export qsort
func qsort(base uint32, n, size int32, cmp uint32)

//export bsearch
func bsearch(key, base uint32, n, size int32, cmp uint32) uint32

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
	dst  = 8
	srcA = 128
	srcB = 512
	arr  = 1024
)

var ncmp int

func cmp(a, b uint32) int32 {
	ncmp++
	// Through the cached mirror, since Go stops at a nil pointer.
	x := volatile.LoadUint8((*uint8)(yaroze.Pointer(uintptr(a | 0x80000000))))
	y := volatile.LoadUint8((*uint8)(yaroze.Pointer(uintptr(b | 0x80000000))))
	return int32(x) - int32(y)
}

func cmpAddr() uint32 {
	f := cmp
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

func reset() {
	fill(srcA)
	setNull("NULLY")
	put(srcA, "0123456789abcdef")
	put(srcB, "0123456789abcdef")
}

func try(name string, n int32) {
	ps(name)
	ps(" n=")
	pd(n)
	ps(":\n")
}

func wr(ret uint32) {
	ps(" ret=")
	pr(ret)
	ps(" mem=")
	dump(0, 40)
	ps(" null=")
	dumpNull()
	putc('\n')
}

var negs = [...]int32{-1, -2147483648}

func main() {
	reset()
	// The reads: a difference at the sixth byte stops one that goes on.
	wb(srcB+5, 'z')
	for _, n := range negs {
		try("memcmp diff at 5", n)
		ps(" ret=")
		pd(memcmp(at(srcA), at(srcB), n))
		putc('\n')
		try("bcmp diff at 5", n)
		ps(" ret=")
		pd(bcmp(at(srcA), at(srcB), n))
		putc('\n')
		try("memcmp diff at 0", n)
		wb(srcB, 'z')
		ps(" ret=")
		pd(memcmp(at(srcA), at(srcB), n))
		wb(srcB, '0')
		putc('\n')
		try("memchr hit at 3", n)
		ps(" ret=")
		pr(memchr(at(srcA), '3', n))
		putc('\n')
	}
	// The writes.
	for _, n := range negs {
		reset()
		try("strncpy", n)
		wr(strncpy(at(dst), at(srcA), n))
		reset()
		try("memset", n)
		wr(memset(at(dst), 0x41, n))
		reset()
		try("bzero", n)
		bzero(at(dst), n)
		wr(0)
		reset()
		try("memcpy", n)
		wr(memcpy(at(dst), at(srcA), n))
		reset()
		try("memmove d<s", n)
		wr(memmove(at(dst), at(srcA), n))
		reset()
		try("memmove d>s", n)
		put(dst, "0123456789")
		wr(memmove(at(dst+2), at(dst), n))
		reset()
		try("bcopy", n)
		bcopy(at(srcA), at(dst), n)
		wr(0)
	}
	// The sort and search calls, last as they may not come back.
	f := cmpAddr()
	for i := range 8 {
		wb(arr+i*4, byte(8-i))
	}
	ncmp = 0
	try("qsort base=null", 1)
	qsort(0, 1, 4, f)
	ps(" compares=")
	pd(int32(ncmp))
	putc('\n')
	ncmp = 0
	setNull("NULLY")
	put(srcA, "N")
	try("bsearch base=null key=N", 1)
	ps(" ret=")
	pr(bsearch(at(srcA), 0, 1, 4, f))
	ps(" compares=")
	pd(int32(ncmp))
	putc('\n')
	ncmp = 0
	put(srcA, "Q")
	try("bsearch base=null key=Q", 1)
	ps(" ret=")
	pr(bsearch(at(srcA), 0, 1, 4, f))
	ps(" compares=")
	pd(int32(ncmp))
	putc('\n')
	for _, n := range negs {
		ncmp = 0
		try("bsearch", n)
		ps(" ret=")
		pr(bsearch(at(arr), at(arr), n, 4, f))
		ps(" compares=")
		pd(int32(ncmp))
		putc('\n')
	}
	ncmp = 0
	try("bsearch size=0", 8)
	ps(" ret=")
	pr(bsearch(at(srcA), at(arr), 8, 0, f))
	ps(" compares=")
	pd(int32(ncmp))
	putc('\n')
	ncmp = 0
	try("qsort size=0", 8)
	qsort(at(arr), 8, 0, f)
	ps(" compares=")
	pd(int32(ncmp))
	ps(" out=")
	dump(arr, 32)
	putc('\n')
	end()
}
