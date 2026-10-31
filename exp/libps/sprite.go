//go:build yaroze

package libps

import "unsafe"

// Sprites and boxes: the 2D primitives, each a GPU packet built at
// GsOUT_PACKET_P and linked into an ordering table.

// GsSPRITE attribute bits.
const (
	spriteSemi    = 1 << 6  // drawn translucent
	spriteVFlip   = 1 << 22 // GsVFLIP
	spriteHFlip   = 1 << 23 // GsHFLIP
	spriteRotOff  = 1 << 27 // GsROTOFF: never rotated or scaled
	spriteNoShade = 1 << 30 // the texture's own colors
	spriteOff     = 1 << 31 // not drawn
)

// sortPacket links the packet at p, of n words after its tag, into the
// table at priority pri, and returns where the next packet goes. A
// priority before the table's offset is reported and taken as the first
// entry: the original wrote before the table.
//
// libps 0x8002048C, 148 bytes
func sortPacket(p *uint32, ot *GsOT, pri uint16, n uint8) *uint32 {
	z := int32(pri) - int32(ot.Offset)
	if z < 0 {
		kprintf("ps_sort_sprite,bg: z resolution over\n")
		z = 0
	}
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), z*4))
	*p = *entry
	*(*uint8)(unsafe.Add(unsafe.Pointer(p), 3)) = n
	*entry = uint32(uintptr(unsafe.Pointer(p)))
	*(*uint8)(unsafe.Add(unsafe.Pointer(entry), 3)) = 0
	return (*uint32)(unsafe.Add(unsafe.Pointer(p), int(n)*4+4))
}

// drawModeFor is the E1 word a sprite or box draws under: its texture
// page's low bits, the blending rate from attribute bits 28 and 29 and
// the depth from bits 24 and 25, dithered.
func drawModeFor(attr uint32, tpage uint16) uint32 {
	return 0xe1000200 | attr>>17&0x180 | uint32(tpage)&0x1f | attr>>23&0x60
}

// GsSortSprite sorts a sprite into ot at pri. One neither rotated nor
// scaled, or with GsROTOFF, is a GPU sprite at its position less its
// center; any other is a four-point textured polygon whose corners the
// GTE places from the rotation, the scale and the position.
//
// libps 0x800208C4, 1276 bytes
//
//export GsSortSprite
func GsSortSprite(sp *GsSPRITE, ot *GsOT, pri uint16) {
	attr := sp.Attribute
	if attr&spriteOff != 0 || sp.W == 0 || sp.H == 0 {
		return
	}
	p := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	w := unsafe.Slice(p, 10)
	plain := attr&spriteRotOff != 0
	if !plain && sp.Scalex == 4096 && sp.Scaley == 4096 && sp.Rotate == 0 {
		plain = attr&(spriteVFlip|spriteHFlip) == 0
	}
	rgb := uint32(sp.B)<<16 | uint32(sp.G)<<8 | uint32(sp.R)
	shade := attr>>5&0x02000000 | attr<<18&0x01000000
	clut := uint32(sp.Cy)<<22 | uint32(sp.Cx)<<12&0x3f0000
	if plain {
		w[1] = drawModeFor(attr, sp.Tpage)
		w[2] = shade | 0x64000000 | rgb
		x := uint32(uint16(sp.X + gsOfsX - sp.Mx))
		y := uint32(uint16(sp.Y + gsOfsY - sp.My))
		w[3] = x | y<<16
		w[4] = uint32(sp.U) | uint32(sp.V)<<8 | clut
		w[5] = uint32(sp.W) | uint32(sp.H)<<16
		GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(sortPacket(p, ot, pri, 5)))
		return
	}

	var m MATRIX
	if sp.Rotate == 0 {
		m = GsIDMATRIX
	} else {
		// The rotation is in 4096ths of a degree; the table takes
		// 4096ths of a turn.
		const inv360 = -1240768329 // 0xb60b60b7 as the signed multiplier it is
		hi := int32((int64(sp.Rotate) * inv360) >> 32)
		angle := (hi + sp.Rotate) >> 8
		angle -= sp.Rotate >> 31
		RotMatrix(&SVECTOR{0, 0, int16(angle), 0}, &m)
	}
	if sp.Scalex != 4096 || sp.Scaley != 4096 {
		ScaleMatrix(&m, &VECTOR{int32(sp.Scalex), int32(sp.Scaley), 0, 0})
	}
	TransMatrix(&m, &VECTOR{int32(sp.X), int32(sp.Y), getGeomScreen(), 0})
	setRotMatrix(&m)
	setTransMatrix(&m)
	mx, my := -sp.Mx, -sp.My
	v0 := SVECTOR{mx, my, 0, 0}
	v1 := SVECTOR{mx + int16(sp.W), my, 0, 0}
	v2 := SVECTOR{mx, my + int16(sp.H), 0, 0}
	v3 := SVECTOR{mx + int16(sp.W), my + int16(sp.H), 0, 0}
	var sxy [4]uint32
	var depth, flag uint32
	rotTransPers4(&v0, &v1, &v2, &v3, &sxy[0], &sxy[1], &sxy[2], &sxy[3], &depth, &flag)

	// The corners' texture coordinates, flipped by the attribute; the
	// width and height are taken as bytes, as the original did.
	u0, u1 := uint32(sp.U), uint32(sp.U)+uint32(uint8(sp.W))-1
	if attr&spriteHFlip != 0 {
		u0, u1 = u1, u0
	}
	v0t, v1t := uint32(sp.V), uint32(sp.V)+uint32(uint8(sp.H))-1
	if attr&spriteVFlip != 0 {
		v0t, v1t = v1t, v0t
	}
	u0, u1, v0t, v1t = u0&0xff, u1&0xff, v0t&0xff, v1t&0xff
	tpage := uint32(sp.Tpage)&0x1f | attr>>17&0x180 | attr>>23&0x60
	w[1] = shade | 0x2c000000 | rgb
	w[2] = sxy[0]
	w[3] = u0 | v0t<<8 | clut
	w[4] = sxy[1]
	w[5] = u1 | v0t<<8 | tpage<<16
	w[6] = sxy[2]
	w[7] = u0 | v1t<<8
	w[8] = sxy[3]
	w[9] = u1 | v1t<<8
	GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(sortPacket(p, ot, pri, 9)))
}

// GsSortBoxFill sorts a filled box into ot at pri: a GPU tile at the
// box's position, under the box's blending rate and depth.
//
// libps 0x80020628, 216 bytes
//
//export GsSortBoxFill
func GsSortBoxFill(bp *GsBOXF, ot *GsOT, pri uint16) {
	attr := bp.Attribute
	if attr&spriteOff != 0 {
		return
	}
	p := (*uint32)(unsafe.Pointer(GsOUT_PACKET_P))
	w := unsafe.Slice(p, 5)
	w[1] = 0xe1000200 | uint32(int32(attr)>>17)&0x180 | uint32(int32(attr)>>23)&0x60
	w[2] = uint32(bp.B)<<16 | uint32(bp.G)<<8 | uint32(bp.R) | (0x60|uint32(int32(attr)>>29)&2)<<24
	w[3] = uint32(uint16(bp.X+gsOfsX)) | uint32(uint16(bp.Y+gsOfsY))<<16
	w[4] = uint32(bp.W) | uint32(bp.H)<<16
	GsOUT_PACKET_P = (*PACKET)(unsafe.Pointer(sortPacket(p, ot, pri, 4)))
}
