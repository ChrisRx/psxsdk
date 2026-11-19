//go:build yaroze

// padcost times, in system clock cycles, the clean library's pads, for
// spec 45: on a console the clean vertical blank interrupt runs some 40
// cycles longer than Sony's before the callback and 50 to 85 after it,
// where the emulator, with its costs as the console's (cpucost to cpucost4),
// has them about equal. The clean handler meets Sony's times with pads:
// spin's loop of a load and a store (pace, spend), unrolled stores (space),
// loads (loadSpace), spaceRun's loop of stores, and libps_delay's run of
// nops; Sony's runs its own code. If a console prices a pad otherwise than
// the emulator, the clean handler drifts there and Sony's doesn't.
//
// The pads here are copies of exp/cleanlib/fidelity.go's, compiled by the
// same compiler; libps_delay's nops are machine code the probe writes. Each
// is timed six times with interrupts off, between two reads of timer 2,
// the first time with its code out of the instruction cache. It prints a
// line a pad: its name, its size, and the six times, then END, and returns.
// "empty" is the two reads alone.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const reps = 6

var (
	spinCount uint32
	spacer    uint32
	runCount  uint32
)

// spin is fidelity.go's.
//
//go:noinline
func spin(n uint32) {
	for range n {
		volatile.StoreUint32(&spinCount, volatile.LoadUint32(&spinCount)+1)
	}
}

func timer() uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(0x1f801120))) }

//go:noinline
func empty() uint32 {
	a := timer()
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func timeSpin(n uint32) uint32 {
	a := timer()
	spin(n)
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func space16() uint32 {
	a := timer()
	for range 16 {
		volatile.StoreUint32(&spacer, 0)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func loadSpace16() uint32 {
	a := timer()
	for range 16 {
		_ = volatile.LoadUint32(&spacer)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func spaceRun() uint32 {
	a := timer()
	for i := volatile.LoadUint32(&runCount); i != 0; i-- {
		volatile.StoreUint32(&spacer, 0)
	}
	b := timer()
	return (b - a) & 0xffff
}

//go:noinline
func space16Load() uint32 {
	a := timer()
	for range 16 {
		volatile.StoreUint32(&spacer, 0)
	}
	_ = volatile.LoadUint32(&spinCount)
	b := timer()
	return (b - a) & 0xffff
}

// sled is libps_delay's shape: a jump into a run of 128 nops, and back.
var sled [4 + 132 + 4]uint32

func sledCode() uintptr {
	i := 0
	for uintptr(unsafe.Pointer(&sled[i]))%16 != 0 {
		i++
	}
	start := i
	for j := 0; j < 128; j++ {
		sled[i] = 0
		i++
	}
	sled[i] = 31<<21 | 0x08 // jr ra
	sled[i+1] = 0
	return uintptr(unsafe.Pointer(&sled[start]))
}

var sledFn func()

//go:noinline
func timeSled() uint32 {
	a := timer()
	sledFn()
	b := timer()
	return (b - a) & 0xffff
}

func show(name string, size int, f func() uint32) {
	var t [reps]uint32
	for i := range t {
		t[i] = f()
	}
	print(name, " ", size, ":")
	for _, x := range t {
		print(" ", x)
	}
	println()
}

func main() {
	println("padcost")
	yaroze.ResetGraph(0)
	p := sledCode()
	fv := (*[2]uintptr)(unsafe.Pointer(&sledFn))
	fv[0], fv[1] = 0, p
	yaroze.FlushCache()

	mode := (*uint32)(yaroze.Pointer(0x1f801124))
	saved := volatile.LoadUint32(mode)
	yaroze.EnterCriticalSection()
	volatile.StoreUint32(mode, 0) // system clock, free running
	show("empty", 0, empty)
	for _, n := range []uint32{0, 1, 2, 10, 50, 100} {
		show("spin", int(n), func() uint32 { return timeSpin(n) })
	}
	show("space", 16, space16)
	show("space then load", 16, space16Load)
	show("loadSpace", 16, loadSpace16)
	for _, n := range []uint32{1, 16} {
		runCount = n
		show("spaceRun", int(n), spaceRun)
	}
	show("nops", 128, timeSled)
	volatile.StoreUint32(mode, saved)
	yaroze.ExitCriticalSection()
	println("END")
}
