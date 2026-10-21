package psxplay

import (
	"cmp"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
)

// TestFastStart runs the square example's disc through the fast start,
// which should end at the program's first picture, a few seconds of frames
// in, and then leave the machine to run in real time. The disc is read from
// the directory PSX_DISCS names, or the repository's .build, and the test is
// skipped until task build:disc NAME=square has built it.
func TestFastStart(t *testing.T) {
	cue := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../../.build"), "square", "square.cue")
	if _, err := os.Stat(cue); err != nil {
		t.Skipf("no disc for square: %v", err)
	}
	m, err := emu.New(openbios.Image)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	d, err := emu.OpenDisc(cue)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer d.Close()
	m.InsertDisc(d)

	g := &game{machine: m, fast: true}
	ticks := 0
	for g.fast {
		g.fast, err = g.fastStart()
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		ticks++
	}
	t.Logf("%d frames in %d ticks", g.fastFrames, ticks)
	assert.Equal(t, true, m.Shown(), "the fast start ends at the first picture")
	assert.Equal(t, true, g.fastFrames > 120, "after the BIOS's two seconds or so")
}
