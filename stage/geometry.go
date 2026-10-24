package stage

import "go.chrisrx.dev/psx/yaroze"

// The ground is the X-Z plane, and most of what moves on it is a point
// or a step on it; the units are whatever the use says, world units
// mostly, sixteenths of one for the hero's position, and 4096ths for
// the camera's directions. Up is -Y, as the console has it.

// A Vec is a point or a step on the ground.
type Vec struct{ X, Z int }

func (v Vec) Add(o Vec) Vec { return Vec{v.X + o.X, v.Z + o.Z} }
func (v Vec) Sub(o Vec) Vec { return Vec{v.X - o.X, v.Z - o.Z} }

// dot is the dot product, for how far along a direction in 4096ths v
// reaches: v.Dot(dir) >> 12.
func (v Vec) Dot(o Vec) int { return v.X*o.X + v.Z*o.Z }

// Units turns a position in sixteenths of a unit into units.
func (v Vec) Units() Vec { return Vec{v.X >> 4, v.Z >> 4} }

// Sixteenths turns units into sixteenths of a unit, which a game may keep
// its positions in.
func (v Vec) Sixteenths() Vec { return Vec{v.X << 4, v.Z << 4} }

// Within reports whether v is less than dx across and dz along from o.
func (v Vec) Within(o Vec, dx, dz int) bool {
	return abs(v.X-o.X) < dx && abs(v.Z-o.Z) < dz
}

// A Color is red, green and blue, 128 each leaving a texture as it is.
type Color [3]uint8

func Gray(v uint8) Color { return Color{v, v, v} }

// shade scales the Color by a brightness, 128 leaving it as it is.
func (c Color) Shade(bright int) Color {
	return Color{scale(c[0], bright), scale(c[1], bright), scale(c[2], bright)}
}

//go:inline
func scale(c uint8, bright int) uint8 { return uint8(min(int(c)*bright/128, 255)) }

// word packs the Color as a GPU packet holds one, under a command code.
//
//go:inline
func (c Color) Word(code uint32) uint32 {
	return code<<24 | uint32(c[2])<<16 | uint32(c[1])<<8 | uint32(c[0])
}

//go:inline
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A skin is the texel words of a quad's four corners, as a packet takes
// them: the first with the palette, the second with the texture page.
type skin [4]uint32

// A Surface is a texture loaded into video memory, with the skin of a
// quad that shows the whole of it worked out once, since most quads do.
type Surface struct {
	yaroze.Texture
	full skin
}

// NewSurface makes a surface of a texture loaded into video memory.
func NewSurface(tex yaroze.Texture) Surface {
	return Surface{Texture: tex, full: skinOf(&tex, &fullUV)}
}

// skinOf is the texel words at the corners uv gives within the texture.
//
//go:inline
func skinOf(t *yaroze.Texture, uv *[4][2]uint8) skin {
	return skin{t.WithClut(uv[0][0], uv[0][1]), t.WithPage(uv[1][0], uv[1][1]), t.Texel(uv[2][0], uv[2][1]), t.Texel(uv[3][0], uv[3][1])}
}

// fullUV is the texture corners of a quad that shows the whole of a 64
// by 64 texture.
var fullUV = [4][2]uint8{{0, 0}, {63, 0}, {0, 63}, {63, 63}}

// A polygon is the corners of the quad or triangle being drawn, as the
// GTE takes them and as it gives them back on the screen: the drawing
// code sets the corners, and quad or tri transforms them, drops the
// polygon when it faces away, is too near or is off the screen, and
// builds its packet. One polygon is kept for the whole program, since
// handing its address to the GTE must not allocate.
type polygon struct {
	v    [4]yaroze.SVECTOR
	sxy  [3]uint32
	sxy3 uint32
}

var poly polygon

// corner sets corner n: the quad's are top left, top right, bottom left
// and bottom right as seen from its front, the triangle's the first
// three, clockwise from its front.
//
//go:inline
func (p *polygon) corner(n, x, y, z int) {
	p.v[n] = yaroze.SVECTOR{Vx: int16(x), Vy: int16(y), Vz: int16(z)}
}

// The screen's size, which the quads are culled against: the usual 320
// by 240.
const screenW, screenH = 320, 240

// The GPU's codes for a textured quad and triangle in one shade, and the
// bit that makes either half see-through.
const (
	quadCode = 0x2c
	triCode  = 0x24
	ghostBit = 0x02
)

// quad draws the quad in the corners, when its front is towards the
// camera, textured with the whole of sf and colored, as a textured quad
// packet sorted at the depth the GTE averages for it, back entries
// further back. A ghost quad is half see-through, averaged with what is
// behind it, which the texture's colors allow when png2tim made it with
// --semi. It reports whether the quad was drawn.
//
//go:inline
func (p *polygon) quad(ot *yaroze.GsOT, sf *Surface, c Color, ghost bool, back int) bool {
	return p.place(ot, &p.v, &sf.full, c.Word(quadCode|ghosted(ghost)), back, true)
}

// ground draws the quad in the corners as a piece of the ground: the
// whole of sf, colored, back entries further back, and whichever way its
// corners turn on the screen. The ground is only ever seen from above,
// and a cell on a slope, whose corners are not in one plane, seen edge
// on can turn its first three corners the wrong way while most of it
// faces the camera, which would leave a hole.
//
//go:inline
func (p *polygon) ground(ot *yaroze.GsOT, sf *Surface, c Color, ghost bool, back int) bool {
	return p.place(ot, &p.v, &sf.full, c.Word(quadCode|ghosted(ghost)), back, false)
}

// quadUV is quad with the texture at the corners uv gives within tex.
//
//go:inline
func (p *polygon) quadUV(ot *yaroze.GsOT, tex *yaroze.Texture, uv *[4][2]uint8, c Color, ghost bool, back int) bool {
	sk := skinOf(tex, uv)
	return p.place(ot, &p.v, &sk, c.Word(quadCode|ghosted(ghost)), back, true)
}

//go:inline
func ghosted(ghost bool) uint32 {
	if ghost {
		return ghostBit
	}
	return 0
}

// place draws the quad whose corners are v, wherever they are kept,
// skinned with sk and colored by the packet word rgb, code included,
// back entries further back, when it faces the camera if cull, and
// reports whether it was drawn. The quad is left out when a corner is
// behind the camera or beyond the GTE's range, when it is nearer than
// nearest, and when the whole of it lies off one side of the screen,
// where the GPU would clip it away after the packet was built.
func (p *polygon) place(ot *yaroze.GsOT, v *[4]yaroze.SVECTOR, sk *skin, rgb uint32, back int, cull bool) bool {
	if yaroze.RotTransPers3(&v[0], &v[1], &v[2], &p.sxy) < 0 {
		return false
	}
	if cull {
		area := yaroze.NormalClip()
		if area == 0 {
			// The first three corners have no area on the screen, as at
			// the end of a cliff face that tapers to nothing, so the
			// other three decide, so that the face is not dropped whole.
			// The second triangle of a quad in the GPU's order, top left,
			// top right, bottom left, bottom right, winds the other way
			// round from the first.
			if yaroze.RotTransPers3(&v[1], &v[2], &v[3], &p.sxy) < 0 {
				return false
			}
			area = -yaroze.NormalClip()
			// The corners again in their order, for the packet and the
			// depth.
			yaroze.RotTransPers3(&v[0], &v[1], &v[2], &p.sxy)
		}
		if area <= 0 {
			return false
		}
	}
	if yaroze.RotTransPers(&v[3], &p.sxy3) < 0 {
		return false
	}
	otz := int(yaroze.AverageZ4())
	if otz < nearest || p.offScreen() {
		return false
	}
	if cur.block != 0 {
		cur.covers(p, 4, otz)
	}
	k := yaroze.AllocPacket(10)
	k[0] = 9 << 24
	k[1] = rgb
	k[2] = p.sxy[0]
	k[3] = sk[0]
	k[4] = p.sxy[1]
	k[5] = sk[1]
	k[6] = p.sxy[2]
	k[7] = sk[2]
	k[8] = p.sxy3
	k[9] = sk[3]
	ot.Link(min(otz>>Shift+back, 1<<Depth-1), k)
	return true
}

// offScreen reports whether the quad's four corners, as the GTE gave them
// back, all lie beyond one edge of the screen, whose origin is its
// middle.
//
//go:inline
func (p *polygon) offScreen() bool {
	const hw, hh = screenW / 2, screenH / 2
	x0, x1, x2, x3 := int16(p.sxy[0]), int16(p.sxy[1]), int16(p.sxy[2]), int16(p.sxy3)
	if x0 < -hw && x1 < -hw && x2 < -hw && x3 < -hw || x0 > hw && x1 > hw && x2 > hw && x3 > hw {
		return true
	}
	y0, y1, y2, y3 := int16(p.sxy[0]>>16), int16(p.sxy[1]>>16), int16(p.sxy[2]>>16), int16(p.sxy3>>16)
	return y0 < -hh && y1 < -hh && y2 < -hh && y3 < -hh || y0 > hh && y1 > hh && y2 > hh && y3 > hh
}

// tri draws the triangle in the first three corners, when they turn
// clockwise on the screen, textured from tex at the corners uv gives
// within it, in one shade, as quadUV draws a quad.
func (p *polygon) tri(ot *yaroze.GsOT, tex *yaroze.Texture, uv *[3][2]uint8, c Color, ghost bool) bool {
	if yaroze.RotTransPers3(&p.v[0], &p.v[1], &p.v[2], &p.sxy) < 0 || yaroze.NormalClip() <= 0 {
		return false
	}
	otz := int(yaroze.AverageZ3())
	if otz < nearest {
		return false
	}
	if cur.block != 0 {
		cur.covers(p, 3, otz)
	}
	k := yaroze.AllocPacket(8)
	k[0] = 7 << 24
	k[1] = c.Word(triCode | ghosted(ghost))
	k[2] = p.sxy[0]
	k[3] = tex.WithClut(uv[0][0], uv[0][1])
	k[4] = p.sxy[1]
	k[5] = tex.WithPage(uv[1][0], uv[1][1])
	k[6] = p.sxy[2]
	k[7] = tex.Texel(uv[2][0], uv[2][1])
	ot.Link(min(otz>>Shift, 1<<Depth-1), k)
	return true
}

// bounds is the box on the screen round the first n corners as they
// were last transformed.
func (p *polygon) bounds(n int) (minX, minY, maxX, maxY int) {
	minX, maxX = int(int16(p.sxy[0])), int(int16(p.sxy[0]))
	minY, maxY = int(int16(p.sxy[0]>>16)), int(int16(p.sxy[0]>>16))
	for k := 1; k < n; k++ {
		s := p.sxy3
		if k < 3 {
			s = p.sxy[k]
		}
		x, y := int(int16(s)), int(int16(s>>16))
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return
}
