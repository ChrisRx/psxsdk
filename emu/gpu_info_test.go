package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestGPUInfoRequests checks what GP1(10h) answers through GPUREAD: the
// settings packed as the commands that made them take them, with the
// drawing offset's Y at bit 11 as GP0(E5h) has it, and the GPU's type.
// The library's ClearImage reads the offset back this way to restore it
// after its fills, so a wrong answer moves everything drawn afterwards.
func TestGPUInfoRequests(t *testing.T) {
	m := newTestMachine(t)
	const (
		gp0 = 0x1f801810
		gp1 = 0x1f801814
	)
	read := func(req uint32) uint32 {
		t.Helper()
		m.Bus.Write32(gp1, 0x10000000|req)
		return m.Bus.Read32(gp1 - 4)
	}

	m.Bus.Write32(gp0, 0xe3000000|20<<10|10)   // drawing area from (10, 20)
	m.Bus.Write32(gp0, 0xe4000000|239<<10|319) // to (319, 239)
	m.Bus.Write32(gp0, 0xe5000000|120<<11|160) // drawing offset (160, 120)
	assert.Equal(t, uint32(20<<10|10), read(3), "drawing area top left")
	assert.Equal(t, uint32(239<<10|319), read(4), "drawing area bottom right")
	assert.Equal(t, uint32(120<<11|160), read(5), "drawing offset")

	x, y := int32(-16), int32(-8)
	negative := uint32(y)&0x7ff<<11 | uint32(x)&0x7ff
	m.Bus.Write32(gp0, 0xe5000000|negative) // offset (-16, -8)
	assert.Equal(t, negative, read(5), "a negative offset keeps its bits")

	assert.Equal(t, uint32(2), read(7), "the GPU's type")
	assert.Equal(t, uint32(0), read(8), "request 08h")
}
