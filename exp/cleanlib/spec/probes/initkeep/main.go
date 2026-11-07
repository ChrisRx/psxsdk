//go:build yaroze

// Command initkeep is a probe for spec 37: what GsInitGraph does to the buffers'
// corners and the buffer drawn into, when it isn't the first call or follows
// GsDefDispBuff. Each line prints PSDIDX, PSDOFSX, PSDOFSY, the active buffer, the
// display's corner and the drawing area's corner and offset.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDIDX
var psdIDX int32

//go:extern PSDOFSX
var psdOFSX int32

//go:extern PSDOFSY
var psdOFSY int32

func line(what string) {
	d, e := yaroze.GsDISPENV(), yaroze.GsDRAWENV()
	println(what, "idx", psdIDX, "ofs", psdOFSX, psdOFSY, "active", yaroze.GsGetActiveBuff(),
		"disp", d.Disp.X, d.Disp.Y, d.Disp.W, d.Disp.H,
		"clip", e.Clip.X, e.Clip.Y, e.Clip.W, e.Clip.H, "offset", e.Ofs[0], e.Ofs[1])
}

func main() {
	yaroze.ResetGraph(0)
	line("start")
	yaroze.GsDefDispBuff(16, 8, 32, 300)
	line("GsDefDispBuff before any GsInitGraph")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	line("first GsInitGraph")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	line("GsDefDispBuff")
	yaroze.GsSwapDispBuff()
	line("swap, active 1")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	line("second GsInitGraph, same size")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsInitGraph(384, 240, yaroze.GsOFSGTE, 1, 0)
	line("third GsInitGraph, other size and mode")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsDefDispBuff(10, 20, 30, 40)
	line("GsDefDispBuff")
	yaroze.ResetGraph(0)
	line("ResetGraph(0)")
	yaroze.ResetGraph(1)
	line("ResetGraph(1)")
	yaroze.GsInit3D()
	line("GsInit3D")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	line("fourth GsInitGraph")
	psdOFSX, psdOFSY = 0x00200010, 0x00400030
	line("set ofs")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	line("fifth GsInitGraph")
	yaroze.GsSwapDispBuff()
	line("swap")
	println("END")
	for {
	}
}
