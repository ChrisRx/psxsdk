//go:build yaroze

// Command heapoddfree is a probe for spec 18: free of a pointer that is not a
// multiple of 4, then realloc of one. If the call faults the run stops there,
// which the TTY shows.
package main

import "unsafe"

var (
	initHeap func(p, n uint32)
	malloc   func(n uint32) uint32
	free     func(p uint32)
)

var arena [64]uint32

func main() {
	(*[2]uintptr)(unsafe.Pointer(&initHeap))[1] = 0x80010700
	(*[2]uintptr)(unsafe.Pointer(&malloc))[1] = 0x80010754
	(*[2]uintptr)(unsafe.Pointer(&free))[1] = 0x80010924
	base := uint32(uintptr(unsafe.Pointer(&arena[0])))
	initHeap(base, 64)
	p := malloc(16)
	println("free at an odd address")
	free(p + 1)
	println("returned")
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
