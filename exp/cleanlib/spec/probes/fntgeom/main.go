//go:build yaroze

// Command fntgeom is a probe for the spec: where FntFlush wraps and
// stops in a window whose width, height or origin is not a multiple of
// 8. Each case loads the font again (closing every stream), opens one
// stream with the case's window, prints the text, flushes it and prints
// the packets the flush built over the TTY, one primitive a line, as
// hex words (the tag's address field left out), so that the sprites'
// positions can be read.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

type geom struct {
	x, y, w, h int32
	isbg       int32
	text       string
}

var cases = []geom{
	// Widths off the 8 grid: 12 glyphs fit in 100 whole, the 13th ends
	// at 104.
	{0, 0, 100, 240, 0, "ABCDEFGHIJKLMNOP"},
	{0, 0, 316, 240, 0, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB"}, // the 40th ends at 320
	{0, 0, 324, 240, 0, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB"},
	{0, 0, 100, 240, 0, "ABCDEFGHIJKL B"},  // the space ends at 104
	{0, 0, 100, 240, 0, "ABCDEFGHIJK\tB"},  // the tab ends at 120
	{0, 0, 100, 240, 0, "ABCDEFGHIJKL\nB"}, // 96 then a newline
	// The origin off the grid.
	{4, 0, 100, 240, 0, "ABCDEFGHIJKLMNOP"},
	{4, 0, 96, 240, 0, "ABCDEFGHIJKLMNOP"},
	{0, 4, 320, 100, 0, "\n\n\n\n\n\n\n\n\n\n\n\nAB"},
	// Heights off the 8 grid: 12 rows fit in 100 whole, the 13th ends
	// at 104.
	{0, 0, 320, 100, 0, "\n\n\n\n\n\n\n\n\n\n\n\nAB"},
	{0, 0, 320, 100, 0, "\n\n\n\n\n\n\n\n\n\n\n\n\nAB"},
	{0, 0, 320, 100, 0, "\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB"},
	// Narrower than a glyph.
	{0, 0, 4, 240, 0, "ABC"},
	{0, 0, 8, 240, 0, "ABC"},
	// Shorter than a glyph, and empty.
	{0, 0, 320, 4, 0, "AB\nC"},
	{0, 0, 320, 0, 0, "AB\nC"},
	{0, 0, 0, 240, 0, "AB"},
	// The background with no width or height given: its size.
	{0, 0, 0, 0, 1, "AB\nCDE"},
	{0, 0, 100, 0, 1, "AB\nCDE"},
	{0, 0, 0, 100, 1, "AB\nCDE"},
	{0, 0, 100, 100, 1, "AB\nCDE"},
	{0, 0, 0, 240, 1, "ABC"},         // no newline at all
	{0, 0, 0, 240, 1, "ABCDE\nAB\n"}, // the longest line first, the last one ended
	{8, 16, 0, 240, 1, "AB\nCDE"},    // the origin off zero
	{0, 0, 0, 0, 1, "AB\nCD\nEF"},    // newlines after the one that ends the flush
	{0, 0, 0, 0, 1, "ABCDE\nAB\nEFGHIJ\n"},
}

const hexDigits = "0123456789abcdef"

func hex(w uint32) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = hexDigits[w&15]
		w >>= 4
	}
	return string(b[:])
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	for i, c := range cases {
		yaroze.FntLoad(960, 256)
		yaroze.FntOpen(c.x, c.y, c.w, c.h, c.isbg, 512)
		yaroze.DrawSync(0)
		yaroze.Print(c.text)
		head := yaroze.FntFlush(-1)
		yaroze.DrawSync(0)
		println("case", i)
		p := head
		for n := 0; n < 200 && p != nil; n++ {
			tag := *p
			size := int(tag >> 24)
			line := "  " + hex(tag)[:2]
			for k := 1; k <= size; k++ {
				line += " " + hex(*(*uint32)(unsafe.Add(unsafe.Pointer(p), 4*k)))
			}
			println(line)
			next := tag & 0xffffff
			if next == 0xffffff {
				break
			}
			p = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
		}
		yaroze.VSync(0)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
