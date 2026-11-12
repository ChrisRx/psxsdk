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

var imgsync = flag.String("imgsync", "", "the cue sheet of the imgsync probe's disc, as probe.sh builds it, for TestImageSync")

// imgsyncCase is what Sony's library does in one case of spec/probes/imgsync:
// the call's length from the marker before it to the one after, how long
// before that second marker the channel started (0 if it did not), the
// length of the DrawSync(1) after it, what that returned, and how many more
// polls of DrawSync(1) came before a 0.
type imgsyncCase struct {
	kind, size        string
	call, start, sync int
	gave, polls       int
}

// Sony's, in the emulator with the drawing time on (spec 49).
var imgsyncCases = []imgsyncCase{
	{"LOAD", "16x16", 700, 108, 118, 1, 0},
	{"LOAD", "16x1", 796, 0, 128, 0, 0},
	{"LOAD", "20x2", 766, 108, 128, 0, 0},
	{"LOAD", "0x4", 590, 0, 128, 0, 0},
	{"LOAD", "64x64", 700, 108, 118, 1, 30},
	{"LOAD", "256x256", 700, 108, 118, 1, 513},
	{"STORE", "16x16", 712, 106, 128, 0, 0},
	{"STORE", "16x1", 808, 0, 128, 0, 0},
	{"STORE", "20x2", 778, 106, 128, 0, 0},
	{"STORE", "0x4", 586, 0, 128, 0, 0},
	{"STORE", "64x64", 712, 106, 128, 0, 0},
	{"STORE", "256x256", 712, 106, 128, 0, 0},
	{"MOVE", "16x16", 472, 92, 134, 1, 0},
	{"MOVE", "16x1", 472, 92, 128, 0, 0},
	{"MOVE", "20x2", 472, 92, 128, 0, 0},
	{"MOVE", "0x4", 110, 0, 128, 0, 0},
	{"MOVE", "64x64", 472, 92, 134, 1, 31},
	{"MOVE", "256x256", 472, 92, 134, 1, 526},
	{"CLEAR", "16x16", 732, 108, 134, 1, 0},
	{"CLEAR", "16x1", 732, 108, 128, 0, 0},
	{"CLEAR", "20x2", 732, 108, 128, 0, 0},
	{"CLEAR", "0x4", 732, 108, 128, 0, 0},
	{"CLEAR", "64x64", 732, 108, 134, 1, 31},
	{"CLEAR", "256x256", 732, 108, 134, 1, 526},
	{"FILL", "16x16", 594, 104, 134, 1, 6},
	{"FILL", "16x1", 594, 104, 128, 0, 0},
	{"FILL", "20x2", 594, 104, 128, 0, 0},
	{"FILL", "0x4", 594, 104, 134, 1, 0},
	{"FILL", "64x64", 594, 104, 134, 1, 31},
	{"FILL", "256x256", 594, 104, 134, 1, 526},
}

// TestImageSync runs spec 49's probe, spec/probes/imgsync, with the GPU
// taking time to draw (emu's SetDrawTime), and checks the image calls made
// with the GPU free against Sony's library: each call's length, when its
// transfer starts before it returns, and what a DrawSync(1) straight after
// gives and takes. A free LoadImage of whole blocks starts them 108 cycles
// before the marker after it, so they are still going when it returns and
// DrawSync(1) gives 1; until spec 49 the clean library started them 272
// before and gave 0. It also checks what MoveImage returns for a rectangle
// of no size, and DrawSync(1)'s count with requests queued behind a drawing.
// Build the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh imgsync exp/cleanlib/spec/probes/imgsync
//
// and pass -imgsync=$PWD/.build/imgsync-cimg-clean/imgsync-cimg-clean.cue.
func TestImageSync(t *testing.T) {
	if *imgsync == "" {
		t.Skip("-imgsync names the probe's disc")
	}
	if _, err := os.Stat(*imgsync); err != nil {
		t.Fatal(err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*imgsync), emutest.AllowFaults())
	m.SetDrawTime(true)

	// From outside: the probe's markers (GP1 10000200h plus 4 a case, the
	// call between n and n+1 and the DrawSync(1) between n+1 and n+2) and
	// the writes that start channel 2.
	type timing struct {
		begin, started, end, synced uint64
	}
	var seen [2][]timing
	cur := -1
	round := 0
	m.OnBus(func(addr uint32, size int, v uint32, write bool) {
		if !write {
			return
		}
		now := m.CPU.TotalCycles
		switch addr & 0x1fffffff {
		case 0x1f801814:
			if v>>8 != 0x100002 {
				return
			}
			n := int(v & 0xff)
			round, cur = n/128, n%128/4
			for len(seen[round]) <= cur {
				seen[round] = append(seen[round], timing{})
			}
			s := &seen[round][cur]
			switch n % 4 {
			case 0:
				*s = timing{begin: now}
			case 1:
				s.end = now
			case 2:
				s.synced = now
				cur = -1
			}
		case 0x1f8010a8:
			if cur >= 0 && v&(1<<24) != 0 && seen[round][cur].end == 0 {
				seen[round][cur].started = now
			}
		}
	})
	m.RunUntilTTY("END", 120*emutest.Second)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")

	lines := map[string]string{}
	var moves, queued []string
	for l := range strings.SplitSeq(tty, "\n") {
		f := strings.Fields(l)
		switch {
		case len(f) == 8 && f[2] == "round":
			lines[f[0]+" "+f[1]+" "+f[3]] = strings.Join(f[4:], " ")
		case len(f) > 0 && f[0] == "MOVEIMAGE":
			moves = append(moves, strings.Join(f[1:], " "))
		case len(f) > 0 && f[0] == "QUEUED":
			queued = append(queued, strings.Join(f[3:8], " "))
		}
	}
	for r := range 2 {
		if len(seen[r]) != len(imgsyncCases) {
			t.Fatalf("round %d: %d cases seen, want %d\n%s", r, len(seen[r]), len(imgsyncCases), tty)
		}
		for i, c := range imgsyncCases {
			name := fmt.Sprintf("%s %s round %d", c.kind, c.size, r)
			s := seen[r][i]
			assert.Between(t, c.call-4, c.call+4, int(s.end-s.begin), name+": the call's length")
			start := 0
			if s.started != 0 {
				start = int(s.end - s.started)
			}
			assert.Between(t, max(c.start-4, 0), c.start+4, start, name+": the transfer's start before the return")
			assert.Between(t, c.sync-2, c.sync+2, int(s.synced-s.end), name+": DrawSync(1)'s length")
			var gave, polls int
			if _, err := fmt.Sscanf(lines[fmt.Sprintf("%s %s %d", c.kind, c.size, r)], "sync %d polls %d", &gave, &polls); err != nil {
				t.Fatalf("%s: %v in\n%s", name, err, tty)
			}
			assert.Equal(t, c.gave, gave, name+": DrawSync(1) straight after")
			assert.Between(t, c.polls-c.polls/50-1, c.polls+c.polls/50+1, polls, name+": polls of DrawSync(1) to 0")
		}
	}
	// A rectangle of no width or no height is not moved, and one of a
	// negative size is.
	assert.Equal(t, []string{
		"16 16 returns 0",
		"0 4 returns -1",
		"4 0 returns -1",
		"0 0 returns -1",
		"-1 4 returns 0",
		"4 -1 returns 0",
		"1 1 returns 0",
	}, moves, "MoveImage by size")
	// Eight image calls queued behind a drawing: DrawSync(1) counts them.
	assert.Equal(t, []string{"sync 8 8 8 8", "sync 8 8 8 8"}, queued, "DrawSync(1) with eight requests waiting")
}
