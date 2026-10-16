package tests

import (
	"path/filepath"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// bootExample boots the BIOS with an example's disc in the drive and runs
// until the BIOS starts the program.
func bootExample(t *testing.T, name string) *emutest.Machine {
	t.Helper()
	cue := disc(t, name)
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc(cue),
		emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")),
	)
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	return m
}

// The examples' screenshots: each once the program's own frame counter
// reaches a count, which puts the test at the same point in the program's
// life whichever BIOS started it, and however long that BIOS took reading
// its data off the disc; a count of frames since the BIOS said it was
// starting the program came out a frame or so apart between Sony's and
// OpenBIOS. Run them with -update-golden to accept a change.

// frameBudget is the time a program is given to reach a count, with its
// loading included.
const frameBudget = 30 * emutest.Second

// TestMDECScreenshot checks a frame of the movie the MDEC decodes, shown in
// 24-bit colour.
func TestMDECScreenshot(t *testing.T) {
	m := bootExample(t, "mdec")
	m.RunUntilCount("main.shown", 86, frameBudget)
	emutest.Golden(t, m.Display(), "mdec.png")
}

// TestTODScreenshot checks the hand partway through its animation.
func TestTODScreenshot(t *testing.T) {
	m := bootExample(t, "tod")
	m.RunUntilCount("main.frameNumber", 270, frameBudget)
	emutest.Golden(t, m.Display(), "tod.png")
}

// TestOffscrScreenshot checks the sphere wearing the texture the sprite is
// drawn into every frame, past the point where a block of VRAM is copied
// over the square's texture. The program prints the count of horizontal
// syncs VSync returns, which is the BIOS's to make: Sony's says 51 a frame
// and OpenBIOS 52, so this golden is Sony's by that one line.
func TestOffscrScreenshot(t *testing.T) {
	m := bootExample(t, "offscr")
	m.RunUntilCount("main.frameNumber", 330, frameBudget)
	emutest.Golden(t, m.Display(), "offscr.png")
}

// TestTunnelScreenshot checks the tunnel after the flight in, with the
// score drawn in sprites.
func TestTunnelScreenshot(t *testing.T) {
	m := bootExample(t, "tunnel")
	m.RunUntilCount("main.frameNumber", 166, frameBudget)
	emutest.Golden(t, m.Display(), "tunnel.png")
}

// TestSolarCirclingView works the solar system's menus with the pad: into
// the menus with Start, back round to Select object, along its submenus to
// the views, and on to the second of them, circling the sun.
func TestSolarCirclingView(t *testing.T) {
	m := bootExample(t, "solar")
	m.RunUntilCount("main.frameNumber", 121, frameBudget)
	m.Hold(1, emu.ButtonStart, 10)
	m.Hold(1, emu.ButtonSquare, 10) // the main menu round to Select object
	m.Hold(1, emu.ButtonCross, 10)  // down to its submenu
	for range 4 {
		m.Hold(1, emu.ButtonRight, 10) // along to Change views
	}
	m.Hold(1, emu.ButtonCross, 10)
	m.Hold(1, emu.ButtonCross, 10) // down to the view
	m.Hold(1, emu.ButtonLeft, 10)  // the external view
	m.Hold(1, emu.ButtonLeft, 10)  // the circling view
	m.RunFrames(60)
	emutest.Golden(t, m.Display(), "solar-circling.png")
}
