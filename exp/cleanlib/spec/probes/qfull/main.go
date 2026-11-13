//go:build yaroze

// qfull times MoveImage and ClearImage called with the drawing queue full
// (spec 55): behind a clear and a 640 by 480 box it queues 63 LoadImage
// calls, as qlen does, and then makes the one call, which waits in the
// library until the queue has room. The call lies between GP1 marker words
// (10000200h plus a number) for a bus trace (PROBE_BUS=start): the marker
// before the call of round k is 4k, the one after it 4k+1. Even rounds call
// MoveImage and odd ones ClearImage, on a rectangle. It prints the last
// queued LoadImage's time in eighths of the system clock (timer 2), the
// call's time in eighths ("took") and in whole cycles by timer 0, which
// wraps at 65536 ("fine"), the same pair from GsDrawOt's return to the
// call's ("all", "allfine"), which is the queue's whole time whatever the
// calls that fill it take, and DrawSync(1) after the call. The eighths wrap
// at 524288 cycles, and a call here takes more.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer0Value = 0x1f801100
	timer0Mode  = 0x1f801104
	timer2Value = 0x1f801120
	timer2Mode  = 0x1f801124
	gp1         = 0x1f801814
	fill        = 63
	rounds      = 8
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [16384]uint32
	box     yaroze.GsBOXF
	pixels  [fill][16 * 16 / 2]uint32
	last    [rounds]uint32
	coarse  [rounds]uint32
	fine    [rounds]uint32
	all     [rounds]uint32
	allFine [rounds]uint32
	sync    [rounds]int32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func store(addr uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(addr)), v) }

func main() {
	println("start")
	store(timer0Mode, 0)
	store(timer2Mode, 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	for k := range uint32(rounds) {
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		box = yaroze.GsBOXF{Attribute: 1 << 30, W: 640, H: 480, R: 20, G: 40, B: 90}
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
		d2 := load(timer2Value)
		d0 := load(timer0Value)
		for i := range fill {
			r := yaroze.RECT{X: int16(640 + 16*(i%8)), Y: int16(16 * (i / 8)), W: 16, H: 16}
			t := load(timer2Value)
			yaroze.LoadImage(&r, &pixels[i][0])
			last[k] = (load(timer2Value) - t) & 0xffff
		}
		r := yaroze.RECT{X: 641, Y: 256, W: 16, H: 16}
		store(gp1, 0x10000200|4*k)
		t2 := load(timer2Value)
		t0 := load(timer0Value)
		if k%2 == 0 {
			yaroze.MoveImage(&r, 700, 300)
		} else {
			yaroze.ClearImage(&r, 1, 2, 3)
		}
		e0 := load(timer0Value)
		e2 := load(timer2Value)
		fine[k] = (e0 - t0) & 0xffff
		coarse[k] = (e2 - t2) & 0xffff
		allFine[k] = (e0 - d0) & 0xffff
		all[k] = (e2 - d2) & 0xffff
		store(gp1, 0x10000200|4*k|1)
		sync[k] = yaroze.DrawSync(1)
		yaroze.DrawSync(0)
	}
	for k := range rounds {
		println("QFULL", k, "last", last[k], "took", coarse[k], "fine", fine[k], "all", all[k], "allfine", allFine[k], "sync", sync[k])
	}
	println("END")
}
