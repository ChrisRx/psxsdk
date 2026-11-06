package tests

import (
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
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

// TestHelloScreenshotResident is TestHelloScreenshot with the program built
// for the resident library, yaroze --libps resident (task build:disc:resident NAME=hello
// DIR=docs/tutorial/hello), which has to show the same picture.
func TestHelloScreenshotResident(t *testing.T) {
	m := bootExample(t, "hello-resident")
	m.RunUntilCount("main.screen", 30, 2*emutest.Second)
	m.Golden("hello.png")
	noComplaints(t, m)
}

// noComplaints fails the test when the library printed one of its
// timeouts.
func noComplaints(t *testing.T, m *emutest.Machine) {
	t.Helper()
	for _, c := range []string{"VSync: timeout", "GPU timeout", "intr timeout", "unexpected interrupt"} {
		if strings.Contains(m.TTY(), c) {
			t.Errorf("the library printed %q", c)
		}
	}
}
