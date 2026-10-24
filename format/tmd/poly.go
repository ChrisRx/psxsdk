package tmd

import (
	"errors"
	"fmt"
)

// Mode bits of a polygon.
const (
	modePolygon = 0x20
	modeGouraud = 0x10 // IIP: shaded or colored per vertex
	modeQuad    = 0x08
	modeTexture = 0x04 // TME
	modeSemi    = 0x02 // ABE
	modeUnlit   = 0x01 // TGE: colors, or the texture, as they are
)

// Flag bits of a primitive.
const (
	FlagUnlit       = 0x01 // LGT: not lit by the library
	FlagDoubleSided = 0x02 // FCE: drawn from both sides
	FlagGradation   = 0x04 // GRD: a lit polygon with a color per vertex
)

// RGB is a color, 128 being the texture's own brightness.
type RGB struct{ R, G, B uint8 }

// A Texture is how a polygon is textured: the texture page and CLUT, as a
// TIM gives them, and each vertex's U and V within the page.
type Texture struct {
	TPage uint16 // with the blending rate in bits 5-6 for a semitransparent polygon
	CBA   uint16
	UV    [4][2]uint8
}

// A Polygon is a triangle or a quad, which Primitive lays out as the TMD
// packet that draws it. A quad's vertices are in strip order: the first
// three are a triangle, and the last three another, so a square is top
// left, top right, bottom left, bottom right. A side faces the camera when
// its first three vertices go clockwise on the screen.
type Polygon struct {
	Vertices []uint16

	// Normals light the polygon: one for flat shading, one per vertex
	// for gouraud. With none it is not lit, and drawn in its colors, or
	// its texture as it is.
	Normals []uint16

	// Colors are one color, or one per vertex for a gradation. A lit
	// textured polygon has none; an unlit one needs them.
	Colors []RGB

	Texture     *Texture
	Semi        bool
	DoubleSided bool
}

func pair(lo, hi uint16) uint32 { return uint32(lo) | uint32(hi)<<16 }

func rgbWord(c RGB, top byte) uint32 {
	return uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16 | uint32(top)<<24
}

// Primitive returns the polygon's packet.
func (p Polygon) Primitive() (Primitive, error) {
	n := len(p.Vertices)
	if n != 3 && n != 4 {
		return Primitive{}, fmt.Errorf("tmd: a polygon of %d vertices", n)
	}
	lit := len(p.Normals) > 0
	if lit && len(p.Normals) != 1 && len(p.Normals) != n {
		return Primitive{}, fmt.Errorf("tmd: %d normals for %d vertices", len(p.Normals), n)
	}
	if len(p.Colors) != 0 && len(p.Colors) != 1 && len(p.Colors) != n {
		return Primitive{}, fmt.Errorf("tmd: %d colors for %d vertices", len(p.Colors), n)
	}
	textured := p.Texture != nil
	if lit && textured && len(p.Colors) > 0 {
		return Primitive{}, errors.New("tmd: a lit textured polygon takes no colors")
	}
	if (!textured || !lit) && len(p.Colors) == 0 {
		return Primitive{}, errors.New("tmd: an untextured or unlit polygon needs a color")
	}
	gradation := len(p.Colors) > 1

	mode := byte(modePolygon)
	var flag byte
	if n == 4 {
		mode |= modeQuad
	}
	if textured {
		mode |= modeTexture
	}
	if p.Semi {
		mode |= modeSemi
	}
	if p.DoubleSided {
		flag |= FlagDoubleSided
	}
	gouraud := lit && len(p.Normals) == n || !lit && gradation
	if gouraud {
		mode |= modeGouraud
	}
	if !lit {
		mode |= modeUnlit
		flag |= FlagUnlit
	} else if gradation {
		flag |= FlagGradation
	}

	var w []uint32
	if textured {
		t := p.Texture
		uv := func(i int) uint16 { return uint16(t.UV[i][0]) | uint16(t.UV[i][1])<<8 }
		w = append(w, pair(uv(0), t.CBA), pair(uv(1), t.TPage), pair(uv(2), 0))
		if n == 4 {
			w = append(w, pair(uv(3), 0))
		}
	}
	colors := func(top byte) {
		for i, c := range p.Colors {
			if i == 0 {
				w = append(w, rgbWord(c, top))
			} else {
				w = append(w, rgbWord(c, 0))
			}
		}
	}
	v := p.Vertices
	if lit {
		if !textured {
			colors(mode)
		}
		if len(p.Normals) == 1 {
			w = append(w, pair(p.Normals[0], v[0]), pair(v[1], v[2]))
			if n == 4 {
				w = append(w, pair(v[3], 0))
			}
		} else {
			for i := range n {
				w = append(w, pair(p.Normals[i], v[i]))
			}
		}
	} else {
		if textured {
			colors(0)
		} else {
			colors(mode)
		}
		w = append(w, pair(v[0], v[1]))
		if n == 4 {
			w = append(w, pair(v[2], v[3]))
		} else {
			w = append(w, pair(v[2], 0))
		}
	}

	// The GPU primitive the library makes: the size of POLY_F3 and the
	// rest, less its tag.
	olen := map[[3]bool]byte{
		{false, false, false}: 4, {false, true, false}: 6, {false, false, true}: 7, {false, true, true}: 9,
		{true, false, false}: 5, {true, true, false}: 8, {true, false, true}: 9, {true, true, true}: 12,
	}[[3]bool{n == 4, gouraud || gradation, textured}]
	return Primitive{Mode: mode, Flag: flag, Olen: olen, Packet: w}, nil
}

// A Line is a line between two vertices, in one color or shaded between
// two. Sony's FAQ warns that the Net Yaroze library does not draw lines in
// a TMD, so a thin polygon is the safer choice.
type Line struct {
	Vertices [2]uint16
	Colors   []RGB
	Semi     bool
}

// Primitive returns the line's packet.
func (l Line) Primitive() (Primitive, error) {
	if len(l.Colors) != 1 && len(l.Colors) != 2 {
		return Primitive{}, fmt.Errorf("tmd: a line of %d colors", len(l.Colors))
	}
	mode := byte(0x40)
	if len(l.Colors) == 2 {
		mode |= modeGouraud
	}
	if l.Semi {
		mode |= modeSemi
	}
	w := []uint32{rgbWord(l.Colors[0], mode)}
	if len(l.Colors) == 2 {
		w = append(w, rgbWord(l.Colors[1], 0))
	}
	w = append(w, pair(l.Vertices[0], l.Vertices[1]))
	return Primitive{Mode: mode, Flag: FlagUnlit, Olen: byte(len(w) + 1), Packet: w}, nil
}

// A Sprite is a rectangle of texture at a vertex, always facing the camera.
// Sony's FAQ warns that the Net Yaroze library does not draw sprites in a
// TMD.
type Sprite struct {
	Vertex     uint16
	TPage, CBA uint16
	U, V       uint8
	W, H       uint16
	Semi       bool
}

// Primitive returns the sprite's packet: free size, or 1x1, 8x8 or 16x16
// when it is one of those.
func (s Sprite) Primitive() (Primitive, error) {
	mode := byte(0x64)
	switch {
	case s.W == 1 && s.H == 1:
		mode = 0x6c
	case s.W == 8 && s.H == 8:
		mode = 0x74
	case s.W == 16 && s.H == 16:
		mode = 0x7c
	}
	if s.Semi {
		mode |= modeSemi
	}
	w := []uint32{pair(s.Vertex, s.TPage), pair(uint16(s.U)|uint16(s.V)<<8, s.CBA)}
	olen := byte(4)
	if mode&^modeSemi == 0x64 {
		w = append(w, pair(s.W, s.H))
		olen = 5
	}
	return Primitive{Mode: mode, Flag: FlagUnlit, Olen: olen, Packet: w}, nil
}
