//go:build yaroze

// Command gsclip is a probe for the spec: GsSetOrign, GsSetDrawBuffOffset,
// GsSetClip2D, GsSetDrawBuffClip and their effect on GsSwapDispBuff, in
// the GsOFSGPU and then the GsOFSGTE mode; then GsSortGLine, GsSortOt and
// GsGetWorkBase. Each case goes between GP1 marker words (10000100h plus
// the case), so the GPU stream shows what it sends, and the TTY carries
// the library's state after it: the drawing environment's clip and
// offset, POSITION, the GTE's screen offset, and the packet area's use.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n int) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|uint32(n))
}

var gte bool

func ofx() int32 {
	return int32(device.AsmFull(".set push\n.set noat\n.word 0x485cc000\nnop\nmove {}, $$28\n.set pop", nil))
}

func ofy() int32 {
	return int32(device.AsmFull(".set push\n.set noat\n.word 0x485cc800\nnop\nmove {}, $$28\n.set pop", nil))
}

func state(n int) {
	e := yaroze.GsDRAWENV()
	p := yaroze.POSITION()
	print("case ", n, ": clip ", e.Clip.X, " ", e.Clip.Y, " ", e.Clip.W, " ", e.Clip.H,
		" ofs ", e.Ofs[0], " ", e.Ofs[1], " pos ", p.Offx, " ", p.Offy, " active ", yaroze.GsGetActiveBuff())
	d := yaroze.GsDISPENV()
	print(" disp ", d.Disp.X, " ", d.Disp.Y, " ", d.Disp.W, " ", d.Disp.H)
	if gte {
		print(" gte ", ofx(), " ", ofy())
	}
	println()
}

var n int

// step runs f as the next case.
func step(f func()) {
	mark(n)
	f()
	yaroze.DrawSync(0)
	state(n)
	n++
}

var clips = []yaroze.RECT{
	{X: 5, Y: 6, W: 100, H: 50},
	{X: 0, Y: 0, W: 320, H: 240},
	{X: -10, Y: -10, W: 400, H: 300},
	{X: 300, Y: 200, W: 100, H: 100},
	{X: 0, Y: 0, W: 0, H: 0},
	{X: 1000, Y: 600, W: 30, H: 30},
	{X: -2000, Y: 3, W: 5000, H: 7},
}

var origins = [][2]int32{{10, 20}, {-5, -7}, {160, 120}, {2000, 3000}, {-3000, 70000}, {0, 0}}

func clipsAndOrigins() {
	for i := range origins {
		o := origins[i]
		step(func() { yaroze.GsSetOrign(o[0], o[1]) })
		step(func() { yaroze.GsSetDrawBuffOffset() })
		step(func() { yaroze.GsSwapDispBuff() })
		step(func() { yaroze.GsSwapDispBuff() })
	}
	for i := range clips {
		c := clips[i]
		step(func() { yaroze.GsSetClip2D(&c) })
		step(func() { yaroze.GsSetDrawBuffClip() })
		step(func() { yaroze.GsSwapDispBuff() })
		step(func() { yaroze.GsSwapDispBuff() })
	}
	// Mixed with GsSetOffset and GsSetClip, and the clip changed after
	// GsSetClip2D took it.
	step(func() { yaroze.GsSetOrign(33, 44) })
	step(func() {
		r := yaroze.RECT{X: 8, Y: 9, W: 64, H: 32}
		yaroze.GsSetClip2D(&r)
		r.W = 1
	})
	step(func() { yaroze.GsSetOffset(1, 2) })
	step(func() { yaroze.GsSetDrawBuffOffset() })
	step(func() {
		r := yaroze.RECT{X: 1, Y: 1, W: 10, H: 10}
		yaroze.GsSetClip(&r)
	})
	step(func() { yaroze.GsSetDrawBuffClip() })
	step(func() { yaroze.GsSwapDispBuff() })
	step(func() { yaroze.GsSwapDispBuff() })
	step(func() { yaroze.GsDefDispBuff(0, 0, 320, 0) })
	step(func() { yaroze.GsSetDrawBuffClip() })
	step(func() { yaroze.GsSetDrawBuffOffset() })
	step(func() { yaroze.GsSwapDispBuff() })
	step(func() { yaroze.GsSwapDispBuff() })
}

var (
	packets [2][4096]uint32
	tags    [3][1 << 6]yaroze.GsOT_TAG
	ots     [3]yaroze.GsOT
)

func off(p *yaroze.PACKET) int {
	return int(uintptr(unsafe.Pointer(p)) - uintptr(unsafe.Pointer(&packets[0][0])))
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	state(-1)
	clipsAndOrigins()

	// The same in the GsOFSGTE mode, the GTE enabled by GsInit3D.
	step(func() { yaroze.GsInitGraph(320, 240, yaroze.GsOFSGTE, 0, 0) })
	step(func() { yaroze.GsDefDispBuff(0, 0, 0, 240) })
	step(func() { yaroze.GsInit3D() })
	gte = true
	state(-2)
	clipsAndOrigins()

	// GsSortGLine, GsSortOt and GsGetWorkBase.
	step(func() { yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0) })
	gte = false
	step(func() { yaroze.GsDefDispBuff(0, 0, 0, 240) })
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0][0]))
	println("work", off(yaroze.GsGetWorkBase()))
	lens := [3]uint32{4, 6, 1}
	for i := range ots {
		ots[i].Length = lens[i]
		ots[i].Org = &tags[i][0]
	}
	lines := []yaroze.GsGLINE{
		{X0: 10, Y0: 20, X1: 200, Y1: 100, R0: 255, G0: 0, B0: 0, R1: 0, G1: 255, B1: 128},
		{Attribute: 1 << 30, X0: -5, Y0: -5, X1: 400, Y1: 300, R0: 1, G0: 2, B0: 3, R1: 4, G1: 5, B1: 6},
		{Attribute: 1<<30 | 1<<28, X0: 0, Y0: 0, X1: 0, Y1: 0, R0: 9, G0: 9, B0: 9, R1: 9, G1: 9, B1: 9},
		{Attribute: 1<<30 | 3<<28, X0: 100, Y0: 100, X1: 50, Y1: 10, R0: 128, G0: 128, B0: 128, R1: 255, G1: 255, B1: 255},
		{Attribute: 1 << 31, X0: 1, Y0: 2, X1: 3, Y1: 4},
		{Attribute: 2 << 28, X0: -32768, Y0: 32767, X1: 1023, Y1: 511, R0: 7, B1: 7},
		{Attribute: 0x0fffffff, X0: 11, Y0: 12, X1: 13, Y1: 14, R0: 15, G0: 16, B0: 17, R1: 18, G1: 19, B1: 20},
		{Attribute: 1 << 29, X0: 21, Y0: 22, X1: 23, Y1: 24, R0: 25, G0: 26, B0: 27, R1: 28, G1: 29, B1: 30},
	}
	// Depths past a table's end land in the arrays' unused entries or
	// the next table's: the sorts do not clamp them, and a depth past
	// the arrays would write wherever it lands.
	pris := []uint16{0, 3, 15, 16, 40, 63, 7, 1}
	for round := range 4 {
		yaroze.GsClearOt(0, 0, &ots[0])
		yaroze.GsClearOt(0, 5, &ots[1])
		yaroze.GsClearOt(0, 1, &ots[2])
		for i := range lines {
			yaroze.GsSortGLine(&lines[i], &ots[0], pris[(i+round)%len(pris)])
			yaroze.GsSortGLine(&lines[i], &ots[1], pris[(i+round*3)%len(pris)]+10)
			println("work", off(yaroze.GsGetWorkBase()))
		}
		if round == 0 {
			// GsSortLine past the end, for comparison.
			l := yaroze.GsLINE{X0: 1, Y0: 2, X1: 3, Y1: 4, R: 5, G: 6, B: 7}
			yaroze.GsSortLine(&l, &ots[0], 20)
			b := yaroze.GsBOXF{X: 3, Y: 3, W: 2, H: 2, G: 60}
			yaroze.GsSortBoxFill(&b, &ots[0], 30)
			sp := yaroze.GsSPRITE{X: 4, Y: 4, W: 8, H: 8, R: 128, G: 128, B: 128, Scalex: 4096, Scaley: 4096}
			yaroze.GsSortSprite(&sp, &ots[0], 33)
		}
		box := yaroze.GsBOXF{X: 1, Y: 1, W: 8, H: 8, R: 50}
		yaroze.GsSortBoxFill(&box, &ots[2], 0)
		var r *yaroze.GsOT
		switch round {
		case 0:
			r = yaroze.GsSortOt(&ots[1], &ots[0])
		case 1:
			ots[1].Point = 2
			r = yaroze.GsSortOt(&ots[1], &ots[0])
			r = yaroze.GsSortOt(&ots[2], &ots[0])
		case 2:
			ots[1].Point = 100
			yaroze.GsSortOt(&ots[2], &ots[1])
			r = yaroze.GsSortOt(&ots[1], &ots[0])
		case 3:
			// A clear sorted in front moves Tag; the entry is past the
			// end of no table (a point past the destination's arrays
			// would write wherever it lands).
			yaroze.GsSortClear(1, 2, 3, &ots[0])
			ots[0].Point = 50
			r = yaroze.GsSortOt(&ots[0], &ots[1])
			// Sorting into the source after.
			yaroze.GsSortGLine(&lines[0], &ots[0], 0)
		}
		println("sortot ret", r == &ots[0], r == &ots[1], "work", off(yaroze.GsGetWorkBase()))
		dest := &ots[0]
		if round == 3 {
			dest = &ots[1]
		}
		step(func() {
			yaroze.GsDrawOt(dest)
		})
		// The OTs' words as offsets.
		for k := range ots {
			print("ot", k, " point ", ots[k].Point, " tag ", where(uintptr(unsafe.Pointer(ots[k].Tag))), ":")
			for t := range len(tags[k]) {
				print(" ", where(uintptr(tags[k][t])))
			}
			println()
		}
	}
	// GsSortClear against the table it is given, the next drawn, and a
	// Tag moved by hand.
	yaroze.GsClearOt(0, 0, &ots[0])
	yaroze.GsClearOt(0, 0, &ots[1])
	yaroze.GsClearOt(0, 0, &ots[2])
	yaroze.GsSortGLine(&lines[0], &ots[1], 3)
	yaroze.GsSortGLine(&lines[1], &ots[2], 0)
	before := off(yaroze.GsGetWorkBase())
	yaroze.GsSortClear(10, 20, 30, &ots[2])
	println("clear work", off(yaroze.GsGetWorkBase())-before, "tag", where(uintptr(unsafe.Pointer(ots[2].Tag))))
	step(func() { yaroze.GsDrawOt(&ots[1]) })
	step(func() { yaroze.GsDrawOt(&ots[2]) })
	step(func() { yaroze.GsDrawOt(&ots[1]) })
	// A clear in each buffer's table, a swap between, and a line sorted
	// into the head entry after the clear.
	yaroze.GsClearOt(0, 0, &ots[0])
	yaroze.GsClearOt(0, 0, &ots[1])
	yaroze.GsSortGLine(&lines[0], &ots[0], 3)
	yaroze.GsSortGLine(&lines[1], &ots[1], 3)
	yaroze.GsSortClear(40, 50, 60, &ots[0])
	yaroze.GsSortGLine(&lines[3], &ots[0], 15)
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsSortClear(70, 80, 90, &ots[1])
	step(func() { yaroze.GsDrawOt(&ots[0]) })
	step(func() { yaroze.GsDrawOt(&ots[1]) })
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsClearOt(0, 0, &ots[1])
	yaroze.GsSortClear(1, 1, 1, &ots[1])
	step(func() { yaroze.GsDrawOt(&ots[0]) })
	step(func() { yaroze.GsDrawOt(&ots[1]) })
	yaroze.GsClearOt(0, 0, &ots[0])
	yaroze.GsClearOt(0, 0, &ots[1])
	yaroze.GsSortGLine(&lines[0], &ots[0], 3)
	yaroze.GsSortGLine(&lines[1], &ots[0], 12)
	ots[0].Tag = &tags[0][10]
	ots[0].Point = 7
	yaroze.GsSortOt(&ots[0], &ots[1])
	print("moved tag:")
	for t := range 16 {
		print(" ", where(uintptr(tags[1][t])))
	}
	println()
	step(func() { yaroze.GsDrawOt(&ots[1]) })
	println("done")
	for {
		yaroze.VSync(0)
	}
}

// where names the address a (its low 24 bits): a packet's or a tag's
// offset, end for the end of a list, or ? for anything else.
func where(a uintptr) string {
	a &= 0xffffff
	if a == 0xffffff {
		return "end"
	}
	base := uintptr(unsafe.Pointer(&packets[0][0])) & 0xffffff
	if a >= base && a < base+uintptr(len(packets[0]))*8 {
		return "p" + itoa(int(a-base))
	}
	for k := range tags {
		b := uintptr(unsafe.Pointer(&tags[k][0])) & 0xffffff
		if a >= b && a < b+uintptr(len(tags[k]))*4 {
			return "t" + itoa(k) + "." + itoa(int(a-b)/4)
		}
	}
	return "?"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
