package gpu

import (
	"testing"
	"unsafe"

	"go.chrisrx.dev/x/assert"
)

// check sets a packet up and checks its size in words against the GPU's,
// that Init left its tag alone, and its command against the GPU's, which
// is the high byte of the word after the tag whatever the type.
func check[T any, P Packet[T]](t *testing.T, name string, words int, command uint8) {
	t.Helper()
	var p T
	P(&p).Init()
	assert.Equal(t, uintptr(words*4), unsafe.Sizeof(p), name)
	w := (*[2]uint32)(unsafe.Pointer(&p))
	assert.Equal(t, uint32(0), w[0], name)
	assert.Equal(t, command, uint8(w[1]>>24), name)
}

// TestPacketTypes checks each packet type's size and command.
func TestPacketTypes(t *testing.T) {
	check[FlatTriangle](t, "FlatTriangle", 5, 0x20)
	check[FlatQuad](t, "FlatQuad", 6, 0x28)
	check[TexturedTriangle](t, "TexturedTriangle", 8, 0x24)
	check[TexturedQuad](t, "TexturedQuad", 10, 0x2c)
	check[ShadedTriangle](t, "ShadedTriangle", 7, 0x30)
	check[ShadedQuad](t, "ShadedQuad", 9, 0x38)
	check[ShadedTexturedTriangle](t, "ShadedTexturedTriangle", 10, 0x34)
	check[ShadedTexturedQuad](t, "ShadedTexturedQuad", 13, 0x3c)
	check[Line](t, "Line", 4, 0x40)
	check[ShadedLine](t, "ShadedLine", 5, 0x50)
	check[Tile](t, "Tile", 4, 0x60)
	check[Sprite](t, "Sprite", 5, 0x64)
	check[DrawMode](t, "DrawMode", 2, 0xe1)
	check[TextureWindow](t, "TextureWindow", 3, 0xe2)
	check[VRAMCopy](t, "VRAMCopy", 6, 0x01)
}

// TestTextureWindow checks the window's word: none after Init, with the
// word after it clear whatever was there, and the masks and the corner
// Repeat works out, in the GPU's units of 8 pixels.
func TestTextureWindow(t *testing.T) {
	win := TextureWindow{nop: 0xffffffff}
	win.Init()
	w := (*[3]uint32)(unsafe.Pointer(&win))
	assert.Equal(t, [3]uint32{0, 0xe2000000, 0}, *w)
	win.Repeat(64, 32, 32, 16)
	assert.Equal(t, uint32(0xe2000000|4<<15|8<<10|0x1e<<5|0x1c), win.Window)
	win.Repeat(0, 0, 256, 256)
	assert.Equal(t, uint32(0xe2000000), win.Window, "the whole page is no window")
}

// TestVRAMCopy checks the copy's words: the cache's command, the copy's,
// and the two places and the size after them.
func TestVRAMCopy(t *testing.T) {
	c := VRAMCopy{
		From: At(832, 352),
		To:   At(836, 352),
		W:    1,
		H:    1,
	}
	c.Init()
	w := (*[6]uint32)(unsafe.Pointer(&c))
	assert.Equal(t, [6]uint32{0, 0x01000000, 0x80000000, 352<<16 | 832, 352<<16 | 836, 1<<16 | 1}, *w)
}

// TestLayout checks that the fields lie where the GPU reads them: the
// code over the color, and a corner's point, texture coordinate and
// palette in the words after.
func TestLayout(t *testing.T) {
	q := TexturedQuad{
		Color: Color{R: 1, G: 2, B: 3},
		XY0:   At(-2, 5),
		UV0:   UV{U: 6, V: 7},
		Clut:  0x7801,
		Page:  0x15,
	}
	q.Init()
	q.Code |= CodeTranslucent
	w := (*[10]uint32)(unsafe.Pointer(&q))
	assert.Equal(t, uint32(0x2e030201), w[1])
	assert.Equal(t, uint32(0x0005fffe), w[2])
	assert.Equal(t, uint32(0x78010706), w[3])
	assert.Equal(t, uint32(0x0015), w[5]>>16)
}
