package main

// The textures: groups of TIMs in the second archive, each group loaded
// into video memory together and described by a table the drawing code
// reads texture pages and corners from.

import "go.chrisrx.dev/psx/yaroze"

// A texture is one TIM once it is in video memory: where its pixels went
// and the fields a sprite or a polygon names it by.
type texture struct {
	px, py int16  // the pixels' place in video memory, in 16-bit units
	w, h   int16  // its size in pixels
	u, v   uint8  // its corner within its texture page
	tpageX uint16 // px & 0x3c0, the texture page's column
	tpageY uint16 // py & 0x100, the texture page's row
	cx, cy int16  // the palette's place in video memory
	depth  uint8  // 0 for 4-bit pixels, 1 for 8, 2 for 16
}

// tpage returns the GPU's texture page word for the texture: its page in
// video memory with its color depth and the given semi-transparency rate.
func (t *texture) tpage(abr uint16) uint16 {
	return yaroze.GetTPage(int32(t.depth), int32(abr), int32(t.tpageX), int32(t.tpageY))
}

// clut returns the GPU's palette word for the texture.
func (t *texture) clut() uint16 {
	return uint16(t.cy)<<6 | uint16(t.cx)>>4&0x3f
}

// loadTextures unpacks every TIM of group g of the archive into video memory
// and returns their table. Each is unpacked into scratch, which must hold
// the largest, loaded, and left behind.
func loadTextures(a *archive, g int, scratch []uint32) []texture {
	table := make([]texture, a.entries(g))
	for e := range table {
		words := a.unpackWords(g, e, scratch)
		im := yaroze.TIM(words).Load()
		depth := uint8(im.Pmode & 3)
		t := &table[e]
		t.px, t.py = im.Px, im.Py
		t.w = int16(im.Pw) << (2 - depth)
		t.h = int16(im.Ph)
		t.u = uint8(im.Px&0x3f) << (2 - depth)
		t.v = uint8(im.Py)
		t.tpageX = uint16(im.Px) & 0x3c0
		t.tpageY = uint16(im.Py) & 0x100
		t.cx, t.cy = im.Cx, im.Cy
		t.depth = depth
	}
	return table
}
