//go:build yaroze

// teal is showcase-gasgar's display with nothing on it: 640 by 480
// interlaced from one buffer, ResetGraph(1) at the top of each frame, and a
// teal clear sent with GsDrawOt. A frame whose clear misses a field shows
// what was in video memory before. It runs two minutes, then prints END and
// returns.
package main

import "go.chrisrx.dev/psx/yaroze"

var (
	ot   yaroze.GsOT
	tags [1 << 4]yaroze.GsOT_TAG
	pk   [1024]uint32
)

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &tags[0]
	yaroze.SetDispMask(1)
	println("setup done")
	for f := 0; f < 2*60*60; f++ {
		if f%600 == 0 {
			println("frame", f, "vsync", yaroze.VSync(-1))
		}
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&pk[0]))
		yaroze.GsSortClear(0, 128, 128, &ot)
		yaroze.GsDrawOt(&ot)
	}
	println("END")
}
