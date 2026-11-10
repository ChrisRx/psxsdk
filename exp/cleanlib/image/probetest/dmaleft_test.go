package probetest

import (
	"flag"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var dmaleft = flag.String("dmaleft", "", "the cue sheet of the dmaleft probe's disc, as probe.sh builds it, for TestDMALeft")

// dmaleftSony is what spec/probes/dmaleft prints on Sony's library in the
// emulator: DPCR and the GPU's, the sound's and the ordering tables'
// channel control registers after each call. Sony's start sets every
// channel's priority to 3 and disables them all, ResetGraph and the sound
// enable theirs, GsClearOt the ordering tables', and a read of the disc the
// drive's. ResetGraph(1) and (3) leave D2_CHCR 401h, as (0) does.
var dmaleftSony = []string{
	"DMA entry : DPCR 333b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA ResetGraph(0) : DPCR 333b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA GsInitGraph : DPCR 333b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA GsClearOt : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA GsDrawOt : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA DrawSync(0) : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA LoadImage : DPCR 3b3b3b33 D2 00000201 D4 00000000 D6 00000002",
	"DMA LoadImage DrawSync(0) : DPCR 3b3b3b33 D2 00000201 D4 00000000 D6 00000002",
	"DMA StoreImage DrawSync(0) : DPCR 3b3b3b33 D2 00000200 D4 00000000 D6 00000002",
	"DMA VSync(0) : DPCR 3b3b3b33 D2 00000200 D4 00000000 D6 00000002",
	"DMA ResetGraph(1) : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA frame DrawSync(0) : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA ResetGraph(1) after a frame : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA frame running : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA ResetGraph(1) on a running frame : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA ResetGraph(3) : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA ResetGraph(0) again : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA frame DrawSync(0) : DPCR 3b3b3b33 D2 00000401 D4 00000000 D6 00000002",
	"DMA CdReadFile : DPCR 3b3bbb33 D2 00000401 D4 00000000 D6 00000002",
	"DMA CdReadSync(0) : DPCR 3b3bbb33 D2 00000401 D4 00000000 D6 00000002",
}

// TestDMALeft runs spec 45's probe, spec/probes/dmaleft, and checks that the
// clean library leaves DPCR and the channels' control registers as Sony's
// does after each call, so that a program returns with them as Sony's
// leaves them (spec 44 found DPCR 08089899h and D2_CHCR 0 after rerun, where
// Sony's left 3B3B3B33h and 401h). Build the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh dmaleft exp/cleanlib/spec/probes/dmaleft
//
// and pass -dmaleft=$PWD/.build/dmaleft-cimg-clean/dmaleft-cimg-clean.cue.
func TestDMALeft(t *testing.T) {
	if *dmaleft == "" {
		t.Skip("-dmaleft names the probe's disc")
	}
	if _, err := os.Stat(*dmaleft); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*dmaleft), emutest.AllowFaults())
	m.RunUntilTTY("END", 30*emutest.Second)
	var got []string
	for line := range strings.SplitSeq(strings.ReplaceAll(m.TTY(), "\r", ""), "\n") {
		if strings.HasPrefix(line, "DMA ") {
			got = append(got, line)
		}
	}
	assert.Equal(t, dmaleftSony, got)
}
