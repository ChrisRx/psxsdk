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
// reads one controller's half of it once a frame and tells a press from a
// hold.

// Buttons is what PadRead returns: the buttons held, one bit each, with the
// first controller in the low sixteen bits and the second in the high
// sixteen.
type Buttons uint32

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

// PadInit finds the library's controller buffers, once. The library
// refreshes them every vertical blank from then on. PadRead calls it if it
// has not been.
func PadInit() {
	GetPadBuf(&padBuf0, &padBuf1)
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

// A Pad is a controller as a frame sees it: what is held, and what has
// changed since the last frame, so that a button that is to act once per
// press acts once rather than once a frame while it is down. Port is which
// controller, 0 for the first, which the zero value reads, and 1 for the
// second; either way the PAD constants name its buttons. Update reads the
// controller, once a frame, and the rest ask about the reading.
//
//	pad.Update()
//	if pad.Pressed(yaroze.PADstart) {
//		paused = !paused
//	}
//	dx, dy := pad.Dir()
//	x += dx * speed
type Pad struct {
	Port       int
	held, last Buttons

	// frames counts, for each button, the frames it has been held.
	frames [16]uint16
}

// Update reads the controller and returns the buttons held.
func (p *Pad) Update() Buttons {
	p.last = p.held
	p.held = PadRead() >> (16 * p.Port) & 0xffff
	for i := range p.frames {
		if p.held&(1<<i) == 0 {
			p.frames[i] = 0
		} else if p.frames[i] < 0xffff {
			p.frames[i]++
		}
	}
	return p.held
}

// Duration returns how many frames the button b has been held, 1 on the
// frame it went down and 0 while it is up, for a button that repeats while
// it is held, as a menu's do. For several buttons in b it is the longest.
func (p *Pad) Duration(b Buttons) int {
	n := 0
	for i := range p.frames {
		if b&(1<<i) != 0 {
			n = max(n, int(p.frames[i]))
		}
	}
	return n
}

// Connected reports whether a controller is plugged into the Pad's port.
func (p *Pad) Connected() bool {
	if padBuf0 == nil || padBuf1 == nil {
		PadInit()
	}
	buf := padBuf0
	if p.Port == 1 {
		buf = padBuf1
	}
	return *buf == 0
}

// Held reports whether any of the buttons in b is down.
func (p *Pad) Held(b Buttons) bool { return p.held&b != 0 }

// Pressed reports whether any of the buttons in b went down since the last
// Update.
func (p *Pad) Pressed(b Buttons) bool { return p.held&^p.last&b != 0 }

// Released reports whether any of the buttons in b came up since the last
// Update.
func (p *Pad) Released(b Buttons) bool { return p.last&^p.held&b != 0 }

// Dir returns the way the direction buttons point, as steps of -1, 0 or 1
// across and down, for adding to a position on the screen, where y grows
// downwards. Opposite buttons held together cancel.
func (p *Pad) Dir() (dx, dy int) {
	return axis(p.held, PADLleft, PADLright), axis(p.held, PADLup, PADLdown)
}

// axis is -1 while neg is held, 1 while pos is, and 0 for both or neither.
//
//go:inline
func axis(held, neg, pos Buttons) int {
	d := 0
	if held&neg != 0 {
		d--
	}
	if held&pos != 0 {
		d++
	}
	return d
}
