//go:build yaroze

// fieldtime checks whether a frame's drawing finishes inside the field it
// started in, on a console or the emulator, under either library. It copies
// showcase-gasgar's loop, as imgdrop does: a 640 by 480 interlaced display
// of one buffer, ResetGraph(1) at the top of each frame, a clear and k
// semitransparent 640 by 480 boxes with GsDrawOt, then n small LoadImage
// calls behind them, eight as gasgar has, and 0 and 32 to find what each
// queued request costs. Before the clear, the CPU waits w
// milliseconds, standing in for the game's own work, which puts the
// drawing later in the field.
//
// Drawing to a 480-line interlaced display with E1 bit 10 clear writes only
// the lines of the field not being shown, so a frame drawn inside one field
// is never seen half done. One that runs past the field change finishes on
// the other field's lines and leaves what it had drawn, the clear included,
// on the lines then shown.
//
// Each frame it reads timer 2, at the system clock over eight, from the
// return of VSync, at the vertical blank, then spins until channel 2 and
// the GPU are idle, reading only the hardware, so that the libraries are
// called as gasgar calls them. A frame is late if that is more than a
// field, 1/59.94s, after VSync returned: past the next vertical blank,
// when the field shown changes. For each w and k it prints the late
// frames, those never idle in 25ms, and the mean and the longest times, in
// microseconds from VSync's return, to GsDrawOt's return and to the GPU
// going idle.
package main

import (
	"runtime/volatile"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	images = 32
	frames = 60

	timerValue = 0x1f801120
	timerMode  = 0x1f801124
	gpuStat    = 0x1f801814
	d2CHCR     = 0x1f8010a8

	// fieldTicks is a field, 1/59.94s, in timer 2's ticks: from one
	// vertical blank to the next, when the field shown changes.
	fieldTicks = 33869 * 1000 / 8 * 1000 / 59940
)

// waits are the CPU's waits, in milliseconds, before the frames of eight
// images behind 1, 3 and 5 boxes. A console run took 0, 6 and 10, which
// change nothing but when the drawing starts; the emulator, whose drawing
// is slower, waits out the 25ms cap on all but one of the others, so the
// probe as kept takes 0 alone (spec 41).
var waits = []int{0}

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [16384]uint32
	boxes   [8]yaroze.GsBOXF
	pixels  [images][16 * 16 / 2]uint32
)

func load(addr uintptr) uint32 { return volatile.LoadUint32((*uint32)(yaroze.Pointer(addr))) }

// clock is timer 2 counted on past its 16 bits, which it wraps in under a
// field: each call has to come less than 15ms after the last.
type clock struct{ last, high uint32 }

func (c *clock) now() uint32 {
	t := load(timerValue) & 0xffff
	if t < c.last {
		c.high += 0x10000
	}
	c.last = t
	return c.high | t
}

// us is ticks of the system clock over eight in microseconds.
func us(t uint32) uint32 { return t * 8 * 1000 / 33869 }

func hex(v uint32) {
	const d = "0123456789abcdef"
	for i := 28; i >= 0; i -= 4 {
		print(string(rune(d[v>>uint(i)&15])))
	}
}

func rect(i int) yaroze.RECT {
	return yaroze.RECT{X: int16(640 + 16*(i%8)), Y: int16(16 * (i / 8)), W: 16, H: 16}
}

func idle() bool {
	return load(d2CHCR)&(1<<24) == 0 && load(gpuStat)&(1<<26|1<<28) == 1<<26|1<<28
}

func run(w, k, n int) {
	late, never := 0, 0
	var sumOt, maxOt, sumIdle, maxIdle uint32
	for f := 0; f < frames; f++ {
		yaroze.VSync(0)
		c := clock{last: load(timerValue) & 0xffff}
		start := c.last
		yaroze.ResetGraph(1)
		for c.now()-start < uint32(w)*33869/8 {
		}
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		for i := range k {
			boxes[i] = yaroze.GsBOXF{Attribute: 1 << 30, X: 0, Y: 0, W: 640, H: 480, R: uint8(20 + 8*i), G: 40, B: 90}
			yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
		}
		yaroze.GsDrawOt(&ot)
		tOt := c.now() - start
		for i := range n {
			r := rect(i)
			yaroze.LoadImage(&r, &pixels[i][0])
		}
		// Idle means idle for a while: a queued request starts at an
		// interrupt's end. The spin ends there, or 25ms after VSync.
		var tIdle uint32
		quiet := 0
		for {
			t := c.now() - start
			if t > 25000*33869/8000 {
				break
			}
			if !idle() {
				quiet = 0
			} else if quiet++; quiet == 200 {
				tIdle = t
				break
			}
		}
		if f == 0 && tIdle == 0 {
			print("never idle: GPUSTAT ")
			hex(load(gpuStat))
			print(" D2 CHCR ")
			hex(load(d2CHCR))
			println()
		}
		if tIdle == 0 || tIdle > fieldTicks {
			late++
		}
		if tIdle == 0 {
			never++
			tIdle = 25000 * 33869 / 8000
		}
		sumOt += tOt
		maxOt = max(maxOt, tOt)
		sumIdle += tIdle
		maxIdle = max(maxIdle, tIdle)
	}
	println("FIELD wait", w, "boxes", k, "images", n, "late", late, "of", frames, "never idle", never,
		"; GsDrawOt mean", us(sumOt/frames), "max", us(maxOt),
		"; idle mean", us(sumIdle/frames), "max", us(maxIdle))
}

func main() {
	println("start")
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("setup done")
	for _, w := range waits {
		for _, k := range []int{1, 3, 5} {
			run(w, k, 8)
		}
	}
	// What each request queued behind the drawing costs: the same frame
	// with more and fewer images.
	for _, k := range []int{0, 1} {
		for _, n := range []int{0, 8, 32} {
			run(0, k, n)
		}
	}
	println("END")
}
