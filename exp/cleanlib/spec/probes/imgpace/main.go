//go:build yaroze

// imgpace is fieldtime (spec 41) cut to the frames that show how fast
// LoadImage calls queued behind a drawing go out once the GPU has finished
// it, for a bus trace (PROBE_BUS=trace) with the drawing time on: gasgar's
// 640 by 480 interlaced display, ResetGraph(1), a clear and one
// semitransparent 640 by 480 box with GsDrawOt, then n 16 by 16 LoadImage
// calls behind it, and DrawSync(0). It prints, for n of 8 and 32, the time
// from VSync's return to DrawSync(0)'s, in eighths of the system clock. The
// line "trace" comes before the last frame, of 32 images.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	images     = 32
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [16384]uint32
	box     yaroze.GsBOXF
	pixels  [images][16 * 16 / 2]uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

// clock is timer 2 counted on past its 16 bits.
type clock struct{ last, high uint32 }

func (c *clock) now() uint32 {
	t := load(timerValue) & 0xffff
	if t < c.last {
		c.high += 0x10000
	}
	c.last = t
	return c.high | t
}

func frame(n int) uint32 {
	yaroze.VSync(0)
	c := clock{last: load(timerValue) & 0xffff}
	start := c.last
	yaroze.ResetGraph(1)
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsSortClear(0, 0, 0, &ot)
	box = yaroze.GsBOXF{Attribute: 1 << 30, W: 640, H: 480, R: 20, G: 40, B: 90}
	yaroze.GsSortBoxFill(&box, &ot, 1)
	yaroze.GsDrawOt(&ot)
	for i := range n {
		r := yaroze.RECT{X: int16(640 + 16*(i%8)), Y: int16(16 * (i / 8)), W: 16, H: 16}
		yaroze.LoadImage(&r, &pixels[i][0])
		c.now()
	}
	// The wait is 19ms or so, past the timer's wrap: poll it.
	for yaroze.DrawSync(1) != 0 {
		c.now()
	}
	return c.now() - start
}

func main() {
	println("start")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	for _, n := range []int{8, 32, 8, 32} {
		println("PACE images", n, "done at", frame(n))
	}
	println("trace")
	println("PACE images", 32, "done at", frame(32))
	yaroze.DrawSync(0)
	println("END")
}
