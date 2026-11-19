//go:build yaroze

// vsynccount shows where in the vertical blank interrupt the library counts
// the blank, for spec 45, on gasgar's 640 by 480 interlaced display with the
// controller driver running as the library's start leaves it.
//
// Its VSyncCallback adds 1 to a word in the scratchpad (1F800300h) and
// writes what VSync(-1) gives there to 1F800308h, and the
// program, after "measure", reads VSync(-1) in a loop and writes each new
// value to the scratchpad (1F800304h), with timer 1 (lines) when it saw it.
// probetest's TestVSyncCount watches those writes, the interrupt and the
// controller port from outside. The program prints the lines between the
// counts it saw, as statlines does, by the parity of the count: on a console
// spec 42 saw 263 and 262 on Sony's library and 274 and 251 on the clean one.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer1  = 0x1f801110
	scratch = 0x1f800300
	counts  = 120
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func store(addr uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(addr)), v) }

// tick counts the callbacks, and keeps what VSync(-1) gives in the callback,
// to tell whether the blank is counted before the callback or after it.
func tick() {
	store(scratch, load(scratch)+1)
	store(scratch+8, uint32(yaroze.VSync(-1)))
}

var lines [counts]uint16

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	store(scratch, 0)
	cb := tick
	yaroze.VSyncCallback((*[2]uintptr)(unsafe.Pointer(&cb))[1])
	yaroze.VSync(0)
	yaroze.VSync(0)
	println("measure")
	last := uint32(yaroze.VSync(-1))
	store(scratch+4, last)
	prev := load(timer1) & 0xffff
	for n := 0; n < counts; {
		c := uint32(yaroze.VSync(-1))
		if c == last {
			continue
		}
		store(scratch+4, c)
		now := load(timer1) & 0xffff
		lines[n] = uint16((now - prev) & 0xffff)
		prev = now
		last = c
		n++
	}
	yaroze.VSyncCallback(0)
	// The first interval runs from before the first count seen, so it is left out.
	var least, most, sum, seen [2]uint32
	least = [2]uint32{0xffff, 0xffff}
	for i := 1; i < counts; i++ {
		p, l := i&1, uint32(lines[i])
		least[p] = min(least[p], l)
		most[p] = max(most[p], l)
		sum[p] += l
		seen[p]++
	}
	for p := range 2 {
		println("VCOUNT parity", p, "least", least[p], "most", most[p], "mean", sum[p]/seen[p])
	}
	println("callbacks", load(scratch), "; VSync(-1) in the last", load(scratch+8), "; after", last)
	println("END")
}
