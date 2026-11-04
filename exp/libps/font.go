//go:build yaroze

package libps

//go:generate go run ./gen/font

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The font: libgpu's text streams. FntLoad puts the font into video
// memory, FntOpen opens a window of up to n characters, FntPrint adds
// text to it, and FntFlush draws what was printed, each character an
// 8 by 8 sprite, with the window's background first when it has one.

// A fontStream is one window: its background tile, the drawing mode its
// sprites are drawn in, and the text and sprites it was given.
//
// libps 0x800476D8, 8 by 48 bytes
type fontStream struct {
	tile    tilePacket
	mode    modePacket
	max     int32   // the characters it holds
	sprites *uint32 // its sprite packets, 4 words each
	text    *byte   // its text
	n       int32   // the characters printed so far
	// autosize is set for a window opened with no width: it does not
	// wrap, and its background is sized to the text when flushed.
	autosize int32
}

// A tilePacket is a GPU tile: a filled rectangle at an offset.
type tilePacket struct {
	Tag  uint32
	R    uint8
	G    uint8
	B    uint8
	Code uint8
	X    int16
	Y    int16
	W    int16
	H    int16
}

// A modePacket is a drawing mode: the texture page and window.
type modePacket struct {
	Tag  uint32
	Code [2]uint32
}

const fontStreams = 8

// fontClut is the font's palette: a transparent color and white.
var fontClut = [8]uint32{0xffff0000}

var (
	fontStream0 [fontStreams]fontStream
	fontCount   int32  // libps 0x80047858: streams opened
	fontDump    int32  // libps 0x8004785C: the stream FntPrint takes by default
	fontUsed    int32  // libps 0x80048260: characters given out to streams
	fontTPage   uint16 // libps 0x8005416C
	fontClutID  uint16 // libps 0x80054170
	// fontOpenPace is what FntOpen's loop counts, to take as long as the
	// original's did.
	fontOpenPace [2]uint32
	fontText     [1024]byte       // libps 0x8004FD6C
	fontSprites  [1024 * 4]uint32 // libps 0x8005016C: 1024 sprite packets
)

// FntLoad puts the font at tx, ty in video memory, 128 by 32 pixels of
// 4-bit color, with its palette 128 lines below, and closes every window.
//
// libps 0x8002527C, 164 bytes
//
//export FntLoad
func FntLoad(tx, ty int32) {
	fontClutID = loadClut(&fontClut[0], tx, ty+128)
	fontTPage = loadTPage(&fontPixels[0], 0, 0, tx, ty, 128, 32)
	fontCount = 0
	fontStream0 = [fontStreams]fontStream{}
}

// loadClut puts a palette of sixteen colors at x, y and returns its id.
//
// libps 0x80025E30, 104 bytes
func loadClut(clut *uint32, x, y int32) uint16 {
	r := RECT{int16(x), int16(y), 16, 1}
	LoadImage(&r, clut)
	return GetClut(x, y)
}

// loadTPage puts a w by h image of depth tp at x, y, counting the width in
// pixels, and returns its texture page value.
//
// libps 0x80025CDC, 236 bytes
func loadTPage(pix *uint32, tp, abr, x, y, w, h int32) uint16 {
	r := RECT{int16(x), int16(y), int16(w), int16(h)}
	switch tp {
	case 0:
		r.W = int16(w >> 2)
	case 1:
		r.W = int16(w >> 1)
	}
	LoadImage(&r, pix)
	return GetTPage(tp, abr, x, y)
}

// GetClut is the palette id for a palette at x, y.
//
// libps 0x800274CC, 24 bytes
//
//export GetClut
func GetClut(x, y int32) uint16 {
	return uint16(y<<6 | (x>>4)&0x3f)
}

// FntOpen opens a window at x, y of w by h for up to n characters, with a
// background when isbg is set (2 for a see-through one), and returns its
// id, or -1 when eight are open. A window of no width does not wrap. The
// windows share 1024 characters between them.
//
// libps 0x80025320, 696 bytes
//
//export FntOpen
func FntOpen(x, y, w, h, isbg, n int32) int32 {
	if fontCount >= fontStreams {
		return -1
	}
	if fontCount == 0 {
		fontUsed = 0
	}
	id := fontCount
	s := &fontStream0[id]
	if w == 0 {
		s.autosize = 1
	} else {
		s.autosize = 0
	}
	if fontUsed+n > 1024 {
		n = 1024 - fontUsed
	}
	tw := RECT{0, 0, 256, 256}
	setDrawMode(&s.mode, 0, 0, fontTPage, &tw)
	if isbg != 0 {
		s.tile.Tag = 3 << 24
		s.tile.Code = 0x60
		s.tile.R, s.tile.G, s.tile.B = 0, 0, 0
		if isbg == 2 {
			s.tile.Code |= 2
		} else {
			s.tile.Code &^= 2
		}
	}
	s.tile.X, s.tile.Y, s.tile.W, s.tile.H = int16(x), int16(y), int16(w), int16(h)
	s.max = n
	s.n = 0
	s.text = &fontText[fontUsed]
	s.sprites = &fontSprites[fontUsed*4]
	*s.text = 0
	sprites := unsafe.Slice(s.sprites, n*4)
	for i := int32(0); i < n; i++ {
		sprites[i*4] = 3 << 24
		sprites[i*4+1] = 0x74 << 24
		*(*uint16)(unsafe.Pointer(&sprites[i*4+3])) = 0
		*(*uint16)(unsafe.Add(unsafe.Pointer(&sprites[i*4+3]), 2)) = fontClutID
		// The original's loop took 28 cycles a character, this one 12:
		// the difference moves a program's first CD commands, and their
		// interrupts, against the vertical blank.
		volatile.StoreUint32(&fontOpenPace[0], volatile.LoadUint32(&fontOpenPace[0])+volatile.LoadUint32(&fontOpenPace[1]))
		volatile.StoreUint32(&fontOpenPace[1], volatile.LoadUint32(&fontOpenPace[1])+1)
	}
	fontUsed += n
	fontCount++
	return id
}

// setDrawMode builds a drawing mode packet.
//
// libps 0x80028F44, 88 bytes
func setDrawMode(p *modePacket, dfe, dtd uint8, tpage uint16, tw *RECT) {
	p.Tag = p.Tag&0xffffff | 2<<24
	p.Code[0] = drawMode(dfe, dtd, tpage)
	p.Code[1] = texWindow(tw)
}

// FntPrint adds s to the window FntPrint prints to, the first opened
// unless SetDumpFnt chose another, and returns how many characters the
// window holds, or -1 when it is full. A %% is one %; the other
// directives the original formatted from arguments it was given, which
// this library's callers never pass, so a % stays as it is.
//
// libps 0x800258FC, 992 bytes
//
//export FntPrint
func FntPrint(str *byte) int32 {
	pace(paceFntPrint)
	id := fontDump
	s := &fontStream0[id]
	if s.text == nil {
		return -1
	}
	if s.n > s.max {
		return -1
	}
	// The count and the limit stay in registers through the loop: Sony's
	// library spends 50 cycles a character here (spec 08, "FntPrint and
	// FntFlush by the character").
	text, n, max := s.text, s.n, s.max
	for p := str; *p != 0; p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1)) {
		c := *p
		if c == '%' && *(*byte)(unsafe.Add(unsafe.Pointer(p), 1)) == '%' {
			p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
		}
		*(*byte)(unsafe.Add(unsafe.Pointer(text), n)) = c
		n++
		volatile.StoreUint32(&paceWord, 0)
		volatile.StoreUint32(&paceWord, 0)
		volatile.StoreUint32(&paceWord, 0)
		volatile.StoreUint32(&paceWord, 0)
		if n > max {
			// The print that cuts: 20 more than the characters kept.
			// Five stores here make the compiler reshape the whole
			// function (every print 40 dearer); three leave it alone.
			volatile.StoreUint32(&paceWord, uint32(n))
			volatile.StoreUint32(&paceWord, uint32(n))
			volatile.StoreUint32(&paceWord, uint32(n))
			s.n = n
			return -1
		}
	}
	*(*byte)(unsafe.Add(unsafe.Pointer(text), n)) = 0
	s.n = n
	return n
}

// FntWrite adds s to window id, as FntPrint adds to the default window,
// for a Go caller with a string: a % is taken as it is. It returns how
// many characters the window holds, or -1 when it is full or not open.
func FntWrite(id int32, s string) int32 {
	return FntWriteBytes(id, unsafe.Slice(unsafe.StringData(s), len(s)))
}

// FntDump is the window FntPrint prints to unless it is given another.
func FntDump() int32 { return fontDump }

// FntAddBytes is FntWriteBytes as FntPrint answers a window that is not
// open: with 0 rather than -1, adding nothing.
func FntAddBytes(id int32, b []byte) int32 {
	if id < 0 || id >= fontCount || fontStream0[id].text == nil {
		return 0
	}
	return FntWriteBytes(id, b)
}

// FntWriteBytes is FntWrite with the text as bytes.
func FntWriteBytes(id int32, b []byte) int32 {
	if id < 0 || id >= fontCount {
		return -1
	}
	s := &fontStream0[id]
	if s.text == nil || s.n > s.max {
		return -1
	}
	text := unsafe.Slice(s.text, 1025)
	for _, c := range b {
		text[s.n] = c
		s.n++
		if s.n > s.max {
			return -1
		}
	}
	text[s.n] = 0
	return s.n
}

// FntFlush draws window id's text, the window FntPrint prints to for an
// id below 0, with the background behind it, and empties it. It returns
// the drawing mode packet that heads what it drew, or nil for a window
// never opened.
//
// libps 0x800255D8, 804 bytes
//
//export FntFlush
//go:nobounds
func FntFlush(id int32) *uint32 {
	pace(paceFntFlush)
	// An id that is not open flushes the default stream, as Sony's does
	// (spec 01).
	if id < 0 || id >= fontCount {
		id = fontDump
	}
	s := &fontStream0[id]
	if s.text == nil {
		return nil
	}
	head := &s.mode.Tag
	x0, y0 := int32(s.tile.X), int32(s.tile.Y)
	x1, y1 := x0+int32(s.tile.W), y0+int32(s.tile.H)
	x, y := x0, y0
	left := s.max
	maxX := int32(0)
	r, g, b := uint8(0x80), uint8(0x80), uint8(0x80)
	*head |= 0xffffff
	sprite := s.sprites
	// Each kind of character takes what Sony's library takes (spec 08,
	// "FntPrint and FntFlush by the character"): 140 cycles a glyph, 34
	// a space, 46 a tab, 62 a newline, 28 more for a line wrap. The loop
	// is kept lean (the text walked by index with bounds checks off, the
	// wrap test hoisted, unsigned glyph arithmetic) and each path then
	// spends the rest in stores to the scratch word.
	wrap := s.autosize == 0
	text := unsafe.Slice(s.text, 1025)
	i := 0
	for ; left != 0; left-- {
		c := text[i]
		if c == 0 {
			break
		}
		// An if-chain, not a switch: the compiler turns a switch on the
		// character into a jump table, nine instructions on every
		// character. Spaces first, then the range of the glyphs.
		if c == ' ' {
			x += 8
			if wrap && x >= x1 {
				// Each wrap's pad stores a different value, or the compiler
				// merges the paths' stores into one tail and the counts
				// stop being independent.
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.StoreUint32(&paceWord, uint32(x))
				volatile.LoadUint32(&paceWord)
				goto wrapspace
			}
			volatile.StoreUint32(&paceWord, 0)
			volatile.StoreUint32(&paceWord, 0)
			volatile.StoreUint32(&paceWord, 0)
			volatile.StoreUint32(&paceWord, 0)
			goto next
		}
		if c > ' ' {
			if c != '~' {
				goto glyph
			}
			// ~cRGB sets the color from three digits; any other
			// escape is skipped with the character after it.
			i++
			if text[i] == 'c' {
				r, g, b = (text[i+1]-'0')<<4, (text[i+2]-'0')<<4, (text[i+3]-'0')<<4
				i += 3
			}
			goto next
		}
		if c == '\n' {
			volatile.StoreUint32(&paceWord, uint32(c))
			volatile.StoreUint32(&paceWord, uint32(c))
			volatile.StoreUint32(&paceWord, uint32(c))
			volatile.StoreUint32(&paceWord, uint32(c))
			volatile.StoreUint32(&paceWord, uint32(c))
			goto newline
		}
		if c == '\t' {
			x += 32
			volatile.StoreUint32(&paceWord, 0)
			volatile.StoreUint32(&paceWord, 0)
			volatile.StoreUint32(&paceWord, 0)
			if wrap && x >= x1 {
				volatile.StoreUint32(&paceWord, uint32(left))
				volatile.StoreUint32(&paceWord, uint32(left))
				volatile.StoreUint32(&paceWord, uint32(left))
				volatile.StoreUint32(&paceWord, uint32(left))
				volatile.StoreUint32(&paceWord, uint32(left))
				goto wraptab
			}
			goto next
		}
		// Any other control character draws as a glyph, as the original
		// did.
	glyph:
		{
			// The cell comes from a table (fontcell.go): Sony's signed
			// division, which places every byte, costs 12 cycles a glyph.
			k := uint32(fontCell[c])
			p := (*[4]uint32)(unsafe.Pointer(sprite))
			p[1] = p[1]&0xff000000 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
			p[2] = uint32(uint16(x)) | uint32(uint16(y))<<16
			p[3] = p[3]&0xffff0000 | k
			addPrim(head, &p[0])
			sprite = (*uint32)(unsafe.Add(unsafe.Pointer(sprite), 16))
			x += 8
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			volatile.StoreUint32(&paceWord, k)
			// An odd number of stores costs 4 for the last, not 2: the
			// compiler's delay-slot filling.
			volatile.LoadUint32(&paceWord)
			if wrap && x >= x1 {
				volatile.StoreUint32(&paceWord, k)
				volatile.StoreUint32(&paceWord, k)
				volatile.StoreUint32(&paceWord, k)
				volatile.StoreUint32(&paceWord, k)
				volatile.StoreUint32(&paceWord, k)
				volatile.StoreUint32(&paceWord, k)
				goto wrapline
			}
		}
	next:
		i++
		continue
	newline:
		if maxX < x {
			maxX = x
		}
		y += 8
		x = x0
		if y >= y1 {
			// Sony's newline that reaches the bottom costs 52, 10 less
			// than one that does not, and ends the flush.
			volatile.StoreUint32(&paceWord, uint32(y))
			volatile.StoreUint32(&paceWord, uint32(y))
			volatile.StoreUint32(&paceWord, uint32(y))
			volatile.StoreUint32(&paceWord, uint32(y))
			volatile.StoreUint32(&paceWord, uint32(y))
			break
		}
		i++
		continue
	wrapline:
		// A wrap's new line: as a newline's, with its own pad at the
		// bottom (Sony's wrap that reaches the bottom costs 18 where its
		// newline costs 52). One copy for each kind of wrap, since the
		// compiler lays out each path's jump differently and the bottom
		// pads come out different; each copy's pad stores its own value.
		if maxX < x {
			maxX = x
		}
		y += 8
		x = x0
		if y >= y1 {
			volatile.StoreUint32(&paceWord, uint32(x1))
			volatile.StoreUint32(&paceWord, uint32(x1))
			volatile.StoreUint32(&paceWord, uint32(x1))
			volatile.LoadUint32(&paceWord)
			break
		}
		i++
		continue
	wrapspace:
		if maxX < x {
			maxX = x
		}
		y += 8
		x = x0
		if y >= y1 {
			volatile.StoreUint32(&paceWord, uint32(x0))
			volatile.StoreUint32(&paceWord, uint32(x0))
			volatile.StoreUint32(&paceWord, uint32(x0))
			volatile.StoreUint32(&paceWord, uint32(x0))
			volatile.StoreUint32(&paceWord, uint32(x0))
			break
		}
		i++
		continue
	wraptab:
		if maxX < x {
			maxX = x
		}
		y += 8
		x = x0
		if y >= y1 {
			volatile.StoreUint32(&paceWord, uint32(y1))
			volatile.StoreUint32(&paceWord, uint32(y1))
			volatile.StoreUint32(&paceWord, uint32(y1))
			volatile.StoreUint32(&paceWord, uint32(y1))
			break
		}
		i++
	}
	if left == 0 {
		// A full stream ends by the count without reading a terminator,
		// which Sony's flush charges like the terminator's exit.
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
		volatile.StoreUint32(&paceWord, uint32(i))
	}
	if s.tile.Code != 0 {
		// The isbg tile costs 40 in Sony's flush (spec 08).
		addPrim(head, &s.tile.Tag)
		volatile.StoreUint32(&paceWord, uint32(s.tile.Code))
		volatile.StoreUint32(&paceWord, uint32(s.tile.Code))
		if s.autosize != 0 {
			s.tile.W = int16(maxX - x0)
			s.tile.H = int16(y - (y0 - 8))
		}
	}
	DrawOTag(head)
	s.n = 0
	*s.text = 0
	return head
}

// FontSprites is the sprite packets the windows draw with, for the
// twin-call test to compare against the original's.
func FontSprites() *uint32 { return &fontSprites[0] }
