//go:build yaroze

// Command storeimage is a probe for the spec: StoreImage of images whose
// word count is or is not a whole number of 16-word blocks, and LoadImage
// of the same sizes, each call between two GP1 marker words (10000100h
// plus the case number) so the I/O trace shows the order of the CPU reads
// against the DMA and the time of the call. Video memory at (640, 0),
// 128 by 32, holds a pattern (each pixel its own index, 1000h plus 128
// times the row plus the column) loaded first, and each StoreImage reads
// a rectangle of it into a buffer filled with 5a5a5a5ah; the TTY shows
// the buffer's words against the ones expected from the pattern, so the
// memory words the CPU reads fill, and those the DMA fills, are known.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

// mark writes a GP1 marker word. A GP1 10h whose low bits are 2 to 5, 7
// or 8 asks the GPU for information, answered through GPUREAD in place of
// the next word of an image being read, so case numbers keep clear of
// them: the first probe run lost the first word of each such case to the
// marker before its call.
func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var nibbles = []uint32{0, 1, 6, 9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf}

// caseNum is the marker number of case i.
func caseNum(i int) uint32 {
	return uint32(i/len(nibbles))<<4 | nibbles[i%len(nibbles)]
}

var hex = "0123456789abcdef"

func word(w uint32) {
	for i := range 8 {
		d := w >> (28 - 4*i) & 15
		print(hex[d : d+1])
	}
}

const (
	px, py = 640, 0
	pw, ph = 128, 32
	fill   = 0x5a5a5a5a
)

// pattern is the image at (px, py), and buf what StoreImage reads back.
var pattern [pw * ph / 2]uint32
var buf [pw * ph / 2]uint32

func pixel(x, y int) uint32 { return 0x1000 + uint32(y*pw+x) }

type scase struct {
	w, h int16
}

// cases: the word count is w*h/2 (rounded up), so 8 words (the spec's
// 16 by 1 palette), the counts just below, at and just above whole blocks
// of 16, several remainders r of 16k+r, and whole blocks of 1, 2, 3, 4, 16
// and 128.
var cases = []scase{
	{2, 1},    // 1 word
	{16, 1},   // 8
	{30, 1},   // 15
	{32, 1},   // 16, one block
	{34, 1},   // 17
	{33, 1},   // 17, odd pixels
	{48, 1},   // 24
	{62, 1},   // 31
	{64, 1},   // 32, two blocks
	{66, 1},   // 33
	{22, 3},   // 33, three rows
	{94, 1},   // 47
	{96, 1},   // 48, three blocks
	{64, 2},   // 64, four blocks
	{102, 5},  // 255
	{64, 8},   // 256, sixteen blocks
	{128, 32}, // 2048, the whole pattern
}

func store(n *int, c scase) {
	for i := range len(buf) {
		buf[i] = fill
	}
	r := yaroze.RECT{X: px, Y: py, W: c.w, H: c.h}
	words := (int(c.w)*int(c.h) + 1) / 2
	mark(caseNum(*n))
	yaroze.StoreImage(&r, &buf[0])
	mark(caseNum(*n))
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	print("store case ", *n, " ", c.w, "x", c.h, " ", words, " words:")
	bad, first := 0, -1
	for i := range words {
		k := 2 * i
		want := pixel(k%int(c.w), k/int(c.w))
		if k+1 < int(c.w)*int(c.h) {
			want |= pixel((k+1)%int(c.w), (k+1)/int(c.w)) << 16
		}
		if buf[i] != want {
			bad++
			if first < 0 {
				first = i
			}
		}
	}
	print(" bad ", bad, " first ", first)
	if words < len(buf) {
		print(" after ", buf[words] == fill)
	}
	println()
	if words <= 64 || bad > 0 {
		for i := 0; i < words; i++ {
			word(buf[i])
			if i%8 == 7 || i == words-1 {
				println()
			} else {
				print(" ")
			}
		}
	}
	*n++
}

func load(n *int, c scase) {
	r := yaroze.RECT{X: px, Y: py + 256, W: c.w, H: c.h}
	words := (int(c.w)*int(c.h) + 1) / 2
	mark(caseNum(*n))
	yaroze.LoadImage(&r, &pattern[0])
	mark(caseNum(*n))
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	println("load case ", *n, " ", c.w, "x", c.h, " ", words, " words")
	*n++
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	for y := range ph {
		for x := 0; x < pw; x += 2 {
			pattern[(y*pw+x)/2] = pixel(x, y) | pixel(x+1, y)<<16
		}
	}
	r := yaroze.RECT{X: px, Y: py, W: pw, H: ph}
	yaroze.LoadImage(&r, &pattern[0])
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	n := 0
	for _, c := range cases {
		store(&n, c)
	}
	for _, c := range cases {
		load(&n, c)
	}
	mark(0xff)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
