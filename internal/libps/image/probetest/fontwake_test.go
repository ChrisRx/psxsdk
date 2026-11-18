package probetest

import (
	"flag"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var fontwake = flag.String("fontwake", "", "the cue sheet of the fontwake probe's disc, as probe.sh builds it, for TestFontWake")

// TestFontWake runs spec 43's probe, spec/probes/fontwake, with the GPU
// taking time to draw, and checks that a FntFlush queued behind a frame's
// drawing goes out as Sony's does: channel 2's completion interrupt turned on
// for it, the handler waiting for the GPU and sending it, and the text's list
// done by the next VSync(0)'s return, so that the next frame's ResetGraph(1)
// does not stop it partway through. Build the disc with
//
//	SIDES=clean PROBE_DRAWTIME=1 internal/libps/image/probe.sh fontwake internal/libps/spec/probes/fontwake
//
// and pass -fontwake=$PWD/.build/fontwake-cimg-clean/fontwake-cimg-clean.cue.
func TestFontWake(t *testing.T) {
	if *fontwake == "" {
		t.Skip("-fontwake names the probe's disc")
	}
	if _, err := os.Stat(*fontwake); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*fontwake), emutest.AllowFaults())
	m.SetDrawTime(true)
	m.RunUntilTTY("END", 60*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	t.Log("\n" + tty)

	// Sony's, in the emulator with the drawing time on (spec 43).
	sony := []string{
		"FWAKE part 0 k 0 frames 11 idle 11 font 11 irqdraw 0 irqflush 0 away 0 least 143 most 262",
		"FWAKE part 0 k 1 frames 11 idle 11 font 11 irqdraw 0 irqflush 11 away 11 least 288 most 288",
		"FWAKE part 0 k 2 frames 11 idle 11 font 11 irqdraw 0 irqflush 11 away 11 least 65535 most 0",
		"FWAKE part 1 k 0 frames 11 idle 11 font 11 irqdraw 0 irqflush 0 away 0 least 65535 most 0",
		"FWAKE part 1 k 1 frames 11 idle 10 font 10 irqdraw 1 irqflush 11 away 0 least 65535 most 0",
		"FWAKE part 1 k 2 frames 11 idle 0 font 0 irqdraw 11 irqflush 11 away 0 least 65535 most 0",
	}
	var got []string
	for line := range strings.SplitSeq(tty, "\n") {
		if strings.HasPrefix(line, "FWAKE ") {
			got = append(got, line)
		}
	}
	assert.Equal(t, sony, got, "the summary lines")
}
