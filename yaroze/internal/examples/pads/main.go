// Every kind of controller: what is plugged into each port, what it is
// sending, and a cursor any of them can move.
//
// examples/pads on the yaroze package's Go API: each port is a input.Controller,
// whose Update reads what is plugged in, its kind, its buttons and what else
// it sends, a mouse's movement and a GunCon's aim among them.
//
// # Controls
//
// Whatever is plugged in moves the cursor: the directions, a stick, a
// mouse, a neGcon's twist, or a GunCon's aim. Every button held is listed.
package main

import (
	"go.chrisrx.dev/psx/input"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	screen = yaroze.Screen{
		Background: yaroze.Color{R: 20, G: 30, B: 50},
		Depth:      1,
	}

	// cursorX and cursorY are where the cursor is, in the screen's
	// coordinates from the top left, kept apart for a test to read by
	// name.
	cursorX, cursorY int

	ports = [2]input.Controller{{Port: 0}, {Port: 1}}

	// The cursor: a cross of two bars and a box at its middle.
	across = yaroze.Box{
		W:     17,
		H:     1,
		Color: yaroze.Color{R: 255, G: 220, B: 80},
	}
	down = yaroze.Box{
		W:     1,
		H:     17,
		Color: yaroze.Color{R: 255, G: 220, B: 80},
	}
	middle = yaroze.Box{
		W:     5,
		H:     5,
		Color: yaroze.Color{R: 255, G: 80, B: 40},
	}

	text yaroze.Text
)

// The names of the buttons each kind sends, in the order they are listed.
type buttonName struct {
	b    input.Buttons
	name string
}

var padButtons = []buttonName{
	{input.Up, "UP"}, {input.Down, "DOWN"}, {input.Left, "LEFT"}, {input.Right, "RIGHT"},
	{input.Triangle, "TRIANGLE"}, {input.Circle, "CIRCLE"}, {input.Cross, "CROSS"}, {input.Square, "SQUARE"},
	{input.L1, "L1"}, {input.L2, "L2"}, {input.R1, "R1"}, {input.R2, "R2"},
	{input.L3, "L3"}, {input.R3, "R3"}, {input.Start, "START"}, {input.Select, "SELECT"},
}

var mouseButtons = []buttonName{{input.MouseLeft, "LEFT"}, {input.MouseRight, "RIGHT"}}

var negconButtons = []buttonName{
	{input.Up, "UP"}, {input.Down, "DOWN"}, {input.Left, "LEFT"}, {input.Right, "RIGHT"},
	{input.NeGconA, "A"}, {input.NeGconB, "B"}, {input.NeGconR, "R"}, {input.NeGconStart, "START"},
}

var gunconButtons = []buttonName{{input.GunConTrigger, "TRIGGER"}, {input.GunConA, "A"}, {input.GunConB, "B"}}

func main() {
	screen.Open()
	text = screen.Font(8, 8, 304, 224, 512)
	cursorX, cursorY = 160, 180

	for frame := range screen.Frames() {
		for i := range ports {
			ports[i].Update()
		}
		for i := range ports {
			describe(i, &ports[i])
			move(&ports[i])
		}

		cursorX = max(8, min(cursorX, 311))
		cursorY = max(8, min(cursorY, 231))
		x, y := cursorX, cursorY
		across.MoveTo(x-8, y)
		down.MoveTo(x, y-8)
		middle.MoveTo(x-2, y-2)
		middle.Draw(frame, 0)
		across.Draw(frame, 0)
		down.Draw(frame, 0)
	}
}

// describe writes what the controller in a port is and is sending.
func describe(port int, c *input.Controller) {
	text.Print("PORT ").Int(port + 1).Print("  ")
	if !c.Connected {
		text.Print("~c900NOTHING CONNECTED~c999\n\n\n\n")
		return
	}
	var names []buttonName
	switch c.Kind {
	case input.ControllerStandard:
		text.Print("~c090STANDARD CONTROLLER")
		names = padButtons
	case input.ControllerAnalog:
		text.Print("~c090ANALOG CONTROLLER")
		names = padButtons
	case input.ControllerAnalogJoystick:
		text.Print("~c090ANALOG JOYSTICK")
		names = padButtons
	case input.ControllerMouse:
		text.Print("~c090MOUSE")
		names = mouseButtons
	case input.ControllerNeGcon:
		text.Print("~c090NEGCON")
		names = negconButtons
	case input.ControllerGunCon:
		text.Print("~c090GUNCON")
		names = gunconButtons
	default:
		text.Print("~c990KIND ").Int(int(c.Kind))
	}
	text.Print("~c999\n HELD:")
	held := false
	for _, n := range names {
		if c.Held(n.b) {
			text.Print(" ").Print(n.name)
			held = true
		}
	}
	if !held {
		text.Print(" NOTHING")
	}
	text.Print("\n ")
	switch c.Kind {
	case input.ControllerAnalog, input.ControllerAnalogJoystick:
		text.Print("LEFT ").Int(c.Axes[2]).Print(",").Int(c.Axes[3])
		text.Print("  RIGHT ").Int(c.Axes[0]).Print(",").Int(c.Axes[1])
	case input.ControllerMouse:
		text.Print("MOVED ").Int(c.DX).Print(",").Int(c.DY)
	case input.ControllerNeGcon:
		text.Print("TWIST ").Int(c.Axes[0]).Print(" I ").Int(c.Axes[1])
		text.Print(" II ").Int(c.Axes[2]).Print(" L ").Int(c.Axes[3])
	case input.ControllerGunCon:
		text.Print("AIM ").Int(c.AimX).Print(",").Int(c.AimY)
	}
	text.Print("\n\n")
}

// The GunCon's units across a 320 by 240 picture: dots from the horizontal
// sync and lines from the vertical, about where an NTSC picture starts and
// ends. A game would have the player aim at targets to calibrate them.
const (
	gunLeft, gunRight = 77, 461
	gunTop, gunBottom = 25, 264
)

// move moves the cursor by what the controller is sending.
func move(c *input.Controller) {
	if !c.Connected {
		return
	}
	switch c.Kind {
	case input.ControllerStandard, input.ControllerAnalog, input.ControllerAnalogJoystick:
		dx, dy := c.Dir()
		cursorX += dx * 2
		cursorY += dy * 2
		if c.Kind != input.ControllerStandard {
			x, y := c.Stick(0, 24)
			cursorX += x / 32
			cursorY += y / 32
		}
	case input.ControllerMouse:
		cursorX += c.DX
		cursorY += c.DY
	case input.ControllerNeGcon:
		if t := c.Axes[0] - 128; t > 8 || t < -8 {
			cursorX += t / 16
		}
		cursorY += (c.Axes[2] - c.Axes[1]) / 64
	case input.ControllerGunCon:
		if c.AimX < 10 {
			return // pointing off the screen
		}
		cursorX = (c.AimX - gunLeft) * 320 / (gunRight - gunLeft)
		cursorY = (c.AimY - gunTop) * 240 / (gunBottom - gunTop)
	}
}
