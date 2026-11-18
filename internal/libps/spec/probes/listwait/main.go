//go:build yaroze

// listwait asks whether the library's interrupt handler gives up waiting
// for the GPU before it sends a packet list queued behind a drawing (spec
// 43). Run it with the emulator's drawing time on (PROBE_DRAWTIME=1). Each
// case draws a clear and a 640 by 480 semitransparent box with GsDrawOt and
// queues a line of text behind it with FntFlush(-1); then, with interrupts
// off, it waits for channel 2 to finish, keeps the GPU busy for a long time
// by writing n large rectangles to GP0 itself (about 15 ms each), and turns
// interrupts on. Channel 2's completion interrupt is then taken, and a
// handler that waits for the GPU before sending the text keeps the CPU
// that long. It prints DICR after FntFlush, the lines the CPU was away (timer
// 1, 16 bits: a wait of 1,200 rectangles wraps it four times), the blanks
// counted meanwhile, and GPUSTAT bit 26, D2_CHCR and D2_MADR after.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	gp0     = 0x1f801810
	gpuStat = 0x1f801814
	d2CHCR  = 0x1f8010a8
	d2MADR  = 0x1f8010a0
	timer1  = 0x1f801110
	dicr    = 0x1f8010f4
)

var (
	ot      yaroze.GsOT
	tags    [4]yaroze.GsOT_TAG
	packets [256]uint32
	box     = yaroze.GsBOXF{Attribute: 1 << 30, X: -320, Y: -240, W: 640, H: 480, R: 40, G: 40, B: 120}
	textBuf [64]byte
	text    yaroze.Buffer
)

func load(a uintptr) uint32     { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }
func store(a uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(a)), v) }

func busy(n int) {
	for range n {
		store(gp0, 0x60204060)
		store(gp0, 0)
		store(gp0, 511<<16|1023)
	}
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 2
	ot.Org = &tags[0]
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(64, 48, 400, 24, 0, 64)
	text.Init(textBuf[:])
	yaroze.SetDispMask(1)
	for _, n := range []int{20, 1200} {
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		yaroze.GsSortBoxFill(&box, &ot, 1)
		yaroze.GsDrawOt(&ot)
		text.Reset()
		text.WriteString("HWAIT").Print()
		yaroze.FntFlush(-1)
		d := load(dicr)
		yaroze.EnterCriticalSection()
		for load(d2CHCR)&(1<<24) != 0 {
		}
		busy(n)
		b0 := yaroze.VSync(-1)
		l0 := load(timer1) & 0xffff
		yaroze.ExitCriticalSection()
		l1 := load(timer1) & 0xffff
		b1 := yaroze.VSync(-1)
		println("n", n, "dicr", d, "away lines", (l1-l0)&0xffff, "blanks", b1-b0, "ready", load(gpuStat)>>26&1, "chcr", load(d2CHCR), "madr", load(d2MADR))
		yaroze.DrawSync(0)
	}
	println("END")
}
