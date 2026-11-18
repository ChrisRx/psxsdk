//go:build yaroze

// Command vsyncmodes is a probe for spec 42: which VSync modes wait for the
// field on a 480-line interlaced display, and what they return and record.
// It is vsynclate's table (spec 01) on showcase-gasgar's 640 by 480
// interlaced display: VSync(n) called d lines after a VSync(0), each case
// twice, so that the VSync(0) before it ends after each kind of field. Each
// line gives n, d, the call's return, the blanks from the VSync(0) to the
// call and to the return, the lines the call took, GPUSTAT bits 31 and 13
// when the VSync(0) returned and when the call returned, and what a
// VSync(1) right after the call returns: the lines since the time the call
// recorded. Then the same on a 640 by 240 display that isn't interlaced.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timer1  = 0x1f801110
	gpuStat = 0x1f801814
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func now() uint32 { return load(timer1) & 0xffff }

// bits is GPUSTAT's bit 31 and bit 13 as two digits.
func bits(s uint32) uint32 { return s>>31&1*10 + s>>13&1 }

func table() {
	for _, n := range []int32{0, 1, 2, 3} {
		for _, d := range []uint32{0, 20, 240, 262, 270, 300, 520, 530, 560} {
			for range 2 {
				yaroze.VSync(0)
				s0 := load(gpuStat)
				from := uint32(yaroze.VSync(-1))
				start := now()
				for (now()-start)&0xffff < d {
				}
				at := uint32(yaroze.VSync(-1)) - from
				before := now()
				r := yaroze.VSync(n)
				after := now()
				s1 := load(gpuStat)
				one := yaroze.VSync(1)
				took := (after - before) & 0xffff
				println("n", n, "d", d, "return", r, "blanks", at, uint32(yaroze.VSync(-1))-from, "took", took, "bits", bits(s0), bits(s1), "one", one)
			}
		}
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("interlaced 480")
	table()
	yaroze.GsInitGraph(640, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	yaroze.VSync(0)
	println("progressive 240")
	table()
	println("END")
}
