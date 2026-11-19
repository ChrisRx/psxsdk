//go:build yaroze

// Command kanjinoglyph is a probe for spec 51: Krom2Tim and the Kanji flush on
// codes of the non-kanji range that have no glyph, beside strings of the same
// shape made of glyphs. Each call sits between two GP1 marker words (10000100h
// plus the case number), so a bus trace gives its time. Before a Krom2Tim call
// the stack below sp is filled with words that hold their own place, and after
// it the words that changed are hashed with their places, in four ranges, for
// what the call leaves there. It returns.
package main

import (
	"device"
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

var buf [128]byte

func cstr(s string) *byte {
	buf = [128]byte{}
	copy(buf[:], s)
	return &buf[0]
}

const (
	kan = "\x82\xa0"
	no  = "\x81\xad" // an unassigned cell
)

func rep(s string, n int) string {
	r := ""
	for range n {
		r += s
	}
	return r
}

type kcase struct {
	label string
	s     string
}

var kroms = []kcase{
	{"no 817f", "\x81\x7f"},
	{"no 81ad", "\x81\xad"},
	{"no 81c0", "\x81\xc0"},
	{"no 81d0", "\x81\xd0"},
	{"no 81ea", "\x81\xea"},
	{"no 81f8", "\x81\xf8"},
	{"no 81fd", "\x81\xfd"},
	{"no 8200", "\x82\x00"},
	{"no 823f", "\x82\x3f"},
	{"no 8240", "\x82\x40"},
	{"no 8259", "\x82\x59"},
	{"no 827a", "\x82\x7a"},
	{"no 827f", "\x82\x7f"},
	{"no 829b", "\x82\x9b"},
	{"no 82f2", "\x82\xf2"},
	{"no 82fd", "\x82\xfd"},
	{"no 837f", "\x83\x7f"},
	{"no 8397", "\x83\x97"},
	{"no 83b7", "\x83\xb7"},
	{"no 83d7", "\x83\xd7"},
	{"no 8461", "\x84\x61"},
	{"no 847f", "\x84\x7f"},
	{"no 8492", "\x84\x92"},
	{"glyph 8140", "\x81\x40"},
	{"glyph 8180", "\x81\x80"},
	{"glyph 81b8", "\x81\xb8"},
	{"glyph 81c8", "\x81\xc8"},
	{"glyph 81da", "\x81\xda"},
	{"glyph 81f0", "\x81\xf0"},
	{"glyph 81fc", "\x81\xfc"},
	{"glyph 824f", "\x82\x4f"},
	{"glyph 8260", "\x82\x60"},
	{"glyph 8281", "\x82\x81"},
	{"glyph 82a0", "\x82\xa0"},
	{"glyph 8340", "\x83\x40"},
	{"glyph 8380", "\x83\x80"},
	{"glyph 839f", "\x83\x9f"},
	{"glyph 83bf", "\x83\xbf"},
	{"glyph 8440", "\x84\x40"},
	{"glyph 8470", "\x84\x70"},
	{"glyph 8480", "\x84\x80"},
	{"glyph 849f", "\x84\x9f"},
	{"glyph 84be", "\x84\xbe"},
	{"lead alone", "\x82"},
	{"kanji bad trail 897f", "\x89\x7f"},
	{"3 glyphs", kan + kan + kan},
	{"glyph no glyph", kan + no + kan},
	{"3 no", no + no + no},
	{"16 glyphs", rep(kan, 16)},
	{"16 no", rep(no, 16)},
	{"8 and 8", rep(kan+no, 8)},
	{"17 glyphs", rep(kan, 17)},
	{"17 no", rep(no, 17)},
	{"nl glyphs", kan + "\n" + kan + kan},
	{"nl no", no + "\n" + no + no},
	{"no then bad", no + "\x85\x40"},
	{"glyph then bad", kan + "\x85\x40"},
}

var streams = []kcase{
	{"glyph", kan},
	{"81ad", no},
	{"823f", "\x82\x3f"},
	{"lead alone", "\x82"},
	{"3 glyphs", kan + kan + kan},
	{"glyph no glyph", kan + no + kan},
	{"3 no", no + no + no},
	{"16 glyphs", rep(kan, 16)},
	{"16 no", rep(no, 16)},
	{"20 glyphs", rep(kan, 20)},
	{"20 no", rep(no, 20)},
	{"no then bad", no + "\x85\x40"},
	{"glyph then bad", kan + "\x85\x40"},
}

const (
	patTop   = 0xa5000000
	patWords = 20000 // 80,000 bytes below sp
	patSkip  = 64
)

var (
	tim    [1200]uint32
	img    [16 * 2 * 16 * 2]uint32
	baseSP uint32
	result int32
)

// krom fills the stack, makes the call between markers and hashes what
// changed below sp.
//
//go:noinline
func krom(n uint32, two bool) {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil)) &^ 3
	baseSP = sp
	for i := uint32(patSkip); i < patWords; i++ {
		*(*uint32)(yaroze.Pointer(uintptr(sp - 4*i))) = patTop | i
	}
	// From a blank, so that each call has one blank in it.
	yaroze.VSync(0)
	mark(n)
	if two {
		result = yaroze.Krom2Tim2(&buf[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
	} else {
		result = yaroze.Krom2Tim(&buf[0], &tim[0], 640, 0, 0, 480, 0x7fff, 0)
	}
	mark(n)
	// The ranges, in bytes below sp: under the frame, the glyphs' bytes, the
	// image, and the rest up to the words left alone.
	var h [4]uint32
	var c [4]uint32
	for k := range 4 {
		h[k] = 2166136261
	}
	for i := uint32(patSkip); i < patWords; i++ {
		w := *(*uint32)(yaroze.Pointer(uintptr(sp - 4*i)))
		if w == patTop|i {
			continue
		}
		k := 3
		switch {
		case 4*i > 73800:
			k = 0
		case 4*i > 65600:
			k = 1
		case 4*i > 56000:
			k = 2
		}
		h[k] = (h[k] ^ i) * 16777619
		h[k] = (h[k] ^ w) * 16777619
		c[k]++
	}
	hold = h
	held = c
}

var hold, held [4]uint32

func kromCase(n uint32, c kcase, two bool) {
	for i := 0; i < len(tim); i++ {
		tim[i] = 0x5a5a5a5a
	}
	cstr(c.s)
	krom(n, two)
	if two {
		print("Krom2Tim2")
	} else {
		print("Krom2Tim")
	}
	print(" case ", n, " ", c.label, " -> ", result)
	h := uint32(2166136261)
	last := -1
	for i := 0; i < len(tim); i++ {
		if tim[i] != 0x5a5a5a5a {
			last = i
		}
	}
	for i := 0; i <= last; i++ {
		h = (h ^ tim[i]) * 16777619
	}
	print(" last ", last, " tim ")
	word(h)
	print(" stack")
	for k := range 4 {
		print(" ", held[k], ":")
		word(hold[k])
	}
	println()
}

func streamCase(n uint32, c kcase, isbg int32) {
	yaroze.KanjiFntClose()
	rect := yaroze.RECT{X: 768, Y: 256, W: 64, H: 32}
	yaroze.ClearImage(&rect, 0x50, 0x50, 0x50)
	yaroze.DrawSync(0)
	id := yaroze.KanjiFntOpen(8, 8, 300, 100, 768, 256, 768, 504, isbg, 64)
	p := yaroze.KanjiFntPrint(cstr(c.s))
	yaroze.VSync(0)
	mark(n)
	ok := yaroze.KanjiFntFlush(id) != nil
	mark(n)
	yaroze.DrawSync(0)
	for i := 0; i < len(img); i++ {
		img[i] = 0x5a5a5a5a
	}
	yaroze.StoreImage(&rect, &img[0])
	yaroze.DrawSync(0)
	// The first row of cells but the first word, which the read-back does
	// not give the same in the emulator as on a console.
	h := uint32(2166136261)
	for i := 1; i < 16*32; i++ {
		h = (h ^ img[i]) * 16777619
	}
	print("flush case ", n, " ", c.label, " isbg ", isbg, " print ", p, " -> ", ok, " image ")
	word(h)
	println()
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	println("start")
	n := uint32(0)
	for i := 0; i < len(kroms); i++ {
		kromCase(n, kroms[i], false)
		n++
	}
	for i := 0; i < len(kroms); i++ {
		kromCase(n, kroms[i], true)
		n++
	}
	for i := 0; i < len(streams); i++ {
		streamCase(n, streams[i], 0)
		n++
	}
	for i := 0; i < len(streams); i++ {
		streamCase(n, streams[i], 1)
		n++
	}
	println("END")
	// A moment before the return, for a harness that looks for the END now
	// and then: the return prints more.
	for i := 0; i < 200000; i++ {
		spin.Set(uint32(i))
	}
}

var spin volatile.Register32
