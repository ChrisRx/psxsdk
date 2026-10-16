package tests

import (
	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"testing"
)

// TestCubeScreenshot runs the tutorial's program, which docs/tutorial shows
// and task disc:tutorial builds, so that the page cannot drift from
// something that works: the cube after a turn to the left and a tip
// downwards, lit from over the camera's shoulder.
func TestCubeScreenshot(t *testing.T) {
	m := bootExample(t, "cube")
	// The texture is the second file read, so its ID word arriving in the
	// buffer means loading is done and the frames have begun.
	texture := m.Sym("main.textureData")
	m.RunUntil(func() bool { return m.Read32(texture) == 0x10 }, frameBudget, "the texture to be read")
	m.RunFrames(60)
	m.Hold(1, emu.ButtonLeft, 30)
	m.Hold(1, emu.ButtonDown, 20)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "cube.png")
}
