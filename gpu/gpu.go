// Package gpu is the GPU's packets as it takes them: the triangles,
// quads, lines, rectangles and sprites it draws, and the drawing mode it
// draws them in, each laid out as the SDK's libgpu.h has it under a name
// that says what it draws.
//
// A packet is a Tag, which the GPU reads the packet's length and the next
// packet from, then the command and its arguments. A program builds one
// in the frame's packet space, beside the packets the library's drawing
// builds, and links it into the frame at a depth. The yaroze package's
// Frame.Packet does all of that: it takes room for the packet, sets its
// tag, has the packet set its command code, links it in, and returns it
// to fill in:
//
//	sky := frame.Packet[gpu.ShadedQuad](frame.Depths() - 1)
//	sky.Color0, sky.XY0 = top, gpu.At(-160, -120)
//	sky.Color1, sky.XY1 = top, gpu.At(160, -120)
//	sky.Color2, sky.XY2 = bottom, gpu.At(-160, 120)
//	sky.Color3, sky.XY3 = bottom, gpu.At(160, 120)
//
// The names say how a primitive is colored. A Flat one, or one with
// neither word, has one color. A Shaded one has a color for each corner
// and blends between them across its face, which the SDK calls Gouraud
// shading and marks with a G. A Textured one takes its pixels from a
// texture, tinted by its color or colors.
//
// A quad's corners go 0 1 over 2 3. A textured primitive's first corner
// carries the palette and its second the texture page, as a yaroze
// Texture gives them.
package gpu

// A Tag heads every packet: the length of the packet in words after it,
// and the address of the next packet, which linking the packet into a
// frame fills.
type Tag uint32

// An XY is a screen point as a packet takes it.
type XY struct {
	X, Y int16
}

// At returns the screen point x, y as a packet takes it.
func At(x, y int) XY { return XY{X: int16(x), Y: int16(y)} }

// A UV is a texture coordinate as a packet takes it, within the texture
// page.
type UV struct {
	U, V uint8
}

// A Color is a color as the GPU takes it, a byte a component.
type Color struct {
	R, G, B uint8
}

// A Code is a packet's command byte, which the packet's Init sets for
// its type, with the bits a program may add.
type Code uint8

const (
	// CodeTranslucent blends the primitive with what is behind it, by
	// the blend the drawing mode holds.
	CodeTranslucent Code = 1 << 1

	// CodeRaw draws a texture as it is, without the primitive's color.
	CodeRaw Code = 1 << 0
)

// A Packet is a pointer to a packet type T that sets its own command
// code, which is what Frame.Packet asks of the type it is given. Every
// packet type here is one. So is a program's own type for a command this
// package has no type for, once it starts with a Tag, is whole words
// long, and has an Init that does the same.
type Packet[T any] interface {
	*T

	// Init sets the packet's command code, and any other word the
	// command fixes, and leaves the rest of the packet as it was. The
	// tag is not its to set: Frame.Packet works the length out from the
	// type's size.
	Init()
}

// FlatTriangle is a triangle of one color, the SDK's POLY_F3.
type FlatTriangle struct {
	Tag   Tag
	Color Color
	Code  Code
	XY0   XY
	XY1   XY
	XY2   XY
}

// FlatQuad is a quad of one color, the SDK's POLY_F4.
type FlatQuad struct {
	Tag   Tag
	Color Color
	Code  Code
	XY0   XY
	XY1   XY
	XY2   XY
	XY3   XY
}

// TexturedTriangle is a textured triangle tinted one color, the SDK's
// POLY_FT3.
type TexturedTriangle struct {
	Tag   Tag
	Color Color
	Code  Code
	XY0   XY
	UV0   UV
	Clut  uint16
	XY1   XY
	UV1   UV
	Page  uint16
	XY2   XY
	UV2   UV
	_     uint16
}

// TexturedQuad is a textured quad tinted one color, the SDK's POLY_FT4.
type TexturedQuad struct {
	Tag   Tag
	Color Color
	Code  Code
	XY0   XY
	UV0   UV
	Clut  uint16
	XY1   XY
	UV1   UV
	Page  uint16
	XY2   XY
	UV2   UV
	_     uint16
	XY3   XY
	UV3   UV
	_     uint16
}

// ShadedTriangle is a triangle shaded between a color at each corner,
// the SDK's POLY_G3.
type ShadedTriangle struct {
	Tag    Tag
	Color0 Color
	Code   Code
	XY0    XY
	Color1 Color
	_      uint8
	XY1    XY
	Color2 Color
	_      uint8
	XY2    XY
}

// ShadedQuad is a quad shaded between a color at each corner, the SDK's
// POLY_G4.
type ShadedQuad struct {
	Tag    Tag
	Color0 Color
	Code   Code
	XY0    XY
	Color1 Color
	_      uint8
	XY1    XY
	Color2 Color
	_      uint8
	XY2    XY
	Color3 Color
	_      uint8
	XY3    XY
}

// ShadedTexturedTriangle is a textured triangle shaded between a color
// at each corner, the SDK's POLY_GT3.
type ShadedTexturedTriangle struct {
	Tag    Tag
	Color0 Color
	Code   Code
	XY0    XY
	UV0    UV
	Clut   uint16
	Color1 Color
	_      uint8
	XY1    XY
	UV1    UV
	Page   uint16
	Color2 Color
	_      uint8
	XY2    XY
	UV2    UV
	_      uint16
}

// ShadedTexturedQuad is a textured quad shaded between a color at each
// corner, the SDK's POLY_GT4.
type ShadedTexturedQuad struct {
	Tag    Tag
	Color0 Color
	Code   Code
	XY0    XY
	UV0    UV
	Clut   uint16
	Color1 Color
	_      uint8
	XY1    XY
	UV1    UV
	Page   uint16
	Color2 Color
	_      uint8
	XY2    XY
	UV2    UV
	_      uint16
	Color3 Color
	_      uint8
	XY3    XY
	UV3    UV
	_      uint16
}

// Line is a line of one color, the SDK's LINE_F2.
type Line struct {
	Tag   Tag
	Color Color
	Code  Code
	XY0   XY
	XY1   XY
}

// ShadedLine is a line shaded between a color at each end, the SDK's
// LINE_G2.
type ShadedLine struct {
	Tag    Tag
	Color0 Color
	Code   Code
	XY0    XY
	Color1 Color
	_      uint8
	XY1    XY
}

// Tile is a filled rectangle of one color, the SDK's TILE.
type Tile struct {
	Tag   Tag
	Color Color
	Code  Code
	XY    XY
	W, H  uint16
}

// Sprite is a rectangle of a texture, drawn upright at its own size from
// the texture page the drawing mode holds, the SDK's SPRT.
//
// It is the GPU's sprite, the one command. The yaroze package's Sprite is
// the library's, the SDK's GsSPRITE: it carries its own texture page and
// blend, can be scaled and turned, and is drawn with its Draw, which
// builds the packets it takes, this one among them.
type Sprite struct {
	Tag   Tag
	Color Color
	Code  Code
	XY    XY
	UV    UV
	Clut  uint16
	W, H  uint16
}

// DrawMode sets the texture page and the blend for the packets after it,
// the SDK's DR_MODE: Page is the command and under it the page as a
// yaroze Texture gives it, with the blend in bits 5 and 6, which
// Texture.Mode returns whole.
type DrawMode struct {
	Tag  Tag
	Page uint32
}

// Init sets the packet's command code. Frame.Packet calls it; a program
// calls it for a packet it keeps in memory of its own.
func (p *FlatTriangle) Init() { p.Code = 0x20 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *FlatQuad) Init() { p.Code = 0x28 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *TexturedTriangle) Init() { p.Code = 0x24 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *TexturedQuad) Init() { p.Code = 0x2c }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *ShadedTriangle) Init() { p.Code = 0x30 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *ShadedQuad) Init() { p.Code = 0x38 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *ShadedTexturedTriangle) Init() { p.Code = 0x34 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *ShadedTexturedQuad) Init() { p.Code = 0x3c }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *Line) Init() { p.Code = 0x40 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *ShadedLine) Init() { p.Code = 0x50 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *Tile) Init() { p.Code = 0x60 }

// Init sets the packet's command code, as FlatTriangle's does.
func (p *Sprite) Init() { p.Code = 0x64 }

// Init sets the packet's command, with page 0 and the average blend
// until Page is set.
func (p *DrawMode) Init() { p.Page = 0xe1 << 24 }
