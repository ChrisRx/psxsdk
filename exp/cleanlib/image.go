//go:build yaroze

package cleanlib

import "unsafe"

// Images in video memory: loading them, and the TIM files they come in.

// GetTPage returns the texture page value for a texture at x, y in video
// memory, of color depth tp (0 for 4-bit, 1 for 8-bit, 2 for 16-bit), with
// the semi-transparency rate abr.
//
//export GetTPage
//go:noinline
func GetTPage(tp, abr, x, y int32) uint16 {
	v := uint16((tp&3)<<7 | (abr&3)<<5 | (y&0x100)>>4 | (x&0x3ff)>>6)
	space(4)
	pace(getTPagePad)
	return v
}

// getTPagePad is the rest of GetTPage's 96 cycles (spec 21), with the four stores
// for what its turns leave.
const getTPagePad = 26

// LoadImage copies the pixels at p into the rectangle rect of video
// memory. It returns once the copy has started, or for an image that is
// not whole blocks of 16 words, once the CPU has written the last of it;
// DrawSync waits for it either way. With the queue idle, the size it kept
// within video memory is left in rect too (spec 21).
//
//export LoadImage
//go:noinline
func LoadImage(rect *RECT, p *uint32) int32 {
	if loadImage(*rect, unsafe.Pointer(p)) {
		rect.W, rect.H = imageSize(*rect)
	}
	return 0
}

// GsIMAGE is a TIM image as GsGetTimInfo describes it: its pixel mode,
// where its pixels and palette go in video memory, and where they are.
type GsIMAGE struct {
	Pmode  uint32
	Px, Py int16
	Pw, Ph uint16
	Pixel  *uint32
	Cx, Cy int16
	Cw, Ch uint16
	Clut   *uint32
}

// timHasClut is the bit of a TIM's flags that says a palette comes first.
const timHasClut = 1 << 3

// GsGetTimInfo fills info from the TIM image at im, which points at the
// word after the file's ID: the flags, then the palette's block if there
// is one, then the pixels' block. Each block is its length in bytes, its
// place and size in video memory, and the data. With no palette the palette's
// fields stay as they were: programs load whatever they last held.
//
//export GsGetTimInfo
func GsGetTimInfo(im *uint32, info *GsIMAGE) {
	flags := *im
	p := unsafe.Add(unsafe.Pointer(im), 4)
	// Pmode is the flags as they are, with the palette bit, which a
	// program tests to know whether there is a palette to load.
	info.Pmode = flags
	if flags&timHasClut != 0 {
		info.Cx, info.Cy, info.Cw, info.Ch, info.Clut = timBlock(p)
		p = unsafe.Add(p, *(*uint32)(p))
	}
	info.Px, info.Py, info.Pw, info.Ph, info.Pixel = timBlock(p)
}

// timBlock reads a TIM block's place and size in video memory and returns
// them with the address of its data.
func timBlock(p unsafe.Pointer) (x, y int16, w, h uint16, data *uint32) {
	h16 := unsafe.Slice((*uint16)(unsafe.Add(p, 4)), 4)
	return int16(h16[0]), int16(h16[1]), h16[2], h16[3], (*uint32)(unsafe.Add(p, 12))
}
