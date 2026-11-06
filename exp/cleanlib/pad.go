//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The controllers, which the kernel's driver reads every vertical blank.

// padSize is the length of a controller's receive buffer as the kernel
// takes it.
const padSize = 34

// pads is the two ports' receive buffers, word aligned, and started
// whether the driver is running.
var pads struct {
	buf     [2][(padSize + 3) / 4]uint32
	started bool
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
	words := (*[2 * (padSize + 3) / 4]uint32)(unsafe.Pointer(&pads.buf))
	for i := range words {
		volatile.StoreUint32(&words[i], 0xffffffff)
	}
	*buf1, *buf2 = (*uint8)(unsafe.Pointer(&pads.buf[0])), (*uint8)(unsafe.Pointer(&pads.buf[1]))
	pace(getPadBufPad)
}

// getPadBufPad is the rest of GetPadBuf's 834 cycles (spec 21).
const getPadBufPad = 734

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
	b0 := unsafe.Pointer(&pads.buf[0])
	b1 := unsafe.Pointer(&pads.buf[1])
	libStart()
	pads.started = true
	libps_biosB(biosInitPad, uintptr(b0), padSize, uintptr(b1), padSize)
	padReadingStart()
}
