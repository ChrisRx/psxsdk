//go:build playstation

// Interrupt control for the Sony PlayStation, standalone.
//
// Masking is the IEc bit of the R3000's COP0 status register, which only
// privileged code can touch, so the two halves are in
// targets/playstation.S.
//
// A bare-metal target has to supply this file even when it has nothing
// interesting to say, because the generic interrupt_none.go is built only
// when the baremetal tag is absent.

package interrupt

//export psx_disable_interrupts
func psx_disable_interrupts() int32

//export psx_restore_interrupts
func psx_restore_interrupts(state int32)

// State records whether the matching Disable was the one that masked
// interrupts, so that nested critical sections only unmask at the outermost
// Restore.
type State int32

// Disable masks interrupts and returns the previous state.
func Disable() (state State) {
	return State(psx_disable_interrupts())
}

// Restore unmasks interrupts if this Disable was the one that masked them.
func Restore(state State) {
	psx_restore_interrupts(int32(state))
}

// In reports whether an interrupt handler is currently running.
//
// Always false. Dispatching an interrupt means installing an exception
// handler at 0x80000080, which overwrites the one the BIOS put there and
// takes responsibility for everything else that arrives through it. That is
// a decision for a program, not for the runtime, so interrupt.New is not
// supported on this target.
func In() bool {
	return false
}

// Enable would unmask a single interrupt source. Unsupported for the reason
// given on In.
func (irq Interrupt) Enable() {
}
