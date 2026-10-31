//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The Kanji font under both libraries: a window of ASCII and Shift JIS,
// flushed to the screen, and strings rendered to TIMs.

//export KanjiFntOpen_orig
func KanjiFntOpen_orig(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32

//export KanjiFntPrint_orig
func KanjiFntPrint_orig(s *byte) int32

//export KanjiFntFlush_orig
func KanjiFntFlush_orig(id int32) *uint32

//export KanjiFntClose_orig
func KanjiFntClose_orig()

//export Krom2Tim_orig
func Krom2Tim_orig(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32

//export Krom2Tim2_orig
func Krom2Tim2_orig(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32

//export Krom2RawAdd_orig
func Krom2RawAdd_orig(code uint32) uint32

//export Krom2RawAdd2_orig
func Krom2RawAdd2_orig(code uint32) uint32

type libKanji struct {
	open      func(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32
	print     func(*byte) int32
	flush     func(int32) *uint32
	close     func()
	krom2Tim  func(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32
	krom2Tim2 func(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32
	rawAdd    func(uint32) uint32
	rawAdd2   func(uint32) uint32
}

func kanjiFor(mode int32) libKanji {
	if mode == 0 {
		return libKanji{
			open: func(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32 {
				return KanjiFntOpen_orig(x, y, w, h, dx, dy, cx, cy, isbg, n)
			},
			print: func(s *byte) int32 { return KanjiFntPrint_orig(s) },
			flush: func(id int32) *uint32 { return KanjiFntFlush_orig(id) },
			close: func() { KanjiFntClose_orig() },
			krom2Tim: func(s *uint8, t *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
				return Krom2Tim_orig(s, t, dx, dy, cdx, cdy, fg, bg)
			},
			krom2Tim2: func(s *uint8, t *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
				return Krom2Tim2_orig(s, t, dx, dy, cdx, cdy, fg, bg)
			},
			rawAdd:  func(c uint32) uint32 { return Krom2RawAdd_orig(c) },
			rawAdd2: func(c uint32) uint32 { return Krom2RawAdd2_orig(c) },
		}
	}
	return libKanji{
		open: func(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32 {
			return yaroze.KanjiFntOpen(x, y, w, h, dx, dy, cx, cy, isbg, n)
		},
		print: func(s *byte) int32 { return yaroze.KanjiFntPrint(s) },
		flush: func(id int32) *uint32 { return yaroze.KanjiFntFlush(id) },
		close: func() { yaroze.KanjiFntClose() },
		krom2Tim: func(s *uint8, t *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
			return yaroze.Krom2Tim(s, t, dx, dy, cdx, cdy, fg, bg)
		},
		krom2Tim2: func(s *uint8, t *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
			return yaroze.Krom2Tim2(s, t, dx, dy, cdx, cdy, fg, bg)
		},
		rawAdd:  func(c uint32) uint32 { return Krom2RawAdd(c) },
		rawAdd2: func(c uint32) uint32 { return Krom2RawAdd2(c) },
	}
}

// The Go library's address lookups, which the bindings do not offer.

//export Krom2RawAdd
func Krom2RawAdd(code uint32) uint32

//export Krom2RawAdd2
func Krom2RawAdd2(code uint32) uint32

// sumChain sums the packets linked from the ordering table entry ot,
// each by its length and the words after its tag, as sumPackets does.
func sumChain(ot *uint32) uint32 {
	s := uint32(0)
	for p := *ot & 0xffffff; p != 0xffffff; {
		w := (*uint32)(unsafe.Pointer(uintptr(p | 0x80000000)))
		n := *w >> 24
		s = s*31 + n
		for _, x := range unsafe.Slice(w, n+1)[1:] {
			s = s*31 + x
		}
		p = *w & 0xffffff
	}
	return s
}

var (
	kanjiTIM  [16 + 4*32 + 8]uint32
	kanjiTIM2 [16 + 2*512 + 8]uint32
)

// kanjiScript draws a window of text: ASCII, hiragana, katakana and
// kanji, with a proportional run and a tab, and renders strings to
// TIMs, a row, two rows, a bad code, and user-defined characters.
func kanjiScript(mode int32) {
	K := kanjiFor(mode)
	L := libFor(mode)
	// The texture at 640,256 and the palette at 640,480, the window at
	// 16,16 with a background.
	print("kanji open ", K.open(16, 16, 288, 64, 640, 256, 640, 480, 1, 64), "\n")
	text := []byte("ABC abc 19 %%\n~pfoo \x8a\xbf\x8e\x9a\x82\xa9\x82\xc8\x83\x63\x83\x69\tz\x00")
	print("kanji print ", K.print(&text[0]), "\n")
	ot := K.flush(-1)
	print("kanji flush ", sumChain(ot), "\n")
	L.drawSync(0)
	// The texture's area in video memory holds what the original's stack
	// held beyond the glyphs, so it is cleared before the comparison.
	rect := yaroze.RECT{X: 640, Y: 256, W: 256, H: 32}
	print("kanji clear ", L.clearImage(&rect, 0, 0, 0), "\n")
	// A byte the ROM has not: nothing drawn.
	bad := []byte("x\x7f\x00")
	print("kanji print2 ", K.print(&bad[0]), "\n")
	print("kanji bad ", K.flush(-1) == nil, "\n")
	K.close()
	L.drawSync(0)

	s := []byte("\x8a\xbf\x8e\x9aAB\x00")
	print("krom2tim ", K.krom2Tim(&s[0], &kanjiTIM[0], 320, 0, 320, 500, 0x7fff, 0), " ", sum32words(kanjiTIM[:]), "\n")
	wide := []byte("ABCDEFGHIJKLMNOPQ\x00")
	print("krom2tim wide ", K.krom2Tim(&wide[0], &kanjiTIM2[0], 0, 0, 0, 0, 0x03e0, 0x7c00), " ", sum32words(kanjiTIM2[:]), "\n")
	rowed := []byte("AB\nC\x00")
	print("krom2tim rows ", K.krom2Tim(&rowed[0], &kanjiTIM2[0], 0, 0, 0, 0, 1, 2), " ", sum32words(kanjiTIM2[:]), "\n")
	badcode := []byte("\x99\x40\x00")
	print("krom2tim bad ", K.krom2Tim(&badcode[0], &kanjiTIM[0], 0, 0, 0, 0, 0, 0), "\n")
	user := []byte("\x85\x40\x85\x41A\x01B\x00")
	print("krom2tim2 ", K.krom2Tim2(&user[0], &kanjiTIM[0], 0, 0, 0, 0, 0x7fff, 0), " ", sum32words(kanjiTIM[:]), "\n")
	print("krom2rawadd ", K.rawAdd(0x8140), " ", K.rawAdd2(0x8540), " ", K.rawAdd2(0x9999), " ", K.rawAdd2(0x889f), "\n")
}
