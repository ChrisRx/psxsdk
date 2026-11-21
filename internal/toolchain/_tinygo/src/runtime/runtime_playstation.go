//go:build playstation

// Runtime support for the Sony PlayStation (R3000A, MIPS I).
//
// As the target comes, nothing is assumed to be in memory besides the BIOS.
// The program is the whole of the machine: it is loaded at 0x80010000, owns
// RAM from the end of its own .bss to the stack, and reaches the console's
// hardware by writing to the I/O registers itself. An SDK that puts
// something else under the program, or beside it, changes that through the
// hooks below.
//
// What is deliberately absent is any access to the GPU, the controllers, the
// CD-ROM or the SPU. Those belong in a package of their own rather than in
// the runtime, which only has to answer for memory, time and a way to print.
//
// # The platform hooks
//
// The runtime itself touches no hardware. Everything it needs of the machine
// it asks of the six functions declared below, and it expects the crt0 and
// the link script to have done the rest before main is called. The target's
// files define all of it for a program that owns the console, and define it
// weak, so that a program that does not, one that runs under a development
// kit's resident library, a kernel of its own, or another SDK's C library,
// can replace any piece by defining the same symbol in a C or assembly file
// of its own. Nothing in TinyGo has to change for that: a target file that
// inherits this one and lists the file is enough,
//
//	{
//		"inherits": ["playstation"],
//		"extra-files": ["path/to/hooks.S"]
//	}
//
// and the linker takes the strong definition over the weak one, leaving the
// default out of the program. A hook written in C costs nothing to call if
// it is small: C is optimized together with the program at link time, and
// the hook is inlined into the runtime. The hooks, each with its default:
//
//   - psx_init() is called once, first thing in main, before the heap is
//     set up and before any package is initialized, so it may not allocate.
//     It is the place to start a clock or to tell a library where the
//     program's memory ends. Default (targets/playstation-libc.c): start
//     root counter 2 counting the system clock divided by eight.
//
//   - psx_putchar(c) writes one byte of the program's output, for print,
//     println and a panic's message. Default (targets/playstation.S): the
//     BIOS's std_out_putchar.
//
//   - psx_nanotime() returns the time since start-up in nanoseconds, and
//     must never go backwards. It is called for time.Now and its like.
//     Default (targets/playstation-libc.c): root counter 2, which has to
//     be read at least every 15.5ms to be right.
//
//   - psx_sleep(ns) returns after the given number of nanoseconds, for
//     time.Sleep. A clock coarser than a nanosecond rounds as it sees fit.
//     Default (targets/playstation-libc.c): poll psx_nanotime's counter.
//
//   - psx_disable_interrupts() masks interrupts and returns a state to hand
//     to psx_restore_interrupts(state), which unmasks them if the matching
//     disable was the one that masked them: zero means they were masked
//     already and are to stay so. They bracket the runtime's critical
//     sections and are what runtime/interrupt's Disable and Restore call.
//     Default (targets/playstation.S): the IEc bit of the COP0 status
//     register.
//
// The pieces that are not functions the runtime calls can be replaced the
// same way:
//
//   - _start, the entry point (targets/playstation.S), which sets the stack
//     pointer, clears .bss and calls main. Whatever replaces it has to have
//     .bss cleared by the time it does.
//
//   - memset, memcpy, memmove, memcmp and strlen
//     (targets/playstation-libc.c), which the compiler emits calls to.
//
//   - The link script, by the target file's linkerscript. It has to define
//     what TinyGo's bare-metal runtime reads: _heap_start and _heap_end,
//     _globals_start and _globals_end, and _stack_top.

package runtime

//export psx_init
func psx_init()

//export psx_putchar
func psx_putchar(c int32)

//export psx_nanotime
func psx_nanotime() int64

//export psx_sleep
func psx_sleep(ns int64)

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

// main is the entry point the crt0 jumps to.
//
// There is no preinit, which on a typical bare-metal target clears .bss and
// copies .data out of ROM: the crt0 has cleared .bss, and there is no .data
// to copy, since the loader reads the whole image into RAM before the
// program starts and initialized globals are already where they belong.
//
//export main
func main() {
	psx_init()
	run()
}

// Time is counted in nanoseconds, which is what the clock hooks deal in, so
// a tick is a nanosecond.

func ticksToNanoseconds(ticks timeUnit) int64 {
	return int64(ticks)
}

func nanosecondsToTicks(ns int64) timeUnit {
	return timeUnit(ns)
}

func ticks() timeUnit {
	return timeUnit(psx_nanotime())
}

func sleepTicks(d timeUnit) {
	psx_sleep(int64(d))
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
