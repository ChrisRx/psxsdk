//go:build yaroze

// Command cmem is a probe for spec 16: memcpy, memset, memcmp, memmove, memchr,
// bcopy, bzero and bcmp, on destinations between guard bytes (EE), with a known
// string at address 0. Each overlap shows the direction of the copy, and what is
// copied at a time. Negative lengths that may run away are in probe cneg.
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
)

func a(s string) uint32 { return put(srcA, s) }
func b(s string) uint32 { return put(srcB, s) }

func wr(name string, ret uint32) {
	ps(name)
	ps(" ret=")
	pr(ret)
	ps(" mem=")
	dump(0, 48)
	ps(" null=")
	dumpNull()
	putc('\n')
}

func wrv(name string) { wr(name, 0) }

func in(name string, v int32) {
	ps(name)
	ps(" = ")
	pd(v)
	putc('\n')
}

func reset() {
	fill(srcA)
	setNull("NULLY")
}

const text = "0123456789abcdefghijklmnopqrstuvwxyz"

// copies runs one copying call over the lengths, offsets and overlaps; call takes
// destination, source and length and returns what the call returned.
func copies(name string, call func(d, s uint32, n int32) uint32) {
	for _, n := range [...]int32{0, 1, 2, 3, 4, 5, 7, 8, 9, 16, 17} {
		reset()
		ps(name)
		ps(" n=")
		pd(n)
		wr("", call(at(dst), a(text), n))
	}
	for _, do := range [...]int{0, 1, 2, 3} {
		for _, so := range [...]int{0, 1, 2, 3} {
			reset()
			a(text)
			ps(name)
			ps(" d+")
			pd(int32(do))
			ps(" s+")
			pd(int32(so))
			ps(" n=13")
			wr("", call(at(dst+do), at(srcA+so), 13))
		}
	}
	// Overlaps: destination after the source, then before.
	for _, sh := range [...]int{1, 2, 3, 4, 5, 8, 12} {
		reset()
		put(dst, text[:24])
		ps(name)
		ps(" d=s+")
		pd(int32(sh))
		ps(" n=16")
		wr("", call(at(dst+sh), at(dst), 16))
		reset()
		put(dst+sh, text[:24])
		ps(name)
		ps(" d=s-")
		pd(int32(sh))
		ps(" n=16")
		wr("", call(at(dst), at(dst+sh), 16))
	}
	reset()
	put(dst, text[:24])
	ps(name)
	wr(" d=s n=16", call(at(dst), at(dst), 16))
	// Null pointers.
	reset()
	ps(name)
	wr(" d=null n=3", call(0, a(text), 3))
	reset()
	ps(name)
	wr(" s=null n=3", call(at(dst), 0, 3))
	reset()
	ps(name)
	wr(" both null n=3", call(0, 0, 3))
	reset()
	ps(name)
	wr(" d=null n=0", call(0, a(text), 0))
	reset()
	ps(name)
	wr(" s=null n=0", call(at(dst), 0, 0))
}

func runSet() {
	for _, n := range [...]int32{0, 1, 2, 3, 4, 5, 7, 8, 9, 16, 17} {
		reset()
		ps("memset n=")
		pd(n)
		wr("", memset(at(dst), 0x41, n))
		reset()
		ps("bzero n=")
		pd(n)
		bzero(at(dst), n)
		wrv("")
	}
	for _, do := range [...]int{1, 2, 3} {
		reset()
		ps("memset d+")
		pd(int32(do))
		wr(" n=13", memset(at(dst+do), 0x41, 13))
		reset()
		ps("bzero d+")
		pd(int32(do))
		bzero(at(dst+do), 13)
		wrv(" n=13")
	}
	for _, c := range [...]int32{0, 1, 0x7f, 0x80, 0xff, 0x100, 0x141, 0x1ff, -1, -128, 0x12345678} {
		reset()
		ps("memset c=")
		pd(c)
		wr(" n=6", memset(at(dst), c, 6))
	}
	reset()
	wr("memset null n=3", memset(0, 0x41, 3))
	reset()
	wr("memset null n=0", memset(0, 0x41, 0))
	reset()
	bzero(0, 3)
	wrv("bzero null n=3")
}

var cmps = [...][2]string{
	{"abc", "abc"}, {"abc", "abd"}, {"abd", "abc"}, {"a", "c"}, {"c", "a"}, {"a", "z"}, {"z", "a"},
	{"\x80", "a"}, {"a", "\x80"}, {"\xff", "\x01"}, {"\x01", "\xff"}, {"\xff", "\x7f"}, {"\x80", "\xff"},
	{"\x00", "\xff"}, {"\xff", "\x00"},
}

func runCmp() {
	for _, n := range [...]int32{0, 1, 2, 3, 4} {
		for _, p := range cmps {
			ps("cmp n=")
			pd(n)
			putc(' ')
			hb(p[0][0])
			putc(' ')
			hb(p[1][0])
			ps(" len=")
			pd(int32(len(p[0])))
			x := put(srcA, p[0])
			y := put(srcB, p[1])
			ps(" memcmp=")
			pd(memcmp(x, y, n))
			ps(" bcmp=")
			pd(bcmp(x, y, n))
			putc('\n')
		}
	}
	// NUL bytes do not stop a compare. Word boundaries with a difference
	// after them.
	put(srcA, "a\x00b")
	put(srcB, "a\x00c")
	in("memcmp past NUL n=3", memcmp(at(srcA), at(srcB), 3))
	in("bcmp past NUL n=3", bcmp(at(srcA), at(srcB), 3))
	for off := range 16 {
		for i := range 20 {
			wb(srcA+i, 'x')
			wb(srcB+i, 'x')
		}
		wb(srcB+off, 'z')
		ps("diff at ")
		pd(int32(off))
		ps(" memcmp=")
		pd(memcmp(at(srcA), at(srcB), 20))
		ps(" bcmp=")
		pd(bcmp(at(srcA), at(srcB), 20))
		ps(" memcmp+1=")
		pd(memcmp(at(srcA+1), at(srcB+1), 19))
		ps(" memcmp+1+2=")
		pd(memcmp(at(srcA+1), at(srcB+2), 18))
		putc('\n')
	}
	// How far a compare of n bytes reads: a difference at each place, for
	// each small n.
	for n := int32(0); n < 6; n++ {
		ps("reach n=")
		pd(n)
		for off := range 6 {
			for i := range 8 {
				wb(srcA+i, 'x')
				wb(srcB+i, 'x')
			}
			wb(srcB+off, 'z')
			ps(" ")
			pd(memcmp(at(srcA), at(srcB), n))
			putc('/')
			pd(bcmp(at(srcA), at(srcB), n))
		}
		putc('\n')
	}
	setNull("NULLY")
	a("ZZZZZ")
	in("memcmp null ZZZZZ n=0", memcmp(0, at(srcA), 0))
	in("memcmp ZZZZZ null n=0", memcmp(at(srcA), 0, 0))
	in("bcmp null ZZZZZ n=0", bcmp(0, at(srcA), 0))
	in("bcmp ZZZZZ null n=0", bcmp(at(srcA), 0, 0))
	a("NULLY")
	in("memcmp null null n=3", memcmp(0, 0, 3))
	in("memcmp null s n=3", memcmp(0, at(srcA), 3))
	in("memcmp s null n=3", memcmp(at(srcA), 0, 3))
	in("memcmp null s n=0", memcmp(0, at(srcA), 0))
	in("memcmp s null n=0", memcmp(at(srcA), 0, 0))
	in("bcmp null null n=3", bcmp(0, 0, 3))
	in("bcmp null s n=3", bcmp(0, at(srcA), 3))
	in("bcmp s null n=3", bcmp(at(srcA), 0, 3))
	in("bcmp null s n=0", bcmp(0, at(srcA), 0))
	in("bcmp s null n=0", bcmp(at(srcA), 0, 0))
	a("ZZZZZ")
	in("memcmp null ZZZZZ n=3", memcmp(0, at(srcA), 3))
	in("memcmp ZZZZZ null n=3", memcmp(at(srcA), 0, 3))
	in("bcmp null ZZZZZ n=3", bcmp(0, at(srcA), 3))
	in("bcmp ZZZZZ null n=3", bcmp(at(srcA), 0, 3))
}

func ptr(name string, v uint32) {
	ps(name)
	ps(" = ")
	pr(v)
	putc('\n')
}

func runChr() {
	s := "abcabc\x80\xff\x00zq"
	for i := 0; i < len(s); i++ {
		wb(srcA+i, s[i])
	}
	for _, c := range [...]int32{'a', 'c', 'z', 'q', 'y', 0, 0x80, 0xff, -128, -1, 0x161, 0x100, 0x180} {
		for _, n := range [...]int32{0, 1, 3, 4, 8, 9, 10, 11, 100} {
			ps("memchr c=")
			pd(c)
			ps(" n=")
			pd(n)
			ptr("", memchr(at(srcA), c, n))
		}
	}
	ptr("memchr +1 a n=8", memchr(at(srcA+1), 'a', 8))
	setNull("NULLY")
	ptr("memchr null L n=4", memchr(0, 'L', 4))
	ptr("memchr null N n=4", memchr(0, 'N', 4))
	ptr("memchr null q n=4", memchr(0, 'q', 4))
	ptr("memchr null L n=0", memchr(0, 'L', 0))
}

func main() {
	copies("memcpy", func(d, s uint32, n int32) uint32 { return memcpy(d, s, n) })
	copies("memmove", func(d, s uint32, n int32) uint32 { return memmove(d, s, n) })
	copies("bcopy", func(d, s uint32, n int32) uint32 {
		bcopy(s, d, n)
		return 0
	})
	runSet()
	runCmp()
	runChr()
	end()
}
