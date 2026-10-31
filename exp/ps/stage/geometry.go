package stage

import "go.chrisrx.dev/psx/exp/ps"

// The ground is the X-Z plane, and most of what moves on it is a point
// or a step on it; the units are whatever the use says, world units
// mostly, sixteenths of one for the hero's position, and 4096ths for
// the camera's directions. Up is -Y, as the console has it.

// A Vec is a point or a step on the ground.
type Vec struct{ X, Z int }

func (v Vec) Add(o Vec) Vec { return Vec{v.X + o.X, v.Z + o.Z} }
func (v Vec) Sub(o Vec) Vec { return Vec{v.X - o.X, v.Z - o.Z} }

// Dot is the dot product, for how far along a direction in 4096ths v
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

// Shade scales the Color by a brightness, 128 leaving it as it is.
func (c Color) Shade(bright int) Color {
	return Color{scale(c[0], bright), scale(c[1], bright), scale(c[2], bright)}
}

//go:inline
func scale(c uint8, bright int) uint8 { return uint8(min(int(c)*bright/128, 255)) }

// RGB is the Color as a packet takes it.
//
//go:inline
func (c Color) RGB() ps.Color { return ps.Color{R: c[0], G: c[1], B: c[2]} }

//go:inline
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A skin is the texture corners of a quad, as a packet takes them.
type skin [4]ps.UV

// A Surface is a texture loaded into video memory, with the skin of a
// quad that shows the whole of it worked out once, since most quads do.
type Surface struct {
	ps.Texture
	full skin
}

// NewSurface makes a surface of a texture loaded into video memory.
func NewSurface(tex ps.Texture) Surface {
	return Surface{Texture: tex, full: skinOf(&tex, &fullUV)}
}

// skinOf is the texture corners uv gives within the texture.
//
//go:inline
func skinOf(t *ps.Texture, uv *[4][2]uint8) skin {
	return skin{
		{U: t.U + uv[0][0], V: t.V + uv[0][1]}, {U: t.U + uv[1][0], V: t.V + uv[1][1]},
		{U: t.U + uv[2][0], V: t.V + uv[2][1]}, {U: t.U + uv[3][0], V: t.V + uv[3][1]},
	}
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
	v    [4]ps.Vertex
	sxy  [3]ps.XY
	sxy3 ps.XY
}

var poly polygon

// corner sets corner n: the quad's are top left, top right, bottom left
// and bottom right as seen from its front, the triangle's the first
// three, clockwise from its front.
//
//go:inline
func (p *polygon) corner(n, x, y, z int) {
	p.v[n] = ps.Vertex{X: int16(x), Y: int16(y), Z: int16(z)}
}

// The screen's size, which the quads are culled against: the usual 320
// by 240.
const screenW, screenH = 320, 240

// quad draws the quad in the corners, when its front is towards the
// camera, textured with the whole of sf and colored, as a textured quad
// packet sorted at the depth the GTE averages for it, back entries
// further back. A ghost quad is half see-through, averaged with what is
// behind it, which the texture's colors allow when png2tim made it with
// --semi. It reports whether the quad was drawn.
//
//go:inline
func (p *polygon) quad(f *ps.Frame, sf *Surface, c Color, ghost bool, back int) bool {
	return p.place(f, &p.v, &sf.full, &sf.Texture, c.RGB(), ghost, back, true)
}

// ground draws the quad in the corners as a piece of the ground: the
// whole of sf, colored, back entries further back, and whichever way its
// corners turn on the screen. The ground is only ever seen from above,
// and a cell on a slope, whose corners are not in one plane, seen edge
// on can turn its first three corners the wrong way while most of it
// faces the camera, which would leave a hole.
//
//go:inline
func (p *polygon) ground(f *ps.Frame, sf *Surface, c Color, ghost bool, back int) bool {
	return p.place(f, &p.v, &sf.full, &sf.Texture, c.RGB(), ghost, back, false)
}

// quadUV is quad with the texture at the corners uv gives within tex.
//
//go:inline
func (p *polygon) quadUV(f *ps.Frame, tex *ps.Texture, uv *[4][2]uint8, c Color, ghost bool, back int) bool {
	sk := skinOf(tex, uv)
	return p.place(f, &p.v, &sk, tex, c.RGB(), ghost, back, true)
}

// place draws the quad whose corners are v, wherever they are kept,
// skinned with sk of tex and colored rgb, back entries further back,
// when it faces the camera if cull, and reports whether it was drawn.
// The quad is left out when a corner is behind the camera or beyond the
// GTE's range, when it is nearer than nearest, and when the whole of it
// lies off one side of the screen, where the GPU would clip it away
// after the packet was built.
func (p *polygon) place(f *ps.Frame, v *[4]ps.Vertex, sk *skin, tex *ps.Texture, rgb ps.Color, ghost bool, back int, cull bool) bool {
	if !ps.Project3(&v[0], &v[1], &v[2], &p.sxy) {
		return false
	}
	if cull {
		area := ps.ClipArea()
		if area == 0 {
			// The first three corners have no area on the screen, as at
			// the end of a cliff face that tapers to nothing, so the
			// other three decide, so that the face is not dropped whole.
			// The second triangle of a quad in the GPU's order, top left,
			// top right, bottom left, bottom right, winds the other way
			// round from the first.
			if !ps.Project3(&v[1], &v[2], &v[3], &p.sxy) {
				return false
			}
			area = -ps.ClipArea()
			// The corners again in their order, for the packet and the
			// depth.
			ps.Project3(&v[0], &v[1], &v[2], &p.sxy)
		}
		if area <= 0 {
			return false
		}
	}
	if !ps.Project(&v[3], &p.sxy3) {
		return false
	}
	otz := ps.AverageDepth4()
	if otz < nearest || p.offScreen() {
		return false
	}
	if cur.block != 0 {
		cur.covers(p, 4, otz)
	}
	k := f.Packet[ps.PolyFT4](min(otz>>Shift+back, f.Depths()-1))
	k.Color = rgb
	if ghost {
		k.Code |= ps.CodeTranslucent
	}
	k.XY0, k.XY1, k.XY2, k.XY3 = p.sxy[0], p.sxy[1], p.sxy[2], p.sxy3
	k.UV0, k.UV1, k.UV2, k.UV3 = sk[0], sk[1], sk[2], sk[3]
	k.Clut, k.Page = tex.Clut(), tex.Page
	return true
}

// offScreen reports whether the quad's four corners, as the GTE gave them
// back, all lie beyond one edge of the screen, whose origin is its
// middle.
//
//go:inline
func (p *polygon) offScreen() bool {
	const hw, hh = screenW / 2, screenH / 2
	x0, x1, x2, x3 := p.sxy[0].X, p.sxy[1].X, p.sxy[2].X, p.sxy3.X
	if x0 < -hw && x1 < -hw && x2 < -hw && x3 < -hw || x0 > hw && x1 > hw && x2 > hw && x3 > hw {
		return true
	}
	y0, y1, y2, y3 := p.sxy[0].Y, p.sxy[1].Y, p.sxy[2].Y, p.sxy3.Y
	return y0 < -hh && y1 < -hh && y2 < -hh && y3 < -hh || y0 > hh && y1 > hh && y2 > hh && y3 > hh
}

// tri draws the triangle in the first three corners, when they turn
// clockwise on the screen, textured from tex at the corners uv gives
// within it, in one shade, as quadUV draws a quad.
func (p *polygon) tri(f *ps.Frame, tex *ps.Texture, uv *[3][2]uint8, c Color, ghost bool) bool {
	if !ps.Project3(&p.v[0], &p.v[1], &p.v[2], &p.sxy) || ps.ClipArea() <= 0 {
		return false
	}
	otz := ps.AverageDepth3()
	if otz < nearest {
		return false
	}
	if cur.block != 0 {
		cur.covers(p, 3, otz)
	}
	k := f.Packet[ps.PolyFT3](min(otz>>Shift, f.Depths()-1))
	k.Color = c.RGB()
	if ghost {
		k.Code |= ps.CodeTranslucent
	}
	k.XY0, k.XY1, k.XY2 = p.sxy[0], p.sxy[1], p.sxy[2]
	k.UV0 = ps.UV{U: tex.U + uv[0][0], V: tex.V + uv[0][1]}
	k.UV1 = ps.UV{U: tex.U + uv[1][0], V: tex.V + uv[1][1]}
	k.UV2 = ps.UV{U: tex.U + uv[2][0], V: tex.V + uv[2][1]}
	k.Clut, k.Page = tex.Clut(), tex.Page
	return true
}

// bounds is the box on the screen round the first n corners as they
// were last transformed.
func (p *polygon) bounds(n int) (minX, minY, maxX, maxY int) {
	minX, maxX = int(p.sxy[0].X), int(p.sxy[0].X)
	minY, maxY = int(p.sxy[0].Y), int(p.sxy[0].Y)
	for k := 1; k < n; k++ {
		s := p.sxy3
		if k < 3 {
			s = p.sxy[k]
		}
		x, y := int(s.X), int(s.Y)
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return
}
