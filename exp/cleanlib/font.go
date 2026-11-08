//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The debug font: print streams that collect text and draw it as sprites
// from the font FntLoad put into video memory.

const (
	// fontStreams is how many streams can be open at once.
	fontStreams = 8

	// fontChars is the characters all the open streams hold together,
	// Sony's pool (spec 01): 1024 characters of text and a 16-byte sprite
	// packet for each.
	fontChars = 1024

	// glyphSize is a glyph's width and height in pixels, and tabSize what
	// a tab advances by.
	glyphSize = 8
	tabSize   = 32

	// noWrap is the right edge of a stream opened with w 0, which never
	// wraps: beyond any x a line of the pool's 1024 characters can reach,
	// even of tabs (32,768 past the origin).
	noWrap = 1 << 30
)

// A stream is a window on the screen and the text printed to it since it
// was last flushed.
type stream struct {
	x, y, w, h int16

	// xend is the window's right edge, x+w, where a line wraps, and ymax
	// the last y a row is drawn at, y+h-1 (a row is drawn whenever its y
	// is below the bottom edge y+h, which is not rounded to the glyph
	// grid; spec 01), kept so that a flush need not work them out. A
	// stream opened with w 0 never wraps (spec 01): its xend is noWrap,
	// beyond any x a line can reach.
	xend int32
	ymax int16
	isbg bool

	// text holds the stream's room for characters and one byte more, for
	// the NUL after a full stream's text or the character that did not fit
	// (the next stream's first byte, or the pool's spare); packets holds
	// room for a sprite for each character. n is the characters printed
	// since the last flush: one more than the room once a print has been
	// cut, which refuses every print until a flush (spec 01).
	text    []byte
	n       int
	packets []uint32
}

// fontBase is where the font's state is, for FntPrint's assembly: libps_va_FntPrint in the
// target's assembly, as C calls it.
//
//go:extern libps_font_base
var fontBase uintptr

var font struct {
	// tpage and clut are the font's texture page and palette, as a packet
	// gives them, and u, v its place in the page.
	tpage, clut uint16
	u, v        uint8

	// tex is the sprites' texture word less the glyph's offsets: the
	// palette, v and u. cmd is the sprites' command word with the default
	// color, cells the table of the glyphs' cells, fontCells, and link the
	// mask of a tag's link, endOfList: a flush loads them with the rest of
	// what it needs rather than building them from immediates, an
	// instruction or two each. cmdTop is the command byte alone and
	// colorMask keeps a ~c directive's three digits to their bytes: the
	// tilde path loads them where it would otherwise use constants, which
	// the flush's loop hoists into registers before the first character
	// as long as it has registers to spare, so that every flush with text
	// pays for them (spec 08).
	tex       uint32
	cmd       uint32
	cells     *[256]uint16
	link      uint32
	cmdTop    uint32
	colorMask uint32

	// pad is a word the isbg tile's pad stores to: the path holds the
	// font's address, so the store costs 2 where one to spacer, whose
	// address it would first load, costs 4.
	pad uint32

	streams [fontStreams]stream
	open    int // streams opened since FntLoad
	used    int // characters of the pool given to streams

	// text is the pool, and a few bytes more for what is written and read
	// past its end: FntPrint's NUL, or the character that did not fit, in
	// the first of them, and a flush's walk, which skips bytes without
	// looking at them at a tilde (spec 01). The walk may go on past these,
	// but only reads: it builds at most the stream's room in packets.
	text    [fontChars + 4]byte
	packets [fontChars * 4]uint32

	// mode is the packet that sets the texture page for the sprites, the
	// first of every flush's list and what FntFlush returns, and bg the
	// one that clears a window.
	mode [3]uint32
	bg   [4]uint32

	// sent is the drawing queue's tail after the last flush: the queue's
	// head reaches it once that flush's list has gone out, and the
	// packets may be built again.
	sent uint32
}

// FntLoad puts the font into video memory at tx, ty, a 4-bit image 128
// pixels wide and 32 high, and its palette at tx, ty+128, and closes every
// stream.
//
//export FntLoad
func FntLoad(tx, ty int32) {
	fontBase = uintptr(unsafe.Pointer(&font))
	// The palette is transparent for index 0 and white for index 1, which
	// the font draws with; the rest are unused.
	var clut [8]uint32
	clut[0] = 0xffff0000
	loadImage(RECT{X: int16(tx), Y: int16(ty + 128), W: 16, H: 1}, unsafe.Pointer(&clut[0]))
	loadImage(RECT{X: int16(tx), Y: int16(ty), W: 32, H: 32}, unsafe.Pointer(&fontPixels[0]))
	font.tpage = uint16(tx/64) | uint16(ty/256)<<4
	font.clut = uint16(tx/16) | uint16(ty+128)<<6
	font.u = uint8(tx % 64 * 4)
	font.v = uint8(ty % 256)
	font.tex = uint32(font.clut)<<16 | uint32(font.v)<<8 | uint32(font.u)
	font.cmd = 0x74000000 | 0x808080
	font.cmdTop = 0x74000000
	font.colorMask = 0xf0f0f0
	font.cells = &fontCells
	font.link = endOfList
	// The packet that sets the texture page is the same for every flush
	// but for its link, and every flush's list starts with it.
	font.mode[1] = 0xe1000000 | uint32(font.tpage)
	font.mode[2] = 0xe2000000
	font.open, font.used = 0, 0
	// Sony's FntLoad clears its 384 bytes of stream state here, a byte at
	// a time (spec 08); the clean library clears its own, and waits for
	// the bytes it does not have.
	clearBytes(unsafe.Pointer(&font.streams), unsafe.Sizeof(font.streams))
	spinCycles((384-uint32(unsafe.Sizeof(font.streams)))*8 - fntLoadImageTime)
	nops(fntLoadImageNops)
}

// FntLoad's two loadImage calls came to 226 cycles more with spec 39's
// work, the queue's checks and the calls no longer inlined; they are taken
// off its wait, in whole turns of spin and the 2-cycle nops after it.
const (
	fntLoadImageTime = 228
	fntLoadImageNops = 1
)

// FntOpen opens a stream that draws in the window at x, y, w by h, holding
// at most n characters, and returns its id, or -1 if no more can be
// opened. Its count starts at 0 and its first text byte is terminated, so
// that stale text in the pool is not drawn (spec 01). With isbg 1 every
// flush clears the window to black before the text is drawn; with 0 the
// text is drawn over what is there.
//
//export FntOpen
func FntOpen(x, y, w, h, isbg, n int32) int32 {
	id := font.open
	if id == fontStreams {
		return -1
	}
	n = min(max(n, 0), int32(fontChars-font.used))
	start, end := font.used, font.used+int(n)
	xend := x + w
	if w == 0 {
		xend = noWrap
	}
	font.streams[id] = stream{
		x: int16(x), y: int16(y), w: int16(w), h: int16(h),
		isbg:    isbg != 0,
		xend:    xend,
		ymax:    int16(y + h - 1),
		text:    font.text[start : end+1],
		packets: font.packets[4*start : 4*end],
	}
	font.text[start] = 0
	font.used = end
	font.open++
	// Each of the stream's packets gets its tag's length, its code and
	// the font's palette now, as Sony's FntOpen sets them (spec 01); the
	// rest is left as it was until a flush fills it in.
	p := unsafe.Pointer(unsafe.SliceData(font.packets[:]))
	for i := start; i < end; i++ {
		q := unsafe.Add(p, 16*i)
		(*volatile.Register8)(unsafe.Add(q, 3)).Set(3)
		(*volatile.Register8)(unsafe.Add(q, 7)).Set(0x74)
		(*volatile.Register16)(unsafe.Add(q, 14)).Set(font.clut)
	}
	// Sony's FntOpen takes 28 cycles a character over these stores, the
	// loop above 16: the rest is a spin loop's 12 a character.
	spin(uint32(n))
	return int32(id)
}

// FntPrint appends the NUL-terminated string s to stream 0 and a NUL after
// it, and returns the characters in the stream. Each character is written
// and counted before the room is checked, so the one that does not fit
// lands at text[room], no NUL follows it and the result is -1, and every
// print after it until a flush, the empty one too, returns -1 and writes
// nothing. A stream filled exactly is not cut yet (spec 01). With no
// stream open it returns -1. Sony's library writes to stream 0 whichever
// was opened last. s is a printf format to Sony's: "%%" is one "%"; a lone
// "%" is a directive formatted from whatever the arguments are, undefined,
// and is copied as it is here.
//
//export FntPrint
//go:noinline
//go:nobounds
func FntPrint(s *byte) int32 {
	if font.open == 0 {
		return -1
	}
	st := &font.streams[0]
	// The count and the room are kept in locals through the loop: the
	// compiler would reload the fields after every store otherwise. The
	// string is walked as a slice, by index, since a load through a raw
	// pointer carries a nil check each time; its length is nominal (bounds
	// are not checked here), the walk ends at the NUL.
	n := st.n
	room := len(st.text) - 1
	if n > room {
		// A call of its own, so that its pacing does not reshape this
		// function's prologue, which every print pays.
		return fntRefused(s)
	}
	text := st.text
	src := unsafe.Slice(s, fontChars+1)
	n0, j := n, 0
	for {
		c := src[j]
		if c == 0 {
			break
		}
		j++
		text[n] = c
		n++
		if n > room {
			// The character that did not fit is in, and is all the cut
			// costs: Sony's 20 more than the kept characters' print, and
			// nothing for the ones after it (spec 08).
			st.n = n
			space(fntPrintSpace + fntPrintCutSpace)
			pace(fntPrintPad)
			return -1
		}
		if c == '%' && src[j] == '%' {
			// "%%" is one "%": the second is skipped. Sony's charges the
			// pair 10 more than a character (spec 08); the pair's path
			// stores a value of its own so that the compiler keeps its
			// pad apart from the character's.
			j++
			spaceApart(fntPrintPercentSpace, uint32(j))
			continue
		}
		space(fntPrintChar)
	}
	text[n] = 0
	st.n = n
	// Sony's FntPrint costs 168 cycles for an empty string and 182 plus 50
	// a character otherwise (spec 08).
	if n == n0 {
		space(fntPrintEmptySpace)
		pace(fntPrintEmptyPad)
		return int32(n)
	}
	space(fntPrintSpace)
	pace(fntPrintPad)
	return int32(n)
}

// fntRefused is FntPrint on a stream a print has cut: nothing is written and
// the result is -1, for an empty string too. Sony's takes 16 less than an
// empty print for the empty string and 10 more than that for any other,
// whatever its length (spec 08).
//
//go:noinline
func fntRefused(s *byte) int32 {
	if *s == 0 {
		space(fntPrintRefusedEmptySpace)
	}
	pace(fntPrintRefusedPad)
	return -1
}

// FntFlush queues stream id's text for drawing, empties the stream, and
// returns the list it draws: the texture-page packet, which the sprites
// follow, as Sony's does (spec 01). An id that is not open, -1 among them,
// flushes stream 0; with no stream open nothing is drawn and the result is
// nil. It does not wait for the drawing, only for the previous flush's
// packets to have gone out before it builds its own.
//
//export FntFlush
//go:noinline
func FntFlush(id int32) *uint32 {
	if uint32(id) >= uint32(font.open) {
		id = 0
	}
	st := &font.streams[id]
	// With no stream open stream 0 has no text: FntLoad cleared it.
	if st.text == nil {
		return nil
	}
	st.flush()
	return &font.mode[0]
}

// flush draws the stream and empties it.
//
// The layout follows Sony's (spec 01, under FntFlush): a glyph or a space
// advances x by 8 and a tab by 32, and as soon as an advance reaches the
// window's right edge the line wraps, there and then, so that a newline
// right after a filled line leaves a blank row, and a space or a tab that
// reaches the edge wraps as a glyph does.
//
// The text is walked to a NUL or for the stream's room in characters,
// whichever comes first, and the count printed plays no part (spec 01): a
// tilde skips the byte after it whatever it is, so that a tilde that ends
// the text runs the flush on into what the pool holds after it, as Sony's
// does, and counts one however many bytes it takes. After the last
// character one more byte is read, and the walk ends either way, 4 cycles
// later when it is not the NUL (spec 08), the cost of a stream whose print
// was cut. Every byte but a space, a tab, a newline, a tilde and the NUL
// draws a cell of the font, the bytes outside it some cell of the texture
// by the same formula, from fontCells.
//
// The loop over the text is kept lean so that each kind of character can
// be padded to what it costs in Sony's library (spec 08, "FntPrint and
// FntFlush by the character"): a glyph 140 cycles, a space 34, a tab 46,
// a newline 62, 28 more for the glyph, space or tab that fills a line,
// and 52 for the newline and 18 for the wrap that reaches the window's
// bottom. The text is walked by pointer, the paths test for the NUL that
// ends it, then for a space, then for a glyph (anything above a space but
// a tilde) before the rarer characters are told apart (a switch on the
// character would become a jump table), the glyph's cell is one load from
// a table, and the positions are kept in screen coordinates so that a
// sprite's are one mask and one or. What the loop needs of the stream and
// the font is kept worked out in their fields, so that entering the loop
// costs little more than the loads, and the loop makes no call, which
// would have the compiler save registers around it.
//
//go:nobounds
func (st *stream) flush() {
	// The packets of the flush before may still be waiting in the drawing
	// queue. If they were the last thing started and the GPU's channel is idle
	// they have gone, whether or not their interrupt has come in.
	if h := volatile.LoadUint32(&queue.head); int32(h-font.sent) < 0 && (h+1 != font.sent || dmaGPUControl.Get()&dmaBusy != 0) {
		waitSent()
	}
	// The packets are written through a pointer, which could point at
	// anything for all the compiler knows: the stream's and the font's
	// fields are read once before the loop, not again after every packet,
	// and the loop keeps its counts in locals for the same reason.
	packets := unsafe.Pointer(unsafe.SliceData(st.packets))
	p := packets
	text := unsafe.Pointer(unsafe.SliceData(st.text))
	// A text that starts with the NUL draws nothing, and skips the loop
	// and what it loads first, which Sony's empty flush does not pay for
	// either (536 against 676 for one glyph; spec 08).
	if *(*byte)(text) == 0 {
		space(fntFlushEmptySpace)
		nops(2)
	} else {
		cmd := font.cmd
		// The text is walked by pointer from t to a NUL. (The bottom is
		// only tested at a newline or a wrap, so the first row is drawn
		// even in a window 0 tall; spec 01.)
		t := text
		// x and y are the next character's place on the screen; a line
		// runs from x0 to the edge xend, and the rows to ymax, the last y
		// a row is drawn at (the row before the bottom edge, which may
		// overhang it). Once a row below ymax is reached nothing more is
		// drawn.
		x0 := int(st.x)
		x, y := x0, int(st.y)
		xend, ymax := int(st.xend), int(st.ymax)
		// A glyph's texture word is tex plus its cell's v and u. (They are
		// added as bytes in a word rather than wrapping within a byte,
		// which differs only for a font put where its own image does not
		// fit its texture page.)
		tex := font.tex
		cells := font.cells
		link := font.link
		left := len(st.text) - 1
		for ; ; t = unsafe.Add(t, 1) {
			c := *(*byte)(t)
			if c == 0 {
				// The NUL ends the walk 4 cycles sooner than the limit
				// does in Sony's (spec 08): each exit has a pad to set the
				// difference, the limit's storing the character, so that
				// the compiler keeps the two apart.
				space(fntNulExitSpace)
				break
			}
			if left == 0 {
				spaceWith(fntLimitExitSpace, uint32(c))
				break
			}
			left--
			if c == ' ' {
				x += glyphSize
				space(fntSpaceSpace)
				// A space that reaches the edge wraps too, and the wrap
				// costs what a glyph's does.
				if x >= xend {
					x, y = x0, y+glyphSize
					if y > ymax {
						// The wrap that reaches the bottom ends the flush,
						// after a space as after a glyph (Sony's 18). This
						// pad stores x to the other word, so that the
						// compiler does not sink the four end pads into one
						// block (see the counts in gpu.go).
						spaceApart(fntSpaceWrapEndSpace, uint32(x))
						break
					}
					space(fntSpaceWrapSpace)
				}
				continue
			}
			if c < ' ' || c == '~' {
				// The characters that are not glyphs: a newline, a tab and
				// a tilde. The other controls fall through to the glyph's
				// path.
				if c == '\n' {
					x, y = x0, y+glyphSize
					if y > ymax {
						// The newline that reaches the window's bottom ends
						// the flush; Sony's charges it 52, less than one
						// that does not. Its pad stores y, not space's 0:
						// with the same value the compiler hoists the two
						// paths' stores above the test and every newline
						// gets the end's count.
						spaceWith(fntNewlineEndSpace, uint32(y))
						break
					}
					space(fntNewlineSpace)
					continue
				}
				if c == '\t' {
					// A tab advances 32, always, not to a tab stop, and
					// draws nothing.
					x += tabSize
					space(fntTabSpace)
					if x >= xend {
						x, y = x0, y+glyphSize
						if y > ymax {
							// The wrap after a tab that reaches the bottom
							// ends the flush too, at the same 18; its pad
							// is kept apart as the space's is.
							spaceApart(fntSpaceWrapEndSpace, uint32(x))
							break
						}
						space(fntTabWrapSpace)
					}
					continue
				}
				if c == '~' {
					// ~c and three digits sets the color of the text after
					// it, each digit a sixteenth of full brightness, red
					// first; a tilde followed by anything else skips that
					// byte. Neither looks for the NUL (spec 01): the
					// digits, or the skipped byte, are taken from the pool
					// whatever they are.
					if *(*byte)(unsafe.Add(t, 1)) == 'c' {
						cmd = font.cmdTop | textColor(t)
						t = unsafe.Add(t, 4)
					} else {
						t = unsafe.Add(t, 1)
					}
					continue
				}
			}
			space(fntGlyphSpace)
			// Each sprite goes in front of the ones before it, so the list
			// runs from the last character to the first: a packet links
			// to the one below it, and the first packet's link is set to
			// end the list after the loop.
			q := (*[4]uint32)(p)
			q[0] = 3<<24 | uint32(uintptr(unsafe.Add(p, -16)))&link
			q[1] = cmd
			q[2] = uint32(uint16(x)) | uint32(y)<<16
			q[3] = tex + uint32(cells[c])
			p = unsafe.Add(p, 16)
			x += glyphSize
			if x >= xend {
				// The glyph filled its line: the wrap follows it at once.
				x, y = x0, y+glyphSize
				if y > ymax {
					// The wrap that reaches the window's bottom ends the
					// flush; Sony's charges it 18, less than one that does
					// not. Its pad stores y, as the ending newline's does,
					// so that the compiler keeps it apart from the other
					// wrap's stores.
					spaceWith(fntWrapEndSpace, uint32(y))
					break
				}
				space(fntWrapSpace)
			}
		}
	}
	// The sprites come after the packet that sets the texture page and,
	// with isbg, the window's black background, which is drawn even when
	// the stream is empty.
	next := uint32(endOfList)
	if p != packets {
		*(*uint32)(packets) = 3<<24 | endOfList
		next = physical(unsafe.Add(p, -16))
		space(fntGlyphsSpace)
	} else {
		space(fntNoGlyphSpace)
	}
	if st.isbg {
		// The isbg tile costs 40 in Sony's flush (spec 08).
		b := &font.bg
		b[0] = 3<<24 | next
		b[1] = 0x60000000
		b[2] = pair(st.x, st.y)
		b[3] = pair(st.w, st.h)
		if st.w == 0 {
			// A window with no width has its background sized from the
			// text (spec 01).
			b[3] = textSize(st.text, int(st.h))
		}
		next = physical(unsafe.Pointer(&b[0]))
		spaceAt(&font.pad)
	}
	// The flush empties the stream: the NUL at its start ends the next
	// flush's walk where FntPrint's text does not.
	st.text[0] = 0
	font.mode[0] = 2<<24 | next
	sendList(unsafe.Pointer(&font.mode[0]))
	font.sent = volatile.LoadUint32(&queue.tail)
	st.n = 0
}

// waitSent waits for the packets of the flush before to have gone out of
// the drawing queue. It is a call of its own, made only when they have
// not, so that its loop's registers are not saved on every flush.
//
//go:noinline
func waitSent() {
	for int32(volatile.LoadUint32(&queue.head)-font.sent) < 0 {
		pollQueue()
	}
}

// textSize is the background tile's size, as a packet's pair, for a stream
// opened with w 0 and isbg 1, which Sony's library sizes from the text
// (spec 01): as wide as the longest line that a newline ended (0 when no
// newline was printed), and 8 tall for every row started, the row a
// newline moved to counted even when it reached the bottom of a window h
// tall and ended the flush. A stream with w 0 never wraps, so the rows
// are the newlines' alone. text is the stream's, whose length is its room
// and one more: the walk is the flush's, to a NUL or for the room in
// characters.
//
//go:noinline
func textSize(text []byte, h int) uint32 {
	// x and y are in the window; y is the row being started.
	w, x, y := 0, 0, 0
	t := unsafe.Pointer(unsafe.SliceData(text))
	for left := len(text) - 1; ; t = unsafe.Add(t, 1) {
		c := *(*byte)(t)
		if c == 0 || left == 0 {
			break
		}
		left--
		switch c {
		case '\n':
			w = max(w, x)
			x, y = 0, y+glyphSize
			if y >= h {
				// The newline that reaches the bottom ends the flush.
				return pair(int16(w), int16(y+glyphSize))
			}
		case '\t':
			x += tabSize
		case '~':
			// A color directive takes no room, and neither does a tilde
			// and the byte it skips, as in flush.
			if *(*byte)(unsafe.Add(t, 1)) == 'c' {
				t = unsafe.Add(t, 4)
			} else {
				t = unsafe.Add(t, 1)
			}
		default:
			x += glyphSize
		}
	}
	return pair(int16(w), int16(y+glyphSize))
}

// textColor reads the three characters after a ~c directive at t as a
// color word, red in the low byte: each is taken as a digit, its distance
// from '0' times 16, kept to a byte, whatever it is ('F' gives 60h, as the
// reference's does). '0' times 16 is 300h, so the distance is the digit's
// own low nibble: the three bytes are put in a word, shifted up a nibble
// together and masked to their bytes with colorMask. It indexes from the
// tilde's own pointer, which the caller has read through, rather than
// from a pointer two past it, whose nil check the flush's loop would hoist
// as a constant in a register.
func textColor(t unsafe.Pointer) uint32 {
	s := unsafe.Slice((*byte)(t), 5)
	v := uint32(s[2]) | uint32(s[3])<<8 | uint32(s[4])<<16
	return v << 4 & font.colorMask
}

// fontCells is the cell of the font a byte is drawn from, v in the high
// byte and u in the low, to add to the font's texture word. It is Sony's
// formula (spec 01, under FntFlush): k is the byte less 32, or less 64 for
// a to z alone (so that the letters fold to upper case, while '`' and '{'
// do not), u is k mod 16 times 8 and v is k over 16 times 8, with C's
// signed division toward zero, each kept to a byte. So every byte outside
// the font draws some cell of the texture page. The table is one load
// where the division would cost a dozen cycles a glyph, more than the
// glyph's pads hold.
var fontCells = [256]uint16{
	0xf000, 0xf888, 0xf890, 0xf898, 0xf8a0, 0xf8a8, 0xf8b0, 0xf8b8, 0xf8c0, 0xf8c8, 0xf8d0, 0xf8d8, 0xf8e0, 0xf8e8, 0xf8f0, 0xf8f8,
	0xf800, 0x0088, 0x0090, 0x0098, 0x00a0, 0x00a8, 0x00b0, 0x00b8, 0x00c0, 0x00c8, 0x00d0, 0x00d8, 0x00e0, 0x00e8, 0x00f0, 0x00f8,
	0x0000, 0x0008, 0x0010, 0x0018, 0x0020, 0x0028, 0x0030, 0x0038, 0x0040, 0x0048, 0x0050, 0x0058, 0x0060, 0x0068, 0x0070, 0x0078,
	0x0800, 0x0808, 0x0810, 0x0818, 0x0820, 0x0828, 0x0830, 0x0838, 0x0840, 0x0848, 0x0850, 0x0858, 0x0860, 0x0868, 0x0870, 0x0878,
	0x1000, 0x1008, 0x1010, 0x1018, 0x1020, 0x1028, 0x1030, 0x1038, 0x1040, 0x1048, 0x1050, 0x1058, 0x1060, 0x1068, 0x1070, 0x1078,
	0x1800, 0x1808, 0x1810, 0x1818, 0x1820, 0x1828, 0x1830, 0x1838, 0x1840, 0x1848, 0x1850, 0x1858, 0x1860, 0x1868, 0x1870, 0x1878,
	0x2000, 0x1008, 0x1010, 0x1018, 0x1020, 0x1028, 0x1030, 0x1038, 0x1040, 0x1048, 0x1050, 0x1058, 0x1060, 0x1068, 0x1070, 0x1078,
	0x1800, 0x1808, 0x1810, 0x1818, 0x1820, 0x1828, 0x1830, 0x1838, 0x1840, 0x1848, 0x1850, 0x2858, 0x2860, 0x2868, 0x2870, 0x2878,
	0x3000, 0x3008, 0x3010, 0x3018, 0x3020, 0x3028, 0x3030, 0x3038, 0x3040, 0x3048, 0x3050, 0x3058, 0x3060, 0x3068, 0x3070, 0x3078,
	0x3800, 0x3808, 0x3810, 0x3818, 0x3820, 0x3828, 0x3830, 0x3838, 0x3840, 0x3848, 0x3850, 0x3858, 0x3860, 0x3868, 0x3870, 0x3878,
	0x4000, 0x4008, 0x4010, 0x4018, 0x4020, 0x4028, 0x4030, 0x4038, 0x4040, 0x4048, 0x4050, 0x4058, 0x4060, 0x4068, 0x4070, 0x4078,
	0x4800, 0x4808, 0x4810, 0x4818, 0x4820, 0x4828, 0x4830, 0x4838, 0x4840, 0x4848, 0x4850, 0x4858, 0x4860, 0x4868, 0x4870, 0x4878,
	0x5000, 0x5008, 0x5010, 0x5018, 0x5020, 0x5028, 0x5030, 0x5038, 0x5040, 0x5048, 0x5050, 0x5058, 0x5060, 0x5068, 0x5070, 0x5078,
	0x5800, 0x5808, 0x5810, 0x5818, 0x5820, 0x5828, 0x5830, 0x5838, 0x5840, 0x5848, 0x5850, 0x5858, 0x5860, 0x5868, 0x5870, 0x5878,
	0x6000, 0x6008, 0x6010, 0x6018, 0x6020, 0x6028, 0x6030, 0x6038, 0x6040, 0x6048, 0x6050, 0x6058, 0x6060, 0x6068, 0x6070, 0x6078,
	0x6800, 0x6808, 0x6810, 0x6818, 0x6820, 0x6828, 0x6830, 0x6838, 0x6840, 0x6848, 0x6850, 0x6858, 0x6860, 0x6868, 0x6870, 0x6878,
}
