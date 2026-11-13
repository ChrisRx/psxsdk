//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Kanji text (spec 13): Krom2Tim and Krom2Tim2, which render a Shift JIS
// string in the console's ROM font as a 4-bit TIM, and the Kanji print
// streams, which draw Shift JIS text with the same glyphs.

// The BIOS ROM's font (psx-spx, "BIOS Character Sets"): 16 by 15 pixel
// glyphs of 30 bytes, a row in two bytes with the leftmost pixel in the top
// bit, the non-kanji characters first and then the first level kanji.
const (
	romNonKanji = 0xbfc66000
	romKanji    = 0xbfc69d68
	romGlyph    = 30

	// kanjiCell is a glyph cell's width and height in pixels, kanjiRow the
	// cells in a row of an image, and kanjiRows the most rows of them: at the
	// end of the last the string is over, whatever it goes on to hold.
	kanjiCell = 16
	kanjiRow  = 16
	kanjiRows = 16
)

// nonKanji lists the assigned cells of JIS rows 1 to 8 as ranges of Shift
// JIS codes, in the order the ROM holds their glyphs: the cells between
// the ranges have none.
var nonKanji = [][2]uint16{
	{0x8140, 0x817e}, {0x8180, 0x81ac}, {0x81b8, 0x81bf}, {0x81c8, 0x81ce},
	{0x81da, 0x81e8}, {0x81f0, 0x81f7}, {0x81fc, 0x81fc}, // symbols
	{0x824f, 0x8258}, {0x8260, 0x8279}, {0x8281, 0x829a}, // digits, letters
	{0x829f, 0x82f1},                   // hiragana
	{0x8340, 0x837e}, {0x8380, 0x8396}, // katakana
	{0x839f, 0x83b6}, {0x83bf, 0x83d6}, // Greek
	{0x8440, 0x8460}, {0x8470, 0x847e}, {0x8480, 0x8491}, // Cyrillic
	{0x849f, 0x84be}, // box drawing
}

// fullWidth is the full-width character each printable ASCII character,
// from the space to the tilde, is drawn as.
var fullWidth = [95]uint16{
	0x8140, 0x8149, 0x8168, 0x8194, 0x8190, 0x8193, 0x8195, 0x8166, // space to '
	0x8169, 0x816a, 0x8196, 0x817b, 0x8143, 0x817c, 0x8144, 0x815e, // ( to /
	0x824f, 0x8250, 0x8251, 0x8252, 0x8253, 0x8254, 0x8255, 0x8256, // 0 to 7
	0x8257, 0x8258, 0x8146, 0x8147, 0x8171, 0x8181, 0x8172, 0x8148, // 8 to ?
	0x8197, 0x8260, 0x8261, 0x8262, 0x8263, 0x8264, 0x8265, 0x8266, // @ to G
	0x8267, 0x8268, 0x8269, 0x826a, 0x826b, 0x826c, 0x826d, 0x826e, // H to O
	0x826f, 0x8270, 0x8271, 0x8272, 0x8273, 0x8274, 0x8275, 0x8276, // P to W
	0x8277, 0x8278, 0x8279, 0x816d, 0x818f, 0x816e, 0x814f, 0x8151, // X to _
	0x8165, 0x8281, 0x8282, 0x8283, 0x8284, 0x8285, 0x8286, 0x8287, // ` to g
	0x8288, 0x8289, 0x828a, 0x828b, 0x828c, 0x828d, 0x828e, 0x828f, // h to o
	0x8290, 0x8291, 0x8292, 0x8293, 0x8294, 0x8295, 0x8296, 0x8297, // p to w
	0x8298, 0x8299, 0x829a, 0x816f, 0x8162, 0x8170, 0x8150, // x to ~
}

// A glyphKind says what a character draws as.
type glyphKind uint8

const (
	glyphROM        glyphKind = iota // a glyph of the ROM font
	glyphBlank                       // a blank cell
	glyphBadSJIS                     // a two-byte code outside the font
	glyphBadHankaku                  // a single byte that is not ASCII
)

// nextKanji reads the character of a Shift JIS string at p: a two-byte
// code for a lead byte from 81h to 98h, whatever the byte after it is, the
// NUL included, or else a single byte. It returns the character, 0 at the
// end of the string, and where the next one starts, which stays at the
// end.
func nextKanji(p unsafe.Pointer) (uint16, unsafe.Pointer) {
	b := *(*byte)(p)
	switch {
	case b >= 0x81 && b <= 0x98:
		return uint16(b)<<8 | uint16(*(*byte)(unsafe.Add(p, 1))), unsafe.Add(p, 2)
	case b != 0:
		return uint16(b), unsafe.Add(p, 1)
	}
	return 0, p
}

// glyphFor returns the ROM address of the glyph character c draws as, and
// what it draws as. user is Krom2Tim2's reading, the user-defined
// characters' version, which draws a blank cell for a two-byte code the
// font has no place for.
func glyphFor(c uint16, user bool) (uintptr, glyphKind) {
	switch {
	case c >= ' ' && c <= '~':
		// The table entry, worked out from c itself: one instruction more than
		// from c-' ', which the pads of Krom2Tim and KanjiFntFlush were measured with.
		c = *(*uint16)(unsafe.Add(unsafe.Pointer(&fullWidth), 2*uintptr(c)-2*' '))
	case c <= 0xff:
		return 0, glyphBadHankaku
	}
	switch {
	case c >= 0x8140 && c <= 0x84be:
		i := 0
		for _, r := range nonKanji {
			if c < r[0] {
				break
			}
			if c <= r[1] {
				return romNonKanji + uintptr(i+int(c-r[0]))*romGlyph, glyphROM
			}
			i += int(r[1]-r[0]) + 1
		}
		// A cell with no glyph, or a trail byte Shift JIS does not use.
		// Krom2Tim draws the 30 bytes just before the table, whatever the
		// ROM holds there (spec 51): the end of a copyright line and four
		// addresses on most of Sony's, zeros on some and on OpenBIOS.
		if user {
			return 0, glyphBlank
		}
		return romNonKanji - romGlyph, glyphROM
	case c >= 0x889f && c <= 0x9872:
		row, cell, ok := jisCell(c)
		if !ok {
			// A trail byte outside 40h to FCh: Krom2Tim draws the
			// glyph just before the kanji, the last box drawing
			// character's.
			if user {
				return 0, glyphBlank
			}
			return romKanji - romGlyph, glyphROM
		}
		return romKanji + uintptr((row-16)*94+cell-1)*romGlyph, glyphROM
	}
	if user {
		return 0, glyphBlank
	}
	return 0, glyphBadSJIS
}

// nonKanjiExtra lists the assigned cells of Krom2RawAdd2's second range, in its
// order, which come before the ROM's non-kanji cells (spec 18).
var nonKanjiExtra = [][2]uint16{
	{0x8540, 0x8544}, {0x8546, 0x8546}, {0x8548, 0x8549}, {0x854b, 0x854b},
	{0x8550, 0x855b}, {0x85a0, 0x85ac}, {0x85b0, 0x85bc}, {0x8640, 0x8641},
	{0x8643, 0x8643}, {0x8645, 0x8645}, {0x8647, 0x8648}, {0x864a, 0x864c},
	{0x864e, 0x864f}, {0x8660, 0x866f}, {0x869f, 0x86b6}, {0x86b8, 0x86f5},
	{0x8740, 0x875a}, {0x875c, 0x8762}, {0x8764, 0x8796},
}

// rangeIndex returns the place of c among the cells the ranges list.
func rangeIndex(ranges [][2]uint16, c uint16) (uint32, bool) {
	i := uint32(0)
	for _, r := range ranges {
		if c < r[0] {
			break
		}
		if c <= r[1] {
			return i + uint32(c-r[0]), true
		}
		i += uint32(r[1]-r[0]) + 1
	}
	return 0, false
}

// kromKernel is the kernel's address of the glyph for code.
func kromKernel(code uint16) uint32 {
	return uint32(libps_biosB(biosKrom2RawAdd, uintptr(code), 0, 0, 0))
}

// libKrom2RawAdd2 returns the address of the glyph for the low 16 bits of code,
// with the kernel giving only the bases, or the blank, the full-width space, for
// a code with no glyph. Unlike Krom2Tim's glyphFor it does not widen single
// bytes, and a kanji code with a bad trail byte is a blank.
//
//export Krom2RawAdd2
func libKrom2RawAdd2(code uint32) uint32 {
	c := uint16(code)
	switch {
	case c >= 0x8140 && c <= 0x84be:
		base := kromKernel(0x8140)
		if i, ok := rangeIndex(nonKanji, c); ok {
			return base + i*romGlyph
		}
	case c >= 0x8540 && c <= 0x8796:
		base := kromKernel(0x8140)
		if i, ok := rangeIndex(nonKanjiExtra, c); ok {
			return base - 0x2000 + i*romGlyph
		}
	case c >= 0x889f && c <= 0x9872:
		base := kromKernel(0x889f)
		if row, cell, ok := jisCell(c); ok {
			return base + uint32((row-16)*94+cell-1)*romGlyph
		}
	}
	return kromKernel(0x8140)
}

// jisCell returns the JIS row and cell, from 1, of the two-byte Shift JIS
// code c, and whether its trail byte is one Shift JIS uses.
func jisCell(c uint16) (row, cell int, ok bool) {
	lead, trail := int(c>>8), int(c&0xff)
	if trail < 0x40 || trail == 0x7f || trail > 0xfc {
		return 0, 0, false
	}
	row = (lead-0x81)*2 + 1
	switch {
	case trail >= 0x9f:
		return row + 1, trail - 0x9e, true
	case trail > 0x7f:
		return row, trail - 0x40, true
	}
	return row, trail - 0x3f, true
}

// nibbles spreads the eight pixels of a glyph row's byte over a word of
// 4-bit pixels, the leftmost in the low nibble: index 1 where the glyph
// is set, 0 where it is not.
func nibbles(b byte) uint32 {
	var w uint32
	for k := range 8 {
		if b&(0x80>>k) != 0 {
			w |= 1 << (4 * k)
		}
	}
	return w
}

// A kanjiCursor is the cell a Krom2Tim string's next character goes in.
type kanjiCursor struct{ x, y int }

// past returns the cursor moved past the character c: a newline to the
// start of the next row, any other character to the next cell, the next
// row's first after the sixteenth.
func (at kanjiCursor) past(c uint16) kanjiCursor {
	at.x++
	if c == '\n' || at.x == kanjiRow {
		at.x, at.y = 0, at.y+1
	}
	return at
}

// putGlyph writes the glyph at the ROM address addr, or a blank cell for
// 0, into cell (col, row) of the 4-bit image at pix, cols cells wide. The
// cell's sixteenth row is always blank.
func putGlyph(pix []uint32, cols, col, row int, addr uintptr) {
	stride := cols * 2
	i := row*kanjiCell*stride + col*2
	for r := range kanjiCell {
		var hi, lo byte
		if addr != 0 && r < 15 {
			hi = *(*byte)(unsafe.Pointer(addr + uintptr(2*r)))
			lo = *(*byte)(unsafe.Pointer(addr + uintptr(2*r+1)))
		}
		pix[i] = nibbles(hi)
		pix[i+1] = nibbles(lo)
		i += stride
	}
}

// Krom2Tim renders the Shift JIS string sjis as a 4-bit TIM at taddr, its
// pixels for dx, dy in video memory and its palette for cx, cy, with
// index 0 the color bg and 1 fg. It returns 0, or -1 without writing
// anything if a character is not one the font has, after printing which.
//
//export Krom2Tim
func Krom2Tim(sjis *byte, taddr *uint32, dx, dy, cx, cy int32, fg, bg uint32) int32 {
	clearKromStack(krom2TimFrame)
	r, shape := krom2Tim(sjis, taddr, dx, dy, cx, cy, fg, bg, false, krom2TimFrame)
	if r == 0 {
		leaveKromImage(krom2TimFrame, taddr, shape)
		spinCycles(krom2TimPad + krom2TimCellPad*shape.cells + krom2TimBlankPad*(shape.reached-shape.glyphs) - krom2TimGlyphBack*shape.glyphs - krom2TimRowBack*shape.rows)
	}
	return r
}

// kromShape is what a string made of a TIM, for what Krom2Tim spends and leaves:
// the image's cells, columns and rows, the cells the string's characters and
// newlines went past, and the glyphs among them.
type kromShape struct {
	cells   uint32
	cols    uint32
	rows    uint32
	reached uint32
	glyphs  uint32
}

// The rest of Krom2Tim's time. Sony's is 610,500 cycles, 2,050 for each cell of the
// image, 2,225 for each it reached, and 875 for each glyph (probe: 1 to 33 characters,
// with and without newlines), against the work here of 1,035 a cell and 3,180 a
// glyph, and the time of what it leaves on the stack: the clear's 589,568 and the
// calls' 48, and 384 a cell and 288 a row for the image, and for a glyph its ROM
// bytes. The figure for a glyph comes off two-byte and one-byte characters alike.
// The first frame's vertical blank goes on top of the pad. The clear was 589,632
// with a frame of 80 bytes: the frame is 88 since kromCharTime walks its table of
// gaps (spec 51), and that check takes 16 cycles of its own, which come off here.
const (
	krom2TimPad       = 595000 - 589568 - 48 - 206
	krom2TimCellPad   = 1015 - 384
	krom2TimBlankPad  = 2225
	krom2TimGlyphBack = 100 + 396
	krom2TimRowBack   = 288

	// krom2TimFrame and krom2Tim2Frame are how far Krom2Tim's and Krom2Tim2's sp
	// are below their callers', which what they leave on the stack is placed by.
	krom2TimFrame  = 88
	krom2Tim2Frame = 72
)

// Krom2Tim2 is Krom2Tim reading the string as user-defined characters: a
// two-byte code the font has no glyph for is a blank cell, and a byte that
// is not ASCII a blank cell that makes it return -1 once the whole TIM is
// written.
//
//export Krom2Tim2
func Krom2Tim2(sjis *byte, taddr *uint32, dx, dy, cx, cy int32, fg, bg uint32) int32 {
	clearKromStack(krom2Tim2Frame)
	r, shape := krom2Tim(sjis, taddr, dx, dy, cx, cy, fg, bg, true, krom2Tim2Frame)
	leaveKromImage(krom2Tim2Frame, taddr, shape)
	return r
}

func krom2Tim(s *byte, taddr *uint32, dx, dy, cx, cy int32, fg, bg uint32, user bool, frame uint32) (int32, kromShape) {
	// The first pass checks the characters and lays them out: they fill
	// rows of 16 cells, and a newline starts the next row. The image is
	// as wide as the characters up to 16 cells, or 16 with a newline, and
	// as high as the rows they reach, at least one.
	result := int32(0)
	count, lines := 0, false
	var at kanjiCursor
	for c, p := nextKanji(unsafe.Pointer(s)); c != 0 && at.y < kanjiRows; c, p = nextKanji(p) {
		at = at.past(c)
		if c == '\n' {
			lines = true
			continue
		}
		count++
		if !user {
			kromCharTime(c)
		}
		switch _, kind := glyphFor(c, user); kind {
		case glyphBadSJIS:
			ttyPrintf("bad sjis code 0x%x\n", uint32(c), 0)
			leaveKromGlyphs(frame, s)
			return -1, kromShape{}
		case glyphBadHankaku:
			if !user {
				ttyPrintf("bad hankaku code 0x%x\n", uint32(c), 0)
				leaveKromGlyphs(frame, s)
				return -1, kromShape{}
			}
			result = -1
		}
	}
	cols := min(count, kanjiRow)
	if lines {
		cols = kanjiRow
	}
	rows := at.y
	if at.x > 0 || rows == 0 {
		rows++
	}

	// The places are 32-bit ORs, as the palette's colors are: a coordinate
	// past 15 bits leaks into the other half, and nothing is clipped.
	t := unsafe.Slice(taddr, 16+cols*2*rows*kanjiCell)
	t[0] = 0x10 // the TIM's ID
	t[1] = 0x08 // 4-bit, with a palette
	t[2] = 12 + 16*2
	t[3] = uint32(cy)<<16 | uint32(cx)
	t[4] = pair(16, 1)
	t[5] = fg<<16 | bg
	for i := 6; i < 13; i++ {
		t[i] = 0
	}
	t[13] = uint32(12 + cols*8*rows*kanjiCell)
	t[14] = uint32(dy)<<16 | uint32(dx)
	t[15] = pair(int16(cols*4), int16(rows*kanjiCell))
	pix := t[16:]
	for i := range pix {
		pix[i] = 0
	}
	at = kanjiCursor{}
	for c, p := nextKanji(unsafe.Pointer(s)); c != 0 && at.y < kanjiRows; c, p = nextKanji(p) {
		if c != '\n' {
			addr, _ := glyphFor(c, user)
			putGlyph(pix, cols, at.x, at.y, addr)
			leaveKromGlyph(frame, at.x, at.y, addr)
		}
		at = at.past(c)
	}
	return result, kromShape{
		cells:   uint32(cols * rows),
		cols:    uint32(cols),
		rows:    uint32(rows),
		reached: uint32(at.y*kanjiRow + at.x),
		glyphs:  uint32(count),
	}
}

const (
	// kanjiStreams is how many Kanji print streams can be open at once.
	kanjiStreams = 4

	// kanjiPool is the bytes of text the open streams share, and
	// kanjiPoolOpen the most of them the n values of open streams may add
	// up to.
	kanjiPool     = 1024
	kanjiPoolOpen = 1023

	// kanjiDrawn is the most characters a flush draws, which bounds its
	// glyph image and packets. A flush of more fails (spec 13): Sony's
	// library breaks there.
	kanjiDrawn = 301

	// kanjiQuad is a quadrangle packet's size in bytes, and kanjiBlock the
	// bytes of the text pool and the packets after it, one for each of the
	// pool's bytes, which text printed past the pool runs on into.
	kanjiQuad  = 40
	kanjiBlock = kanjiPool * (1 + kanjiQuad)
)

// A kanjiStream is a window on the screen, the place of its glyph image
// and palette in video memory, and the text printed to it since it was
// last flushed.
type kanjiStream struct {
	x, y, w, h  int16
	dx, dy      int16
	tpage, clut uint16
	isbg        bool

	// n is how many characters and newlines a flush draws, and how
	// many bytes the stream takes of the pool, from off. count is what
	// KanjiFntPrint has counted into it since the last flush.
	n     int32
	off   int
	count int32

	// end is where the text's NUL is, where the next print goes on.
	end int
}

var kanji kanjiState

// kanjiState is the Kanji streams' state.
type kanjiState struct {
	streams [kanjiStreams]kanjiStream
	open    int // streams opened since KanjiFntClose
	used    int // the n values of the open streams, added up

	// opened is set by the first KanjiFntOpen. KanjiFntClose only lets the
	// next open start over: stream 0 goes on taking prints and flushing until
	// then (probe spec/probes/cfntclose).
	opened bool

	// pool holds the streams' texts, each ended by a NUL, and quads, right
	// after it and part of one block with it, the flushes' packets, a
	// textured quadrangle for each character, a stream's from the place
	// that matches its offset in the pool. A print does not stop at the
	// pool's end, so text can land in the packets, and a flush leaves the
	// bytes it does not set as they are, so that some of the text goes out
	// with them (spec 13). Only the block's end stops a print.
	pool  [kanjiPool]byte
	quads [kanjiPool][kanjiQuad / 4]uint32

	// bg is the packet that clears the window.
	bg [4]uint32

	// ot is the one-entry ordering table KanjiFntFlush draws through,
	// and sent the drawing queue's tail after the last flush (see
	// font.sent).
	ot   uint32
	sent uint32
}

// kanjiPlaced is a character a flush draws: the ROM address of its glyph
// and its place in the window.
type kanjiPlaced struct {
	addr uintptr
	x, y int16
}

// kanjiFrame is a flush's working memory, on the stack for the length of the
// call, as a pattern below sp shows Sony's to be: 65,536 bytes above the calls
// it makes, the image written into them from the bottom a cell's row at a time,
// 65,672 bytes below the caller's sp, the words it does not write left as they
// were. So the cells of the image's last row that no character reaches load
// what the stack held, on either library.
type kanjiFrame struct {
	// image is the glyph image a flush loads, rows of 16 cells with a cell
	// for each character drawn and one more.
	image [kanjiImageWords]uint32

	// placed is the characters of the text a flush draws: it reads them all
	// before it builds a packet, which may overwrite what is left.
	placed [kanjiDrawn]kanjiPlaced

	_ [kanjiFrameBytes - kanjiImageWords*4 - kanjiDrawn*8]byte
}

const kanjiImageWords = (kanjiDrawn/kanjiRow + 1) * kanjiCell * kanjiRow * 2

// kanjiFlushFrame calls libps_kanji_flush(st, frame) with size bytes taken off
// the stack, in assembly: the compiler moves a local this big to the heap.
//
//export libps_kanji_frame
func kanjiFlushFrame(st *kanjiStream, size uint32) *uint32

// kanjiFlushOn is what kanjiFlushFrame calls.
//
//export libps_kanji_flush
func kanjiFlushOn(st *kanjiStream, f *kanjiFrame) *uint32 {
	return st.flush(f)
}

// kanjiClut is the streams' palette: index 0 transparent black, 1 white,
// as Krom2Tim makes it with fg FFFFh and bg 0.
var kanjiClut = [8]uint32{0xffff0000}

// KanjiFntOpen loads the Kanji streams' palette into video memory at cx,
// cy, white on transparent black, and opens a stream that draws in the
// window at x, y, w by h, with its glyph image at dx, dy, at most n
// characters and newlines each flush. It returns the stream's id, or -1
// once four are open or when the n values of the open streams and this one
// add up to more than the pool gives out (a negative n lowers the sum);
// a refused open changes nothing. With isbg not 0 every flush clears the
// window to black before the text is drawn.
//
//export KanjiFntOpen
func KanjiFntOpen(x, y, w, h, dx, dy, cx, cy, isbg, n int32) int32 {
	// The palette goes in whether or not a stream is left.
	loadImage(RECT{X: int16(cx), Y: int16(cy), W: 16, H: 1}, unsafe.Pointer(&kanjiClut[0]))
	id := kanji.open
	if id == kanjiStreams || kanji.used+int(n) > kanjiPoolOpen {
		spinCycles(kanjiFntOpenPad - kanjiFntOpenRefusedLess)
		nops(kanjiFntOpenRefusedNops)
		return -1
	}
	// Sony's takes longer by the characters the stream has room for.
	spinCycles(kanjiFntOpenCharPad * uint32(max(n, 0)))
	// A negative sum puts the stream at the pool's start: where Sony's
	// puts it was not probed, and its memory before the pool is not the
	// clean library's to overwrite.
	off := max(kanji.used, 0)
	kanji.streams[id] = kanjiStream{
		x: int16(x), y: int16(y), w: int16(w), h: int16(h),
		dx: int16(dx), dy: int16(dy),
		tpage: GetTPage(0, 0, dx, dy),
		clut:  uint16(cy)<<6 | uint16(cx>>4)&0x3f,
		isbg:  isbg != 0,
		n:     n,
		off:   off,
		end:   off,
	}
	kanji.pool[off] = 0
	kanji.used += int(n)
	kanji.open++
	kanji.opened = true
	spinCycles(kanjiFntOpenPad)
	nops(kanjiFntOpenNops)
	return int32(id)
}

// The rest of KanjiFntOpen's 67,200 cycles, refused or not, and of its 48 more for
// each character the stream has room for: 91,880 at the 512 most programs ask for.
// The palette's loadImage came to 146 cycles more with spec 39's work, the queue's
// checks and the call no longer inlined: 156 come off the pad, in whole turns of
// spin, and the 2-cycle nops give 12 back. A refused open's came to 6 more: a
// turn less, and 3 nops more.
const (
	kanjiFntOpenPad         = 66400 - 156
	kanjiFntOpenNops        = 6
	kanjiFntOpenRefusedLess = 12
	kanjiFntOpenRefusedNops = 9
	kanjiFntOpenCharPad     = 48
)

// KanjiFntClose has the next KanjiFntOpen start over from the first stream.
//
//export KanjiFntClose
func KanjiFntClose() {
	kanji.open, kanji.used = 0, 0
}

// KanjiFntPrint appends the string s to stream 0, the first opened, and
// returns the count of what the stream holds, or -1 without appending if
// no stream was ever opened or the count is already past the stream's n. The
// count is of bytes, with the text a conversion makes counted twice, as
// Sony's library counts it. The pool's end stops nothing: the text runs on
// into the packets after it. Percent directives are formatted as printf
// does, %% as a percent sign; with no arguments passed, a numeric
// conversion prints 0 and the others nothing.
//
//export KanjiFntPrint
func KanjiFntPrint(s *byte) int32 {
	if !kanji.opened {
		return -1
	}
	st := &kanji.streams[0]
	if st.count > st.n {
		return -1
	}
	// The text goes on at its NUL, whatever the count says.
	end := st.end
	var added int32
	for p := unsafe.Pointer(s); ; p = unsafe.Add(p, 1) {
		c := *(*byte)(p)
		if c == 0 {
			break
		}
		if c != '%' {
			end = kanji.put(end, c)
			added++
			continue
		}
		// Flags, a width and a precision, which nothing printed here
		// uses, and the conversion.
		for {
			p = unsafe.Add(p, 1)
			if c = *(*byte)(p); !printfFlag(c) {
				break
			}
		}
		switch c {
		case 0:
			p = unsafe.Add(p, -1)
		case '%':
			end = kanji.put(end, '%')
			added++
		case 'd', 'i', 'u', 'o', 'x', 'X', 'p':
			end = kanji.put(end, '0')
			added += 2
		}
	}
	*kanji.textByte(end) = 0
	st.end = end
	st.count += added
	return st.count
}

// printfFlag reports whether c can come between a percent sign and its
// conversion: a flag, a digit of the width or precision, or a size.
func printfFlag(c byte) bool {
	switch c {
	case '-', '+', ' ', '#', '.', 'l', 'h':
		return true
	}
	return c >= '0' && c <= '9'
}

// textByte is the byte i of the block of the pool and the packets.
func (k *kanjiState) textByte(i int) *byte {
	return (*byte)(unsafe.Add(unsafe.Pointer(&k.pool), i))
}

// put stores c at i in the block, unless i is its last byte, kept for the
// NUL, and returns where the next byte goes. Sony's has no such end and
// runs on over its other memory, which is no behavior to keep.
func (k *kanjiState) put(i int, c byte) int {
	if i == kanjiBlock-1 {
		return i
	}
	*k.textByte(i) = c
	return i + 1
}

// KanjiFntFlush draws stream id's text, stream 0's for an id that is not
// open, empties the stream and returns the ordering table it draws
// through. It returns nil, drawing nothing and leaving the text, if no
// stream was ever opened or the text, up to where the window's end stops it, has
// a character the font cannot draw, or more than 301 characters: Sony's
// library breaks at the 302nd, which here counts as one the font has not.
//
//export KanjiFntFlush
func KanjiFntFlush(id int32) *uint32 {
	if !kanji.opened {
		space(kanjiFlushClosedSpace)
		return nil
	}
	if id < 0 || int(id) >= kanji.open {
		id = 0
	}
	r := kanjiFlushFrame(&kanji.streams[id], kanjiFrameExtra+kanjiFrameBytes)
	spinCycles(kanjiFntFlushPad)
	space(kanjiFlushEndSpace)
	return r
}

// kanjiFntFlushPad is what KanjiFntFlush's mean of 98,800 cycles has over the
// work here (spec 21). The stores on each path keep the times the flush had with
// its working memory in .bss, to the cycle: a character, a stream with and without
// isbg, with and without characters, and a flush that fails.
const (
	kanjiFntFlushPad       = 11412
	kanjiFlushCharSpace    = 4
	kanjiFlushQuadSpace    = 2
	kanjiFlushNoBgSpace    = 8
	kanjiFlushEmptySpace   = 2
	kanjiFlushBgEmptySpace = 1
	kanjiFlushBgSpace      = 2
	kanjiFlushEndSpace     = 5
	kanjiFlushBadSpace     = 19
	kanjiFlushBadSJISSpace = 22
	kanjiFlushClosedSpace  = 14
	kanjiFlushNoRoomSpace  = 1
)

// flush lays the stream's text out, loads its glyph image and queues the
// packets that draw it. The packets are built after the text is read, since
// its end may lie in them.
func (st *kanjiStream) flush(f *kanjiFrame) *uint32 {
	// The image and packets of the flush before may still be on their
	// way to the GPU.
	for int32(volatile.LoadUint32(&queue.head)-kanji.sent) < 0 {
		pollQueue()
	}
	next := uint32(endOfList)
	k := 0
	x, y := 0, 0
	p := unsafe.Pointer(kanji.textByte(st.off))
	// The text is laid out as it is read, and checked as it is laid out:
	// a newline starts the next line, a tab moves four cells on, a space
	// one, and a tilde is skipped; each counts toward n. A character is
	// drawn where x and y stand, whatever w and h are, and then x moves
	// on. Whenever x moves and comes to or past w it wraps to the next
	// line, and whenever y moves, by a wrap or a newline, and comes to or
	// past h the text ends there: nothing after it is placed or checked.
	if st.n <= 0 {
		spaceWith(kanjiFlushNoRoomSpace, uint32(st.off))
	}
	for range max(st.n, 0) {
		var c uint16
		if c, p = nextKanji(p); c == 0 {
			break
		}
		moved := false
		switch c {
		case '\n':
			x, y = 0, y+kanjiCell
			moved = true
		case '\t':
			x += 4 * kanjiCell
		case ' ':
			x += kanjiCell
		case '~':
			continue
		default:
			addr, kind := glyphFor(c, false)
			switch kind {
			case glyphBadSJIS:
				ttyPrintf("bad sjis code 0x%x\n", uint32(c), 0)
				spaceWith(kanjiFlushBadSJISSpace, uint32(c))
				return nil
			case glyphBadHankaku:
				spaceWith(kanjiFlushBadSpace, uint32(c))
				return nil
			}
			if k == kanjiDrawn {
				spaceWith(kanjiFlushBadSpace, uint32(c))
				return nil
			}
			f.placed[k] = kanjiPlaced{
				addr: addr,
				x:    st.x + int16(x),
				y:    st.y + int16(y),
			}
			k++
			space(kanjiFlushCharSpace)
			x += kanjiCell
		}
		if !moved && x >= int(st.w) {
			x, y = 0, y+kanjiCell
			moved = true
		}
		if moved && y >= int(st.h) {
			break
		}
	}

	// The image has a cell for each character and one more, blank.
	cells := k + 1
	cols, rows := min(cells, kanjiRow), (cells-1)/kanjiRow+1
	pix := f.image[:cols*2*rows*kanjiCell]
	for i := range cells {
		var addr uintptr
		if i < k {
			addr = f.placed[i].addr
		}
		putGlyph(pix, cols, i%kanjiRow, i/kanjiRow, addr)
	}
	if k == 0 {
		spaceApart(kanjiFlushEmptySpace, uint32(cells))
	}
	for i := range k {
		next = st.quad(i, f.placed[i].x, f.placed[i].y, next)
		space(kanjiFlushQuadSpace)
	}
	loadImage(RECT{X: st.dx, Y: st.dy, W: int16(cols * 4), H: int16(rows * kanjiCell)}, unsafe.Pointer(&pix[0]))
	// On a console the DMA goes on after loadImage returns, and the image goes
	// with the frame. In the emulator it is already over: one read.
	for dmaGPUControl.Get()&dmaBusy != 0 {
	}

	if st.isbg {
		b := &kanji.bg
		b[0] = 3<<24 | next
		b[1] = 0x60000000
		b[2] = pair(st.x, st.y)
		b[3] = pair(st.w, st.h)
		next = physical(unsafe.Pointer(&b[0]))
		if k == 0 {
			spaceWith(kanjiFlushBgEmptySpace, next)
		} else {
			spaceApart(kanjiFlushBgSpace, uint32(k))
		}
	} else {
		space(kanjiFlushNoBgSpace)
	}
	kanji.ot = next
	sendList(unsafe.Pointer(&kanji.ot))
	nops(fntFlushLeaveNops)
	kanji.sent = volatile.LoadUint32(&queue.tail)
	st.count = 0
	st.end = st.off
	kanji.pool[st.off] = 0
	return &kanji.ot
}

// quad builds the packet that draws cell k of the stream's glyph image
// at x, y, 16 pixels square, as an opaque quadrangle in the image's raw
// colors, linked in front of next, and returns its address for the link.
// The command word's color bytes and the high halves of the last two
// texture words are left as they are (spec 13): they were 0 unless text
// ran past the pool.
func (st *kanjiStream) quad(k int, x, y int16, next uint32) uint32 {
	u := uint32(k%kanjiRow) * kanjiCell
	v := uint32(k/kanjiRow) * kanjiCell & 0xff // as a packet holds it, past the page too
	const e = kanjiCell - 1
	q := &kanji.quads[st.off+k]
	q[0] = 9<<24 | next
	*(*byte)(unsafe.Add(unsafe.Pointer(&q[1]), 3)) = 0x2d
	q[2] = pair(x, y)
	q[3] = uint32(st.clut)<<16 | v<<8 | u
	q[4] = pair(x+e, y)
	q[5] = uint32(st.tpage)<<16 | v<<8 | (u + e)
	q[6] = pair(x, y+e)
	*(*uint16)(unsafe.Pointer(&q[7])) = uint16((v+e)<<8 | u)
	q[8] = pair(x+e, y+e)
	*(*uint16)(unsafe.Pointer(&q[9])) = uint16((v+e)<<8 | (u + e))
	return physical(unsafe.Pointer(q))
}
