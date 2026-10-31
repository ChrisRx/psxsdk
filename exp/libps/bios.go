//go:build yaroze

package libps

import "unsafe"

// The kernel's functions, reached through the tables at 0xA0, 0xB0 and
// 0xC0 with the function number in t1. The trampolines are in
// targets/playstation-yaroze-bios.S: a call inside inline assembly would
// clobber registers the compiler is not told about. The library reaches
// each through a stub of its own, one per function, which these are.

//export libps_biosA
func biosA(n, a0, a1, a2, a3 uint32) uint32

//export libps_biosB
func biosB(n, a0, a1, a2, a3 uint32) uint32

//export libps_biosC
func biosC(n, a0, a1, a2, a3 uint32) uint32

// fileWrite is B(0x35) write: n bytes at p to the file fd, 1 being the
// TTY, which is where everything the library prints goes.
//
// libps 0x80042F88
func fileWrite(fd int32, p *byte, n int32) int32 {
	return int32(biosB(0x35, uint32(fd), uint32(uintptr(unsafe.Pointer(p))), uint32(n), 0))
}

// gpuCw is A(0x49) GPU_cw: a command word to GP0 through the kernel.
//
// libps 0x80042E38
func gpuCw(cmd uint32) { biosA(0x49, cmd, 0, 0, 0) }

// cdRemove is A(0x72), which takes the kernel's CD-ROM handlers out of
// the interrupt chain, since the library handles interrupts itself.
//
// libps 0x80042E68
func cdRemove() { biosA(0x72, 0, 0, 0, 0) }

// returnFromException is B(0x17), which ends an interrupt handler by
// restoring the interrupted program. The trampoline calls it after the
// handler; it is here for completeness and does not return.
//
// libps 0x80042EF8
func returnFromException() { biosB(0x17, 0, 0, 0, 0) }

// setDefaultExitFromException is B(0x18): the kernel's own end to an
// exception again.
//
// libps 0x80042F08
func setDefaultExitFromException() { biosB(0x18, 0, 0, 0, 0) }

// setCustomExitFromException is B(0x19): the kernel ends every exception
// by restoring the registers in buf, a jmp_buf, as longjmp would. That is
// how the library gets control after each interrupt.
//
// libps 0x80042F18
func setCustomExitFromException(buf *jmpBuf) {
	biosB(0x19, uint32(uintptr(unsafe.Pointer(buf))), 0, 0, 0)
}

// initPAD is B(0x12): the kernel's pad service, told where to leave each
// port's data.
//
// libps 0x80042EE8
func initPAD(buf0 *uint8, n0 uint32, buf1 *uint8, n1 uint32) {
	biosB(0x12, uint32(uintptr(unsafe.Pointer(buf0))), n0, uint32(uintptr(unsafe.Pointer(buf1))), n1)
}

// startPAD is B(0x13): the kernel's pad service reading the ports at
// every vertical blank.
func startPAD() { biosB(0x13, 0, 0, 0, 0) }

// changeClearPAD is B(0x5B): whether the kernel's pad handler
// acknowledges the vertical blank interrupt, which it must not once the
// library handles that interrupt.
//
// libps 0x80043018
func changeClearPAD(v int32) { biosB(0x5b, uint32(v), 0, 0, 0) }

// changeClearRCnt is C(0x0A): the same for root counter t's interrupt.
//
// libps 0x80043038
func changeClearRCnt(t, v int32) { biosC(0x0a, uint32(t), uint32(v), 0, 0) }

// deliverEvent is B(0x07): the kernel's DeliverEvent, which runs whatever
// the program opened on the event, or nothing.
func deliverEvent(class, spec uint32) { biosB(0x07, class, spec, 0, 0) }

// waitEvent is B(0x0a): WaitEvent, which returns when the event has been
// delivered.
func waitEvent(ev uint32) { biosB(0x0a, ev, 0, 0, 0) }
