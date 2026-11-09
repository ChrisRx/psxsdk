//go:build yaroze

// fieldphase checks where in the field a frame's clear lands, on a console
// or the emulator, under either library. It runs teal's loop, showcase-
// gasgar's display with nothing on it: 640 by 480 interlaced from one
// buffer, VSync(0), ResetGraph(1), and a clear sent with GsDrawOt.
//
// With E1 bit 10 clear, the GPU writes only the lines of the field not
// being shown, and GPUSTAT bit 31 says which: 0 for even lines or the
// vertical blank, 1 for odd. A clear that ends inside the vertical blank
// every frame writes the same field every frame, and the other keeps
// whatever was in video memory.
//
// Each frame it reads timer 2, at the system clock over eight, and GPUSTAT
// when VSync returns, then spins, reading only the hardware, until channel
// 2 and the GPU have been idle for 200 reads (the clear done), and reads
// GPUSTAT again; then spins on until bit 31 reads 1 or bit 13 changes, the
// end of the vertical blank, for at most 3ms. For each library it prints
// how many frames saw each pair of bit 31 and bit 13 when VSync returned
// and when the clear was done, and the mean times, in microseconds from
// VSync's return, to the clear being done and to the blank's end.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	frames = 600

	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	gpuStat    = 0x1f801814
	d2CHCR     = 0x1f8010a8
)

var (
	ot   yaroze.GsOT
	tags [1 << 4]yaroze.GsOT_TAG
	pk   [1024]uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

// clock is timer 2 counted on past its 16 bits; each call has to come less
// than 15ms after the last.
type clock struct{ last, high uint32 }

func (c *clock) now() uint32 {
	t := load(timerValue) & 0xffff
	if t < c.last {
		c.high += 0x10000
	}
	c.last = t
	return c.high | t
}

func us(t uint32) uint32 { return t * 8 * 1000 / 33869 }

func idle() bool {
	return load(d2CHCR)&(1<<24) == 0 && load(gpuStat)&(1<<26|1<<28) == 1<<26|1<<28
}

// bits is GPUSTAT's bit 31 and bit 13 as two digits, 0 to 3.
func bits(s uint32) int { return int(s>>31&1)<<1 | int(s>>13&1) }

func main() {
	println("start")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &tags[0]
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("setup done")

	var pairs [4][4]int
	var sumDone, sumEnd uint32
	noEnd := 0
	for range frames {
		yaroze.VSync(0)
		c := clock{last: load(timerValue) & 0xffff}
		start := c.last
		s0 := load(gpuStat)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&pk[0]))
		yaroze.GsSortClear(0, 128, 128, &ot)
		yaroze.GsDrawOt(&ot)
		quiet := 0
		for quiet < 200 && c.now()-start < 10000*33869/8000 {
			if idle() {
				quiet++
			} else {
				quiet = 0
			}
		}
		done := c.now() - start
		s1 := load(gpuStat)
		end := uint32(0)
		for c.now()-start < 3000*33869/8000 {
			s := load(gpuStat)
			if s>>31&1 == 1 || s>>13&1 != s0>>13&1 {
				end = c.now() - start
				break
			}
		}
		if end == 0 {
			noEnd++
		}
		pairs[bits(s0)][bits(s1)]++
		sumDone += done
		sumEnd += end
	}
	for a := range 4 {
		for b := range 4 {
			if pairs[a][b] > 0 {
				println("PHASE vsync b31", a>>1, "b13", a&1, "; done b31", b>>1, "b13", b&1, ":", pairs[a][b], "frames")
			}
		}
	}
	println("PHASE clear done mean", us(sumDone/frames), "; blank end mean", us(sumEnd/uint32(frames-noEnd+1)), "; no end seen", noEnd)
	println("END")
}
