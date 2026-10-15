package emu

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestTriangleSpans draws random triangles, of every kind, into random VRAM
// with random drawing areas and offsets, and checks that the span
// rasterizer draws exactly what psxe's per-pixel one does.
func TestTriangleSpans(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	got, want := newGPU(nil, nil), newGPU(nil, nil)
	for i := range got.vram {
		got.vram[i] = uint16(r.Uint32())
	}
	want.vram = got.vram

	coord := func() int16 {
		if r.IntN(8) == 0 {
			return int16(r.IntN(4096) - 2048) // anywhere, often too big to draw
		}
		return int16(r.IntN(400) - 40)
	}
	for trial := range 3000 {
		for _, g := range []*gpu{got, want} {
			r2 := rand.New(rand.NewPCG(uint64(trial), 3))
			g.drawX1, g.drawY1 = uint32(r2.IntN(64)), uint32(r2.IntN(64))
			g.drawX2, g.drawY2 = uint32(r2.IntN(1024)), uint32(r2.IntN(512))
			g.offX, g.offY = int32(r2.IntN(64)-32), int32(r2.IntN(64)-32)
			g.gpustat = uint32(r2.IntN(4)) << 5
			g.texwMX, g.texwMY = uint32(r2.IntN(4))<<3, uint32(r2.IntN(4))<<3
			g.texwOX, g.texwOY = uint32(r2.IntN(256)), uint32(r2.IntN(256))
		}

		var d polyData
		d.attrib = uint8(r.IntN(32)) &^ attrQuad
		d.clut, d.texp = uint16(r.Uint32()), uint16(r.Uint32())
		for i := range 3 {
			d.v[i] = vertex{x: coord(), y: coord(), c: r.Uint32() & 0xffffff, tx: uint8(r.Uint32()), ty: uint8(r.Uint32())}
		}

		got.triangle(d.v[0], d.v[1], d.v[2], d)
		want.triangleRef(d.v[0], d.v[1], d.v[2], d)
		if got.vram != want.vram {
			for i := range got.vram {
				if got.vram[i] != want.vram[i] {
					msg := fmt.Sprintf("trial %d, %+v: pixel (%d, %d)", trial, d, i%vramWidth, i/vramWidth)
					assert.Equal(t, want.vram[i], got.vram[i], msg)
					t.FailNow()
				}
			}
		}
	}
}

// triangleRef is psxe's rasterizer as first ported, testing every pixel of
// the bounding box against every edge.
func (g *gpu) triangleRef(v0, v1, v2 vertex, d polyData) {
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

	for y := ymin; y < ymax; y++ {
		for x := xmin; x < xmax; x++ {
			if !g.inDrawArea(x, y) {
				continue
			}
			p := vertex{x: int16(x), y: int16(y)}

			z0 := float32(edge(b, c, p))
			if topLeft(z0, b, c) {
				continue
			}
			z1 := float32(edge(c, a, p))
			if topLeft(z1, c, a) {
				continue
			}
			z2 := float32(edge(a, b, p))
			if topLeft(z2, a, b) {
				continue
			}

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

// topLeft reports whether a pixel on the edge ab is excluded.
func topLeft(z float32, a, b vertex) bool {
	return z < 0 || (z == 0 && (b.y > a.y || (b.y == a.y && b.x < a.x)))
}

// TestRectFills draws random rectangles, and fills random rectangles of
// VRAM, and checks that the flat ones, drawn a row at a time, come out as
// psxe's per-pixel loops draw them.
func TestRectFills(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	got, want := newGPU(nil, nil), newGPU(nil, nil)
	for i := range got.vram {
		got.vram[i] = uint16(r.Uint32())
	}
	want.vram = got.vram

	for trial := range 3000 {
		for _, g := range []*gpu{got, want} {
			r2 := rand.New(rand.NewPCG(uint64(trial), 5))
			g.drawX1, g.drawY1 = uint32(r2.IntN(64)), uint32(r2.IntN(64))
			g.drawX2, g.drawY2 = uint32(r2.IntN(1024)), uint32(r2.IntN(512))
			g.offX, g.offY = int32(r2.IntN(64)-32), int32(r2.IntN(64)-32)
			g.gpustat = uint32(r2.IntN(4)) << 5
		}

		if trial%2 == 0 {
			// Only the flat rectangles take the new path, but drawing the
			// others as well checks they are left as they were.
			d := rectData{
				attrib: uint8(r.IntN(32)) &^ attrTextured,
				v0:     vertex{x: int16(r.IntN(2400) - 1200), y: int16(r.IntN(1200) - 600), c: r.Uint32() & 0xffffff},
				width:  uint16(r.IntN(1100)),
				height: uint16(r.IntN(600)),
			}
			got.drawRect(d)
			want.drawRectRef(d)
		} else {
			// The command with its arguments all received.
			buf := [3]uint32{0x02000000 | r.Uint32()&0xffffff, r.Uint32(), r.Uint32()}
			for _, g := range []*gpu{got, want} {
				copy(g.buf[:], buf[:])
				g.state, g.argsRemaining = gpuRecvArgs, 0
			}
			got.fill()
			want.fillRef()
		}
		if got.vram != want.vram {
			for i := range got.vram {
				if got.vram[i] != want.vram[i] {
					msg := fmt.Sprintf("trial %d: pixel (%d, %d)", trial, i%vramWidth, i/vramWidth)
					assert.Equal(t, want.vram[i], got.vram[i], msg)
					t.FailNow()
				}
			}
		}
	}
}

// drawRectRef and fillRef are psxe's rectangle and fill, as first ported,
// a pixel at a time.

func (g *gpu) drawRectRef(d rectData) {
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

func (g *gpu) fillRef() {
	if !g.expect(2) {
		return
	}
	x0 := int(g.buf[1] & 0xffff & 0x3f0)
	y0 := int(g.buf[1] >> 16 & 0x1ff)
	w := int((g.buf[2]&0x3ff + 0x0f) & 0xfffffff0)
	h := int(g.buf[2] >> 16 & 0x1ff)
	color := bgr555(g.buf[0] & 0xffffff)
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if x < vramWidth && y < vramHeight {
				g.vram[x+y*vramWidth] = color
			}
		}
	}
	g.state = gpuRecvCmd
}
