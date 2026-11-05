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
