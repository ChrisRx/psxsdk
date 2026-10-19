//go:build playstation

// The architecture details arch_mipsle.go leaves out.
//
// That file carries no build constraint and is shared with linux/mipsle. It
// supplies GOARCH, TargetBits, align, callInstSize and
// getCurrentStackPointer, all of which are equally true of a bare-metal
// R3000A. What it does not supply are the pieces that only a target with no
// operating system underneath it has to answer for, and which on linux come
// from runtime_unix.go.

package runtime

// zeroSizeAllocPtr is handed out for zero sized allocations. It only has to
// be a non-nil address that no real object can occupy and that nothing will
// ever dereference. Address 16 is in the middle of the BIOS exception
// vectors at the bottom of RAM, which is never part of the heap.
const zeroSizeAllocPtr uintptr = 16

// procPin and procUnpin keep a goroutine pinned to its processor. There is
// one processor and, with the scheduler disabled, one goroutine, so masking
// interrupts is enough to make the section atomic with respect to anything
// that could observe it.
var procPinnedMask int32

//go:linkname procPin sync/atomic.runtime_procPin
func procPin() {
	procPinnedMask = psx_disable_interrupts()
}

//go:linkname procUnpin sync/atomic.runtime_procUnpin
func procUnpin() {
	psx_restore_interrupts(procPinnedMask)
}
