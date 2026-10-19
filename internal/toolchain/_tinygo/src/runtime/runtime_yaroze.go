//go:build yaroze

// Runtime support for the Sony PlayStation (R3000A, MIPS I).
//
// This target expects the program to be linked against the Net Yaroze
// resident library, which the console's boot disc leaves in RAM at a fixed
// address. That library is this platform's C library: putchar, VSync and the
// critical section calls are entry points inside it, and the linker resolves
// them against the address table in targets/playstation-yaroze-symusr.S,
// converted from the SDK's symusr.o.
//
// Most of what a bare-metal runtime normally does has already happened by the
// time Go gets control. The SDK's crt0 sets up the stack, global and frame
// pointers, clears .bss and initialises the library's heap, and only then
// calls main, so preinit has nothing left to do. The heap bounds the
// collector wants come from the generic baremetal.go through linker symbols
// that targets/playstation-yaroze.ld defines.

package runtime

import "unsafe"

//export putchar
func libps_putchar(c int32) int32

//export VSync
func libps_vsync(mode int32) int32

//export InitHeap
func libps_initHeap(head unsafe.Pointer, size uint32)

func putchar(c byte) {
	libps_putchar(int32(c))
}

func getchar() byte {
	// There is no console input on this platform.
	return 0
}

func buffered() int {
	return 0
}

// main is the entry point the crt0 calls, in place of the C main it would
// otherwise find.
//
//export main
func main() {
	// The crt0 handed the library's allocator everything from the end of
	// the image up to the stack, which is where the Go heap goes too. Hand
	// it only the reserve the link script leaves it below _heap_start, so
	// that its malloc, should anything in the library call it, stops there
	// rather than growing into Go's heap. Nothing has allocated from it yet,
	// so starting it again loses nothing.
	libps_initHeap(unsafe.Pointer(globalsEnd), uint32(heapStart-globalsEnd))
	run()
}

// preinit would zero .bss and copy .data out of ROM on a typical bare-metal
// target. Here the crt0 has already cleared .bss, and there is no ROM to copy
// from: the whole image is loaded into RAM before it runs.
func preinit() {}

// The only clock reachable without installing an interrupt handler is the
// vertical blank, so time advances in frames: 1/60s on NTSC.
const nanosecondsPerFrame = 1000 * 1000 * 1000 / 60

func ticksToNanoseconds(ticks timeUnit) int64 {
	return int64(ticks) * nanosecondsPerFrame
}

func nanosecondsToTicks(ns int64) timeUnit {
	return timeUnit(ns / nanosecondsPerFrame)
}

// ticks returns the number of vertical blanks since startup, which is what
// the library counts.
func ticks() timeUnit {
	return timeUnit(libps_vsync(-1))
}

// sleepTicks waits for the given number of frames.
func sleepTicks(d timeUnit) {
	for range d {
		libps_vsync(0)
	}
}

func exit(code int) {
	abort()
}

func abort() {
	// There is nowhere to return to: the loader is gone and nothing else is
	// running. Stopping here at least leaves the last frame on screen.
	for {
	}
}
