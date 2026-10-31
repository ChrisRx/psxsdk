//go:build yaroze

package libps

import "unsafe"

// The memory cards go through the kernel: its card driver, started as the
// resident library's startup started it, with eight events opened on it
// that TestCard waits on, four from the card hardware and four from the
// kernel's own layer; and its file system, which the file calls reach as
// BIOS calls.

// The kernel's event and card calls.
func openEvent(class, spec, mode, fn uint32) uint32 { return biosB(0x08, class, spec, mode, fn) }
func testEvent(ev uint32) uint32                    { return biosB(0x0b, ev, 0, 0, 0) }
func enableEvent(ev uint32)                         { biosB(0x0c, ev, 0, 0, 0) }
func initCard(pads uint32)                          { biosB(0x4a, pads, 0, 0, 0) }
func startCard()                                    { biosB(0x4b, 0, 0, 0, 0) }
func buInit()                                       { biosA(0x70, 0, 0, 0, 0) }
func cardInfo(port uint32) uint32                   { return biosA(0xab, port, 0, 0, 0) }
func allowNewCard()                                 { biosB(0x50, 0, 0, 0, 0) }

func readCardSector(port, sector uint32, dst *byte) uint32 {
	return biosB(0x4f, port, sector, addr(unsafe.Pointer(dst)), 0)
}

func writeCardSector(port, sector uint32, src *byte) uint32 {
	return biosB(0x4e, port, sector, addr(unsafe.Pointer(src)), 0)
}

func addr(p unsafe.Pointer) uint32 { return uint32(uintptr(p)) }

// The events: the card hardware's, class 0xf4000001, and the kernel's
// card layer's, class 0xf0000011, each for completion, an error, a
// timeout and a new card.
//
// libps 0x80063FD4 and 0x800640B8
var (
	cardHWEvents, cardSWEvents [4]uint32
	cardSpecs                  = [4]uint32{0x0004, 0x8000, 0x0100, 0x2000}
)

// cardStarted is whether the kernel's card driver has been started. The
// resident library's startup did that before the program; this library
// does it the first time the program asks for a card.
var cardStarted bool

// cardStart opens the events, starts the kernel's card driver and
// enables the events, once.
//
// libps 0x800103F8 and 0x80010528
func cardStart() {
	if cardStarted {
		return
	}
	cardStarted = true
	EnterCriticalSection()
	for i := range cardSpecs {
		cardHWEvents[i] = openEvent(0xf4000001, cardSpecs[i], 0x2000, 0)
	}
	for i := range cardSpecs {
		cardSWEvents[i] = openEvent(0xf0000011, cardSpecs[i], 0x2000, 0)
	}
	ExitCriticalSection()
	EnterCriticalSection()
	initCard(1)
	startCard()
	buInit()
	// Starting the driver has the kernel's pad handler acknowledge the
	// vertical blank again; see GetPadBuf.
	changeClearPAD(0)
	for i := range cardHWEvents {
		enableEvent(cardHWEvents[i])
	}
	for i := range cardSWEvents {
		enableEvent(cardSWEvents[i])
	}
	ExitCriticalSection()
}

// cardWait waits for one of four events and says which: 0 for
// completion, 1 for an error, 2 for a timeout, 3 for a new card.
//
// libps 0x80010240 and 0x8001031C
func cardWait(events *[4]uint32) int32 {
	for {
		for i := range events {
			if testEvent(events[i]) == 1 {
				return int32(i)
			}
		}
	}
}

// cardClear takes whatever the kernel's events hold.
//
// libps 0x800103A0
func cardClear() {
	for i := range cardSWEvents {
		testEvent(cardSWEvents[i])
	}
}

// cardSector is where TestCard reads a card's first sector.
var cardSector [128]byte

// TestCard asks the kernel about the card in slot 0 or 1: 1 for a card
// that was there before, 0 for none, 2 for a new card with a file
// system, 4 for a new card without one, 3 for an error, and -1 for a
// slot that is neither.
//
// libps 0x8001012C, 276 bytes
//
//export TestCard
func TestCard(slot int32) int32 {
	cardStart()
	var port uint32
	switch slot {
	case 1:
		port = 0x10
	case 0:
		port = 0
	default:
		return -1
	}
	for cardInfo(port) == 0 {
	}
	switch cardWait(&cardHWEvents) {
	case 0:
		return 1
	case 1:
		return 3
	case 2:
		return 0
	case 3:
		// A new card: the kernel is told to take it, and a write to its
		// last sector, from nowhere, as the original did, makes it known.
		cardClear()
		allowNewCard()
		writeCardSector(port, 0x3f, nil)
		cardWait(&cardSWEvents)
	}
	cardClear()
	readCardSector(port, 0, &cardSector[0])
	switch cardWait(&cardSWEvents) {
	case 1:
		return 3
	case 2:
		return 0
	}
	if cardSector[0] == 'M' && cardSector[1] == 'C' {
		return 2
	}
	return 4
}

// The kernel's file system, on the cards, through its BIOS calls. Each
// starts the card driver first, as the resident library's startup had.

// libps 0x80042F58, 16 bytes
//
//export open
func fileOpen(name *byte, mode int32) int32 {
	cardStart()
	return int32(biosB(0x32, addr(unsafe.Pointer(name)), uint32(mode), 0, 0))
}

// libps 0x80042F78, 16 bytes
//
//export read
func fileRead(fd int32, buf *byte, n int32) int32 {
	return int32(biosB(0x34, uint32(fd), addr(unsafe.Pointer(buf)), uint32(n), 0))
}

// libps 0x80042F88, 16 bytes
//
//export write
func fileWriteFd(fd int32, buf *byte, n int32) int32 {
	return int32(biosB(0x35, uint32(fd), addr(unsafe.Pointer(buf)), uint32(n), 0))
}

// libps 0x80042F98, 16 bytes
//
//export close
func fileClose(fd int32) int32 { return int32(biosB(0x36, uint32(fd), 0, 0, 0)) }

// libps 0x80042FA8, 16 bytes
//
//export format
func fileFormat(name *byte) int32 {
	cardStart()
	return int32(biosB(0x41, addr(unsafe.Pointer(name)), 0, 0, 0))
}

// libps 0x80042FB8, 16 bytes
//
//export firstfile
func fileFirst(name *byte, dir *DIRENTRY) *DIRENTRY {
	cardStart()
	return (*DIRENTRY)(unsafe.Pointer(uintptr(biosB(0x42, addr(unsafe.Pointer(name)), addr(unsafe.Pointer(dir)), 0, 0))))
}

// libps 0x80042FC8, 16 bytes
//
//export nextfile
func fileNext(dir *DIRENTRY) *DIRENTRY {
	return (*DIRENTRY)(unsafe.Pointer(uintptr(biosB(0x43, addr(unsafe.Pointer(dir)), 0, 0, 0))))
}

// libps 0x80042FE8, 8 bytes
//
//export delete
func fileDelete(name *byte) int32 {
	cardStart()
	return int32(biosB(0x45, addr(unsafe.Pointer(name)), 0, 0, 0))
}

// The file calls for other packages, which cannot reach the unexported
// names the C library's names were given to.
func FileOpen(name *byte, mode int32) int32         { return fileOpen(name, mode) }
func FileRead(fd int32, buf *byte, n int32) int32   { return fileRead(fd, buf, n) }
func FileWrite(fd int32, buf *byte, n int32) int32  { return fileWriteFd(fd, buf, n) }
func FileClose(fd int32) int32                      { return fileClose(fd) }
func FileFormat(name *byte) int32                   { return fileFormat(name) }
func FileFirst(name *byte, dir *DIRENTRY) *DIRENTRY { return fileFirst(name, dir) }
func FileNext(dir *DIRENTRY) *DIRENTRY              { return fileNext(dir) }
func FileDelete(name *byte) int32                   { return fileDelete(name) }
