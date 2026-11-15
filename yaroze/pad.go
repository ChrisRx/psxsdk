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

// The buttons under their own names, in the same bits as the PAD
// constants, for the first controller or a Pad's.
const (
	L2       Buttons = 1 << 0
	R2       Buttons = 1 << 1
	L1       Buttons = 1 << 2
	R1       Buttons = 1 << 3
	Triangle Buttons = 1 << 4
	Circle   Buttons = 1 << 5
	Cross    Buttons = 1 << 6
	Square   Buttons = 1 << 7
	Select   Buttons = 1 << 8
	L3       Buttons = 1 << 9 // the sticks pressed, on a controller that has them
	R3       Buttons = 1 << 10
	Start    Buttons = 1 << 11
	Up       Buttons = 1 << 12
	Right    Buttons = 1 << 13
	Down     Buttons = 1 << 14
	Left     Buttons = 1 << 15

	// The other controllers' buttons, in the bits they share with the
	// standard controller's.
	MouseLeft  Buttons = R1
	MouseRight Buttons = L1

	NeGconStart Buttons = Start
	NeGconA     Buttons = Circle
	NeGconB     Buttons = Triangle
	NeGconR     Buttons = R1

	GunConTrigger Buttons = Circle
	GunConA       Buttons = Start
	GunConB       Buttons = Cross
)

// Has says whether every button in b is held.
func (held Buttons) Has(b Buttons) bool { return held&b == b }

// Any says whether any button in b is held.
func (held Buttons) Any(b Buttons) bool { return held&b != 0 }

// PadButtons returns the buttons held on the controller in port 0 or 1
// as of the last vertical blank, in the low sixteen bits. A port with
// nothing connected reports none.
func PadButtons(port int) Buttons { return padButtons(port & 1) }

// A Kind is what a controller is, which it says of itself.
type Kind uint8

const (
	Mouse          Kind = 1
	NeGcon         Kind = 2
	Digital        Kind = 4 // the standard controller
	AnalogJoystick Kind = 5
	GunCon         Kind = 6
	Analog         Kind = 7 // the analog controller in analog mode
)

// A Pad is a controller as a frame sees it: what is held, and
// what has changed since the last frame, so that a button that is to act
// once per press acts once rather than once a frame while it is down.
// Port is which controller, 0 for the first, which the zero value reads,
// and 1 for the second; either way the PAD constants, or the buttons'
// own names, name its buttons. Update reads it, once a frame, and the
// rest ask about the reading.
//
//	pad.Update()
//	if pad.Pressed(yaroze.Start) {
//		paused = !paused
//	}
//	dx, dy := pad.Dir()
type Pad struct {
	Port int

	// Connected is whether anything is plugged in, and Kind what it is;
	// the rest is zero when nothing is.
	Connected bool
	Kind      Kind

	// Axes are the four bytes a controller sends after its buttons. An
	// analog controller's or joystick's are its right stick's X and Y and
	// its left stick's X and Y, 0 to 255 and about 128 at rest, X
	// growing to the right and Y downwards. A neGcon's are its twist,
	// about 128 when straight, and how far I, II and L are pressed.
	Axes [4]int

	// DX and DY are how far a mouse moved since the last vertical blank.
	DX, DY int

	// AimX and AimY are where a GunCon points, X counted in the GPU's dot
	// clock from the horizontal sync and Y in lines, both in the gun's
	// own units, which a program calibrates against the screen. Both are
	// small, under 10, when it points off the screen.
	AimX, AimY int

	held, last Buttons

	// frames counts, for each button, the frames it has been held.
	frames [16]uint16
}

// Update reads the controller and returns the buttons held.
func (p *Pad) Update() Buttons {
	connected, kind, axes, held := padReading(p.Port & 1)
	p.Connected, p.Kind = connected, kind
	for i, a := range axes {
		p.Axes[i] = int(a)
	}
	p.DX, p.DY, p.AimX, p.AimY = 0, 0, 0, 0
	switch kind {
	case Mouse:
		p.DX, p.DY = int(int8(axes[0])), int(int8(axes[1]))
	case GunCon:
		p.AimX = int(axes[0]) | int(axes[1])<<8
		p.AimY = int(axes[2]) | int(axes[3])<<8
	}
	return p.update(held)
}

func (p *Pad) update(held Buttons) Buttons {
	p.last, p.held = p.held, held
	for i := range p.frames {
		if held&(1<<i) == 0 {
			p.frames[i] = 0
		} else if p.frames[i] < 0xffff {
			p.frames[i]++
		}
	}
	return held
}

// Buttons returns the buttons held at the last Update.
func (p *Pad) Buttons() Buttons { return p.held }

// Held says whether any button in b is held. Buttons().Has(b) says
// whether every one is.
func (p *Pad) Held(b Buttons) bool { return p.held&b != 0 }

// Pressed says whether any button in b went down since the last frame.
func (p *Pad) Pressed(b Buttons) bool { return p.held&^p.last&b != 0 }

// Released says whether any button in b came up since the last frame.
func (p *Pad) Released(b Buttons) bool { return p.last&^p.held&b != 0 }

// Duration returns how many frames the button b has been held, 1 on the
// frame it went down and 0 while it is up, for a button that repeats
// while it is held, as a menu's do. For several buttons in b it is the
// longest.
func (p *Pad) Duration(b Buttons) int {
	n := 0
	for i := range p.frames {
		if b&(1<<i) != 0 {
			n = max(n, int(p.frames[i]))
		}
	}
	return n
}

// Dir is which way the directions push: -1, 0 or 1 across and down.
func (p *Pad) Dir() (dx, dy int) {
	return axis(p.held, Left, Right), axis(p.held, Up, Down)
}

// axis is -1 while neg is held, 1 while pos is, and 0 for both or
// neither.
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

// Stick returns an analog controller's or joystick's left (0) or right
// (1) stick as -128 to 127 across and down, with a dead zone of dead
// either side of the center reported as 0, since the sticks rest near
// the center rather than on it. Any other controller's sticks are 0.
func (p *Pad) Stick(n, dead int) (x, y int) {
	if p.Kind != Analog && p.Kind != AnalogJoystick {
		return 0, 0
	}
	i := 0
	if n == 0 {
		i = 2
	}
	return deadZone(p.Axes[i], dead), deadZone(p.Axes[i+1], dead)
}

// deadZone is an axis byte as -128 to 127, 0 within dead of the center.
func deadZone(v, dead int) int {
	d := v - 128
	if d > -dead && d < dead {
		return 0
	}
	return d
}
