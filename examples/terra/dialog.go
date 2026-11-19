package main

// Conversations: a box at the foot of the screen with the speaker's
// portrait and their words appearing a glyph at a time, Cross or Square
// hurrying them and going on to the next line, each line able to wait on
// the story and to advance it when it ends.

import (
	"go.chrisrx.dev/psx/gpu"
	"go.chrisrx.dev/psx/yaroze"
)

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
	portraitPkt gpu.Sprite
	glyphPkts   [64]gpu.Sprite
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
	portraitPkt = rawSprite(-0x76, 0x20, tex.u, tex.v, tex.clut(), int(tex.w), int(tex.h))
	text := l.en
	if language == 0 && l.jp != "" {
		text = l.jp
	}
	font := &panels[0]
	clut := font.clut()
	x, y := 100, 0x9a
	glyphCount = 0
	glyphsShown = 1
	i := 0
	for i < len(text) && glyphCount < maxGlyphs {
		c := text[i]
		if c != '%' {
			g := &glyphPkts[glyphCount]
			if c+0x80 < 0x20 || c > 0xdf {
				// A Japanese glyph, two bytes, ten by eleven.
				idx := glyphIndexJP(text[i:])
				*g = rawSprite(x-0xa0, y-0x78, uint8((idx%25)*10), uint8((idx/25)*11), clut, 10, 0xb)
				x += 11
				i += 2
			} else {
				idx := glyphIndex(c)
				w := int(glyphWidth(idx))
				*g = rawSprite(x-0xa0, y-0x78, uint8((idx%25)*10), uint8((idx/25)*15-0x40), clut, w, 0xf)
				x += w + 1
				i++
			}
			g.Code |= gpu.CodeTranslucent
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
			dy := -0x76
			if language == 1 {
				dy = -0x74
			}
			glyphPkts[glyphCount] = rawSprite(x-0xa0, y+dy, tex.u, tex.v, tex.clut(), 9, 9)
			x += 11
			glyphCount++
		case cmd >= '4' && cmd <= '7':
			tex := &panels[12]
			dy := -0x76
			if language == 1 {
				dy = -0x74
			}
			glyphPkts[glyphCount] = rawSprite(x-0xa0, y+dy, uint8(int(tex.u)+int(cmd-'4')*0x18), tex.v, tex.clut(), 0x13, 9)
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

// rawSprite returns a sprite packet of w by h at x, y, of the texture
// from u, v drawn in its own colors with the palette clut, to be linked
// later by linkSprite.
func rawSprite(x, y int, u, v uint8, clut uint16, w, h int) gpu.Sprite {
	var s gpu.Sprite
	s.Init()
	s.Color = gpu.Color{}
	s.Code |= gpu.CodeRaw
	s.XY = gpu.At(x, y)
	s.UV = gpu.UV{U: u, V: v}
	s.Clut = clut
	s.W, s.H = uint16(w), uint16(h)
	return s
}

// linkSprite links a copy of the sprite packet s into f at depth.
func linkSprite(f *yaroze.Frame, depth int, s *gpu.Sprite) {
	p := f.Packet[gpu.Sprite](depth)
	tag := p.Tag
	*p = *s
	p.Tag = tag
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
	const depth = 1
	l := &dialogLines[0]
	linkSprite(over, depth, &portraitPkt)
	tex := &panels[l.portrait]
	over.Packet[gpu.DrawMode](depth).Page = tex.mode() | 0x80
	for i := 0; i < glyphsShown && i < glyphCount; i++ {
		linkSprite(over, depth, &glyphPkts[i])
	}
	bevel(over, depth, -0x80, 0x18, 0x7f, 0x18, gray(0xff))
	bevel(over, depth, -0x80, 0x18, -0x80, 0x67, gray(0xff))
	bevel(over, depth, -0x80, 0x67, 0x7f, 0x67, gray(0))
	bevel(over, depth, 0x7f, 0x18, 0x7f, 0x67, gray(0))
	font := &panels[0]
	over.Packet[gpu.DrawMode](depth).Page = font.mode()
	fillRect(over, depth, -0x7c, 0x68, 0x100, 4, 0x20, 0x20, 0x20, true, 2)
	fillRect(over, depth, 0x80, 0x1c, 4, 0x4c, 0x20, 0x20, 0x20, true, 2)
	panel := &panels[1]
	windowSprite(over, depth, -0x80, 0x18, 0x100, 0x50, panel, 0x80, 0x80, 0x80, true, 0)
	screenSprite(over, depth, -0x80, 0x18, 0x100, 0x50, int16((1-yaroze.GsGetActiveBuff())*320)+0x28, 0x98, 0x80, 0x80, 0x80, false, 2)
}

// bevel links a semi-transparent line from one point to another, in the
// color, into f at depth.
func bevel(f *yaroze.Frame, depth int, x0, y0, x1, y1 int16, c gpu.Color) {
	b := f.Packet[gpu.Line](depth)
	b.Color = c
	b.Code |= gpu.CodeTranslucent
	b.XY0 = gpu.XY{X: x0, Y: y0}
	b.XY1 = gpu.XY{X: x1, Y: y1}
}

// windowSprite links a sprite of w by h filled with the texture repeated,
// through a texture window set to its size, into f at depth,
// semi-transparent at the rate or opaque.
func windowSprite(f *yaroze.Frame, depth int, x, y, w, h int16, tex *texture, r, g, b uint8, semi bool, abr uint32) {
	// The window is put back to none after the sprite, which the packet
	// linked first, drawn last, does.
	f.Packet[gpu.TextureWindow](depth)
	p := f.Packet[gpu.Sprite](depth)
	p.Color = gpu.Color{
		R: r,
		G: g,
		B: b,
	}
	if semi {
		p.Code |= gpu.CodeTranslucent
	}
	p.XY = gpu.XY{X: x, Y: y}
	// The corner is worked out as for a 4-bit texture whatever the
	// texture's depth, and the page is sent without a depth, as the
	// original had them: the panel's pattern comes out as it did there.
	p.UV = gpu.UV{U: uint8(tex.px&0x3f) << 2, V: tex.v}
	p.Clut = tex.clut()
	p.W, p.H = uint16(w), uint16(h)
	f.Packet[gpu.DrawMode](depth).Page = tex.mode() | abr&3<<5
	// The window's masks come out of the texture's size the way the
	// original worked them, which repeats a texture of 32 by 16.
	tw, th := int(tex.w), int(tex.h)
	win := f.Packet[gpu.TextureWindow](depth)
	win.Window |= uint32(tex.v)<<12 | uint32(tex.u)<<7 | uint32(th<<2|th<<3|th<<4|th<<5)&0x3e0 | uint32(tw>>3|tw>>2|tw>>1|tw)&0x1f
}

// screenSprite links sprites copying a w by h area of video memory at
// sx, sy, 16-bit pixels of the frame buffer, to x, y: one sprite for each
// texture page the area crosses, as the original split it, which for
// 16-bit pixels takes a page for 256 wide and so wraps within one. The
// pixels are bits deep by the GPU's count: 0 for 4, 1 for 8, 2 for 16.
func screenSprite(f *yaroze.Frame, depth int, x, y, w, h, sx, sy int16, r, g, b uint8, semi bool, bits uint32) {
	shift := 2 - bits
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
			p := f.Packet[gpu.Sprite](depth)
			p.Color = gpu.Color{
				R: r,
				G: g,
				B: b,
			}
			p.Code |= gpu.CodeRaw
			if semi {
				p.Code |= gpu.CodeTranslucent
			}
			p.XY = gpu.At(tx, ty)
			p.UV = gpu.UV{U: uint8((u & 0x3f) << shift), V: uint8(v)}
			p.Clut = 0
			p.W, p.H = uint16(cols), uint16(rows)
			f.Packet[gpu.DrawMode](depth).Page = modeDither | bits&3<<7 | uint32((v&0x100)>>4) | uint32((u&0x3c0)>>6)
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
