package emu

import (
	"image"
	"math/rand/v2"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestRGBA15 checks that rgba15, which may use SIMD, converts every length
// of row, from every alignment, as rgba15Scalar does.
func TestRGBA15(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	src := make([]uint16, 64)
	for i := range src {
		src[i] = uint16(r.Uint32())
	}
	src[0], src[1] = 0x0000, 0xffff
	for off := range 8 {
		for n := range len(src) - off {
			got, want := make([]byte, 4*n), make([]byte, 4*n)
			rgba15(got, src[off:off+n])
			rgba15Scalar(want, src[off:off+n])
			assert.Equal(t, want, got)
		}
	}
}

// TestRGB24 checks that rgb24, which may use SIMD, converts every length of
// row, from every alignment, as rgb24Scalar does, as many pixels as fit.
func TestRGB24(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	src := make([]uint16, 64)
	for i := range src {
		src[i] = uint16(r.Uint32())
	}
	for off := range 8 {
		for n := range len(src) - off {
			for _, pixels := range []int{2 * n / 3, n} {
				got, want := make([]byte, 4*pixels), make([]byte, 4*pixels)
				gotN, wantN := rgb24(got, src[off:off+n]), rgb24Scalar(want, src[off:off+n])
				assert.Equal(t, wantN, gotN)
				assert.Equal(t, want, got)
			}
		}
	}
}

// TestDisplay24Wrap checks the 24-bit display against VRAM read as bytes, for
// display areas from every column near the right of VRAM, which wrap.
func TestDisplay24Wrap(t *testing.T) {
	m := newTestMachine(t)
	r := rand.New(rand.NewPCG(5, 6))
	for i := range m.gpu.vram {
		m.gpu.vram[i] = uint16(r.Uint32())
	}
	const gp1 = 0x1f801814
	m.Bus.Write32(gp1, 0x03000000)            // display on
	m.Bus.Write32(gp1, 0x07000000|256<<10|16) // 240 lines
	m.Bus.Write32(gp1, 0x08000011)            // 320 wide, 24-bit
	var img *image.RGBA
	for _, x := range []uint32{0, 1, 2, 3, 541, 542, 543, 544, 545, 1018, 1019, 1020, 1021, 1022, 1023} {
		m.Bus.Write32(gp1, 0x05000000|x|100<<10) // display area at (x, 100)
		img = m.DisplayInto(img)
		for y := range img.Rect.Dy() {
			row := m.gpu.vram[(100+y)*vramWidth:][:vramWidth]
			for px := range img.Rect.Dx() {
				var want [4]byte
				for i := range 3 {
					b := (2*int(x) + 3*px + i) % (2 * vramWidth)
					want[i] = byte(row[b/2] >> (8 * (b % 2)))
				}
				want[3] = 0xff
				if got := [4]byte(img.Pix[y*img.Stride+4*px:]); got != want {
					t.Fatalf("area at %d: pixel (%d, %d) is %v, want %v", x, px, y, got, want)
				}
			}
		}
	}
}

func TestRGBA15Pixel(t *testing.T) {
	// Red, green and blue at full and the lowest bit, and the mask bit,
	// which is not shown.
	for p, want := range map[uint32]uint32{
		0x001f: 0xff0000f8,
		0x03e0: 0xff00f800,
		0x7c00: 0xfff80000,
		0x0421: 0xff080808,
		0x8000: 0xff000000,
	} {
		assert.Equal(t, want, rgba15Pixel(p))
	}
}

// BenchmarkDisplay converts a 320x240 display, as a frontend does every
// frame.
func BenchmarkDisplay(b *testing.B) {
	m := newTestMachine(b)
	const gp1 = 0x1f801814
	m.Bus.Write32(gp1, 0x03000000)            // display on
	m.Bus.Write32(gp1, 0x07000000|256<<10|16) // 240 lines
	m.Bus.Write32(gp1, 0x08000001)            // 320 wide
	img := m.DisplayInto(nil)
	if img.Rect.Dx() != 320 || img.Rect.Dy() != 240 {
		b.Fatalf("display is %v, not 320x240", img.Rect)
	}
	for b.Loop() {
		img = m.DisplayInto(img)
	}
}

// TestFill16 checks that fill16 fills every length of row, from every
// alignment, and nothing either side of it.
func TestFill16(t *testing.T) {
	for off := range 8 {
		for n := range 64 {
			got := make([]uint16, off+n+8)
			want := make([]uint16, off+n+8)
			fill16(got[off:off+n], 0x7c1f)
			fill16Scalar(want[off:off+n], 0x7c1f)
			assert.Equal(t, want, got)
		}
	}
}

// BenchmarkFill16 fills a 320-pixel row, as a fill or flat span does.
func BenchmarkFill16(b *testing.B) {
	row := make([]uint16, 320)
	for b.Loop() {
		fill16(row, 0x7c1f)
	}
}

func BenchmarkFill16Scalar(b *testing.B) {
	row := make([]uint16, 320)
	for b.Loop() {
		fill16Scalar(row, 0x7c1f)
	}
}
