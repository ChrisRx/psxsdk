//go:build yaroze

package cleanlib

import "unsafe"

// The sorts that put a 2D primitive into an ordering table.
//
// In the GsOFSGTE mode the drawing environment's offset stays 0 and the
// GTE's screen offset places the 3D primitives, so these sorts add that
// offset, OFX and OFY in whole pixels, to every coordinate they write
// (spec 09, "The sorts in the GsOFSGTE mode"). In the GsOFSGPU mode the
// environment carries it and they add nothing.

// GsBOXF is a filled rectangle.
type GsBOXF struct {
	Attribute uint32
	X, Y      int16
	W, H      uint16
	R, G, B   uint8
	_         uint8
}

// GsSPRITE is a sprite: a rectangle of a texture page drawn at a point,
// with a brightness, a rotation and a scale.
type GsSPRITE struct {
	Attribute      uint32
	X, Y           int16
	W, H           uint16
	Tpage          uint16
	U, V           uint8
	Cx, Cy         int16
	R, G, B        uint8
	_              uint8
	Mx, My         int16
	Scalex, Scaley int16
	Rotate         int32
}

// Attribute bits.
const (
	attrBrightOff = 1 << 6  // a sprite's R, G and B are ignored
	attrVFlip     = 1 << 22 // a turned sprite's texture upside down
	attrHFlip     = 1 << 23 // a turned sprite's texture left to right
	attrRotOff    = 1 << 27 // a sprite, whatever its turn, scale and flips say
	attrSemi      = 1 << 30 // drawn semi-transparent
	attrHidden    = 1 << 31 // not drawn
)

// What GsSortLine spends past its work, in 2-cycle stores, for Sony's 170 cycles on
// average in the web archive's programs (spec 22).
const (
	gsSortLineSpace = 24
)

// semiRate is the semi-transparency rate of an attribute, bits 28 and 29.
//
//go:inline
func semiRate(attr uint32) uint32 { return attr >> 28 & 3 }

// sortOffset returns what a 2D sort adds to its coordinates: the GTE's
// screen offset in the GsOFSGTE mode, nothing in the GsOFSGPU mode.
func sortOffset() (x, y int16) {
	if gs.ofsGPU {
		return 0, 0
	}
	return int16(gteOFX), int16(gteOFY)
}

// placed returns the point word xy, x in the low half, as a 2D sort writes it: with
// the GTE's screen offset in the GsOFSGTE mode, as it is in the GsOFSGPU mode.
//
//go:inline
func placed(xy uint32) uint32 {
	if gs.ofsGPU {
		return xy
	}
	return pair(int16(xy)+int16(gteOFX), int16(xy>>16)+int16(gteOFY))
}

// mode is the E1h command a sorted primitive starts with: the texture
// page tpage, dithered whatever GsInitGraph said (check and breakout,
// with dithering off, show it).
//
//go:inline
func mode(tpage uint32) uint32 {
	return 0xe1000000 | tpage&0x1ff | 1<<9
}

// insert links the packet p into ot at the entry for pri, 0 the front
// and the table's last entry the back. Within an entry the last packet
// linked is drawn first. As Sony's library does, pri is not clamped to
// the table: a larger one writes the word that far past Org (probe
// spec/probes/gsclip).
//
//go:inline
func insert(ot *GsOT, pri uint16, p []uint32) {
	e := (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(at(ot).Org)) + 4*uintptr(pri)))
	link(at(e), p)
}

// link puts the packet p first in the entry whose word is e: p leads to
// what e led to, and e to p, each keeping its own top byte.
//
//go:inline
func link(e *uint32, p []uint32) {
	old := *e
	p[0] = uint32(len(p)-1)<<24 | old&0xffffff
	*e = old&0xff000000 | physical(unsafe.Pointer(&p[0]))
}

// rgb is a GPU command word: the code in the top byte and the color, red
// in the low byte.
func rgb(code uint32, r, g, b uint8) uint32 {
	return code<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// GsSortBoxFill sorts the filled rectangle bp into ot at depth pri. In the target's
// assembly, with GsSortSprite and GsSortFastSprite.
//
//export GsSortBoxFill
func GsSortBoxFill(bp *GsBOXF, ot *GsOT, pri uint16)

// wordIn returns the ith word of the structure at p. The 2D sorts copy their fields
// a word at a time: a structure of a C program holding a uint32 is aligned to it.
//
//go:inline
func wordIn(p unsafe.Pointer, i uintptr) uint32 {
	return *(*uint32)(unsafe.Add(p, 4*i))
}

// GsSortSprite sorts the sprite sp into ot at depth pri, with its point
// Mx, My at X, Y. With GsROTOFF, or with no turn, a scale of exactly 1 and no
// flip, it is a sprite packet (64h). Any other is a textured quad (2Ch), a scale
// of 0 too, with its corners on one axis or point.
//
// The quad's corners are the GTE's: each one's offset from the point goes through
// a turn of Rotate (4096 to a degree) with Scalex and Scaley (4096 being 1) applied
// as ScaleMatrix applies them, at X, Y and the projection distance, so that the
// perspective comes to 1 and the screen offset is added. The GTE keeps this matrix
// after it, as Sony's does. The flips swap the corners' texture coordinates, which
// are each kept to 8 bits: a W of 256 gives a u1 of U-1.
//
//export GsSortSprite
func GsSortSprite(sp *GsSPRITE, ot *GsOT, pri uint16)

// spriteQuad is the quad of GsSortSprite, from its assembly.
//
//export libps_sprite_quad
func spriteQuad(sp *GsSPRITE, ot *GsOT, pri uint16) {
	attr := sp.Attribute
	if attr&attrHidden != 0 || sp.W == 0 || sp.H == 0 {
		return
	}
	code := uint32(0x2c)
	if attr&attrSemi != 0 {
		code |= 0x02
	}
	if attr&attrBrightOff != 0 {
		code |= 0x01
	}
	// No turn is the identity, not the table's values for an angle of 0.
	sn, cs := int32(0), int32(4096)
	if sp.Rotate != 0 {
		// Rotate is in 4096ths of a degree; the sine table's angles are
		// 4096ths of a turn, and it takes them in 16 bits, so that a turn
		// past 32767 of them is a negative angle.
		a := int32(int16(sp.Rotate / 360))
		sn, cs = sin(a), cos(a)
	}
	sx, sy := int32(sp.Scalex), int32(sp.Scaley)
	m := &spriteMatrix
	m.M[0][0], m.M[0][1] = int16(cs*sx>>12), int16(-sn*sy>>12)
	m.M[1][0], m.M[1][1] = int16(sn*sx>>12), int16(cs*sy>>12)
	// A scale goes on the third column as 0.
	m.M[2][2] = 4096
	if sx != 4096 || sy != 4096 {
		m.M[2][2] = 0
	}
	m.T = [3]int32{int32(sp.X), int32(sp.Y), gteH}
	SetRotMatrix(m)
	SetTransMatrix(m)
	x0, y0 := -sp.Mx, -sp.My
	x1, y1 := int16(sp.W)-sp.Mx, int16(sp.H)-sp.My

	u0, v0 := uint32(sp.U), uint32(sp.V)
	u1, v1 := (u0+uint32(sp.W&255)-1)&0xff, (v0+uint32(sp.H&255)-1)&0xff
	if attr&attrHFlip != 0 {
		u0, u1 = u1, u0
	}
	if attr&attrVFlip != 0 {
		v0, v1 = v1, v0
	}
	p := packet(10)
	p[1] = rgb(code, sp.R, sp.G, sp.B)
	p[2] = spriteCorner(x0, y0)
	p[3] = spriteClut(sp)<<16 | v0<<8 | u0
	p[4] = spriteCorner(x1, y0)
	p[5] = spriteTpage(attr, uint32(sp.Tpage))<<16 | v0<<8 | u1
	p[6] = spriteCorner(x0, y1)
	p[7] = v1<<8 | u0
	p[8] = spriteCorner(x1, y1)
	p[9] = v1<<8 | u1
	insert(ot, pri, p)
	scaled := sx != 4096 || sy != 4096
	switch {
	case sp.Rotate != 0 && scaled:
		pace(spriteBothPad)
	case sp.Rotate != 0:
		pace(spriteTurnPad)
	case scaled:
		pace(spriteScalePad)
	default:
		pace(spriteFlipPad)
	}
}

// The rest of a quad sprite's time, by what makes it a quad, for Sony's times.
const (
	spriteFlipPad  = 48
	spriteTurnPad  = 186
	spriteScalePad = 192
	spriteBothPad  = 336
)

// spriteMatrix is GsSortSprite's turn. As a local it would take a matrix of a
// program's small stack. Its third row and column stay those of the identity, but
// for the last element: a corner's z is 0, and its depth the projection distance.
var spriteMatrix MATRIX

// spriteCorner returns the screen point of the corner at x, y from a sprite's
// point, through the matrix in the GTE.
//
//go:inline
func spriteCorner(x, y int16) uint32 {
	rtps(pair(x, y), 0)
	return sxy2()
}

// spriteClut is a sprite's palette as a packet takes it, in a halfword.
func spriteClut(sp *GsSPRITE) uint32 {
	return (uint32(sp.Cy)<<6 | uint32(sp.Cx>>4)&0x3f) & 0xffff
}

// spriteTpage is a sprite's texture page: the low 5 bits of Tpage, its place,
// with the attribute's color mode (bits 24 and 25) and semi-transparency rate.
// Tpage's own mode and rate are ignored.
//
//go:inline
func spriteTpage(attr, tpage uint32) uint32 {
	return tpage&0x1f | semiRate(attr)<<5 | (attr>>24&3)<<7
}

// GsSortFastSprite sorts the sprite sp into ot at depth pri, unrotated and
// unscaled, with its top left corner at X, Y: it ignores Mx and My.
//
//export GsSortFastSprite
func GsSortFastSprite(sp *GsSPRITE, ot *GsOT, pri uint16)

// gsPlace is what the sorts in assembly add to a point: x and y of the GTE's screen
// offset in the GsOFSGTE mode, nothing in the GsOFSGPU mode.
//
//go:extern libps_place
var gsPlace uint32

// updatePlace sets gsPlace, and the rectangle a polygon to cut is dropped against
// (tmd.go), after the mode, the screen or the offset changes.
func updatePlace() {
	w, h := int32(gs.width)/2, int32(gs.height)/2
	d := &divLimits
	d.xlo, d.xhi = (gteOFX-w)<<16, (gteOFX+w)<<16
	d.ylo, d.yhi = (gteOFY-h)<<16, (gteOFY+h+1)<<16
	if gs.ofsGPU {
		gsPlace = 0
		return
	}
	gsPlace = pair(int16(gteOFX), int16(gteOFY))
}

// GsLINE is a straight line in one color.
type GsLINE struct {
	Attribute      uint32
	X0, Y0, X1, Y1 int16
	R, G, B        uint8
	_              uint8
}

// GsSortLine sorts the line lp into ot at depth pri: the mode word, then
// a line (40h, 42h semi-transparent) from X0, Y0 to X1, Y1.
//
//export GsSortLine
func GsSortLine(lp *GsLINE, ot *GsOT, pri uint16) {
	w := unsafe.Pointer(lp)
	attr := *(*uint32)(w)
	if attr&attrHidden != 0 {
		return
	}
	p := packet(5)
	p[1] = mode(semiRate(attr) << 5)
	p[2] = (0x40|attr>>29&2)<<24 | wordIn(w, 3)&0xffffff
	p[3] = placed(wordIn(w, 1))
	p[4] = placed(wordIn(w, 2))
	insert(ot, pri, p)
	space(gsSortLineSpace)
}

// GsGLINE is a line shaded from one color at its start to another at its
// end.
type GsGLINE struct {
	Attribute      uint32
	X0, Y0, X1, Y1 int16
	R0, G0, B0     uint8
	R1, G1, B1     uint8
	_              [2]uint8
}

// GsSortGLine sorts the line lp into ot at depth pri as GsSortLine sorts
// its line: the mode word, then a shaded line (50h, 52h semi-transparent)
// with the first color and point, then the second color and point.
//
//export GsSortGLine
func GsSortGLine(lp *GsGLINE, ot *GsOT, pri uint16) {
	if lp.Attribute&attrHidden != 0 {
		return
	}
	code := uint32(0x50)
	if lp.Attribute&attrSemi != 0 {
		code |= 0x02
	}
	ox, oy := sortOffset()
	p := packet(6)
	p[1] = mode(semiRate(lp.Attribute) << 5)
	p[2] = rgb(code, lp.R0, lp.G0, lp.B0)
	p[3] = pair(lp.X0+ox, lp.Y0+oy)
	p[4] = rgb(0, lp.R1, lp.G1, lp.B1)
	p[5] = pair(lp.X1+ox, lp.Y1+oy)
	insert(ot, pri, p)
}

// GsSortOt sorts the whole of src into dest at the entry src.Point, which
// is not clamped to dest's length: dest's entry then leads to src's Tag,
// and the last element of src's entry 0 (the entry itself if it is
// empty), which led to the end of src, leads to what dest's entry led to.
// Each link keeps the packet length in its top byte. It returns dest.
//
//export GsSortOt
func GsSortOt(src, dest *GsOT) *GsOT {
	end := physical(unsafe.Pointer(&otEnd))
	last := at(src.Org)
	for {
		next := *last & 0xffffff
		if next == end || next == endOfList {
			break
		}
		last = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
	e := at((*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(dest.Org)) + 4*uintptr(src.Point))))
	*last = *last&0xff000000 | *e&0xffffff
	*e = *e&0xff000000 | physical(unsafe.Pointer(src.Tag))
	return dest
}
