package probetest

import (
	"flag"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var irqgaps = flag.String("irqgaps", "", "the cue sheet of the irqgaps probe's disc, as probe.sh builds it, for TestIRQGaps")

// TestIRQGaps runs spec 45's probe spec/probes/irqgaps, which polls timer 2,
// timer 1 and VSync(-1) after VSync(0) for three fields at 480 lines and
// three at 240, and checks that the program sees each new count of vertical
// blanks where it does on Sony's library: on a poll before the stretch the
// blank's interrupt takes, in all six fields. Both libraries count the blank
// at the same cycle in the interrupt; where the program sees it depends on
// where VSync(-1) reads the count and on where VSync(0) leaves the program
// when the next blank comes. The clean library saw three of six after the
// stretch, the 11 lines late that spec 45's console runs show. Build the
// disc with
//
//	SIDES=clean internal/libps/image/probe.sh irqgaps internal/libps/spec/probes/irqgaps
//
// and pass -irqgaps=$PWD/.build/irqgaps-cimg-clean/irqgaps-cimg-clean.cue.
func TestIRQGaps(t *testing.T) {
	if *irqgaps == "" {
		t.Skip("-irqgaps names the probe's disc")
	}
	if _, err := os.Stat(*irqgaps); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*irqgaps), emutest.AllowFaults())
	m.RunUntilTTY("END", 60*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	t.Log("\n" + tty)

	gaps := map[string]int{}
	display := ""
	seenBefore := false
	for line := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 3 && f[0] == "GAPS":
			display = f[1]
			seenBefore = false
		case len(f) > 1 && f[0] == "GAPS" && strings.Contains(line, "counted outside a gap"):
			seenBefore = true
		case len(f) > 1 && f[0] == "GAPS" && strings.Contains(line, ": gap"):
			gaps[display]++
			name := display + " " + strings.Join(f[1:3], " ")
			assert.Equal(t, true, strings.HasSuffix(line, "counted 0"), name+": not counted in the stretch")
			assert.Equal(t, true, seenBefore, name+": counted on a poll before the stretch")
			seenBefore = false
		}
	}
	assert.Equal(t, 3, gaps["480i"], "480 lines: stretches")
	assert.Equal(t, 3, gaps["240p"], "240 lines: stretches")
}
