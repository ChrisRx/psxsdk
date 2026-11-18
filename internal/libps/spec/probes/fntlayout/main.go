//go:build yaroze

// Command fntlayout is a probe for the spec: what FntFlush makes of a
// line wrap followed by a newline, of tabs, and of spaces at the right
// edge. Each case is printed into a 320 by 240 window at the origin and
// flushed, and the packets the flush built are printed over the TTY, one
// primitive a line, as hex words, so that the sprites' positions can be
// read.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var cases = []string{
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nB",   // 40 glyphs fill the line, then a newline
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nB",    // 39 glyphs, then a newline
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB",     // 40 glyphs, then a glyph
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n\nB", // 40 glyphs, two newlines
	"A\tB",
	"AB\tC",
	"ABC\tD",
	"ABCD\tE",
	"ABCDE\tF",
	"\tB",
	"\t\tB",
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\tB",   // 38 glyphs, a tab near the edge
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA    B",  // 37 glyphs, 4 spaces to the edge, a glyph
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA B",  // 40 glyphs, a space, a glyph
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\tB", // 40 glyphs, a tab, a glyph
	"a~c700b", // a color directive
	// The window is 30 rows tall: text on the last row and past it.
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAB",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAB",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAB",
	"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABC", // the 40th glyph fills the last row
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
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 0, 320, 240, 0, 512)
	yaroze.DrawSync(0)
	for i, s := range cases {
		yaroze.Print(s)
		head := yaroze.FntFlush(-1)
		yaroze.DrawSync(0)
		println("case", i)
		p := head
		for n := 0; n < 200 && p != nil; n++ {
			tag := *p
			size := int(tag >> 24)
			line := "  " + hex(tag)
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
