package main

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestFont checks the art parses into every glyph: the space is blank and
// every other character has dots, within the five columns.
func TestFont(t *testing.T) {
	assert.Equal(t, [7]byte{}, fontRows[0], "space")
	for i := 1; i < fontCount; i++ {
		var any byte
		for _, r := range fontRows[i] {
			any |= r
			assert.Equal(t, byte(0), r&^0x1f, string(rune(fontFirst+i)))
		}
		assert.Equal(t, true, any != 0, string(rune(fontFirst+i)))
	}
	// A, as the clean library draws it.
	assert.Equal(t, [7]byte{0x04, 0x0a, 0x11, 0x11, 0x1f, 0x11, 0x11}, fontRows['A'-fontFirst])
	assert.Equal(t, [7]byte{0x00, 0x00, 0x0e, 0x01, 0x0f, 0x11, 0x0f}, fontRows['a'-fontFirst])
}
