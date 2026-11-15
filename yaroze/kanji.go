package yaroze

// The Kanji font: the console's ROM holds Shift-JIS glyphs, 16 by 15
// pixels, which the library draws from a texture it builds in video
// memory. A string for it is Shift-JIS bytes, two a character for the
// Japanese ones and one for ASCII, which the library turns into their
// full-width forms.

// A KanjiText is a window of the Kanji font, which Screen.KanjiFont
// opened: what is printed to it during a frame is drawn at the frame's
// end, and the window empties for the next.
type KanjiText struct {
	id int32
}

// KanjiFont opens a window for Shift-JIS text at x, y, w by h, holding
// up to chars characters at a time, with the glyphs' texture built at
// texX, texY in video memory and their palette at clutX, clutY, and
// returns it. The library has four such windows and 1024 characters
// between them.
func (s *Screen) KanjiFont(x, y, w, h, texX, texY, clutX, clutY, chars int) KanjiText {
	id := kanjiOpen(x, y, w, h, texX, texY, clutX, clutY, chars)
	if id >= 0 && s.nkanji < len(s.kanji) {
		s.kanji[s.nkanji] = id
		s.nkanji++
	}
	return KanjiText{id}
}

// Print adds s, Shift-JIS, to the window. Text past what the window
// holds is dropped.
func (t KanjiText) Print(s string) KanjiText {
	kanjiPrint(t.id, s)
	return t
}

// KanjiImage renders s, Shift-JIS, as a 4-bit TIM into words, which
// holds it, for LoadTIM: 16 pixels a character across and 16 down, a
// row a line of the text, up to 16 characters a row and 256 in all,
// with the pixels at x, y in video memory and the palette at clutX,
// clutY when the TIM is loaded, in fg over bg, each a 16-bit color as a
// TIM holds them. It returns false for a string the ROM has no glyph
// for.
func KanjiImage(s string, words []uint32, x, y, clutX, clutY int, fg, bg uint16) bool {
	return kanjiImage(s, words, x, y, clutX, clutY, fg, bg)
}
