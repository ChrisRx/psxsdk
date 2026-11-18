//go:build yaroze

// Command kanjilimits is a probe for the spec beside spec/probes/kanji:
// the limits of the Kanji print streams. Which n KanjiFntOpen refuses,
// alone and summed over the streams; how many characters one
// KanjiFntFlush draws (64 to 302 in one stream, with special characters
// after them); and the streams' text pool, found by opening a second
// stream right where the pool is thought to end, printing past it into
// the first and flushing the second, whose glyphs are the pool's bytes at
// its offset. Each case sits between two GP1 marker words (10000100h plus
// its number) and prints its results; a pool case prints a hash of each
// cell of the glyph image read back from video memory. The case that
// crashes Sony's library comes last, its line printed before the calls.
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

// buf holds the text for a print, NUL-terminated.
var buf [8*1024 + 64]byte

// kana is the trail byte of character i of the probe's text: hiragana
// 82A0h to 82D1h, so that a cell's glyph tells which character it is.
func kana(i int) byte { return byte(0xa0 + i%50) }

// text fills buf with count characters, the first being character from
// of the probe's text, and returns it.
func text(from, count int) *byte {
	return textS(from, count, "")
}

// textS is text with suffix after the characters.
func textS(from, count int, suffix string) *byte {
	for i := range count {
		buf[2*i] = 0x82
		buf[2*i+1] = kana(from + i)
	}
	copy(buf[2*count:], suffix)
	buf[2*count+len(suffix)] = 0
	buf[2*count+len(suffix)+1] = 0
	return &buf[0]
}

const (
	// The glyph image's place and the palette's.
	dx, dy = 768, 0
	cx, cy = 896, 504
	// The window: 64 cells to a row, 64 rows, so no text here ends at
	// the window's edge.
	w, h = 1024, 1024
)

func open(y, n int32) int32 {
	return yaroze.KanjiFntOpen(0, y, w, h, dx, dy, cx, cy, 0, n)
}

// img is the glyph image read back: two rows of 16 cells.
var img [64 * 32 / 2]uint32

// cells reads back the first cells of the glyph image, two rows of 16,
// and prints a hash of each of the first count cells.
func cells(count int) {
	for i := range len(img) {
		img[i] = 0x5a5a5a5a
	}
	r := yaroze.RECT{X: dx, Y: dy, W: 64, H: 32}
	yaroze.StoreImage(&r, &img[0])
	yaroze.DrawSync(0)
	print(" cells")
	for c := range count {
		hash := uint32(2166136261)
		for row := range 16 {
			i := (c/16*16+row)*32 + c%16*2
			hash = (hash ^ img[i]) * 16777619
			hash = (hash ^ img[i+1]) * 16777619
		}
		print(" ")
		word(hash)
	}
	println()
}

// clearImage clears the glyph image's first two rows to a known color.
func clearImage() {
	r := yaroze.RECT{X: dx, Y: dy, W: 64, H: 32}
	yaroze.ClearImage(&r, 0x50, 0x50, 0x50)
	yaroze.DrawSync(0)
}

var caseNo uint32

func begin(label string) {
	yaroze.KanjiFntClose()
	clearImage()
	mark(caseNo)
	print("case ", caseNo, " ", label, ":")
}

func end() {
	yaroze.DrawSync(0)
	mark(caseNo)
	println()
	caseNo++
}

// opens opens streams with the given n values and prints the ids.
func opens(label string, ns ...int32) {
	begin(label)
	for _, k := range ns {
		print(" ", open(0, k))
	}
	end()
}

// flushCount prints count characters into one stream of n and flushes
// it: the GPU words carry a quadrangle for each character drawn and the
// glyph image's rectangle.
func flushCount(label string, n int32, count int) {
	flushCountS(label, n, count, "")
}

// flushCountS is flushCount with suffix printed after the characters.
func flushCountS(label string, n int32, count int, suffix string) {
	begin(label)
	print(" n ", n, " count ", count, " suffix ", len(suffix), " id ", open(0, n))
	print(" ", yaroze.KanjiFntPrint(textS(0, count, suffix)))
	print(" ", yaroze.KanjiFntFlush(0) != nil)
	end()
}

// pool opens stream A of n a and stream B of n b right after it, prints
// count characters into A and flushes B: B's glyphs are the pool's bytes
// from offset a, up to the NUL.
func pool(label string, a, b int32, count int) {
	begin(label)
	print(" ids ", open(0, a), " ", open(512, b))
	print(" ", yaroze.KanjiFntPrint(text(0, count)))
	print(" B ", yaroze.KanjiFntFlush(1) != nil)
	end()
	cells(17)
}

// ref prints the 16 characters from character from into a stream of n
// 16 and flushes it, for the cell hashes a pool case should show.
func ref(label string, from int) {
	begin(label)
	print(" id ", open(0, 16))
	print(" ", yaroze.KanjiFntPrint(text(from, 16)))
	print(" ", yaroze.KanjiFntFlush(0) != nil)
	end()
	cells(17)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)

	// KanjiFntOpen: whether n, alone or summed over the streams, is
	// ever refused, and whether a refused open or a negative n counts.
	opens("opens 1024 1", 1024, 1)
	opens("opens 1023 1", 1023, 1)
	opens("opens 1023 0", 1023, 0)
	opens("opens 1022 1", 1022, 1)
	opens("opens 1025 1", 1025, 1)
	opens("opens 0 1024", 0, 1024)
	opens("opens 512 512 1", 512, 512, 1)
	opens("opens 512 512 512", 512, 512, 512)
	opens("opens 1016 8", 1016, 8)
	opens("opens 1016 7 1", 1016, 7, 1)
	opens("opens 2048 1", 2048, 1)
	opens("opens 65536 1", 65536, 1)
	opens("opens 1<<30 1 1 1 1", 1<<30, 1, 1, 1, 1)
	opens("opens -1 1", -1, 1)
	opens("opens -100 1100", -100, 1100)
	opens("opens -100 1123", -100, 1123)
	opens("opens -100 1124", -100, 1124)
	opens("opens 256 x5", 256, 256, 256, 256, 256)

	// A stream of a large or a negative n.
	flushCount("n 1023 10", 1023, 10)
	flushCount("n -1 10", -1, 10)
	flushCount("n -100 10", -100, 10)

	// The pool's bytes past a stream's n, the control: A holds 20
	// characters, B at offset 16 sees characters 8 on.
	ref("ref 8", 8)
	pool("pool 16 20", 16, 16, 20)
	ref("ref 504", 504)

	// How many characters one flush draws: the counts up to 301 and,
	// after them, special characters that are read but not drawn.
	for _, c := range []int{64, 100, 127, 128, 129, 200, 256, 300, 301} {
		flushCount("flush", 600, c)
	}
	flushCount("flush n 1023", 1023, 301)
	flushCountS("flush space", 1023, 301, " ")
	flushCountS("flush newline", 1023, 301, "\n")
	flushCountS("flush tilde", 1023, 301, "~")
	flushCountS("flush tab", 1023, 301, "\t")
	flushCountS("flush spaces", 1023, 301, "       ")
	flushCountS("flush newlines", 1023, 301, "\n\n\n\n")

	// The pool's end: B of n 15 right at offset 1008, A's text printed
	// past 1024, and a flush after each.
	pool("pool 1008 520", 1008, 15, 520)
	flushCount("flush after 520", 600, 64)
	pool("pool 1008 1000", 1008, 15, 1000)
	flushCount("flush after 1000", 600, 64)

	// A flush of 302 characters, which Sony's library does not survive.
	flushCount("flush n 1023", 1023, 302)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
