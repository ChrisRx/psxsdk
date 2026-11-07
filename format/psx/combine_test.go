package psx

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// A program may name RAM by its uncached mirror, which is the same RAM.
func TestEmbedUncachedAddress(t *testing.T) {
	f := New(0x80010000, 0x80010000, make([]byte, 16))
	assert.NoError(t, Embed(f, 0xa0010008, []byte{1, 2, 3, 4}))
	assert.Equal(t, uint32(0x80010000), f.TextAddr)
	assert.Equal(t, []byte{1, 2, 3, 4}, f.Section("text").Data[8:12])
}

// TestEmbedWholeSectors keeps the text size a whole number of sectors, which
// is all the BIOS loads, whatever the size of the data.
func TestEmbedWholeSectors(t *testing.T) {
	f := New(0x80010000, 0x80010000, make([]byte, 2048))
	assert.NoError(t, Embed(f, 0x80020000, []byte{1, 2, 3}))
	assert.Equal(t, uint32(0x10800), f.TextSize)
	assert.Equal(t, 0x10800, len(f.Section("text").Data))
	assert.Equal(t, []byte{1, 2, 3}, f.Section("text").Data[0x10000:0x10003])
}
