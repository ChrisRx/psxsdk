//go:build yaroze

// Command vsynclate is a probe for spec 01: VSync(n) called d lines after a
// VSync(0), for delays on both sides of the point where n blanks have passed. Each
// line gives n, d, the call's return, the blanks from the VSync(0) to the call and
// to the return, and the lines the call took. The table runs on the start-up's
// display and again on an interlaced one.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var lines = (*uint32)(unsafe.Pointer(uintptr(0x1f801110)))

func now() uint32 { return volatile.LoadUint32(lines) & 0xffff }

func table() {
	for _, n := range []int32{0, 1, 2, 3, 4, 8} {
		for _, d := range []uint32{0, 20, 130, 240, 262, 270, 400, 520, 530, 700, 790, 800, 1060, 1500, 2000, 2200} {
			yaroze.VSync(0)
			from := uint32(yaroze.VSync(-1))
			start := now()
			for (now()-start)&0xffff < d {
			}
			at := uint32(yaroze.VSync(-1)) - from
			before := now()
			r := yaroze.VSync(n)
			took := (now() - before) & 0xffff
			println("n", n, "d", d, "return", r, "blanks", at, uint32(yaroze.VSync(-1))-from, "took", took)
		}
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.VSync(0)
	table()
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	println("interlaced")
	yaroze.VSync(0)
	table()
	println("END")
}
