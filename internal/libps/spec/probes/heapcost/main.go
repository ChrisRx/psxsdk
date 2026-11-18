//go:build yaroze

// Command heapcost is a probe for spec 18: each call once, in an order the kernel
// log can read with PROBE_WATCH on the four entries (80010754, 80010924, 8001095c,
// 800109f0), whose lines give the cycles at each entry and return. Each case runs
// three times, and the last is the one to read. It prints nothing but END, and the
// second counter around each case, to set against the cycles.
package main

import (
	"runtime/volatile"
	"unsafe"
)

var (
	initHeap func(p, n uint32)
	malloc   func(n uint32) uint32
	free     func(p uint32)
	realloc  func(p, n uint32) uint32
	calloc   func(n, m uint32) uint32
)

var arena [1536]uint32

func entry(fn unsafe.Pointer, addr uintptr) { (*[2]uintptr)(fn)[1] = addr }

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))
)

func main() {
	entry(unsafe.Pointer(&initHeap), 0x80010700)
	entry(unsafe.Pointer(&malloc), 0x80010754)
	entry(unsafe.Pointer(&free), 0x80010924)
	entry(unsafe.Pointer(&realloc), 0x8001095c)
	entry(unsafe.Pointer(&calloc), 0x800109f0)
	base := uint32(uintptr(unsafe.Pointer(&arena[0])))
	for range 3 {
		initHeap(base, 4096)
		malloc(0)
		volatile.StoreUint32(mode, 0)
		t0 := volatile.LoadUint32(count)
		p := malloc(16) // from the top
		t1 := volatile.LoadUint32(count)
		println("counter malloc(16) from the top", t1-t0)
		q := malloc(16)
		malloc(4)
		free(p)
		free(0)
		malloc(16) // a block taken whole
		free(p)
		malloc(4) // a block split
		free(q)
		p = malloc(32) // a join of two
		realloc(p, 32)
		realloc(p, 16)
		realloc(p, 24)
		calloc(1, 16)
		calloc(1, 256)
		initHeap(base, 4096)
		realloc(0, 16)
		p = malloc(256)
		malloc(4)
		realloc(p, 260)
		initHeap(base, 4096)
		malloc(8000)
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
