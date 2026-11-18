//go:build yaroze

// tealbox is tealwait with a picture that changes: one 64 by 64 box moving
// across the middle of the screen over the teal clear. A field that the
// frame's drawing misses keeps older pictures, which show as trails or
// flashing. It runs 30 seconds with the frame started as soon as VSync(0)
// returns, 30 seconds waiting waitLines lines after it first, and 30
// seconds without again, then prints END and returns.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	waitLines = 12
	timer1    = 0x1f801110
	part      = 30 * 60
)

var (
	ot   yaroze.GsOT
	tags [1 << 4]yaroze.GsOT_TAG
	pk   [1024]uint32
	box  = yaroze.GsBOXF{X: 0, Y: 208, W: 64, H: 64, R: 240, G: 140, B: 20}
)

func lines() uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(timer1))) & 0xffff }

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &tags[0]
	yaroze.SetDispMask(1)
	println("setup done")
	for f := 0; f < 3*part; f++ {
		wait := f/part == 1
		if f%part == 0 {
			println("part", f/part, "wait", wait)
		}
		yaroze.VSync(0)
		if wait {
			start := lines()
			for (lines()-start)&0xffff < waitLines {
			}
		}
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&pk[0]))
		yaroze.GsSortClear(0, 128, 128, &ot)
		box.X += 4
		if box.X > 576 {
			box.X = 0
		}
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
	}
	println("END")
}
