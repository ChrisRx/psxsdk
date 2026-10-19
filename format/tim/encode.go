package tim

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"slices"
)

// Black is what Encode makes of an opaque black pixel. The GPU skips 0x0000,
// so black has to be either that, and not drawn, or 0x8000, STP set, which
// is drawn black.
type Black int

const (
	BlackOpaque      Black = iota // 0x8000, drawn: what a modern image means by black
	BlackTransparent              // 0x0000, skipped: the Net Yaroze convention for sprites
)

// Options says how Encode lays an image out.
type Options struct {
	// Mode is the pixel mode, or Auto to choose: an indexed picture with
	// at most 16 colours is 4-bit, one with at most 256 is 8-bit, and any
	// other picture with that few colours is the same; the rest are 16-bit.
	Mode Mode

	// X, Y is where the pixels go in video memory. A texture page is 64
	// units wide, and an image used as a texture must not cross one.
	X, Y int

	// CLUTX, CLUTY is where the CLUT goes, for an indexed mode. X must be a
	// multiple of 16.
	CLUTX, CLUTY int

	// Black is what an opaque black pixel becomes. A pixel whose alpha is
	// under half is transparent, 0x0000, whatever this says.
	Black Black

	// Semi sets STP on every colour that is drawn, so that the whole image
	// blends when a sprite or a polygon turns semitransparency on.
	Semi bool
}

// Auto is the Mode that asks Encode to choose.
const Auto Mode = -1

// Encode makes a TIM of img. An *image.Paletted keeps its palette and its
// indices, so that an image drawn for a palette keeps the order the program
// may depend on; any other image is reduced to the mode's colours by median
// cut when it has more. Widths are padded to a whole number of 16-bit
// units, four pixels for 4-bit and two for 8-bit, with index 0.
func Encode(img image.Image, o Options) (*TIM, error) {
	if o.CLUTX%16 != 0 {
		return nil, fmt.Errorf("tim: the CLUT's x, %d, is not a multiple of 16", o.CLUTX)
	}
	if o.X < 0 || o.X >= 1024 || o.Y < 0 || o.Y >= 512 || o.CLUTX < 0 || o.CLUTX >= 1024 || o.CLUTY < 0 || o.CLUTY >= 512 {
		return nil, errors.New("tim: a position is outside video memory, 1024x512")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, errors.New("tim: the image is empty")
	}

	var idx []uint8     // indices, row by row, for an indexed result
	var pal []uint16    // the palette they index
	var direct []uint16 // colours, row by row, for a 16-bit result

	if p, ok := img.(*image.Paletted); ok {
		used := 0
		for y := range h {
			for x := range w {
				if i := int(p.ColorIndexAt(b.Min.X+x, b.Min.Y+y)); i+1 > used {
					used = i + 1
				}
			}
		}
		mode := o.Mode
		if mode == Auto {
			mode = Mode8
			if used <= 16 {
				mode = Mode4
			}
		}
		if mode.Indexed() {
			if used > mode.Colors() {
				return nil, fmt.Errorf("tim: the image uses palette index %d, more than %s holds", used-1, mode)
			}
			for _, c := range p.Palette {
				pal = append(pal, o.color(c))
			}
			idx = make([]uint8, w*h)
			for y := range h {
				for x := range w {
					idx[y*w+x] = p.ColorIndexAt(b.Min.X+x, b.Min.Y+y)
				}
			}
			return o.indexed(mode, w, h, idx, pal)
		}
		o.Mode = mode
	}

	direct = make([]uint16, w*h)
	for y := range h {
		for x := range w {
			direct[y*w+x] = o.color(img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	mode := o.Mode
	if mode == Auto {
		n := len(distinct(direct))
		switch {
		case n <= 16:
			mode = Mode4
		case n <= 256:
			mode = Mode8
		default:
			mode = Mode16
		}
	}
	switch mode {
	case Mode16:
		t := &TIM{Mode: Mode16, Pixels: Block{X: o.X, Y: o.Y, W: w, H: h, Data: direct}}
		return t, nil
	case Mode24:
		return o.direct24(img), nil
	}
	pal = quantize(direct, mode.Colors())
	idx = make([]uint8, len(direct))
	lookup := map[uint16]uint8{}
	for i, c := range direct {
		j, ok := lookup[c]
		if !ok {
			j = nearest(pal, c)
			lookup[c] = j
		}
		idx[i] = j
	}
	return o.indexed(mode, w, h, idx, pal)
}

// direct24 lays img out as 24-bit pixels, three bytes each, red first, the
// width padded to an even number of pixels so that a row is whole units.
// Such an image can only be displayed, not drawn with.
func (o Options) direct24(img image.Image) *TIM {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	units := (w + 1) / 2 * 3
	data := make([]uint16, units*h)
	for y := range h {
		for x := range w {
			n := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			for k, v := range [3]uint8{n.R, n.G, n.B} {
				i := 3*x + k
				data[y*units+i/2] |= uint16(v) << (8 * (i % 2))
			}
		}
	}
	return &TIM{Mode: Mode24, Pixels: Block{X: o.X, Y: o.Y, W: units, H: h, Data: data}}
}

// color is c as a 16-bit colour, with the options' rules for transparency.
func (o Options) color(c color.Color) uint16 {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if n.A < 128 {
		return 0
	}
	v := uint16(n.R>>3) | uint16(n.G>>3)<<5 | uint16(n.B>>3)<<10
	if v == 0 && o.Black == BlackTransparent {
		return 0
	}
	if v == 0 || o.Semi {
		v |= 0x8000
	}
	return v
}

func (o Options) indexed(mode Mode, w, h int, idx []uint8, pal []uint16) (*TIM, error) {
	per := 16 / mode.Bits() // pixels to a unit
	units := (w + per - 1) / per
	data := make([]uint16, units*h)
	for y := range h {
		for x := range w {
			data[y*units+x/per] |= uint16(idx[y*w+x]) << (mode.Bits() * (x % per))
		}
	}
	clut := make([]uint16, mode.Colors())
	copy(clut, pal)
	return &TIM{
		Mode:   mode,
		CLUT:   &Block{X: o.CLUTX, Y: o.CLUTY, W: len(clut), H: 1, Data: clut},
		Pixels: Block{X: o.X, Y: o.Y, W: units, H: h, Data: data},
	}, nil
}

// distinct returns the colours in cs, each once, in the order first seen.
func distinct(cs []uint16) []uint16 {
	seen := map[uint16]bool{}
	var out []uint16
	for _, c := range cs {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// quantize returns at most n colours for cs: the colours themselves when
// there are few enough, or else the averages of the boxes a median cut of
// their colour space makes. Transparency, 0x0000, keeps an entry of its own,
// index 0, so that it stays exact.
func quantize(cs []uint16, n int) []uint16 {
	d := distinct(cs)
	transparent := slices.Contains(d, 0)
	if len(d) <= n {
		if transparent {
			d = slices.DeleteFunc(d, func(c uint16) bool { return c == 0 })
			d = append([]uint16{0}, d...)
		}
		return d
	}
	counts := map[uint16]int{}
	for _, c := range cs {
		if c != 0 {
			counts[c]++
		}
	}
	slots := n
	if transparent {
		slots--
	}
	type entry struct {
		c uint16
		n int
	}
	var all []entry
	for c, k := range counts {
		all = append(all, entry{c, k})
	}
	slices.SortFunc(all, func(a, b entry) int { return int(a.c) - int(b.c) })
	comp := func(c uint16, ch int) int { return int(c>>(5*ch)) & 31 }
	boxes := [][]entry{all}
	for len(boxes) < slots {
		// Split the box with the widest range along its widest channel,
		// at the median by pixel count.
		best, bestCh, bestRange := -1, 0, 0
		for i, bx := range boxes {
			if len(bx) < 2 {
				continue
			}
			for ch := range 3 {
				lo, hi := 31, 0
				for _, e := range bx {
					v := comp(e.c, ch)
					lo, hi = min(lo, v), max(hi, v)
				}
				if hi-lo > bestRange || best < 0 {
					best, bestCh, bestRange = i, ch, hi-lo
				}
			}
		}
		if best < 0 {
			break
		}
		bx := boxes[best]
		slices.SortFunc(bx, func(a, b entry) int { return comp(a.c, bestCh) - comp(b.c, bestCh) })
		total := 0
		for _, e := range bx {
			total += e.n
		}
		half, cut := 0, 1
		for i, e := range bx[:len(bx)-1] {
			half += e.n
			cut = i + 1
			if 2*half >= total {
				break
			}
		}
		boxes = append(boxes[:best], append([][]entry{bx[:cut], bx[cut:]}, boxes[best+1:]...)...)
	}
	var out []uint16
	if transparent {
		out = append(out, 0)
	}
	for _, bx := range boxes {
		var sum [3]int
		total, stp := 0, uint16(0)
		for _, e := range bx {
			for ch := range 3 {
				sum[ch] += comp(e.c, ch) * e.n
			}
			total += e.n
			stp |= e.c & 0x8000
		}
		c := stp
		for ch := range 3 {
			c |= uint16((sum[ch]+total/2)/total) << (5 * ch)
		}
		if c == 0 {
			c = 0x8000
		}
		out = append(out, c)
	}
	return out
}

// nearest returns the index of the colour in pal closest to c. Only
// transparency matches transparency.
func nearest(pal []uint16, c uint16) uint8 {
	best, dist := 0, 1<<30
	for i, p := range pal {
		if (p == 0) != (c == 0) {
			continue
		}
		d := 0
		for ch := range 3 {
			v := int(p>>(5*ch))&31 - int(c>>(5*ch))&31
			d += v * v
		}
		if d < dist {
			best, dist = i, d
		}
	}
	return uint8(best)
}
