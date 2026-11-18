//go:build yaroze

// qlen finds how many requests the library's drawing queue takes: behind a
// long drawing, it calls LoadImage until one call takes long, the queue
// being full, and prints each call's time and DrawSync(1).
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	n          = 80
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [16384]uint32
	box     yaroze.GsBOXF
	pixels  [n][16 * 16 / 2]uint32
	took    [n]uint32
	sync    [n]int32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func main() {
	println("start")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	for k := range 2 {
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		box = yaroze.GsBOXF{Attribute: 1 << 30, W: 640, H: 480, R: 20, G: 40, B: 90}
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
		for i := range n {
			r := yaroze.RECT{X: int16(640 + 16*(i%8)), Y: int16(16 * (i / 8)), W: 16, H: 16}
			t := load(timerValue) & 0xffff
			yaroze.LoadImage(&r, &pixels[i][0])
			took[i] = (load(timerValue) - t) & 0xffff
			sync[i] = yaroze.DrawSync(1)
		}
		yaroze.DrawSync(0)
		print("QLEN run ", k)
		for i := range n {
			print(" ", i, ":", took[i], "/", sync[i])
		}
		println()
	}
	println("END")
}
