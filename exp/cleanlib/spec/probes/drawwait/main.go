// drawwait measures what GsDrawOt does when it finds the queue empty and
// channel 2 idle while the GPU is still drawing, on a console, under either
// library (spec 40). Its frame is four 320 by 240 semitransparent boxes,
// about 7.4 ms of drawing. Times are in eighths of the system clock, from
// timer 2 in mode 200h, counted from the first GsDrawOt.
//
// tail: the first GsDrawOt, then a spin until its DMA has ended, the GPU
// still drawing the last box, then a second GsDrawOt.
// reset: the first GsDrawOt, then ResetGraph(1) at once, which stops the
// DMA with the GPU in a box, then a second GsDrawOt.
//
// For the second call it prints how long it took, GPUSTAT bit 26 and
// D2_CHCR's busy bit when it returned, DrawSync(1) then, and when after the
// return channel 2 first showed busy, or that it hadn't within 60,000.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	gpuStat    = 0x1f801814
	d2Chcr     = 0x1f8010a8
)

func reg(a uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }

// clock counts eighths across the timer's wraps; it must be read at least
// once every 65,536.
type clock struct{ t0, prev, wraps uint32 }

func (c *clock) start() {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	c.t0 = reg(timerValue) & 0xffff
	c.prev, c.wraps = c.t0, 0
}

func (c *clock) now() uint32 {
	v := reg(timerValue) & 0xffff
	if v < c.prev {
		c.wraps++
	}
	c.prev = v
	return c.wraps<<16 + v - c.t0
}

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [8192]uint32
	boxes   [4]yaroze.GsBOXF
)

func build() {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	for i := range boxes {
		boxes[i] = yaroze.GsBOXF{Attribute: 1 << 30, X: 0, Y: 0, W: 320, H: 240, R: uint8(10 + i), G: 40, B: 90}
		yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
	}
}

func run(name string, reset bool) {
	build()
	yaroze.DrawSync(0)
	var c clock
	c.start()
	yaroze.GsDrawOt(&ot)
	if reset {
		yaroze.ResetGraph(1)
	} else {
		for reg(d2Chcr)&(1<<24) != 0 && c.now() < 60000 {
		}
	}
	t1 := c.now()
	ready1 := reg(gpuStat) >> 26 & 1
	yaroze.GsDrawOt(&ot)
	t2 := c.now()
	ready2 := reg(gpuStat) >> 26 & 1
	busy2 := reg(d2Chcr) >> 24 & 1
	n := yaroze.DrawSync(1)
	started := uint32(0)
	if busy2 == 0 {
		for {
			t := c.now()
			if reg(d2Chcr)&(1<<24) != 0 {
				started = t
				break
			}
			if t-t2 > 60000 {
				break
			}
		}
	}
	yaroze.DrawSync(0)
	t3 := c.now()
	println("WAIT", name, "second call at", t1, "ready", ready1, "took", t2-t1, "; on return ready", ready2, "dma busy", busy2, "DrawSync(1)", n, "; dma started at", started, "; all done at", t3)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("setup done")
	for range 3 {
		run("tail", false)
	}
	for range 3 {
		run("reset", true)
	}
	println("END")
}
