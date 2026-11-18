//go:build yaroze

// gpustuck asks whether the library's waits for the GPU give up (spec 43).
// With the emulator's drawing time on (PROBE_DRAWTIME=1) the probe keeps
// the GPU busy for a long time by writing large rectangles to GP0 itself,
// about 15 ms each, and then makes one call that has to wait for the GPU:
// DrawSync(0), GsDrawOt of a list with the queue empty and channel 2
// idle, and LoadImage. For each it prints how many lines (timer 1) and
// vertical blanks the call took, whether the GPU was still busy when it
// returned, and GPUSTAT and D2_CHCR then. A call that returns with the GPU
// still busy has given up waiting. Anything the library prints shows in
// the TTY between the lines.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	gp0     = 0x1f801810
	gpuStat = 0x1f801814
	d2CHCR  = 0x1f8010a8
	timer1  = 0x1f801110
)

var (
	ot      yaroze.GsOT
	tags    [4]yaroze.GsOT_TAG
	packets [256]uint32
	pix     [16 * 16 / 2]uint32
)

func load(a uintptr) uint32     { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }
func store(a uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(a)), v) }

// busy writes n rectangles of 1023 by 511 to GP0, each after the GPU
// takes a command word, which under the drawing time it does at once.
func busy(n int) {
	for range n {
		store(gp0, 0x60204060)
		store(gp0, 0)
		store(gp0, 511<<16|1023)
	}
}

// lines is timer 1 counted on past its 16 bits: called often enough.
var hi, last uint32

func lines() uint32 {
	t := load(timer1) & 0xffff
	if t < last {
		hi += 0x10000
	}
	last = t
	return hi | t
}

func report(name string, l0, b0 uint32) {
	l, b := lines()-l0, uint32(yaroze.VSync(-1))-b0
	stat, chcr := load(gpuStat), load(d2CHCR)
	println(name, "lines", l, "blanks", b, "ready", stat>>26&1, "stat", stat, "chcr", chcr)
	// Let the GPU finish before the next case.
	for load(gpuStat)&(1<<26) == 0 {
		lines()
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 2
	ot.Org = &tags[0]
	yaroze.SetDispMask(1)
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	for _, n := range []int{10, 60, 200} {
		println("rects", n)
		lines()
		busy(n)
		l0, b0 := lines(), uint32(yaroze.VSync(-1))
		yaroze.DrawSync(0)
		report("DrawSync", l0, b0)

		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		lines()
		busy(n)
		l0, b0 = lines(), uint32(yaroze.VSync(-1))
		yaroze.GsDrawOt(&ot)
		report("GsDrawOt", l0, b0)
		yaroze.DrawSync(0)

		r := yaroze.RECT{X: 704, Y: 0, W: 16, H: 16}
		lines()
		busy(n)
		l0, b0 = lines(), uint32(yaroze.VSync(-1))
		yaroze.LoadImage(&r, &pix[0])
		report("LoadImage", l0, b0)
		yaroze.DrawSync(0)
	}
	println("END")
}
