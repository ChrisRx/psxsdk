//go:build yaroze

// imgrace looks for the console hang that buildup found inside LoadImage
// (spec 43): each frame draws a large box with GsDrawOt and then makes
// eight LoadImage calls of 16 by 16, as buildup's stage 2 does, but first
// waits a number of loop passes that grows by gapStep each frame, so that
// the calls land at every point of the drawing and of the queue's images
// going out. The frame and the gap go to the serial port every 60 frames;
// a hang stops them. A ring of the frame and the call, as buildup's, is in
// the scratchpad. It returns, so it ends on Sony's library too.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	frames   = 3000
	gapStep  = 3
	gapLimit = 6000
	ringAddr = 0x1f800000
)

var (
	ot      yaroze.GsOT
	otTags  [4]yaroze.GsOT_TAG
	packets [1024]uint32
	box     yaroze.GsBOXF
	pixels  [8][16 * 16 / 2]uint32
	spin    uint32
)

func word(a uintptr) *uint32 { return (*uint32)(yaroze.Pointer(a)) }

func mark(f, gap, call int) {
	volatile.StoreUint32(word(ringAddr+4), uint32(f)<<16|uint32(call))
	volatile.StoreUint32(word(ringAddr+8), uint32(gap))
	volatile.StoreUint32(word(ringAddr+12), volatile.LoadUint32(word(0x1f8010a8)))
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 2
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	volatile.StoreUint32(word(ringAddr), 0x45434152) // "RACE"
	for f := range frames {
		gap := f * gapStep % gapLimit
		if f%60 == 0 {
			println("frame", f, "gap", gap)
		}
		mark(f, gap, 1)
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		box = yaroze.GsBOXF{W: 640, H: 240, R: uint8(f), G: 60, B: 90}
		yaroze.GsSortBoxFill(&box, &ot, 1)
		mark(f, gap, 2)
		yaroze.GsDrawOt(&ot)
		for range gap {
			volatile.StoreUint32(&spin, volatile.LoadUint32(&spin)+1)
		}
		for i := range 8 {
			r := yaroze.RECT{X: int16(704 + 16*(i%2)), Y: int16(16 * (i / 2)), W: 16, H: 16}
			mark(f, gap, 16+i)
			yaroze.LoadImage(&r, &pixels[i][0])
		}
		mark(f, gap, 3)
	}
	yaroze.DrawSync(0)
	println("END")
}
