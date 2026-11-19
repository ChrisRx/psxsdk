//go:build yaroze

// Command revctrl is a probe for spec 37: what SsUtSetReverbType does to SPUCNT's reverb
// bit, with the reverb on and off, for each type, a negative type and one out of range.
// Each case prints the return, what SsUtGetReverbType gives and SPUCNT after, and the
// bus trace (PROBE_BUS=START) shows the writes between. The depth goes in between cases,
// so that writing it to 0 shows. The first lines show whether the type clears its work
// area in sound memory.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	spuCnt = (*uint16)(unsafe.Pointer(uintptr(0x1f801daa)))
	depthL = (*uint16)(unsafe.Pointer(uintptr(0x1f801d84)))
)

func ctrl() uint16 { return volatile.LoadUint16(spuCnt) & 0xffbf }

func set(what string, t int16) {
	yaroze.SsUtSetReverbDepth(40, 40)
	before := ctrl()
	r := yaroze.SsUtSetReverbType(t)
	println(what, t, "=", r, "kept", yaroze.SsUtGetReverbType(), "cnt", before, "->", ctrl(), "depth", volatile.LoadUint16(depthL))
}

// The work area's two ends, by DMA, as a check that a negative type clears it.
var (
	spuAt   = (*uint16)(unsafe.Pointer(uintptr(0x1f801da6)))
	dmaAddr = (*uint32)(unsafe.Pointer(uintptr(0x1f8010c0)))
	dmaSize = (*uint32)(unsafe.Pointer(uintptr(0x1f8010c4)))
	dmaCtrl = (*uint32)(unsafe.Pointer(uintptr(0x1f8010c8)))
	buf     [256]uint32
)

func xfer(addr uint32, mode uint16, ctrl uint32) {
	volatile.StoreUint16(spuAt, uint16(addr>>3))
	c := volatile.LoadUint16(spuCnt) &^ 0x30
	volatile.StoreUint16(spuCnt, c|mode)
	volatile.StoreUint32(dmaAddr, uint32(uintptr(unsafe.Pointer(&buf[0])))&0xffffff)
	volatile.StoreUint32(dmaSize, 16<<16|16)
	volatile.StoreUint32(dmaCtrl, ctrl)
	for volatile.LoadUint32(dmaCtrl)&(1<<24) != 0 {
	}
	volatile.StoreUint16(spuCnt, c)
}

func mark(addr uint32) {
	for i := range buf {
		volatile.StoreUint32(&buf[i], 0x12345678)
	}
	xfer(addr, 0x20, 0x01000201)
}

func marks(addr uint32) int {
	xfer(addr, 0x30, 0x01000200)
	n := 0
	for i := range buf {
		if volatile.LoadUint32(&buf[i]) != 0 {
			n++
		}
	}
	return n
}

// area marks a kilobyte just below the work area and three in it, sets the type, and
// prints how many words of 256 are still nonzero in each.
func area(what string, t int16, base uint32) {
	yaroze.SsUtSetReverbType(0)
	for _, a := range [...]uint32{base - 0x400, base, base + 0x400, 0x7fc00} {
		mark(a)
	}
	yaroze.SsUtSetReverbType(t)
	println(what, t, "below", marks(base-0x400), "first", marks(base), "next", marks(base+0x400), "top", marks(0x7fc00))
}

func main() {
	yaroze.ResetGraph(0)
	area("area", 1, 0xfb28<<3)
	area("area", -1, 0xfb28<<3)
	area("area", 3, 0xf6f8<<3)
	area("area", -3, 0xf6f8<<3)
	area("area", -9, 0xf880<<3)
	yaroze.SsUtSetReverbType(1)
	println("START")
	types := [...]int16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, -1, -3, -9, -10}
	for _, t := range types {
		yaroze.SsUtReverbOff()
		set("off", t)
	}
	for _, t := range types {
		yaroze.SsUtReverbOn()
		set("on", t)
	}
	// Twice in a row, the bit having been put back or not.
	yaroze.SsUtReverbOn()
	set("on then", 2)
	set("again", 2)
	set("to zero", 0)
	set("from zero", 3)
	println("END")
	for {
	}
}
