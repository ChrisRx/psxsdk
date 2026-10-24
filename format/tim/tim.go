// Package tim reads and writes TIM files, the PlayStation's image format:
// pixels laid out as video memory holds them, with the palette (the CLUT)
// that 4- and 8-bit pixels index, and where in video memory each goes.
//
// A file is little-endian words:
//
//	000h  ID, 0x10
//	004h  flags: the pixel mode in bits 0-2, bit 3 set when a CLUT follows
//	      (Sony's manual numbers it bit 4 counting from 1)
//	008h  the CLUT block, when there is one, then the pixel block, each:
//	        the block's length in bytes, its own 12 bytes of header included
//	        x, y in video memory (16 bits each)
//	        width in 16-bit units, height (16 bits each)
//	        the 16-bit units, a row at a time
//
// A 16-bit color is 5 bits each of red, green and blue from the low bit,
// and STP in bit 15. The GPU skips a texel of 0x0000 entirely, which is how
// sprites and textures are transparent; STP on any other color makes the
// texel semitransparent when the primitive asks for it, so 0x8000 is a
// black that is drawn.
package tim

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
)

// ID is the first word of a TIM file.
const ID = 0x10

// Mode is how a TIM's pixels are stored.
type Mode int

const (
	Mode4  Mode = 0 // 4-bit indices into a 16-color CLUT, four to a unit
	Mode8  Mode = 1 // 8-bit indices into a 256-color CLUT, two to a unit
	Mode16 Mode = 2 // 16-bit colors, one to a unit
	Mode24 Mode = 3 // 24-bit colors, two in three units, for display only
)

// Bits returns the bits per pixel.
func (m Mode) Bits() int { return [...]int{4, 8, 16, 24}[m] }

// Indexed reports whether the pixels index a CLUT.
func (m Mode) Indexed() bool { return m == Mode4 || m == Mode8 }

// Colors returns how many colors one CLUT holds, 0 for a direct mode.
func (m Mode) Colors() int {
	switch m {
	case Mode4:
		return 16
	case Mode8:
		return 256
	}
	return 0
}

func (m Mode) String() string {
	if m < 0 || m > Mode24 {
		return fmt.Sprintf("Mode(%d)", int(m))
	}
	return fmt.Sprintf("%d-bit", m.Bits())
}

// A Block is a rectangle of video memory and the 16-bit units that fill it,
// a row at a time. W is in units, not pixels.
type Block struct {
	X, Y, W, H int
	Data       []uint16
}

// A TIM is one image: its pixels and, for an indexed mode, its CLUT, which
// may hold several palettes, one to a row.
type TIM struct {
	Mode   Mode
	CLUT   *Block
	Pixels Block
}

// Width returns the image's width in pixels.
func (t *TIM) Width() int {
	switch t.Mode {
	case Mode4:
		return t.Pixels.W * 4
	case Mode8:
		return t.Pixels.W * 2
	case Mode24:
		return t.Pixels.W * 2 / 3
	}
	return t.Pixels.W
}

// Height returns the image's height in pixels.
func (t *TIM) Height() int { return t.Pixels.H }

// TPage returns the texture page a sprite or a polygon names the image by,
// as GetTPage makes it: the mode, and the 64x256 page the pixels start in.
// The blending rate, bits 5-6, is left 0.
func (t *TIM) TPage() uint16 {
	return uint16(t.Mode&3)<<7 | uint16(t.Pixels.Y/256)<<4 | uint16(t.Pixels.X/64)
}

// CBA returns the CLUT's position as a primitive names it: x/16 in bits
// 0-5 and y in bits 6-14, or 0 when there is no CLUT.
func (t *TIM) CBA() uint16 {
	if t.CLUT == nil {
		return 0
	}
	return uint16(t.CLUT.Y)<<6 | uint16(t.CLUT.X/16)
}

// UV returns where the pixels start inside their texture page, in texels,
// which is what a primitive's U and V are relative to.
func (t *TIM) UV() (u, v int) {
	return (t.Pixels.X % 64) * 16 / t.Mode.Bits(), t.Pixels.Y % 256
}

// Decode reads a TIM from the bytes of a file. A file that holds more than
// one image, as some games' archives do, is read up to the end of the first.
func Decode(b []byte) (*TIM, error) {
	t, _, err := decode(b)
	return t, err
}

// Size returns how many bytes of b the TIM at its start takes.
func Size(b []byte) (int, error) {
	_, n, err := decode(b)
	return n, err
}

func decode(b []byte) (*TIM, int, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("tim: too short for a header")
	}
	if id := binary.LittleEndian.Uint32(b); id != ID {
		return nil, 0, fmt.Errorf("tim: ID is %#x, not %#x", id, ID)
	}
	flags := binary.LittleEndian.Uint32(b[4:])
	t := &TIM{Mode: Mode(flags & 7)}
	if t.Mode > Mode24 {
		return nil, 0, fmt.Errorf("tim: unknown pixel mode %d", t.Mode)
	}
	n := 8
	if flags&8 != 0 {
		c, m, err := decodeBlock(b[n:])
		if err != nil {
			return nil, 0, fmt.Errorf("tim: CLUT: %w", err)
		}
		t.CLUT = &c
		n += m
	}
	p, m, err := decodeBlock(b[n:])
	if err != nil {
		return nil, 0, fmt.Errorf("tim: pixels: %w", err)
	}
	t.Pixels = p
	return t, n + m, nil
}

func decodeBlock(b []byte) (Block, int, error) {
	if len(b) < 12 {
		return Block{}, 0, errors.New("too short for a block header")
	}
	n := int(binary.LittleEndian.Uint32(b))
	blk := Block{
		X: int(binary.LittleEndian.Uint16(b[4:])),
		Y: int(binary.LittleEndian.Uint16(b[6:])),
		W: int(binary.LittleEndian.Uint16(b[8:])),
		H: int(binary.LittleEndian.Uint16(b[10:])),
	}
	units := blk.W * blk.H
	if 12+2*units > len(b) {
		return Block{}, 0, fmt.Errorf("%dx%d units at (%d,%d) run past the end of the file", blk.W, blk.H, blk.X, blk.Y)
	}
	if n < 12+2*units {
		// Some tools wrote a length that leaves out the header; trust
		// the rectangle, which is what the library goes by.
		n = 12 + 2*units
	}
	if n > len(b) {
		n = len(b)
	}
	blk.Data = make([]uint16, units)
	for i := range blk.Data {
		blk.Data[i] = binary.LittleEndian.Uint16(b[12+2*i:])
	}
	return blk, n, nil
}

// Bytes returns the TIM as a file.
func (t *TIM) Bytes() []byte {
	flags := uint32(t.Mode)
	if t.CLUT != nil {
		flags |= 8
	}
	b := binary.LittleEndian.AppendUint32(nil, ID)
	b = binary.LittleEndian.AppendUint32(b, flags)
	if t.CLUT != nil {
		b = t.CLUT.append(b)
	}
	return t.Pixels.append(b)
}

func (blk *Block) append(b []byte) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(12+2*len(blk.Data)))
	for _, v := range [...]int{blk.X, blk.Y, blk.W, blk.H} {
		b = binary.LittleEndian.AppendUint16(b, uint16(v))
	}
	for _, v := range blk.Data {
		b = binary.LittleEndian.AppendUint16(b, v)
	}
	return b
}

// Index returns the CLUT index of the pixel at (x, y) of an indexed image.
func (t *TIM) Index(x, y int) int {
	unit := t.Pixels.Data[y*t.Pixels.W+x*t.Mode.Bits()/16]
	switch t.Mode {
	case Mode4:
		return int(unit>>(4*(x%4))) & 15
	case Mode8:
		return int(unit>>(8*(x%2))) & 255
	}
	panic("tim: Index of a direct color image")
}

// Color returns the 16-bit color of the pixel at (x, y), looked up in the
// palette on row pal of the CLUT for an indexed image. A 24-bit image has
// none, and its color is narrowed to 15 bits with STP clear.
func (t *TIM) Color(x, y, pal int) uint16 {
	switch t.Mode {
	case Mode16:
		return t.Pixels.Data[y*t.Pixels.W+x]
	case Mode24:
		row := t.Pixels.Data[y*t.Pixels.W:]
		i := x * 3
		byteAt := func(k int) uint8 { return uint8(row[k/2] >> (8 * (k % 2))) }
		r, g, b := byteAt(i), byteAt(i+1), byteAt(i+2)
		return uint16(r>>3) | uint16(g>>3)<<5 | uint16(b>>3)<<10
	}
	return t.CLUT.Data[pal*t.CLUT.W+t.Index(x, y)]
}

// Image returns the image as Go sees it, with the palette on row pal of the
// CLUT for an indexed one: an *image.Paletted for an indexed image, an
// *image.NRGBA otherwise. A color of 0x0000 is transparent, as the GPU
// draws it.
func (t *TIM) Image(pal int) image.Image {
	r := image.Rect(0, 0, t.Width(), t.Height())
	if t.Mode.Indexed() {
		p := make(color.Palette, t.Mode.Colors())
		for i := range p {
			c := uint16(0)
			if j := pal*t.CLUT.W + i; i < t.CLUT.W && j < len(t.CLUT.Data) {
				c = t.CLUT.Data[j]
			}
			p[i] = RGBA(c)
		}
		img := image.NewPaletted(r, p)
		for y := range t.Height() {
			for x := range t.Width() {
				img.Pix[y*img.Stride+x] = uint8(t.Index(x, y))
			}
		}
		return img
	}
	img := image.NewNRGBA(r)
	for y := range t.Height() {
		for x := range t.Width() {
			img.Set(x, y, RGBA(t.Color(x, y, 0)))
		}
	}
	return img
}

// RGBA returns the color a 16-bit PlayStation color is, widening each
// 5-bit component to 8 bits. 0x0000 is transparent.
func RGBA(c uint16) color.NRGBA {
	if c == 0 {
		return color.NRGBA{}
	}
	w := func(v uint16) uint8 { v &= 31; return uint8(v<<3 | v>>2) }
	return color.NRGBA{R: w(c), G: w(c >> 5), B: w(c >> 10), A: 255}
}
