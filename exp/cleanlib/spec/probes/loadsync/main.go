//go:build yaroze

// Command loadsync is a console probe for spec 49: how long after a free
// image call returns its transfer is still going, as DrawSync(1) sees it.
// For LoadImage, MoveImage and ClearImage's fill at a few sizes, it makes
// the call with the GPU free, waits d turns of a small loop, d from 0 to 63,
// and keeps what DrawSync(1) then
// gives. Each line has the 64 answers as digits, the first d that gave 0,
// and the call's and the DrawSync(1)'s time at d of 0, in eighths of the
// system clock, the least of eight. A library that starts the transfer
// sooner before it returns gives 0 at a smaller d. The line TURN gives 1,000
// turns of the loop in eighths.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	delays     = 64
)

var (
	pixels [64 * 64]uint16
	turn   uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func now() uint32 { return load(timerValue) & 0xffff }

// wait spends n turns of a loop of a load and a store.
//
//go:noinline
func wait(n uint32) {
	for range n {
		volatile.StoreUint32(&turn, volatile.LoadUint32(&turn)+1)
	}
}

type probe struct {
	name string
	rect yaroze.RECT
	call func(r *yaroze.RECT)
}

var probes = [...]probe{
	{"LOAD 16x16", yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}, loadImage},
	{"LOAD 24x24", yaroze.RECT{X: 640, Y: 0, W: 24, H: 24}, loadImage},
	{"LOAD 32x32", yaroze.RECT{X: 640, Y: 0, W: 32, H: 32}, loadImage},
	{"LOAD 48x48", yaroze.RECT{X: 640, Y: 0, W: 48, H: 48}, loadImage},
	{"MOVE 16x16", yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}, moveImage},
	{"MOVE 32x32", yaroze.RECT{X: 640, Y: 0, W: 32, H: 32}, moveImage},
	{"FILL 64x4", yaroze.RECT{X: 640, Y: 0, W: 64, H: 4}, clearImage},
	{"FILL 64x16", yaroze.RECT{X: 640, Y: 0, W: 64, H: 16}, clearImage},
}

func loadImage(r *yaroze.RECT)  { yaroze.LoadImage(r, (*uint32)(unsafe.Pointer(&pixels[0]))) }
func moveImage(r *yaroze.RECT)  { yaroze.MoveImage(r, 640, 256) }
func clearImage(r *yaroze.RECT) { yaroze.ClearImage(r, 1, 2, 3) }

var (
	answers [delays]byte
	line    [delays]byte
)

//go:noinline
func once(p *probe, d uint32) (sync int32, call, poll uint32) {
	yaroze.VSync(0)
	yaroze.DrawSync(0)
	r := p.rect
	t0 := now()
	p.call(&r)
	t1 := now()
	wait(d)
	t2 := now()
	sync = yaroze.DrawSync(1)
	t3 := now()
	yaroze.DrawSync(0)
	return sync, (t1 - t0) & 0xffff, (t3 - t2) & 0xffff
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	saved := load(timerMode)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200) // the system clock over 8
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	t := now()
	wait(1000)
	println("TURN 1000 turns", (now()-t)&0xffff)
	for i := range probes {
		p := &probes[i]
		call, poll := uint32(0xffff), uint32(0xffff)
		for range 8 {
			_, c, s := once(p, 0)
			call, poll = min(call, c), min(poll, s)
		}
		first := -1
		for d := range uint32(delays) {
			s, _, _ := once(p, d)
			line[d] = byte('0' + s)
			if s == 0 && first < 0 {
				first = int(d)
			}
		}
		println("LOADSYNC", p.name, string(line[:]), "first 0 at", first, "call", call, "DrawSync(1)", poll)
	}
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), saved)
	println("END")
}
