//go:build yaroze

package cleanlib

import "unsafe"

// The memory cards and the kernel's file functions (spec 11). The file
// calls are the kernel's own, passed straight through: the kernel keeps
// the memory card file system ("bu00:" and "bu10:") and the descriptors.
// What the library adds is the card driver's start and TestCard.

// The kernel functions the file and card calls use.
const (
	biosOpen      = 0x32 // B0: opens a file
	biosLseek     = 0x33 // B0: moves a descriptor's position
	biosRead      = 0x34 // B0: reads from a descriptor
	biosClose     = 0x36 // B0: closes a descriptor
	biosFormat    = 0x41 // B0: formats a memory card
	biosFirstFile = 0x42 // B0: the first file matching a name
	biosNextFile  = 0x43 // B0: the next
	biosRename    = 0x44 // B0: renames a file
	biosErase     = 0x45 // B0: deletes a file
	biosTestEvent = 0x0b // B0: whether an event has come, clearing it
	biosInitCard  = 0x4a // B0: sets up the card driver
	biosStartCard = 0x4b // B0: starts it
	biosCardWrite = 0x4e // B0: writes a sector of a card
	biosCardRead  = 0x4f // B0: reads one
	biosNewCard   = 0x50 // B0: has the driver take a card as known
	biosGetErrno  = 0x54 // B0: the error the last file call left
	biosBuInit    = 0x70 // A0: sets up the card file system
	biosCardInfo  = 0xab // A0: asks a card how it is
)

// The card driver's event classes: the file system's (software) and the
// driver's own (hardware).
const (
	cardSwClass = 0xf4000001
	cardHwClass = 0xf0000011
)

// The events of each class, in the order the library opens and tests
// them: the operation done, an error, no answer, and a new card.
var cardSpecs = [4]uintptr{0x0004, 0x8000, 0x0100, 0x2000}

const (
	cardDone = iota
	cardError
	cardTimeout
	cardNew
)

var card struct {
	started bool
	sw, hw  [4]int32

	// sector is where TestCard reads a new card's first sector.
	sector [128]byte
}

// cardStart starts the card driver, once, as Sony's library starts it
// before the program: from the library's start-up (startup, last of the
// three systems), or from the first card call in a program that makes
// one before any ResetGraph. It opens the eight events, starts the driver
// and the file system, has the kernel leave the vertical blank's
// acknowledgement to the library again, and enables the events.
func cardStart() {
	if card.started {
		return
	}
	card.started = true
	// The driver reads the controllers too, into the buffers the
	// controller driver was given: without them, into address 0.
	padStart()
	for i, spec := range cardSpecs {
		card.sw[i] = libps_biosB(biosOpenEvent, cardSwClass, spec, evModeNoIntr, 0)
	}
	for i, spec := range cardSpecs {
		card.hw[i] = libps_biosB(biosOpenEvent, cardHwClass, spec, evModeNoIntr, 0)
	}
	libps_biosB(biosInitCard, 1, 0, 0, 0)
	libps_biosB(biosStartCard, 0, 0, 0, 0)
	libps_biosA(biosBuInit, 0, 0, 0, 0)
	libps_biosB(biosClearPad, 0, 0, 0, 0)
	for _, ev := range card.sw {
		libps_biosB(biosEnableEvent, uintptr(ev), 0, 0, 0)
	}
	for _, ev := range card.hw {
		libps_biosB(biosEnableEvent, uintptr(ev), 0, 0, 0)
	}
}

// cardFile starts the card driver if name is on a memory card ("bu").
func cardFile(name *byte) {
	if name == nil {
		return
	}
	p := unsafe.Slice(name, 2)
	if p[0]|0x20 == 'b' && p[1]|0x20 == 'u' {
		cardStart()
	}
}

// cardWait tests the events evs in turn until one has come, and returns
// which.
func cardWait(evs *[4]int32) int {
	for {
		for i, ev := range evs {
			if libps_biosB(biosTestEvent, uintptr(ev), 0, 0, 0) == 1 {
				return i
			}
		}
	}
}

// cardClear tests each of the driver's events once, clearing any that
// has come.
func cardClear() {
	for _, ev := range card.hw {
		libps_biosB(biosTestEvent, uintptr(ev), 0, 0, 0)
	}
}

// TestCard asks the card in slot (0 or 1) how it is and waits for the
// answer: 1 for a card, 0 for none, 3 for an error. A card put in since
// the last test is taken as known, a sector written and its first sector
// read: 2 if that holds a directory ("MC"), 4 if not, 0 if the read got
// no answer.
//
//export TestCard
func TestCard(slot int32) int32 {
	cardStart()
	port := uintptr(slot) << 4
	libps_biosA(biosCardInfo, port, 0, 0, 0)
	switch cardWait(&card.sw) {
	case cardDone:
		return 1
	case cardError:
		return 3
	case cardTimeout:
		return 0
	}
	cardClear()
	libps_biosB(biosNewCard, port, 0, 0, 0)
	libps_biosB(biosCardWrite, port, 0x3f, 0, 0)
	cardWait(&card.hw)
	cardClear()
	libps_biosB(biosCardRead, port, 0, uintptr(unsafe.Pointer(&card.sector[0])), 0)
	switch cardWait(&card.hw) {
	case cardDone:
		if card.sector[0] == 'M' && card.sector[1] == 'C' {
			return 2
		}
		return 4
	case cardTimeout:
		return 0
	}
	return 3
}

// open opens the file name with flags through the kernel and returns the
// descriptor, or -1.
//
//export open
func fileOpen(name *byte, flags int32) int32 {
	cardFile(name)
	return libps_biosB(biosOpen, uintptr(unsafe.Pointer(name)), uintptr(flags), 0, 0)
}

// read reads n bytes from fd into buf through the kernel.
//
//export read
func fileRead(fd int32, buf *byte, n int32) int32 {
	return libps_biosB(biosRead, uintptr(fd), uintptr(unsafe.Pointer(buf)), uintptr(n), 0)
}

// write writes n bytes from buf to fd through the kernel. File 1 is the
// TTY.
//
//export write
func fileWrite(fd int32, buf *byte, n int32) int32 {
	return libps_biosB(biosWrite, uintptr(fd), uintptr(unsafe.Pointer(buf)), uintptr(n), 0)
}

// lseek moves fd's position through the kernel.
//
//export lseek
func fileLseek(fd int32, off uint32, whence int32) int32 {
	return libps_biosB(biosLseek, uintptr(fd), uintptr(off), uintptr(whence), 0)
}

// close closes fd through the kernel.
//
//export close
func fileClose(fd int32) int32 {
	return libps_biosB(biosClose, uintptr(fd), 0, 0, 0)
}

// delete deletes the file name through the kernel: 1, or 0.
//
//export delete
func fileDelete(name *byte) int32 {
	cardFile(name)
	return libps_biosB(biosErase, uintptr(unsafe.Pointer(name)), 0, 0, 0)
}

// rename renames the file from to to through the kernel: 1, or 0.
//
//export rename
func fileRename(from, to *byte) int32 {
	cardFile(from)
	return libps_biosB(biosRename, uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0, 0)
}

// firstfile finds the first file matching name through the kernel,
// filling in dir, and returns dir, or nil.
//
//export firstfile
func fileFirst(name *byte, dir unsafe.Pointer) unsafe.Pointer {
	cardFile(name)
	return unsafe.Pointer(uintptr(libps_biosB(biosFirstFile, uintptr(unsafe.Pointer(name)), uintptr(dir), 0, 0)))
}

// nextfile continues firstfile's listing through the kernel.
//
//export nextfile
func fileNext(dir unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(uintptr(libps_biosB(biosNextFile, uintptr(dir), 0, 0, 0)))
}

// format formats the memory card name through the kernel: 1, or 0.
//
//export format
func fileFormat(name *byte) int32 {
	cardFile(name)
	return libps_biosB(biosFormat, uintptr(unsafe.Pointer(name)), 0, 0, 0)
}
