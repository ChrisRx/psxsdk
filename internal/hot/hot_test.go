package hot_test

import (
	"os"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/internal/hot"
)

// The fixtures task build:hot writes: v1 built with --hot onto a disc,
// its ELF, and the patch for v2 built against it.
const (
	disc  = "../../.build/hot/v1.cue"
	base  = "../../.build/hot/prog.elf"
	patch = "../../.build/hot/v2.patch.elf"
)

// TestPatch runs v1, which prints a line on teal and counts frames, and
// applies v2's patch between frames: speed, which the program never
// writes, takes its new initial value, the background turns orange, from
// the new loop body calling the new function, and the count goes on from
// where it was, since the patch keeps a variable the program has written.
func TestPatch(t *testing.T) {
	data, err := os.ReadFile(patch)
	if err != nil {
		t.Skipf("no patch to apply (run task build:hot): %v", err)
	}
	p, err := hot.Parse(data)
	assert.NoError(t, err)
	assert.Equal(t, true, p.Safe != 0, "the patch should know where VSync is")
	assert.Equal(t, true, len(p.Slots) >= 2, "the patch should point main.main and its loop body's slots at new code")

	baseData, err := os.ReadFile(base)
	assert.NoError(t, err)
	start, end, err := hot.Region(baseData)
	assert.NoError(t, err)
	assert.Equal(t, true, p.Start >= start && p.End <= end, "the patch should lie in the program's region")

	assert.Equal(t, true, len(p.Inits) >= 1, "the patch should carry speed's new initial value")

	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.Symbols(base))
	// Until the BIOS has loaded the program its variables' addresses hold
	// whatever the BIOS keeps there, which under Sony's is not zero, so
	// wait for the program to start before watching its count.
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	m.RunUntilCount("main.frames", 30, 20*emutest.Second)
	assert.Equal(t, uint32(1), m.Read32(m.Sym("main.speed")), "v1's speed")
	before := m.Display()
	bg := before.RGBAAt(300, 200)
	assert.Equal(t, true, bg.R < 100 && bg.B > 100, "v1's background is teal, not %v", bg)

	applied := false
	p.ApplyAt(m.Machine, func() { applied = true })
	m.RunFrames(2)
	assert.Equal(t, true, applied, "the patch should have been applied at VSync")
	count := m.Read32(m.Sym("main.frames"))
	assert.Equal(t, true, count >= 30, "the count should go on from v1's, not start again: %d", count)
	assert.Equal(t, uint32(5), m.Read32(m.Sym("main.speed")), "speed, never written, should take v2's initial value")

	m.RunFrames(2)
	after := m.Display()
	bg = after.RGBAAt(300, 200)
	assert.Equal(t, true, bg.R > 150 && bg.B < 50, "v2's background is orange, not %v", bg)
	assert.Equal(t, true, m.Read32(m.Sym("main.frames")) > count, "v2 should keep counting")
	m.RunFrames(60)
}
