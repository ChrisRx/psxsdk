//go:build yaroze

// Command kanjistream is a probe for the spec beside spec/probes/kanji:
// the Kanji print streams over more inputs, each case between two GP1
// marker words (10000100h plus its number). Each case closes every
// stream, opens one, prints to it, flushes it and prints the results, and
// then the glyph image the flush loaded, read back from video memory.
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
var buf [128]byte

func cstr(s string) *byte {
	buf = [128]byte{}
	copy(buf[:], s)
	return &buf[0]
}

// img is the glyph image read back.
var img [64 * 16 * 2]uint32

type kcase struct {
	label         string
	x, y, w, h, n int32
	dx, dy        int32
	isbg          int32
	prints        []string
	flush         int32
}

const kan = "\x82\xa0"

func rep(s string, n int) string {
	r := ""
	for range n {
		r += s
	}
	return r
}

var cases = []kcase{
	{label: "A", w: 200, h: 100, n: 16, prints: []string{"A"}},
	{label: "AB", w: 200, h: 100, n: 16, prints: []string{"AB"}},
	{label: "kanA", w: 200, h: 100, n: 16, prints: []string{kan + "A"}},
	{label: "nl", w: 200, h: 100, n: 16, prints: []string{"\n"}},
	{label: "kan nl kan", w: 200, h: 100, n: 16, prints: []string{kan + "\n" + kan}},
	{label: "14 then A A kan", w: 300, h: 100, n: 16, prints: []string{rep(kan, 14), "A", "A", kan}},
	{label: "16", w: 300, h: 100, n: 16, prints: []string{rep(kan, 16)}},
	{label: "20", w: 400, h: 100, n: 16, prints: []string{rep(kan, 20)}},
	{label: "n4", w: 200, h: 100, n: 4, prints: []string{kan, kan, kan, "A"}},
	{label: "n1", w: 200, h: 100, n: 1, prints: []string{"A", kan}},
	{label: "n0", w: 200, h: 100, n: 0, prints: []string{"A"}},
	{label: "wrap w40", w: 40, h: 100, n: 16, prints: []string{rep(kan, 5)}},
	{label: "clip h20", w: 32, h: 20, n: 16, prints: []string{rep(kan, 5)}},
	{label: "main60", w: 200, h: 20, n: 16, prints: []string{main60}},
	{label: "main60 h100", w: 200, h: 100, n: 16, prints: []string{main60}},
	{label: "main60 w400", w: 400, h: 20, n: 16, prints: []string{main60}},
	{label: "nl 14", w: 200, h: 100, n: 16, prints: []string{"\n" + rep(kan, 14)}},
	{label: "nl 13", w: 200, h: 100, n: 16, prints: []string{"\n" + rep(kan, 13)}},
	{label: "w40 nl 5", w: 40, h: 100, n: 16, prints: []string{kan + "\n" + rep(kan, 5)}},
	{label: "w40 5 nl", w: 40, h: 100, n: 16, prints: []string{rep(kan, 5) + "\n" + kan}},
	{label: "w40 two prints", w: 40, h: 100, n: 16, prints: []string{rep(kan, 5), "\n" + kan}},
	{label: "w40 h20 nl", w: 40, h: 20, n: 16, prints: []string{kan + "\n" + rep(kan, 5)}},
	{label: "n4 nl", w: 200, h: 100, n: 4, prints: []string{kan + "\n" + rep(kan, 5)}},
	{label: "n4 nl nl", w: 200, h: 100, n: 4, prints: []string{"\n\n\n\n\n" + kan}},
	{label: "space", w: 200, h: 100, n: 16, prints: []string{" " + kan}},
	{label: "bang", w: 200, h: 100, n: 16, prints: []string{"!" + kan}},
	{label: "tilde", w: 200, h: 100, n: 16, prints: []string{"~" + kan}},
	{label: "tilde c", w: 200, h: 100, n: 16, prints: []string{"~c" + kan}},
	{label: "del", w: 200, h: 100, n: 16, prints: []string{"\x7f" + kan}},
	{label: "tab", w: 200, h: 100, n: 16, prints: []string{"\t" + kan}},
	{label: "hankaku", w: 200, h: 100, n: 16, prints: []string{"\xa1" + kan}},
	{label: "sjis bad", w: 200, h: 100, n: 16, prints: []string{kan + "\x85\x40" + kan + "\x98\x9f"}},
	{label: "trail bad", w: 200, h: 100, n: 16, prints: []string{"\x82\x3f" + kan + "\x89\x7f" + kan + "\x81\xad"}},
	{label: "dx", w: 200, h: 100, n: 16, dx: 770, dy: 300, prints: []string{kan + kan}},
	{label: "flush 1", w: 200, h: 100, n: 16, prints: []string{kan}, flush: 1},
	{label: "flush 5", w: 200, h: 100, n: 16, prints: []string{kan}, flush: 5},
	{label: "flush -2", w: 200, h: 100, n: 16, prints: []string{kan}, flush: -2},
	{label: "nl 15", w: 300, h: 100, n: 16, prints: []string{"\n" + rep(kan, 15)}},
	{label: "A nl 14", w: 300, h: 100, n: 16, prints: []string{"A\n" + rep(kan, 14)}},
	{label: "nl A15", w: 300, h: 100, n: 16, prints: []string{"\n" + rep("A", 15)}},
	{label: "nl x16", w: 300, h: 300, n: 16, prints: []string{rep("\n", 16)}},
	{label: "nl x20", w: 300, h: 400, n: 16, prints: []string{rep("\n", 20)}},
	{label: "20 nl", w: 400, h: 100, n: 16, prints: []string{rep(kan, 20) + "\n"}},
	{label: "A40 nl", w: 700, h: 100, n: 16, prints: []string{rep("A", 40) + "\n"}},
	{label: "nl A40", w: 700, h: 100, n: 16, prints: []string{"\n" + rep("A", 40)}},
	{label: "14 nl", w: 300, h: 100, n: 16, prints: []string{rep(kan, 14) + "\n"}},
	{label: "15 nl", w: 300, h: 100, n: 16, prints: []string{rep(kan, 15) + "\n"}},
	{label: "n32 main60", w: 400, h: 100, n: 32, prints: []string{main60}},
	{label: "n32 nl 40", w: 400, h: 100, n: 32, prints: []string{"\n" + rep(kan, 40)}},
	{label: "n8 nl 10", w: 400, h: 100, n: 8, prints: []string{"\n" + rep(kan, 10)}},
	{label: "n8 A nl 10", w: 400, h: 100, n: 8, prints: []string{"A\n" + rep(kan, 10)}},
	{label: "n8 10", w: 400, h: 100, n: 8, prints: []string{rep(kan, 10)}},
	{label: "two prints nl", w: 400, h: 100, n: 16, prints: []string{kan, "\n" + rep(kan, 15)}},
	{label: "A tab", w: 400, h: 100, n: 16, prints: []string{"A\t" + kan}},
	{label: "A5 tab", w: 400, h: 100, n: 16, prints: []string{"AAAAA\t" + kan}},
	{label: "tab wrap", w: 40, h: 100, n: 16, prints: []string{kan + "\t" + kan}},
	{label: "x01", w: 400, h: 100, n: 16, prints: []string{"\x01" + kan}},
	{label: "cr", w: 400, h: 100, n: 16, prints: []string{kan + "\r" + kan}},
	{label: "x1f", w: 400, h: 100, n: 16, prints: []string{"\x1f" + kan}},
	{label: "x80", w: 400, h: 100, n: 16, prints: []string{"\x80" + kan}},
	{label: "x99", w: 400, h: 100, n: 16, prints: []string{"\x99\x40" + kan}},
	{label: "x82 end", w: 400, h: 100, n: 16, prints: []string{kan + "\x82"}},
	{label: "space wrap", w: 40, h: 100, n: 16, prints: []string{kan + kan + "  " + kan}},
	{label: "tilde x3", w: 400, h: 100, n: 16, prints: []string{"~~~" + kan}},
	{label: "spaces n", w: 400, h: 100, n: 4, prints: []string{"     " + kan}},
	{label: "tilde n", w: 400, h: 100, n: 4, prints: []string{"~~~~~" + kan}},
	{label: "n17 17", w: 400, h: 100, n: 17, prints: []string{rep(kan, 17)}},
	{label: "n40 33", w: 600, h: 100, n: 40, prints: []string{rep(kan, 33)}},
	{label: "pct pct", w: 300, h: 100, n: 16, prints: []string{"%%" + kan}},
	{label: "kan pct kan", w: 300, h: 100, n: 16, prints: []string{kan + "%%" + kan}},
	{label: "pct x4", w: 300, h: 100, n: 16, prints: []string{"%%%%" + kan}},
	{label: "digits", w: 300, h: 100, n: 16, prints: []string{"12" + kan}},
	{label: "bad then flush again", w: 300, h: 100, n: 16, prints: []string{"\x01" + kan}},
	{label: "ascii 0", w: 300, h: 100, n: 32, prints: []string{"!\"#$&'()*+,-./01"}},
	{label: "ascii 1", w: 300, h: 100, n: 32, prints: []string{"23456789:;<=>?@A"}},
	{label: "ascii 2", w: 300, h: 100, n: 32, prints: []string{"BCDEFGHIJKLMNOPQ"}},
	{label: "ascii 3", w: 300, h: 100, n: 32, prints: []string{"RSTUVWXYZ[\\]^_`a"}},
	{label: "ascii 4", w: 300, h: 100, n: 32, prints: []string{"bcdefghijklmnopq"}},
	{label: "ascii 5", w: 300, h: 100, n: 32, prints: []string{"rstuvwxyz{|}"}},
}

// main60 is the text of the newline case of spec/probes/kanji.
const main60 = "\x82\xa0\n\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2"

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	n := uint32(0)
	mark(n)
	print("print before open: ", yaroze.KanjiFntPrint(cstr(kan)), "\n")
	print("flush before open: ", yaroze.KanjiFntFlush(0) != nil, "\n")
	yaroze.DrawSync(0)
	mark(n)
	n++
	for _, c := range cases {
		dx, dy := c.dx, c.dy
		if dx == 0 && dy == 0 {
			dx, dy = 768, 256
		}
		yaroze.KanjiFntClose()
		clear := yaroze.RECT{X: int16(dx), Y: int16(dy), W: 80, H: 16}
		yaroze.ClearImage(&clear, 0x50, 0x50, 0x50)
		yaroze.DrawSync(0)
		mark(n)
		print("case ", n, " ", c.label, ":")
		id := yaroze.KanjiFntOpen(8+c.x, 8+c.y, c.w, c.h, dx, dy, 768, 504, c.isbg, c.n)
		print(" id ", id)
		for _, s := range c.prints {
			print(" ", yaroze.KanjiFntPrint(cstr(s)))
		}
		print(" ", yaroze.KanjiFntFlush(c.flush) != nil)
		if c.label == "bad then flush again" {
			print(" ", yaroze.KanjiFntFlush(0) != nil, " ", yaroze.KanjiFntPrint(cstr(kan)))
		}
		yaroze.DrawSync(0)
		mark(n)
		println()
		n++
		dump(dx, dy)
	}
	// Opening more than eight, with n of 1, 4 and 100.
	for _, k := range []int32{1, 4, 100} {
		yaroze.KanjiFntClose()
		mark(n)
		print("opens n ", k, ":")
		for range 10 {
			print(" ", yaroze.KanjiFntOpen(8, 8, 100, 100, 768, 256, 768, 504, 0, k))
		}
		yaroze.DrawSync(0)
		mark(n)
		println()
		n++
	}
	// Two streams: printing past the first's n, and flushing each.
	yaroze.KanjiFntClose()
	mark(n)
	print("two:")
	print(" ", yaroze.KanjiFntOpen(8, 8, 100, 100, 768, 256, 768, 504, 0, 2))
	print(" ", yaroze.KanjiFntOpen(8, 108, 100, 100, 832, 256, 768, 505, 0, 8))
	print(" ", yaroze.KanjiFntPrint(cstr(kan+kan)), " ", yaroze.KanjiFntPrint(cstr(kan+kan)))
	print(" ", yaroze.KanjiFntFlush(1) != nil, " ", yaroze.KanjiFntFlush(0) != nil)
	yaroze.DrawSync(0)
	mark(n)
	println()
	n++
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// dump reads back the first 20 cells of the glyph image and prints them.
func dump(dx, dy int32) {
	for i := range len(img) {
		img[i] = 0x5a5a5a5a
	}
	r := yaroze.RECT{X: int16(dx), Y: int16(dy), W: 80, H: 16}
	yaroze.StoreImage(&r, &img[0])
	yaroze.DrawSync(0)
	for i := 0; i < 80*16/2; i++ {
		word(img[i])
		if i%8 == 7 {
			println()
		} else {
			print(" ")
		}
	}
}
