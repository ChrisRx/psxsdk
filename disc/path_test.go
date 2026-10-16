package disc

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestCDPath(t *testing.T) {
	for name, want := range map[string]string{
		"DATA/WALL0.TMD":     `\DATA\WALL0.TMD;1`,
		"data/sound/std0.vh": `\DATA\SOUND\STD0.VH;1`,
		"/MOVIE.STR":         `\MOVIE.STR;1`,
		`\DATA\TEN.TMD;1`:    `\DATA\TEN.TMD;1`,
		"SYSTEM.CNF":         `\SYSTEM.CNF;1`,
		"":                   `\;1`,
		"DATA/STONE.TIM;1":   `\DATA\STONE.TIM;1`,
		"a/very/deep/x.y":    `\A\VERY\DEEP\X.Y;1`,
	} {
		var buf [64]byte
		n := cdPath(buf[:], name)
		assert.Equal(t, want, string(buf[:n]), "%q", name)
		assert.Equal(t, byte(0), buf[n], "%q", name)
	}

	// Too small a buffer cuts the path short and still terminates it.
	var small [8]byte
	n := cdPath(small[:], "DATA/WALL0.TMD")
	assert.Equal(t, 7, n)
	assert.Equal(t, `\DATA\W`, string(small[:n]))
	assert.Equal(t, byte(0), small[7])
}
