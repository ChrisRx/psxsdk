package yaroze

import "unsafe"

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
// the PAD constants name its bits, under the names of their pad.h. A Pad
// reads it once a frame and tells a press from a hold.

// Button bits of what PadRead returns, for the first controller; the
// second's are the same bits sixteen higher. The L in a name is the left
// hand's cluster, the direction buttons, and the R the right hand's, so
// PADRup is triangle, PADRright circle, PADRdown cross and PADRleft square.
const (
	PADLup    = 1 << 12
	PADLdown  = 1 << 14
	PADLleft  = 1 << 15
	PADLright = 1 << 13
	PADRup    = 1 << 4
	PADRdown  = 1 << 6
	PADRleft  = 1 << 7
	PADRright = 1 << 5
	PADL1     = 1 << 2
	PADL2     = 1 << 0
	PADR1     = 1 << 3
	PADR2     = 1 << 1
	PADstart  = 1 << 11
	PADselect = 1 << 8

	// PADi and PADj are the stick buttons of a controller that has
	// them, which the SDK's pad.h names and nothing of the time used.
	PADi = 1 << 9
	PADj = 1 << 10
)

var padBuf0, padBuf1 *uint8

// PadInit finds the library's controller buffers, once, before PadRead is
// called. The library refreshes them every vertical blank from then on.
func PadInit() {
	GetPadBuf(&padBuf0, &padBuf1)
}

// PadRead returns the buttons held on the controllers, one bit each: the
// first controller in the low sixteen bits and the second in the high
// sixteen, as the SDK's samples read them, with the PAD constants naming
// the bits. A controller that is not connected reports no buttons. It
// returns 0 before PadInit.
func PadRead() uint32 {
	if padBuf0 == nil || padBuf1 == nil {
		return 0
	}
	return ^(buttons(padBuf0) | buttons(padBuf1)<<16)
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

// A Pad is the controllers as a frame sees them: what is held, and what
// has changed since the last frame, so that a button that is to act once
// per press acts once rather than once a frame while it is down. Update
// reads the controllers, once a frame, and the rest ask about the reading.
//
//	pad.Update()
//	if pad.Pressed(yaroze.PADstart) {
//		paused = !paused
//	}
//	if pad.Held(yaroze.PADLleft) {
//		x--
//	}
type Pad struct {
	held, last uint32
}

// Update reads the controllers, as PadRead does, and returns the buttons
// held.
func (p *Pad) Update() uint32 {
	p.last = p.held
	p.held = PadRead()
	return p.held
}

// Held reports whether any of the buttons in b is down.
func (p *Pad) Held(b uint32) bool { return p.held&b != 0 }

// Pressed reports whether any of the buttons in b went down since the last
// Update.
func (p *Pad) Pressed(b uint32) bool { return p.held&^p.last&b != 0 }

// Released reports whether any of the buttons in b came up since the last
// Update.
func (p *Pad) Released(b uint32) bool { return p.last&^p.held&b != 0 }
