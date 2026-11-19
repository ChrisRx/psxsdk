package input

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestButtons(t *testing.T) {
	held := Start | Select | Cross
	assert.Equal(t, true, held.Has(Start|Select))
	assert.Equal(t, false, held.Has(Start|Triangle))
	assert.Equal(t, true, held.Any(Triangle|Cross))
	assert.Equal(t, false, held.Any(Up|Down))
}

// TestController checks a press from a hold: a button acts the frame it
// goes down, not while it stays down, and the directions give an axis.
func TestController(t *testing.T) {
	var c Controller
	c.update(Start | Right)
	assert.Equal(t, true, c.Pressed(Start))
	assert.Equal(t, true, c.Held(Start))
	c.update(Start | Right | Down)
	assert.Equal(t, false, c.Pressed(Start), "still held")
	assert.Equal(t, true, c.Pressed(Down))
	assert.Equal(t, 2, c.Duration(Start))
	dx, dy := c.Dir()
	assert.Equal(t, [2]int{1, 1}, [2]int{dx, dy})
	c.update(Left | Right)
	assert.Equal(t, true, c.Released(Start))
	assert.Equal(t, 0, c.Duration(Start))
	dx, dy = c.Dir()
	assert.Equal(t, [2]int{0, 0}, [2]int{dx, dy}, "both ways cancel")
}

// TestStick checks the sticks' dead zone and that a controller without
// sticks reports none.
func TestStick(t *testing.T) {
	c := Controller{
		Connected: true,
		Kind:      ControllerAnalog,
		Axes:      [4]int{0xff, 0x80, 0x10, 0x85},
	}
	x, y := c.Stick(0, 24)
	assert.Equal(t, [2]int{-112, 0}, [2]int{x, y}, "the left stick, its small push in the dead zone")
	x, y = c.Stick(1, 24)
	assert.Equal(t, [2]int{127, 0}, [2]int{x, y}, "the right stick hard right")
	c.Kind = ControllerStandard
	x, y = c.Stick(0, 0)
	assert.Equal(t, [2]int{0, 0}, [2]int{x, y}, "no sticks on a standard controller")
	assert.Equal(t, 0, deadZone(0x80, 0), "the center is 0")
}

// TestRead checks the buffers decoded: a standard controller in port 0
// holding Start and Cross, a mouse in port 1 moved left and down, and a
// port with nothing connected.
func TestRead(t *testing.T) {
	defer SetSource(nil)
	assert.Panic(t, "input: no controller buffers; the yaroze package provides them", func() { Read(0) })

	port0 := [34]uint8{0, 0x41, ^uint8(Start >> 8), ^uint8(Cross)}
	port1 := [34]uint8{0, 0x12, 0xff, ^uint8(MouseLeft), 0xfe, 0x03}
	SetSource(func() (*uint8, *uint8) { return &port0[0], &port1[0] })
	assert.Equal(t, Start|Cross, Read(0))

	mouse := Controller{Port: 1}
	assert.Equal(t, MouseLeft, mouse.Update())
	assert.Equal(t, ControllerMouse, mouse.Kind)
	assert.Equal(t, [2]int{-2, 3}, [2]int{mouse.DX, mouse.DY})

	port1[0] = 0xff
	assert.Equal(t, Buttons(0), mouse.Update())
	assert.Equal(t, false, mouse.Connected)
}
