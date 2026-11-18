//go:build yaroze

// Command gspsd is a probe for spec 19: PSDIDX and PSDOFSX and PSDOFSY across the calls
// that move them, and whether a program's write to PSDIDX steers GsGetActiveBuff and
// the next GsSwapDispBuff.
package main

import "go.chrisrx.dev/psx/yaroze"

//go:extern PSDIDX
var psdIDX int32

//go:extern PSDOFSX
var psdOFSX int32

//go:extern PSDOFSY
var psdOFSY int32

func line(what string) {
	println(what, "idx", psdIDX, "ofs", psdOFSX, psdOFSY, "active", yaroze.GsGetActiveBuff(), "disp", yaroze.GsDISPENV().Disp.X, yaroze.GsDISPENV().Disp.Y, "clip", yaroze.GsDRAWENV().Clip.X, yaroze.GsDRAWENV().Clip.Y)
}

func main() {
	yaroze.ResetGraph(0)
	line("ResetGraph")
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	line("GsInitGraph")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	line("GsDefDispBuff")
	yaroze.GsInit3D()
	line("GsInit3D")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsDefDispBuff(16, 8, 32, 300)
	line("GsDefDispBuff moved")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	psdIDX = 1
	line("set 1")
	yaroze.GsSwapDispBuff()
	line("swap")
	yaroze.GsSwapDispBuff()
	line("swap")
	psdIDX = 0
	line("set 0")
	yaroze.GsSwapDispBuff()
	line("swap")
	psdOFSX, psdOFSY = 100, 200
	line("set ofs")
	yaroze.GsSwapDispBuff()
	line("swap")
	println("END")
	for {
	}
}
