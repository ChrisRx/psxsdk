package probetest

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var vsynccount = flag.String("vsynccount", "", "the cue sheet of the vsynccount probe's disc, as probe.sh builds it, for TestVSyncCount")

// The scratchpad words vsynccount writes: its VSyncCallback's count, and
// each new VSync(-1) its loop sees.
const (
	countCallback = 0x1f800300
	countSeen     = 0x1f800304
)

// blankTimes is one vertical blank interrupt, in cycles from the cycle
// I_STAT's bit 0 was seen set; -1 where it did not happen.
type blankTimes struct {
	field                       uint32 // GPUSTAT bit 13 at the blank
	taken, padFirst, padLast    int64
	ack, callback, seen         int64
	count                       [4]int64
	padAccesses, iStatAckWrites int
}

// measureBlanks runs the vsynccount probe on disc and times n vertical
// blank interrupts after its "measure" line: when the CPU takes the
// interrupt, the controller driver's first and last access to the
// controller port (1F801040h to 1F80104Fh), the write to I_STAT that
// clears bit 0, the change of the library's count of blanks (found as the
// one word of RAM that holds what VSync(-1) returned), the VSyncCallback's
// write and the program's write of the new count.
func measureBlanks(t *testing.T, disc string, n int) ([]blankTimes, string) {
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.AllowFaults())
	m.RunUntilTTY("measure", 30*emutest.Second)

	// Find the count: the words of RAM, past the kernel, that hold what the
	// program last wrote at countSeen, each time it writes one.
	var cands []uint32
	seenWrites := 0
	var cur *blankTimes
	var out []blankTimes
	quiet := false
	var trace *strings.Builder
	if os.Getenv("VSYNCCOUNT_TRACE") != "" {
		trace = &strings.Builder{}
		defer func() { t.Log("\n" + trace.String()) }()
	}
	var start int64 = -1
	rel := func() int64 { return int64(m.CPU.TotalCycles) - start }
	m.OnBus(func(addr uint32, size int, v uint32, write bool) {
		if quiet {
			return
		}
		a := addr & 0x1fffffff
		if trace != nil && len(out) == 10 && a != 0x1f801814 {
			dir := "r"
			if write {
				dir = "w"
			}
			fmt.Fprintf(trace, "%s %08x %d %08x at %d\n", dir, a, size, v, rel())
		}
		switch {
		case a == countSeen && write:
			seenWrites++
			if cands == nil {
				for p := uint32(0x10000); p < 0x1fff00; p += 4 {
					if m.Read32(0x80000000|p) == v {
						cands = append(cands, 0x80000000|p)
					}
				}
			} else if cur == nil || seenWrites < 6 {
				keep := cands[:0]
				for _, p := range cands {
					if m.Read32(p) == v {
						keep = append(keep, p)
					}
				}
				cands = keep
			}
			if cur != nil && cur.seen < 0 {
				cur.seen = rel()
			}
		case a == countCallback && write:
			if cur != nil && cur.callback < 0 {
				cur.callback = rel()
			}
		case a >= 0x1f801040 && a < 0x1f801050:
			if cur != nil {
				if cur.padFirst < 0 {
					cur.padFirst = rel()
				}
				cur.padLast = rel()
				cur.padAccesses++
			}
		case a == 0x1f801070 && write && v&1 == 0:
			if cur != nil {
				cur.iStatAckWrites++
				if cur.ack < 0 {
					cur.ack = rel()
				}
			}
		}
	})
	// Settle the candidates over a few counts first.
	m.RunUntil(func() bool { return seenWrites >= 6 }, 2*emutest.Second, "six counts seen")
	if len(cands) == 0 || len(cands) > 4 {
		t.Fatalf("%d words of RAM hold the count: %08x", len(cands), cands)
	}
	var count [4]uint32
	for i, p := range cands {
		count[i] = m.Read32(p)
	}
	pending := false
	budget := m.Cycles() + emutest.Cycles(n+2)*emutest.Second/50
	for len(out) < n && m.Cycles() < budget {
		m.Step()
		if m.CPU.Excode == cpu.ExcInt && cur != nil && cur.taken < 0 {
			cur.taken = rel()
		}
		for i, p := range cands {
			if c := m.Read32(p); c != count[i] {
				count[i] = c
				if cur != nil && cur.count[i] < 0 {
					cur.count[i] = rel()
				}
			}
		}
		quiet = true
		stat := m.Read32(0x1f801070)
		gpu := m.Read32(0x1f801814)
		quiet = false
		if stat&1 != 0 && !pending {
			pending = true
			if cur != nil {
				out = append(out, *cur)
			}
			start = int64(m.CPU.TotalCycles)
			cur = &blankTimes{field: gpu >> 13 & 1, taken: -1, padFirst: -1, padLast: -1, ack: -1, count: [4]int64{-1, -1, -1, -1}, callback: -1, seen: -1}
		} else if stat&1 == 0 {
			pending = false
		}
	}
	return out, fmt.Sprintf("count at %08x", cands)
}

// TestVSyncCount runs spec 45's probe, spec/probes/vsynccount, and checks
// that the clean library counts the vertical blank where in the interrupt
// Sony's does. In the emulator both count it at the end of the interrupt,
// in the library's handler that the kernel goes to after its own handlers,
// the controller driver's among them, the same in every field: Sony's
// acknowledges the blank 10,246 cycles after I_STAT bit 0 is raised,
// counts it at 10,298 and writes the probe's callback word at 10,326 (10,330
// with its VSync(-1)). The clean library did each about 80 cycles sooner.
// Build the discs with
//
//	exp/cleanlib/image/probe.sh vsynccount exp/cleanlib/spec/probes/vsynccount
//
// and pass -vsynccount with the cue sheet of either side's disc
// (.build/vsynccount-cimg-clean/vsynccount-cimg-clean.cue for the clean
// one). VSYNCCOUNT_TRACE=1 logs the I/O of one interrupt.
func TestVSyncCount(t *testing.T) {
	if *vsynccount == "" {
		t.Skip("-vsynccount names the probe's disc")
	}
	if _, err := os.Stat(*vsynccount); err != nil {
		t.Fatal(err)
	}
	blanks, where := measureBlanks(t, *vsynccount, 24)
	var b strings.Builder
	fmt.Fprintf(&b, "%s; cycles from I_STAT bit 0:\n", where)
	fmt.Fprintf(&b, "field taken padFirst padLast padN ack ackN count callback seen\n")
	for _, x := range blanks {
		fmt.Fprintf(&b, "%d %d %d %d %d %d %d %v %d %d\n", x.field, x.taken, x.padFirst, x.padLast, x.padAccesses, x.ack, x.iStatAckWrites, x.count, x.callback, x.seen)
	}
	t.Log("\n" + b.String())
	assert.Between(t, 20, 24, len(blanks), "blanks timed")
	fields := map[uint32]int{}
	for i, x := range blanks {
		name := fmt.Sprintf("blank %d", i)
		fields[x.field]++
		assert.Equal(t, int64(0), x.taken, name+": the interrupt taken at once")
		assert.Equal(t, true, x.padLast > 0 && x.padLast < x.count[0], name+": the controller driver before the count")
		assert.Equal(t, 1, x.iStatAckWrites, name+": one write acknowledges the blank")
		assert.Between(t, 10244, 10248, int(x.ack), name+": acknowledged at Sony's cycle")
		assert.Between(t, 10296, 10300, int(x.count[0]), name+": counted at Sony's cycle")
		assert.Between(t, 10320, 10334, int(x.callback), name+": the callback at Sony's cycle")
		assert.Equal(t, true, x.seen > x.callback, name+": the program sees the count after the callback")
	}
	assert.Equal(t, 2, len(fields), "blanks after both fields")
}
