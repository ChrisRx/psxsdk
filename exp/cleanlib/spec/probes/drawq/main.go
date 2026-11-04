//go:build yaroze

// Command drawq is a probe for the spec: the calls that go through the
// drawing queue, each between two GP1 marker words (10000200h plus the
// case number), so that a trace shows the time each one takes and the
// register work it does.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000200|n)
}

var (
	pixels [16 * 16]uint16
	back   [16 * 16]uint16
	work   [256]uint32
	tags   [16]yaroze.GsOT_TAG
	ot     = yaroze.GsOT{Length: 4, Org: &tags[0]}
)

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 64, 0, 512)
	yaroze.DrawSync(0)
	for round := range uint32(2) {
		n := round * 16
		pal := yaroze.RECT{X: 0, Y: 480, W: 16, H: 1}
		img := yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
		mark(n + 0)
		yaroze.LoadImage(&pal, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
		yaroze.LoadImage(&img, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 2)
		yaroze.StoreImage(&pal, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 3)
		yaroze.MoveImage(&img, 704, 0)
		mark(n + 4)
		yaroze.ClearImage(&img, 1, 2, 3)
		mark(n + 5)
		yaroze.DrawSync(0)
		yaroze.GsSetWorkBase(yaroze.Packets(&work[0]))
		mark(n + 6)
		yaroze.GsClearOt(0, 0, &ot)
		mark(n + 7)
		yaroze.GsSortClear(0, 0, 0, &ot)
		mark(n + 8)
		yaroze.GsDrawOt(&ot)
		mark(n + 9)
		yaroze.Print("hi")
		mark(n + 10)
		yaroze.FntFlush(-1)
		mark(n + 11)
		yaroze.PutDrawEnv(yaroze.GsDRAWENV())
		mark(n + 12)
		yaroze.GsSwapDispBuff()
		mark(n + 13)
		yaroze.DrawSync(0)
		mark(n + 14)
		yaroze.VSync(0)
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
