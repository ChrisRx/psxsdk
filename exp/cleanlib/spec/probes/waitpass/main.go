//go:build yaroze

// waitpass measures, at 240 lines and from the program alone, how widely
// VSync(0)'s return after the interrupt spreads with where the library's wait
// loop is when the interrupt comes (spec 57). Each field starts straight after
// an interrupt, waits a delay of one scratchpad store more than the field
// before (wrapping at steps), calls VSync(0), reads timer 2 at its return, and
// polls timer 2 alone to the next interrupt, as retphase's A does: A is the
// cycles from the return to the last read before that interrupt. The blanks
// come a fixed time apart, so A's spread over the delays is the return's
// spread, which is the wait loop's pass and the poll's own step. A console's
// interrupt varies in length by more than the pass, which scrambles the phase
// each field there: on a console compare the variance of A between the
// libraries, not A by delay. It prints a line a field after the run, then the
// shortest, mean and longest A, then END, and returns.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer2    = 0x1f801120
	timer2Mod = 0x1f801124
	scratch   = 0x1f800300
	minGap    = 2000
	steps     = 64
	fields    = 256
	settle    = 16
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func store(addr uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(addr)), v) }

func timer() uint32 { return load(timer2) & 0xffff }

// pollA polls timer 2 until the interrupt, and returns the cycles from from
// to the last read before it.
func pollA(from uint32) uint32 {
	prev, total := from, uint32(0)
	for n := 0; n < 4000000; n++ {
		now := timer()
		d := (now - prev) & 0xffff
		if d > minGap {
			return total
		}
		total += d
		prev = now
	}
	return 0
}

var got [fields]uint32

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := load(timer2Mod)
	store(timer2Mod, 0) // system clock, free running
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	for i := 0; i < settle; i++ {
		yaroze.VSync(0)
	}
	pollA(timer())
	for f := uint32(0); f < fields; f++ {
		for i := uint32(0); i < f%steps; i++ {
			store(scratch, i)
		}
		yaroze.VSync(0)
		r := timer()
		got[f] = pollA(r)
	}
	least, most, sum := ^uint32(0), uint32(0), int32(0)
	for f, a := range got {
		println("A", f, f%steps, a)
		least, most, sum = min(least, a), max(most, a), sum+int32(a-got[0])
	}
	println("SUM least", least, "mean", sum/fields+int32(got[0]), "most", most, "spread", most-least)
	store(timer2Mod, saved)
	println("END")
}
