package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestFillMemory(t *testing.T) {
	fill := func(seed uint64) *Machine {
		m, err := New(make([]byte, 512<<10))
		assert.NoError(t, err)
		m.FillMemory(seed)
		return m
	}
	a, b, c := fill(1), fill(1), fill(2)
	for _, addr := range []uint32{0x00000000, 0x000a0000, 0x001ffffc, 0x1f800000, 0x1f8003fc} {
		assert.Equal(t, a.Bus.Read32(addr), b.Bus.Read32(addr))
		assert.Equal(t, true, a.Bus.Read32(addr) != c.Bus.Read32(addr), addr)
	}
	assert.Equal(t, a.gpu.vram, b.gpu.vram)
	assert.Equal(t, true, a.gpu.vram != c.gpu.vram)
}
