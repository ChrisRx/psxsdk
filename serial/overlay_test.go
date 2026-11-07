package serial

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestTakeOver(t *testing.T) {
	monitor := &Program{
		Segments: []Segment{{Name: "text", Addr: 0x80068000, Data: make([]byte, 16)}},
		PC:       0x80068000,
		GP:       0x80080000,
		SP:       defaultStack,
	}
	got, err := TakeOver(monitor, Segment{Name: "image", Addr: 0x80010000, Data: make([]byte, 6)})
	assert.NoError(t, err)
	assert.Equal(t, uint32(loaderAddr), got.PC)
	assert.Equal(t, monitor.GP, got.GP)
	assert.Equal(t, monitor.SP, got.SP)
	assert.Equal(t, 3, len(got.Segments))
	assert.Equal(t, uint32(loaderAddr-24), got.Segments[0].Addr, "the copies should sit under the loader")
	assert.Equal(t, 8, len(got.Segments[0].Data), "a copy goes by whole words")
	assert.Equal(t, uint32(loaderAddr-16), got.Segments[1].Addr)
	assert.Equal(t, uint32(loaderAddr), got.Segments[2].Addr)
	assert.Equal(t, namesAt+len("tty mem cdx "), len(got.Segments[2].Data))
}
