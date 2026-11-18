//go:build yaroze

// Command palscreen is a probe for the spec: what GsInitGraph leaves in
// GsDISPENV's screen in each video mode and height, and the display range
// a swap writes after the mode changes. It prints the screen after each
// GsInitGraph; the GP1 07h words show in the GPU stream.
package main

import "go.chrisrx.dev/psx/yaroze"

func show(mode int32, w, h uint16, inter uint16) {
	yaroze.SetVideoMode(mode)
	yaroze.GsInitGraph(w, h, inter|yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, h)
	s := yaroze.GsDISPENV().Screen
	println("mode", mode, "w", w, "h", h, "inter", inter, "screen", s.X, s.Y, s.W, s.H)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
}

func main() {
	yaroze.ResetGraph(0)
	for _, m := range []int32{yaroze.MODE_NTSC, yaroze.MODE_PAL} {
		for _, h := range []uint16{200, 240, 256} {
			show(m, 320, h, 0)
		}
		show(m, 640, 480, yaroze.GsINTER)
	}
	// NTSC init, then PAL, as dino does, with the screen moved down 27.
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(640, 240, 0, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.SetVideoMode(yaroze.MODE_PAL)
	yaroze.GsDISPENV().Screen.Y += 27
	for range 3 {
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
	}
	println("dino done")
	for {
		yaroze.VSync(0)
	}
}
