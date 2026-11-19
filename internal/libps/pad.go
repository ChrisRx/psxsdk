//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The controllers, which the kernel's driver reads every vertical blank.

// padSize is the length of a controller's receive buffer as the kernel
// takes it.
const padSize = 34

// padBufs is the two ports' receive buffers, each padded to a word, in the target's
// assembly: its putchar reads them too.
//
//go:extern libps_pad_buf
var padBufs [2][(padSize + 3) / 4]uint32

// pads is whether the driver is running.
var pads struct {
	started bool
}

// classLow is the 16 bytes before the buffers, in the target's assembly. All that is
// known of them from outside is what classes bytes 93h to 96h have after a GsInitGraph.
//
//go:extern libps_class_low
var classLow [16]byte

// highClass is what Sony's character class table says of a byte from 80h up, a signed
// byte indexing the memory below the table. From A3h to E8h that memory is the
// controllers' two buffers, so a byte's class is whatever the kernel or a program
// last left there. 93h to 96h are a word that GsInitGraph sets, and the rest read 0 on the
// probes. As in the C library's classes, bit 0 is upper case, 1 lower, 2 a digit, 3
// space, 4 punctuation, 5 control, 6 a hexadecimal letter and 7 a blank. printf,
// putchar, toupper and tolower all go by it: a tab stop moves by what a program's
// controller holds.
func highClass(c byte) byte {
	if c < 0x93 || c > 0xe8 {
		return 0
	}
	return *(*byte)(unsafe.Add(unsafe.Pointer(&classLow), int(c)-0x93))
}

// classSetInitGraph leaves what a GsInitGraph leaves in classLow: 93h prints, 94h is
// upper and lower case, 95h lower and 96h prints, whatever the call's arguments.
func classSetInitGraph() {
	classLow[0], classLow[1], classLow[2], classLow[3] = 0x10, 0x03, 0x02, 0x80
}

// GetPadBuf stores the addresses of the receive buffers of ports 1 and 2
// in buf1 and buf2, starting the kernel's controller driver if the
// library's start-up (startup, at the first ResetGraph) has not. The
// kernel fills each every vertical blank: a byte that is 0 when a
// controller answered and FFh when none did, a byte of its type and
// length, and what it sent. Each call fills both buffers with FFh, so
// they say that nothing is connected until the kernel's next read, as
// Sony's library's does (probed against it: the first byte reads FFh
// right after any call, the driver running or not).
//
//export GetPadBuf
//go:noinline
func GetPadBuf(buf1, buf2 **uint8) {
	padStart()
	// Only a buffer's own bytes: the two that make it up to a word stay 0.
	for b := range padBufs {
		bytes := (*[padSize]byte)(unsafe.Pointer(&padBufs[b]))
		for i := range bytes {
			volatile.StoreUint8(&bytes[i], 0xff)
		}
	}
	*buf1, *buf2 = (*uint8)(unsafe.Pointer(&padBufs[0])), (*uint8)(unsafe.Pointer(&padBufs[1]))
	pace(getPadBufPad)
}

// getPadBufPad is the rest of GetPadBuf's 834 cycles (spec 21), a byte at a time.
const getPadBufPad = 638

// padStart starts the kernel's controller driver, once: from the
// library's start-up (startup), or from GetPadBuf or the card driver's
// start in a program that uses them before any ResetGraph.
func padStart() {
	if !pads.started {
		padBegin()
	}
}

// padBegin is padStart's work, apart so that a call after the start costs no
// stack.
//
//go:noinline
func padBegin() {
	b0 := unsafe.Pointer(&padBufs[0])
	b1 := unsafe.Pointer(&padBufs[1])
	libStart()
	pads.started = true
	imagePad(residentPadPad)
	libps_biosB(biosInitPad, uintptr(b0), padSize, uintptr(b1), padSize)
	padReadingStart()
}
