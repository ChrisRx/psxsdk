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

var fieldtime = flag.String("fieldtime", "", "the cue sheet of the fieldtime probe's disc, as probe.sh builds it, for TestFieldTime")

// TestFieldTime runs spec 41's probe, spec/probes/fieldtime, with the GPU
// taking time to draw (emu's SetDrawTime), and checks that LoadImage calls
// queued behind a long drawing go out as quickly as on Sony's library: the
// first as soon as the GPU has finished drawing, and each one after it as
// soon as the one before has gone out, all in the same interrupt. Behind a
// clear and one 640 by 480 box, the GPU goes idle at Sony's times with 0, 8
// and 32 images. Build the disc with
//
//	SIDES=clean internal/libps/image/probe.sh fieldtime internal/libps/spec/probes/fieldtime
//
// and pass -fieldtime=$PWD/.build/fieldtime-cimg-clean/fieldtime-cimg-clean.cue.
func TestFieldTime(t *testing.T) {
	if *fieldtime == "" {
		t.Skip("-fieldtime names the probe's disc")
	}
	if _, err := os.Stat(*fieldtime); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*fieldtime), emutest.AllowFaults())
	m.SetDrawTime(true)
	m.RunUntilTTY("END", 60*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	t.Log("\n" + tty)

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
	// Sony's, in the emulator with the drawing time on and an image's DMA
	// taking time (since c31746e): the mean time from VSync's return to the
	// GPU going idle, in microseconds, behind one box and the clear, by the
	// images queued behind them (spec 45).
	sony := map[int]int{0: 18621, 8: 18999, 32: 19684}
	seen := 0
	for line := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] != "FIELD" || num(f, "wait") != 0 || num(f, "boxes") != 1 {
			continue
		}
		n := num(f, "images")
		want, ok := sony[n]
		if !ok {
			continue
		}
		seen++
		name := fmt.Sprintf("1 box, %d images", n)
		assert.Equal(t, true, strings.Contains(line, " never idle 0 "), name+": the GPU goes idle every frame")
		// The line ends "idle mean N max M".
		assert.Between(t, want-40, want+40, num(f[len(f)-4:], "mean"), name+": idle at Sony's time")
	}
	// The 8-image line comes twice, from the sweep of boxes and from the
	// sweep of images.
	assert.Equal(t, 4, seen, "lines for one box")
}
