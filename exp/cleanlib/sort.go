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
	attrSemi      = 1 << 30 // drawn semi-transparent
	attrHidden    = 1 << 31 // not drawn
)

// semiRate is the semi-transparency rate of an attribute, bits 28 and 29.
func semiRate(attr uint32) uint32 { return attr >> 28 & 3 }

// sortOffset returns what a 2D sort adds to its coordinates: the GTE's
// screen offset in the GsOFSGTE mode, nothing in the GsOFSGPU mode.
func sortOffset() (x, y int16) {
	if gs.ofsGPU {
		return 0, 0
	}
	return int16(gteOFX), int16(gteOFY)
}

// mode is the E1h command a sorted primitive starts with: the texture
// page tpage, dithered whatever GsInitGraph said (check and breakout,
// with dithering off, show it).
func mode(tpage uint32) uint32 {
	return 0xe1000000 | tpage&0x1ff | 1<<9
}

// insert links the packet p into ot at the entry for pri, 0 the front
// and the table's last entry the back. Within an entry the last packet
// linked is drawn first. As Sony's library does, pri is not clamped to
// the table: a larger one writes the word that far past Org (probe
// spec/probes/gsclip).
func insert(ot *GsOT, pri uint16, p []uint32) {
	e := (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), 4*uintptr(pri)))
	link(e, p)
}

// link puts the packet p first in the entry whose word is e: p leads to
// what e led to, and e to p, each keeping its own top byte.
func link(e *uint32, p []uint32) {
	p[0] = uint32(len(p)-1)<<24 | *e&0xffffff
	*e = *e&0xff000000 | physical(unsafe.Pointer(&p[0]))
}

// rgb is a GPU command word: the code in the top byte and the color, red
// in the low byte.
func rgb(code uint32, r, g, b uint8) uint32 {
	return code<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// GsSortBoxFill sorts the filled rectangle bp into ot at depth pri.
//
//export GsSortBoxFill
func GsSortBoxFill(bp *GsBOXF, ot *GsOT, pri uint16) {
	if bp.Attribute&attrHidden != 0 {
		return
	}
	code := uint32(0x60)
	if bp.Attribute&attrSemi != 0 {
		code |= 0x02
	}
	ox, oy := sortOffset()
	p := packet(5)
	p[1] = mode(semiRate(bp.Attribute) << 5)
	p[2] = rgb(code, bp.R, bp.G, bp.B)
	p[3] = pair(bp.X+ox, bp.Y+oy)
	p[4] = pair(int16(bp.W), int16(bp.H))
	insert(ot, pri, p)
}

// GsSortSprite sorts the sprite sp into ot at depth pri, with its point
// Mx, My at X, Y. Unrotated and unscaled it is a sprite packet (64h); a
// sprite turned by Rotate (4096 to a degree) or scaled by Scalex and
// Scaley (4096 being 1) about that point becomes a textured quad (2Ch),
// whose corners are each corner's offset from the point, through the turn
// with the scale folded in, added to X, Y.
//
//export GsSortSprite
func GsSortSprite(sp *GsSPRITE, ot *GsOT, pri uint16) {
	if sp.Rotate == 0 && (sp.Scalex == 4096 || sp.Scalex == 0) && (sp.Scaley == 4096 || sp.Scaley == 0) {
		sortSprite(sp, ot, pri, sp.X-sp.Mx, sp.Y-sp.My)
		return
	}
	if sp.Attribute&attrHidden != 0 || sp.W == 0 || sp.H == 0 {
		return
	}
	code := uint32(0x2c)
	if sp.Attribute&attrSemi != 0 {
		code |= 0x02
	}
	if sp.Attribute&attrBrightOff != 0 {
		code |= 0x01
	}
	tpage := spriteTpage(sp)
	clut := uint32(sp.Cx>>4)&0x3f | (uint32(sp.Cy)&0x1ff)<<6
	// Rotate is in 4096ths of a degree; the sine table's angles are
	// 4096ths of a turn.
	a := sp.Rotate / 360
	sn, cs := sin(a), cos(a)
	w, h := int32(sp.W), int32(sp.H)
	// The scale is folded into the turn first (dogtale's logo shows the
	// rounding).
	sx, sy := int32(sp.Scalex), int32(sp.Scaley)
	m00, m01 := cs*sx>>12, -sn*sy>>12
	m10, m11 := sn*sx>>12, cs*sy>>12
	ox, oy := sortOffset()
	corner := func(cx, cy int32) uint32 {
		dx, dy := cx-int32(sp.Mx), cy-int32(sp.My)
		x := int32(sp.X) + (dx*m00+dy*m01)>>12
		y := int32(sp.Y) + (dx*m10+dy*m11)>>12
		return pair(int16(x)+ox, int16(y)+oy)
	}
	u0, v0 := uint32(sp.U), uint32(sp.V)
	u1, v1 := u0+uint32(w)-1, v0+uint32(h)-1
	p := packet(10)
	p[1] = rgb(code, sp.R, sp.G, sp.B)
	p[2] = corner(0, 0)
	p[3] = clut<<16 | v0<<8 | u0
	p[4] = corner(w, 0)
	p[5] = tpage<<16 | v0<<8 | u1&0xff
	p[6] = corner(0, h)
	p[7] = v1<<8 | u0
	p[8] = corner(w, h)
	p[9] = v1<<8 | u1&0xff
	insert(ot, pri, p)
}

// sortSprite sorts the sprite sp into ot at depth pri with its top left
// corner at x, y.
func sortSprite(sp *GsSPRITE, ot *GsOT, pri uint16, x, y int16) {
	if sp.Attribute&attrHidden != 0 || sp.W == 0 || sp.H == 0 {
		return
	}
	code := uint32(0x64)
	if sp.Attribute&attrSemi != 0 {
		code |= 0x02
	}
	if sp.Attribute&attrBrightOff != 0 {
		code |= 0x01
	}
	tpage := spriteTpage(sp)
	ox, oy := sortOffset()
	p := packet(6)
	p[1] = mode(tpage)
	p[2] = rgb(code, sp.R, sp.G, sp.B)
	p[3] = pair(x+ox, y+oy)
	clut := uint32(sp.Cx>>4)&0x3f | (uint32(sp.Cy)&0x1ff)<<6
	p[4] = clut<<16 | uint32(sp.V)<<8 | uint32(sp.U)
	p[5] = pair(int16(sp.W), int16(sp.H))
	insert(ot, pri, p)
}

// spriteTpage is a sprite's texture page: Tpage with the semi-transparency
// rate from the attribute, and the attribute's color mode (bits 24 and 25)
// added to its own.
func spriteTpage(sp *GsSPRITE) uint32 {
	return uint32(sp.Tpage)&^(3<<5) | semiRate(sp.Attribute)<<5 | (sp.Attribute>>24&3)<<7
}

// GsSortFastSprite sorts the sprite sp into ot at depth pri, unrotated and
// unscaled, with its top left corner at X, Y: it ignores Mx and My.
//
//export GsSortFastSprite
func GsSortFastSprite(sp *GsSPRITE, ot *GsOT, pri uint16) {
	sortSprite(sp, ot, pri, sp.X, sp.Y)
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
	if lp.Attribute&attrHidden != 0 {
		return
	}
	code := uint32(0x40)
	if lp.Attribute&attrSemi != 0 {
		code |= 0x02
	}
	ox, oy := sortOffset()
	p := packet(5)
	p[1] = mode(semiRate(lp.Attribute) << 5)
	p[2] = rgb(code, lp.R, lp.G, lp.B)
	p[3] = pair(lp.X0+ox, lp.Y0+oy)
	p[4] = pair(lp.X1+ox, lp.Y1+oy)
	insert(ot, pri, p)
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
	last := src.Org
	for {
		next := *last & 0xffffff
		if next == end || next == endOfList {
			break
		}
		last = (*uint32)(unsafe.Pointer(uintptr(0x80000000 | next)))
	}
	e := (*uint32)(unsafe.Add(unsafe.Pointer(dest.Org), 4*uintptr(src.Point)))
	*last = *last&0xff000000 | *e&0xffffff
	*e = *e&0xff000000 | physical(unsafe.Pointer(src.Tag))
	return dest
}
