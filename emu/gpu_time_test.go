package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestDrawTime checks the GPU's drawing time under SetDrawTime: a packet
// list of two 100 by 100 rectangles leaves the GPU drawing for 20,000
// cycles, its DMA ending when the second reaches the GPU, half way; a list
// sent meanwhile waits for the GPU to finish; and with the switch off the
// GPU is ready at once.
func TestDrawTime(t *testing.T) {
	const (
		gpustat = 0x1f801814
		madr    = 0x1f8010a0
		chcr    = 0x1f8010a8
	)
	m := newTestMachine(t)
	m.Bus.Write32(0x1f801810, 0xe4000000|511<<10|1023) // draw anywhere
	// A list of a blue and a green rectangle of 100 by 100, at 100, and
	// one of a red one, at 120.
	list := []uint32{
		0x03000110, 0x60ff0000, 0, 100<<16 | 100,
		0x03ffffff, 0x6000ff00, 0, 100<<16 | 100,
		0x03ffffff, 0x600000ff, 0, 100<<16 | 100,
	}
	for i, w := range list {
		m.Bus.Write32(0x100+uint32(i)*4, w)
	}
	at := func(c uint64) {
		m.CPU.TotalCycles = c
		m.dma.update()
	}
	ready := func() bool { return m.Bus.Read32(gpustat)&gpustatReadyCmd != 0 }
	busy := func() bool { return m.Bus.Read32(chcr)&chcrBusy != 0 }
	send := func(addr uint32) {
		m.Bus.Write32(madr, addr)
		m.Bus.Write32(chcr, 0x01000401)
	}
	const green, red = 0x03e0, 0x001f

	send(0x120)
	assert.Equal(t, false, busy(), "off, the list goes at once")
	assert.Equal(t, true, ready(), "and the GPU is ready")
	assert.Equal(t, uint16(red), m.pixel(0, 0))

	m.SetDrawTime(true)
	at(1000)
	send(0x100)
	assert.Equal(t, uint16(green), m.pixel(0, 0), "the pixels land as the commands arrive")
	at(1001)
	assert.Equal(t, false, ready(), "the GPU draws for 20,000 cycles")
	assert.Equal(t, true, busy(), "the DMA runs until the last rectangle reaches the GPU")
	at(10999)
	assert.Equal(t, true, busy())
	at(11000)
	assert.Equal(t, false, busy(), "which is half way")
	assert.Equal(t, false, ready())
	at(11001)
	send(0x120)
	at(11002)
	assert.Equal(t, true, busy(), "a list sent while the GPU draws is held")
	assert.Equal(t, uint16(green), m.pixel(0, 0), "and not yet sent")
	at(21000)
	assert.Equal(t, uint16(red), m.pixel(0, 0), "it goes once the GPU is free")
	assert.Equal(t, false, busy(), "its one rectangle at once")
	assert.Equal(t, false, ready(), "and keeps the GPU drawing")
	at(30999)
	assert.Equal(t, false, ready())
	at(31000)
	assert.Equal(t, true, ready())
}
