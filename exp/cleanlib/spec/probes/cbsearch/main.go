//go:build yaroze

// Command cbsearch is a probe for spec 16: the compare calls bsearch makes, from a
// Go function passed as the C function pointer, logged and printed after each
// call. A compare prints as i:j, the element indexes of its two arguments (K for
// the key, T for a place outside the array). Elements hold their key in the first
// byte.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

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

const arr = 64

var (
	size    int32
	keyAddr uint32
	mode    int
	logA    [4096]uint16
	logB    [4096]uint16
	nlog    int
)

// place is an argument of a compare, as an index, 0xfffe for the key, and 0xffff
// for somewhere else.
func place(p uint32) uint16 {
	if p == keyAddr && keyAddr != 0 {
		return 0xfffe
	}
	if p >= at(arr) && p < at(arr)+3584 && (p-at(arr))%uint32(size) == 0 {
		return uint16((p - at(arr)) / uint32(size))
	}
	return 0xffff
}

func keyOf(p uint32) int32 {
	return int32(volatile.LoadUint8((*uint8)(yaroze.Pointer(uintptr(p | 0x80000000)))))
}

func cmp(a, b uint32) int32 {
	if nlog < len(logA) {
		logA[nlog] = place(a)
		logB[nlog] = place(b)
	}
	nlog++
	d := keyOf(a) - keyOf(b)
	if mode == 5 {
		if d < 0 {
			return -0x7fffffff
		}
		if d > 0 {
			return 0x7fffffff
		}
	}
	return d
}

func cmpAddr() uint32 {
	f := cmp
	return (*[2]uint32)(unsafe.Pointer(&f))[1]
}

func pplace(v uint16) {
	switch v {
	case 0xfffe:
		putc('K')
	case 0xffff:
		putc('T')
	default:
		pd(int32(v))
	}
}

func showLog() {
	ps(" compares=")
	pd(int32(nlog))
	ps(":\n ")
	for i := 0; i < nlog && i < len(logA); i++ {
		pplace(logA[i])
		putc(':')
		pplace(logB[i])
		if i%40 == 39 {
			ps("\n ")
		} else {
			putc(' ')
		}
	}
	putc('\n')
}

// find searches a sorted array of n elements with keys 10, 20 and so on.
func find(n int, key byte) {
	nlog = 0
	put(16, "")
	wb(16, key)
	keyAddr = at(16)
	ps("bsearch n=")
	pd(int32(n))
	ps(" size=")
	pd(size)
	ps(" key=")
	pd(int32(key))
	ps(" ret=")
	r := bsearch(keyAddr, at(arr), int32(n), size, cmpAddr())
	switch {
	case r == 0:
		putc('0')
	case r >= at(arr) && (r-at(arr))%uint32(size) == 0:
		ps("[")
		pd(int32((r - at(arr)) / uint32(size)))
		ps("]")
	default:
		pr(r)
	}
	showLog()
}

func runBsearch() {
	for _, s := range [...]int32{4, 1, 2, 8, 12} {
		size = s
		for _, n := range [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17} {
			if s != 4 && n > 5 {
				continue
			}
			for i := range n {
				p := arr + i*int(size)
				wb(p, byte(10*(i+1)))
				for j := 1; j < int(size); j++ {
					wb(p+j, 0xa5)
				}
			}
			for k := 0; k <= n; k++ {
				find(n, byte(10*k+5))
				if k < n {
					find(n, byte(10*(k+1)))
				}
			}
		}
	}
	// No elements, with the key the same as the place the first would be.
	size = 4
	wb(arr, 10)
	find(0, 10)
	// Equal keys: which of them comes back.
	size = 4
	for _, n := range [...]int{2, 3, 4, 5, 8, 16} {
		for i := range n {
			wb(arr+i*4, 10)
		}
		find(n, 10)
	}
	for i := range 8 {
		k := byte(10)
		if i >= 3 {
			k = 20
		}
		wb(arr+i*4, k)
	}
	find(8, 10)
	find(8, 20)
	// The sign alone counts, whatever its size.
	mode = 5
	for i := range 7 {
		wb(arr+i*4, byte(10*(i+1)))
	}
	find(7, 30)
	find(7, 35)
	mode = 0
	// A null key with no elements.
	nlog = 0
	keyAddr = 0
	ps("bsearch null key n=0 ret=")
	pr(bsearch(0, at(arr), 0, 4, cmpAddr()))
	showLog()
	ps("bsearch null base n=0 ret=")
	pr(bsearch(at(16), 0, 0, 4, cmpAddr()))
	showLog()
}

func main() {
	runBsearch()
	end()
}
