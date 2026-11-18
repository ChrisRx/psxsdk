package probetest

import (
	"cmp"
	"flag"
	"hash/fnv"
	"slices"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

var terra = flag.Bool("terra", false, "run the Terra comparisons, on the discs terra.sh builds")

func bootTerra(t *testing.T, name string) *emutest.Machine {
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc("../../../../.build/"+name+"/"+name+".cue"), emutest.AllowFaults())
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	return m
}

// TestTerra compares the two images' pictures every 30 frames, with Start and
// Cross held now and then to move the game on.
func TestTerra(t *testing.T) {
	if !*terra {
		t.Skip("terra.sh runs this")
	}
	sony, clean := bootTerra(t, "terra-cimg"), bootTerra(t, "terra-cimg-clean")
	same := 0
	for i := range 150 {
		for _, m := range []*emutest.Machine{sony, clean} {
			if i%4 == 3 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
				m.RunFrames(28)
			} else {
				m.RunFrames(30)
			}
		}
		if clean.CPU.R[24] == 0xdead0000 {
			t.Fatalf("second %d: parked at the entry with low half %#x", i+1, clean.CPU.R[25])
		}
		a, b := sony.Display(), clean.Display()
		if a.Bounds() == b.Bounds() && string(a.Pix) == string(b.Pix) {
			same++
			continue
		}
		n := 0
		if a.Bounds() == b.Bounds() {
			for k := range a.Pix {
				if a.Pix[k] != b.Pix[k] {
					n++
				}
			}
		}
		t.Logf("second %d: %d components differ (%v, %v)", i+1, n, a.Bounds(), b.Bounds())
	}
	t.Logf("%d of 150 pictures the same", same)
	if same != 150 {
		t.Fail()
	}
}

// TestTerraStack reports the lowest sp in the scratchpad on each image, with the
// PC in the image and in the program, and stops where sp leaves the scratchpad.
func TestTerraStack(t *testing.T) {
	if !*terra {
		t.Skip("terra.sh runs this")
	}
	for _, name := range []string{"terra-cimg", "terra-cimg-clean"} {
		m := bootTerra(t, name)
		start := m.Frames()
		lib, prog, at := uint32(0x1f800400), uint32(0x1f800400), uint32(0)
		f, held := 0, 0
		for m.Frames() < start+4500 {
			pc, sp := m.CPU.PC, m.CPU.R[29]
			if sp >= 0x1f000000 && sp < 0x1f800400 {
				if pc >= 0x80010000 && pc < 0x80090000 {
					if sp < lib {
						lib, at = sp, pc
					}
				} else if pc >= 0x80090000 && pc < 0x80200000 && sp < prog {
					prog = sp
				}
				if sp < 0x1f800000 {
					t.Errorf("%s: sp %08x below the scratchpad at frame %d, pc %08x", name, sp, m.Frames()-start, pc)
					break
				}
			}
			if g := int(m.Frames() - start); g != f {
				f = g
				if f/30%4 == 3 && f%30 == 0 {
					m.Pad(1).Press(emu.ButtonStart | emu.ButtonCross)
					held = f + 2
				}
				if held != 0 && f >= held {
					m.Pad(1).Release(emu.ButtonStart | emu.ButtonCross)
					held = 0
				}
			}
			m.Step()
		}
		t.Logf("%s: %d frames, lowest sp %08x in the image (at pc %08x), %08x in the program", name, m.Frames()-start, lib, at, prog)
	}
}

// A pictureChange is a frame whose picture differs from the one before.
type pictureChange struct {
	frame int
	sum   uint64
}

// pictureChanges runs a number of frames and lists each picture change, holding
// Start and Cross for 2 frames at each frame in presses. It stops early if stop,
// which may be nil, says to at the end of a frame.
func pictureChanges(m *emutest.Machine, frames int, presses map[int]bool, stop func() bool) []pictureChange {
	var list []pictureChange
	held, last := 0, uint64(0)
	for f := 1; f <= frames; f++ {
		if presses[f] {
			m.Pad(1).Press(emu.ButtonStart | emu.ButtonCross)
			held = f + 2
		}
		if held != 0 && f >= held {
			m.Pad(1).Release(emu.ButtonStart | emu.ButtonCross)
			held = 0
		}
		before := m.Frames()
		// A frame is under 600,000 cycles, and an instruction takes at least one.
		if _, err := m.Machine.RunFrame(1 << 20); err != nil || m.Frames() == before {
			break
		}
		h := fnv.New64a()
		h.Write(m.Display().Pix)
		if s := h.Sum64(); s != last {
			list = append(list, pictureChange{
				frame: f,
				sum:   s,
			})
			last = s
		}
		if stop != nil && stop() {
			break
		}
	}
	return list
}

// comparePictures says whether the two lists hold the same pictures in the same
// order, and at which change the clean image's lead over Sony's moves.
func comparePictures(t *testing.T, sony, clean []pictureChange) {
	t.Logf("%d picture changes on Sony's, %d on clean", len(sony), len(clean))
	lead := 0
	for i := range min(len(sony), len(clean)) {
		if sony[i].sum != clean[i].sum {
			t.Errorf("change %d: different pictures, frame %d on Sony's, %d on clean", i, sony[i].frame, clean[i].frame)
			return
		}
		if d := sony[i].frame - clean[i].frame; d != lead {
			t.Logf("from Sony's frame %d, change %d: clean leads by %d", sony[i].frame, i, d)
			lead = d
		}
	}
}

// TestTerraOrder compares every picture change with no input.
func TestTerraOrder(t *testing.T) {
	if !*terra {
		t.Skip("terra.sh runs this")
	}
	sony := pictureChanges(bootTerra(t, "terra-cimg"), 4500, nil, nil)
	clean := pictureChanges(bootTerra(t, "terra-cimg-clean"), 4500, nil, nil)
	comparePictures(t, sony, clean)
}

// TestTerraOrderPressed compares every picture change with TestTerra's presses.
func TestTerraOrderPressed(t *testing.T) {
	if !*terra {
		t.Skip("terra.sh runs this")
	}
	presses := map[int]bool{}
	for k := range 37 {
		presses[91+120*k] = true
	}
	sony := pictureChanges(bootTerra(t, "terra-cimg"), 4500, presses, nil)
	clean := pictureChanges(bootTerra(t, "terra-cimg-clean"), 4500, presses, nil)
	comparePictures(t, sony, clean)
}

// entryCost is what a run spent in one library entry.
type entryCost struct {
	calls, cycles uint64
}

// libraryCosts runs to a frame and totals, by entry address, the cycles from a
// jump out of the program into the library until the program runs again. Entry
// 0 is the time in interrupts that began in the program.
func libraryCosts(m *emutest.Machine, frames int) map[uint32]entryCost {
	inProgram := func(pc uint32) bool { return pc >= 0x80090000 && pc < 0x80200000 }
	costs := map[uint32]entryCost{}
	start, at := m.Frames(), m.CPU.TotalCycles
	entry, was := uint32(0), inProgram(m.CPU.PC)
	// A frame is under 600,000 cycles, and an instruction takes at least one.
	for steps := frames << 20; int(m.Frames()-start) < frames && steps > 0; steps-- {
		if m.Machine.Step() != nil {
			break
		}
		pc := m.CPU.PC
		now := inProgram(pc)
		switch {
		case was && !now:
			entry, at = pc, m.CPU.TotalCycles
			if pc < 0x80010000 || pc >= 0x80090000 {
				entry = 0
			}
		case !was && now:
			c := costs[entry]
			c.calls++
			c.cycles += m.CPU.TotalCycles - at
			costs[entry] = c
		}
		was = now
	}
	return costs
}

// TestTerraCosts compares time in each library entry over a run with no input,
// largest difference first.
func TestTerraCosts(t *testing.T) {
	if !*terra {
		t.Skip("terra.sh runs this")
	}
	for _, frames := range []int{204, 824} {
		sony := libraryCosts(bootTerra(t, "terra-cimg"), frames)
		clean := libraryCosts(bootTerra(t, "terra-cimg-clean"), frames)
		var entries []uint32
		for e := range sony {
			entries = append(entries, e)
		}
		for e := range clean {
			if _, ok := sony[e]; !ok {
				entries = append(entries, e)
			}
		}
		diff := func(e uint32) int64 { return int64(sony[e].cycles) - int64(clean[e].cycles) }
		slices.SortFunc(entries, func(a, b uint32) int {
			return cmp.Compare(max(diff(b), -diff(b)), max(diff(a), -diff(a)))
		})
		total := int64(0)
		for _, e := range entries {
			total += diff(e)
		}
		t.Logf("to frame %d: Sony's spends %d more in the library", frames, total)
		for _, e := range entries[:min(25, len(entries))] {
			t.Logf("  %08x: Sony %6d calls %11d, clean %6d calls %11d, difference %d", e,
				sony[e].calls, sony[e].cycles, clean[e].calls, clean[e].cycles, diff(e))
		}
	}
}
