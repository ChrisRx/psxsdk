//go:build yaroze

package libps

import "unsafe"

// The drawing and display environments and the ordering tables, as
// libgpu's env.c and prim.c had them.

// PutDrawEnv makes env the drawing environment: its packet is built and
// sent through the queue, and env is remembered as the one in effect.
//
// libps 0x80028724, 196 bytes
//
//export PutDrawEnv
func PutDrawEnv(env *DRAWENV) *DRAWENV {
	if gpuDebug >= 2 {
		kprintf("PutDrawEnv(%08x)...\n", uint32(uintptr(unsafe.Pointer(env))))
	}
	setDrawEnv(&env.Dr_env, env)
	env.Dr_env.Tag |= 0xffffff
	addque(gpuCwc, unsafe.Pointer(&env.Dr_env), uint32(unsafe.Sizeof(env.Dr_env)), 0)
	curDraw = *env
	return env
}

// setDrawEnv builds the packet for a drawing environment: the drawing
// area, its offset, the texture page and window, the mask setting, and
// the clear of the area when the environment asks for one, by a fill when
// the area is on 64-pixel columns and by a tile otherwise.
//
// libps 0x80028F9C, 676 bytes
func setDrawEnv(p *DR_ENV, env *DRAWENV) {
	p.Code[0] = drawAreaTL(int32(env.Clip.X), int32(env.Clip.Y))
	p.Code[1] = drawAreaBR(int32(env.Clip.X)+int32(env.Clip.W)-1, int32(env.Clip.Y)+int32(env.Clip.H)-1)
	p.Code[2] = drawOffset(int32(env.Ofs[0]), int32(env.Ofs[1]))
	p.Code[3] = drawMode(env.Dfe, env.Dtd, env.Tpage)
	p.Code[4] = texWindow(&env.Tw)
	p.Code[5] = 0xe6000000
	n := 7
	if env.Isbg != 0 {
		x, y, w, h := env.Clip.X, env.Clip.Y, env.Clip.W, env.Clip.H
		switch {
		case w < 0:
			w = 0
		case w > gpuXMax-1:
			w = gpuXMax - 1
		}
		switch {
		case h < 0:
			h = 0
		case h > gpuYMax-1:
			h = gpuYMax - 1
		}
		rgb := uint32(env.B0)<<16 | uint32(env.G0)<<8 | uint32(env.R0)
		if x&0x3f == 0 && w&0x3f == 0 {
			p.Code[6] = 0x02000000 | rgb
			p.Code[7] = uint32(uint16(x)) | uint32(uint16(y))<<16
			p.Code[8] = uint32(uint16(w)) | uint32(uint16(h))<<16
		} else {
			p.Code[6] = 0x60000000 | rgb
			p.Code[7] = uint32(uint16(x-env.Ofs[0])) | uint32(uint16(y-env.Ofs[1]))<<16
			p.Code[8] = uint32(uint16(w)) | uint32(uint16(h))<<16
		}
		n = 10
	}
	p.Tag = p.Tag&0x00ffffff | uint32(n-1)<<24
}

// wideGPU reports whether the GPU is of type 1 or 2, whose environment
// commands take 12-bit coordinates and whose texture page value is laid
// out differently; type 0, which the console's GPU answers as, and types
// 3 and 4 take the 10-bit forms.
func wideGPU() bool { return gpuKind == 1 || gpuKind == 2 }

// clampX and clampY keep a coordinate inside video memory.
func clampX(x int32) int32 {
	if x < 0 {
		return 0
	}
	if x > int32(gpuXMax)-1 {
		return int32(gpuXMax) - 1
	}
	return x
}

func clampY(y int32) int32 {
	if y < 0 {
		return 0
	}
	if y > int32(gpuYMax)-1 {
		return int32(gpuYMax) - 1
	}
	return y
}

// drawAreaTL is the E3 command: the drawing area's top left.
//
// libps 0x8002929C, 204 bytes
func drawAreaTL(x, y int32) uint32 {
	x, y = clampX(x), clampY(y)
	if wideGPU() {
		return 0xe3000000 | uint32(y&0xfff)<<12 | uint32(x&0xfff)
	}
	return 0xe3000000 | uint32(y&0x3ff)<<10 | uint32(x&0x3ff)
}

// drawAreaBR is the E4 command: the drawing area's bottom right.
//
// libps 0x80029368, 204 bytes
func drawAreaBR(x, y int32) uint32 {
	x, y = clampX(x), clampY(y)
	if wideGPU() {
		return 0xe4000000 | uint32(y&0xfff)<<12 | uint32(x&0xfff)
	}
	return 0xe4000000 | uint32(y&0x3ff)<<10 | uint32(x&0x3ff)
}

// drawOffset is the E5 command: the drawing offset.
//
// libps 0x80029434, 72 bytes
func drawOffset(x, y int32) uint32 {
	if wideGPU() {
		return 0xe5000000 | uint32(y&0xfff)<<12 | uint32(x&0xfff)
	}
	return 0xe5000000 | uint32(y&0x7ff)<<11 | uint32(x&0x7ff)
}

// drawMode is the E1 command: the texture page, with dithering and
// drawing to the display area.
//
// libps 0x80029240, 92 bytes
func drawMode(dfe, dtd uint8, tpage uint16) uint32 {
	if wideGPU() {
		w := uint32(0xe1000000)
		if dtd != 0 {
			w |= 0x800
		}
		v := uint32(tpage) & 0x27ff
		if dfe != 0 {
			v |= 0x1000
		}
		return w | v
	}
	w := uint32(0xe1000000)
	if dtd != 0 {
		w |= 0x200
	}
	v := uint32(tpage) & 0x9ff
	if dfe != 0 {
		v |= 0x400
	}
	return w | v
}

// texWindow is the E2 command: the texture window, in units of eight
// pixels, or 0 for none.
//
// libps 0x8002947C, 132 bytes
func texWindow(tw *RECT) uint32 {
	if tw == nil {
		return 0
	}
	x := uint32(uint8(tw.X)) >> 3
	y := uint32(uint8(tw.Y)) >> 3
	w := uint32(int32(int8(uint8(-tw.W))) >> 3 & 0xff)
	h := uint32(int32(int8(uint8(-tw.H))) >> 3 & 0xff)
	return 0xe2000000 | x<<10 | y<<15 | h<<5 | w
}

// PutDispEnv makes env the display environment: where the display reads
// from, which part of the screen it fills and the video mode, each sent
// only when it differs from the environment in effect.
//
// Only the later GPUs' form is here; the original had another for types
// 1 and 2, which no emulator or console of this project is.
//
// libps 0x800288FC, 1200 bytes
//
//export PutDispEnv
func PutDispEnv(env *DISPENV) *DISPENV {
	if gpuDebug >= 2 {
		kprintf("PutDispEnv(%08x)...\n", uint32(uintptr(unsafe.Pointer(env))))
	}
	gpuCtl(0x05000000 | uint32(env.Disp.Y&0x3ff)<<10 | uint32(env.Disp.X&0x3ff))
	if env.Screen != curDisp.Screen {
		env.Pad0 = uint8(GetVideoMode())
		x0 := int32(env.Screen.X)*10 + 608
		y0 := int32(env.Screen.Y) + 16
		if env.Pad0 != 0 {
			y0 = int32(env.Screen.Y) + 19
		}
		x1 := x0 + 2560
		if env.Screen.W != 0 {
			x1 = x0 + int32(env.Screen.W)*10
		}
		y1 := y0 + 240
		if env.Screen.H != 0 {
			y1 = y0 + int32(env.Screen.H)
		}
		switch {
		case x0 < 500:
			x0 = 500
		case x0 >= 3291:
			x0 = 3290
		}
		switch {
		case x1 < x0+80:
			x1 = x0 + 80
		case x1 >= 3291:
			x1 = 3290
		}
		switch {
		case y0 < 16:
			y0 = 16
		case env.Pad0 != 0 && y0 >= 311:
			y0 = 310
		case env.Pad0 == 0 && y0 >= 257:
			y0 = 256
		}
		switch {
		case y1 < y0+2:
			y1 = y0 + 2
		case env.Pad0 != 0 && y1 >= 313:
			y1 = 312
		case env.Pad0 == 0 && y1 >= 259:
			y1 = 258
		}
		gpuCtl(0x06000000 | uint32(x1&0xfff)<<12 | uint32(x0&0xfff))
		gpuCtl(0x07000000 | uint32(y1&0x3ff)<<10 | uint32(y0&0x3ff))
	}
	if env.Isinter != curDisp.Isinter || env.Isrgb24 != curDisp.Isrgb24 || env.Pad0 != curDisp.Pad0 || env.Pad1 != curDisp.Pad1 || env.Disp != curDisp.Disp {
		env.Pad0 = uint8(GetVideoMode())
		mode := uint32(0x08000000)
		if env.Pad0 == 1 {
			mode |= 8
		}
		if env.Isrgb24 != 0 {
			mode |= 0x10
		}
		if env.Isinter != 0 {
			mode |= 0x20
		}
		if gpuReverse != 0 {
			mode |= 0x80
		}
		switch w := int32(env.Disp.W); {
		case w < 281:
		case w < 353:
			mode |= 1
		case w < 401:
			mode |= 0x40
		case w < 561:
			mode |= 2
		default:
			mode |= 3
		}
		lines := int32(257)
		if env.Pad0 != 0 {
			lines = 289
		}
		if int32(env.Disp.H) >= lines {
			mode |= 0x24
		}
		gpuCtl(mode)
	}
	curDisp = *env
	return env
}

// ClearOTagR clears an ordering table of n entries at ot, linked from its
// last entry to its first, which is how a table drawn by DrawOTag from
// its last entry puts what was sorted deepest first.
//
// libps 0x800285B8, 152 bytes
//
//export ClearOTagR
func ClearOTagR(ot *uint32, n int32) {
	if gpuDebug >= 2 {
		kprintf("ClearOTagR(%08x,%d)...\n", uint32(uintptr(unsafe.Pointer(ot))), n)
	}
	gpuOtc(unsafe.Pointer(ot), uint32(n))
	*ot = uint32(uintptr(unsafe.Pointer(&otTerminator[0]))) & 0xffffff
}

// otTerminator is the packet every cleared table ends with: four words
// the GPU does nothing with, and no next.
//
// libps 0x800483A0
var otTerminator = [5]uint32{0x04ffffff, 0, 0, 0, 0}

// DrawOTag draws the packets linked from p, through the queue.
//
// libps 0x800286B0, 116 bytes
//
//export DrawOTag
func DrawOTag(p *uint32) {
	if gpuDebug >= 2 {
		kprintf("DrawOTag(%08x)...\n", uint32(uintptr(unsafe.Pointer(p))))
	}
	addque(gpuCwc, unsafe.Pointer(p), 0, 0)
}

// addPrim links the packet p into the ordering table entry ot, ahead of
// what the entry already links.
//
// libps 0x8002762C, 60 bytes (AddPrim)
func addPrim(ot, p *uint32) {
	*p = *p&0xff000000 | *ot&0xffffff
	*ot = *ot&0xff000000 | uint32(uintptr(unsafe.Pointer(p)))&0xffffff
}
