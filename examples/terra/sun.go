package main

// The sun and its glare. The sun is a sprite where its direction projects
// on the screen, behind the world, and whether the world hides it is found
// out by drawing a white pixel there in the frame, behind the world, and
// copying it back out in front of everything, to be read next frame: when
// the pixel comes back white the sun is in view and a glare washes over
// the screen with a line of flares across it.

import "go.chrisrx.dev/psx/yaroze"

var (
	// probeOn turns the pixel test on; probeX and probeY are where on the
	// screen it tests, 0, 0 for nowhere, and probeSeen is what came back.
	probeOn        bool
	probeX, probeY int16
	probeSeen      bool
	sunDir         yaroze.SVECTOR
	sunXY          uint32
	sunLS          yaroze.MATRIX
	probeRect      yaroze.RECT
	probePixel     [2]uint32
	flareSpacing   = [7]int32{-4096, -2867, -819, 819, 2048, 3276, 6144}
	sunTextures    []texture
)

// The pixel the probe copies around lives at 832, 352 in video memory, and
// its copy comes back to 836, 352.
const (
	probeSrcX, probeSrcY = 0x340, 0x160
	probeDstX, probeDstY = 0x344, 0x160
)

// probeInit puts the white pixel in video memory.
func probeInit() {
	probePixel[0] = 0xffff
	probeRect = yaroze.RECT{X: probeSrcX, Y: probeSrcY, W: 1, H: 1}
	probeRect.Load(&probePixel[0])
	yaroze.DrawSync(0)
}

// probeFrame does the frame's part of the pixel test, between the swap
// and the drawing: it sorts the copy of the white pixel to the probe's
// place at the back of the table and the copy back from there at the
// front, or blanks the result when there is nowhere to test, and reads
// what the last frame's copy brought back.
func probeFrame(ot *yaroze.GsOT) {
	if !probeOn {
		return
	}
	if probeX == 0 && probeY == 0 {
		probeRect = yaroze.RECT{X: probeDstX, Y: probeDstY, W: 1, H: 1}
		probeRect.Clear(0, 0, 0)
	} else {
		x := int32(probeX) + (1-yaroze.GsGetActiveBuff())*320
		copyPixel(ot.Entry(4093), probeSrcX, probeSrcY, x, int32(probeY))
		copyPixel(ot.Entry(1), x, int32(probeY), probeDstX, probeDstY)
	}
	probeRect = yaroze.RECT{X: probeDstX, Y: probeDstY, W: 1, H: 1}
	probeRect.Store(&probePixel[0])
	yaroze.DrawSync(0)
	probeSeen = probePixel[0]&0x8000 != 0
}

// copyPixel links a copy of one pixel of video memory, from one place to
// another, at tag.
func copyPixel(tag *uint32, sx, sy, dx, dy int32) {
	p := yaroze.AllocPacket(6)
	p[0] = 5 << 24
	p[1] = 0x01000000
	p[2] = 0x80000000
	p[3] = xy(int16(sx), int16(sy))
	p[4] = xy(int16(dx), int16(dy))
	p[5] = 1 | 1<<16
	yaroze.LinkTag(tag, p)
}

// drawSun draws the sun behind the world and, when the last frame's probe
// saw it, the glare and flares over everything.
func drawSun(ot, over *yaroze.GsOT) {
	sunDir = yaroze.SVECTOR{Vx: -4096, Vy: -1365, Vz: -4096}
	if areaStyle == areaDark {
		sunDir.Vy = -682
	}
	sunLS = *yaroze.GsWSMATRIX()
	sunLS.T = [3]int32{}
	yaroze.GsSetLsMatrix(&sunLS)
	flag := yaroze.RotTransPers(&sunDir, &sunXY)
	sx, sy := int16(sunXY), int16(sunXY>>16)
	if flag < 0 || uint16(sx+160) > 0x13f || uint16(sy+120) > 0xef {
		probeX, probeY = 0, 0
	} else {
		probeX, probeY = sx+160, sy+120
	}
	if flag >= 0 {
		t := &sunTextures[0]
		sprite(ot.Entry(4094), sx-t.w/2, sy-t.h/2, t, 0x67)
	}
	tpagePacket(ot.Entry(4094), &sunTextures[1], 1)
	if !probeSeen {
		return
	}
	d := int32(sx) * int32(sy)
	if d < 0 {
		d = -d
	}
	if d < 0x1900 {
		c := uint8((0x1900 - d) * 0x5c / 0x1900)
		p := yaroze.AllocPacket(6)
		p[0] = 5 << 24
		p[1] = 0x2a<<24 | uint32(c)<<16 | uint32(c)<<8 | uint32(c)
		p[2] = xy(-160, -120)
		p[3] = xy(160, -120)
		p[4] = xy(-160, 120)
		p[5] = xy(160, 120)
		yaroze.LinkTag(over.Entry(2), p)
	}
	for i := range 7 {
		t := &sunTextures[1+i]
		k := flareSpacing[i]
		sprite(over.Entry(2), int16(int32(sx)*k>>12)-t.w/2, int16(int32(sy)*k>>12)-t.h/2, t, 0x67)
	}
	tpagePacket(over.Entry(2), &sunTextures[1], 1)
}

// sprite links a sprite of the whole texture at x, y with the given GPU
// code, 0x64 for plain and 0x66 or 0x67 for semi-transparent, at tag.
func sprite(tag *uint32, x, y int16, t *texture, code uint32) {
	p := yaroze.AllocPacket(5)
	p[0] = 4 << 24
	p[1] = code<<24 | 0x808080
	p[2] = xy(x, y)
	p[3] = uint32(t.u) | uint32(t.v)<<8 | uint32(t.clut())<<16
	p[4] = xy(t.w, t.h)
	yaroze.LinkTag(tag, p)
}

// tpagePacket links a texture page setting for the texture, at the
// semi-transparency rate, at tag. The packets at a tag run last to
// first, so it goes after the sprites it is for.
func tpagePacket(tag *uint32, t *texture, abr uint32) {
	p := yaroze.AllocPacket(2)
	p[0] = 1 << 24
	p[1] = 0xe1000200 | uint32(t.tpageY>>4&0x10) | uint32((t.tpageX&0x3ff)>>6) | abr<<5 | uint32((t.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, p)
}
