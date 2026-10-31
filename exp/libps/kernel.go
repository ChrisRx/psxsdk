//go:build yaroze

package libps

import "device"

// The critical sections are kernel syscalls: 1 masks interrupts and 2
// unmasks them. Each returns what the kernel leaves in v0, the state
// before the call, which the runtime keeps to know whether to unmask.
//
// The assembly sets a0, which the compiler is not told about, so neither
// function may be inlined into a caller that has something in a0.

// EnterCriticalSection masks interrupts.
//
// libps 0x80042F28, 16 bytes
//
//go:noinline
//export EnterCriticalSection
func EnterCriticalSection() int32 {
	return int32(device.AsmFull(`.set push
	.set noat
	li $$a0, 1
	syscall 0
	move {}, $$v0
	.set pop`, nil))
}

// ExitCriticalSection unmasks interrupts.
//
// libps 0x80042F38, 16 bytes
//
//go:noinline
//export ExitCriticalSection
func ExitCriticalSection() {
	device.AsmFull(`li $$a0, 2
	syscall 0`, nil)
}
