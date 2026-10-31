package tests

import (
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// TestEnemiesScreenshot runs the program the cookbook's Enemies page shows
// and plays, which task build:disc:cookbook builds: the enemy closes on
// the player while circle is held and draws away when it is let go, and in
// the random states it changes state, which the picture shows.
func TestEnemiesScreenshot(t *testing.T) {
	parallel(t)
	m := bootExample(t, "enemies")
	m.RunUntilCount("main.screen", 30, frameBudget)
	dist := m.Sym("main.distance")
	far := int32(m.Read32(dist))
	m.Pad(1).Press(emu.ButtonCircle)
	m.RunFrames(60)
	near := int32(m.Read32(dist))
	m.Pad(1).Release(emu.ButtonCircle)
	m.RunFrames(60)
	away := int32(m.Read32(dist))
	t.Logf("distance %d, %d attacking, %d evading", far, near, away)
	assert.Equal(t, true, near < far-50, "attack closes in")
	assert.Equal(t, true, away > near+20, "evade draws away")

	for range 5 {
		m.Hold(1, emu.ButtonR1, 6)
		m.RunFrames(4)
	}
	assert.Equal(t, uint32(5), m.Read32(m.Sym("main.mode")), "the random states")
	st := m.Sym("main.state")
	first := m.Read32(st)
	m.RunUntil(func() bool { return m.Read32(st) != first }, 60*emutest.Second, "a change of state")
	m.RunFrames(40)
	m.Golden("enemies.png")
}
