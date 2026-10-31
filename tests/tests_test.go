// Package tests runs the example programs in the emulator.
//
// The programs are built with TinyGo onto disc images first, each into a
// directory of its own under .build, as task build:disc does, or under the
// directory PSX_DISCS names. They boot OpenBIOS, or the BIOS PSX_BIOS names.
// A test whose disc has not been built is skipped. task test:emu builds the
// discs and runs them.
package tests

import (
	"cmp"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
)

// parallel lets the test run beside the others, each on a machine of its
// own, unless CI is set, as CI services set it, where they run one at a
// time.
func parallel(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Parallel()
	}
}

// disc returns the cue sheet of the named example's disc, or skips the
// test if there is none.
func disc(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../.build"), name, name+".cue")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no disc for %s: %v", name, err)
	}
	return path
}

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
