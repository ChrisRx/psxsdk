// Package tests runs the example programs in the emulator.
//
// The programs are built with TinyGo onto disc images first, each into a
// directory of its own under the one PSX_DISCS names, as task disc does
// under .build. The tests are given that and a BIOS by environment variable;
// without them they are skipped. task test:emu builds the discs and runs
// them.
package tests

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// disc returns the cue sheet of the named example's disc, or skips the
// test if there is none.
func disc(t *testing.T, name string) string {
	t.Helper()
	dir := os.Getenv("PSX_DISCS")
	if dir == "" {
		t.Skip("PSX_DISCS is not set")
	}
	path := filepath.Join(dir, name, name+".cue")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no disc for %s: %v", name, err)
	}
	return path
}

// TestCheckQuitReturnsToBIOS boots the check example and quits it with
// Select, which returns from main. The program should come back to the BIOS
// cleanly: through the crt0's saved return address, which the Yaroze
// startup stub has to preserve, and to a BIOS whose stack frame the
// program's stack has not overwritten, which needs the SYSTEM.CNF stack
// mkpsxiso writes. The BIOS then reports "End of Main".
//
// That message is Sony's BIOS's.
func TestCheckQuitReturnsToBIOS(t *testing.T) {
	cue := disc(t, "check")
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc(cue),
		emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")),
	)

	m.RunUntilTTY("Execute !", 30*emutest.Second)

	// Loading takes a while, and how long depends on the drive, so rather
	// than guess, run until the main loop reads the pad. Nothing else
	// calls padRaw.
	m.RunUntilFunc("main.padRaw", 60*emutest.Second)
	m.Hold(1, emu.ButtonSelect, 10)

	m.RunUntilTTY("End of Main", 10*emutest.Second)
}

// TestCheckScreenshot compares what check shows a second into its main loop
// with a golden image: the balls where a second of bouncing leaves them,
// and the text overlay. Run it with -update-golden to accept a change.
func TestCheckScreenshot(t *testing.T) {
	cue := disc(t, "check")
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc(cue),
		emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")),
	)
	m.RunUntilFunc("main.padRaw", 90*emutest.Second)
	m.RunFrames(60)
	emutest.Golden(t, m.Display(), "check-main-loop.png")
}

// TestCheckRunMatchesStep runs check twice, once a step at a time and once
// with Run, which lets the CPU run steps itself between the ones the devices
// need, and checks the two machines end up the same and made the same sound.
// check keeps every device busy, which the BIOS alone does not.
func TestCheckRunMatchesStep(t *testing.T) {
	cue := disc(t, "check")
	bios, err := os.ReadFile(emutest.BIOSFromEnv(t))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	boot := func() *emu.Machine {
		m, err := emu.New(bios)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		d, err := emu.OpenDisc(cue)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		t.Cleanup(func() { d.Close() })
		m.InsertDisc(d)
		m.SetAudio(true)
		return m
	}

	// Into the main loop, a frame's worth at a time, draining the sound as
	// a frontend would.
	const frames, budget = 250, 1_000_000
	var steppedAudio, runAudio []int16
	stepped := boot()
	for range frames {
		for range budget {
			if !assert.NoError(t, stepped.Step()) {
				t.FailNow()
			}
		}
		steppedAudio = stepped.Audio(steppedAudio)
	}
	run := boot()
	for range frames {
		for left := uint64(budget); left > 0; {
			ran, err := run.Run(left)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			left -= ran
		}
		runAudio = run.Audio(runAudio)
	}

	a, b := stepped.CPU, run.CPU
	assert.Equal(t, a.R, b.R, "registers")
	assert.Equal(t, a.COP0, b.COP0, "COP0")
	assert.Equal(t, [2]uint32{a.PC, a.NextPC}, [2]uint32{b.PC, b.NextPC}, "PC")
	assert.Equal(t, a.TotalCycles, b.TotalCycles, "cycles")
	assert.Equal(t, stepped.Frames(), run.Frames(), "frames")
	assert.Equal(t, true, slices.Equal(stepped.VRAM(), run.VRAM()), "VRAM")
	assert.Equal(t, len(steppedAudio), len(runAudio), "samples")
	assert.Equal(t, true, slices.Equal(steppedAudio, runAudio), "sound")
}
