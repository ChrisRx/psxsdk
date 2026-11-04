//go:build yaroze

// Command kanji is a probe for the spec: Krom2Tim and Krom2Tim2 over
// Shift JIS strings, colors and positions, printing each result and the
// TIM they write, and the Kanji print streams, each case between two GP1
// marker words (10000100h plus the case number), so the GPU stream shows
// what each call sends.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

func cstr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

const fill = 0x5a5a5a5a

// tim is big enough for 40 characters past the size the manual gives.
var tim [(128*40 + 84 + 256) / 4]uint32

var hex = "0123456789abcdef"

func word(w uint32) {
	var b [8]byte
	for i := range b {
		b[i] = hex[w>>(28-4*i)&15]
	}
	print(string(b[:]))
}

// dump prints how far the call wrote (the last word not left at fill),
// the first 32 words in full, and a hash of all words written.
func dump() {
	last := -1
	for i, w := range tim {
		if w != fill {
			last = i
		}
	}
	print(" last ", last, "\n")
	for i := 0; i < 32 && i <= last; i++ {
		word(tim[i])
		if i%8 == 7 {
			println()
		} else {
			print(" ")
		}
	}
	h := uint32(2166136261)
	for i := 0; i <= last; i++ {
		h = (h ^ tim[i]) * 16777619
	}
	print("\nhash ")
	word(h)
	println()
}

type kcase struct {
	s              string
	dx, dy, cx, cy int32
	fg, bg         uint32
}

var strs = []string{
	"\x82\xa0",                 // a hiragana
	"\x82\xa0\x82\xa2\x82\xa4", // three hiragana
	"\x81\x40",                 // full-width space
	"\x82\x60\x82\x81\x82\x4f", // full-width A, a, 0
	"\x88\x9f",                 // first level 1 kanji
	"\x98\x72",                 // last level 1 kanji
	"\x98\x9f",                 // first level 2 kanji
	"\xea\xa4",                 // last level 2 kanji
	"\x84\x40",                 // Cyrillic
	"\x85\x40",                 // an empty row
	"\x81\xad",                 // an empty cell in a used row
	"\xeb\x40",                 // past the last row
	"\xf0\x40",                 // user-defined area
	"\x82\x3f",                 // a trail byte below 40h
	"\x82\x7f",                 // the trail byte 7Fh
	"\x82\xfd",                 // a trail byte past FCh
	"A",                        // ASCII
	"\x82\xa0A\x82\xa2",        // ASCII between two
	"\xa1",                     // half-width katakana
	"\x82",                     // a lead byte alone
	"",                         // empty
	"\x82\xa0\x82\xa2\x82\xa4\x82\xa6\x82\xa8\x82\xa9\x82\xab\x82\xad" +
		"\x82\xaf\x82\xb1\x82\xb3\x82\xb5\x82\xb7\x82\xb9\x82\xbb\x82\xbd", // 16
	"\x82\xa0\x82\xa2\x82\xa4\x82\xa6\x82\xa8\x82\xa9\x82\xab\x82\xad" +
		"\x82\xaf\x82\xb1\x82\xb3\x82\xb5\x82\xb7\x82\xb9\x82\xbb\x82\xbd\x82\xbf", // 17
}

var places = []kcase{
	{dx: 640, dy: 0, cx: 0, cy: 480, fg: 0x7fff, bg: 0},
	{dx: 641, dy: 3, cx: 17, cy: 481, fg: 0x001f, bg: 0x8000},
	{dx: 0, dy: 0, cx: 0, cy: 0, fg: 0, bg: 0},
	{dx: 1020, dy: 510, cx: 1008, cy: 511, fg: 0xffff, bg: 0x7c00},
	{dx: 640, dy: 0, cx: 0, cy: 480, fg: 0x12345, bg: 0xfedcba98},
}

func krom(n *uint32, name string, two bool) {
	for si, s := range strs {
		for pi, p := range places {
			if pi > 0 && si != 1 {
				continue
			}
			for i := range tim {
				tim[i] = fill
			}
			mark(*n)
			r := call(two, cstr(s), p)
			mark(*n)
			print(name, " case ", *n, " str ", si, " place ", pi, " -> ", r)
			dump()
			*n++
		}
	}
}

func screen() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
}

func stream(n *uint32, label string, f func()) {
	mark(*n)
	print("stream case ", *n, " ", label, ":")
	f()
	yaroze.DrawSync(0)
	mark(*n)
	println()
	*n++
}

func main() {
	screen()
	var n uint32
	krom(&n, "Krom2Tim", false)
	krom(&n, "Krom2Tim2", true)

	text := cstr("\x82\xa0\x82\xa2\x82\xa4\x8a\xbf\x8e\x9a")
	var id int32
	stream(&n, "open", func() {
		id = yaroze.KanjiFntOpen(8, 196, 200, 20, 768, 256, 768, 504, 0, 16)
		print(" id ", id)
	})
	stream(&n, "print", func() { print(" ", yaroze.KanjiFntPrint(text)) })
	stream(&n, "flush", func() { print(" ", yaroze.KanjiFntFlush(id) != nil) })
	stream(&n, "flush empty", func() { print(" ", yaroze.KanjiFntFlush(id) != nil) })
	stream(&n, "print flush again", func() {
		print(" ", yaroze.KanjiFntPrint(text), " ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "print other", func() {
		print(" ", yaroze.KanjiFntPrint(cstr("\x88\x9fA\x82\x60")), " ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "newline and wrap", func() {
		print(" ", yaroze.KanjiFntPrint(cstr("\x82\xa0\n\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2\x82\xa0\x82\xa2")))
		print(" ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "past n", func() {
		for range 3 {
			print(" ", yaroze.KanjiFntPrint(text))
		}
		print(" ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "percent", func() {
		print(" ", yaroze.KanjiFntPrint(cstr("%d\x82\xa0%%")), " ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "second stream isbg", func() {
		id2 := yaroze.KanjiFntOpen(8, 40, 64, 64, 768, 0, 768, 505, 1, 8)
		print(" id ", id2, " ", yaroze.KanjiFntPrint(text))
		print(" ", yaroze.KanjiFntFlush(id2) != nil)
	})
	stream(&n, "print goes to", func() {
		print(" ", yaroze.KanjiFntPrint(cstr("\x82\xa0")))
		print(" ", yaroze.KanjiFntFlush(id) != nil, " ", yaroze.KanjiFntFlush(1) != nil)
	})
	stream(&n, "close", func() { yaroze.KanjiFntClose() })
	stream(&n, "reopen", func() {
		id = yaroze.KanjiFntOpen(0, 0, 320, 240, 832, 256, 832, 504, 1, 64)
		print(" id ", id, " ", yaroze.KanjiFntPrint(text), " ", yaroze.KanjiFntFlush(id) != nil)
	})
	stream(&n, "with ascii font", func() {
		yaroze.FntLoad(960, 256)
		fid := yaroze.FntOpen(16, 16, 128, 32, 0, 32)
		print(" fid ", fid, " ", yaroze.FntPrint(cstr("AB")), " ", yaroze.KanjiFntPrint(text))
		print(" ", yaroze.FntFlush(fid) != nil, " ", yaroze.KanjiFntFlush(id) != nil)
	})
	println("done")
	for {
		yaroze.VSync(0)
	}
}

func call(two bool, s *byte, p kcase) int32 {
	if two {
		return yaroze.Krom2Tim2(s, &tim[0], p.dx, p.dy, p.cx, p.cy, p.fg, p.bg)
	}
	return yaroze.Krom2Tim(s, &tim[0], p.dx, p.dy, p.cx, p.cy, p.fg, p.bg)
}
