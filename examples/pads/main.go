// Every kind of controller: what is plugged into each port, what it is
// sending, and a cursor any of them can move.
//
// It is the SCEE tutorial's chapter 1 in one program
// (docs/manuals/tutorial-peripherals.md): the peripherals tester of its first
// example, which names what is in each port and lists the buttons held, and
// the mouse, analog and GunCon examples after it, which move something with
// each. The tutorial decoded the library's controller buffers with a header
// of its own, cntrl.h; yaroze.ReadController does the same here, and the
// rest is the tester's screen.
//
// The standard controller and the analog controller move the cursor with
// the direction buttons, and an analog controller or joystick with its left
// stick too, past a dead zone, since a stick at rest is near its centre
// rather than on it. A mouse moves it by what it reports moving each frame.
// A neGcon steers it with its twist, and I and II move it up and down. A
// GunCon puts it where the gun points, scaled from the gun's units, the
// GPU's dot clock across and lines down, to the screen's.
//
// Differences from the tutorial: the tester's nine font windows are one,
// written a line at a time with a Buffer; the four examples are one
// program; and the analog example's calibration screen is left out, since
// Sony's checklist (docs/manuals/requirements.md) forbids one for the
// DUAL SHOCK, whose sticks calibrate themselves.
//
// # Controls
//
// Whatever is plugged in moves the cursor: the directions, a stick, a
// mouse, a neGcon's twist, or a GunCon's aim. Every button held is listed.
package main

import "go.chrisrx.dev/psx/yaroze"

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 20, G: 30, B: 50}, Depth: 1}

	// cursorX and cursorY are where the cursor is, in the screen's
	// coordinates from the top left, kept apart for a test to read by name.
	cursorX, cursorY int32

	ports [2]yaroze.Controller

	// The cursor: a cross of two bars and a box at its middle.
	across = yaroze.GsBOXF{W: 17, H: 1, R: 255, G: 220, B: 80}
	down   = yaroze.GsBOXF{W: 1, H: 17, R: 255, G: 220, B: 80}
	middle = yaroze.GsBOXF{W: 5, H: 5, R: 255, G: 80, B: 40}

	text    yaroze.Buffer
	textBuf [512]byte
)

// The names of the buttons each kind sends, in the order they are listed.
type buttonName struct {
	b    yaroze.Buttons
	name string
}

var padButtons = []buttonName{
	{yaroze.PADLup, "UP"}, {yaroze.PADLdown, "DOWN"}, {yaroze.PADLleft, "LEFT"}, {yaroze.PADLright, "RIGHT"},
	{yaroze.PADRup, "TRIANGLE"}, {yaroze.PADRright, "CIRCLE"}, {yaroze.PADRdown, "CROSS"}, {yaroze.PADRleft, "SQUARE"},
	{yaroze.PADL1, "L1"}, {yaroze.PADL2, "L2"}, {yaroze.PADR1, "R1"}, {yaroze.PADR2, "R2"},
	{yaroze.PADi, "L3"}, {yaroze.PADj, "R3"}, {yaroze.PADstart, "START"}, {yaroze.PADselect, "SELECT"},
}

var mouseButtons = []buttonName{{yaroze.MouseLeft, "LEFT"}, {yaroze.MouseRight, "RIGHT"}}

var negconButtons = []buttonName{
	{yaroze.PADLup, "UP"}, {yaroze.PADLdown, "DOWN"}, {yaroze.PADLleft, "LEFT"}, {yaroze.PADLright, "RIGHT"},
	{yaroze.NeGconA, "A"}, {yaroze.NeGconB, "B"}, {yaroze.NeGconR, "R"}, {yaroze.NeGconStart, "START"},
}

var gunconButtons = []buttonName{{yaroze.GunConTrigger, "TRIGGER"}, {yaroze.GunConA, "A"}, {yaroze.GunConB, "B"}}

func main() {
	screen.Open()
	screen.Font(8, 8, 304, 224, 512)
	text.Init(textBuf[:])
	cursorX, cursorY = 160, 180

	for ot := range screen.Frames() {
		for i := range ports {
			ports[i] = yaroze.ReadController(i)
		}
		text.Reset()
		for i := range ports {
			describe(i, &ports[i])
			move(&ports[i])
		}
		text.Print()

		cursorX = max(8, min(cursorX, 311))
		cursorY = max(8, min(cursorY, 231))
		x, y := int16(cursorX), int16(cursorY)
		across.X, across.Y = x-8, y
		down.X, down.Y = x, y-8
		middle.X, middle.Y = x-2, y-2
		middle.Sort(ot, 0)
		across.Sort(ot, 0)
		down.Sort(ot, 0)
	}
}

// describe writes what the controller in a port is and is sending.
func describe(port int, c *yaroze.Controller) {
	text.WriteString("PORT ").WriteInt(port + 1).WriteString("  ")
	if !c.Connected {
		text.WriteString("~c900NOTHING CONNECTED~c999\n\n\n\n")
		return
	}
	var names []buttonName
	switch c.Kind {
	case yaroze.KindStandard:
		text.WriteString("~c090STANDARD CONTROLLER")
		names = padButtons
	case yaroze.KindAnalog:
		text.WriteString("~c090ANALOG CONTROLLER")
		names = padButtons
	case yaroze.KindAnalogJoystick:
		text.WriteString("~c090ANALOG JOYSTICK")
		names = padButtons
	case yaroze.KindMouse:
		text.WriteString("~c090MOUSE")
		names = mouseButtons
	case yaroze.KindNeGcon:
		text.WriteString("~c090NEGCON")
		names = negconButtons
	case yaroze.KindGunCon:
		text.WriteString("~c090GUNCON")
		names = gunconButtons
	default:
		text.WriteString("~c990KIND ").WriteInt(c.Kind)
	}
	text.WriteString("~c999\n HELD:")
	held := false
	for _, n := range names {
		if c.Buttons&n.b != 0 {
			text.WriteString(" ").WriteString(n.name)
			held = true
		}
	}
	if !held {
		text.WriteString(" NOTHING")
	}
	text.WriteString("\n ")
	switch c.Kind {
	case yaroze.KindAnalog, yaroze.KindAnalogJoystick:
		text.WriteString("LEFT ").WriteInt(c.Axes[2]).WriteString(",").WriteInt(c.Axes[3])
		text.WriteString("  RIGHT ").WriteInt(c.Axes[0]).WriteString(",").WriteInt(c.Axes[1])
	case yaroze.KindMouse:
		text.WriteString("MOVED ").WriteInt(c.DX).WriteString(",").WriteInt(c.DY)
	case yaroze.KindNeGcon:
		text.WriteString("TWIST ").WriteInt(c.Axes[0]).WriteString(" I ").WriteInt(c.Axes[1])
		text.WriteString(" II ").WriteInt(c.Axes[2]).WriteString(" L ").WriteInt(c.Axes[3])
	case yaroze.KindGunCon:
		text.WriteString("AIM ").WriteInt(c.X).WriteString(",").WriteInt(c.Y)
	}
	text.WriteString("\n\n")
}

// The GunCon's units across a 320 by 240 picture: dots from the horizontal
// sync and lines from the vertical, about where an NTSC picture starts and
// ends. A game would have the player aim at targets to calibrate them.
const (
	gunLeft, gunRight = 77, 461
	gunTop, gunBottom = 25, 264
)

// move moves the cursor by what the controller is sending.
func move(c *yaroze.Controller) {
	if !c.Connected {
		return
	}
	switch c.Kind {
	case yaroze.KindStandard, yaroze.KindAnalog, yaroze.KindAnalogJoystick:
		if c.Buttons&yaroze.PADLleft != 0 {
			cursorX -= 2
		}
		if c.Buttons&yaroze.PADLright != 0 {
			cursorX += 2
		}
		if c.Buttons&yaroze.PADLup != 0 {
			cursorY -= 2
		}
		if c.Buttons&yaroze.PADLdown != 0 {
			cursorY += 2
		}
		if c.Kind != yaroze.KindStandard {
			x, y := c.Stick(0, 24)
			cursorX += int32(x / 32)
			cursorY += int32(y / 32)
		}
	case yaroze.KindMouse:
		cursorX += int32(c.DX)
		cursorY += int32(c.DY)
	case yaroze.KindNeGcon:
		if t := int32(c.Axes[0]) - 128; t > 8 || t < -8 {
			cursorX += t / 16
		}
		cursorY += (int32(c.Axes[2]) - int32(c.Axes[1])) / 64
	case yaroze.KindGunCon:
		if c.X < 10 {
			return // pointing off the screen
		}
		cursorX = (int32(c.X) - gunLeft) * 320 / (gunRight - gunLeft)
		cursorY = (int32(c.Y) - gunTop) * 240 / (gunBottom - gunTop)
	}
}
