// Package bmp decodes Windows BMP files, which is how most Net Yaroze art
// was kept, so that the asset tools can read it with image.Decode. A file of
// 1, 4 or 8 bits a pixel decodes to an *image.Paletted, keeping its palette
// in order, since a program may name colors by their index; 16, 24 and
// 32-bit files decode to an *image.NRGBA. Run-length compressed files are
// not supported.
package bmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math/bits"
)

func init() {
	image.RegisterFormat("bmp", "BM", Decode, DecodeConfig)
}

type header struct {
	offset      int
	headerSize  int
	width       int
	height      int // negative for rows stored top first
	bpp         int
	compression uint32
	colors      int
	masks       [4]uint32 // red, green, blue, alpha, for bit fields
}

const (
	biRGB       = 0
	biBitfields = 3
)

func readHeader(b []byte) (header, error) {
	if len(b) < 26 || string(b[:2]) != "BM" {
		return header{}, errors.New("bmp: not a BMP file")
	}
	h := header{
		offset:     int(binary.LittleEndian.Uint32(b[10:])),
		headerSize: int(binary.LittleEndian.Uint32(b[14:])),
	}
	if h.headerSize == 12 {
		// OS/2's BITMAPCOREHEADER, with 16-bit sizes.
		h.width = int(int16(binary.LittleEndian.Uint16(b[18:])))
		h.height = int(int16(binary.LittleEndian.Uint16(b[20:])))
		h.bpp = int(binary.LittleEndian.Uint16(b[24:]))
		return h, nil
	}
	if h.headerSize < 40 || len(b) < 14+40 {
		return header{}, fmt.Errorf("bmp: header of %d bytes", h.headerSize)
	}
	h.width = int(int32(binary.LittleEndian.Uint32(b[18:])))
	h.height = int(int32(binary.LittleEndian.Uint32(b[22:])))
	h.bpp = int(binary.LittleEndian.Uint16(b[28:]))
	h.compression = binary.LittleEndian.Uint32(b[30:])
	h.colors = int(binary.LittleEndian.Uint32(b[46:]))
	switch {
	case h.compression == biBitfields && h.headerSize >= 56 && len(b) >= 14+56:
		for i := range 4 {
			h.masks[i] = binary.LittleEndian.Uint32(b[54+4*i:])
		}
	case h.compression == biBitfields && len(b) >= 14+40+12:
		// The masks follow a 40-byte header.
		for i := range 3 {
			h.masks[i] = binary.LittleEndian.Uint32(b[54+4*i:])
		}
	case h.bpp == 16:
		h.masks = [4]uint32{0x7c00, 0x03e0, 0x001f, 0}
	case h.bpp == 32:
		h.masks = [4]uint32{0xff0000, 0xff00, 0xff, 0}
	}
	if h.compression != biRGB && h.compression != biBitfields {
		return header{}, fmt.Errorf("bmp: compression %d is not supported", h.compression)
	}
	if h.width <= 0 || h.height == 0 {
		return header{}, fmt.Errorf("bmp: %dx%d", h.width, h.height)
	}
	return h, nil
}

func (h header) palette(b []byte) (color.Palette, error) {
	n := h.colors
	if n == 0 {
		n = 1 << h.bpp
	}
	entry := 4
	if h.headerSize == 12 {
		entry = 3
	}
	start := 14 + h.headerSize
	if h.compression == biBitfields && h.headerSize == 40 {
		start += 12
	}
	if start+n*entry > len(b) {
		return nil, errors.New("bmp: palette runs past the end of the file")
	}
	p := make(color.Palette, n)
	for i := range n {
		e := b[start+i*entry:]
		p[i] = color.NRGBA{R: e[2], G: e[1], B: e[0], A: 255}
	}
	return p, nil
}

// DecodeConfig returns a BMP file's size and color model.
func DecodeConfig(r io.Reader) (image.Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return image.Config{}, err
	}
	h, err := readHeader(b)
	if err != nil {
		return image.Config{}, err
	}
	c := image.Config{Width: h.width, Height: abs(h.height), ColorModel: color.NRGBAModel}
	if h.bpp <= 8 {
		p, err := h.palette(b)
		if err != nil {
			return image.Config{}, err
		}
		c.ColorModel = p
	}
	return c, nil
}

// Decode reads a BMP file.
func Decode(r io.Reader) (image.Image, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	h, err := readHeader(b)
	if err != nil {
		return nil, err
	}
	w, ht := h.width, abs(h.height)
	stride := (w*h.bpp + 31) / 32 * 4
	if h.offset+stride*ht > len(b) {
		return nil, errors.New("bmp: pixels run past the end of the file")
	}
	row := func(y int) []byte {
		if h.height > 0 {
			y = ht - 1 - y
		}
		return b[h.offset+y*stride:]
	}
	switch h.bpp {
	case 1, 2, 4, 8:
		p, err := h.palette(b)
		if err != nil {
			return nil, err
		}
		img := image.NewPaletted(image.Rect(0, 0, w, ht), p)
		mask := byte(1<<h.bpp - 1)
		for y := range ht {
			src := row(y)
			for x := range w {
				bit := x * h.bpp
				img.Pix[y*img.Stride+x] = src[bit/8] >> (8 - h.bpp - bit%8) & mask
			}
		}
		return img, nil
	case 16, 24, 32:
		img := image.NewNRGBA(image.Rect(0, 0, w, ht))
		for y := range ht {
			src := row(y)
			for x := range w {
				var c color.NRGBA
				switch h.bpp {
				case 24:
					e := src[3*x:]
					c = color.NRGBA{R: e[2], G: e[1], B: e[0], A: 255}
				case 16:
					c = h.field(uint32(binary.LittleEndian.Uint16(src[2*x:])))
				case 32:
					c = h.field(binary.LittleEndian.Uint32(src[4*x:]))
				}
				i := y*img.Stride + 4*x
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
			}
		}
		return img, nil
	}
	return nil, fmt.Errorf("bmp: %d bits a pixel is not supported", h.bpp)
}

// field takes a pixel apart by the header's masks, widening each component
// to 8 bits.
func (h header) field(v uint32) color.NRGBA {
	get := func(mask uint32) uint8 {
		if mask == 0 {
			return 255
		}
		shift := bits.TrailingZeros32(mask)
		width := bits.OnesCount32(mask)
		c := (v & mask) >> shift
		// Widen by repeating the high bits into the low ones.
		out := uint32(0)
		for got := 0; got < 8; got += width {
			out = out<<width | c
		}
		return uint8(out >> (((8 + width - 1) / width * width) - 8))
	}
	c := color.NRGBA{R: get(h.masks[0]), G: get(h.masks[1]), B: get(h.masks[2]), A: get(h.masks[3])}
	return c
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
