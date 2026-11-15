package yaroze

import "unsafe"

// GPU packets a program builds itself, in the frame's packet space
// beside the ones the library's drawing builds, and links into the frame
// at a depth as they are. A packet is a Tag the GPU reads the packet's
// length and the next packet from, then the command and its arguments
// as the GPU takes them; the types below are the SDK's libgpu.h layouts
// under Go names. Frame.Packet takes room for one, sets its tag and its
// command code, links it in, and returns it to fill in:
//
//	sky := frame.Packet[yaroze.PolyG4](frame.Depths() - 1)
//	sky.Color0, sky.XY0 = top, yaroze.XY{X: -160, Y: -120}
//	sky.Color1, sky.XY1 = top, yaroze.XY{X: 160, Y: -120}
//	sky.Color2, sky.XY2 = bottom, yaroze.XY{X: -160, Y: 120}
//	sky.Color3, sky.XY3 = bottom, yaroze.XY{X: 160, Y: 120}

// A Tag heads every packet: the length of the packet in words after it,
// and the address of the next packet, which the frame's sorting fills.
type Tag uint32

// An XY is a screen point as a packet takes it.
type XY struct {
	X, Y int16
}

// At returns the position x, y as a packet takes it.
func At(x, y int) XY { return XY{X: int16(x), Y: int16(y)} }

// A UV is a texture coordinate as a packet takes it, within the texture
// page.
type UV struct {
	U, V uint8
}

// A Code is a packet's command byte, which Frame.Packet sets for the
// packet's type, with the bits a program may add.
type Code uint8

const (
	// CodeTranslucent blends the primitive with what is behind it, by
	// the blend the drawing mode holds.
	CodeTranslucent Code = 1 << 1

	// CodeRaw draws a texture as it is, without the primitive's color.
	CodeRaw Code = 1 << 0
)

// The packet types. A quad's corners go 0 1 over 2 3. A textured
// primitive's first corner carries the palette and its second the
// texture page, as a Texture gives them.
type (
	// PolyF3 is a triangle of one color.
	PolyF3 struct {
		Tag   Tag
		Color Color
		Code  Code
		XY0   XY
		XY1   XY
		XY2   XY
	}

	// PolyF4 is a quad of one color.
	PolyF4 struct {
		Tag   Tag
		Color Color
		Code  Code
		XY0   XY
		XY1   XY
		XY2   XY
		XY3   XY
	}

	// PolyFT3 is a textured triangle of one color.
	PolyFT3 struct {
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

	// PolyFT4 is a textured quad of one color.
	PolyFT4 struct {
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

	// PolyG3 is a triangle shaded between a color at each corner.
	PolyG3 struct {
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

	// PolyG4 is a quad shaded between a color at each corner.
	PolyG4 struct {
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

	// PolyGT3 is a textured triangle shaded between a color at each
	// corner.
	PolyGT3 struct {
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

	// PolyGT4 is a textured quad shaded between a color at each corner.
	PolyGT4 struct {
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

	// LineF2 is a line of one color.
	LineF2 struct {
		Tag   Tag
		Color Color
		Code  Code
		XY0   XY
		XY1   XY
	}

	// LineG2 is a line shaded between a color at each end.
	LineG2 struct {
		Tag    Tag
		Color0 Color
		Code   Code
		XY0    XY
		Color1 Color
		_      uint8
		XY1    XY
	}

	// Tile is a filled rectangle of one color.
	Tile struct {
		Tag   Tag
		Color Color
		Code  Code
		XY    XY
		W, H  uint16
	}

	// Sprt is a rectangle of a texture, drawn upright at its own size,
	// from the texture page the drawing mode holds.
	Sprt struct {
		Tag   Tag
		Color Color
		Code  Code
		XY    XY
		UV    UV
		Clut  uint16
		W, H  uint16
	}

	// DrawMode sets the texture page and the blend for the packets after
	// it: the page as a Texture gives it, with the blend in bits 5 and 6.
	DrawMode struct {
		Tag  Tag
		Page uint32
	}
)

// Packet takes room for a T, one of the packet types, from the frame's
// packet space, sets its tag and its command code, links it into the
// frame at depth, and returns it to fill in. Nothing checks that the
// space has room: a frame that builds more than Screen.Packets holds
// writes past it. A type that is not a packet type gets its tag alone.
func (f *Frame) Packet[T any](depth int) *T {
	p := f.packet[T]()
	linkPacket(f, depth, (*Tag)(unsafe.Pointer(p)))
	return p
}

// PacketAfter is Packet with the packet linked after the packet prev,
// of any type, rather than into the frame: it is drawn right after prev
// wherever prev was linked. A primitive goes after the drawing mode it
// needs this way; at one depth, Packet puts each packet in front of the
// last, so the one linked last is drawn first.
func (f *Frame) PacketAfter[T any](prev *Tag) *T {
	p := f.packet[T]()
	linkAfter(prev, (*Tag)(unsafe.Pointer(p)))
	return p
}

// packet takes the room, and sets the tag's length and the code.
func (f *Frame) packet[T any]() *T {
	var zero T
	words := int(unsafe.Sizeof(zero) / 4)
	p := (*T)(unsafe.Pointer(allocPacket(words)))
	*(*Tag)(unsafe.Pointer(p)) = Tag(words-1) << 24
	setCode(any(p))
	return p
}

// setCode sets a packet's command code for its type.
func setCode(p any) {
	switch p := p.(type) {
	case *PolyF3:
		p.Code = 0x20
	case *PolyF4:
		p.Code = 0x28
	case *PolyFT3:
		p.Code = 0x24
	case *PolyFT4:
		p.Code = 0x2c
	case *PolyG3:
		p.Code = 0x30
	case *PolyG4:
		p.Code = 0x38
	case *PolyGT3:
		p.Code = 0x34
	case *PolyGT4:
		p.Code = 0x3c
	case *LineF2:
		p.Code = 0x40
	case *LineG2:
		p.Code = 0x50
	case *Tile:
		p.Code = 0x60
	case *Sprt:
		p.Code = 0x64
	case *DrawMode:
		p.Page = 0xe1 << 24
	}
}

// Mode returns the drawing mode's page word for a texture with the
// blend abr, 0 to 3: average, add, subtract or a quarter.
func (t Texture) Mode(abr int) uint32 {
	return 0xe1<<24 | uint32(t.Page)&^0x60 | uint32(abr&3)<<5
}
