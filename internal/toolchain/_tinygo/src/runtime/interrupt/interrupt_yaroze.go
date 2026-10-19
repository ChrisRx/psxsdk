//go:build yaroze

// Interrupt control for the Sony PlayStation.
//
// The R3000 masks interrupts through the IEc bit of the COP0 status register,
// but there is no need to touch it directly. The resident library wraps
// exactly that in EnterCriticalSection and ExitCriticalSection, and going
// through the library keeps the kernel's own view of the state consistent
// with ours.
//
// A bare-metal target has to supply this file even when it has nothing
// interesting to say, because the generic interrupt_none.go is built only
// when the baremetal tag is absent.

package interrupt

// EnterCriticalSection disables interrupts and reports whether it was the
// call that did so, which is what makes nesting work.
//
//export EnterCriticalSection
func libps_enterCriticalSection() int32

//export ExitCriticalSection
func libps_exitCriticalSection()

// State records whether the matching Disable was the one that masked
// interrupts, so that nested critical sections only re-enable at the
// outermost Restore.
type State uintptr

// Disable masks interrupts and returns the previous state.
func Disable() (state State) {
	return State(libps_enterCriticalSection())
}

// Restore unmasks interrupts if this Disable was the one that masked them.
func Restore(state State) {
	if state != 0 {
		libps_exitCriticalSection()
	}
}

// In reports whether an interrupt handler is currently running.
//
// Always false here. Installing a Go handler would mean taking over the
// kernel's own dispatcher, which the resident library is in the middle of
// using, so interrupt.New is not supported on this target.
func In() bool {
	return false
}

// Enable would unmask a single interrupt source. Unsupported for the reason
// given on In.
func (irq Interrupt) Enable() {
}
