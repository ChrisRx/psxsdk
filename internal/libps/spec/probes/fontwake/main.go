//go:build yaroze

// fontwake looks at when a FntFlush queued behind a frame's drawing goes
// out, on buildup's display (640 by 480 interlaced, one buffer, spec 43).
// Each frame: VSync(0), then D2_CHCR, D2_MADR and GPUSTAT read, ResetGraph(1),
// the same read again, a clear and k semitransparent 640 by 480 boxes with
// GsDrawOt, DICR read, a line of text with FntPrint and FntFlush(-1), DICR
// read. In the first part the program then spins until the next vertical
// blank and 300 lines past VSync's return, reading only the hardware and a
// blank count its VSyncCallback keeps, and records each change of channel
// 2's busy bit or of where D2_MADR points, and each time the CPU was away
// from the loop. In the second it goes straight on to the next frame's
// VSync(0), as buildup does. After each part and k it prints a summary line
// (summary). Run it with the drawing time on (PROBE_DRAWTIME=1): with the
// GPU always free, nothing waits.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	frames  = 12
	maxEv   = 12
	d2MADR  = 0x1f8010a0
	d2CHCR  = 0x1f8010a8
	gpuStat = 0x1f801814
	timer1  = 0x1f801110
	dicr    = 0x1f8010f4
	t2val   = 0x1f801120
	t2mode  = 0x1f801124
)

var (
	ot      yaroze.GsOT
	tags    [16]yaroze.GsOT_TAG
	packets [4096]uint32
	box     = yaroze.GsBOXF{Attribute: 1 << 30, X: -320, Y: -240, W: 640, H: 480, R: 40, G: 40, B: 120}
	textBuf [96]byte
	text    yaroze.Buffer
	blanks  uint32
)

// An event is a change of channel 2's busy bit or of where D2_MADR points
// (the program's memory or the library's), or the CPU away from the loop
// for more than 40 ticks of timer 2 (an interrupt), with the gap in ticks.
type event struct{ line, blank, chcr, madr, gap uint32 }

type record struct {
	atVSync, atReset  [3]uint32 // CHCR, MADR, GPUSTAT
	dicrDraw, dicrFnt uint32    // DICR after GsDrawOt, after FntFlush
	n                 int
	ev                [maxEv]event
}

var recs [frames]record

func load(a uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }

func tick() { volatile.StoreUint32(&blanks, volatile.LoadUint32(&blanks)+1) }

func regs(r *[3]uint32) {
	r[0], r[1], r[2] = load(d2CHCR), load(d2MADR), load(gpuStat)
}

func frame(k int, spin bool, r *record) {
	yaroze.VSync(0)
	start := load(timer1) & 0xffff
	b0 := volatile.LoadUint32(&blanks)
	regs(&r.atVSync)
	yaroze.ResetGraph(1)
	regs(&r.atReset)
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsSortClear(0, 0, 0, &ot)
	for range k {
		yaroze.GsSortBoxFill(&box, &ot, 1)
	}
	yaroze.GsDrawOt(&ot)
	r.dicrDraw = load(dicr)
	text.Reset()
	text.WriteString("FONTWAKE ").WriteInt(k).Print()
	yaroze.FntFlush(-1)
	r.dicrFnt = load(dicr)
	if !spin {
		return
	}
	busy := load(d2CHCR) & (1 << 24)
	region := load(d2MADR)&0xffffff >= 0x80000
	r.ev[0] = event{(load(timer1) - start) & 0xffff, 0, load(d2CHCR), load(d2MADR), 0}
	r.n = 1
	t := load(t2val) & 0xffff
	for {
		c := load(d2CHCR)
		m := load(d2MADR)
		b := volatile.LoadUint32(&blanks) - b0
		l := (load(timer1) - start) & 0xffff
		now := load(t2val) & 0xffff
		gap := (now - t) & 0xffff
		t = now
		if (c&(1<<24) != busy || (m&0xffffff >= 0x80000) != region || gap > 40) && r.n < maxEv {
			busy = c & (1 << 24)
			region = m&0xffffff >= 0x80000
			if gap <= 40 {
				gap = 0
			}
			r.ev[r.n] = event{l, b, c, m, gap}
			r.n++
		}
		if b >= 1 && l > 300 || b >= 2 {
			break
		}
	}
}

// lib reports whether a D2_MADR value points into the library, where its
// font's packets are, rather than into the program or at a list's end.
func lib(madr uint32) bool {
	a := madr & 0xffffff
	return a < 0x80000
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &tags[0]
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(64, 48, 400, 24, 0, 96)
	text.Init(textBuf[:])
	volatile.StoreUint32((*uint32)(yaroze.Pointer(t2mode)), 0x200)
	cb := tick
	yaroze.VSyncCallback((*[2]uintptr)(unsafe.Pointer(&cb))[1])
	yaroze.SetDispMask(1)
	for part := range 2 {
		spin := part == 0
		for _, k := range []int{0, 1, 2} {
			for f := range recs {
				recs[f] = record{}
				frame(k, spin, &recs[f])
			}
			yaroze.VSync(0)
			yaroze.ResetGraph(1)
			yaroze.DrawSync(0)
			summary(part, k)
		}
	}
	yaroze.VSyncCallback(0)
	println("END")
}

// summary prints, for the frames of a part and k after the first, how many
// found channel 2 idle when VSync returned, with D2_MADR in the library (the
// font's list sent last), how many had channel 2's completion interrupt on
// after GsDrawOt and after FntFlush (DICR bit 18), and from the spin, how many
// saw the CPU away for over 20,000 ticks (a handler waiting for the GPU), and
// the least and most lines from VSync's return to channel 2 going idle with
// D2_MADR in the library.
func summary(part, k int) {
	var idle, font, irqDraw, irqFlush, away int
	least, most := uint32(0xffff), uint32(0)
	for f := 1; f < frames; f++ {
		r := &recs[f]
		if r.atVSync[0]&(1<<24) == 0 {
			idle++
			if lib(r.atVSync[1]) {
				font++
			}
		}
		if r.dicrDraw&(1<<18) != 0 {
			irqDraw++
		}
		if r.dicrFnt&(1<<18) != 0 {
			irqFlush++
		}
		for i := 1; i < r.n; i++ {
			e := &r.ev[i]
			if e.gap > 20000 {
				away++
			}
			if e.chcr&(1<<24) == 0 && lib(e.madr) {
				least, most = min(least, e.line), max(most, e.line)
			}
		}
	}
	println("FWAKE part", part, "k", k, "frames", frames-1, "idle", idle, "font", font, "irqdraw", irqDraw, "irqflush", irqFlush, "away", away, "least", least, "most", most)
}
