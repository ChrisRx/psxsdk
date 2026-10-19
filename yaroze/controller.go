package yaroze

import "unsafe"

// Every kind of controller, not only the standard one. The library reads
// whatever is plugged into each port every vertical blank, into the buffers
// GetPadBuf finds: a byte that is 0 while something is connected, a byte
// whose high four bits are what it is and whose low four are half the
// length of what follows, and what it sent. The standard controller sends
// its buttons; the others send their buttons and then more, which
// ReadController takes apart. The SCEE tutorial's chapter 1 decodes the
// same buffers in C (docs/manuals/tutorial-peripherals.md).

// Kinds of controller, the high four bits of a buffer's second byte.
const (
	KindMouse          = 1
	KindNeGcon         = 2
	KindStandard       = 4
	KindAnalogJoystick = 5
	KindGunCon         = 6
	KindAnalog         = 7 // the analog controller (DUAL SHOCK) in analog mode
)

// Buttons of the other controllers, as bits of a Controller's Buttons, in
// the same word as the PAD constants, which is why they share their values.
const (
	MouseLeft  Buttons = 1 << 3 // PADR1's bit
	MouseRight Buttons = 1 << 2 // PADL1's bit

	NeGconStart Buttons = PADstart
	NeGconA     Buttons = 1 << 5 // PADRright's bit
	NeGconB     Buttons = 1 << 4 // PADRup's bit
	NeGconR     Buttons = 1 << 3 // PADR1's bit

	GunConTrigger Buttons = 1 << 5  // PADRright's bit
	GunConA       Buttons = 1 << 11 // PADstart's bit
	GunConB       Buttons = 1 << 6  // PADRdown's bit
)

// A Controller is what one port's controller sent at the last vertical
// blank.
type Controller struct {
	// Connected is whether anything is plugged in. The rest is zero when
	// nothing is.
	Connected bool

	// Kind is what it is, one of the Kind constants.
	Kind uint8

	// Buttons are the buttons held, in PadRead's word, the PAD constants
	// naming the standard and analog controllers' and the Mouse, NeGcon and
	// GunCon constants the others'. The direction buttons of a neGcon are
	// the PAD constants'.
	Buttons Buttons

	// Axes are the four bytes after the buttons. An analog controller's or
	// joystick's are its right stick's X and Y and its left stick's X and Y,
	// 0 to 255 and about 128 when centred, X growing to the right and Y
	// downwards. A neGcon's are its twist, about 128 when straight, and how
	// far I, II and L are pressed.
	Axes [4]uint8

	// DX and DY are how far a mouse moved since the last vertical blank.
	DX, DY int8

	// X and Y are where a GunCon points, X counted in the GPU's dot clock
	// from the horizontal sync and Y in lines, both in the gun's own
	// units, which a program calibrates against the screen. Both are
	// small, under 10, when it points off the screen.
	X, Y uint16
}

// ReadController returns what the controller in port 0 or 1 sent at the
// last vertical blank.
func ReadController(port int) Controller {
	if padBuf0 == nil || padBuf1 == nil {
		PadInit()
	}
	buf := padBuf0
	if port == 1 {
		buf = padBuf1
	}
	at := func(i uintptr) uint8 { return *(*uint8)(unsafe.Add(unsafe.Pointer(buf), i)) }
	var c Controller
	if at(0) != 0 {
		return c
	}
	c.Connected = true
	c.Kind = at(1) >> 4
	c.Buttons = Buttons(^buttons(buf) & 0xffff)
	switch c.Kind {
	case KindMouse:
		c.DX, c.DY = int8(at(4)), int8(at(5))
	case KindNeGcon, KindAnalogJoystick, KindAnalog:
		c.Axes = [4]uint8{at(4), at(5), at(6), at(7)}
	case KindGunCon:
		c.X = uint16(at(4)) | uint16(at(5))<<8
		c.Y = uint16(at(6)) | uint16(at(7))<<8
	}
	return c
}

// Stick returns an analog controller's or joystick's left (0) or right (1)
// stick as -128 to 127 across and down, with a dead zone of dead either
// side of the centre reported as 0, as Sony's checklist asks of a game:
// the sticks rest near 128, not on it.
func (c Controller) Stick(n, dead int) (x, y int) {
	i := 0
	if n == 0 {
		i = 2
	}
	axis := func(v uint8) int {
		d := int(v) - 128
		if d > -dead && d < dead {
			return 0
		}
		return d
	}
	return axis(c.Axes[i]), axis(c.Axes[i+1])
}
