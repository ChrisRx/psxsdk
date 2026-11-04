//go:build yaroze

// Command clearimage is a probe for the spec: ClearImage on rectangles of
// different positions and sizes, each case between two GP1 marker words
// (10000100h plus the case number), so the GPU stream shows the commands
// each one sends.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var rects = []yaroze.RECT{
	{X: 0, Y: 0, W: 16, H: 16},
	{X: 0, Y: 0, W: 320, H: 240},
	{X: 836, Y: 352, W: 1, H: 1},
	{X: 836, Y: 352, W: 16, H: 1},
	{X: 832, Y: 352, W: 16, H: 1},
	{X: 832, Y: 352, W: 1, H: 1},
	{X: 832, Y: 352, W: 15, H: 4},
	{X: 832, Y: 352, W: 17, H: 4},
	{X: 840, Y: 100, W: 8, H: 8},
	{X: 3, Y: 5, W: 64, H: 32},
	{X: 0, Y: 0, W: 0, H: 0},
	{X: 1000, Y: 500, W: 64, H: 32},
	{X: 512, Y: 256, W: 512, H: 256},
	{X: 0, Y: 0, W: 1024, H: 512},
	{X: 0, Y: 0, W: 64, H: 128},
	{X: 0, Y: 0, W: 64, H: 239},
	{X: 0, Y: 0, W: 64, H: 240},
	{X: 0, Y: 0, W: 64, H: 241},
	{X: 0, Y: 0, W: 64, H: 256},
	{X: 0, Y: 0, W: 64, H: 511},
	{X: 0, Y: 0, W: 64, H: 512},
	{X: 0, Y: 0, W: 1, H: 240},
	{X: 0, Y: 0, W: 16, H: 240},
	{X: 0, Y: 0, W: 319, H: 240},
	{X: 0, Y: 0, W: 321, H: 240},
	{X: 0, Y: 0, W: 1023, H: 240},
	{X: 0, Y: 0, W: 1024, H: 240},
	{X: 16, Y: 0, W: 64, H: 240},
	{X: 0, Y: 16, W: 64, H: 240},
	{X: 0, Y: 0, W: 0, H: 16},
	{X: 0, Y: 0, W: 16, H: 0},
	{X: 0, Y: 0, W: 1023, H: 511},
	{X: 0, Y: 0, W: 0, H: 240},
	{X: 5, Y: 7, W: 100, H: 300},
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	for i := range rects {
		mark(uint32(i))
		yaroze.ClearImage(&rects[i], uint8(i*16), 0x40, 0x80)
		yaroze.DrawSync(0)
	}
	mark(0xff)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
