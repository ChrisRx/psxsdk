package tests

import (
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

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
// 24-bit color.
func TestMDECScreenshot(t *testing.T) {
	m := bootExample(t, "mdec")
	m.RunUntilCount("main.shown", 86, frameBudget)
	emutest.Golden(t, m.Display(), "mdec.png")
}

// TestDungeonScreenshot checks the maze after a turn to the left and a walk
// up to a wall, with the compass turned to match. The program prints the
// count of horizontal syncs VSync returns, which is the BIOS's to make, so
// this golden is OpenBIOS's by that one line.
func TestDungeonScreenshot(t *testing.T) {
	m := bootExample(t, "dungeon")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonLeft, 40)
	m.Hold(1, emu.ButtonUp, 150)
	m.RunFrames(10)
	emutest.Golden(t, m.Display(), "dungeon.png")
}

// TestCarScreenshot checks the car fifty frames after its drive in from
// the distance, close to the viewpoint and turning.
func TestCarScreenshot(t *testing.T) {
	m := bootExample(t, "car")
	m.RunUntilCount("main.frameNumber", 520, frameBudget)
	emutest.Golden(t, m.Display(), "car.png")
}

// TestOdenScreenshot checks the three shapes under the three lights after
// the ball is swapped for its textured version and the scene is turned.
func TestOdenScreenshot(t *testing.T) {
	m := bootExample(t, "oden")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonLeft, 3)
	m.Hold(1, emu.ButtonSquare, 20)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "oden.png")
}

// TestMorphScreenshot checks the sheet most of the way to becoming a
// cylinder, with the picture wrapped round it.
func TestMorphScreenshot(t *testing.T) {
	m := bootExample(t, "morph")
	m.RunUntilCount("main.frameNumber", 400, frameBudget)
	emutest.Golden(t, m.Display(), "morph.png")
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

// TestFlameScreenshot checks the sheet of fire a second and a half in, with
// the hot spots along its bottom edge and the heat risen through the
// palette above them.
func TestFlameScreenshot(t *testing.T) {
	m := bootExample(t, "flame")
	m.RunUntilCount("main.frameNumber", 90, frameBudget)
	emutest.Golden(t, m.Display(), "flame.png")
}

// TestInvadersScreenshot checks the invaders on their march after the ship
// has slid to the left and fired a shot.
func TestInvadersScreenshot(t *testing.T) {
	m := bootExample(t, "invaders")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonLeft, 20)
	m.RunFrames(5)
	m.Hold(1, emu.ButtonTriangle, 5)
	m.RunFrames(40)
	emutest.Golden(t, m.Display(), "invaders.png")
}

// TestBreakoutScreenshot checks the wall of bricks with the ball on its
// way down and the bat moved to the right to meet it.
func TestBreakoutScreenshot(t *testing.T) {
	m := bootExample(t, "breakout")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonRight, 20)
	m.RunFrames(100)
	emutest.Golden(t, m.Display(), "breakout.png")
}

// TestBlowScreenshot checks the shuttle's first piece a second and a half
// into its explosion, with the menu on the explosion's page: R1 steps the
// menu back from the first page to the last, Circle moves to its second
// line and Triangle turns the explosion on.
func TestBlowScreenshot(t *testing.T) {
	m := bootExample(t, "blow")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonR1, 3)
	m.RunFrames(15)
	m.Hold(1, emu.ButtonCircle, 3)
	m.RunFrames(15)
	m.Hold(1, emu.ButtonTriangle, 3)
	m.RunFrames(90)
	emutest.Golden(t, m.Display(), "blow.png")
}

// TestDinoScreenshot checks the dinosaur in front of the castle two
// seconds into its walk.
func TestDinoScreenshot(t *testing.T) {
	m := bootExample(t, "dino")
	m.RunUntilCount("main.frameNumber", 120, frameBudget)
	emutest.Golden(t, m.Display(), "dino.png")
}

// TestDogTaleScreenshot checks the level a little way in: the title is
// left by choosing Play with Right and Cross, "get ready" is shown for 75
// frames, and then the man walks right, past the first bones, and jumps
// before he reaches the first cactus.
func TestDogTaleScreenshot(t *testing.T) {
	m := bootExample(t, "dogtale")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonRight, 3)
	m.RunFrames(3)
	m.Hold(1, emu.ButtonCross, 3)
	m.RunFrames(90)
	m.Hold(1, emu.ButtonRight, 25)
	m.Hold(1, emu.ButtonCross|emu.ButtonRight, 3)
	m.RunFrames(10)
	emutest.Golden(t, m.Display(), "dogtale.png")
}

// TestSquareScreenshot checks the box after it is moved up and to the right
// over the turning cube, with left and right held together for a while to
// show that they cancel, and that the music plays, stops while Start has
// paused it, and plays again after Start resumes it. The square is the
// program the playground starts from. Its frames are counted by the
// screen's own count, the first word of yaroze.Screen, and the cube turns
// by that count, so that the picture is the same however long the BIOS
// took to start the program and the disc to deliver its files.
func TestSquareScreenshot(t *testing.T) {
	m := bootExample(t, "square")
	m.SetAudio(true)
	m.RunUntilCount("main.screen", 150, frameBudget)

	// Playing is a peak in the thousands. Paused, what is left of the
	// notes' release after half a second is a few steps from silence.
	const quiet = 100
	assert.Equal(t, true, peak(m, 60) > quiet, "the music plays")
	m.Hold(1, emu.ButtonStart, 2)
	peak(m, 30) // the notes' release, which also empties what Audio keeps
	assert.Equal(t, true, peak(m, 30) < quiet, "the music is paused")
	m.Hold(1, emu.ButtonStart, 2)
	assert.Equal(t, true, peak(m, 60) > quiet, "the music plays again")

	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 60)
	m.Hold(1, emu.ButtonLeft|emu.ButtonRight, 20)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "square.png")
}

// peak runs the machine for frames frames and returns the loudest sample
// the SPU made meanwhile.
func peak(m *emutest.Machine, frames int) int {
	var p int
	var buf []int16
	for range frames {
		m.RunFrames(1)
		buf = m.Audio(buf[:0])
		for _, s := range buf {
			p = max(p, int(s), -int(s))
		}
	}
	return p
}
