//go:build yaroze

// irqgaps finds where interrupts take the CPU during about three fields, and
// whether the library's count of vertical blanks (VSync(-1)) goes up in one,
// on a console or the emulator, for spec 45's late count. After VSync(0)
// returns it polls timer 2, counting the system clock, timer 1, which both
// libraries set counting lines, and VSync(-1) in a tight loop. A poll that
// comes more than minGap cycles after the one before is a gap: something
// else ran. It prints each gap with the line it ended on, its length in
// cycles, and whether the count went up across it, then, for each count
// that went up outside a gap, the line it was seen on. A 480-line
// interlaced display first, then 240 lines.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer1    = 0x1f801110
	timer2    = 0x1f801120
	timer2Mod = 0x1f801124
	minGap    = 400
	maxEv     = 200
)

type event struct {
	line    uint16
	cycles  uint16 // 0 for a count seen outside a gap
	counted bool
}

var events [maxEv]event

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func record(name string) {
	yaroze.VSync(0)
	yaroze.VSync(0)
	start := load(timer1) & 0xffff
	prev := load(timer2) & 0xffff
	count := yaroze.VSync(-1)
	n := 0
	for n < maxEv {
		now := load(timer2) & 0xffff
		lines := (load(timer1) - start) & 0xffff
		c := yaroze.VSync(-1)
		if lines > 800 {
			break
		}
		d := (now - prev) & 0xffff
		prev = now
		switch {
		case d > minGap:
			events[n] = event{uint16(lines), uint16(d), c != count}
			n++
		case c != count:
			events[n] = event{uint16(lines), 0, true}
			n++
		}
		count = c
	}
	println("GAPS", name, n)
	for _, e := range events[:n] {
		counted := 0
		if e.counted {
			counted = 1
		}
		if e.cycles == 0 {
			println("GAPS  line", e.line, ": counted outside a gap")
		} else {
			println("GAPS  line", e.line, ": gap", e.cycles, "cycles, counted", counted)
		}
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := load(timer2Mod)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), 0) // system clock, free running
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	record("480i")
	yaroze.GsInitGraph(640, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	record("240p")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timer2Mod)), saved)
	println("END")
}
