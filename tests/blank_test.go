package tests

import (
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
)

// TestBlankScreenshot runs examples/blank, the program the playground's
// New project starts from nothing with and the guide's Getting started
// shows, so that neither can drift from something that works: the line of
// text on the teal screen.
func TestBlankScreenshot(t *testing.T) {
	parallel(t)
	m := bootExample(t, "blank")
	m.RunUntilCount("main.screen", 30, frameBudget)
	m.Golden("blank.png")
}

// TestBlankScreenshotResident is TestBlankScreenshot with the program built
// for the resident library, yaroze --libps resident (task build:disc:resident
// NAME=blank), which has to show the same picture.
func TestBlankScreenshotResident(t *testing.T) {
	m := bootExample(t, "blank-resident")
	m.RunUntilCount("main.screen", 30, 2*emutest.Second)
	m.Golden("blank.png")
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
