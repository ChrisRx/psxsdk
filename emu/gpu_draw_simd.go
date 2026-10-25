//go:build goexperiment.simd && (amd64 || wasm)

package emu

import (
	"simd/archsimd"
	"unsafe"
)

// Triangles and rectangles are drawn four pixels at a time. Each lane of a
// vector is a pixel, and works through what triangle or drawRect works
// through for it, operation for operation, so a lane rounds just as the
// scalar code does; only the texel fetches go a lane at a time.

// lanes numbers the lanes, and ditherRows holds each row of ditherKernel
// twice over, so that the dither of four pixels from any column is four in
// a row.
var (
	lanes      = [4]int32{0, 1, 2, 3}
	ditherRows = func() (rows [4][8]float32) {
		for i := range rows {
			copy(rows[i][:4], ditherKernel[i*4:])
			copy(rows[i][4:], ditherKernel[i*4:])
		}
		return rows
	}()
)

// triangleVec draws a triangle that is not flat four pixels at a time, and
// reports whether it did. It leaves to triangle one too close to the right
// edge of VRAM for four pixels from its last, and one whose texture might
// be among the pixels it draws, since four pixels' texels are fetched
// before any of them is drawn.
func (g *gpu) triangleVec(r *raster) bool {
	attrib := r.d.attrib
	textured := attrib&attrTextured != 0
	if !haveSIMD || r.x1 > vramWidth-4 ||
		textured && drawsOnTexture(r.tpx, r.tpy, r.depth, r.clutx, r.cluty, r.x0, r.x1, r.y0, r.y1) {
		return false
	}
	shaded := attrib&attrShaded != 0
	transp := attrib&attrTransp != 0
	raw := attrib&attrRaw != 0

	inv := archsimd.BroadcastFloat32x4(r.inv)
	lane := archsimd.LoadInt32x4(lanes[:])

	// The edge functions at the four pixels of a group, and what moving on
	// to the next group adds to them.
	dx0, dx1, dx2 := int32(r.edges[0].dx), int32(r.edges[1].dx), int32(r.edges[2].dx)
	ramp0, ramp1, ramp2 := lane.Mul(archsimd.BroadcastInt32x4(dx0)), lane.Mul(archsimd.BroadcastInt32x4(dx1)), lane.Mul(archsimd.BroadcastInt32x4(dx2))
	step0, step1, step2 := archsimd.BroadcastInt32x4(4*dx0), archsimd.BroadcastInt32x4(4*dx1), archsimd.BroadcastInt32x4(4*dx2)

	// The corners' colors, by component, and texture coordinates.
	comp := func(shift uint) (archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4) {
		return archsimd.BroadcastFloat32x4(float32(r.a.c >> shift & 0xff)), archsimd.BroadcastFloat32x4(float32(r.b.c >> shift & 0xff)), archsimd.BroadcastFloat32x4(float32(r.c.c >> shift & 0xff))
	}
	ra, rb, rc := comp(0)
	ga, gb, gc := comp(8)
	ba, bb, bc := comp(16)
	txa, txb, txc := archsimd.BroadcastFloat32x4(float32(r.a.tx)), archsimd.BroadcastFloat32x4(float32(r.b.tx)), archsimd.BroadcastFloat32x4(float32(r.c.tx))
	tya, tyb, tyc := archsimd.BroadcastFloat32x4(float32(r.a.ty)), archsimd.BroadcastFloat32x4(float32(r.b.ty)), archsimd.BroadcastFloat32x4(float32(r.c.ty))

	// Without shading, every pixel has the first corner's color, and
	// modulating by the neutral color leaves a texel as it is but for its
	// top bit.
	c := r.d.v[0].c
	neutral := !shaded && c == 0x808080
	mr, mg, mb := archsimd.BroadcastUint32x4(c&0xff), archsimd.BroadcastUint32x4(c>>8&0xff), archsimd.BroadcastUint32x4(c>>16&0xff)
	flat := archsimd.BroadcastUint32x4(uint32(bgr555(c)))

	tex := g.newTexVec(r)
	zero := archsimd.BroadcastUint32x4(0)
	ea, eb, ec := &r.edges[0], &r.edges[1], &r.edges[2]
	for y := r.y0; y <= r.y1; y++ {
		lo, hi := ec.clip(eb.clip(ea.clip(0, r.x1-r.x0)))
		e0 := archsimd.BroadcastInt32x4(int32(ea.next(lo))).Add(ramp0)
		e1 := archsimd.BroadcastInt32x4(int32(eb.next(lo))).Add(ramp1)
		e2 := archsimd.BroadcastInt32x4(int32(ec.next(lo))).Add(ramp2)
		if lo > hi {
			continue
		}
		x, end := r.x0+lo, r.x0+hi
		dither := archsimd.LoadFloat32x4(ditherRows[(y-r.ymin)&3][(x-r.xmin)&3:][:4])
		row := y * vramWidth
		for ; x <= end; x, e0, e1, e2 = x+4, e0.Add(step0), e1.Add(step1), e2.Add(step2) {
			// The lanes past the end of the span are worked out like the
			// rest, but VRAM keeps what it had there.
			draw := lane.Less(archsimd.BroadcastInt32x4(int32(end - x + 1)))
			z0, z1, z2 := e0.ConvertToFloat32(), e1.ConvertToFloat32(), e2.ConvertToFloat32()

			cr, cg, cb := mr, mg, mb
			if shaded {
				cr = round255(interp4(z0, z1, z2, ra, rb, rc, inv).Add(dither))
				cg = round255(interp4(z0, z1, z2, ga, gb, gc, inv).Add(dither))
				cb = round255(interp4(z0, z1, z2, ba, bb, bc, inv).Add(dither))
			}

			var color archsimd.Uint32x4
			semi := draw
			switch {
			case textured:
				t := g.bilinear4(&tex, interp4(z0, z1, z2, txa, txb, txc, inv), interp4(z0, z1, z2, tya, tyb, tyc, inv))
				draw = draw.And(t.NotEqual(zero))
				semi = t.And(archsimd.BroadcastUint32x4(0x8000)).NotEqual(zero)
				switch {
				case raw:
					color = t
				case neutral:
					color = t.And(archsimd.BroadcastUint32x4(0x7fff))
				default:
					color = modulate4(t, cr, cg, cb)
				}
			case shaded:
				color = bgr4(cr, cg, cb)
			default:
				color = flat
			}

			p := (*uint64)(unsafe.Pointer(&g.vram[row+x]))
			back := unpack4(*p)
			if transp {
				color = blend4(color, back, r.mode).IfElse(semi, color)
			}
			*p = pack4(color.IfElse(draw, back))
		}
	}
	return true
}

// rectVec draws a rectangle that is textured or semi-transparent as
// drawRect does, from x0, y0 to short of xmax, ymax, four pixels at a time,
// and reports whether it did. It leaves to drawRect what triangleVec leaves
// to triangle.
func (g *gpu) rectVec(d rectData, x0, y0, xmax, ymax int) bool {
	textured := d.attrib&attrTextured != 0
	clutx := uint32(d.clut&0x3f) << 4
	cluty := uint32(d.clut>>6) & 0x1ff
	xl, xr := max(x0, int(g.drawX1)), min(xmax-1, int(g.drawX2))
	yl, yr := max(y0, int(g.drawY1)), min(ymax-1, int(g.drawY2))
	if !haveSIMD || xr > vramWidth-4 ||
		textured && drawsOnTexture(g.texpX, g.texpY, g.texpD, clutx, cluty, xl, xr, yl, yr) {
		return false
	}
	transp := d.attrib&attrTransp != 0
	mode := g.gpustat >> 5 & 3
	c := d.v0.c
	neutral := c == 0x808080
	mr, mg, mb := archsimd.BroadcastUint32x4(c&0xff), archsimd.BroadcastUint32x4(c>>8&0xff), archsimd.BroadcastUint32x4(c>>16&0xff)
	flat := archsimd.BroadcastUint32x4(uint32(bgr555(c)))
	lane := archsimd.LoadInt32x4(lanes[:])
	zero := archsimd.BroadcastUint32x4(0)

	// The texture coordinates of the pixels count from the rectangle's
	// corner, through the window.
	keepX, setX := archsimd.BroadcastUint32x4(0xff&^g.texwMX), archsimd.BroadcastUint32x4(g.texwOX&g.texwMX)
	clut := clutx + cluty*vramWidth
	for y := yl; y <= yr; y++ {
		v := (uint32(int(d.v0.ty)+y-y0)&^g.texwMY | g.texwOY&g.texwMY) & 0xff
		page := archsimd.BroadcastUint32x4(g.texpX + (g.texpY+v)*vramWidth)
		u := archsimd.BroadcastUint32x4(uint32(int(d.v0.tx) + xl - x0)).Add(lane.ToBits())
		row := y * vramWidth
		for x := xl; x <= xr; x, u = x+4, u.Add(archsimd.BroadcastUint32x4(4)) {
			draw := lane.Less(archsimd.BroadcastInt32x4(int32(xr - x + 1)))
			semi := draw
			color := flat
			if textured {
				tu := u.And(keepX).Or(setX)
				var t archsimd.Uint32x4
				switch g.texpD {
				case 0:
					t = g.lookup4(page.Add(tu.ShiftAllRight(2)), tu.And(archsimd.BroadcastUint32x4(3)).ShiftAllLeft(2), clut, 0xf)
				case 1:
					t = g.lookup4(page.Add(tu.ShiftAllRight(1)), tu.And(archsimd.BroadcastUint32x4(1)).ShiftAllLeft(3), clut, 0xff)
				default:
					t = g.gather4(page.Add(tu))
				}
				draw = draw.And(t.NotEqual(zero))
				semi = t.And(archsimd.BroadcastUint32x4(0x8000)).NotEqual(zero)
				if neutral {
					color = t.And(archsimd.BroadcastUint32x4(0x7fff))
				} else {
					color = modulate4(t, mr, mg, mb)
				}
			}

			p := (*uint64)(unsafe.Pointer(&g.vram[row+x]))
			back := unpack4(*p)
			if transp {
				color = blend4(color, back, mode).IfElse(semi, color)
			}
			*p = pack4(color.IfElse(draw, back))
		}
	}
	return true
}

// drawsOnTexture reports whether a texture page, of a depth, or its CLUT
// overlaps a box of pixels, or wraps round the edge of VRAM, where it is not
// worth finding out.
func drawsOnTexture(tpx, tpy, depth, clutx, cluty uint32, x0, x1, y0, y1 int) bool {
	overlaps := func(left, right, top, bottom int) bool {
		return left <= x1 && x0 <= right && top <= y1 && y0 <= bottom
	}
	// A row of the page is 256 texels, which are 64, 128 or 256 pixels of
	// VRAM, and a CLUT as many colors as an index has values.
	width := 256 >> (2 - min(depth, 2))
	px, py := int(tpx), int(tpy)
	if px+width > vramWidth || overlaps(px, px+width-1, py, py+255) {
		return true
	}
	if depth >= 2 {
		return false
	}
	colors := 16 << (4 * depth)
	cx, cy := int(clutx), int(cluty)
	return cx+colors > vramWidth || overlaps(cx, cx+colors-1, cy, cy)
}

// interp4 is triangle's interp for four lanes.
func interp4(z0, z1, z2, ca, cb, cc, inv archsimd.Float32x4) archsimd.Float32x4 {
	return z0.Mul(ca).Add(z1.Mul(cb)).Add(z2.Mul(cc)).Mul(inv)
}

// round255 is clamp255 for four lanes: each rounded half away from zero by
// adding one to the whole part where the fraction is at least a half, which
// unlike adding a half before truncating never rounds the sum, then
// saturated. Saturating last comes to the same, and does it in integers.
func round255(v archsimd.Float32x4) archsimd.Uint32x4 {
	w := v.Trunc()
	up := archsimd.BroadcastInt32x4(1).Masked(v.Sub(w).GreaterEqual(archsimd.BroadcastFloat32x4(0.5)))
	return w.ConvertToInt32().Add(up).Min(archsimd.BroadcastInt32x4(255)).Max(archsimd.BroadcastInt32x4(0)).ToBits()
}

// bgr4 is bgr555 for four lanes of components.
func bgr4(r, g, b archsimd.Uint32x4) archsimd.Uint32x4 {
	return r.ShiftAllRight(3).Or(g.ShiftAllRight(3).ShiftAllLeft(5)).Or(b.ShiftAllRight(3).ShiftAllLeft(10))
}

// modulate4 is modulate for four lanes, with the color by component.
func modulate4(t, r, g, b archsimd.Uint32x4) archsimd.Uint32x4 {
	c := func(t, m archsimd.Uint32x4) archsimd.Uint32x4 {
		return t.And(archsimd.BroadcastUint32x4(0x1f)).ShiftAllLeft(3).Mul(m).Add(archsimd.BroadcastUint32x4(64)).ShiftAllRight(7).Min(archsimd.BroadcastUint32x4(255))
	}
	return bgr4(c(t, r), c(t.ShiftAllRight(5), g), c(t.ShiftAllRight(10), b))
}

// blend4 is blend for four lanes. With components of five bits, each of
// blend's float sums is exact, and what it saturates and rounds comes out
// as these.
func blend4(f, b archsimd.Uint32x4, mode uint32) archsimd.Uint32x4 {
	m := archsimd.BroadcastUint32x4(0x1f)
	c := func(f, b archsimd.Uint32x4) archsimd.Uint32x4 {
		f, b = f.And(m), b.And(m)
		switch mode {
		case 0:
			return b.Add(f).ShiftAllRight(1)
		case 1:
			return b.Add(f).Min(m)
		case 2:
			return b.Max(f).Sub(f)
		}
		return b.Add(f.ShiftAllRight(2)).Min(m)
	}
	return c(f, b).
		Or(c(f.ShiftAllRight(5), b.ShiftAllRight(5)).ShiftAllLeft(5)).
		Or(c(f.ShiftAllRight(10), b.ShiftAllRight(10)).ShiftAllLeft(10))
}

// packIndex picks the low two bytes of each 32-bit lane for pack4, and
// zeros for the rest.
var packIndex = [16]int8{0, 1, 4, 5, 8, 9, 12, 13, -1, -1, -1, -1, -1, -1, -1, -1}

// pack4 narrows four lanes to the pixels in their low halves, packed into
// 64 bits.
func pack4(v archsimd.Uint32x4) uint64 {
	return permute(v.ReshapeToUint8s(), archsimd.LoadInt8x16(packIndex[:])).ReshapeToUint64s().GetElem(0)
}

// unpack4 widens four pixels to a lane each.
func unpack4(p uint64) archsimd.Uint32x4 {
	var v archsimd.Uint64x2
	return v.SetElem(0, p).ReshapeToUint16s().ExtendLo4ToUint32()
}

// texVec is what bilinear4 needs of a triangle's texture, as vectors where
// it works on vectors: the texture window, applied to a coordinate by
// keeping the bits keepX or keepY and setting setX or setY, and the page,
// the address of its first pixel, and the CLUT's.
type texVec struct {
	keepX, setX, keepY, setY archsimd.Uint32x4
	page                     archsimd.Uint32x4
	depth                    uint32
	clut                     uint32
}

func (g *gpu) newTexVec(r *raster) texVec {
	return texVec{
		keepX: archsimd.BroadcastUint32x4(0xff &^ g.texwMX),
		setX:  archsimd.BroadcastUint32x4(g.texwOX & g.texwMX),
		keepY: archsimd.BroadcastUint32x4(0xff &^ g.texwMY),
		setY:  archsimd.BroadcastUint32x4(g.texwOY & g.texwMY),
		page:  archsimd.BroadcastUint32x4(r.tpx + r.tpy*vramWidth),
		depth: r.depth,
		clut:  r.clutx + r.cluty*vramWidth,
	}
}

// bilinear4 is texelBilinear for four lanes.
func (g *gpu) bilinear4(t *texVec, tx, ty archsimd.Float32x4) archsimd.Uint32x4 {
	txf, tyf := tx.Floor(), ty.Floor()
	one := archsimd.BroadcastFloat32x4(1)
	txc, tyc := txf.Add(one), tyf.Add(one)

	// The texel coordinates, through the window, and the rows of the page
	// they are in.
	x0 := txf.ConvertToInt32().ToBits().And(t.keepX).Or(t.setX)
	x1 := txc.ConvertToInt32().ToBits().And(t.keepX).Or(t.setX)
	y0 := tyf.ConvertToInt32().ToBits().And(t.keepY).Or(t.setY).ShiftAllLeft(10).Add(t.page)
	y1 := tyc.ConvertToInt32().ToBits().And(t.keepY).Or(t.setY).ShiftAllLeft(10).Add(t.page)

	// The four texels of each lane, s0 to s3 as texelBilinear has them.
	// For 4- and 8-bit textures, each is looked up by the index in a pixel
	// of the page, at a shift by where it is in the pixel.
	var s0, s1, s2, s3 archsimd.Uint32x4
	switch t.depth {
	case 0:
		sx0, sx1 := x0.And(archsimd.BroadcastUint32x4(3)).ShiftAllLeft(2), x1.And(archsimd.BroadcastUint32x4(3)).ShiftAllLeft(2)
		x0, x1 = x0.ShiftAllRight(2), x1.ShiftAllRight(2)
		s0, s1 = g.lookup4(y0.Add(x0), sx0, t.clut, 0xf), g.lookup4(y0.Add(x1), sx1, t.clut, 0xf)
		s2, s3 = g.lookup4(y1.Add(x0), sx0, t.clut, 0xf), g.lookup4(y1.Add(x1), sx1, t.clut, 0xf)
	case 1:
		sx0, sx1 := x0.And(archsimd.BroadcastUint32x4(1)).ShiftAllLeft(3), x1.And(archsimd.BroadcastUint32x4(1)).ShiftAllLeft(3)
		x0, x1 = x0.ShiftAllRight(1), x1.ShiftAllRight(1)
		s0, s1 = g.lookup4(y0.Add(x0), sx0, t.clut, 0xff), g.lookup4(y0.Add(x1), sx1, t.clut, 0xff)
		s2, s3 = g.lookup4(y1.Add(x0), sx0, t.clut, 0xff), g.lookup4(y1.Add(x1), sx1, t.clut, 0xff)
	default:
		s0, s1 = g.gather4(y0.Add(x0)), g.gather4(y0.Add(x1))
		s2, s3 = g.gather4(y1.Add(x0)), g.gather4(y1.Add(x1))
	}

	wx0, wx1 := txc.Sub(tx), tx.Sub(txf)
	wy0, wy1 := tyc.Sub(ty), ty.Sub(tyf)
	m := archsimd.BroadcastUint32x4(0x1f)
	c := func(s0, s1, s2, s3 archsimd.Uint32x4) archsimd.Uint32x4 {
		ch := func(s archsimd.Uint32x4) archsimd.Float32x4 { return s.And(m).BitsToInt32().ConvertToFloat32() }
		q1 := lerp4(ch(s0), ch(s1), wx0, wx1)
		q2 := lerp4(ch(s2), ch(s3), wx0, wx1)
		return lerp4(q1, q2, wy0, wy1).ConvertToInt32().ToBits()
	}
	out := c(s0, s1, s2, s3).
		Or(c(s0.ShiftAllRight(5), s1.ShiftAllRight(5), s2.ShiftAllRight(5), s3.ShiftAllRight(5)).ShiftAllLeft(5)).
		Or(c(s0.ShiftAllRight(10), s1.ShiftAllRight(10), s2.ShiftAllRight(10), s3.ShiftAllRight(10)).ShiftAllLeft(10)).
		Or(s0.Or(s1).Or(s2).Or(s3).And(archsimd.BroadcastUint32x4(0x8000)))
	// A transparent first texel makes the whole lane transparent.
	return out.Masked(s0.NotEqual(archsimd.BroadcastUint32x4(0)))
}

// gather4 fetches the pixels at four addresses in VRAM, one lane at a time.
func (g *gpu) gather4(a archsimd.Uint32x4) archsimd.Uint32x4 {
	v := &g.vram
	const mask = vramWidth*vramHeight - 1
	var p archsimd.Uint32x4
	return p.SetElem(0, uint32(v[a.GetElem(0)&mask])).
		SetElem(1, uint32(v[a.GetElem(1)&mask])).
		SetElem(2, uint32(v[a.GetElem(2)&mask])).
		SetElem(3, uint32(v[a.GetElem(3)&mask]))
}

// lookup4 fetches four colors from the CLUT at clut, each by the index of
// bits m at a shift in the pixel at an address.
func (g *gpu) lookup4(a, shift archsimd.Uint32x4, clut, m uint32) archsimd.Uint32x4 {
	v := &g.vram
	const mask = vramWidth*vramHeight - 1
	i := g.gather4(a)
	var p archsimd.Uint32x4
	return p.SetElem(0, uint32(v[(clut+i.GetElem(0)>>(shift.GetElem(0)&15)&m)&mask])).
		SetElem(1, uint32(v[(clut+i.GetElem(1)>>(shift.GetElem(1)&15)&m)&mask])).
		SetElem(2, uint32(v[(clut+i.GetElem(2)>>(shift.GetElem(2)&15)&m)&mask])).
		SetElem(3, uint32(v[(clut+i.GetElem(3)>>(shift.GetElem(3)&15)&m)&mask]))
}

// lerp4 is texelBilinear's lerp for four lanes.
func lerp4(a, b, wa, wb archsimd.Float32x4) archsimd.Float32x4 {
	return a.Mul(wa).Add(b.Mul(wb))
}
