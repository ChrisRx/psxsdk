package serial

import (
	"encoding/binary"
	"slices"
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

// TakeOverMonitor uploads the monitor alone, and its loader saves the
// interrupt mask in s0 first and puts it back last, where TakeOver's leaves
// it clear, and opens the library's events again.
func TestTakeOverMonitor(t *testing.T) {
	monitor := &Program{
		Segments: []Segment{{Name: "text", Addr: 0x80068000, Data: make([]byte, 16)}},
		PC:       0x80068000,
		GP:       0x80080000,
		SP:       defaultStack,
	}
	got, err := TakeOverMonitor(monitor)
	assert.NoError(t, err)
	assert.Equal(t, uint32(loaderAddr), got.PC)
	assert.Equal(t, 2, len(got.Segments))
	assert.Equal(t, uint32(loaderAddr-16), got.Segments[0].Addr)

	words := func(p *Program) []uint32 {
		code := p.Segments[len(p.Segments)-1].Data[:namesAt]
		w := make([]uint32, len(code)/4)
		for i := range w {
			w[i] = binary.LittleEndian.Uint32(code[4*i:])
		}
		return w
	}
	saveMask := iType(0x23, regT0, regS0, 0)
	restoreMask := iType(0x2b, regT0, regS0, 0)
	assert.Equal(t, true, slices.Contains(words(got), saveMask))
	assert.Equal(t, true, slices.Contains(words(got), restoreMask))

	// It opens the library's eight events again, each with OpenEvent and
	// EnableEvent, B(08h) and B(0Ch).
	calls := func(p *Program, fn uint32) (n int) {
		for _, w := range words(p) {
			if w == iType(0x0d, regT1, regT1, fn) {
				n++
			}
		}
		return n
	}
	assert.Equal(t, len(libraryEvents), calls(got, callOpen))
	assert.Equal(t, len(libraryEvents), calls(got, callEnable))
	assert.Equal(t, 8, len(libraryEvents))

	full, err := TakeOver(monitor, Segment{Name: "image", Addr: 0x80010000, Data: make([]byte, 8)})
	assert.NoError(t, err)
	assert.Equal(t, false, slices.Contains(words(full), saveMask))
	assert.Equal(t, false, slices.Contains(words(full), restoreMask))
	assert.Equal(t, 0, calls(full, callOpen))
}
