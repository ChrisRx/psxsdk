//go:build yaroze

// Command dispswap is a probe for the spec: when GsSwapDispBuff writes the
// display mode (GP1 08h) after PutDispEnv, in NTSC and PAL. Each test
// starts from GsInitGraph; each call goes between GP1 marker words.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n int) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|uint32(n))
}

var n int

func step(f func()) {
	mark(n)
	f()
	yaroze.DrawSync(0)
	println("pads", n, plain.Pad0, screen.Pad0, moved.Pad0, yaroze.GsDISPENV().Pad0, plain.Pad1, yaroze.GsDISPENV().Pad1)
	n++
}

var (
	plain  = yaroze.DISPENV{Disp: yaroze.RECT{W: 320, H: 240}}
	screen = yaroze.DISPENV{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 2, H: 2}}
	moved  = yaroze.DISPENV{Disp: yaroze.RECT{X: 8, W: 320, H: 240}}
	put    = func(e *yaroze.DISPENV) func() { return func() { yaroze.PutDispEnv(e) } }
	swap   = func() { yaroze.GsSwapDispBuff() }
)

func init3(before, after int32) {
	yaroze.SetVideoMode(before)
	step(func() {
		yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
		yaroze.GsDefDispBuff(0, 0, 0, 240)
	})
	yaroze.SetVideoMode(after)
	d := yaroze.GsDISPENV()
	println("env", d.Disp.X, d.Disp.Y, d.Disp.W, d.Disp.H, d.Screen.X, d.Screen.Y, d.Screen.W, d.Screen.H, d.Isinter, d.Isrgb24, d.Pad0, d.Pad1)
}

func main() {
	yaroze.ResetGraph(0)
	for _, m := range [][2]int32{{0, 0}, {0, 1}, {1, 1}, {1, 0}} {
		init3(m[0], m[1])
		step(swap)
		step(swap)
		step(swap)
		init3(m[0], m[1])
		step(put(&screen))
		step(put(&plain))
		step(swap)
		step(swap)
		init3(m[0], m[1])
		step(put(&plain))
		step(swap)
		init3(m[0], m[1])
		step(put(&moved))
		step(put(&plain))
		step(swap)
		init3(m[0], m[1])
		step(put(&screen))
		step(swap)
		step(swap)
		step(swap)
	}
	yaroze.SetVideoMode(0)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
