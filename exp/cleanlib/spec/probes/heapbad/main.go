//go:build yaroze

// Command heapbad is a probe for spec 18: free on pointers that are not blocks
// of the heap, and headers a program has overwritten, with the state printed as
// in the heap probe.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	initHeap func(p, n uint32)
	malloc   func(n uint32) uint32
	free     func(p uint32)
	realloc  func(p, n uint32) uint32
	calloc   func(n, m uint32) uint32
)

func entry(fn unsafe.Pointer, addr uintptr) { (*[2]uintptr)(fn)[1] = addr }

func setup() {
	entry(unsafe.Pointer(&initHeap), 0x80010700)
	entry(unsafe.Pointer(&malloc), 0x80010754)
	entry(unsafe.Pointer(&free), 0x80010924)
	entry(unsafe.Pointer(&realloc), 0x8001095c)
	entry(unsafe.Pointer(&calloc), 0x800109f0)
}

var (
	line  [2048]byte
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

func pi(v int32) {
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

func hex(v uint32) {
	for i := range 8 {
		putc("0123456789abcdef"[v>>(28-4*i)&15])
	}
}

const fill = 0xa5a5a5a5

var arena [1536]uint32

func base() uint32 { return uint32(uintptr(unsafe.Pointer(&arena[0]))) }

// rel prints v as an offset in the arena if it is in or just past it.
func rel(v uint32) {
	b := base()
	if v == 0 {
		ps("0")
	} else if v >= b && v <= b+uint32(len(arena))*4 {
		ps("A+")
		pi(int32(v - b))
	} else {
		ps("x")
		hex(v)
	}
}

func reset() {
	for i := range arena {
		arena[i] = fill
	}
}

// dump prints every word of the arena that is not the fill.
func dump() {
	ps(" [")
	for i := range arena {
		if v := arena[i]; v != fill {
			putc(' ')
			pi(int32(i * 4))
			putc(':')
			rel(v)
		}
	}
	ps(" ]\n")
}

var slot [32]uint32

func start(what string, n uint32) {
	ps("== ")
	ps(what)
	ps(" size ")
	pi(int32(n))
	ps("\n")
	reset()
	initHeap(base(), n)
	for i := range slot {
		slot[i] = 0
	}
}

func m(i int, n uint32) {
	slot[i] = malloc(n)
	ps("m")
	pi(int32(i))
	ps("=malloc(")
	pi(int32(n))
	ps(") ")
	rel(slot[i])
	dump()
}

func f(i int) {
	free(slot[i])
	ps("free m")
	pi(int32(i))
	dump()
}

func r(i, from int, n uint32) {
	slot[i] = realloc(slot[from], n)
	ps("m")
	pi(int32(i))
	ps("=realloc(m")
	pi(int32(from))
	putc(',')
	pi(int32(n))
	ps(") ")
	rel(slot[i])
	dump()
}

func c(i int, n, s uint32) {
	slot[i] = calloc(n, s)
	ps("m")
	pi(int32(i))
	ps("=calloc(")
	pi(int32(n))
	putc(',')
	pi(int32(s))
	ps(") ")
	rel(slot[i])
	dump()
}

func put(off, v uint32) { arena[off/4] = v }

// freeAt frees a block with its header at offset off, holding v, of a heap of S
// at offset h, with used blocks already made, then calls malloc.
func freeAt(h, S, off, v uint32, used int) {
	ps("== header at ")
	pi(int32(off))
	ps(" holds ")
	hex(v)
	ps(" heap at ")
	pi(int32(h))
	ps(" size ")
	pi(int32(S))
	ps(" used ")
	pi(int32(used))
	ps("\n")
	reset()
	initHeap(base()+h, S)
	for i := range used {
		slot[i] = malloc(16)
	}
	put(off, v)
	free(base() + off + 4)
	ps("freed")
	dump()
	m(10, 16)
	m(11, 16)
	m(12, 16)
}

var foreign [4]uint32

func map2(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func main() {
	setup()
	// Above the top, in the heap's range.
	for _, v := range [...]uint32{0, 1, 2, 4, 16, 17, 64, 65, 0x7ffffff0, 0x80000000, 0xa5a5a5a4, 0xfffffffe} {
		freeAt(0, 4096, 400, v, 1)
	}
	// Past the heap's end.
	for _, v := range [...]uint32{16, 64, 0x7ffffff0} {
		freeAt(0, 64, 400, v, 1)
	}
	// Before the heap.
	for _, v := range [...]uint32{16, 64, 0x7ffffff0} {
		freeAt(512, 4096, 400, v, 1)
	}
	// With no block made.
	for _, v := range [...]uint32{16, 64, 0x7ffffff0} {
		freeAt(0, 4096, 400, v, 0)
	}
	// In a block, on the first word of its data.
	for _, v := range [...]uint32{16, 0x7ffffff0} {
		freeAt(0, 4096, 4, v, 2)
	}
	// Outside the array altogether.
	for _, v := range [...]uint32{0x100, 0x7ffffff0} {
		ps("== foreign holds ")
		hex(v)
		ps("\n")
		reset()
		initHeap(base(), 4096)
		m(0, 16)
		foreign = [4]uint32{v, fill, fill, fill}
		free(uint32(uintptr(unsafe.Pointer(&foreign[0]))) + 4)
		m(1, 16)
		hex(foreign[0])
		putc(' ')
		hex(foreign[1])
		putc(' ')
		hex(foreign[2])
		ps("\n")
	}
	// Headers overwritten in a used block.
	type bad struct {
		off, v uint32
		what   string
	}
	for _, b := range [...]bad{
		{20, 0, "second header 0"},
		{20, 1, "second header free, no bytes"},
		{20, 8, "second header short"},
		{20, 100, "second header long, past the top"},
		{20, 0x3f, "second header free, misaligned length"},
		{40, 0, "sentinel 0"},
		{40, 1, "sentinel 1"},
		{0, 0xfffffffe, "first header is the sentinel value"},
		{20, 0xfffffffe, "second header is the sentinel value"},
		{20, 0xffffffff, "second header is all ones"},
	} {
		ps("== bad: ")
		ps(b.what)
		ps("\n")
		reset()
		initHeap(base(), 4096)
		m(0, 16)
		m(1, 16)
		put(b.off, b.v)
		dump()
		m(2, 8)
		m(3, 8)
		m(4, 200)
	}
	// The first call after InitHeap, and after a failure, on a header written
	// by the program.
	for _, fail := range [...]bool{false, true} {
		ps("== crafted header, a failure first: ")
		pi(int32(map2(fail)))
		ps("\n")
		reset()
		initHeap(base(), 4096)
		if fail {
			m(0, 8000)
		}
		put(0, 16)
		put(20, 0xfffffffe)
		dump()
		m(1, 4)
		m(2, 4)
	}
	// Headers the search reads: the lowest free block, overwritten.
	for _, v := range [...]uint32{0, 4, 8, 1, 0x10, 0x11, 0x31, 0x64, 0x65, 0x40, 0x41, 0xfffffffd, 0xffffffff, 0x80000000, 0x80000001} {
		ps("== search reads ")
		hex(v)
		ps("\n")
		reset()
		initHeap(base(), 4096)
		m(0, 16)
		m(1, 16)
		m(2, 16)
		f(0)
		put(0, v)
		dump()
		m(3, 8)
		m(4, 8)
	}
	// The end mark the search reads is not where the top is.
	ps("== a used block past the end mark\n")
	reset()
	initHeap(base(), 4096)
	m(0, 16)
	m(1, 16)
	put(40, 0x10)
	put(60, 0xfffffffe)
	dump()
	m(2, 8)
	m(3, 8)
	ps("== an end mark short of the top\n")
	reset()
	initHeap(base(), 4096)
	m(0, 16)
	m(1, 16)
	f(0)
	put(20, 0xfffffffe)
	dump()
	m(2, 8)
	m(3, 8)
	m(4, 8)
	m(5, 8)
	ps("== a join meets the end mark, not at the top\n")
	reset()
	initHeap(base(), 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	f(0)
	put(20, 0xfffffffe)
	dump()
	m(3, 100)
	m(4, 8)
	ps("== a join meets the top, its mark gone\n")
	reset()
	initHeap(base(), 4096)
	m(0, 16)
	m(1, 16)
	f(1)
	put(40, 0x10)
	dump()
	m(2, 100)
	m(3, 8)
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
