package libps_test

import (
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// The sprite program (exp/libps/sprites) built against the resident
// library (task build:disc NAME=sprites DIR=exp/libps/sprites) and
// against the Go library (task build:disc:go, the same NAME and DIR) has
// to show the same picture: the golden is recorded from the resident
// build with -update-golden, and the Go build is held to it.

func spritesScreenshot(t *testing.T, name string) {
	m := boot(t, name)
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	// The resident build reaches its 30th frame within a second of the
	// program starting; a build that takes longer has stalled.
	m.RunUntilCount("main.frameNumber", 30, 2*emutest.Second)
	m.Hold(1, emu.ButtonRight, 20)
	m.RunFrames(40)
	m.Golden("sprites.png")
	noComplaints(t, m)
}

func TestSpritesScreenshot(t *testing.T)   { spritesScreenshot(t, "sprites") }
func TestSpritesScreenshotGo(t *testing.T) { spritesScreenshot(t, "sprites-go") }
