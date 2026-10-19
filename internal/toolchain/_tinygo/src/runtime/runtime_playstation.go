//go:build playstation

// Runtime support for the Sony PlayStation (R3000A, MIPS I), standalone.
//
// Nothing is assumed to be in memory besides the BIOS. The program is the
// whole of the machine: it is loaded at 0x80010000, owns RAM from the end of
// its own .bss to the stack, and reaches the console's hardware by writing
// to the I/O registers itself.
//
// This is the plain console. For a Net Yaroze, where the boot disc leaves
// Sony's development library resident and the program calls into it, see the
// playstation-yaroze target: there the library is the C library and provides
// the clock, the console and the critical sections that are done here by
// hand.
//
// What is deliberately absent is any access to the GPU, the controllers, the
// CD-ROM or the SPU. Those belong in a package of their own rather than in
// the runtime, which only has to answer for memory, time and a way to print.

package runtime

import "unsafe"

// Written in targets/playstation.S, because they are a BIOS call and two
// COP0 writes.

//export psx_putchar
func psx_putchar(c int32)

//export psx_disable_interrupts
func psx_disable_interrupts() int32

//export psx_restore_interrupts
func psx_restore_interrupts(state int32)

func putchar(c byte) {
	psx_putchar(int32(c))
}

func getchar() byte {
	// The BIOS console is output only unless a serial cable is attached and
	// driven, which is not something the runtime should assume.
	return 0
}

func buffered() int {
	return 0
}

//go:extern _sbss
var _sbss [0]byte

//go:extern _ebss
var _ebss [0]byte

// main is the entry point the crt0 jumps to.
//
//export main
func main() {
	preinit()
	initTimer()
	run()
}

// preinit clears .bss. There is no .data to copy: the loader reads the whole
// image into RAM before the program starts, so initialised globals are
// already where they belong.
//
// It runs before anything else in Go, which is why it may not touch a global
// of its own.
func preinit() {
	ptr := unsafe.Pointer(&_sbss)
	for ptr != unsafe.Pointer(&_ebss) {
		*(*uint32)(ptr) = 0
		ptr = unsafe.Add(ptr, 4)
	}
}

// Root counter 2, one of the three hardware timers, is the clock. Counter 0
// is tied to the dot clock and counter 1 to the horizontal retrace, both of
// which stop meaning anything if the program reprograms the display; counter
// 2 is driven by the CPU clock and keeps running whatever the GPU is doing.
//
// I/O is uncached at these addresses in every region, so no cache
// maintenance is needed around them.
const (
	timerCount  = 0x1f801120 // current value, 16 bits
	timerMode   = 0x1f801124 // clock source and counting mode
	timerTarget = 0x1f801128

	// Clock source 2 is the CPU clock divided by eight. Writing the mode
	// register also resets the counter.
	timerModeDiv8 = 2 << 8

	// 33.8688MHz divided by eight. Expressed as a ratio against a
	// nanosecond it is 312500/1323 exactly, which is what the conversions
	// below use instead of rounding to 236ns a tick.
	timerNsNum = 312500
	timerNsDen = 1323
)

func mmio32(addr uintptr) *uint32 {
	return (*uint32)(unsafe.Pointer(addr))
}

func initTimer() {
	*mmio32(timerTarget) = 0
	*mmio32(timerMode) = timerModeDiv8
	timerLast = 0
	timerHigh = 0
}

// The counter is 16 bits and wraps every 15.5ms, so the high bits are kept
// in software and advanced whenever a read comes back lower than the one
// before it.
//
// That makes the clock correct only if it is read more often than it wraps.
// sleepTicks polls, so a sleep of any length is accounted for exactly, but a
// program that computes a duration across more than 15.5ms of doing nothing
// else will read it short. Keeping the whole thing honest would mean taking
// the timer interrupt, which would mean owning the exception vector, which is
// not the runtime's to own on a machine where the program may want it.
var (
	timerLast uint16
	timerHigh uint64
)

func ticks() timeUnit {
	now := uint16(*mmio32(timerCount))
	if now < timerLast {
		timerHigh += 1 << 16
	}
	timerLast = now
	return timeUnit(timerHigh + uint64(now))
}

// Both conversions are exact. They overflow after about 80 days of uptime,
// which is longer than the hardware has ever been asked to stay on.
func ticksToNanoseconds(ticks timeUnit) int64 {
	return int64(ticks) * timerNsNum / timerNsDen
}

func nanosecondsToTicks(ns int64) timeUnit {
	return timeUnit(ns * timerNsDen / timerNsNum)
}

// sleepTicks busy-waits. With the scheduler disabled there is nothing else to
// run, and halting the CPU would need an interrupt to wake it.
func sleepTicks(d timeUnit) {
	target := ticks() + d
	for ticks() < target {
	}
}

func exit(code int) {
	abort()
}

func abort() {
	// There is nowhere to return to. The BIOS has been overwritten as far as
	// its RAM-resident parts go, nothing else is running, and a retail
	// console has no reset vector to jump to that would not just run this
	// program again.
	for {
	}
}
