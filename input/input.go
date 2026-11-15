// Package input reads the controllers: what each port's controller holds,
// what changed since the last frame, and the sticks, mouse and gun of the
// controllers that have them.
//
// The library reads both ports every vertical blank into two buffers of its
// own and leaves decoding them to the program. Each buffer starts with a
// byte that is 0 while a controller is connected and 0xff while none is, a
// byte whose high four bits are the controller's kind and whose low four
// are half the length of what follows, and then what the controller sent:
// two bytes of buttons, one bit each, clear while the button is held, and
// for the controllers with more, four bytes after them.
//
// The yaroze package hands this package those buffers when it is imported,
// which every program does, so a program only reads:
//
//	var pad = input.Controller{Port: 0}
//
//	for frame := range screen.Frames() {
//		pad.Update()
//		if pad.Pressed(input.Start) {
//			paused = !paused
//		}
//	}
package input

import "unsafe"

// Buttons is a set of buttons, one bit each: what a controller holds. The
// bits are the ones the SDK's samples read, and yaroze's PAD constants name
// the same bits, with the second controller's sixteen higher in what
// PadRead returns.
type Buttons uint32

// The buttons under their own names.
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

// A ControllerKind is what a controller is, which it says of itself.
type ControllerKind uint8

const (
	ControllerMouse          ControllerKind = 1
	ControllerNeGcon         ControllerKind = 2
	ControllerStandard       ControllerKind = 4 // the digital controller
	ControllerAnalogJoystick ControllerKind = 5
	ControllerGunCon         ControllerKind = 6
	ControllerAnalog         ControllerKind = 7 // the analog controller in analog mode
)

// source returns the library's two buffers, and buffers keeps them once
// it has.
var (
	source  func() (port0, port1 *uint8)
	buffers [2]*uint8
)

// SetSource tells the package where the library keeps the controllers'
// buffers: source returns the two, and is called the first time a
// controller is read. The yaroze package sets it as it starts; a program
// does not.
func SetSource(f func() (port0, port1 *uint8)) {
	source = f
	buffers = [2]*uint8{}
}

// buffer returns the library's buffer for port 0 or 1, eight bytes of it:
// the two about the controller, the buttons and the four after them.
func buffer(port int) []uint8 {
	if buffers[0] == nil {
		if source == nil {
			panic("input: no controller buffers; the yaroze package provides them")
		}
		buffers[0], buffers[1] = source()
	}
	return unsafe.Slice(buffers[port&1], 8)
}

// Read returns the buttons held on the controller in port 0 or 1 as of
// the last vertical blank. A port with nothing connected reports none.
// A Controller reads the same and also tells a press from a hold.
func Read(port int) Buttons {
	_, _, _, held := reading(port)
	return held
}

// reading decodes a port's buffer: whether a controller is there, what it
// is, the four bytes after its buttons, and the buttons held.
func reading(port int) (connected bool, kind ControllerKind, axes [4]uint8, held Buttons) {
	buf := buffer(port)
	if buf[0] != 0 {
		return false, 0, axes, 0
	}
	copy(axes[:], buf[4:8])
	return true, ControllerKind(buf[1] >> 4), axes, Buttons(^(uint16(buf[3]) | uint16(buf[2])<<8))
}

// A Controller is a controller as a frame sees it: what is held, and what
// has changed since the last frame, so that a button that is to act once
// per press acts once rather than once a frame while it is down. Port is
// which controller, 0 for the first, which the zero value reads, and 1 for
// the second. Update reads it, once a frame, and the rest ask about the
// reading.
//
//	pad.Update()
//	if pad.Pressed(input.Start) {
//		paused = !paused
//	}
//	dx, dy := pad.Dir()
type Controller struct {
	Port int

	// Connected is whether anything is plugged in, and Kind what it is;
	// the rest is zero when nothing is.
	Connected bool
	Kind      ControllerKind

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
func (c *Controller) Update() Buttons {
	connected, kind, axes, held := reading(c.Port)
	c.Connected, c.Kind = connected, kind
	for i, a := range axes {
		c.Axes[i] = int(a)
	}
	c.DX, c.DY, c.AimX, c.AimY = 0, 0, 0, 0
	switch kind {
	case ControllerMouse:
		c.DX, c.DY = int(int8(axes[0])), int(int8(axes[1]))
	case ControllerGunCon:
		c.AimX = int(axes[0]) | int(axes[1])<<8
		c.AimY = int(axes[2]) | int(axes[3])<<8
	}
	return c.update(held)
}

func (c *Controller) update(held Buttons) Buttons {
	c.last, c.held = c.held, held
	for i := range c.frames {
		if held&(1<<i) == 0 {
			c.frames[i] = 0
		} else if c.frames[i] < 0xffff {
			c.frames[i]++
		}
	}
	return held
}

// Buttons returns the buttons held at the last Update.
func (c *Controller) Buttons() Buttons { return c.held }

// Held says whether any button in b is held. Buttons().Has(b) says
// whether every one is.
func (c *Controller) Held(b Buttons) bool { return c.held&b != 0 }

// Pressed says whether any button in b went down since the last frame.
func (c *Controller) Pressed(b Buttons) bool { return c.held&^c.last&b != 0 }

// Released says whether any button in b came up since the last frame.
func (c *Controller) Released(b Buttons) bool { return c.last&^c.held&b != 0 }

// Duration returns how many frames the button b has been held, 1 on the
// frame it went down and 0 while it is up, for a button that repeats
// while it is held, as a menu's do. For several buttons in b it is the
// longest.
func (c *Controller) Duration(b Buttons) int {
	n := 0
	for i := range c.frames {
		if b&(1<<i) != 0 {
			n = max(n, int(c.frames[i]))
		}
	}
	return n
}

// Dir is which way the directions push: -1, 0 or 1 across and down.
func (c *Controller) Dir() (dx, dy int) {
	return axis(c.held, Left, Right), axis(c.held, Up, Down)
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
func (c *Controller) Stick(n, dead int) (x, y int) {
	if c.Kind != ControllerAnalog && c.Kind != ControllerAnalogJoystick {
		return 0, 0
	}
	i := 0
	if n == 0 {
		i = 2
	}
	return deadZone(c.Axes[i], dead), deadZone(c.Axes[i+1], dead)
}

// deadZone is an axis byte as -128 to 127, 0 within dead of the center.
func deadZone(v, dead int) int {
	d := v - 128
	if d > -dead && d < dead {
		return 0
	}
	return d
}
