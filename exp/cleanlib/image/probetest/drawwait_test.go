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

var drawwait = flag.String("drawwait", "", "the cue sheet of the drawwait probe's disc, as probe.sh builds it, for TestDrawWait")

// TestDrawWait runs spec 40's probe, spec/probes/drawwait, with the GPU taking
// time to draw (emu's SetDrawTime), and checks that a GsDrawOt that finds the
// queue empty and channel 2 idle, with the GPU still drawing, behaves as on
// Sony's library: it waits in the call until the GPU is ready for a command,
// and returns with channel 2 sending its list, DrawSync(1) giving 1. That
// holds once the first list's DMA has ended (tail) and after ResetGraph(1)
// has stopped it (reset), and everything is done when it is on Sony's. Build
// the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh drawwait exp/cleanlib/spec/probes/drawwait
//
// and pass -drawwait=$PWD/.build/drawwait-cimg-clean/drawwait-cimg-clean.cue.
func TestDrawWait(t *testing.T) {
	if *drawwait == "" {
		t.Skip("-drawwait names the probe's disc")
	}
	if _, err := os.Stat(*drawwait); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*drawwait), emutest.AllowFaults())
	m.SetDrawTime(true)
	m.RunUntilTTY("END", 30*emutest.Second)
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
	runs := map[string]int{}
	for line := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] != "WAIT" {
			continue
		}
		name := f[1]
		runs[name]++
		// Sony's, in the emulator with the drawing time on: the second call
		// takes 9,617 to 11,506 (tail) or 38,386 to 40,275 (reset), and
		// everything is done at 76,852 to 76,863.
		took := num(f, "took")
		switch name {
		case "tail":
			assert.Between(t, 5000, 20000, took, "tail: the second GsDrawOt waits for the GPU")
		case "reset":
			assert.Between(t, 30000, 45000, took, "reset: the second GsDrawOt waits for the GPU")
		}
		assert.Equal(t, 1, num(f, "busy"), name+": channel 2 is sending when the call returns")
		assert.Equal(t, 1, num(f, "DrawSync(1)"), name+": DrawSync(1) after the call")
		// The line ends "all done at N".
		assert.Between(t, 76500, 77200, num(f[len(f)-2:], "at"), name+": everything done at Sony's time")
	}
	assert.Equal(t, 3, runs["tail"], "tail runs")
	assert.Equal(t, 3, runs["reset"], "reset runs")
}
