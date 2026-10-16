package emu

import "math"

// psxe's rasterizer, ported. It draws with floats, so the arithmetic is
// kept as psxe writes it, rounding to float32 after every operation, with
// explicit conversions so that Go cannot fuse a multiply and an add.

// vertex is a corner of a primitive, with psxe's field widths: coordinates
// are 16 bits, so adding the drawing offset wraps them as in psxe.
type vertex struct {
	x, y   int16
	c      uint32
	tx, ty uint8
}

// Primitive attributes, from the top byte of the command.
const (
	attrRaw      = 0x01 // texture colors are not modulated
	attrTransp   = 0x02 // semi-transparent
	attrTextured = 0x04
	attrQuad     = 0x08
	attrShaded   = 0x10
)

var ditherKernel = [16]float32{
	-4, +0, -3, +1,
	+2, -2, +3, -1,
	-3, +1, -4, +0,
	+3, -1, +2, -2,
}

// se11 sign-extends the 11-bit coordinate of a polygon or rectangle vertex.
func se11(v uint32) int16 { return int16(v<<5) >> 5 }

func (g *gpu) vramIndex(x, y int) int { return (x + y*vramWidth) & (len(g.vram) - 1) }

// inDrawArea compares as psxe does, unsigned, so negative coordinates are
// outside.
func (g *gpu) inDrawArea(x, y int) bool {
	return uint32(x) >= g.drawX1 && uint32(x) <= g.drawX2 &&
		uint32(y) >= g.drawY1 && uint32(y) <= g.drawY2
}

// texel fetches a texture pixel, through the CLUT for 4- and 8-bit
// textures. A zero result is a transparent pixel.
func (g *gpu) texel(tx, ty uint16, tpx, tpy, clutx, cluty uint32, depth uint32) uint16 {
	tx = uint16(uint32(tx)&^g.texwMX | g.texwOX&g.texwMX)
	ty = uint16(uint32(ty)&^g.texwMY | g.texwOY&g.texwMY)
	tx &= 0xff
	ty &= 0xff
	at := func(x, y uint32) uint16 { return g.vram[(x+y*vramWidth)&(vramWidth*vramHeight-1)] }

	switch depth {
	case 0:
		t := at(tpx+uint32(tx>>2), tpy+uint32(ty))
		index := uint32(t>>((tx&3)<<2)) & 0xf
		return at(clutx+index, cluty)
	case 1:
		t := at(tpx+uint32(tx>>1), tpy+uint32(ty))
		index := uint32(t>>((tx&1)<<3)) & 0xff
		return at(clutx+index, cluty)
	}
	return at(tpx+uint32(tx), tpy+uint32(ty))
}

// floor32 is math.Floor for a float32, through int32 where that is exact,
// which is any value of less than 2^23, and so every texture coordinate in
// practice. A value already whole is returned as it is, which keeps -0.
func floor32(v float32) float32 {
	if v <= -(1<<23) || v >= 1<<23 || v != v {
		return float32(math.Floor(float64(v)))
	}
	f := float32(int32(v))
	switch {
	case f == v:
		return v
	case f > v:
		f--
	}
	return f
}

// The parts of texel, split up so texelBilinear can apply the window to
// each of its coordinates once and decide the depth once, and small enough
// to inline.

func (g *gpu) windowX(tx uint16) uint32 { return (uint32(tx)&^g.texwMX | g.texwOX&g.texwMX) & 0xff }
func (g *gpu) windowY(ty uint16) uint32 { return (uint32(ty)&^g.texwMY | g.texwOY&g.texwMY) & 0xff }

func (g *gpu) vramXY(x, y uint32) uint16 {
	return g.vram[(x+y*vramWidth)&(vramWidth*vramHeight-1)]
}

func (g *gpu) texel4(x, y, tpx, tpy, clutx, cluty uint32) uint16 {
	t := g.vramXY(tpx+x>>2, tpy+y)
	return g.vramXY(clutx+uint32(t>>((x&3)<<2))&0xf, cluty)
}

func (g *gpu) texel8(x, y, tpx, tpy, clutx, cluty uint32) uint16 {
	t := g.vramXY(tpx+x>>1, tpy+y)
	return g.vramXY(clutx+uint32(t>>((x&1)<<3))&0xff, cluty)
}

func (g *gpu) texel16(x, y, tpx, tpy uint32) uint16 { return g.vramXY(tpx+x, tpy+y) }

// texelBilinear is psxe's filtered texture fetch for polygons, which blends
// the four texels around a fractional coordinate.
func (g *gpu) texelBilinear(tx, ty float32, tpx, tpy, clutx, cluty, depth uint32) uint16 {
	txf := floor32(tx)
	tyf := floor32(ty)
	txc := float32(txf + 1)
	tyc := float32(tyf + 1)

	// The four texels, as texel fetches them, with the window applied to
	// each coordinate once and the depth decided once.
	x0, x1 := g.windowX(uint16(int32(txf))), g.windowX(uint16(int32(txc)))
	y0, y1 := g.windowY(uint16(int32(tyf))), g.windowY(uint16(int32(tyc)))
	var s0, s1, s2, s3 uint16
	switch depth {
	case 0:
		if s0 = g.texel4(x0, y0, tpx, tpy, clutx, cluty); s0 == 0 {
			return 0
		}
		s1 = g.texel4(x1, y0, tpx, tpy, clutx, cluty)
		s2 = g.texel4(x0, y1, tpx, tpy, clutx, cluty)
		s3 = g.texel4(x1, y1, tpx, tpy, clutx, cluty)
	case 1:
		if s0 = g.texel8(x0, y0, tpx, tpy, clutx, cluty); s0 == 0 {
			return 0
		}
		s1 = g.texel8(x1, y0, tpx, tpy, clutx, cluty)
		s2 = g.texel8(x0, y1, tpx, tpy, clutx, cluty)
		s3 = g.texel8(x1, y1, tpx, tpy, clutx, cluty)
	default:
		if s0 = g.texel16(x0, y0, tpx, tpy); s0 == 0 {
			return 0
		}
		s1 = g.texel16(x1, y0, tpx, tpy)
		s2 = g.texel16(x0, y1, tpx, tpy)
		s3 = g.texel16(x1, y1, tpx, tpy)
	}

	ch := func(s uint16, shift uint) float32 { return float32((s >> shift) & 0x1f) }
	lerp := func(a, b, wa, wb float32) float32 {
		return float32(float32(a*wa) + float32(b*wb))
	}
	wx0, wx1 := float32(txc-tx), float32(tx-txf)
	wy0, wy1 := float32(tyc-ty), float32(ty-tyf)

	var out uint16
	for i, shift := range [3]uint{0, 5, 10} {
		q1 := lerp(ch(s0, shift), ch(s1, shift), wx0, wx1)
		q2 := lerp(ch(s2, shift), ch(s3, shift), wx0, wx1)
		out |= uint16(int32(lerp(q1, q2, wy0, wy1))) << (5 * i)
	}
	return out | (s0|s1|s2|s3)&0x8000
}

// clamp255 saturates a color component, and rounds it as roundf does,
// half away from zero.
func clamp255(v float32) uint32 {
	switch {
	case v >= 255:
		v = 255
	case v <= 0:
		v = 0
	}
	// math.Round, for a value this range: v + 0.5 is exact in float64, so
	// truncating it rounds half up, which for v >= 0 is half away from zero.
	return uint32(float64(v) + 0.5)
}

func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }

// modulate multiplies a texel by a color, where 80h is full brightness.
func modulate(texel uint16, mod uint32) uint16 {
	// psxe works this out in floats, as clamp255(t*m/128), but every step
	// is exact: t*m is at most 248*255, and dividing by 128 only moves the
	// point. So it is the integer t*m/128, rounded half up, at most 255.
	c := func(tshift, mshift uint) uint32 {
		t := uint32(texel>>tshift&0x1f) << 3
		m := mod >> mshift & 0xff
		return min((t*m+64)>>7, 255)
	}
	return bgr555(rgb(c(0, 0), c(5, 8), c(10, 16)))
}

// blend mixes a pixel with what is already in VRAM, by one of the four
// semi-transparency modes.
func (g *gpu) blend(color uint16, x, y int, mode uint32) uint16 {
	back := g.vram[g.vramIndex(x, y)]
	c := func(shift uint) uint32 {
		f := float32(((color >> shift) & 0x1f) << 3)
		b := float32(((back >> shift) & 0x1f) << 3)
		switch mode {
		case 0:
			f = float32(float32(0.5*b) + float32(0.5*f))
		case 1:
			f = float32(b + f)
		case 2:
			f = float32(b - f)
		case 3:
			// psxe's triangles use a double 0.25 here and its
			// rectangles a float one; with these values both are exact,
			// so the results agree.
			f = float32(b + float32(0.25*f))
		}
		return clamp255(f)
	}
	return bgr555(rgb(c(0), c(5), c(10)))
}

// edge is twice the signed area of the triangle abc, in ints, as psxe's
// EDGE macro computes it.
func edge(a, b, c vertex) int {
	return (int(b.x)-int(a.x))*(int(c.y)-int(a.y)) - (int(b.y)-int(a.y))*(int(c.x)-int(a.x))
}

// edgeStep is the edge function of ab, edge(a, b, p), as p steps along a
// row: dx is what a step right adds, and dy a step down. A pixel is drawn
// where the function is at least min. That is 0, or 1 where psxe's top-left
// rule excludes the edge itself, which it decides by the edge's direction so
// that two triangles sharing an edge do not both draw it.
type edgeStep struct {
	a           vertex
	dx, dy, min int
}

func newEdgeStep(a, b vertex) edgeStep {
	s := edgeStep{a: a, dx: -(int(b.y) - int(a.y)), dy: int(b.x) - int(a.x)}
	if b.y > a.y || (b.y == a.y && b.x < a.x) {
		s.min = 1
	}
	return s
}

// at is the edge function at a pixel.
func (s edgeStep) at(x, y int) int {
	return s.dy*(y-int(s.a.y)) + s.dx*(x-int(s.a.x))
}

// clip narrows a span of pixels, offsets lo to hi from where the function is
// e, to those it draws.
func (s edgeStep) clip(e, lo, hi int) (int, int) {
	switch {
	case s.dx > 0:
		lo = max(lo, -floorDiv(e-s.min, s.dx))
	case s.dx < 0:
		hi = min(hi, floorDiv(e-s.min, -s.dx))
	case e < s.min:
		return 0, -1
	}
	return lo, hi
}

// floorDiv divides, rounding down, by a positive divisor.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// polyData is what a polygon command carries besides its vertices.
type polyData struct {
	attrib uint8
	v      [4]vertex
	clut   uint16
	texp   uint16
}

func (g *gpu) triangle(v0, v1, v2 vertex, d polyData) {
	tpx := uint32(d.texp&0xf) << 6
	tpy := uint32(d.texp&0x10) << 4
	clutx := uint32(d.clut&0x3f) << 4
	cluty := uint32(d.clut>>6) & 0x1ff
	depth := uint32(d.texp>>7) & 3
	transp := d.attrib&attrTransp != 0
	var mode uint32
	if d.attrib&attrTextured != 0 {
		mode = uint32(d.texp>>5) & 3
	} else {
		mode = g.gpustat >> 5 & 3
	}

	a, b, c := v0, v1, v2
	if edge(v0, v1, v2) < 0 {
		b, c = v2, v1
	}
	for _, v := range []*vertex{&a, &b, &c} {
		v.x = int16(int32(v.x) + g.offX)
		v.y = int16(int32(v.y) + g.offY)
	}

	xmin := min(int(a.x), int(b.x), int(c.x))
	ymin := min(int(a.y), int(b.y), int(c.y))
	xmax := max(int(a.x), int(b.x), int(c.x))
	ymax := max(int(a.y), int(b.y), int(c.y))
	if xmax-xmin > 2048 || ymax-ymin > 1024 {
		return
	}

	// psxe divides by the area, but it is built with -Ofast, which lets
	// the compiler multiply by the reciprocal instead, and it does: that
	// is what gives the same last bit, and so the same texel, as psxe.
	area := float32(edge(a, b, c))
	inv := float32(1 / area)
	interp := func(z0, z1, z2, ca, cb, cc float32) float32 {
		s := float32(float32(z0*ca) + float32(z1*cb))
		s = float32(s + float32(z2*cc))
		return float32(s * inv)
	}

	// The bounding box, clipped to the drawing area, which psxe compares
	// unsigned, so negative coordinates are outside it.
	x0 := max(xmin, int(g.drawX1))
	x1 := min(xmax-1, int(g.drawX2))
	y0 := max(ymin, int(g.drawY1))
	y1 := min(ymax-1, int(g.drawY2))
	edges := [3]edgeStep{newEdgeStep(b, c), newEdgeStep(c, a), newEdgeStep(a, b)}

	// A flat, opaque, untextured triangle draws one color over each span.
	flat := d.attrib&(attrShaded|attrTextured|attrTransp) == 0
	flatColor := bgr555(d.v[0].c)

	for y := y0; y <= y1; y++ {
		// Each edge function is linear along the row, so the pixels that
		// pass all three are one span, found without testing each.
		lo, hi := 0, x1-x0
		var e [3]int
		for i := range edges {
			e[i] = edges[i].at(x0, y)
			lo, hi = edges[i].clip(e[i], lo, hi)
		}
		if flat {
			// The span is within the drawing area, so within a row of
			// VRAM.
			if lo <= hi {
				row := g.vramIndex(0, y)
				fill16(g.vram[row+x0+lo:row+x0+hi+1], flatColor)
			}
			continue
		}
		for i := range e {
			e[i] += edges[i].dx * lo
		}

		for x := x0 + lo; x <= x0+hi; x, e[0], e[1], e[2] = x+1, e[0]+edges[0].dx, e[1]+edges[1].dx, e[2]+edges[2].dx {
			z0, z1, z2 := float32(e[0]), float32(e[1]), float32(e[2])

			var mod uint32
			if d.attrib&attrShaded != 0 {
				dither := ditherKernel[(x-xmin)&3+((y-ymin)&3)*4]
				ch := func(shift uint) uint32 {
					v := interp(z0, z1, z2,
						float32(a.c>>shift&0xff), float32(b.c>>shift&0xff), float32(c.c>>shift&0xff))
					return clamp255(float32(v + dither))
				}
				mod = ch(16)<<16 | ch(8)<<8 | ch(0)
			} else {
				mod = d.v[0].c
			}

			var color uint16
			if d.attrib&attrTextured != 0 {
				tx := interp(z0, z1, z2, float32(a.tx), float32(b.tx), float32(c.tx))
				ty := interp(z0, z1, z2, float32(a.ty), float32(b.ty), float32(c.ty))
				t := g.texelBilinear(tx, ty, tpx, tpy, clutx, cluty, depth)
				if t == 0 {
					continue
				}
				// The texel's top bit decides semi-transparency, and in
				// psxe that decision carries over to the next pixel.
				if d.attrib&attrTransp != 0 {
					transp = t&0x8000 != 0
				}
				if d.attrib&attrRaw != 0 {
					color = t
				} else {
					color = modulate(t, mod)
				}
			} else {
				color = bgr555(mod)
			}

			if transp {
				color = g.blend(color, x, y, mode)
			}
			g.vram[g.vramIndex(x, y)] = color
		}
	}
}

// rectData is what a rectangle command carries.
type rectData struct {
	attrib        uint8
	v0            vertex
	clut          uint16
	width, height uint16
}

func (g *gpu) drawRect(d rectData) {
	if d.v0.x >= 1024 || d.v0.y >= 512 || d.v0.x <= -1024 || d.v0.y <= -512 {
		return
	}
	var width, height uint16
	switch d.attrib >> 3 & 3 {
	case 0:
		width, height = d.width, d.height
	case 1:
		width, height = 1, 1
	case 2:
		width, height = 8, 8
	case 3:
		width, height = 16, 16
	}

	textured := d.attrib&attrTextured != 0
	transp := d.attrib&attrTransp != 0
	mode := g.gpustat >> 5 & 3
	clutx := uint32(d.clut&0x3f) << 4
	cluty := uint32(d.clut>>6) & 0x1ff

	d.v0.x = int16(int32(d.v0.x) + g.offX)
	d.v0.y = int16(int32(d.v0.y) + g.offY)

	clamp := func(v int) int { return min(max(v, -1024), 1024) }
	xmax := clamp(int(d.v0.x) + int(width))
	ymax := clamp(int(d.v0.y) + int(height))
	x0 := int16(clamp(int(d.v0.x)))
	y0 := int16(clamp(int(d.v0.y)))

	// A flat, opaque rectangle is one color over the part of each row in
	// the drawing area, which is within a row of VRAM.
	if !textured && !transp {
		color := bgr555(d.v0.c)
		xl, xr := max(int(x0), int(g.drawX1)), min(xmax-1, int(g.drawX2))
		for y := max(int(y0), int(g.drawY1)); y <= min(ymax-1, int(g.drawY2)); y++ {
			if xl <= xr {
				row := g.vramIndex(0, y)
				fill16(g.vram[row+xl:row+xr+1], color)
			}
		}
		return
	}

	yc := 0
	for y := y0; int(y) < ymax; y++ {
		xc := 0
		for x := x0; int(x) < xmax; x, xc = x+1, xc+1 {
			if !g.inDrawArea(int(x), int(y)) {
				continue
			}
			var color uint16
			if textured {
				t := g.texel(uint16(int(d.v0.tx)+xc), uint16(int(d.v0.ty)+yc),
					g.texpX, g.texpY, clutx, cluty, g.texpD)
				if t == 0 {
					continue
				}
				if d.attrib&attrTransp != 0 {
					transp = t&0x8000 != 0
				}
				color = modulate(t, d.v0.c)
			} else {
				color = bgr555(d.v0.c)
			}
			if transp {
				color = g.blend(color, int(x), int(y), mode)
			}
			g.vram[g.vramIndex(int(x), int(y))] = color
		}
		yc++
	}
}

// drawLine draws a line in one color, with Bresenham's algorithm from
// whichever end makes it run forwards. The far end is not drawn.
func (g *gpu) drawLine(v0, v1 vertex, color uint16) {
	x0 := int(int16(int32(v0.x) + g.offX))
	y0 := int(int16(int32(v0.y) + g.offY))
	x1 := int(int16(int32(v1.x) + g.offX))
	y1 := int(int16(int32(v1.y) + g.offY))

	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	plot := func(x, y int) {
		if x < vramWidth && y < vramHeight && x >= 0 && y >= 0 && g.inDrawArea(x, y) {
			g.vram[x+y*vramWidth] = color
		}
	}
	low := func(x0, y0, x1, y1 int) {
		dx, dy, yi := x1-x0, y1-y0, 1
		if dy < 0 {
			yi, dy = -1, -dy
		}
		d, y := 2*dy-dx, y0
		for x := x0; x < x1; x++ {
			plot(x, y)
			if d > 0 {
				y += yi
				d += 2 * (dy - dx)
			} else {
				d += 2 * dy
			}
		}
	}
	high := func(x0, y0, x1, y1 int) {
		dx, dy, xi := x1-x0, y1-y0, 1
		if dx < 0 {
			xi, dx = -1, -dx
		}
		d, x := 2*dx-dy, x0
		for y := y0; y < y1; y++ {
			plot(x, y)
			if d > 0 {
				x += xi
				d += 2 * (dx - dy)
			} else {
				d += 2 * dx
			}
		}
	}

	switch {
	case abs(y1-y0) < abs(x1-x0) && x0 > x1:
		low(x1, y1, x0, y0)
	case abs(y1-y0) < abs(x1-x0):
		low(x0, y0, x1, y1)
	case y0 > y1:
		high(x1, y1, x0, y0)
	default:
		high(x0, y0, x1, y1)
	}
}
