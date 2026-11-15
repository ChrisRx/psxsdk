package yaroze

import "go.chrisrx.dev/psx/geom"

// A Color is a color as the GPU takes it, a byte a component.
type Color struct {
	R, G, B uint8
}

// A BlendMode is how a sprite, a box or a line is blended with what is
// behind it. Its values are the library's attribute word's bits 28 to 30,
// the blend and its rate, which Draw shifts into place.
type BlendMode uint8

const (
	// BlendNone draws it over what is behind it.
	BlendNone BlendMode = 0

	// BlendAverage takes half of it and half of what is behind it.
	BlendAverage BlendMode = 4

	// BlendAdd adds it to what is behind it.
	BlendAdd BlendMode = 5

	// BlendSubtract takes it away from what is behind it.
	BlendSubtract BlendMode = 6

	// BlendQuarter adds a quarter of it to what is behind it.
	BlendQuarter BlendMode = 7
)

// attrUnlit is the bit of the library's attribute word that leaves a
// sprite's Color out, which its Draw sets for Unlit.
const attrUnlit = 1 << 6

// attribute is the blend as the attribute word takes it.
func (b BlendMode) attribute() uint32 { return uint32(b&7) << 28 }

// A Texture names an image in video memory as a sprite shows it: the
// texture page, the corner within it, and the palette. Image.Texture
// gives one.
type Texture struct {
	// Page is the texture page as the GPU takes it, as GetTPage makes
	// it: where in video memory the image lies, and its color depth,
	// 4, 8 or 16 bits a pixel, in bits 7 and 8.
	Page uint16

	// U and V are the image's corner within the page, and CX and CY
	// where its palette is in video memory.
	U, V   uint8
	CX, CY int16
}

// depth is the texture's color depth as the attribute word takes it.
func (t Texture) depth() uint32 { return uint32(t.Page) >> 7 & 3 << 24 }

// A Sprite is a rectangle of an image drawn on the screen, scaled and
// turned about its pivot if it has any of either. X and Y are where the
// pivot goes; W and H are the size cut from the texture, from U, V.
// Image.Sprite makes one showing the whole image.
//
// A Sprite starts with the library's own layout, so drawing one copies
// nothing: Draw packs the attribute word the library reads from Blend,
// Hidden, Unlit and the texture's color depth.
type Sprite struct {
	attribute uint32
	X, Y      int16
	W, H      uint16
	Texture   Texture

	// Color tints the sprite: 128 a component leaves it as it is.
	Color Color
	_     uint8

	// Pivot is the point of the sprite that X, Y places and that it is
	// turned and scaled about, from its top left corner.
	Pivot geom.Point

	// ScaleX and ScaleY are its size, One for its own.
	ScaleX, ScaleY geom.Fixed

	// Rotate turns it clockwise about the pivot.
	Rotate geom.Angle

	// Blend is how it is blended with what is behind it.
	Blend BlendMode

	// Hidden leaves it undrawn.
	Hidden bool

	// Unlit draws the texture's colors as they are, ignoring Color.
	Unlit bool
}

// Draw puts the sprite on the frame at depth, 0 in front. A sprite that
// is neither turned nor scaled is drawn the quicker way, a plain sprite
// placed by its pivot, which the library chooses for itself.
func (sp *Sprite) Draw(f *Frame, depth int) {
	if sp.Hidden {
		return
	}
	sp.attribute = sp.pack()
	sortSprite(sp, f, depth)
}

// DrawFast puts the sprite on the frame at depth without its scaling
// and rotation, with X, Y its top left corner, which is quicker still.
func (sp *Sprite) DrawFast(f *Frame, depth int) {
	if sp.Hidden {
		return
	}
	sp.attribute = sp.pack()
	sortFastSprite(sp, f, depth)
}

// pack is the attribute word of the sprite's blend, color depth and
// lighting.
func (sp *Sprite) pack() uint32 {
	attr := sp.Blend.attribute() | sp.Texture.depth()
	if sp.Unlit {
		attr |= attrUnlit
	}
	return attr
}

// MoveTo puts the sprite's pivot at x, y. The layout's fields are 16 bits,
// and MoveTo is how a program working in ints sets them.
func (sp *Sprite) MoveTo(x, y int) { sp.X, sp.Y = int16(x), int16(y) }

// Position returns where the sprite's pivot is, as MoveTo puts it.
func (sp *Sprite) Position() (x, y int) { return int(sp.X), int(sp.Y) }

// Size returns the sprite's width and height, as cut from the texture.
func (sp *Sprite) Size() (w, h int) { return int(sp.W), int(sp.H) }

// A Box is a filled rectangle of one color. Like a Sprite, it starts with
// the library's layout, and Draw packs the attribute word.
type Box struct {
	attribute uint32
	X, Y      int16
	W, H      uint16
	Color     Color
	_         uint8

	// Blend is how it is blended with what is behind it.
	Blend BlendMode

	// Hidden leaves it undrawn.
	Hidden bool
}

// Draw puts the box on the frame at depth.
func (b *Box) Draw(f *Frame, depth int) {
	if b.Hidden {
		return
	}
	b.attribute = b.Blend.attribute()
	sortBox(b, f, depth)
}

// MoveTo puts the box's top left corner at x, y.
func (b *Box) MoveTo(x, y int) { b.X, b.Y = int16(x), int16(y) }

// Position returns where the box's top left corner is.
func (b *Box) Position() (x, y int) { return int(b.X), int(b.Y) }

// Size returns the box's width and height.
func (b *Box) Size() (w, h int) { return int(b.W), int(b.H) }

// Rect is the box's position and size as a Rect.
func (b *Box) Rect() geom.Rect {
	return geom.Rect{
		X: int(b.X),
		Y: int(b.Y),
		W: int(b.W),
		H: int(b.H),
	}
}

// SetRect places and sizes the box to cover r.
func (b *Box) SetRect(r geom.Rect) {
	b.X, b.Y, b.W, b.H = int16(r.X), int16(r.Y), uint16(r.W), uint16(r.H)
}

// A Line is a straight line of one color from one point to another.
type Line struct {
	attribute uint32
	X0, Y0    int16
	X1, Y1    int16
	Color     Color
	_         uint8

	// Blend is how it is blended with what is behind it.
	Blend BlendMode

	// Hidden leaves it undrawn.
	Hidden bool
}

// Draw puts the line on the frame at depth.
func (l *Line) Draw(f *Frame, depth int) {
	if l.Hidden {
		return
	}
	l.attribute = l.Blend.attribute()
	sortLine(l, f, depth)
}

// SetEnds puts the line's ends at x0, y0 and x1, y1.
func (l *Line) SetEnds(x0, y0, x1, y1 int) {
	l.X0, l.Y0, l.X1, l.Y1 = int16(x0), int16(y0), int16(x1), int16(y1)
}

// A GradientLine is a line shaded from one color at its first point to
// another at its second.
type GradientLine struct {
	attribute uint32
	X0, Y0    int16
	X1, Y1    int16
	From      Color
	To        Color
	_         [2]uint8

	// Blend is how it is blended with what is behind it.
	Blend BlendMode

	// Hidden leaves it undrawn.
	Hidden bool
}

// Draw puts the line on the frame at depth.
func (l *GradientLine) Draw(f *Frame, depth int) {
	if l.Hidden {
		return
	}
	l.attribute = l.Blend.attribute()
	sortGradientLine(l, f, depth)
}

// SetEnds puts the line's ends at x0, y0 and x1, y1.
func (l *GradientLine) SetEnds(x0, y0, x1, y1 int) {
	l.X0, l.Y0, l.X1, l.Y1 = int16(x0), int16(y0), int16(x1), int16(y1)
}
