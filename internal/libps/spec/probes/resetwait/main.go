//go:build yaroze

// Command resetwait is a probe for the spec: when ResetGraph(0) waits
// before resetting the GPU. Each case writes a marker, GP1 10000000h plus
// the case number, then does something and calls ResetGraph(0); the
// distance from the marker to the reset's GP1 00h, in the GPU stream with
// cycle counts, is the wait.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000000|n)
}

func main() {
	mark(1)
	yaroze.ResetGraph(0) // the first
	mark(2)
	yaroze.ResetGraph(0) // again at once
	mark(3)
	yaroze.VSync(0)
	yaroze.ResetGraph(0) // after a VSync
	mark(4)
	yaroze.PadInit()
	mark(5)
	yaroze.ResetGraph(0) // after PadInit
	mark(6)
	yaroze.ResetGraph(0) // again
	mark(7)
	yaroze.ResetGraph(1)
	mark(8)
	yaroze.ResetGraph(3)
	mark(9)
	yaroze.ResetGraph(0)
	mark(10)
	println("done")
	for {
		yaroze.VSync(0)
	}
}
