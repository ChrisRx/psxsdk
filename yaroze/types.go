package yaroze

// A Color is a color as the GPU takes it, a byte a component.
type Color struct {
	R, G, B uint8
}

// A Point is a position on the screen.
type Point struct {
	X, Y int16
}

// A Rect is a rectangle on the screen or in video memory.
type Rect struct {
	X, Y int
	W, H int
}

// Overlaps says whether the two rectangles share any pixel.
func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// A Mode is how a sprite or a box is drawn: a set of the Mode bits.
type Mode uint32

const (
	// Translucent blends the sprite with what is behind it, by the blend
	// in the mode: Average unless Add, Subtract or Quarter is set.
	Translucent Mode = 1 << 30

	// Average, the blend when no other is set, takes half the sprite and
	// half the background; Add adds the sprite to the background,
	// Subtract takes it away, and Quarter adds a quarter of it.
	Average  Mode = 0 << 28
	Add      Mode = 1 << 28
	Subtract Mode = 2 << 28
	Quarter  Mode = 3 << 28

	// Unlit draws the texture's colors as they are, ignoring Color.
	Unlit Mode = 1 << 6

	// Upright skips rotation and scaling, which is faster, for a sprite
	// that has neither.
	Upright Mode = 1 << 27

	// Hidden leaves the sprite undrawn.
	Hidden Mode = 1 << 31

	// The texture's color depth, which Image.Sprite sets: 4 bits a pixel,
	// 8, or 16.
	Depth4  Mode = 0 << 24
	Depth8  Mode = 1 << 24
	Depth16 Mode = 2 << 24
)

// A Fixed is a number with twelve bits of fraction: One is 1.0.
type Fixed int16

// One is 1.0 as a Fixed.
const One Fixed = 4096

// An Angle is a turn in units of a 4096th of a degree.
type Angle int32

// Degrees returns the angle of d degrees.
func Degrees(d int) Angle { return Angle(d) << 12 }

// A Texture names an image in video memory as a sprite shows it: the
// texture page, the corner within it, and the palette. Image.Texture
// gives one.
type Texture struct {
	Page   uint16
	U, V   uint8
	CX, CY int16
}

// A Sprite is a rectangle of an image drawn on the screen, scaled and
// turned about its pivot if it has any of either. X and Y are where the
// pivot goes; W and H are the size cut from the texture, from U, V.
// Image.Sprite makes one showing the whole image.
//
// A Sprite is the library's own layout, so drawing one copies nothing.
type Sprite struct {
	Mode    Mode
	X, Y    int16
	W, H    uint16
	Texture Texture

	// Color tints the sprite: 128 a component leaves it as it is.
	Color Color
	_     uint8

	// Pivot is the point of the sprite that X, Y places and that it is
	// turned and scaled about, from its top left corner.
	Pivot Point

	// ScaleX and ScaleY are its size, One for its own.
	ScaleX, ScaleY Fixed

	// Rotate turns it clockwise about the pivot.
	Rotate Angle
}

// Draw puts the sprite on the frame at depth, 0 in front.
func (sp *Sprite) Draw(f *Frame, depth int) { sortSprite(sp, f, depth) }

// DrawFast puts the sprite on the frame at depth without its scaling
// and rotation, with X, Y its top left corner, which is quicker than
// Draw with Upright.
func (sp *Sprite) DrawFast(f *Frame, depth int) { sortFastSprite(sp, f, depth) }

// MoveTo puts the sprite's pivot at x, y. The layout's fields are 16 bits,
// and MoveTo is how a program working in ints sets them.
func (sp *Sprite) MoveTo(x, y int) { sp.X, sp.Y = int16(x), int16(y) }

// Position returns where the sprite's pivot is, as MoveTo puts it.
func (sp *Sprite) Position() (x, y int) { return int(sp.X), int(sp.Y) }

// Size returns the sprite's width and height, as cut from the texture.
func (sp *Sprite) Size() (w, h int) { return int(sp.W), int(sp.H) }

// A Box is a filled rectangle of one color.
type Box struct {
	Mode  Mode
	X, Y  int16
	W, H  uint16
	Color Color
	_     uint8
}

// Draw puts the box on the frame at depth.
func (b *Box) Draw(f *Frame, depth int) { sortBox(b, f, depth) }

// MoveTo puts the box's top left corner at x, y.
func (b *Box) MoveTo(x, y int) { b.X, b.Y = int16(x), int16(y) }

// Position returns where the box's top left corner is.
func (b *Box) Position() (x, y int) { return int(b.X), int(b.Y) }

// Size returns the box's width and height.
func (b *Box) Size() (w, h int) { return int(b.W), int(b.H) }

// Rect is the box's position and size as a Rect.
func (b *Box) Rect() Rect {
	return Rect{
		X: int(b.X),
		Y: int(b.Y),
		W: int(b.W),
		H: int(b.H),
	}
}

// SetRect places and sizes the box to cover r.
func (b *Box) SetRect(r Rect) {
	b.X, b.Y, b.W, b.H = int16(r.X), int16(r.Y), uint16(r.W), uint16(r.H)
}

// A Line is a straight line of one color from one point to another.
type Line struct {
	Mode   Mode
	X0, Y0 int16
	X1, Y1 int16
	Color  Color
	_      uint8
}

// Draw puts the line on the frame at depth.
func (l *Line) Draw(f *Frame, depth int) { sortLine(l, f, depth) }

// SetEnds puts the line's ends at x0, y0 and x1, y1.
func (l *Line) SetEnds(x0, y0, x1, y1 int) {
	l.X0, l.Y0, l.X1, l.Y1 = int16(x0), int16(y0), int16(x1), int16(y1)
}

// A GradientLine is a line shaded from one color at its first point to
// another at its second.
type GradientLine struct {
	Mode   Mode
	X0, Y0 int16
	X1, Y1 int16
	From   Color
	To     Color
	_      [2]uint8
}

// Draw puts the line on the frame at depth.
func (l *GradientLine) Draw(f *Frame, depth int) { sortGradientLine(l, f, depth) }

// SetEnds puts the line's ends at x0, y0 and x1, y1.
func (l *GradientLine) SetEnds(x0, y0, x1, y1 int) {
	l.X0, l.Y0, l.X1, l.Y1 = int16(x0), int16(y0), int16(x1), int16(y1)
}
