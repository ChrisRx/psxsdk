package emu

import (
	"image/color"
	"math"
	"math/rand/v2"
	"testing"

	"go.chrisrx.dev/x/assert"
)

const gp0 = 0x1f801810

// drawing sets up a machine with the whole of VRAM as the drawing area.
func drawing(t *testing.T) *Machine {
	t.Helper()
	m := newTestMachine(t)
	m.Bus.Write32(gp0, 0xe3000000)              // drawing area top left (0, 0)
	m.Bus.Write32(gp0, 0xe4000000|511<<10|1023) // bottom right (1023, 511)
	return m
}

func (m *Machine) pixel(x, y int) uint16 { return m.VRAM()[x+y*vramWidth] }

func TestDrawFlatTriangles(t *testing.T) {
	m := drawing(t)
	// Two triangles sharing the diagonal of a 10x10 square, in red and
	// blue. Each pixel of the square is drawn by exactly one of them.
	m.Bus.Write32(gp0, 0x200000ff)
	m.Bus.Write32(gp0, 10<<16|10)
	m.Bus.Write32(gp0, 10<<16|20)
	m.Bus.Write32(gp0, 20<<16|10)
	m.Bus.Write32(gp0, 0x20ff0000)
	m.Bus.Write32(gp0, 10<<16|20)
	m.Bus.Write32(gp0, 20<<16|20)
	m.Bus.Write32(gp0, 20<<16|10)

	const red, blue = 0x001f, 0x7c00
	counts := map[uint16]int{}
	for y := 10; y < 20; y++ {
		for x := 10; x < 20; x++ {
			counts[m.pixel(x, y)]++
		}
	}
	assert.Equal(t, map[uint16]int{red: 55, blue: 45}, counts, "the square should be covered once")
	assert.Equal(t, uint16(0), m.pixel(20, 10), "the right edge is not drawn")
	assert.Equal(t, uint16(0), m.pixel(10, 20), "nor the bottom one")
}

func TestDrawTexturedRectangle(t *testing.T) {
	m := drawing(t)
	// A 4x1 texture of 4-bit indices 0, 1, 2, 3 at (640, 0), and a CLUT at
	// (0, 480) where index 0 is transparent black.
	clut := [4]uint16{0x0000, 0x001f, 0x03e0, 0x7c00}
	m.Bus.Write32(gp0, 0xa0000000)
	m.Bus.Write32(gp0, 0<<16|640)
	m.Bus.Write32(gp0, 1<<16|2)
	m.Bus.Write32(gp0, 0x00003210)
	m.Bus.Write32(gp0, 0xa0000000)
	m.Bus.Write32(gp0, 480<<16|0)
	m.Bus.Write32(gp0, 1<<16|4)
	m.Bus.Write32(gp0, uint32(clut[1])<<16|uint32(clut[0]))
	m.Bus.Write32(gp0, uint32(clut[3])<<16|uint32(clut[2]))

	m.Bus.Write32(gp0, 0xe100000a)           // texture page (640, 0), 4-bit
	m.Bus.Write32(gp0, 0x65000000)           // raw textured rectangle
	m.Bus.Write32(gp0, 100<<16|50)           // at (50, 100)
	m.Bus.Write32(gp0, uint32(480<<6)<<16|0) // CLUT (0, 480), texcoord (0, 0)
	m.Bus.Write32(gp0, 1<<16|4)              // 4x1

	got := [4]uint16{m.pixel(50, 100), m.pixel(51, 100), m.pixel(52, 100), m.pixel(53, 100)}
	assert.Equal(t, [4]uint16{0, clut[1], clut[2], clut[3]}, got, "index 0 is transparent and left undrawn")
}

func TestDrawLine(t *testing.T) {
	m := drawing(t)
	m.Bus.Write32(gp0, 0x4000ff00) // green line
	m.Bus.Write32(gp0, 5<<16|0)
	m.Bus.Write32(gp0, 5<<16|4)

	var got [5]uint16
	for x := range got {
		got[x] = m.pixel(x, 5)
	}
	assert.Equal(t, [5]uint16{0x03e0, 0x03e0, 0x03e0, 0x03e0, 0}, got, "the far end is not drawn")
}

func TestDrawSemiTransparent(t *testing.T) {
	m := drawing(t)
	m.Bus.Write32(gp0, 0x02f8f8f8) // fill white, 16x1 at (0, 0)
	m.Bus.Write32(gp0, 0)
	m.Bus.Write32(gp0, 1<<16|16)
	m.Bus.Write32(gp0, 0xe1000000) // blend mode 0: half background, half foreground
	m.Bus.Write32(gp0, 0x62000000) // semi-transparent black rectangle
	m.Bus.Write32(gp0, 0)
	m.Bus.Write32(gp0, 1<<16|1)
	assert.Equal(t, uint16(0x3def), m.pixel(0, 0), "half of white")
}

func TestDisplay(t *testing.T) {
	m := drawing(t)
	const gp1 = 0x1f801814
	m.Bus.Write32(gp0, 0x02ff0000) // fill blue, 16x1 at (320, 0)
	m.Bus.Write32(gp0, 320)
	m.Bus.Write32(gp0, 1<<16|16)
	m.Bus.Write32(gp1, 0x03000000)            // display on
	m.Bus.Write32(gp1, 0x05000000|320)        // display area at (320, 0)
	m.Bus.Write32(gp1, 0x07000000|256<<10|16) // 240 lines
	m.Bus.Write32(gp1, 0x08000001)            // 320 wide

	img := m.Display()
	assert.Equal(t, [2]int{320, 240}, [2]int{img.Rect.Dx(), img.Rect.Dy()})
	assert.Equal(t, color.RGBA{B: 0xf8, A: 0xff}, img.RGBAAt(0, 0))
	assert.Equal(t, color.RGBA{A: 0xff}, img.RGBAAt(16, 0))
}

// TestModulateExact checks modulate's integer arithmetic against psxe's
// float version for every texel channel and modulating value.
func TestModulateExact(t *testing.T) {
	ref := func(tc, m uint32) uint32 {
		v := float32(float32(float32(tc<<3)*float32(m)) / 128)
		v = min(max(v, 0), 255)
		return uint32(math.Round(float64(v)))
	}
	for tc := range uint32(32) {
		for m := range uint32(256) {
			// The same value in all three channels.
			texel := uint16(tc | tc<<5 | tc<<10)
			mod := m | m<<8 | m<<16
			want := bgr555(rgb(ref(tc, m), ref(tc, m), ref(tc, m)))
			if got := modulate(texel, mod); got != want {
				t.Fatalf("texel %d, mod %d: got %#04x, want %#04x", tc, m, got, want)
			}
		}
	}
}

// TestFloatShortcuts checks clamp255 and floor32 against the math functions
// they stand in for, across a spread of float32 values: every half, and a
// stride through the bit patterns of both signs.
func TestFloatShortcuts(t *testing.T) {
	check := func(v float32) {
		w := min(max(v, 0), 255)
		if got, want := clamp255(v), uint32(math.Round(float64(w))); got != want {
			t.Fatalf("clamp255(%v): got %d, want %d", v, got, want)
		}
		got, want := floor32(v), float32(math.Floor(float64(v)))
		if math.Float32bits(got) != math.Float32bits(want) && !(got != got && want != want) {
			t.Fatalf("floor32(%v): got %v, want %v", v, got, want)
		}
	}
	for i := -1000; i <= 1000; i++ {
		check(float32(i) / 2)
	}
	for bits := uint32(0); bits < 0x7f800000; bits += 9973 {
		check(math.Float32frombits(bits))
		check(math.Float32frombits(bits | 0x80000000))
	}
	for _, v := range []float32{0, float32(math.Copysign(0, -1)), 1 << 23, -(1 << 23), 1<<23 - 0.5, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		check(v)
	}
}

// TestTexelBilinear compares texelBilinear with the version it was before
// its fetches were split up, which fetched each texel with texel, over
// random VRAM, texture windows, depths and coordinates.
func TestTexelBilinear(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	g := newGPU(nil, nil)
	for i := range g.vram {
		g.vram[i] = uint16(r.Uint32())
		if r.IntN(4) == 0 {
			g.vram[i] = 0 // transparent texels, so the early return is taken
		}
	}
	for range 200_000 {
		g.texwMX, g.texwMY = uint32(r.IntN(32))<<3, uint32(r.IntN(32))<<3
		g.texwOX, g.texwOY = uint32(r.IntN(32))<<3, uint32(r.IntN(32))<<3
		if r.IntN(2) == 0 {
			g.texwMX, g.texwMY = 0, 0
		}
		tx := float32(r.Float64()*300 - 20)
		ty := float32(r.Float64()*300 - 20)
		tpx, tpy := uint32(r.IntN(16))<<6, uint32(r.IntN(2))<<8
		clutx, cluty := uint32(r.IntN(64))<<4, uint32(r.IntN(512))
		depth := uint32(r.IntN(4))
		got := g.texelBilinear(tx, ty, tpx, tpy, clutx, cluty, depth)
		want := texelBilinearRef(g, tx, ty, tpx, tpy, clutx, cluty, depth)
		if got != want {
			t.Fatalf("(%v, %v), page (%d, %d), clut (%d, %d), depth %d: got %#04x, want %#04x",
				tx, ty, tpx, tpy, clutx, cluty, depth, got, want)
		}
	}
}

func texelBilinearRef(g *gpu, tx, ty float32, tpx, tpy, clutx, cluty, depth uint32) uint16 {
	txf := float32(math.Floor(float64(tx)))
	tyf := float32(math.Floor(float64(ty)))
	txc := float32(txf + 1)
	tyc := float32(tyf + 1)

	fetch := func(x, y float32) uint16 {
		return g.texel(uint16(int32(x)), uint16(int32(y)), tpx, tpy, clutx, cluty, depth)
	}
	s0 := fetch(txf, tyf)
	if s0 == 0 {
		return 0
	}
	s1 := fetch(txc, tyf)
	s2 := fetch(txf, tyc)
	s3 := fetch(txc, tyc)

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
