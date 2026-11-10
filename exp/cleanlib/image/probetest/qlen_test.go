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

var qlen = flag.String("qlen", "", "the cue sheet of the qlen probe's disc, as probe.sh builds it, for TestQueueLength")

// TestQueueLength runs spec 41's probe, spec/probes/qlen, with the GPU taking
// time to draw (emu's SetDrawTime), and checks its first run against Sony's
// library (spec 45): behind a clear and one 640 by 480 box, 63 LoadImage
// calls go into the queue and return at once, the 64th waits in the call,
// and DrawSync(1) after the nth image queued gives n, the drawing running
// ahead of them not counted. Build the disc with
//
//	SIDES=clean PROBE_DRAWTIME=1 exp/cleanlib/image/probe.sh qlen exp/cleanlib/spec/probes/qlen
//
// and pass -qlen=$PWD/.build/qlen-cimg-clean/qlen-cimg-clean.cue.
func TestQueueLength(t *testing.T) {
	if *qlen == "" {
		t.Skip("-qlen names the probe's disc")
	}
	if _, err := os.Stat(*qlen); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*qlen), emutest.AllowFaults())
	m.SetDrawTime(true)
	m.RunUntilTTY("QLEN run 1", 60*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	var line string
	for l := range strings.SplitSeq(tty, "\n") {
		if strings.HasPrefix(l, "QLEN run 0 ") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no QLEN line in\n%s", tty)
	}
	t.Log(line)
	// Each call's "i:took/DrawSync(1)", took in eighths of the system clock.
	calls := strings.Fields(line)[3:]
	assert.Equal(t, 80, len(calls), "calls")
	for i, c := range calls[:64] {
		var n, took, sync int
		if _, err := fmt.Sscanf(c, "%d:%d/%d", &n, &took, &sync); err != nil {
			t.Fatalf("call %q: %v", c, err)
		}
		if i < 63 {
			assert.Between(t, 0, 200, took, fmt.Sprintf("call %d returns at once", i))
			assert.Equal(t, i+1, sync, fmt.Sprintf("DrawSync(1) after call %d", i))
		} else {
			assert.Between(t, 5000, 20000, took, "the 64th call waits")
		}
	}
}
