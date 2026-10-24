package main

import "go.chrisrx.dev/psx/yaroze"

// The text box, as Dragon Quest VII's: a box along the bottom of the
// screen with a white border and the scene darkened behind it, in which
// what someone says or what happens is written out a letter at a time, in
// white, wrapped at the box's width, three lines at a time. When the three
// lines are full and there is more, an arrow blinks at the bottom of the
// box until Cross is pressed, and the lines scroll up to make room for
// the next; when all of it has been read, Cross closes the box. Cross
// while the letters are still coming brings the rest of the page at once.
//
// The letters are a sheet of sprites, FONT.TIM, a cell for each printable
// character from the space on, in sixteen columns, with the arrow in the
// last cell; the sheet is written by gen, from glyphs of 5x8 doubled, as
// Dragon Quest's chunky letters were.

const (
	// A character's cell on the sheet and in the box, the box's lines and
	// columns, its padding inside the border, and its size.
	cellW     = 12
	cellH     = 18
	boxCols   = 25
	boxLines  = 3
	boxPad    = 6
	boxBottom = 10 // room under the last line for the arrow
	boxW      = boxCols*cellW + 2*boxPad
	boxH      = boxLines*cellH + boxPad + boxBottom
	arrow     = 127

	// The most text one say holds, and the most lines it wraps to; the
	// frames between the lines scrolling up a line; and how often the
	// arrow blinks.
	maxText    = 512
	maxLines   = 32
	scrollTime = 4
	blinkTime  = 16

	// heroName takes the place of "{Player Name}" in what is said.
	heroName = "Hero"
	namePart = "{Player Name}"
)

// A textBox is the box and the text in it. Its first fields are what
// the tests read: whether it is up, and the line the next letter comes
// on.
type textBox struct {
	up   bool
	line int

	// The text is in buf, n long, wrapped into lines, each a start and
	// an end in the buffer, count of them; top is the first line in the
	// box, col where the next letter comes on its line, wait whether the
	// box waits for Cross, with a page full or the text done, scroll how
	// many lines it has still to scroll up, and tick counts the frames
	// for the scrolling and the blinking.
	buf    [maxText]byte
	n      int
	lines  [maxLines][2]int
	count  int
	top    int
	col    int
	wait   bool
	scroll int
	tick   int

	// glyph is the sprite a letter is drawn with, moved and recut for
	// each; back darkens the scene under the box, and edges are its
	// border: white lines two pixels wide, with a dark line along each
	// side of them, the outer ones stopping short of the corners to round
	// them.
	glyph yaroze.GsSPRITE
	back  yaroze.GsBOXF
	edges [12]yaroze.GsBOXF
	x, y  int16
}

var talk textBox

// setup cuts the letter sprite from the sheet and lays out the box at
// the bottom of the screen.
func (t *textBox) setup(b yaroze.RECT) {
	t.glyph = yaroze.TIM(fontTIM.Data()).Load().Sprite()
	t.glyph.W, t.glyph.H = cellW, cellH
	t.glyph.Mx, t.glyph.My = 0, 0
	t.x, t.y = b.X+(b.W-boxW)/2, b.Y+b.H-8-boxH
	x, y := t.x, t.y
	const w, h = boxW, boxH
	// Semi-transparent, taking its color from what is under it, so the
	// scene shows through darkly.
	t.back = yaroze.GsBOXF{Attribute: 1<<30 | 2<<28, X: x, Y: y, W: w, H: h, R: 150, G: 150, B: 150}
	white := func(n int, x, y int16, w, h uint16) {
		t.edges[n] = yaroze.GsBOXF{X: x, Y: y, W: w, H: h, R: 255, G: 255, B: 255}
	}
	dark := func(n int, x, y int16, w, h uint16) {
		t.edges[n] = yaroze.GsBOXF{X: x, Y: y, W: w, H: h, R: 40, G: 40, B: 48}
	}
	white(0, x, y, w, 2)
	white(1, x, y+h-2, w, 2)
	white(2, x, y, 2, h)
	white(3, x+w-2, y, 2, h)
	dark(4, x, y-1, w, 1)
	dark(5, x, y+h, w, 1)
	dark(6, x-1, y, 1, h)
	dark(7, x+w, y, 1, h)
	dark(8, x+2, y+2, w-4, 1)
	dark(9, x+2, y+h-3, w-4, 1)
	dark(10, x+2, y+2, 1, h-4)
	dark(11, x+w-3, y+2, 1, h-4)
}

// say puts the box up with s, the hero's name in place of namePart, and
// wraps it into lines of at most boxCols letters, at the spaces, and at
// each newline.
func (t *textBox) say(s string) {
	t.n = 0
	for i := 0; i < len(s) && t.n < maxText; i++ {
		if len(s)-i >= len(namePart) && s[i:i+len(namePart)] == namePart {
			t.n += copy(t.buf[t.n:], heroName)
			i += len(namePart) - 1
			continue
		}
		t.buf[t.n] = s[i]
		t.n++
	}
	t.count = 0
	for start := 0; start < t.n && t.count < maxLines; {
		end, cut := start, -1
		for end < t.n && end-start < boxCols && t.buf[end] != '\n' {
			if t.buf[end] == ' ' {
				cut = end
			}
			end++
		}
		// A word cut by the box's edge goes whole onto the next line.
		if end < t.n && t.buf[end] != '\n' && t.buf[end] != ' ' && cut > start {
			end = cut
		}
		t.lines[t.count] = [2]int{start, end}
		t.count++
		start = end
		if start < t.n && (t.buf[start] == ' ' || t.buf[start] == '\n') {
			start++
		}
	}
	t.up = true
	t.top, t.line, t.col = 0, 0, 0
	t.wait = t.count == 0
	t.scroll, t.tick = 0, 0
	hero.walkTick = 0
}

// update brings the next letter, scrolls, and takes Cross, pressed this
// frame when cross: to bring the rest of the page, to scroll on when the
// page is full, and to close the box when the text is done.
func (t *textBox) update(cross bool) {
	t.tick++
	switch {
	case t.scroll > 0:
		if t.tick%scrollTime == 0 {
			t.top++
			t.scroll--
		}
	case t.wait:
		if !cross {
			return
		}
		if t.line >= t.count {
			t.up = false
			return
		}
		t.wait = false
		t.scroll = min(boxLines-1, t.count-t.top-boxLines)
	case cross:
		for !t.wait {
			t.letter()
		}
	default:
		t.letter()
	}
}

// letter brings the next letter, and sets the box waiting when the page
// is full or the text is done.
func (t *textBox) letter() {
	t.col++
	if t.col >= t.lines[t.line][1]-t.lines[t.line][0] {
		t.line++
		t.col = 0
	}
	if t.line >= t.count || t.line-t.top >= boxLines {
		t.wait = true
	}
}

// draw draws the box when it is up: the letters so far of the lines in
// it, the arrow when there is more to read, then the border and the
// darkening under them all, sorted last so that they are drawn first.
func (t *textBox) draw(ot *yaroze.GsOT) {
	if !t.up {
		return
	}
	x0, y0 := t.x+boxPad, t.y+boxPad
	for l := t.top; l < min(t.top+boxLines, t.count) && l <= t.line; l++ {
		s, e := t.lines[l][0], t.lines[l][1]
		if l == t.line {
			e = min(e, s+t.col)
		}
		y := y0 + int16(l-t.top)*cellH
		for k := s; k < e; k++ {
			t.drawGlyph(ot, t.buf[k], x0+int16(k-s)*cellW, y)
		}
	}
	// The arrow's cell has its point on its eighth row, so it sits just
	// above the border.
	if t.wait && t.line < t.count && t.tick&blinkTime == 0 {
		t.drawGlyph(ot, arrow, t.x+(boxW-cellW)/2, t.y+boxH-12)
	}
	for i := range t.edges {
		t.edges[i].Sort(ot, 0)
	}
	t.back.Sort(ot, 0)
}

// drawGlyph draws character c with its top left corner at x, y.
func (t *textBox) drawGlyph(ot *yaroze.GsOT, c byte, x, y int16) {
	if c < ' ' || c > arrow {
		c = '?'
	}
	n := int(c - ' ')
	t.glyph.U, t.glyph.V = uint8(n%16*cellW), uint8(n/16*cellH)
	t.glyph.X, t.glyph.Y = x, y
	t.glyph.SortFast(ot, 0)
}
