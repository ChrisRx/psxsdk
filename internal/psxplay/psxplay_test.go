package psxplay

import (
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
)

// TestFastStart runs the square example's disc through the fast start,
// which should end at the program's first picture, a few seconds of frames
// in, and then leave the machine to run in real time.
func TestFastStart(t *testing.T) {
	discs := os.Getenv("PSX_DISCS")
	if discs == "" {
		t.Skip("PSX_DISCS is not set")
	}
	m, err := emu.New(openbios.Image)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	d, err := emu.OpenDisc(filepath.Join(discs, "square", "square.cue"))
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
