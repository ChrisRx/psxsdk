//go:build yaroze

// Command heaplow is a probe for spec 18: where malloc's search starts. A sequence
// of calls is made from a new heap up to each of its calls in turn; then every
// word of the arena but the end marks is overwritten with a free block's header
// so large that any request fits, and malloc(8) returns the first header the
// search reads. Each line is one call, its result, and that place. The state
// after the call is printed too.
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

type op struct {
	kind byte // m, f, r or c
	i, j int
	n    uint32
}

func run(what string, S uint32, ops []op) {
	ps("== ")
	ps(what)
	ps(" size ")
	pi(int32(S))
	ps("\n")
	for k := 0; k <= len(ops); k++ {
		reset()
		initHeap(base(), S)
		for i := range slot {
			slot[i] = 0
		}
		var res uint32
		for _, o := range ops[:k] {
			switch o.kind {
			case 'm':
				res = malloc(o.n)
				slot[o.i] = res
			case 'f':
				free(slot[o.i])
				res = 0
			case 'r':
				res = realloc(slot[o.j], o.n)
				slot[o.i] = res
			case 'c':
				res = calloc(o.n, 1)
				slot[o.i] = res
			}
		}
		if k > 0 {
			o := ops[k-1]
			putc(o.kind)
			putc(' ')
			pi(int32(o.i))
			putc(' ')
			pi(int32(o.j))
			putc(' ')
			pi(int32(o.n))
			ps(" -> ")
			rel(res)
		}
		dump()
		for i := range arena {
			if arena[i] != 0xfffffffe {
				arena[i] = 0x7ffffc01
			}
		}
		ps(" search starts at ")
		rel(malloc(8) - 4)
		ps("\n")
	}
}

func M(i int, n uint32) op    { return op{'m', i, 0, n} }
func F(i int) op              { return op{'f', i, 0, 0} }
func R(i, j int, n uint32) op { return op{'r', i, j, n} }
func C(i int, n uint32) op    { return op{'c', i, 0, n} }

// Sequences must be globals here: a slice literal in a function would go on
// the Go heap at each call.
var seqs = [...]struct {
	what string
	S    uint32
	ops  []op
}{
	{"from the top", 4096, []op{M(0, 16), M(1, 16), M(2, 16), M(3, 16), M(4, 0), M(5, 8000)}},
	{"a free block, taken whole", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), M(3, 16), M(4, 16)}},
	{"a free block, split", 4096, []op{M(0, 16), M(1, 32), M(2, 16), F(1), M(3, 8), M(4, 8), M(5, 4), M(6, 4)}},
	{"a free block too small, then the top", 4096, []op{M(0, 16), M(1, 8), M(2, 16), F(1), M(3, 16), M(4, 4), M(5, 4)}},
	{"two free blocks, the upper fits", 4096, []op{M(0, 8), M(1, 4), M(2, 32), M(3, 4), F(0), F(2), M(4, 16), M(5, 16), M(6, 8), M(7, 8)}},
	{"two free blocks, the lower taken", 4096, []op{M(0, 8), M(1, 4), M(2, 32), M(3, 4), F(0), F(2), M(4, 8), M(5, 16), M(6, 8)}},
	{"free higher then lower", 4096, []op{M(0, 16), M(1, 16), M(2, 16), M(3, 16), F(2), F(0), M(4, 16), M(5, 16)}},
	{"free lower then higher", 4096, []op{M(0, 16), M(1, 16), M(2, 16), M(3, 16), F(0), F(2), M(4, 16), M(5, 16)}},
	{"a join, the search then fails", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(0), F(1), M(3, 8000), M(4, 8)}},
	{"a run at the top, too big", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), F(2), M(3, 8000), M(4, 8)}},
	{"a run at the top, fits", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), F(2), M(3, 64), M(4, 8)}},
	{"everything freed", 4096, []op{M(0, 16), M(1, 16), F(0), F(1), M(2, 8000), M(3, 8)}},
	{"everything freed, the other order", 4096, []op{M(0, 16), M(1, 16), F(1), F(0), M(2, 64), M(3, 8)}},
	{"the top met past a used block", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(0), M(3, 64), M(4, 16), M(5, 16)}},
	{"the end of the heap", 64, []op{M(0, 16), M(1, 16), F(0), M(2, 40), M(3, 16), M(4, 4)}},
	{"realloc shrink", 4096, []op{M(0, 16), M(1, 16), R(2, 0, 4), M(3, 4), M(4, 4)}},
	{"realloc shrink under a free block", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), R(3, 0, 4), M(4, 4), M(5, 4)}},
	{"realloc grow in place, the next block free", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), R(3, 0, 20), M(4, 4), M(5, 4)}},
	{"realloc grow in place, the next block free and lower still", 4096, []op{M(0, 8), M(1, 16), M(2, 16), M(3, 16), F(0), F(2), R(4, 1, 20), M(5, 4)}},
	{"realloc grow, the last block", 4096, []op{M(0, 16), R(1, 0, 32), M(2, 4)}},
	{"realloc grow, past the top", 64, []op{M(0, 16), R(1, 0, 32), R(2, 1, 64), M(3, 4)}},
	{"realloc by a move", 4096, []op{M(0, 16), M(1, 16), R(2, 0, 32), M(3, 4), M(4, 4)}},
	{"realloc by a move into the free block below", 4096, []op{M(0, 32), M(1, 4), M(2, 16), M(3, 4), F(0), R(4, 2, 24), M(5, 4)}},
	{"realloc fails, the next block used", 4096, []op{M(0, 16), M(1, 16), R(2, 0, 8000), M(3, 4)}},
	{"realloc fails, the next block free", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(1), R(3, 0, 8000), M(4, 4), M(5, 4)}},
	{"realloc fails, the next two free", 4096, []op{M(0, 16), M(1, 16), M(2, 16), M(3, 16), F(1), F(2), R(4, 0, 8000), M(5, 4), M(6, 4)}},
	{"realloc fails, the next up to the top", 4096, []op{M(0, 16), M(1, 16), F(1), R(2, 0, 8000), M(3, 4), M(4, 4)}},
	{"realloc to nothing", 4096, []op{M(0, 16), M(1, 16), R(2, 0, 0), M(3, 8), M(4, 8)}},
	{"realloc in place up to the top", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(2), R(3, 1, 64), M(4, 4)}},
	{"realloc in place up to the top, lower free", 4096, []op{M(0, 16), M(1, 16), M(2, 16), F(0), F(2), R(3, 1, 64), M(4, 4)}},
	{"realloc grow, the last, lower free", 4096, []op{M(0, 16), M(1, 16), F(0), R(2, 1, 64), M(3, 4)}},
	{"realloc to nothing, repeated", 4096, []op{M(0, 16), M(1, 16), M(2, 16), R(3, 2, 0), R(4, 1, 0), R(5, 0, 0), M(6, 8), R(7, 6, 0)}},
	{"calloc", 4096, []op{C(0, 16), C(1, 16), F(0), C(2, 8), C(3, 8)}},
}

func main() {
	setup()
	for i := range seqs {
		s := &seqs[i]
		run(s.what, s.S, s.ops)
	}
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
