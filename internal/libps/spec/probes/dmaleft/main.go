//go:build yaroze

// dmaleft prints DPCR (1F8010F0h) and the GPU channel's D2_CHCR
// (1F8010A8h), with D6_CHCR (ordering tables) and D4_CHCR (sound), after
// each of the calls that may set them, for spec 45: what a program leaves
// in them when it returns. After spec 44's rerun, Sony's library left DPCR
// at 3B3B3B33h and D2_CHCR at 401h, the clean one 08089899h and 0. It ends
// with a read of a sector of the disc, for the drive's channel.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	dpcr   = 0x1f8010f0
	d2chcr = 0x1f8010a8
	d4chcr = 0x1f8010c8
	d6chcr = 0x1f8010e8
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [4096]uint32
	pixels  [16 * 16 / 2]uint32
	back    [16 * 16 / 2]uint32
	sector  [512]uint32
	name    = []byte("\\MAIN.EXE;1\x00")
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func show(what string) {
	println("DMA", what, ": DPCR", hex(load(dpcr)), "D2", hex(load(d2chcr)), "D4", hex(load(d4chcr)), "D6", hex(load(d6chcr)))
}

func hex(v uint32) string {
	var b [8]byte
	for i := range b {
		b[i] = "0123456789abcdef"[v>>(28-4*i)&15]
	}
	return string(b[:])
}

func frame() {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsSortClear(10, 20, 30, &ot)
	yaroze.GsDrawOt(&ot)
}

func main() {
	show("entry")
	yaroze.ResetGraph(0)
	show("ResetGraph(0)")
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	show("GsInitGraph")
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.GsClearOt(0, 0, &ot)
	show("GsClearOt")
	frame()
	show("GsDrawOt")
	yaroze.DrawSync(0)
	show("DrawSync(0)")
	r := yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
	yaroze.LoadImage(&r, &pixels[0])
	show("LoadImage")
	yaroze.DrawSync(0)
	show("LoadImage DrawSync(0)")
	yaroze.StoreImage(&r, &back[0])
	yaroze.DrawSync(0)
	show("StoreImage DrawSync(0)")
	yaroze.VSync(0)
	show("VSync(0)")
	yaroze.ResetGraph(1)
	show("ResetGraph(1)")
	frame()
	yaroze.DrawSync(0)
	show("frame DrawSync(0)")
	yaroze.ResetGraph(1)
	show("ResetGraph(1) after a frame")
	frame()
	show("frame running")
	yaroze.ResetGraph(1)
	show("ResetGraph(1) on a running frame")
	frame()
	yaroze.DrawSync(0)
	yaroze.ResetGraph(3)
	show("ResetGraph(3)")
	yaroze.ResetGraph(0)
	show("ResetGraph(0) again")
	frame()
	yaroze.DrawSync(0)
	show("frame DrawSync(0)")
	n := yaroze.CdReadFile(&name[0], &sector[0], 2048)
	show("CdReadFile")
	yaroze.CdReadSync(0, nil)
	println("read", n, "bytes")
	show("CdReadSync(0)")
	println("END")
}
