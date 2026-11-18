//go:build yaroze

// Command heap is a probe for spec 18: InitHeap, malloc, free, realloc and
// calloc on a heap in an array of the program's own. Every call prints its
// result and then every word of the array that is not the fill, offsets and
// addresses in the array relative to its start, so a block's header and every
// byte that moves show.
//
// A Go program's malloc, free and calloc names resolve to the Go runtime's at
// link time, not the library's, so this probe reaches the library's through
// their entry addresses. The last group prints cycle counts, which are for a
// comparison by eye and differ between images.
package main

import (
	"runtime/volatile"
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
	line  [1024]byte
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

// mark writes words into a block so that a move shows.
func mark(i int, bytes uint32, tag uint32) {
	p := (*[64]uint32)(unsafe.Pointer(uintptr(slot[i])))
	for k := uint32(0); k < bytes/4; k++ {
		p[k] = tag + k
	}
	ps("mark m")
	pi(int32(i))
	putc(' ')
	pi(int32(bytes))
	putc(' ')
	hex(tag)
	ps("\n")
}

// reach brings the top of a new heap of size S to P by steps of malloc.
func reach(S, P uint32) bool {
	initHeap(base(), S)
	p := uint32(0)
	for p < P {
		k := P - p - 4
		if k != 0 && malloc(k) != 0 {
			p += k + 4
			continue
		}
		// Too large a step, or an end met: two smaller ones.
		k = (P - p) / 2 &^ 3
		if k < 8 {
			if P-p == 4 {
				return false
			}
			k = 8
		}
		if malloc(k-4) == 0 {
			return false
		}
		p += k
	}
	return true
}

// grid prints, for a heap of size S and each place the top can be brought to
// by successful calls, whether malloc takes each size: x yes, . no, - place not
// reached.
func grid(S uint32) {
	ps("grid S=")
	pi(int32(S))
	ps("\n")
	for P := uint32(0); P <= S+16; P += 4 {
		pi(int32(P))
		ps(": ")
		for n := uint32(4); n <= S+16; n += 4 {
			if !reach(S, P) {
				putc('-')
				continue
			}
			p := malloc(n)
			switch p {
			case 0:
				putc('.')
			case base() + P + 4:
				putc('x')
			default:
				putc('?')
			}
		}
		ps("\n")
	}
}

func basics() {
	for n := uint32(0); n <= 9; n++ {
		start("round", 4096)
		m(0, n)
		m(1, 4)
	}
	// A free block of 32 between two used ones, requests of 0 to 40.
	for n := uint32(0); n <= 40; n += 4 {
		start("split", 4096)
		m(0, 4)
		m(1, 32)
		m(2, 4)
		f(1)
		m(3, n)
		m(4, 4)
	}
	// Three free in a row: the scan joins them only as far as it must.
	start("merge", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	m(3, 16)
	f(0)
	f(1)
	f(2)
	m(4, 8)
	m(5, 8)
	m(6, 32)
	start("merge as needed", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	m(3, 16)
	f(0)
	f(1)
	f(2)
	m(4, 20)
	m(5, 16)
	start("merge fail", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	m(3, 16)
	f(0)
	f(1)
	f(2)
	m(4, 5000)
	f(3)
	m(5, 5000)
	m(6, 100)
	start("free run at top, fits at its head", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	f(1)
	f(2)
	m(3, 32)
	m(4, 8)
	start("free run at top, too big for the run", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	f(1)
	f(2)
	m(3, 64)
	m(4, 8)
	start("free run at top, an unrelated small call leaves it", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	f(1)
	f(2)
	m(3, 16)
	m(4, 16)
	// First fit against best fit, and whether a search resumes.
	start("order", 4096)
	m(0, 64)
	m(1, 4)
	m(2, 16)
	m(3, 4)
	m(4, 64)
	m(5, 4)
	f(0)
	f(2)
	f(4)
	m(6, 16)
	m(7, 16)
	m(8, 16)
	m(9, 16)
	start("rover", 4096)
	m(0, 16)
	m(1, 16)
	m(2, 16)
	f(0)
	m(3, 32)
	m(4, 16)
	start("last", 4096)
	m(0, 16)
	m(1, 64)
	f(1)
	m(2, 8)
	m(3, 8)
	f(3)
	f(2)
	m(4, 100)
	// A free block of no bytes, its header alone.
	start("empty block", 4096)
	m(0, 28)
	m(1, 4)
	f(0)
	m(2, 24)
	m(3, 4)
	m(4, 4)
	start("empty block joins", 4096)
	m(0, 28)
	m(1, 4)
	m(2, 4)
	f(0)
	m(3, 24)
	f(1)
	m(4, 4)
	m(5, 8)
}

func limits() {
	for _, s := range [...]uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 16, 20, 24, 28, 32} {
		start("small", s)
		m(0, 0)
		f(0)
		start("small", s)
		m(0, 4)
		m(1, 4)
		m(2, 4)
	}
	for _, s := range [...]uint32{4096, 4097, 4098, 4099} {
		for _, n := range [...]uint32{4084, 4088, 4092, 4096, 4100} {
			start("exact", s)
			m(0, n)
			m(1, 0)
			m(2, 4)
		}
	}
	for _, s := range [...]uint32{0x7fffffff, 0x80000000, 0xfffffff0, 0xffffffff} {
		start("a heap past the end of memory", s)
		m(0, 16)
		m(1, 4000)
		m(2, 2000)
	}
	reset()
	grid(16)
	grid(32)
	grid(64)
	grid(68)
	start("exhaust", 64)
	for i := range 10 {
		m(i, 8)
	}
	f(0)
	m(10, 8)
	m(11, 8)
	start("exhaust, then free the last", 40)
	m(0, 8)
	m(1, 8)
	m(2, 8)
	m(3, 8)
	f(2)
	m(4, 16)
	m(5, 8)
	m(6, 8)
	start("exhaust, top past the end", 16)
	m(0, 4)
	m(1, 8)
	m(2, 4)
	m(3, 16)
	m(4, 4)
	f(2)
	m(5, 16)
	for _, n := range [...]uint32{0x7ffffff0, 0x7fffffff, 0x80000000, 0xfffffff0, 0xfffffffc, 0xfffffffd, 0xfffffffe, 0xffffffff} {
		start("huge", 4096)
		m(0, n)
		m(1, 4)
	}
}

var foreign = [4]uint32{0x100, 0x202, 0xfffffffe, 0x7ffffff0}

func frees() {
	start("free null", 4096)
	m(0, 16)
	free(0)
	ps("free(0)")
	dump()
	m(1, 16)
	start("double free", 4096)
	m(0, 16)
	m(1, 16)
	f(0)
	f(0)
	m(2, 16)
	m(3, 16)
	f(2)
	f(2)
	f(3)
	f(3)
	m(4, 32)
	start("free an inner pointer", 4096)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	free(slot[0] + 4)
	ps("free(m0+4)")
	dump()
	m(2, 16)
	start("free a foreign pointer", 4096)
	m(0, 16)
	for i := range foreign {
		free(uint32(uintptr(unsafe.Pointer(&foreign[i]))) + 4)
		ps("foreign ")
		pi(int32(i))
		for k := range foreign {
			putc(' ')
			hex(foreign[k])
		}
		dump()
	}
	m(1, 16)
	start("free outside the heap, in the arena", 64)
	free(base() + 200)
	ps("free(A+200)")
	dump()
	m(0, 8)
	f(0)
	m(1, 4)
	m(2, 4)
}

func reallocs() {
	start("realloc null", 4096)
	slot[1] = 0
	r(0, 1, 16)
	r(2, 1, 0)
	r(3, 1, 1)
	start("realloc to 0, a middle block", 4096)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	r(2, 0, 0)
	m(3, 8)
	start("realloc to 0, the last block", 4096)
	m(0, 16)
	r(1, 0, 0)
	m(2, 8)
	for _, n := range [...]uint32{1, 4, 8, 9, 12, 13, 16} {
		start("realloc shrink, a middle block", 4096)
		m(0, 16)
		m(1, 16)
		mark(0, 16, 0x11110000)
		r(2, 0, n)
		m(3, 4)
	}
	for _, n := range [...]uint32{4, 8, 12, 16} {
		start("realloc shrink, the last block", 4096)
		m(0, 16)
		mark(0, 16, 0x11110000)
		r(1, 0, n)
		m(2, 4)
	}
	for _, n := range [...]uint32{17, 20, 24, 28, 32, 36, 40, 44, 56, 60} {
		start("realloc grow, the next block free (16)", 4096)
		m(0, 16)
		m(1, 16)
		m(2, 16)
		m(3, 16)
		mark(0, 16, 0x11110000)
		f(1)
		r(4, 0, n)
		m(5, 4)
	}
	for _, n := range [...]uint32{20, 32, 60} {
		start("realloc grow, the next two blocks free", 4096)
		m(0, 16)
		m(1, 16)
		m(2, 16)
		m(3, 16)
		mark(0, 16, 0x11110000)
		f(1)
		f(2)
		r(4, 0, n)
		m(5, 4)
	}
	for _, n := range [...]uint32{20, 64, 300, 5000} {
		start("realloc grow, the next block used", 4096)
		m(0, 16)
		m(1, 16)
		mark(0, 16, 0x11110000)
		r(2, 0, n)
		m(3, 4)
	}
	for _, n := range [...]uint32{20, 64, 4000, 5000} {
		start("realloc grow, the last block", 4096)
		m(0, 16)
		mark(0, 16, 0x11110000)
		r(1, 0, n)
		m(2, 4)
	}
	start("realloc grow, the free block before it fits", 4096)
	m(0, 32)
	m(1, 16)
	m(2, 16)
	mark(1, 16, 0x22220000)
	f(0)
	r(3, 1, 24)
	m(4, 4)
	start("realloc grow, the next block free up to the top", 4096)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	f(1)
	r(2, 0, 64)
	m(3, 4)
	start("realloc a freed block", 4096)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	f(0)
	r(2, 0, 16)
	m(3, 4)
	start("realloc a freed block, larger", 4096)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	f(0)
	r(2, 0, 24)
	m(3, 4)
	start("realloc at the end of the heap", 64)
	m(0, 16)
	m(1, 16)
	mark(0, 16, 0x11110000)
	r(2, 0, 28)
	r(3, 2, 40)
	m(4, 4)
	for _, n := range [...]uint32{21, 22, 23, 24, 25} {
		start("realloc by a move, bytes copied", 4096)
		m(0, 16)
		m(1, 16)
		mark(0, 16, 0x11110000)
		mark(1, 16, 0x22220000)
		r(2, 0, n)
	}
	for _, n := range [...]uint32{4, 8, 12, 16, 20} {
		start("realloc a freed block, again", 4096)
		m(0, 16)
		m(1, 16)
		f(0)
		r(2, 0, n)
		m(3, 4)
	}
	for _, n := range [...]uint32{20, 36, 40, 64} {
		start("realloc grow, the next block free, lower blocks free", 4096)
		m(0, 16)
		m(1, 16)
		m(2, 16)
		m(3, 16)
		f(0)
		f(2)
		r(4, 1, n)
		m(5, 4)
		m(6, 4)
	}
	for _, n := range [...]uint32{20, 64, 8000} {
		start("realloc grow, the last block, a lower block free", 4096)
		m(0, 16)
		m(1, 16)
		f(0)
		r(2, 1, n)
		m(3, 4)
		m(4, 4)
	}
	for _, n := range [...]uint32{36, 40, 64, 8000} {
		start("realloc grow, the next block free up to the top, again", 4096)
		m(0, 16)
		m(1, 16)
		m(2, 16)
		f(2)
		r(3, 1, n)
		m(4, 4)
		m(5, 4)
	}
	for _, n := range [...]uint32{0xfffffffd, 0xfffffffe, 0xffffffff} {
		start("realloc huge", 4096)
		m(0, 16)
		m(1, 16)
		mark(0, 16, 0x11110000)
		r(2, 0, n)
		m(3, 4)
	}
	for _, n := range [...]uint32{0x7ffffff0, 0x80000000, 0xfffffffc} {
		start("realloc huge", 4096)
		m(0, 16)
		m(1, 16)
		mark(0, 16, 0x11110000)
		r(2, 0, n)
		m(3, 4)
	}
}

// reallocGrid prints, for a heap of size S with one block of each size, what
// realloc makes of each size: i in place, m moved, . null. With next set, a second
// block lies behind it, used (u) or free (f).
func reallocGrid(S uint32) {
	for _, kind := range [...]byte{'l', 'u', 'f'} {
		ps("realloc grid S=")
		pi(int32(S))
		putc(' ')
		putc(kind)
		ps("\n")
		for k := uint32(4); k <= S; k += 4 {
			pi(int32(k))
			ps(": ")
			for n := uint32(4); n <= S+16; n += 4 {
				initHeap(base(), S)
				p := malloc(k)
				if kind != 'l' {
					q := malloc(8)
					if q == 0 || p == 0 {
						putc('-')
						continue
					}
					if kind == 'f' {
						free(q)
					}
				}
				if p == 0 {
					putc('-')
					continue
				}
				switch realloc(p, n) {
				case p:
					putc('i')
				case 0:
					putc('.')
				default:
					putc('m')
				}
			}
			ps("\n")
		}
	}
}

func callocs() {
	reallocGrid(64)
	reallocGrid(68)
	start("calloc", 4096)
	c(0, 4, 4)
	c(1, 3, 3)
	c(2, 1, 1)
	c(3, 0, 4)
	c(4, 4, 0)
	c(5, 0, 0)
	c(6, 2, 8)
	c(7, 8, 2)
	start("calloc over dirty memory", 4096)
	m(0, 16)
	mark(0, 16, 0x11110000)
	f(0)
	c(1, 2, 4)
	c(2, 1, 4)
	start("calloc overflow", 4096)
	c(0, 0x10000, 0x10000)
	c(1, 0x10001, 0x10000)
	c(2, 0x40000001, 4)
	c(3, 0x80000000, 2)
	c(4, 0x80000001, 2)
	c(5, 0xffffffff, 4)
	c(6, 0xffffffff, 0xffffffff)
	c(7, 4, 4)
	start("calloc too big", 64)
	c(0, 7, 8)
	c(1, 8, 8)
	c(2, 2, 4)
}

func inits() {
	start("init twice", 4096)
	initHeap(base(), 4096)
	dump()
	m(0, 16)
	m(1, 16)
	f(0)
	initHeap(base(), 4096)
	ps("init again")
	dump()
	m(2, 8)
	m(3, 8)
	initHeap(base()+64, 4000)
	ps("init elsewhere")
	dump()
	m(4, 8)
	f(4)
	initHeap(base(), 4096)
	ps("init back")
	dump()
	m(5, 8)
}

// seed is the mixed sequence's pseudo-random source.
var seed uint32 = 0x2545f491

func rnd(n uint32) uint32 {
	seed ^= seed << 13
	seed ^= seed >> 17
	seed ^= seed << 5
	return (seed >> 8) % n
}

// mixed makes random calls on a heap of 1024 bytes, each printed with the whole
// state: the slots a program's blocks would be in.
func mixed(count int) {
	start("mixed", 1024)
	for range count {
		i := int(rnd(16))
		switch k := rnd(10); {
		case slot[i] == 0 && k < 7:
			n := rnd(120)
			if rnd(6) == 0 {
				n = rnd(600)
			}
			m(i, n)
		case slot[i] == 0 && k < 9:
			c(i, rnd(12), rnd(24))
		case slot[i] == 0:
			r(i, i, rnd(200))
		case k < 6:
			f(i)
			slot[i] = 0
		case k < 9:
			// Keep the old pointer when it fails, as a program does.
			old := slot[i]
			r(i, i, rnd(300))
			if slot[i] == 0 {
				slot[i] = old
			}
		default:
			j := int(rnd(16))
			if slot[j] != 0 {
				f(j)
				f(j)
				slot[j] = 0
			}
		}
	}
}

// The second counter, in system clock mode.
var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))
)

//go:noinline
func nothing(a uint32) uint32 { return a }

const runs = 7

// timed prints the least cycles over several runs of call, after prep, less
// what a call of nothing through the same kind of value costs.
func timed(what string, prep func(), call func()) {
	best := uint32(0xffffffff)
	for range runs {
		prep()
		volatile.StoreUint32(mode, 0)
		t0 := volatile.LoadUint32(count)
		call()
		t1 := volatile.LoadUint32(count)
		if t1-t0 < best {
			best = t1 - t0
		}
	}
	ps("cost ")
	ps(what)
	putc(' ')
	pi(int32(best))
	ps("\n")
}

// heapWith makes a heap with used blocks, and then n free blocks of 4 bytes,
// the search starting at the first: apart with a used block after each, or in a
// row.
func heapWith(used, n int, adjacent bool) {
	initHeap(base(), 4096)
	for range used {
		malloc(4)
	}
	for i := range n {
		p := malloc(4)
		slot[i] = p
		if !adjacent {
			malloc(4)
		}
	}
	malloc(4)
	for i := n - 1; i >= 0; i-- {
		free(slot[i])
	}
}

func costs() {
	reset()
	fn := nothing
	timed("nothing", func() {}, func() { fn(0) })
	for _, n := range [...]int{1, 2, 4, 8, 16, 31} {
		ps("used blocks ")
		pi(int32(n))
		ps("\n")
		timed("malloc(16) from the top", func() {
			heapWith(n, 0, false)
		}, func() { malloc(16) })
		timed("malloc(16) past that many free too small", func() {
			heapWith(0, n, false)
		}, func() { malloc(16) })
		timed("malloc joining that many free", func() {
			heapWith(0, n, true)
		}, func() { malloc(uint32(n)*8 - 4) })
	}
	var p uint32
	timed("free", func() {
		initHeap(base(), 4096)
		p = malloc(16)
	}, func() { free(p) })
	timed("free null", func() {}, func() { free(0) })
	timed("malloc(0)", func() { initHeap(base(), 4096) }, func() { malloc(0) })
	timed("malloc(16), a block taken whole", func() {
		initHeap(base(), 4096)
		p = malloc(16)
		malloc(4)
		free(p)
	}, func() { malloc(16) })
	timed("malloc(4), a block split", func() {
		initHeap(base(), 4096)
		p = malloc(16)
		malloc(4)
		free(p)
	}, func() { malloc(4) })
	timed("InitHeap", func() {}, func() { initHeap(base(), 4096) })
	timed("realloc null", func() { initHeap(base(), 4096) }, func() { realloc(0, 16) })
	for _, n := range [...]uint32{16, 64, 256, 1024} {
		ps("bytes ")
		pi(int32(n))
		ps("\n")
		timed("realloc, same size", func() {
			initHeap(base(), 4096)
			p = malloc(n)
		}, func() { realloc(p, n) })
		timed("realloc, by a move", func() {
			initHeap(base(), 4096)
			p = malloc(n)
			malloc(4)
		}, func() { realloc(p, n+4) })
		timed("calloc", func() { initHeap(base(), 4096) }, func() { calloc(1, n) })
		timed("realloc, in place to the next free", func() {
			initHeap(base(), 4096)
			p = malloc(n)
			q := malloc(n)
			malloc(4)
			free(q)
		}, func() { realloc(p, n+4) })
	}
}

func main() {
	setup()
	basics()
	limits()
	frees()
	reallocs()
	callocs()
	inits()
	mixed(120)
	costs()
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
