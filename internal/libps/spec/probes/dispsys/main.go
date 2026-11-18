//go:build yaroze

// Command dispsys is a probe for the spec: PutDispEnv over display
// environments of each width, height, depth and screen area, in NTSC and
// PAL, and a GsSwapDispBuff after; SetDispMask; VSyncCallback; and
// FlushCache around code changed in memory. Each case goes between GP1
// marker words (10000100h plus the case), so the GPU stream shows what it
// sends; the TTY carries the rest.
package main

import (
	"runtime/volatile"
	"unsafe"

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
	n++
}

var envs = []yaroze.DISPENV{
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 240, W: 320, H: 240}},
	{Disp: yaroze.RECT{X: 64, Y: 16, W: 256, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 255, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 257, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 321, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 368, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 384, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 385, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 512, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 513, H: 240}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 640, H: 480}, Isinter: 1},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 640, H: 480}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 640, H: 256}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 640, H: 257}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 0, H: 0}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 1024, H: 512}},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}, Isrgb24: 1},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}, Isinter: 1},
	{Disp: yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}, Isinter: 2, Isrgb24: 3},
	{Disp: yaroze.RECT{X: 1023, Y: 511, W: 320, H: 240}},
	{Disp: yaroze.RECT{X: -1, Y: -1, W: 320, H: 240}},
	{Disp: yaroze.RECT{X: 1025, Y: 513, W: 320, H: 240}},
	{Disp: yaroze.RECT{X: 3, Y: 5, W: 320, H: 240}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 256, H: 240}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 10, Y: 16, W: 0, H: 0}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: -10, Y: -16, W: 100, H: 100}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 300, H: 300}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 300, Y: 300, W: 10, H: 10}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 24, W: 256, H: 240}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 1, H: 1}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: -5, H: -5}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 1000, H: 1000}},
	{Disp: yaroze.RECT{W: 640, H: 480}, Screen: yaroze.RECT{X: 0, Y: 0, W: 256, H: 480}, Isinter: 1},
	{Disp: yaroze.RECT{W: 320, H: 240}, Pad0: 1, Pad1: 2},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 2, H: 2}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 3, H: 3}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 4, H: 4}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 7, H: 5}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 8, H: 6}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 9, H: 7}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 16, H: 8}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{W: 17, H: 9}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: -61, Y: -1, W: 256, H: 240}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: -100, Y: -100, W: 256, H: 240}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: -100, Y: -100, W: 50, H: 50}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 250, Y: 240, W: 6, H: 2}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 260, Y: 241, W: 0, H: 0}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 268, Y: 280, W: 1, H: 1}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 268, H: 242}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 269, H: 243}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: 0, Y: 0, W: 0x7fff, H: 0x7fff}},
	{Disp: yaroze.RECT{W: 320, H: 240}, Screen: yaroze.RECT{X: -0x8000, Y: -0x8000, W: -0x8000, H: -0x8000}},
	{Disp: yaroze.RECT{W: 320, H: 240}},
	{Disp: yaroze.RECT{W: 280, H: 240}},
	{Disp: yaroze.RECT{W: 281, H: 240}},
	{Disp: yaroze.RECT{W: 352, H: 240}},
	{Disp: yaroze.RECT{W: 353, H: 240}},
	{Disp: yaroze.RECT{W: 400, H: 240}},
	{Disp: yaroze.RECT{W: 401, H: 240}},
	{Disp: yaroze.RECT{W: 560, H: 240}},
	{Disp: yaroze.RECT{W: 561, H: 240}},
	{Disp: yaroze.RECT{W: -1, H: -1}},
	{Disp: yaroze.RECT{W: 320, H: 256}},
	{Disp: yaroze.RECT{W: 320, H: 288}},
	{Disp: yaroze.RECT{W: 320, H: 240}},
}

var ticks uint32

// tick is the vertical blank callback.
func tick() {
	volatile.StoreUint32(&ticks, volatile.LoadUint32(&ticks)+1)
}

// addr is the address of a top-level function's code.
func addr(f func()) uintptr {
	return (*[2]uintptr)(unsafe.Pointer(&f))[1]
}

// code is a function written into memory: it returns the constant in its
// second word, an li in the delay slot.
var code [4]uint32

func callCode() int32 {
	var f func() int32
	(*[2]uintptr)(unsafe.Pointer(&f))[1] = uintptr(unsafe.Pointer(&code[0]))
	return f()
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	for mode := range int32(2) {
		yaroze.SetVideoMode(mode)
		for i := range envs {
			e := envs[i]
			var r *yaroze.DISPENV
			step(func() { r = yaroze.PutDispEnv(&e) })
			d := yaroze.GsDISPENV()
			println("put", i, "ret", r == &e, r == nil, "gs", d.Disp.X, d.Disp.Y, d.Disp.W, d.Disp.H, d.Screen.X, d.Screen.Y, d.Screen.W, d.Screen.H)
		}
		step(func() { yaroze.GsSwapDispBuff() })
		step(func() { yaroze.GsSwapDispBuff() })
		e := yaroze.DISPENV{Disp: yaroze.RECT{X: 100, Y: 100, W: 320, H: 240}}
		step(func() { yaroze.PutDispEnv(&e) })
		step(func() { yaroze.GsSwapDispBuff() })
		step(func() { yaroze.GsSwapDispBuff() })
	}
	// The same environment across a change of video mode.
	e := yaroze.DISPENV{Disp: yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}}
	step(func() { yaroze.PutDispEnv(&e) })
	yaroze.SetVideoMode(0)
	step(func() { yaroze.PutDispEnv(&e) })
	yaroze.SetVideoMode(1)
	step(func() { yaroze.PutDispEnv(&e) })
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.SetVideoMode(0)
	step(func() { yaroze.GsSwapDispBuff() })
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.SetVideoMode(1)
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.SetVideoMode(0)
	step(func() { yaroze.GsSwapDispBuff() })
	// GsDISPENV changed by the program between swaps.
	yaroze.GsDISPENV().Screen = yaroze.RECT{X: 4, Y: 4, W: 200, H: 200}
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsDISPENV().Isrgb24 = 1
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsDISPENV().Isrgb24 = 0
	yaroze.GsDISPENV().Pad1 = 9
	step(func() { yaroze.GsSwapDispBuff() })
	step(func() { yaroze.GsSwapDispBuff() })
	yaroze.GsDISPENV().Disp.W = 256
	step(func() { yaroze.GsSwapDispBuff() })
	for _, m := range []int32{0, 1, 0, 2, 3, -1, 0x100, 1} {
		step(func() { yaroze.SetDispMask(m) })
	}

	// VSyncCallback.
	yaroze.VSync(0)
	println("ticks before", ticks)
	yaroze.VSyncCallback(addr(tick))
	for i := range 5 {
		yaroze.VSync(0)
		println("vsync", i, "ticks", volatile.LoadUint32(&ticks))
	}
	yaroze.VSync(3)
	println("vsync 3 ticks", volatile.LoadUint32(&ticks))
	yaroze.VSyncCallback(0)
	t := volatile.LoadUint32(&ticks)
	for range 4 {
		yaroze.VSync(0)
	}
	println("removed, ticks since", volatile.LoadUint32(&ticks)-t)
	yaroze.VSyncCallback(addr(tick))
	t = volatile.LoadUint32(&ticks)
	for range 4 {
		yaroze.VSync(0)
	}
	println("again, ticks since", volatile.LoadUint32(&ticks)-t)
	// A long critical section: the blanks it covers.
	yaroze.EnterCriticalSection()
	t = volatile.LoadUint32(&ticks)
	for range 400000 {
		volatile.LoadUint32(&ticks)
	}
	println("critical, ticks", volatile.LoadUint32(&ticks)-t)
	yaroze.ExitCriticalSection()
	yaroze.VSync(0)
	yaroze.VSyncCallback(0)

	// FlushCache: code changed in memory, called before and after.
	code = [4]uint32{0x03e00008, 0x24020001, 0, 0} // jr ra; li v0, 1
	yaroze.FlushCache()
	println("code", callCode())
	code[1] = 0x24020002
	println("unflushed", callCode())
	yaroze.FlushCache()
	println("flushed", callCode())
	code[1] = 0x24020003
	yaroze.FlushCache()
	println("flushed again", callCode())
	println("done")
	for {
		yaroze.VSync(0)
	}
}
