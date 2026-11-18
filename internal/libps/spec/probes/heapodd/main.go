//go:build yaroze

// Command heapodd is a probe for spec 18: InitHeap at an address that is not a
// multiple of 4. If the call faults the run stops there, which the TTY shows.
package main

import "unsafe"

var initHeap func(p, n uint32)

var malloc func(n uint32) uint32

var arena [64]uint32

func main() {
	(*[2]uintptr)(unsafe.Pointer(&initHeap))[1] = 0x80010700
	(*[2]uintptr)(unsafe.Pointer(&malloc))[1] = 0x80010754
	base := uint32(uintptr(unsafe.Pointer(&arena[0])))
	println("InitHeap at an odd address")
	initHeap(base+1, 64)
	println("returned")
	for i := range 4 {
		println(i, arena[i])
	}
	println("malloc", malloc(4)-base)
	for i := range 4 {
		println(i, arena[i])
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
