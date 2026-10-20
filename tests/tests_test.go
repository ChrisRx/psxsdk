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
	"testing"

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
