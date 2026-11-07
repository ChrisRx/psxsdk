package serial

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestRelocated(t *testing.T) {
	prog := &Program{
		Segments: []Segment{{Name: "text", Addr: 0x80090000, Data: make([]byte, 16)}},
		PC:       0x80090000,
		GP:       0x80098000,
		SP:       defaultStack,
	}
	got, err := Relocated(prog, Segment{Name: "image", Addr: 0x80010000, Data: make([]byte, 6)})
	assert.NoError(t, err)
	assert.Equal(t, uint32(loaderAddr), got.PC)
	assert.Equal(t, prog.GP, got.GP)
	assert.Equal(t, prog.SP, got.SP)
	assert.Equal(t, 3, len(got.Segments))
	assert.Equal(t, uint32(loaderAddr-8), got.Segments[0].Addr, "the copy should sit under the loader")
	assert.Equal(t, 8, len(got.Segments[0].Data), "a copy goes by whole words")
	assert.Equal(t, uint32(loaderAddr), got.Segments[1].Addr)
	assert.Equal(t, prog.Segments[0], got.Segments[2])
}

func TestRelocatedTooBig(t *testing.T) {
	prog := &Program{Segments: []Segment{{Name: "text", Addr: 0x80090000, Data: make([]byte, 0x160000)}}}
	if _, err := Relocated(prog, Segment{Name: "image", Addr: 0x80010000, Data: make([]byte, 4096)}); err == nil {
		t.Error("a program reaching the loader's copies should be refused")
	}
}
