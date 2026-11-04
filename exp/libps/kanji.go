//go:build yaroze

package libps

import "unsafe"

// The Kanji font: libgpu's Shift JIS text streams, drawn from the glyphs
// in the console's kanji ROM, which the kernel finds (Krom2RawAdd).
// KanjiFntOpen opens a window of up to n characters, KanjiFntPrint adds
// text to it, and KanjiFntFlush fetches each character's glyph, puts them
// all into video memory as one 4-bit texture and draws a textured quad
// for each, 16 pixels square. Krom2Tim renders a string to a TIM in
// memory instead, for a program to load where it likes.
//
// A glyph in the ROM is 16 pixels wide and 15 high, a bit each, 30 bytes.
// A character is one byte of ASCII, which the ROM has as full-width
// characters, or two of Shift JIS with the first in 0x81 to 0x98.

const (
	kanjiStreams = 4
	kanjiGlyphs  = 256 // what Krom2Tim renders: 16 glyph rows of 16
	kanjiCells   = 302 // what one flush fetches: 301 characters draw on Sony's, 302 break it
)

// A kanjiStream is one window: its background tile, the ordering table
// entry its packets link from, and the text and quads it was given.
//
// libps 0x80063FE8, 4 by 52 bytes
type kanjiStream struct {
	tile    tilePacket
	ot      uint32
	max     int32      // the characters it holds
	sprites *kanjiQuad // its quads
	text    *byte      // its text
	n       int32      // the characters printed so far
	dx, dy  int32      // where its texture goes in video memory
}

// A kanjiQuad is a textured quad, POLY_FT4: one glyph on the screen.
type kanjiQuad struct {
	Tag     uint32
	R, G, B uint8
	Code    uint8
	X0, Y0  int16
	U0, V0  uint8
	Clut    uint16
	X1, Y1  int16
	U1, V1  uint8
	TPage   uint16
	X2, Y2  int16
	U2, V2  uint8
	_       uint16
	X3, Y3  int16
	U3, V3  uint8
	_       uint16
}

var _ [unsafe.Sizeof(kanjiQuad{}) - 40]byte
var _ [40 - unsafe.Sizeof(kanjiQuad{})]byte

var (
	kanjiStream0 [kanjiStreams]kanjiStream
	kanjiCount   int32  // libps 0x80048268: streams opened
	kanjiDump    int32  // libps 0x8004826C: the stream an id below 0 means; nothing sets it
	kanjiPending int32  // libps 0x80048270: the glyphs fetched for the flush under way
	kanjiUsed    int32  // libps 0x80048294: characters given out to streams
	kanjiClutID  uint16 // libps 0x8005E578
	kanjiTPage   uint16 // libps 0x8005E574: the texture page of the last flush's glyphs

	// kanjiPool holds the streams' text and, right after it, the flushes'
	// packets. The two touch because text printed past the pool runs on
	// into the packets (spec 13).
	kanjiPool struct {
		text    [1024]byte      // libps 0x80054174
		sprites [1024]kanjiQuad // libps 0x80054574
	}

	// kanjiCache holds the glyphs a flush fetched, a row of 16 bits each
	// and a blank sixteenth; kanjiPlaces where each goes on the screen
	// and in the texture.
	kanjiCache  [kanjiCells][16]uint16 // libps 0x8005E57C
	kanjiPlaces [kanjiCells]struct {   // libps 0x80062D30
		x, y int32
		u, v uint8
	}

	// kanjiPixels is the texture being built, 4 bits a pixel, which the
	// original kept on the stack: 64KB there, as here.
	kanjiPixels [0x8000]uint16
)

// kanjiClut is the palette the windows draw with: transparent, then
// white.
//
// libps 0x80048274
var kanjiClut = [8]uint32{0xffff0000}

// Krom2RawAdd is the kernel's: the address in the kanji ROM of the glyph
// of the Shift JIS character code, for a code the kernel knows.
//
// libps 0x80042FF8, 16 bytes
//
//export Krom2RawAdd
func Krom2RawAdd(code uint32) uint32 {
	return biosB(0x51, code, 0, 0, 0)
}

// Krom2RawAdd2 is the address of the glyph of code, worked out from the
// start of its block of the ROM and its offset in the block, with the
// user-defined characters, 0x8540 to 0x8796, found before the first
// block. A code outside the blocks is the space.
//
// libps 0x8002A9FC, 176 bytes
//
//export Krom2RawAdd2
func Krom2RawAdd2(code uint32) uint32 {
	code &= 0xffff
	var base uint32
	switch {
	case code-0x8140 < 895:
		base = Krom2RawAdd(0x8140)
	case code-0x8540 < 599:
		base = Krom2RawAdd(0x8140) - 0x2000
	case code-0x889f < 4052:
		base = Krom2RawAdd(0x889f)
	default:
		return Krom2RawAdd(0x8140)
	}
	off := kanjiOffset(code)
	if off < 0 {
		return Krom2RawAdd(0x8140)
	}
	return base + uint32(off)*30
}

// The ROM's blocks: each row of Shift JIS the ROM has, its first code and
// the index of that code's glyph in the block. kanjiRows is the symbols,
// digits, letters and kana of the first block, 0x81 to 0x84; kanjiUser
// the user-defined rows, 0x85 to 0x87; kanjiLevel1 the kanji rows from
// 0x889f on.
//
// libps 0x80048404, 0x800484D0 and 0x80048450
var (
	kanjiRows = [19][2]uint16{
		{0x8140, 0}, {0x8180, 0x3f}, {0x81b8, 0x6c}, {0x81c8, 0x74}, {0x81da, 0x7b}, {0x81f0, 0x8a}, {0x81fc, 0x92},
		{0x824f, 0x93}, {0x8260, 0x9d}, {0x8281, 0xb7}, {0x829f, 0xd1},
		{0x8340, 0x124}, {0x8380, 0x163}, {0x839f, 0x17a}, {0x83bf, 0x192},
		{0x8440, 0x1aa}, {0x8470, 0x1cb}, {0x8480, 0x1da}, {0x849f, 0x1ec},
	}
	kanjiUser = [19][2]uint16{
		{0x8540, 0}, {0x8546, 5}, {0x8548, 6}, {0x854b, 8}, {0x8550, 9}, {0x85a0, 0x15}, {0x85b0, 0x22},
		{0x8640, 0x2f}, {0x8643, 0x31}, {0x8645, 0x32}, {0x8647, 0x33}, {0x864a, 0x35}, {0x864e, 0x38}, {0x8660, 0x3a}, {0x869f, 0x4a}, {0x86b8, 0x62},
		{0x8740, 0xa0}, {0x875c, 0xbb}, {0x8764, 0xc2},
	}
	kanjiLevel1 = [32][2]uint16{
		{0x889f, 0}, {0x8940, 0x5e}, {0x899f, 0xbc}, {0x8a40, 0x11a}, {0x8a9f, 0x178}, {0x8b40, 0x1d6}, {0x8b9f, 0x234},
		{0x8c40, 0x292}, {0x8c9f, 0x2f0}, {0x8d40, 0x34e}, {0x8d9f, 0x3ac}, {0x8e40, 0x40a}, {0x8e9f, 0x468}, {0x8f40, 0x4c6}, {0x8f9f, 0x524},
		{0x9040, 0x582}, {0x909f, 0x5e0}, {0x9140, 0x63e}, {0x919f, 0x69c}, {0x9240, 0x6fa}, {0x929f, 0x758}, {0x9340, 0x7b6}, {0x939f, 0x814},
		{0x9440, 0x872}, {0x949f, 0x8d0}, {0x9540, 0x92e}, {0x959f, 0x98c}, {0x9640, 0x9ea}, {0x969f, 0xa48}, {0x9740, 0xaa6}, {0x979f, 0xb04}, {0x9840, 0xb62},
	}
)

// kanjiOffset is the index of code's glyph in its block of the ROM, or
// -1 for a code the ROM has no glyph for.
//
// libps 0x8002AAAC, 1328 bytes
func kanjiOffset(code uint32) int32 {
	hi, lo := code>>8&0xff, code&0xff
	var rows *[19][2]uint16
	k := -1
	switch hi {
	case 0x81:
		rows = &kanjiRows
		switch {
		case lo-0x40 < 63:
			k = 0
		case lo-0x80 < 45:
			k = 1
		case lo-0xb8 < 8:
			k = 2
		case lo-0xc8 < 7:
			k = 3
		case lo-0xda < 15:
			k = 4
		case lo-0xf0 < 8:
			k = 5
		case lo == 0xfc:
			k = 6
		}
	case 0x82:
		rows = &kanjiRows
		switch {
		case lo-0x4f < 10:
			k = 7
		case lo-0x60 < 26:
			k = 8
		case lo-0x81 < 26:
			k = 9
		case lo-0x9f < 83:
			k = 10
		}
	case 0x83:
		rows = &kanjiRows
		switch {
		case lo-0x40 < 63:
			k = 11
		case lo-0x80 < 23:
			k = 12
		case lo-0x9f < 24:
			k = 13
		case lo-0xbf < 24:
			k = 14
		}
	case 0x84:
		rows = &kanjiRows
		switch {
		case lo-0x40 < 33:
			k = 15
		case lo-0x70 < 15:
			k = 16
		case lo-0x80 < 18:
			k = 17
		case lo-0x9f < 32:
			k = 18
		}
	case 0x85:
		rows = &kanjiUser
		switch {
		case lo-0x40 < 5:
			k = 0
		case lo == 0x46:
			k = 1
		case lo-0x48 < 2:
			k = 2
		case lo == 0x4b:
			k = 3
		case lo-0x50 < 12:
			k = 4
		case lo-0xa0 < 13:
			k = 5
		case lo-0xb0 < 13:
			k = 6
		}
	case 0x86:
		rows = &kanjiUser
		switch {
		case lo-0x40 < 2:
			k = 7
		case lo == 0x43:
			k = 8
		case lo == 0x45:
			k = 9
		case lo-0x47 < 2:
			k = 10
		case lo-0x4a < 3:
			k = 11
		case lo-0x4e < 2:
			k = 12
		case lo-0x60 < 16:
			k = 13
		case lo-0x9f < 24:
			k = 14
		case lo-0xb8 < 62:
			k = 15
		}
	case 0x87:
		rows = &kanjiUser
		switch {
		case lo-0x40 < 27:
			k = 16
		case lo-0x5c < 7:
			k = 17
		case lo-0x64 < 51:
			k = 18
		}
	default:
		// The kanji: two rows a block, 0x40 to 0x7e and 0x80 to 0xfc,
		// from 0x889f to 0x9872.
		if hi < 0x88 || hi > 0x98 || hi == 0x98 && lo >= 0x73 || hi == 0x88 && lo < 0x9f || lo-0x40 >= 189 || lo == 0x7f {
			return -1
		}
		i := 2 * (hi - 0x88)
		if lo < 0x7f {
			i--
		}
		row := &kanjiLevel1[i]
		return int32(code) - int32(row[0]) + int32(row[1])
	}
	if k < 0 {
		return -1
	}
	row := &rows[k]
	return int32(code) - int32(row[0]) + int32(row[1])
}

// The ASCII characters as the ROM has them: the digits, the capitals and
// the small letters as runs from a first code, and the symbols one by
// one, in ASCII order from the space.
//
// libps 0x8004851C and 0x80048528
var (
	kanjiRuns    = [3][2]uint16{{0x824f, '0'}, {0x8260, 'A'}, {0x8281, 'a'}}
	kanjiSymbols = [33]uint16{
		0x8140, 0x8149, 0x8168, 0x8194, 0x8190, 0x8193, 0x8195, 0x8166, 0x8169, 0x816a, 0x8196, 0x817b, 0x8143, 0x817c, 0x8144, 0x815e,
		0x8146, 0x8147, 0x8171, 0x8181, 0x8172, 0x8148, 0x8197, 0x816d, 0x818f, 0x816e, 0x814f, 0x8151, 0x8165, 0x816f, 0x8162, 0x8170, 0x8150,
	}
)

// kanjiFromASCII is the Shift JIS code of the full-width form of the
// printable ASCII character c, or 0 for another byte, which is reported
// unless quiet.
//
// libps 0x8002AFDC, 308 bytes
func kanjiFromASCII(c uint32, quiet bool) uint32 {
	var sym, run int
	switch {
	case c-0x20 < 16:
		sym = 1
	case c-'0' < 10:
		run = 0
	case c-0x3a < 7:
		sym = 11
	case c-'A' < 26:
		run = 1
	case c-0x5b < 6:
		sym = 37
	case c-'a' < 26:
		run = 2
	case c-0x7b < 4:
		sym = 63
	default:
		if !quiet {
			kprintf("bad hankaku code 0x%x\n", c)
		}
		return 0
	}
	if sym != 0 {
		return uint32(kanjiSymbols[int(c)-sym-31])
	}
	return uint32(kanjiRuns[run][0]) + c - uint32(kanjiRuns[run][1])
}

// kanjiGlyph is the address of the glyph of the character at p, one
// byte of ASCII or two of Shift JIS, and how many bytes it took, or a
// length of 0 for a character the ROM has not, which is reported. With
// user, the lookup is Krom2RawAdd2's, which has a glyph for every code
// and reports nothing: a bad ASCII byte is the space, and says so by a
// negative length.
func kanjiGlyph(p *byte, user bool) (addr uint32, n int32) {
	c := uint32(*p)
	var code uint32
	if c-0x81 < 24 {
		code = c<<8 | uint32(*(*byte)(unsafe.Add(unsafe.Pointer(p), 1)))
		n = 2
	} else {
		code = kanjiFromASCII(c, user)
		n = 1
		if code == 0 {
			if !user {
				return 0, 0
			}
			n = -1
		}
	}
	if user {
		return Krom2RawAdd2(code), n
	}
	var base uint32
	switch {
	case code-0x8140 < 895:
		base = Krom2RawAdd(0x8140)
	case code-0x889f < 4052:
		base = Krom2RawAdd(0x889f)
	default:
		kprintf("bad sjis code 0x%x\n", code)
		return 0, 0
	}
	return uint32(int32(base) + kanjiOffset(code)*30), n
}

// kanjiCopy reads the glyph at addr into the 16 rows at dst, the last of
// them left as it is.
func kanjiCopy(dst *[16]uint16, addr uint32) {
	copy(unsafe.Slice((*byte)(unsafe.Pointer(dst)), 30), unsafe.Slice((*byte)(at(uintptr(addr))), 30))
}

// kanjiExpand spreads a glyph row's 16 bits over 16 pixels of 4 bits, the
// leftmost pixel in the low nibble of the first halfword, into out.
func kanjiExpand(out *[4]uint16, row uint16) {
	out[0] = row>>7&1 | row>>2&0x10 | row<<3&0x100 | row<<8&0x1000
	out[1] = row>>3&1 | row<<2&0x10 | row<<7&0x100 | row&1<<12
	out[2] = row>>15 | row>>10&0x10 | row>>5&0x100 | row&0x1000
	out[3] = row>>11&1 | row>>6&0x10 | row>>1&0x100 | row<<4&0x1000
}

// kanjiFetch reads the glyph of the character at p into the next cache
// slot and records where in the texture it goes, and returns -1 for a
// character the ROM has not.
//
// The original fetches without bound, and a flush that draws 302
// characters never returns on Sony's library (spec 13). Here the 302nd
// is one the ROM has not.
//
// libps 0x80025FA4, 324 bytes
func kanjiFetch(p *byte) int32 {
	i := kanjiPending
	if i >= kanjiCells-1 {
		return -1
	}
	addr, n := kanjiGlyph(p, false)
	if n == 0 {
		return -1
	}
	kanjiCopy(&kanjiCache[i], addr)
	kanjiPlaces[i].u = uint8(i % 16 * 16)
	kanjiPlaces[i].v = uint8(i / 16 * 16)
	return 0
}

// kanjiUpload builds the texture of the glyphs fetched, 16 to a row, and
// puts it into video memory at dx, dy, keeping its page. Fewer than 16
// glyphs make a texture a column wider than them, and the rows are one
// more than the glyphs fill: the original's bounds, which it uploaded
// with whatever its stack held beyond the glyphs; here the rest is
// blank.
//
// libps 0x800260E8, 580 bytes
func kanjiUpload(dx, dy int32) {
	n := kanjiPending
	cols := int32(16)
	if n < 16 {
		cols = n + 1
	}
	rows := n/16 + 1
	stride := cols * 4
	pixels := kanjiPixels[:rows*16*stride]
	clear(pixels)
	for g := int32(0); g < rows; g++ {
		for r := int32(0); r < 16; r++ {
			for c := int32(0); c < cols; c++ {
				i := g*cols + c
				if i >= n {
					continue
				}
				kanjiExpand((*[4]uint16)(pixels[(g*16+r)*stride+c*4:]), kanjiCache[i][r])
			}
		}
	}
	kanjiTPage = loadTPage((*uint32)(unsafe.Pointer(&kanjiPixels[0])), 0, 0, dx, dy, cols*16, rows*16)
}

// KanjiFntOpen opens a window at x, y of w by h for up to n characters,
// with its glyphs' texture at dx, dy in video memory and their palette
// at cx, cy, and a background when isbg is set, and returns its id, or
// -1 when four are open or the windows' 1024 characters are given out.
//
// libps 0x8002632C, 784 bytes
//
//export KanjiFntOpen
func KanjiFntOpen(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32 {
	clear(kanjiCache[:kanjiGlyphs])
	if kanjiCount < kanjiStreams {
		kanjiStream0[kanjiCount] = kanjiStream{}
	}
	DrawSync(0)
	kanjiClutID = loadClut(&kanjiClut[0], cx, cy)
	if kanjiCount >= kanjiStreams {
		return -1
	}
	if kanjiCount == 0 {
		kanjiUsed = 0
	}
	if kanjiUsed+n >= 1024 {
		return -1
	}
	id := kanjiCount
	s := &kanjiStream0[id]
	if isbg != 0 {
		s.tile.Tag = s.tile.Tag&0xffffff | 3<<24
		s.tile.Code = 0x60
		s.tile.R, s.tile.G, s.tile.B = 0, 0, 0
	}
	s.tile.X, s.tile.Y, s.tile.W, s.tile.H = int16(x), int16(y), int16(w), int16(h)
	// A negative n lowers kanjiUsed, so a later stream can start before the
	// pool, as Sony's does: these are pointer sums, not indexes.
	s.text = (*byte)(unsafe.Add(unsafe.Pointer(&kanjiPool.text[0]), kanjiUsed))
	s.sprites = (*kanjiQuad)(unsafe.Add(unsafe.Pointer(&kanjiPool.sprites[0]), int(kanjiUsed)*int(unsafe.Sizeof(kanjiQuad{}))))
	s.max = n
	s.n = 0
	s.dx, s.dy = dx, dy
	*s.text = 0
	// Only the packets inside the array are set up: Sony's writes the ones
	// before it too, over whatever lies there, which is its own layout's.
	for i := max(kanjiUsed, 0); i < kanjiUsed+n; i++ {
		q := &kanjiPool.sprites[i]
		q.Tag = q.Tag&0xffffff | 9<<24
		q.Code = 0x2d
		q.Clut = kanjiClutID
	}
	kanjiUsed += n
	kanjiCount++
	return id
}

// KanjiFntClose closes every window.
//
// libps 0x8002663C, 32 bytes
//
//export KanjiFntClose
func KanjiFntClose() {
	kanjiPending = 0
	kanjiDump = 0
	kanjiCount = 0
}

// KanjiFntPrint adds s to the first window, and returns how many bytes
// the window holds, or -1 when it is full. A %% is one %; the other
// directives the original formatted from arguments it was given, which
// this library's callers never pass, so a % stays as it is.
//
// The original took a window's id ahead of the text as well, and took
// the text first when the first argument was not an open window's id,
// which a pointer never is.
//
// libps 0x80026A2C, 1240 bytes
//
//export KanjiFntPrint
func KanjiFntPrint(str *byte) int32 { return kanjiAdd(kanjiDump, str, true) }

// KanjiWrite adds the text at str to window id, kanjiDump's for an id
// below 0, as it is: for a caller that has formatted it already.
func KanjiWrite(id int32, str *byte) int32 {
	if id < 0 {
		id = kanjiDump
	}
	if id >= int32(len(kanjiStream0)) {
		return -1
	}
	return kanjiAdd(id, str, false)
}

// KanjiAddBytes adds b to Kanji window id, as the original's
// KanjiFntPrint added its text: without the window's size limiting it,
// only the end of the text space the windows share, and 0 for a window
// that is not open. It returns the bytes the window holds.
func KanjiAddBytes(id int32, b []byte) int32 {
	if id < 0 {
		id = kanjiDump
	}
	if id >= int32(len(kanjiStream0)) || kanjiStream0[id].text == nil {
		return 0
	}
	s := &kanjiStream0[id]
	for _, c := range b {
		*kanjiTextAt(s) = c
		s.n++
	}
	*kanjiTextAt(s) = 0
	return s.n
}

// kanjiAdd adds the text at str to window id, a %% as one % when percent
// is set.
func kanjiAdd(id int32, str *byte, percent bool) int32 {
	s := &kanjiStream0[id]
	if s.text == nil {
		return -1
	}
	if s.max < s.n {
		return -1
	}
	// The window's size is checked before the text is added, not after:
	// the text goes in whole and the new count comes back even past the
	// size, and nothing checks the end of the pool: past it the text runs
	// on into the packets (spec 13, KanjiFntPrint).
	for p := str; *p != 0; p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1)) {
		c := *p
		if percent && c == '%' && *(*byte)(unsafe.Add(unsafe.Pointer(p), 1)) == '%' {
			p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
		}
		*kanjiTextAt(s) = c
		s.n++
	}
	*kanjiTextAt(s) = 0
	return s.n
}

// kanjiTextAt returns where stream s's next byte of text goes.
func kanjiTextAt(s *kanjiStream) *byte {
	return (*byte)(unsafe.Add(unsafe.Pointer(s.text), s.n))
}

// KanjiFntFlush draws window id's text, the first window for an id that
// is not an open one's, with the background behind it, and empties it.
// Each character is a quad of 16 pixels square, a space is 16 wide and a
// tab 64; after a ~p the small letters are 12 wide. It returns the
// ordering table entry that heads what it drew, or nil for a window
// never opened, or for a character the ROM has not, when nothing is
// drawn and the text stays.
//
// libps 0x8002665C, 976 bytes
//
//export KanjiFntFlush
func KanjiFntFlush(id int32) *uint32 {
	if id < 0 || id >= kanjiCount {
		id = kanjiDump
	}
	s := &kanjiStream0[id]
	if s.text == nil {
		return nil
	}
	head := &s.ot
	x0, y0 := int32(s.tile.X), int32(s.tile.Y)
	x1, y1 := x0+int32(s.tile.W), y0+int32(s.tile.H)
	x, y := x0, y0
	left := s.max
	kanjiPending = 0
	*head |= 0xffffff
	proportional := false
	text := s.text
	for *text != 0 && left != 0 {
		c := *text
		newline, placed := false, false
		switch {
		case c == ' ':
			x += 16
		case c == '\t':
			x += 64
		case c == '\n':
			newline = true
		case c == '~':
			if *(*byte)(unsafe.Add(unsafe.Pointer(text), 1)) == 'p' {
				proportional = true
				text = (*byte)(unsafe.Add(unsafe.Pointer(text), 1))
			}
		case c < 0x21 || c > 0x7e && c-0x81 >= 24:
			return nil
		default:
			if kanjiFetch(text) < 0 {
				return nil
			}
			if c >= 0x81 {
				text = (*byte)(unsafe.Add(unsafe.Pointer(text), 1))
			}
			placed = true
			kanjiPlaces[kanjiPending].x, kanjiPlaces[kanjiPending].y = x, y
			kanjiPending++
			// The width after ~p is read from the byte the text is at,
			// which for a two-byte character is its second byte.
			if proportional && *text-'a' < 26 {
				x += 12
			} else {
				x += 16
			}
		}
		if (placed || c == ' ' || c == '\t') && x >= x1 {
			newline = true
		}
		left--
		if newline {
			y += 16
			x = x0
			if y >= y1 {
				break
			}
		}
		text = (*byte)(unsafe.Add(unsafe.Pointer(text), 1))
	}
	kanjiUpload(s.dx, s.dy)
	sprites := unsafe.Slice(s.sprites, kanjiPending)
	for i := range sprites {
		q, p := &sprites[i], &kanjiPlaces[i]
		q.TPage = kanjiTPage
		q.U0, q.V0 = p.u, p.v
		q.U1, q.V1 = p.u+15, p.v
		q.U2, q.V2 = p.u, p.v+15
		q.U3, q.V3 = p.u+15, p.v+15
		q.X0, q.Y0 = int16(p.x), int16(p.y)
		q.X1, q.Y1 = int16(p.x+15), int16(p.y)
		q.X2, q.Y2 = int16(p.x), int16(p.y+15)
		q.X3, q.Y3 = int16(p.x+15), int16(p.y+15)
		addPrim(head, &q.Tag)
	}
	if s.tile.Code != 0 {
		addPrim(head, &s.tile.Tag)
	}
	DrawOTag(head)
	s.n = 0
	*s.text = 0
	return head
}

// kanjiGlyphs2 is Krom2Tim's working store of glyphs, the original's
// stack.
var kanjiGlyphs2 [kanjiGlyphs][16]uint16

// Krom2Tim renders the string sjis, up to 256 characters in rows of 16
// that a newline ends early, as a 4-bit TIM at taddr, placed at dx, dy
// in video memory with its palette at cdx, cdy, the glyphs in the color
// fg on bg. It returns 0, or -1 for a character the ROM has not, which
// is reported. A TIM of fewer than 16 characters is as wide as they are;
// one of more is 256 pixels wide and as many rows of 16 as they fill.
//
// libps 0x80026F04, 1280 bytes
//
//export Krom2Tim
func Krom2Tim(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
	return krom2Tim(sjis, taddr, dx, dy, cdx, cdy, fg, bg, false)
}

// Krom2Tim2 is Krom2Tim with the glyphs found as Krom2RawAdd2 finds
// them, the user-defined characters among them: every code has a glyph,
// and a byte that is not a character is the space, which makes the
// result -1 once the TIM is made.
//
// libps.a(krom2tm2.o), linked into a program that calls it
//
//export Krom2Tim2
func Krom2Tim2(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32) int32 {
	return krom2Tim(sjis, taddr, dx, dy, cdx, cdy, fg, bg, true)
}

func krom2Tim(sjis *uint8, taddr *uint32, dx, dy, cdx, cdy int32, fg, bg uint32, user bool) int32 {
	glyphs := &kanjiGlyphs2
	*glyphs = [kanjiGlyphs][16]uint16{}
	clear(kanjiPixels[:])
	result := int32(0)
	n := int32(0)
	for p := sjis; *p != 0 && n < kanjiGlyphs; {
		if *p == '\n' {
			n = (n/16 + 1) * 16
			p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
			continue
		}
		addr, took := kanjiGlyph(p, user)
		if took == 0 {
			return -1
		}
		if took < 0 {
			result = -1
			took = 1
		}
		p = (*byte)(unsafe.Add(unsafe.Pointer(p), int(took)))
		kanjiCopy(&glyphs[n], addr)
		n++
	}
	// The pixels, 256 wide: each row of glyphs until the one the string
	// ends in.
	for g := int32(0); g < 16; g++ {
		for r := int32(0); r < 16; r++ {
			for c := int32(0); c < 16; c++ {
				if i := g*16 + c; i < n {
					kanjiExpand((*[4]uint16)(kanjiPixels[(g*16+r)*64+c*4:]), glyphs[i][r])
				}
			}
		}
		if g*16+15 >= n {
			break
		}
	}
	// The TIM: 16 words of header and palette, then the pixels, trimmed
	// to the string's width when it is a row or less.
	words := n * 32
	if n >= 16 {
		words = ((n-1)/16 + 1) * 512
	}
	tim := unsafe.Slice(taddr, 16+words)
	clear(tim)
	tim[0] = 0x10
	tim[1] = 8
	tim[2] = 0x2c
	tim[3] = uint32(cdy)<<16 | uint32(cdx)
	tim[4] = 1<<16 | 16
	tim[5] = fg<<16 | bg
	tim[13] = uint32(words+3) * 4
	tim[14] = uint32(dy)<<16 | uint32(dx)
	rows := int32(1)
	if n > 0 {
		rows = (n-1)/16 + 1
	}
	w := uint32(0x40)
	if n < 17 {
		w = uint32((n - 1 - (rows-1)*16 + 1) * 4)
	}
	tim[15] = uint32(rows*16)<<16 | w
	pixels := tim[16:]
	if n < 16 {
		width := n * 4
		src := int32(0)
		for i := range pixels {
			if src&63 >= width {
				src = (src/64 + 1) * 64
			}
			pixels[i] = uint32(kanjiPixels[src]) | uint32(kanjiPixels[src+1])<<16
			src += 2
		}
	} else {
		for i := range pixels {
			pixels[i] = uint32(kanjiPixels[2*i]) | uint32(kanjiPixels[2*i+1])<<16
		}
	}
	return result
}
