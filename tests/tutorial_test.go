package tests

import (
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
)

// TestHelloScreenshot runs the tutorial's first program, which
// docs/tutorial/setting-up.md shows and task disc:tutorial builds, so that
// the page cannot drift from something that works: the line of text on the
// teal screen. The rest of the tutorial writes the square example, which
// TestSquareScreenshot checks.
func TestHelloScreenshot(t *testing.T) {
	m := bootExample(t, "hello")
	m.RunUntilCount("main.screen", 30, frameBudget)
	emutest.Golden(t, m.Display(), "hello.png")
}
