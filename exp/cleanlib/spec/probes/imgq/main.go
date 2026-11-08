//go:build yaroze

// Command imgq is a probe for the spec (39), run on a console under either
// library: whether LoadImage and StoreImage wait for drawing already queued.
// It starts a slow frame with GsDrawOt, then calls the image call at once,
// timing it with timer 2 at an eighth of the system clock. Then it reads the
// image's area back: the boxes cover it, so pixels unchanged mean the image
// went in after the drawing. watch shows what starts a queued image.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

// Timer 2, counting an eighth of the system clock once its mode is 200h:
// 15.8 ms before the 16-bit count wraps.
const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
)

func ticks() uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(timerValue))) & 0xffff }

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [8192]uint32
	boxes   [boxCount]yaroze.GsBOXF
	image   [16 * 16 / 2]uint32
	back    [16 * 16 / 2]uint32
	probe   [1]uint32
)

const boxCount = 4

func run(withImage bool) {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	for i := range boxes {
		boxes[i] = yaroze.GsBOXF{Attribute: 1 << 30, X: 0, Y: 0, W: 320, H: 240, R: uint8(10 + i), G: 40, B: 90}
		yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
	}
	rect := yaroze.RECT{X: 100, Y: 100, W: 16, H: 16}
	for i := range image {
		image[i] = 0x7c1f7c1f // magenta, both pixels
	}
	full := yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}
	yaroze.ClearImage(&full, 0, 0, 0)
	yaroze.DrawSync(0)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	t0 := ticks()
	yaroze.GsDrawOt(&ot)
	t1 := ticks()
	stat := volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801814)))
	n1 := yaroze.DrawSync(1)
	var t2 uint32
	n2 := int32(-1)
	if withImage {
		yaroze.LoadImage(&rect, &image[0])
		t2 = ticks()
		n2 = yaroze.DrawSync(1)
	}
	yaroze.DrawSync(0)
	t3 := ticks()
	yaroze.StoreImage(&rect, &back[0])
	yaroze.DrawSync(0)
	spot := yaroze.RECT{X: 10, Y: 10, W: 2, H: 1}
	yaroze.StoreImage(&spot, &probe[0])
	yaroze.DrawSync(0)
	println("box pixel", probe[0], "GPUSTAT after GsDrawOt", stat)
	same := 0
	for i := range back {
		if back[i] == image[i] {
			same++
		}
	}
	if withImage {
		println("IMG drawot", (t1-t0)&0xffff, "loadimage", (t2-t1)&0xffff, "total", (t3-t0)&0xffff, "eighths; DrawSync(1)", n1, "then", n2, "; pixel pairs unchanged", same, "of", len(back))
	} else {
		println("NOIMG drawot", (t1-t0)&0xffff, "total", (t3-t0)&0xffff, "eighths; DrawSync(1)", n1)
	}
}

var (
	during [16 * 16 / 2]uint32
	after  [16 * 16 / 2]uint32
)

// store calls StoreImage on the image's area while the slow frame draws. If
// it's queued behind the drawing, what it returns is the finished picture,
// the same as a read once everything is done.
func store() {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	for i := range boxes {
		yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
	}
	rect := yaroze.RECT{X: 100, Y: 100, W: 16, H: 16}
	full := yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}
	yaroze.ClearImage(&full, 0, 0, 0)
	yaroze.DrawSync(0)
	for i := range during {
		during[i] = 0x12345678
	}
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	t0 := ticks()
	yaroze.GsDrawOt(&ot)
	t1 := ticks()
	n1 := yaroze.DrawSync(1)
	yaroze.StoreImage(&rect, &during[0])
	t2 := ticks()
	n2 := yaroze.DrawSync(1)
	untouched := 0
	for i := range during {
		if volatile.LoadUint32(&during[i]) == 0x12345678 {
			untouched++
		}
	}
	yaroze.DrawSync(0)
	t3 := ticks()
	yaroze.StoreImage(&rect, &after[0])
	yaroze.DrawSync(0)
	same := 0
	for i := range during {
		if during[i] == after[i] {
			same++
		}
	}
	println("STORE drawot", (t1-t0)&0xffff, "storeimage", (t2-t1)&0xffff, "total", (t3-t0)&0xffff, "eighths; DrawSync(1)", n1, "then", n2, "; words not yet written on return", untouched, "; pairs equal to the finished picture", same, "of", len(during))
}

var (
	big     [64 * 64 / 2]uint32
	events  [32][3]uint32
	nevents int
)

// watch starts the slow frame and a 64 by 64 LoadImage behind it, then
// calls nothing in the library for about 31 ms, logging each change of
// GPUSTAT's ready bit and D2_CHCR's busy bit with the time. That shows
// whether the queued image starts on its own when the drawing ends.
func watch(name string, poke func()) {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	for i := range boxes {
		yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
	}
	rect := yaroze.RECT{X: 100, Y: 100, W: 64, H: 64}
	yaroze.DrawSync(0)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	t0 := ticks()
	yaroze.GsDrawOt(&ot)
	yaroze.LoadImage(&rect, &big[0])
	nevents = 0
	last := ^uint32(0)
	prev, wraps := t0, uint32(0)
	poked := poke == nil
	lastT := uint32(0)
	for {
		now := ticks()
		if now < prev {
			wraps++
		}
		prev = now
		t := wraps<<16 + now - t0
		if t > 130000 {
			break
		}
		if t-lastT > 100 && lastT != 0 && nevents < len(events) {
			events[nevents] = [3]uint32{lastT, 8, t - lastT}
			nevents++
		}
		lastT = t
		if !poked && t > 40000 {
			poked = true
			if nevents < len(events) {
				events[nevents] = [3]uint32{t, 9, 9}
				nevents++
			}
			poke()
		}
		stat := volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801814))) & (1 << 26)
		chcr := volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f8010a8))) & (1 << 24)
		if s := stat | chcr; s != last && nevents < len(events) {
			events[nevents] = [3]uint32{t, stat >> 26, chcr >> 24}
			nevents++
			last = s
		}
	}
	yaroze.DrawSync(0)
	for i := range nevents {
		if events[i][1] == 8 {
			println("WATCH", name, "at", events[i][0], "the CPU was away for", events[i][2])
			continue
		}
		if events[i][1] == 9 {
			println("WATCH", name, "at", events[i][0], "calls it")
			continue
		}
		println("WATCH", name, "at", events[i][0], "ready", events[i][1], "dma busy", events[i][2])
	}
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
	run(false)
	run(true)
	run(true)
	store()
	store()
	watch("nothing", nil)
	watch("DrawSync(1)", func() { yaroze.DrawSync(1) })
	watch("VSync(0)", func() { yaroze.VSync(0) })
	watch("GsGetActiveBuff", func() { yaroze.GsGetActiveBuff() })
	watch("GsClearOt", func() { yaroze.GsClearOt(0, 0, &ot) })
	println("END")
}
