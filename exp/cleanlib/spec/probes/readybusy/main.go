//go:build yaroze

// readybusy asks a console whether GPUSTAT says the GPU is ready for a
// command while channel 2 is still sending an image. It calls LoadImage
// with the GPU idle, then reads GPUSTAT and D2_CHCR as fast as it can, and
// prints each change of GPUSTAT bits 26 (ready for a command), 27 (ready to
// send video memory) and 28 (ready for a DMA block) and of the channel's
// busy bit, with the sample it came at. Images of 16 by 16 (8 blocks) and
// 256 by 128 (2048 blocks), then a drawing list of one large box for
// comparison. Then it times, with root counter 2 at the system clock over
// 8, LoadImage calls of several sizes from the call to the channel's end and
// to bit 26 coming back, printed in CPU cycles. It returns, so it ends on
// Sony's library too.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const samples = 3000

var (
	ot      yaroze.GsOT
	otTags  [4]yaroze.GsOT_TAG
	packets [1024]uint32
	box     yaroze.GsBOXF
	pixels  [256 * 128 / 2]uint32
	stat    [samples]uint32
	chcr    [samples]uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

func sample() {
	for i := range samples {
		stat[i] = load(0x1f801814)
		chcr[i] = load(0x1f8010a8)
	}
}

func report(name string) {
	print("RB ", name)
	last := uint32(0xffffffff)
	both := 0
	for i := range samples {
		s := stat[i]>>26&7 | chcr[i]>>24&1<<3
		if s&8 != 0 && s&1 != 0 {
			both++
		}
		if s != last {
			print(" ", i, ":", s>>3&1, "/", s&1, s>>1&1, s>>2&1)
			last = s
		}
	}
	println(" ; busy and ready for a command", both)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 2
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	println("RB key: sample:busy/b26 b27 b28")
	for k := range 3 {
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		r := yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
		yaroze.LoadImage(&r, &pixels[0])
		sample()
		yaroze.DrawSync(0)
		report("load16")

		yaroze.VSync(0)
		r = yaroze.RECT{X: 640, Y: 0, W: 256, H: 128}
		yaroze.LoadImage(&r, &pixels[0])
		sample()
		yaroze.DrawSync(0)
		report("load256x128")

		yaroze.VSync(0)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		box = yaroze.GsBOXF{W: 640, H: 480, R: uint8(20 * k), G: 40, B: 90}
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
		sample()
		yaroze.DrawSync(0)
		report("drawot")
	}
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801124)), 0x200)
	for _, sz := range [...][2]int16{{16, 16}, {64, 64}, {256, 64}, {256, 128}} {
		for range 3 {
			yaroze.DrawSync(0)
			yaroze.VSync(0)
			r := yaroze.RECT{X: 640, Y: 0, W: sz[0], H: sz[1]}
			t0 := load(0x1f801120)
			yaroze.LoadImage(&r, &pixels[0])
			t1 := load(0x1f801120)
			for load(0x1f8010a8)&(1<<24) != 0 {
			}
			t2 := load(0x1f801120)
			for load(0x1f801814)&(1<<26) == 0 {
			}
			t3 := load(0x1f801120)
			println("RBT", sz[0], "x", sz[1], "words", int(sz[0])*int(sz[1])/2, "call", 8*((t1-t0)&0xffff), "channel", 8*((t2-t0)&0xffff), "ready", 8*((t3-t0)&0xffff))
		}
	}
	println("END")
}
