package yaroze

import (
	"unsafe"

	"go.chrisrx.dev/psx/input"
)

// The controllers. The library reads both every vertical blank into two
// buffers of its own, which GetPadBuf finds, and leaves decoding them to
// the program. Each buffer starts with a byte that is 0 while a controller
// is connected and 0xff while none is, a byte whose high four bits are the
// controller's type, 4 for the standard one, and whose low four are half
// the length of what follows, and then what the controller sent: for the
// standard controller two bytes of buttons, one bit each, clear while the
// button is held.
//
// PadRead assembles those into the word the SDK's samples work with, and
// the PAD constants name its bits, under the names of their pad.h. A Pad,
// the input package's Controller, reads one controller's half of it once a
// frame and tells a press from a hold.

// Buttons is what PadRead returns: the buttons held, one bit each, with the
// first controller in the low sixteen bits and the second in the high
// sixteen. It is the input package's Buttons, whose own names for the
// first controller's bits are the PAD constants' too.
type Buttons = input.Buttons

// A Pad is one controller frame by frame: the input package's Controller,
// under the name a program written against the SDK's names knows it by.
type Pad = input.Controller

// Button bits of what PadRead returns, for the first controller; the
// second's are the same bits sixteen higher. The L in a name is the left
// hand's cluster, the direction buttons, and the R the right hand's, so
// PADRup is triangle, PADRright circle, PADRdown cross and PADRleft square.
const (
	PADLup    Buttons = 1 << 12
	PADLdown  Buttons = 1 << 14
	PADLleft  Buttons = 1 << 15
	PADLright Buttons = 1 << 13
	PADRup    Buttons = 1 << 4
	PADRdown  Buttons = 1 << 6
	PADRleft  Buttons = 1 << 7
	PADRright Buttons = 1 << 5
	PADL1     Buttons = 1 << 2
	PADL2     Buttons = 1 << 0
	PADR1     Buttons = 1 << 3
	PADR2     Buttons = 1 << 1
	PADstart  Buttons = 1 << 11
	PADselect Buttons = 1 << 8

	// PADi and PADj are the stick buttons of a controller that has
	// them, which the SDK's pad.h names and nothing of the time used.
	PADi Buttons = 1 << 9
	PADj Buttons = 1 << 10
)

var padBuf0, padBuf1 *uint8

// The input package reads the same buffers, which it asks for the first
// time a program reads a controller.
func init() { input.SetSource(padBuffers) }

// PadInit finds the library's controller buffers, once. The library
// refreshes them every vertical blank from then on. PadRead calls it if it
// has not been.
func PadInit() {
	GetPadBuf(&padBuf0, &padBuf1)
}

// padBuffers returns the library's two controller buffers, for the input
// package.
func padBuffers() (port0, port1 *uint8) {
	if padBuf0 == nil || padBuf1 == nil {
		PadInit()
	}
	return padBuf0, padBuf1
}

// PadRead returns the buttons held on the controllers, one bit each: the
// first controller in the low sixteen bits and the second in the high
// sixteen, as the SDK's samples read them, with the PAD constants naming
// the bits. A controller that is not connected reports no buttons.
func PadRead() Buttons {
	if padBuf0 == nil || padBuf1 == nil {
		PadInit()
	}
	return Buttons(^(buttons(padBuf0) | buttons(padBuf1)<<16))
}

// buttons returns the two button bytes of a controller buffer as the low
// sixteen bits of a word, active low as the hardware reports them, with
// every bit set for a buffer that reports no controller.
//
//go:inline
func buttons(buf *uint8) uint32 {
	if *buf != 0 {
		return 0xffff
	}
	// The byte after the two the library writes about the controller is
	// the buttons' high byte in the word the samples assemble, and the
	// one after that its low byte.
	hi := *(*uint8)(unsafe.Add(unsafe.Pointer(buf), 2))
	lo := *(*uint8)(unsafe.Add(unsafe.Pointer(buf), 3))
	return uint32(lo) | uint32(hi)<<8
}
