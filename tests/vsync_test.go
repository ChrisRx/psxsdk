package tests

import (
	"hash/fnv"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu/emutest"
)

// TestVSyncSkipChangesNothing runs examples on two machines, one skipping
// through the library's wait for a vertical blank and one stepping through
// it, and checks that they agree at every vertical blank, which mostly
// comes while the program waits: the CPU, the clocks, the display and the
// sound, and every tenth frame all of RAM, where the wait counts down.
func TestVSyncSkipChangesNothing(t *testing.T) {
	parallel(t)
	for _, name := range []string{"dino", "check", "morph"} {
		t.Run(name, func(t *testing.T) {
			parallel(t)
			cue := disc(t, name)
			boot := func(skip bool) *emutest.Machine {
				m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(cue),
					emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")))
				m.SkipVSyncWait(skip)
				m.SetAudio(true)
				m.RunUntilTTY("Execute !", 30*emutest.Second)
				return m
			}
			skipped, stepped := boot(true), boot(false)

			var a, b []int16
			for f := range 600 {
				// RunFrame stops at the blank itself, where RunFrames runs
				// on a little past it.
				for _, m := range []*emutest.Machine{skipped, stepped} {
					if _, err := m.RunFrame(4_000_000); err != nil {
						t.Fatal(err)
					}
				}
				a, b = skipped.Audio(a[:0]), stepped.Audio(b[:0])
				assert.Equal(t, machineStateOf(stepped), machineStateOf(skipped), "frame %d", f)
				assert.Equal(t, b, a, "frame %d's sound", f)
				if f%10 == 0 {
					assert.Equal(t, ramHash(stepped), ramHash(skipped), "frame %d's RAM", f)
				}
				if t.Failed() {
					return
				}
			}
		})
	}
}

// machineState is what a frame leaves of the CPU and its clocks, and a hash
// of VRAM.
type machineState struct {
	R                [32]uint32
	HI, LO, PC       uint32
	COP0             [16]uint32
	TotalCycles      uint64
	Frames, VRAMHash uint64
}

func machineStateOf(m *emutest.Machine) machineState {
	h := fnv.New64a()
	for _, v := range m.VRAM() {
		h.Write([]byte{byte(v), byte(v >> 8)})
	}
	c := m.CPU
	return machineState{c.R, c.HI, c.LO, c.PC, c.COP0, c.TotalCycles, m.Frames(), h.Sum64()}
}

func ramHash(m *emutest.Machine) uint64 {
	h := fnv.New64a()
	for a := uint32(0x80000000); a < 0x80200000; a += 4 {
		v := m.Bus.Read32(a)
		h.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
	}
	return h.Sum64()
}
