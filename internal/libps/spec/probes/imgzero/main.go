//go:build yaroze

// Command imgzero is a probe for spec 49: LoadImage and StoreImage of no
// size made behind a drawing that still holds the channel, where a sized
// one goes into the queue. It prints DICR, D2_CHCR and DrawSync(1) after
// the drawing is queued, after each call, and once the drawing is done,
// and then draws a second frame, to show that nothing is left waiting and
// the channel's completion is not left interrupting.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	otTags  [16]yaroze.GsOT_TAG
	ot      = yaroze.GsOT{Length: 4, Org: &otTags[0]}
	packets [1024]uint32
	box     yaroze.GsBOXF
	pixels  [16 * 16]uint16
	dicr    [8]uint32
	chcr    [8]uint32
	sync    [8]int32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func note(i int) {
	dicr[i] = load(0x1f8010f4)
	chcr[i] = load(0x1f8010a8)
	sync[i] = yaroze.DrawSync(1)
}

func draw() {
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSortClear(0, 0, 0, &ot)
	box = yaroze.GsBOXF{Attribute: 1 << 30, W: 320, H: 240, R: 20, G: 40, B: 90}
	yaroze.GsSortBoxFill(&box, &ot, 1)
	yaroze.GsDrawOt(&ot)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	none := yaroze.RECT{X: 640, Y: 0, W: 0, H: 4}
	flat := yaroze.RECT{X: 640, Y: 0, W: 4, H: 0}
	draw()
	note(0)
	yaroze.LoadImage(&none, (*uint32)(unsafe.Pointer(&pixels[0])))
	note(1)
	yaroze.StoreImage(&flat, (*uint32)(unsafe.Pointer(&pixels[0])))
	note(2)
	yaroze.LoadImage(&flat, (*uint32)(unsafe.Pointer(&pixels[0])))
	note(3)
	yaroze.DrawSync(0)
	note(4)
	yaroze.VSync(0)
	draw()
	note(5)
	yaroze.DrawSync(0)
	note(6)
	names := [...]string{"drawing", "LoadImage 0 by 4", "StoreImage 4 by 0", "LoadImage 4 by 0", "DrawSync(0)", "second drawing", "DrawSync(0)"}
	for i := range names {
		println("ZERO", names[i], ": DICR", dicr[i]>>16, "D2_CHCR", chcr[i]>>24, "DrawSync(1)", sync[i])
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
