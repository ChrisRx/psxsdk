//go:build yaroze

// tealwait is teal with the program waiting waitLines lines after each
// VSync(0) before it starts the frame, as Sony's VSync(0) is seen to return
// about 11 lines into the vertical blank on an interlaced display
// (vsyncphase) and the clean library's at its start. It runs 60 seconds
// with the wait, then 30 without, then prints END and returns.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	waitLines = 12
	timer1    = 0x1f801110
)

var (
	ot   yaroze.GsOT
	tags [1 << 4]yaroze.GsOT_TAG
	pk   [1024]uint32
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
	for f := 0; f < 90*60; f++ {
		wait := f < 60*60
		if f%600 == 0 {
			println("frame", f, "wait", wait)
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
		yaroze.GsDrawOt(&ot)
	}
	println("END")
}
