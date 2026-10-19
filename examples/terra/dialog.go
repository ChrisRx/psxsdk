package main

// Conversations: a box at the foot of the screen with the speaker's
// portrait and their words appearing a glyph at a time, Cross or Square
// hurrying them and going on to the next line, each line able to wait on
// the story and to advance it when it ends.

import "go.chrisrx.dev/psx/yaroze"

// A line is one thing said: who says it, the story flag it waits for, or
// -1, the words in Japanese and in English, and the story flag it
// advances when it ends, or -1.
type line struct {
	portrait int
	waitFor  int
	jp, en   string
	advance  int
}

var (
	dialogOn    bool
	dialogHid   bool // the player was hidden for it
	dialogNo    int
	dialogLines []line
	dialogLeft  int
	glyphCount  int
	glyphsShown int
	glyphTick   int
	portraitPkt [5]uint32
	glyphPkts   [64][5]uint32
	maxGlyphs   = 64
	glyphBuf    [320]byte
)

// startDialog starts conversation n, hiding the player if hide is set.
func startDialog(n int, hide bool) {
	dialogNo = n
	dialogLines = dialogs[n]
	dialogLeft = len(dialogLines)
	dialogHid = hide
	layoutDialog(&dialogLines[0])
	if hide {
		playerHide(true)
	}
	dialogOn = true
}

// playerHide hides the player, with everything that runs under the root,
// or shows them again.
func playerHide(hide bool) {
	if hide {
		entRoot.frame &^= entFrameActive
	} else {
		entRoot.frame |= entFrameActive
	}
}

// layoutDialog lays out a line: the portrait, and a sprite for each glyph
// of its words in the language chosen, at 11 pixels a Japanese glyph and
// its own width plus one for a Roman one, with %n for a new line, %0 to
// %7 for the buttons, and %e for the end.
func layoutDialog(l *line) {
	tex := &panels[l.portrait]
	portraitPkt[0] = 4 << 24
	portraitPkt[1] = 0x65 << 24
	portraitPkt[2] = xy(-0x76, 0x20)
	portraitPkt[3] = uint32(tex.u) | uint32(tex.v)<<8 | uint32(tex.clut())<<16
	portraitPkt[4] = uint32(uint16(tex.w)) | uint32(uint16(tex.h))<<16
	text := l.en
	if language == 0 && l.jp != "" {
		text = l.jp
	}
	font := &panels[0]
	clut := uint32(font.clut())
	x, y := 100, 0x9a
	glyphCount = 0
	glyphsShown = 1
	i := 0
	for i < len(text) && glyphCount < maxGlyphs {
		c := text[i]
		if c != '%' {
			g := &glyphPkts[glyphCount]
			g[0] = 4 << 24
			g[1] = 0x67 << 24
			g[2] = xy(int16(x-0xa0), int16(y-0x78))
			if c+0x80 < 0x20 || c > 0xdf {
				// A Japanese glyph, two bytes, ten by eleven.
				idx := glyphIndexJP(text[i:])
				u := uint32(uint8((idx % 25) * 10))
				v := uint32(uint8((idx / 25) * 11))
				g[3] = u | v<<8 | clut<<16
				g[4] = 10 | 0xb<<16
				x += 11
				i += 2
			} else {
				idx := glyphIndex(c)
				u := uint32(uint8((idx % 25) * 10))
				v := uint32(uint8((idx/25)*15 - 0x40))
				g[3] = u | v<<8 | clut<<16
				w := int(glyphWidth(idx))
				g[4] = uint32(w) | 0xf<<16
				x += w + 1
				i++
			}
			glyphCount++
			continue
		}
		if i+1 >= len(text) {
			break
		}
		cmd := text[i+1]
		switch {
		case cmd >= '0' && cmd <= '3':
			// The four buttons' icons are the panels after the portraits.
			tex := &panels[8+int(cmd-'0')]
			g := &glyphPkts[glyphCount]
			g[0] = 4 << 24
			g[1] = 0x65 << 24
			dy := -0x76
			if language == 1 {
				dy = -0x74
			}
			g[2] = xy(int16(x-0xa0), int16(y+dy))
			g[3] = uint32(tex.u) | uint32(tex.v)<<8 | uint32(tex.clut())<<16
			g[4] = 9 | 9<<16
			x += 11
			glyphCount++
		case cmd >= '4' && cmd <= '7':
			tex := &panels[12]
			g := &glyphPkts[glyphCount]
			g[0] = 4 << 24
			g[1] = 0x65 << 24
			dy := -0x76
			if language == 1 {
				dy = -0x74
			}
			g[2] = xy(int16(x-0xa0), int16(y+dy))
			g[3] = uint32(uint8(int(tex.u)+int(cmd-'4')*0x18)) | uint32(tex.v)<<8 | uint32(tex.clut())<<16
			g[4] = 0x13 | 9<<16
			x += 0x16
			glyphCount++
		case cmd == 'e':
			i = len(text)
			continue
		case cmd == 'n':
			x = 100
			y += 0x10
		}
		i += 2
	}
}

// glyphIndex is the glyph's place in the font, or the last glyph's for one
// the font lacks, as the original ran off the end of its table.
func glyphIndex(c byte) int {
	for i := 0; i < len(glyphs); i++ {
		if glyphs[i] == c {
			return i
		}
	}
	return len(glyphs) - 1
}

// glyphIndexJP is the place in the font of the Japanese glyph that starts
// s, two bytes of Shift JIS.
func glyphIndexJP(s string) int {
	n := len(glyphsJP) / 2
	for i := 0; i < n; i++ {
		if len(s) >= 2 && glyphsJP[2*i] == s[0] && glyphsJP[2*i+1] == s[1] {
			return i
		}
	}
	return n - 1
}

// glyphWidth is the width the font gives glyph i.
func glyphWidth(i int) uint8 {
	if i < len(glyphWidths) {
		return glyphWidths[i]
	}
	return 10
}

// drawDialog draws the box: the portrait, the glyphs shown so far, the
// bevel around it, the shadows, the patterned panel behind, and the
// screen under it blended back over, which frosts it.
func drawDialog() {
	over := screen.Overlay()
	tag := over.Entry(1)
	l := &dialogLines[0]
	p := yaroze.AllocPacket(5)
	copy(p, portraitPkt[:])
	yaroze.LinkTag(tag, p)
	tex := &panels[l.portrait]
	q := yaroze.AllocPacket(2)
	q[0] = 1 << 24
	q[1] = 0xe1000200 | uint32(tex.tpageY>>4&0x10) | uint32((tex.tpageX&0x3ff)>>6) | 0x80 | uint32((tex.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, q)
	for i := 0; i < glyphsShown && i < glyphCount; i++ {
		g := yaroze.AllocPacket(5)
		copy(g, glyphPkts[i][:])
		yaroze.LinkTag(tag, g)
	}
	bevel(tag, -0x80, 0x18, 0x7f, 0x18, 0xffffff)
	bevel(tag, -0x80, 0x18, -0x80, 0x67, 0xffffff)
	bevel(tag, -0x80, 0x67, 0x7f, 0x67, 0)
	bevel(tag, 0x7f, 0x18, 0x7f, 0x67, 0)
	font := &panels[0]
	q = yaroze.AllocPacket(2)
	q[0] = 1 << 24
	q[1] = 0xe1000200 | uint32(font.tpageY>>4&0x10) | uint32((font.tpageX&0x3ff)>>6) | uint32((font.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, q)
	fillRect(tag, -0x7c, 0x68, 0x100, 4, 0x20, 0x20, 0x20, true, 2)
	fillRect(tag, 0x80, 0x1c, 4, 0x4c, 0x20, 0x20, 0x20, true, 2)
	panel := &panels[1]
	windowSprite(tag, -0x80, 0x18, 0x100, 0x50, panel, 0x80, 0x80, 0x80, true, 0)
	screenSprite(tag, -0x80, 0x18, 0x100, 0x50, int16((1-yaroze.GsGetActiveBuff())*320)+0x28, 0x98, 0x80, 0x80, 0x80, false, 2)
}

// bevel links a semi-transparent line from one point to another, in the
// color, at tag.
func bevel(tag *uint32, x0, y0, x1, y1 int16, c uint32) {
	b := yaroze.AllocPacket(4)
	b[0] = 3 << 24
	b[1] = 0x42<<24 | c
	b[2] = xy(x0, y0)
	b[3] = xy(x1, y1)
	yaroze.LinkTag(tag, b)
}

// windowSprite links a sprite of w by h filled with the texture repeated,
// through a texture window set to its size, at tag, semi-transparent at
// the rate or opaque.
func windowSprite(tag *uint32, x, y, w, h int16, tex *texture, r, g, b uint8, semi bool, abr uint32) {
	off := yaroze.AllocPacket(3)
	off[0] = 2 << 24
	off[1] = 0xe2000000
	off[2] = 0
	yaroze.LinkTag(tag, off)
	p := yaroze.AllocPacket(5)
	p[0] = 4 << 24
	code := uint32(0x64)
	if semi {
		code = 0x66
	}
	p[1] = code<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	p[2] = xy(x, y)
	// The corner is worked out as for a 4-bit texture whatever the
	// texture's depth, and the page is sent without a depth, as the
	// original had them: the panel's pattern comes out as it did there.
	p[3] = uint32(uint8(tex.px&0x3f)<<2) | uint32(tex.v)<<8 | uint32(tex.clut())<<16
	p[4] = uint32(uint16(w)) | uint32(uint16(h))<<16
	yaroze.LinkTag(tag, p)
	q := yaroze.AllocPacket(2)
	q[0] = 1 << 24
	q[1] = 0xe1000200 | abr&3<<5 | uint32(tex.tpageY>>4&0x10) | uint32((tex.tpageX&0x3ff)>>6) | uint32((tex.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, q)
	// The window's masks come out of the texture's size the way the
	// original worked them, which repeats a texture of 32 by 16.
	win := yaroze.AllocPacket(3)
	win[0] = 2 << 24
	tw, th := int(tex.w), int(tex.h)
	win[1] = 0xe2000000 | uint32(uint8(tex.v))<<12 | uint32(uint8(tex.u))<<7 | uint32(th<<2|th<<3|th<<4|th<<5)&0x3e0 | uint32(tw>>3|tw>>2|tw>>1|tw)&0x1f
	win[2] = 0
	yaroze.LinkTag(tag, win)
}

// screenSprite links sprites copying a w by h area of video memory at
// sx, sy, 16-bit pixels of the frame buffer, to x, y: one sprite for each
// texture page the area crosses, as the original split it, which for
// 16-bit pixels takes a page for 256 wide and so wraps within one.
func screenSprite(tag *uint32, x, y, w, h, sx, sy int16, r, g, b uint8, semi bool, depth uint32) {
	code := uint32(0x65)
	if semi {
		code |= 2
	}
	shift := 2 - depth
	for ty, v := int(y), int(sy); ty < int(y)+int(h); {
		rows := 0x100 - v&0xff
		if int(y)+int(h)-ty < rows {
			rows = int(y) + int(h) - ty
		}
		for tx, u := int(x), int(sx); tx < int(x)+int(w); {
			cols := 0x100 - (u&0x3f)<<shift
			if int(x)+int(w)-tx < cols {
				cols = int(x) + int(w) - tx
			}
			p := yaroze.AllocPacket(5)
			p[0] = 4 << 24
			p[1] = code<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
			p[2] = xy(int16(tx), int16(ty))
			p[3] = uint32(uint8((u&0x3f)<<shift)) | uint32(uint8(v))<<8
			p[4] = uint32(uint16(cols)) | uint32(uint16(rows))<<16
			yaroze.LinkTag(tag, p)
			q := yaroze.AllocPacket(2)
			q[0] = 1 << 24
			q[1] = 0xe1000200 | depth&3<<7 | uint32((v&0x100)>>4) | uint32((u&0x3c0)>>6)
			yaroze.LinkTag(tag, q)
			tx += cols
			u = (u + cols>>shift) & 0x3c0
		}
		ty += rows
		v = (v + 0x100) & 0x100
	}
}

// dialogFrame runs the conversation's frame, and then the scenes over
// it: a line waiting on the story draws nothing until it may be shown;
// otherwise its glyphs come out one every other frame, every frame while
// Cross or Square is held, and a press once they are all out goes on to
// the next line or ends the conversation, counting a found item.
func dialogFrame() {
	if dialogOn {
		l := &dialogLines[0]
		if glyphsShown == 1 && l.waitFor != -1 && flags[l.waitFor] == 0 {
			return
		}
		drawDialog()
		if glyphsShown == glyphCount {
			if padPressedRaw()&(yaroze.PADRdown|yaroze.PADRleft) != 0 {
				if l.advance != -1 && flags[l.advance] < 0xff {
					flags[l.advance]++
				}
				dialogLeft--
				if dialogLeft == 0 {
					if dialogHid {
						playerHide(false)
					}
					switch dialogNo {
					case 5:
						keys++
					case 4:
						potions++
					case 6:
						hearts++
						playerEnt.hp += 0x10
					}
					dialogOn = false
				} else {
					dialogLines = dialogLines[1:]
					layoutDialog(&dialogLines[0])
				}
			}
		} else {
			tick := 2
			if padHeldRaw()&(yaroze.PADRdown|yaroze.PADRleft) == 0 {
				tick = glyphTick + 1
			}
			glyphTick = tick
			if tick > 1 {
				glyphTick = 0
				glyphsShown++
			}
		}
	}
	if titleOn {
		titleFrame()
	}
	if scene2On {
		scene2Frame()
	}
	if scene3On {
		scene3Frame()
	}
}
