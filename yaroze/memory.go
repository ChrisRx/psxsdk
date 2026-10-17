package yaroze

import "unsafe"

// Reaching memory by address. A Net Yaroze program loads its data at fixed
// addresses, between the resident library and itself, as the demos' batch
// files sent it there and as package disc reads it there now, and the
// library hands back pointers into memory it owns. Go has no type for
// either, so these turn an address into a pointer in one place, rather
// than in every program with a conversion go vet rightly suspects.

// Pointer returns a pointer to the memory at addr. It is for an address
// that is fixed by the program or given by the library, never for one that
// came from a Go pointer, which must stay a pointer.
//
//go:inline
func Pointer(addr uintptr) unsafe.Pointer {
	// Written as arithmetic from a nil pointer, which is the form go vet
	// accepts, rather than the conversion of an integer it would flag in
	// every build; the compiler folds it to the same thing. It must not be
	// unsafe.Add from nil, which LLVM takes as a promise that the result
	// is inside an object and optimizes on, to the ruin of everything that
	// reads through the pointer.
	return unsafe.Pointer(uintptr(unsafe.Pointer(nil)) + addr)
}

// At returns the memory at addr as words, which is how the library takes
// the data a program loads: a model for GsMapModelingData, a sequence for
// SsSeqOpen, a buffer for CdReadFile.
//
//go:inline
func At(addr uintptr) *uint32 { return (*uint32)(Pointer(addr)) }

// BytesAt returns the memory at addr as bytes, which is how SsVabTransfer
// takes a sound bank.
//
//go:inline
func BytesAt(addr uintptr) *uint8 { return (*uint8)(Pointer(addr)) }

// Scratch returns the CPU's scratchpad, 1KB of memory at 0x1f800000 that
// is as fast as a register, which GsSortObject4 takes as its work space.
//
//go:inline
func Scratch() *uint32 { return At(0x1f800000) }
