package tests

import (
	"testing"
)

// TestHelloScreenshot runs the tutorial's first program, which
// docs/tutorial/setting-up.md shows and task build:disc:tutorial builds, so
// that the page cannot drift from something that works: the line of text on
// the teal screen. The rest of the tutorial writes the square example, which
// TestSquareScreenshot checks.
func TestHelloScreenshot(t *testing.T) {
	parallel(t)
	m := bootExample(t, "hello")
	m.RunUntilCount("main.screen", 30, frameBudget)
	m.Golden("hello.png")
}
