//go:build yaroze

// vsyncphase measures how many lines after the vertical blank starts
// VSync(0) returns, on a console or the emulator, under either library, on
// showcase-gasgar's 640 by 480 interlaced display.
//
// vsynclate (spec 01) measured VSync from call to return, which matches,
// but not where in the field the return lands. fieldphase showed that on
// the clean library a frame's clear can end inside the vertical blank and
// so write the same field every frame, where on Sony's it doesn't.
//
// Each frame it waits, reading only the hardware, for GPUSTAT bit 31 (odd
// lines) to read 1 and then 0: the odd field's picture ending, which is
// the vertical blank starting. It calls VSync(0) right then and reads timer
// 1, counting lines as both libraries set it, before and after, and
// GPUSTAT bit 31 after. It prints, over the frames that saw the odd field
// end, the least, the most and the mean lines from the blank's start to the
// return, and how many returned with bit 31 already 1, past the blank.
// Then the same on a 240-line display that isn't interlaced (progressive).
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	frames  = 240
	timer1  = 0x1f801110
	gpuStat = 0x1f801814
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("setup done; timer 1 mode", load(timer1+4)&0x3ff)

	seen, past := 0, 0
	least, most, sum := uint32(0xffff), uint32(0), uint32(0)
	for range frames {
		yaroze.VSync(0)
		// Wait for the odd field's picture, then for its end, up to
		// about 40ms in all by counting reads.
		n := 0
		for load(gpuStat)>>31 == 0 && n < 400000 {
			n++
		}
		for load(gpuStat)>>31 == 1 && n < 400000 {
			n++
		}
		if n >= 400000 {
			continue
		}
		before := load(timer1) & 0xffff
		yaroze.VSync(0)
		after := load(timer1) & 0xffff
		s := load(gpuStat)
		lines := (after - before) & 0xffff
		seen++
		if s>>31 == 1 {
			past++
		}
		least = min(least, lines)
		most = max(most, lines)
		sum += lines
	}
	if seen == 0 {
		println("VPHASE no odd field seen")
	} else {
		println("VPHASE frames", seen, "; lines from the blank's start to the return: least", least, "most", most, "mean", sum/uint32(seen), "; past the blank", past)
	}
	progressive()
	println("END")
}

// progressive measures the same on a 640 by 240 display, not interlaced,
// where bit 31 changes every line of the picture and stays 0 through the
// blank: the blank has started once it has read 0 for three lines.
func progressive() {
	yaroze.GsInitGraph(640, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	seen := 0
	least, most, sum := uint32(0xffff), uint32(0), uint32(0)
	for range frames {
		yaroze.VSync(0)
		last := load(timer1) & 0xffff
		n := 0
		for n < 400000 {
			n++
			line := load(timer1) & 0xffff
			if load(gpuStat)>>31 == 1 {
				last = line
			} else if (line-last)&0xffff >= 3 {
				break
			}
		}
		if n >= 400000 {
			continue
		}
		yaroze.VSync(0)
		after := load(timer1) & 0xffff
		lines := (after - last - 1) & 0xffff
		seen++
		least = min(least, lines)
		most = max(most, lines)
		sum += lines
	}
	if seen == 0 {
		println("VPHASE240 no blank seen")
		return
	}
	println("VPHASE240 frames", seen, "; lines from the blank's start to the return: least", least, "most", most, "mean", sum/uint32(seen))
}
