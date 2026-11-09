package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestFieldSkip checks SetFieldSkip: with an interlaced 480-line display and
// drawing to the displayed area off, a rectangle and a fill reach only the
// lines of the field not shown, and the lines move with the field and the
// display area's start. With the switch off, drawing to the displayed area
// allowed, or a 240-line display, every line is drawn.
func TestFieldSkip(t *testing.T) {
	const (
		gp0     = 0x1f801810
		gp1     = 0x1f801814
		blue    = 0x7c00
		red     = 0x001f
		e1      = 0xe1000000
		e1Shown = 0xe1000000 | gpustatDrawDisplayed
	)
	m := newTestMachine(t)
	m.Bus.Write32(gp0, 0xe4000000|511<<10|1023) // draw anywhere
	rect := func() {
		m.Bus.Write32(gp0, 0x020000ff) // a red fill of 16 by 4 at 0,0
		m.Bus.Write32(gp0, 0)
		m.Bus.Write32(gp0, 4<<16|16)
		m.Bus.Write32(gp0, 0x60ff0000) // a blue rectangle of 4 by 4 at 0,0
		m.Bus.Write32(gp0, 0)
		m.Bus.Write32(gp0, 4<<16|4)
	}
	rows := func() (s string) {
		for y := range 4 {
			switch m.pixel(0, y) {
			case blue:
				s += "b"
			case red:
				s += "r"
			default:
				s += "."
			}
		}
		for y := range 4 {
			if m.pixel(8, y) == red {
				s += "R"
			} else {
				s += "."
			}
		}
		return s
	}
	clear := func() {
		for y := range 4 {
			for x := range 16 {
				m.gpu.vram[m.gpu.vramIndex(x, y)] = 0
			}
		}
	}
	field := func(f uint32) { m.gpu.gpustat = m.gpu.gpustat&^gpustatField | f<<13 }

	m.Bus.Write32(gp1, 0x08000027) // 640 by 480, interlaced
	m.Bus.Write32(gp0, e1)
	field(0)
	rect()
	assert.Equal(t, "bbbbRRRR", rows(), "off, every line")

	m.SetFieldSkip(true)
	clear()
	rect()
	assert.Equal(t, ".b.b.R.R", rows(), "field 0 shown: the odd lines only")
	clear()
	field(1)
	rect()
	assert.Equal(t, "b.b.R.R.", rows(), "field 1 shown: the even lines only")
	clear()
	m.Bus.Write32(gp1, 0x05000000|1<<10) // the display starting at line 1
	rect()
	assert.Equal(t, ".b.b.R.R", rows(), "the parity counts from the display's start")
	m.Bus.Write32(gp1, 0x05000000)

	clear()
	m.Bus.Write32(gp0, e1Shown)
	rect()
	assert.Equal(t, "bbbbRRRR", rows(), "drawing to the shown area allowed")

	clear()
	m.Bus.Write32(gp0, e1)
	m.Bus.Write32(gp1, 0x08000001) // 320 by 240
	rect()
	assert.Equal(t, "bbbbRRRR", rows(), "not interlaced")
}

// TestDrawTimeStops checks that under SetDrawTime, GP1(01h) ends the drawing
// at once, and that a list held for the GPU never goes once the program
// stops the channel.
func TestDrawTimeStops(t *testing.T) {
	const (
		gp0     = 0x1f801810
		gpustat = 0x1f801814
		madr    = 0x1f8010a0
		chcr    = 0x1f8010a8
	)
	m := newTestMachine(t)
	m.Bus.Write32(gp0, 0xe4000000|511<<10|1023)
	list := []uint32{0x03ffffff, 0x60ff0000, 0, 100<<16 | 100}
	for i, w := range list {
		m.Bus.Write32(0x100+uint32(i)*4, w)
	}
	at := func(c uint64) {
		m.CPU.TotalCycles = c
		m.dma.update()
	}
	ready := func() bool { return m.Bus.Read32(gpustat)&gpustatReadyCmd != 0 }
	busy := func() bool { return m.Bus.Read32(chcr)&chcrBusy != 0 }
	m.SetDrawTime(true)
	at(1000)
	m.Bus.Write32(madr, 0x100)
	m.Bus.Write32(chcr, 0x01000401)
	at(2000)
	assert.Equal(t, false, ready(), "drawing 10,000 cycles")
	m.Bus.Write32(gpustat, 0x01000000)
	assert.Equal(t, true, ready(), "GP1(01h) ends it")

	at(3000)
	m.Bus.Write32(madr, 0x100)
	m.Bus.Write32(chcr, 0x01000401)
	assert.Equal(t, uint16(0x7c00), m.pixel(0, 0), "a blue rectangle, drawing")
	m.Bus.Write32(0x104, 0x6000ff00)
	at(3001)
	m.Bus.Write32(madr, 0x100)
	m.Bus.Write32(chcr, 0x01000401)
	at(3002)
	assert.Equal(t, true, busy(), "a green one is held")
	m.Bus.Write32(chcr, 0x00000401)
	at(20000)
	assert.Equal(t, false, busy(), "stopped")
	assert.Equal(t, uint16(0x7c00), m.pixel(0, 0), "it never goes")
}
