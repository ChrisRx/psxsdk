package libps_test

import (
	"debug/elf"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/format/psxdisc"
)

// TestMeasure prints what the experiment is judged by, for each program
// under each library: the emulated cycles a frame's work takes, which is
// the frame outside VSync, over 120 frames after the first 120; the frame
// itself; the program's text size; and its executable's size on the disc.
// It runs only with PSX_MEASURE set, and needs every disc built.
func TestMeasure(t *testing.T) {
	if os.Getenv("PSX_MEASURE") == "" {
		t.Skip("set PSX_MEASURE=1 to measure")
	}
	programs := []string{"hello", "sprites", "tiles", "timers", "breakout", "invaders", "dino", "car", "dungeon", "tunnel", "morph", "terra"}
	// The programs written against exp/ps as well, under NAME-ps, which
	// say what the API costs over the library's own calls.
	ported := map[string]bool{"tiles": true, "timers": true, "invaders": true, "dino": true, "square": true}
	programs = append(programs, "square")
	for _, program := range programs {
		names := []string{program, program + "-go"}
		if ported[program] {
			names = append(names, program+"-ps")
		}
		for _, name := range names {
			cue := disc(t, name)
			m := boot(t, name)
			m.RunUntilTTY("Execute !", 30*emutest.Second)
			m.RunFrames(120)
			work, frame := workCycles(t, m, 120)

			f, err := elf.Open(filepath.Join(filepath.Dir(cue), "prog.elf"))
			if err != nil {
				t.Fatal(err)
			}
			text := f.Section(".text").Size
			f.Close()

			r, closer, err := psxdisc.Open(filepath.Join(filepath.Dir(cue), name+".bin"))
			if err != nil {
				t.Fatal(err)
			}
			exe := r.Root.Find("MAIN.EXE").Size
			closer.Close()
			t.Logf("%s: %d work cycles/frame, %d cycles/frame, text %d bytes, exe %d bytes", name, work, frame, text, exe)
		}
	}
}

// workCycles runs n frames one instruction at a time and returns the
// cycles a frame took outside VSync, from its return to the next call,
// along with the cycles of the frame. Both libraries keep VSync as a
// symbol, and a call into it is told from its return by the return
// address the call left.
func workCycles(t *testing.T, m *emutest.Machine, n int) (work, frame emutest.Cycles) {
	t.Helper()
	vsync := m.Sym("VSync")
	target := m.Frames() + uint64(n)
	start := m.Cycles()
	var ra uint32
	inside := false
	for m.Frames() < target {
		pc := m.CPU.PC
		if !inside && pc == vsync {
			inside, ra = true, m.CPU.R[31]
		} else if inside && pc == ra {
			inside = false
		}
		before := m.Cycles()
		m.Step()
		if !inside {
			work += m.Cycles() - before
		}
	}
	return work / emutest.Cycles(n), (m.Cycles() - start) / emutest.Cycles(n)
}
