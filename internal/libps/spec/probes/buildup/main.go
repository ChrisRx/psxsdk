//go:build yaroze

// buildup is showcase-gasgar's display, built up a piece at a time, to find
// which piece makes it flash on a console. It shows 640 by 480 interlaced
// from one buffer, as gasgar does, with ResetGraph(1) at the top of each
// frame and the frame's drawing sent with GsDrawOt, and adds a piece every
// stageFrames frames:
//
//	0: the clear and a line of text
//	1: eight sprites moving across the screen
//	2: eight LoadImage calls every other frame, changing the sprites' texture
//	3: a lit torus of flat triangles, about gasgar's, turning
//	4: a semitransparent box over half the screen
//	5: a second torus
//
// The stage's number and name show on the screen and go to the serial port,
// with the frame and vertical blank counts every 60 frames, and each call
// is marked in RAM (mark).
// After the last stage it prints END and returns, so that it ends on Sony's
// library too.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	stageFrames = 600
	stages      = 6
)

type (
	tmdHdr struct{ ID, Flag, NObjs uint32 }
	tmdObj struct {
		VertTop, NVert, NormTop, NNorm, PrimTop, NPrim uint32
		Scale                                          int32
	}
	tmdVert struct{ Vx, Vy, Vz, Pad int16 }
)

var names = [stages]string{
	"clear and text",
	"+ 8 sprites",
	"+ 8 LoadImage every other frame",
	"+ torus",
	"+ semitransparent box",
	"+ second torus",
}

var (
	ot      yaroze.GsOT
	tags    [1 << 10]yaroze.GsOT_TAG
	packets [1 << 16]uint32
	model   [3 + 7 + tris*4 + verts*2 + tris*2 + 16]uint32
	objs    [2]yaroze.GsDOBJ2
	coords  [2]yaroze.GsCOORDINATE2
	view    yaroze.GsVIEW2
	light   = yaroze.GsF_LIGHT{Vx: 0, Vy: 0, Vz: 1, R: 0xff, G: 0xc0, B: 0x80}
	sprites [8]yaroze.GsSPRITE
	box     = yaroze.GsBOXF{Attribute: 1 << 30, X: -320, Y: 0, W: 640, H: 240, R: 40, G: 40, B: 120}
	tex     [32 * 32 / 2]uint32
	frames  [8][16 * 16 / 2]uint32
	textBuf [96]byte
	text    yaroze.Buffer
	lw, ls  yaroze.MATRIX
)

func put[T any](cursor *uintptr, v T) *T {
	p := (*T)(yaroze.Pointer(*cursor))
	*p = v
	*cursor += unsafe.Sizeof(v)
	return p
}

// build writes the torus as a TMD into model: flat lit triangles, one
// normal each, as gasgar's torus.tmd has.
func build() {
	cursor := uintptr(unsafe.Pointer(&model[0]))
	put(&cursor, tmdHdr{ID: 0x41, Flag: 1, NObjs: 1})
	obj := put(&cursor, tmdObj{})
	prim := cursor
	for k, t := range torusTri {
		put(&cursor, uint32(0x20)<<24|0x0304)
		put(&cursor, uint32(0x20_a08060))
		put(&cursor, uint32(t[0])<<16|uint32(k))
		put(&cursor, uint32(t[2])<<16|uint32(t[1]))
	}
	vtop := cursor
	for _, p := range torusPos {
		put(&cursor, tmdVert{Vx: p[0], Vy: p[1], Vz: p[2]})
	}
	ntop := cursor
	for _, n := range torusNorm {
		put(&cursor, tmdVert{Vx: n[0], Vy: n[1], Vz: n[2]})
	}
	*obj = tmdObj{
		VertTop: uint32(vtop), NVert: verts,
		NormTop: uint32(ntop), NNorm: tris,
		PrimTop: uint32(prim), NPrim: tris,
	}
}

// Each call is marked in a ring in the scratchpad, which a reset leaves
// alone (it clears RAM from 800A0000h up), so that after a hang the last
// calls can be read back through the boot disc's monitor. At ringAddr: the
// magic "BUIL", the count of marks, the count of vertical blank callbacks,
// and D2_MADR at the last callback. Then ringLen entries of eight words:
// the frame and the call (frame<<8 | call, with 80h set for a mark after
// the call returned), GPUSTAT, D2_CHCR, I_STAT | I_MASK<<16, DICR, timer 1
// (the line counter), the callback count, and D2_MADR. The callback count
// in the header against the last entry's shows whether blanks were still
// taken after the hang, and D2_MADR whether the drawing list went on.
const (
	ringAddr = 0x1f800000
	ringLen  = 31
	tailAddr = ringAddr + 16 + 32*ringLen
)

// The calls, as marked.
const (
	callVSync = 1 + iota
	callResetGraph
	callSort
	callSortSprite
	callSortObject
	callDrawOt
	callLoadImage
	callFntPrint
	callFntFlush
	callDone
)

func word(addr uintptr) *uint32 { return (*uint32)(yaroze.Pointer(addr)) }

func mark(f, call int) {
	n := volatile.LoadUint32(word(ringAddr + 4))
	e := uintptr(ringAddr + 16 + 32*(n%ringLen))
	volatile.StoreUint32(word(e), uint32(f)<<8|uint32(call))
	volatile.StoreUint32(word(e+4), volatile.LoadUint32(word(0x1f801814)))
	volatile.StoreUint32(word(e+8), volatile.LoadUint32(word(0x1f8010a8)))
	volatile.StoreUint32(word(e+12), volatile.LoadUint32(word(0x1f801070))&0xffff|volatile.LoadUint32(word(0x1f801074))<<16)
	volatile.StoreUint32(word(e+16), volatile.LoadUint32(word(0x1f8010f4)))
	volatile.StoreUint32(word(e+20), volatile.LoadUint32(word(0x1f801110)))
	volatile.StoreUint32(word(e+24), volatile.LoadUint32(word(ringAddr+8)))
	volatile.StoreUint32(word(e+28), volatile.LoadUint32(word(0x1f8010a0)))
	volatile.StoreUint32(word(ringAddr+4), n+1)
}

// returned marks call as having returned.
func returned(f, call int) { mark(f, call|0x80) }

// tick is the vertical blank callback: it counts the blanks taken and
// keeps where the drawing list had got to, and in the scratchpad's last 16
// bytes (tailAddr) GPUSTAT, D2_CHCR, DICR and I_STAT | I_MASK<<16 as the
// last blank was taken.
func tick() {
	volatile.StoreUint32(word(ringAddr+8), volatile.LoadUint32(word(ringAddr+8))+1)
	volatile.StoreUint32(word(ringAddr+12), volatile.LoadUint32(word(0x1f8010a0)))
	volatile.StoreUint32(word(tailAddr), volatile.LoadUint32(word(0x1f801814)))
	volatile.StoreUint32(word(tailAddr+4), volatile.LoadUint32(word(0x1f8010a8)))
	volatile.StoreUint32(word(tailAddr+8), volatile.LoadUint32(word(0x1f8010f4)))
	volatile.StoreUint32(word(tailAddr+12), volatile.LoadUint32(word(0x1f801070))&0xffff|volatile.LoadUint32(word(0x1f801074))<<16)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.GsInit3D()
	yaroze.GsSetProjection(600)
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(1024, 1024, 1024)
	yaroze.GsSetLightMode(0)
	ot.Length = 10
	ot.Org = &tags[0]
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(64, 48, 400, 24, 0, 96)
	text.Init(textBuf[:])

	// The sprites' texture, 16-bit, right of the display.
	for i := range tex {
		tex[i] = 0x7c1f_03e0
	}
	r := yaroze.RECT{X: 704, Y: 0, W: 32, H: 32}
	yaroze.LoadImage(&r, &tex[0])
	yaroze.DrawSync(0)
	tpage := yaroze.GetTPage(2, 0, 704, 0)
	for i := range sprites {
		sprites[i] = yaroze.GsSPRITE{Attribute: 2<<24 | 1<<27, X: int16(-280 + 70*i), Y: int16(-160 + 40*i), W: 32, H: 32, Tpage: tpage, R: 128, G: 128, B: 128, Scalex: 4096, Scaley: 4096}
	}

	build()
	view.View = yaroze.IDMATRIX
	view.View.T = [3]int32{0, 0, 2400}
	view.Super = yaroze.WORLD
	yaroze.GsSetView2(&view)
	for i := range objs {
		yaroze.GsInitCoordinate2(yaroze.WORLD, &coords[i])
		yaroze.TMD(model[:]).Link(&objs[i], 0)
		objs[i].Coord2 = &coords[i]
	}
	coords[1].Coord.T = [3]int32{300, -200, 600}

	volatile.StoreUint32(word(ringAddr), 0x4c495542)
	volatile.StoreUint32(word(ringAddr+4), 0)
	volatile.StoreUint32(word(ringAddr+8), 0)
	cb := tick
	yaroze.VSyncCallback((*[2]uintptr)(unsafe.Pointer(&cb))[1])
	yaroze.SetDispMask(1)
	println("setup done")
	for f := 0; f < stages*stageFrames; f++ {
		stage := f / stageFrames
		if f%stageFrames == 0 {
			println("STAGE", stage, names[stage])
		}
		if f%60 == 0 {
			println("frame", f, "vsync", yaroze.VSync(-1))
		}
		mark(f, callVSync)
		yaroze.VSync(0)
		returned(f, callVSync)
		mark(f, callResetGraph)
		yaroze.ResetGraph(1)
		mark(f, callSort)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)

		if stage >= 1 {
			for i := range sprites {
				s := &sprites[i]
				s.X += 2
				if s.X > 300 {
					s.X = -300
				}
				mark(f, callSortSprite)
				yaroze.GsSortSprite(s, &ot, 0)
			}
		}
		if stage >= 3 {
			n := 1
			if stage >= 5 {
				n = 2
			}
			for i := range n {
				rot := yaroze.SVECTOR{Vx: int16(f * 7), Vy: int16(f * 11), Vz: int16(f * 3 * (i + 1))}
				t := coords[i].Coord.T
				yaroze.RotMatrix(&rot, &coords[i].Coord)
				coords[i].Coord.T = t
				coords[i].Flg = 0
				yaroze.GsGetLw(&coords[i], &lw)
				yaroze.GsSetLightMatrix(&lw)
				yaroze.GsGetLs(&coords[i], &ls)
				yaroze.GsSetLsMatrix(&ls)
				mark(f, callSortObject)
				yaroze.GsSortObject4(&objs[i], &ot, 4, yaroze.Scratch())
			}
		}
		if stage >= 4 {
			yaroze.GsSortBoxFill(&box, &ot, 1)
		}
		mark(f, callDrawOt)
		yaroze.GsDrawOt(&ot)
		returned(f, callDrawOt)

		if stage >= 2 && f%2 == 0 {
			// The texture's 16 by 16 corners and middles, in a color
			// that changes each time, as gasgar animates its sprites.
			c := uint32(f/2) & 0x7fff
			for i := range frames {
				for j := range frames[i] {
					frames[i][j] = c | c<<16
				}
				r := yaroze.RECT{X: int16(704 + 16*(i%2)), Y: int16(16 * (i / 2 % 2)), W: 16, H: 16}
				mark(f, callLoadImage)
				yaroze.LoadImage(&r, &frames[i][0])
				returned(f, callLoadImage)
			}
		}

		mark(f, callFntPrint)
		text.Reset()
		text.WriteString("STAGE ").WriteInt(stage).WriteString(": ").WriteString(names[stage]).Print()
		mark(f, callFntFlush)
		yaroze.FntFlush(-1)
		returned(f, callFntFlush)
		mark(f, callDone)
	}
	yaroze.VSyncCallback(0)
	println("END")
}
