package probetest

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var imgq = flag.String("imgq", "", "the cue sheet of the imgq probe's disc, as probe.sh builds it, for TestQueuedImages")

// TestQueuedImages runs spec 39's probe, spec/probes/imgq, with the GPU taking
// time to draw (emu's SetDrawTime), and checks that the image calls behave as
// they do on a console under Sony's library: LoadImage and StoreImage called
// just after GsDrawOt return at once, StoreImage's buffer is written only once
// the drawing is done, and the images' words reach the GPU only once it has
// finished drawing. DrawSync(1) gives 1 before the call, as on Sony's, and 2
// after it where Sony's gives 1: the clean library counts the image (spec 39,
// Results). Build the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh imgq exp/cleanlib/spec/probes/imgq
//
// and pass -imgq=$PWD/.build/imgq-cimg-clean/imgq-cimg-clean.cue.
func TestQueuedImages(t *testing.T) {
	if *imgq == "" {
		t.Skip("-imgq names the probe's disc")
	}
	if _, err := os.Stat(*imgq); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*imgq), emutest.AllowFaults())
	m.SetDrawTime(true)
	// The words that start an image's transfer, A0h and C0h, sent while the
	// GPU was still drawing. Nothing else the probe sends has those top bytes.
	early := 0
	m.OnGP0(func(w uint32) {
		if (w>>24 == 0xa0 || w>>24 == 0xc0) && m.GPUDrawing() {
			early++
		}
	})
	m.RunUntilTTY("END", 30*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	t.Log("\n" + tty)

	var noimg, img, store []string
	for line := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
		case f[0] == "NOIMG":
			noimg = f
		case f[0] == "IMG":
			img = f
		case f[0] == "STORE":
			store = f
		}
	}
	if len(noimg) == 0 || len(img) == 0 || len(store) == 0 {
		t.Fatal("the probe's lines are missing")
	}
	num := func(f []string, after string) int {
		t.Helper()
		for i := range len(f) - 1 {
			if f[i] == after {
				var n int
				fmt.Sscan(f[i+1], &n)
				return n
			}
		}
		t.Fatalf("no %q in %q", after, strings.Join(f, " "))
		return 0
	}

	// The frame alone: a console takes 31,087 eighths, the emulator's rough
	// cost more.
	frame := num(noimg, "total")
	assert.Between(t, 20000, 60000, frame, "the frame takes its time to draw")

	assert.Between(t, 1, 400, num(img, "loadimage"), "LoadImage returns at once")
	assert.Equal(t, 1, num(img, "DrawSync(1)"), "DrawSync(1) before LoadImage")
	assert.Equal(t, 2, num(img, "then"), "DrawSync(1) after LoadImage")
	assert.Equal(t, 128, num(img, "unchanged"), "the image is not drawn over")
	assert.Between(t, frame, frame+1000, num(img, "total"), "DrawSync(0) waits for the frame and the image")

	assert.Between(t, 1, 400, num(store, "storeimage"), "StoreImage returns at once")
	assert.Equal(t, 1, num(store, "DrawSync(1)"), "DrawSync(1) before StoreImage")
	assert.Equal(t, 2, num(store, "then"), "DrawSync(1) after StoreImage")
	assert.Equal(t, 128, num(store, "return"), "StoreImage's buffer is untouched when it returns")
	assert.Equal(t, 128, num(store, "picture"), "StoreImage reads the finished picture")

	assert.Equal(t, 0, early, "no image goes to the GPU while it draws")
}
