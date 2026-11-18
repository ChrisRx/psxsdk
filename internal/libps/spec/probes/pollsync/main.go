//go:build yaroze

// pollsync is an emulator probe for spec 50: DrawSync(1) polled in a row
// behind each kind of image call on 256 by 256, and with nothing going on,
// each poll between GP1 marker words (10000600h plus a number) for a bus
// trace (PROBE_BUS=start): the marker before poll i of kind k in round r is
// r*128 + k*16 + i, and the one after the last poll of a kind is i of 12.
// The kinds are idle, LoadImage, StoreImage, MoveImage, ClearImage on a
// rectangle and ClearImage on a fill. It prints what the polls gave.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000600|n)
}

var (
	pixels [256 * 256]uint16
	back   [256 * 256]uint16
)

var big = yaroze.RECT{X: 704, Y: 0, W: 256, H: 256}

const (
	kinds  = 6
	rounds = 2
	polls  = 12
)

var gave [rounds][kinds]int32

//go:noinline
func run(k, n uint32) int32 {
	yaroze.VSync(0)
	yaroze.DrawSync(0)
	r := big
	switch k {
	case 1:
		yaroze.LoadImage(&r, (*uint32)(unsafe.Pointer(&pixels[0])))
	case 2:
		yaroze.StoreImage(&r, (*uint32)(unsafe.Pointer(&back[0])))
	case 3:
		yaroze.MoveImage(&r, 384, 256)
	case 4:
		r.X++
		yaroze.ClearImage(&r, 1, 2, 3)
	case 5:
		yaroze.ClearImage(&r, 1, 2, 3)
	}
	sum := int32(0)
	for i := range uint32(polls) {
		mark(n + i)
		sum += yaroze.DrawSync(1)
	}
	mark(n + polls)
	yaroze.DrawSync(0)
	return sum
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	println("start")
	for r := range uint32(rounds) {
		for k := range uint32(kinds) {
			gave[r][k] = run(k, r*128+k*16)
		}
	}
	for r := range rounds {
		println("POLLSYNC round", r, "gave", gave[r][0], gave[r][1], gave[r][2], gave[r][3], gave[r][4], gave[r][5])
	}
	println("END")
}
