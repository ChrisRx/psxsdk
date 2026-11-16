package main

// The sun and its glare. The sun is a sprite where its direction projects
// on the screen, behind the world, and whether the world hides it is found
// out by drawing a white pixel there in the frame, behind the world, and
// copying it back out in front of everything, to be read next frame: when
// the pixel comes back white the sun is in view and a glare washes over
// the screen with a line of flares across it.

import (
	"go.chrisrx.dev/psx/gpu"
	"go.chrisrx.dev/psx/yaroze"
)

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
func probeFrame() {
	if !probeOn {
		return
	}
	if probeX == 0 && probeY == 0 {
		probeRect = yaroze.RECT{X: probeDstX, Y: probeDstY, W: 1, H: 1}
		probeRect.Clear(0, 0, 0)
	} else {
		x := int32(probeX) + (1-yaroze.GsGetActiveBuff())*320
		copyPixel(4093, probeSrcX, probeSrcY, x, int32(probeY))
		copyPixel(1, x, int32(probeY), probeDstX, probeDstY)
	}
	probeRect = yaroze.RECT{X: probeDstX, Y: probeDstY, W: 1, H: 1}
	probeRect.Store(&probePixel[0])
	yaroze.DrawSync(0)
	probeSeen = probePixel[0]&0x8000 != 0
}

// copyPixel links a copy of one pixel of video memory, from one place to
// another, into the frame at depth.
func copyPixel(depth int, sx, sy, dx, dy int32) {
	p := frame.Packet[vramCopy](depth)
	p.From = gpu.XY{X: int16(sx), Y: int16(sy)}
	p.To = gpu.XY{X: int16(dx), Y: int16(dy)}
	p.W, p.H = 1, 1
}

// drawSun draws the sun behind the world and, when the last frame's probe
// saw it, the glare and flares over everything.
func drawSun() {
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
		sprite(frame, 4094, sx-t.w/2, sy-t.h/2, t)
	}
	frame.Packet[gpu.DrawMode](4094).Page = sunTextures[1].mode() | 1<<5
	if !probeSeen {
		return
	}
	d := int32(sx) * int32(sy)
	if d < 0 {
		d = -d
	}
	if d < 0x1900 {
		c := uint8((0x1900 - d) * 0x5c / 0x1900)
		p := over.Packet[gpu.FlatQuad](2)
		p.Color = gray(c)
		p.Code |= gpu.CodeTranslucent
		p.XY0, p.XY1 = gpu.At(-160, -120), gpu.At(160, -120)
		p.XY2, p.XY3 = gpu.At(-160, 120), gpu.At(160, 120)
	}
	for i := range 7 {
		t := &sunTextures[1+i]
		k := flareSpacing[i]
		sprite(over, 2, int16(int32(sx)*k>>12)-t.w/2, int16(int32(sy)*k>>12)-t.h/2, t)
	}
	over.Packet[gpu.DrawMode](2).Page = sunTextures[1].mode() | 1<<5
}

// sprite links a semi-transparent sprite of the whole texture, in its own
// colors, at x, y, into f at depth.
func sprite(f *yaroze.Frame, depth int, x, y int16, t *texture) {
	p := f.Packet[gpu.Sprite](depth)
	p.Color = gray(0x80)
	p.Code |= gpu.CodeTranslucent | gpu.CodeRaw
	p.XY = gpu.XY{X: x, Y: y}
	p.UV = gpu.UV{U: t.u, V: t.v}
	p.Clut = t.clut()
	p.W, p.H = uint16(t.w), uint16(t.h)
}
