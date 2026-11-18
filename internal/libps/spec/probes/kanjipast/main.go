//go:build yaroze

// Command kanjipast is a probe for the spec beside spec/probes/kanji: the
// open items of spec 13. Krom2Tim and Krom2Tim2 given a bad code after a
// long run of characters (17, 33, 64, 128 and 200), and strings at a dx,
// dy off a texture page's or video memory's edge; Kanji print streams
// whose text runs past the window's end and then has a bad code or a
// special character, windows of no height or width, and a glyph image at
// a dx, dy that is not a page's corner or runs off the page. Each case
// sits between two GP1 marker words (10000100h plus its number) and
// prints its results; a stream case prints the glyph image read back
// from video memory and, where asked, the pixels drawn in the window.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var hex = "0123456789abcdef"

func word(w uint32) {
	for i := range 8 {
		d := w >> (28 - 4*i) & 15
		print(hex[d : d+1])
	}
}

// buf holds a string for a call, NUL-terminated.
var buf [512]byte

func cstr(s string) *byte {
	for i := range len(buf) {
		buf[i] = 0
	}
	copy(buf[:], s)
	return &buf[0]
}

const kan = "\x82\xa0"

func rep(s string, n int) string {
	r := ""
	for range n {
		r += s
	}
	return r
}

const fill = 0x5a5a5a5a

// tim has room for 16 rows of 16 characters past the header.
var tim [16*512 + 64]uint32

type kcase struct {
	label  string
	s      string
	dx, dy int32
	cx, cy int32
}

var kroms = []kcase{
	{label: "17 bad sjis", s: rep(kan, 17) + "\x85\x40"},
	{label: "33 bad sjis", s: rep(kan, 33) + "\x85\x40"},
	{label: "64", s: rep(kan, 64)},
	{label: "64 bad sjis", s: rep(kan, 64) + "\x85\x40"},
	{label: "128 bad hankaku", s: rep(kan, 128) + "\x01"},
	{label: "200 bad sjis", s: rep(kan, 200) + "\x98\x9f"},
	{label: "16 nl bad sjis", s: rep(kan, 16) + "\n\x85\x40"},
	{label: "hankaku 17 sjis", s: "\x01" + rep(kan, 17) + "\x85\x40"},
	{label: "sjis 17 hankaku", s: "\x85\x40" + rep(kan, 17) + "\x01"},
	{label: "17 sjis hankaku", s: rep(kan, 17) + "\x85\x40\x01"},
	{label: "17 hankaku sjis", s: rep(kan, 17) + "\x01\x85\x40"},
	{label: "16 at 1020 510", s: rep(kan, 16), dx: 1020, dy: 510},
	{label: "17 at 1020 510", s: rep(kan, 17), dx: 1020, dy: 510},
	{label: "17 at 1008 496", s: rep(kan, 17), dx: 1008, dy: 496},
	{label: "17 at 960 240", s: rep(kan, 17), dx: 960, dy: 240},
	{label: "3 at -4 -16", s: rep(kan, 3), dx: -4, dy: -16},
	{label: "3 at 70000 70000", s: rep(kan, 3), dx: 70000, dy: 70000},
	{label: "3 palette at 70000 70000", s: rep(kan, 3), cx: 70000, cy: 70000},
}

// krom runs a Krom2Tim case: the result, how far the TIM was written
// (the last word not left at fill), the header's words 3 and 13 to 15
// and a hash of all words written.
func krom(n *uint32, two bool, c kcase) {
	for i := range len(tim) {
		tim[i] = fill
	}
	dx, dy, cx, cy := c.dx, c.dy, c.cx, c.cy
	if dx == 0 && dy == 0 {
		dx, dy = 640, 0
	}
	if cx == 0 && cy == 0 {
		cx, cy = 0, 480
	}
	mark(*n)
	print("krom case ", *n, " ", two, " ", c.label, ":")
	var r int32
	if two {
		r = yaroze.Krom2Tim2(cstr(c.s), &tim[0], dx, dy, cx, cy, 0x7fff, 0)
	} else {
		r = yaroze.Krom2Tim(cstr(c.s), &tim[0], dx, dy, cx, cy, 0x7fff, 0)
	}
	mark(*n)
	last := -1
	for i := range len(tim) {
		if tim[i] != fill {
			last = i
		}
	}
	h := uint32(2166136261)
	for i := 0; i <= last; i++ {
		h = (h ^ tim[i]) * 16777619
	}
	print(" -> ", r, " last ", last, " hdr")
	if last >= 3 {
		print(" ")
		word(tim[3])
	}
	for i := 13; i < 16 && i <= last; i++ {
		print(" ")
		word(tim[i])
	}
	print(" hash ")
	word(h)
	println()
	*n++
}

type scase struct {
	label         string
	x, y, w, h, n int32
	dx, dy        int32
	prints        []string
	drawn         bool
}

var streams = []scase{
	// The text past the window's end, then a bad code or a special
	// character. The window holds two cells by one row and a bit: 4
	// characters, the fifth wraps to y 32, past h.
	{label: "w32 h20 5", w: 32, h: 20, n: 16, prints: []string{rep(kan, 5)}},
	{label: "w32 h20 3 sjis", w: 32, h: 20, n: 16, prints: []string{rep(kan, 3) + "\x85\x40"}},
	{label: "w32 h20 4 sjis", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x85\x40"}},
	{label: "w32 h20 4 level2", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x98\x9f"}},
	{label: "w32 h20 4 hankaku", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x01"}},
	{label: "w32 h20 4 kan sjis", w: 32, h: 20, n: 16, prints: []string{rep(kan, 5) + "\x85\x40"}},
	{label: "w32 h20 4 nl kan", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\n" + kan}},
	{label: "w32 h20 4 tab kan", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\t" + kan}},
	{label: "w32 h20 4 sp kan", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + " " + kan}},
	{label: "w32 h20 4 tilde kan", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "~" + kan}},
	{label: "w32 h20 4 trail kan", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x82\x3f" + kan}},
	{label: "w32 h20 4 x80", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x80"}},
	{label: "w32 h20 n4 4 sjis", w: 32, h: 20, n: 4, prints: []string{rep(kan, 4) + "\x85\x40"}},
	{label: "w32 h20 n5 4 sjis", w: 32, h: 20, n: 5, prints: []string{rep(kan, 4) + "\x85\x40"}},
	{label: "w32 h20 4 sjis flush again", w: 32, h: 20, n: 16, prints: []string{rep(kan, 4) + "\x85\x40"}},
	// The window's height: whether y is checked only when x wraps.
	{label: "h0 kan", w: 200, h: 0, n: 16, prints: []string{kan}},
	{label: "h10 kan", w: 200, h: 10, n: 16, prints: []string{kan}},
	{label: "h16 kan", w: 200, h: 16, n: 16, prints: []string{kan}},
	{label: "h16 nl kan", w: 200, h: 16, n: 16, prints: []string{"\n" + kan}},
	{label: "h20 nl nl kan", w: 200, h: 20, n: 16, prints: []string{"\n\n" + kan}},
	{label: "h20 nl nl kan sjis", w: 200, h: 20, n: 16, prints: []string{"\n\n" + kan + "\x85\x40"}},
	{label: "h20 nl nl sjis", w: 200, h: 20, n: 16, prints: []string{"\n\n\x85\x40"}},
	{label: "w0 kan x3", w: 0, h: 100, n: 16, prints: []string{rep(kan, 3)}},
	{label: "w10 kan x2", w: 10, h: 100, n: 16, prints: []string{rep(kan, 2)}},
	{label: "w0 h16 kan", w: 0, h: 16, n: 16, prints: []string{kan}},
	{label: "w0 h16 kan sjis", w: 0, h: 16, n: 16, prints: []string{kan + "\x85\x40"}},
	// Whether x wraps as a character is placed or as x moves past w: two
	// characters fill w 32 exactly, then a newline or a tab.
	{label: "w32 kan kan nl kan", w: 32, h: 100, n: 16, prints: []string{kan + kan + "\n" + kan}},
	{label: "w32 h20 kan kan nl sjis", w: 32, h: 20, n: 16, prints: []string{kan + kan + "\n\x85\x40"}},
	{label: "w16 kan nl kan", w: 16, h: 100, n: 16, prints: []string{kan + "\n" + kan}},
	{label: "w32 kan kan tab kan", w: 32, h: 100, n: 16, prints: []string{kan + kan + "\t" + kan}},
	{label: "w32 kan kan sp kan", w: 32, h: 100, n: 16, prints: []string{kan + kan + " " + kan}},
	{label: "w32 h16 kan kan sjis", w: 32, h: 16, n: 16, prints: []string{kan + kan + "\x85\x40"}},
	// dx, dy off a page's corner or off the page.
	{label: "dx 770 300 2", w: 200, h: 100, n: 16, dx: 770, dy: 300, prints: []string{kan + kan}, drawn: true},
	{label: "dx 1000 300 17", w: 400, h: 100, n: 32, dx: 1000, dy: 300, prints: []string{rep(kan, 17)}, drawn: true},
	{label: "dx 992 496 17", w: 400, h: 100, n: 32, dx: 992, dy: 496, prints: []string{rep(kan, 17)}, drawn: true},
	{label: "dx 960 240 17", w: 400, h: 100, n: 32, dx: 960, dy: 240, prints: []string{rep(kan, 17)}, drawn: true},
	{label: "dx 768 256 17", w: 400, h: 100, n: 32, dx: 768, dy: 256, prints: []string{rep(kan, 17)}, drawn: true},
}

// img is the glyph image read back, and the drawn window.
var img [80 * 16 / 2]uint32

func dumpRect(x, y, w, h int32) {
	for i := range len(img) {
		img[i] = 0x5a5a5a5a
	}
	r := yaroze.RECT{X: int16(x), Y: int16(y), W: int16(w), H: int16(h)}
	yaroze.StoreImage(&r, &img[0])
	yaroze.DrawSync(0)
	words := int(w) * int(h) / 2
	for i := 0; i < words; i++ {
		word(img[i])
		if i%8 == 7 || i == words-1 {
			println()
		} else {
			print(" ")
		}
	}
}

func stream(n *uint32, c scase) {
	dx, dy := c.dx, c.dy
	if dx == 0 && dy == 0 {
		dx, dy = 768, 256
	}
	yaroze.KanjiFntClose()
	clear := yaroze.RECT{X: int16(dx), Y: int16(dy), W: 80, H: 16}
	yaroze.ClearImage(&clear, 0x50, 0x50, 0x50)
	if c.drawn {
		// The page's corner, where the quadrangles' u, v start, and the
		// window's first cells, to a known color each.
		page := yaroze.RECT{X: int16(dx &^ 63), Y: int16(dy &^ 255), W: 80, H: 32}
		yaroze.ClearImage(&page, 0x30, 0x30, 0x30)
		win := yaroze.RECT{X: int16(8 + c.x), Y: int16(8 + c.y), W: 32, H: 16}
		yaroze.ClearImage(&win, 0x10, 0x10, 0x10)
	}
	yaroze.DrawSync(0)
	mark(*n)
	print("case ", *n, " ", c.label, ":")
	id := yaroze.KanjiFntOpen(8+c.x, 8+c.y, c.w, c.h, dx, dy, 768, 504, 0, c.n)
	print(" id ", id)
	for _, s := range c.prints {
		print(" ", yaroze.KanjiFntPrint(cstr(s)))
	}
	print(" ", yaroze.KanjiFntFlush(id) != nil)
	if c.label == "w32 h20 4 sjis flush again" {
		print(" ", yaroze.KanjiFntFlush(id) != nil, " ", yaroze.KanjiFntPrint(cstr(kan)))
	}
	yaroze.DrawSync(0)
	mark(*n)
	println()
	*n++
	dumpRect(dx, dy, 80, 16)
	if c.drawn {
		println("drawn:")
		dumpRect(8+c.x, 8+c.y, 32, 16)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	n := uint32(0)
	for _, c := range kroms {
		krom(&n, false, c)
	}
	for _, c := range kroms {
		krom(&n, true, c)
	}
	for _, c := range streams {
		stream(&n, c)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
