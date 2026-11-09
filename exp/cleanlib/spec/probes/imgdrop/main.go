//go:build yaroze

// imgdrop checks whether LoadImage calls queued behind a frame's drawing land
// in video memory before the next frame's ResetGraph(1), which empties the
// drawing queue, on a console or in the emulator, under either library. It
// copies showcase-gasgar's loop: a 640 by 480 interlaced display of one
// buffer, ResetGraph(1) at the top of each frame, a clear and the frame's
// drawing with GsDrawOt, then eight small LoadImage calls behind it. The
// drawing is k semitransparent 640 by 480 boxes, for k from 1 to 10, to bring
// it near and past a whole frame. Each frame's images carry the frame's number; after
// the next ResetGraph(1) one pixel of each is read back to see which landed.
//
// For each k it prints how many of the images landed, how many frames had
// all eight, and how many had none.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

const (
	images = 8
	frames = 60
)

var (
	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [16384]uint32
	boxes   [10]yaroze.GsBOXF
	pixels  [images][16 * 16 / 2]uint32
	back    [images][1]uint32
)

// rect is image i's place in video memory, right of the display.
func rect(i int) yaroze.RECT {
	return yaroze.RECT{X: int16(640 + 16*(i%4)), Y: int16(16 * (i / 4)), W: 16, H: 16}
}

// stamp is the pixel frame f's images carry.
func stamp(f int) uint32 { return uint32(f%0x7fff + 1) }

func run(k int) {
	landed, all, none := 0, 0, 0
	for f := 0; f <= frames; f++ {
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		if f > 0 {
			// The last frame's images, if they landed before the reset.
			n := 0
			for i := range images {
				r := rect(i)
				r.W, r.H = 2, 1
				yaroze.StoreImage(&r, &back[i][0])
			}
			yaroze.DrawSync(0)
			for i := range images {
				if back[i][0]&0xffff == stamp(f-1) {
					n++
				}
			}
			landed += n
			if n == images {
				all++
			}
			if n == 0 {
				none++
			}
		}
		if f == frames {
			break
		}
		yaroze.GsClearOt(0, 0, &ot)
		yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
		yaroze.GsSortClear(0, 0, 0, &ot)
		for i := range k {
			boxes[i] = yaroze.GsBOXF{Attribute: 1 << 30, X: 0, Y: 0, W: 640, H: 480, R: uint8(20 + 8*i), G: 40, B: 90}
			yaroze.GsSortBoxFill(&boxes[i], &ot, 1)
		}
		yaroze.GsDrawOt(&ot)
		s := stamp(f)
		for i := range images {
			for j := range pixels[i] {
				pixels[i][j] = s | s<<16
			}
			r := rect(i)
			yaroze.LoadImage(&r, &pixels[i][0])
		}
	}
	println("DROP boxes", k, "landed", landed, "of", frames*images, "; frames with all", all, "with none", none)
}

func main() {
	println("start")
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 480, 1, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 0)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)
	yaroze.VSync(0)
	println("setup done")
	for _, k := range []int{1, 4, 5, 6, 7, 8, 10} {
		run(k)
	}
	println("END")
}
