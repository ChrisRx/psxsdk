// Package mdec encodes images for the PlayStation's motion decoder, the
// MDEC, and reads and writes a movie of them.
//
// The MDEC takes 16x16 macroblocks, each a Cr, a Cb and four Y blocks of 8x8
// samples, as run-length codes of their quantized DCT coefficients, and
// turns them back into 15- or 24-bit pixels. A program streams the codes in
// through DMA channel 0 behind a decode command, and the pixels out through
// channel 1, and uploads them to VRAM a column of macroblocks at a time.
// Sony's movies compress the codes further with a variable-length code that
// the CPU undoes first; the codes here are left as the MDEC takes them,
// which costs space but no time.
package mdec

import (
	"image"
	"image/color"
	"math"
)

// EOB is the code that ends a block, and pads a decode command's input to a
// whole number of DMA blocks.
const EOB = 0xfe00

// Quant is the quantization table, in zigzag order: the MPEG-1 intra matrix,
// which Sony's libraries load into the MDEC.
var Quant = [64]uint8{
	2, 16, 19, 22, 26, 27, 29, 34,
	16, 16, 22, 24, 27, 29, 34, 37,
	19, 22, 26, 27, 29, 34, 34, 38,
	22, 22, 26, 27, 29, 34, 37, 40,
	22, 26, 27, 29, 32, 35, 40, 48,
	26, 27, 29, 32, 35, 40, 48, 58,
	26, 27, 29, 34, 38, 46, 56, 69,
	27, 29, 35, 38, 46, 56, 69, 83,
}

// Scale is the scale table the MDEC's inverse DCT multiplies by: the DCT's
// basis functions, a row to a frequency, in 1.15 fixed point.
var Scale = [64]int16{
	0x5a82, 0x5a82, 0x5a82, 0x5a82, 0x5a82, 0x5a82, 0x5a82, 0x5a82,
	0x7d8a, 0x6a6d, 0x471c, 0x18f8, -0x18f8, -0x471c, -0x6a6d, -0x7d8a,
	0x7641, 0x30fb, -0x30fb, -0x7641, -0x7641, -0x30fb, 0x30fb, 0x7641,
	0x6a6d, -0x18f8, -0x7d8a, -0x471c, 0x471c, 0x7d8a, 0x18f8, -0x6a6d,
	0x5a82, -0x5a82, -0x5a82, 0x5a82, 0x5a82, -0x5a82, -0x5a82, 0x5a82,
	0x471c, -0x7d8a, 0x18f8, 0x6a6d, -0x6a6d, -0x18f8, 0x7d8a, -0x471c,
	0x30fb, -0x7641, 0x7641, -0x30fb, -0x30fb, 0x7641, -0x7641, 0x30fb,
	0x18f8, -0x471c, 0x6a6d, -0x7d8a, 0x7d8a, -0x6a6d, 0x471c, -0x18f8,
}

// zigzag maps a coefficient's place in the zigzag order to its place in the
// block, row by row: the order the MDEC stores the codes in.
var zigzag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// basis is the DCT's basis, a row to a frequency, as Scale holds it.
var basis = func() (c [8][8]float64) {
	for u := range 8 {
		for x := range 8 {
			c[u][x] = math.Cos(float64((2*x+1)*u) * math.Pi / 16)
			if u == 0 {
				c[u][x] = math.Sqrt2 / 2
			}
		}
	}
	return c
}()

// Encode returns the run-length codes for an image: its macroblocks column
// by column, left to right, each top to bottom, which is the order a
// program uploads them in. The image is extended to a multiple of 16
// pixels each way by repeating its last column and row.
//
// qscale, from 1 to 63, scales the quantization table for the coefficients
// other than DC: higher compresses more and looks worse.
func Encode(img image.Image, qscale int) []uint16 {
	qscale = min(max(qscale, 1), 63)
	b := img.Bounds()
	w, h := Size(b.Dx(), b.Dy())

	// Y, Cb and Cr for every pixel, around zero as the MDEC works.
	ys := make([]float64, w*h)
	cbs := make([]float64, w*h)
	crs := make([]float64, w*h)
	for y := range h {
		for x := range w {
			px := min(x, b.Dx()-1) + b.Min.X
			py := min(y, b.Dy()-1) + b.Min.Y
			c := color.NRGBAModel.Convert(img.At(px, py)).(color.NRGBA)
			r, g, bl := float64(c.R)-128, float64(c.G)-128, float64(c.B)-128
			l := 0.299*r + 0.587*g + 0.114*bl
			ys[y*w+x] = l
			cbs[y*w+x] = (bl - l) / 1.772
			crs[y*w+x] = (r - l) / 1.402
		}
	}

	var codes []uint16
	var blk [64]float64
	for mx := 0; mx < w; mx += 16 {
		for my := 0; my < h; my += 16 {
			// The chroma blocks are the color difference averaged over
			// each 2x2 pixels.
			for _, plane := range [][]float64{crs, cbs} {
				for y := range 8 {
					for x := range 8 {
						i := (my+2*y)*w + mx + 2*x
						blk[y*8+x] = (plane[i] + plane[i+1] + plane[i+w] + plane[i+w+1]) / 4
					}
				}
				codes = encodeBlock(codes, &blk, qscale)
			}
			for _, o := range [4][2]int{{0, 0}, {8, 0}, {0, 8}, {8, 8}} {
				for y := range 8 {
					for x := range 8 {
						blk[y*8+x] = ys[(my+o[1]+y)*w+mx+o[0]+x]
					}
				}
				codes = encodeBlock(codes, &blk, qscale)
			}
		}
	}
	return codes
}

// Size returns the size of the picture the MDEC decodes for an image of
// w by h: each rounded up to a multiple of 16.
func Size(w, h int) (int, int) {
	return (w + 15) &^ 15, (h + 15) &^ 15
}

// encodeBlock appends the codes for one block of samples: its DCT,
// quantized as the MDEC undoes it, in zigzag order, each nonzero
// coefficient with the run of zeros before it, ending with EOB.
func encodeBlock(codes []uint16, blk *[64]float64, qscale int) []uint16 {
	// The MDEC's inverse is basisᵀ·F·basis/4, and basis·basisᵀ = 4I, so
	// the forward transform is basis·f·basisᵀ/4.
	var tmp, f [64]float64
	for u := range 8 {
		for x := range 8 {
			s := 0.0
			for y := range 8 {
				s += basis[u][y] * blk[y*8+x]
			}
			tmp[u*8+x] = s
		}
	}
	for u := range 8 {
		for v := range 8 {
			s := 0.0
			for x := range 8 {
				s += tmp[u*8+x] * basis[v][x]
			}
			f[u*8+v] = s / 4
		}
	}

	dc := clamp10(math.Round(f[0] / float64(Quant[0])))
	codes = append(codes, uint16(qscale)<<10|uint16(dc)&0x3ff)
	run := 0
	for k := 1; k < 64; k++ {
		ac := clamp10(math.Round(f[zigzag[k]] * 8 / float64(int(Quant[k])*qscale)))
		if ac == 0 {
			run++
			continue
		}
		codes = append(codes, uint16(run)<<10|uint16(ac)&0x3ff)
		run = 0
	}
	return append(codes, EOB)
}

// clamp10 limits a coefficient to what ten signed bits hold.
func clamp10(v float64) int { return int(min(max(v, -512), 511)) }
