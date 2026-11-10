//go:build yaroze

// synccount follows DrawSync(1) through a frame with images queued behind
// it, and after a LoadImage that found the GPU free, for spec 45, with the
// drawing time on (PROBE_DRAWTIME). In qlen (spec 41), Sony's DrawSync(1)
// counts 1 after the first image queued behind a drawing and one more for
// each after it; the clean one counted one more than that. It prints each
// change of DrawSync(1) as it polls it, with the time in eighths of the
// system clock (timer 2) from the call before, and GPUSTAT bits 26 to 28
// and D2_CHCR's busy bit as the value was read. "trace" comes before the
// part for a bus trace (PROBE_BUS=trace).
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	gpuStat    = 0x1f801814
	d2CHCR     = 0x1f8010a8
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [4096]uint32
	box     yaroze.GsBOXF
	pixels  [4][16 * 16 / 2]uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

type change struct {
	at   uint32
	v    int32
	stat uint32
	busy uint32
}

var changes [64]change

// follow polls DrawSync(1) until it gives 0, at most limit times, keeping each change.
func follow(name string, start uint32) {
	n := 0
	last := int32(-1)
	high, prev := uint32(0), start
	for i := 0; i < 200000 && n < len(changes); i++ {
		v := yaroze.DrawSync(1)
		s := load(gpuStat)
		b := load(d2CHCR) >> 24 & 1
		t := load(timerValue) & 0xffff
		if t < prev {
			high += 0x10000
		}
		prev = t
		if v != last {
			changes[n] = change{high | t - start, v, s >> 26 & 7, b}
			n++
			last = v
		}
		if v == 0 {
			break
		}
	}
	print("SYNC ", name)
	for i := range n {
		c := changes[i]
		print(" ", c.at, ":", c.v, "/", c.stat, c.busy)
	}
	println()
}

func frame(images int) {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsSortClear(0, 0, 0, &ot)
	box = yaroze.GsBOXF{Attribute: 1 << 30, W: 320, H: 240, R: 20, G: 40, B: 90}
	yaroze.GsSortBoxFill(&box, &ot, 1)
	start := load(timerValue) & 0xffff
	yaroze.GsDrawOt(&ot)
	for i := range images {
		r := yaroze.RECT{X: int16(640 + 16*i), Y: 0, W: 16, H: 16}
		yaroze.LoadImage(&r, &pixels[i][0])
	}
	follow("frame+"+string(rune('0'+images)), start)
}

func image() {
	r := yaroze.RECT{X: 640, Y: 64, W: 16, H: 16}
	start := load(timerValue) & 0xffff
	yaroze.LoadImage(&r, &pixels[0][0])
	follow("image", start)
}

func main() {
	println("start")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	for range 2 {
		yaroze.VSync(0)
		frame(0)
		yaroze.VSync(0)
		frame(1)
		yaroze.VSync(0)
		frame(3)
		yaroze.VSync(0)
		image()
	}
	yaroze.VSync(0)
	println("trace")
	frame(3)
	image()
	println("END")
}
