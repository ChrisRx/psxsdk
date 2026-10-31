//go:build yaroze

package libps

//go:generate go run ./gen/font

import "unsafe"

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
	fontCount   int32            // libps 0x80047858: streams opened
	fontDump    int32            // libps 0x8004785C: the stream FntPrint takes by default
	fontUsed    int32            // libps 0x80048260: characters given out to streams
	fontTPage   uint16           // libps 0x8005416C
	fontClutID  uint16           // libps 0x80054170
	fontText    [1024]byte       // libps 0x8004FD6C
	fontSprites [1024 * 4]uint32 // libps 0x8005016C: 1024 sprite packets
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
	id := fontDump
	s := &fontStream0[id]
	if s.text == nil {
		return -1
	}
	if s.n > s.max {
		return -1
	}
	text := unsafe.Slice(s.text, 1025)
	for p := str; *p != 0; p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1)) {
		c := *p
		if c == '%' {
			next := *(*byte)(unsafe.Add(unsafe.Pointer(p), 1))
			if next == '%' {
				p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
			}
		}
		text[s.n] = c
		s.n++
		if s.n > s.max {
			return -1
		}
	}
	text[s.n] = 0
	return s.n
}

// FntWrite adds s to window id, as FntPrint adds to the default window,
// for a Go caller with a string: a % is taken as it is. It returns how
// many characters the window holds, or -1 when it is full or not open.
func FntWrite(id int32, s string) int32 {
	return FntWriteBytes(id, unsafe.Slice(unsafe.StringData(s), len(s)))
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
func FntFlush(id int32) *uint32 {
	if id < 0 {
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
	text := s.text
	for *text != 0 && left != 0 {
		c := *text
		newline := false
		switch {
		case c == ' ':
			x += 8
		case c == '\t':
			x += 32
		case c == '\n':
			newline = true
		case c == '~':
			// ~cRGB sets the color from three digits; any other
			// escape is skipped with the character after it.
			text = (*byte)(unsafe.Add(unsafe.Pointer(text), 1))
			if *text == 'c' {
				d := unsafe.Slice(text, 4)
				r, g, b = (d[1]-'0')<<4, (d[2]-'0')<<4, (d[3]-'0')<<4
				text = (*byte)(unsafe.Add(unsafe.Pointer(text), 3))
			}
		default:
			i := int32(c) - 32
			if c >= 'a' && c <= 'z' {
				i = int32(c) - 64
			}
			p := (*[4]uint32)(unsafe.Pointer(sprite))
			u, v := uint8((i%16)*8), uint8((i/16)*8)
			p[1] = p[1]&0xff000000 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
			p[2] = uint32(uint16(x)) | uint32(uint16(y))<<16
			p[3] = p[3]&0xffff0000 | uint32(v)<<8 | uint32(u)
			addPrim(head, &p[0])
			sprite = (*uint32)(unsafe.Add(unsafe.Pointer(sprite), 16))
			x += 8
		}
		if c != '\n' && c != '~' && x >= x1 && s.autosize == 0 {
			newline = true
		}
		if newline {
			if maxX < x {
				maxX = x
			}
			y += 8
			x = x0
			if y >= y1 {
				break
			}
		}
		text = (*byte)(unsafe.Add(unsafe.Pointer(text), 1))
		left--
	}
	if s.tile.Code != 0 {
		addPrim(head, &s.tile.Tag)
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
