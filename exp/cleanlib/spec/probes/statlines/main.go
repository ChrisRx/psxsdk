//go:build yaroze

// statlines records how GPUSTAT's bit 31 (drawing odd lines) and bit 13
// (interlace field) change over the fields of a 640 by 480 interlaced
// display, and a 640 by 240 one, on a console or the emulator, for
// modelling them in the emulator. After VSync(0) returns it reads, and
// reads only, GPUSTAT and timer 1, which both libraries set counting lines,
// in a tight loop for about three fields, and keeps each change of either
// bit, and each time the library's count of vertical blanks (VSync(-1)) goes
// up, with the lines since the return. It prints each run of changes,
// collapsed: a bit that flips every line is shown as one entry for the run,
// with its first and last line and how many flips.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer1  = 0x1f801110
	gpuStat = 0x1f801814
	maxEv   = 4000
)

type event struct {
	line uint16
	bits uint8 // bit 2: the blank count went up, bit 1: GPUSTAT 31, bit 0: GPUSTAT 13
}

var events [maxEv]event

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func bits(s uint32) uint8 { return uint8(s>>31&1)<<1 | uint8(s>>13&1) }

func record(name string) {
	yaroze.VSync(0)
	yaroze.VSync(0)
	start := load(timer1) & 0xffff
	last := bits(load(gpuStat))
	count := yaroze.VSync(-1)
	n := 0
	events[n] = event{0, last}
	n++
	for n < maxEv {
		lines := (load(timer1) - start) & 0xffff
		if lines > 800 {
			break
		}
		b := bits(load(gpuStat))
		c := yaroze.VSync(-1)
		if c != count {
			events[n] = event{uint16(lines), b | 4}
			n++
			count = c
			last = b
		} else if b != last {
			events[n] = event{uint16(lines), b}
			n++
			last = b
		}
	}
	println("STAT", name, "changes", n-1)
	// Collapse runs where bit 31 flips on consecutive lines with bit 13
	// steady.
	for i := 0; i < n; {
		j := i
		for j+1 < n && events[j+1].bits&4 == 0 && events[j+1].bits&1 == events[i].bits&1 && events[j+1].line-events[j].line <= 1 && events[j+1].line-events[i].line > 0 {
			j++
		}
		if j-i >= 3 {
			println("STAT  lines", events[i].line, "to", events[j].line, ": b31 flipping,", j-i, "flips, b13", events[i].bits&1)
		} else {
			for k := i; k <= j; k++ {
				if events[k].bits&4 != 0 {
					println("STAT  line", events[k].line, ": BLANK COUNTED, b31", events[k].bits>>1&1, "b13", events[k].bits&1)
				} else {
					println("STAT  line", events[k].line, ": b31", events[k].bits>>1&1, "b13", events[k].bits&1)
				}
			}
		}
		i = j + 1
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	record("480i")
	yaroze.GsInitGraph(640, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	record("240p")
	println("END")
}
