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

var vsyncphase = flag.String("vsyncphase", "", "the cue sheet of the vsyncphase probe's disc, as probe.sh builds it, for TestVSyncPhase")

// TestVSyncPhase runs spec 42's probe, spec/probes/vsyncphase, and checks
// that VSync(0) on a 640 by 480 interlaced display returns where Sony's
// does: once GPUSTAT bit 31 has changed after the vertical blank, past the
// blank in every frame, and on a 240-line display as the blank is counted.
// Build the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh vsyncphase exp/cleanlib/spec/probes/vsyncphase
//
// and pass -vsyncphase=$PWD/.build/vsyncphase-cimg-clean/vsyncphase-cimg-clean.cue.
func TestVSyncPhase(t *testing.T) {
	if *vsyncphase == "" {
		t.Skip("-vsyncphase names the probe's disc")
	}
	if _, err := os.Stat(*vsyncphase); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*vsyncphase), emutest.AllowFaults())
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
	seen := map[string]bool{}
	for line := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[1] != "frames" {
			continue
		}
		// Sony's, in the emulator: 278 lines from the odd field's end to
		// VSync(0)'s return at 480 lines, past the blank every frame, and
		// 262 at 240.
		switch f[0] {
		case "VPHASE":
			frames := num(f, "frames")
			assert.Between(t, 100, 240, frames, "480 lines: frames measured")
			assert.Equal(t, 278, num(f, "least"), "480 lines: least")
			assert.Equal(t, 278, num(f, "most"), "480 lines: most")
			assert.Equal(t, frames, num(f, "blank"), "480 lines: past the blank in every frame")
		case "VPHASE240":
			assert.Between(t, 100, 240, num(f, "frames"), "240 lines: frames measured")
			assert.Equal(t, 262, num(f, "least"), "240 lines: least")
			assert.Equal(t, 262, num(f, "most"), "240 lines: most")
		}
		seen[f[0]] = true
	}
	assert.Equal(t, true, seen["VPHASE"], "the 480-line line")
	assert.Equal(t, true, seen["VPHASE240"], "the 240-line line")
}
