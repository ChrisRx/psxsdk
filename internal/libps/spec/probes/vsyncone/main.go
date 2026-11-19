//go:build yaroze

// Command vsyncone is a probe for spec 23: what VSync(1) does to the count of lines
// and the count of blanks later VSync calls start from. Each case is a VSync(0), a wait
// of d lines, then calls, printing each call's return and the blanks from the VSync(0).
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var lines = (*uint32)(unsafe.Pointer(uintptr(0x1f801110)))

func now() uint32 { return volatile.LoadUint32(lines) & 0xffff }

func wait(d uint32) {
	start := now()
	for (now()-start)&0xffff < d {
	}
}

func call(from, n int32) {
	r := yaroze.VSync(n)
	println("  VSync", n, "=", r, "blanks", yaroze.VSync(-1)-from)
}

func start(name string, d uint32) int32 {
	println(name, "d", d)
	yaroze.VSync(0)
	from := yaroze.VSync(-1)
	wait(d)
	return from
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.VSync(0)
	for _, d := range []uint32{0, 40, 100, 240, 300, 700} {
		// Reads in a row, with a wait between two of them.
		from := start("ones", d)
		call(from, 1)
		call(from, 1)
		wait(30)
		call(from, 1)
		// The count after a wait starts at its return.
		call(from, 0)
		call(from, 1)
		wait(30)
		call(from, 1)

		// What a later VSync(n) starts from: the VSync(0) or the VSync(1).
		for _, n := range []int32{2, 3, 4} {
			from = start("then", d)
			call(from, 1)
			call(from, n)
			call(from, 1)
		}

		// A negative mode between: does it move either count?
		from = start("negative", d)
		call(from, -1)
		wait(30)
		call(from, 1)
		call(from, -1)
		call(from, 2)
	}
	println("END")
}
