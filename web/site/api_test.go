package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestWriteAPI(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/yaroze\n")
	write(t, dir, "lib.go", `//go:build yaroze

package yaroze

// Video modes.
const (
	MODE_NTSC = 0
	MODE_PAL  = 1
)

// BOX is a rectangle.
type BOX struct {
	X, Y int16
	pad  int16
}

// Draw draws b.
func (b *BOX) Draw(pri uint16) {}

// NewBOX returns a box.
//
//export NewBOX
func NewBOX() *BOX

func hidden() {}
`)
	write(t, dir, "stub.go", "//go:build !yaroze\n\npackage yaroze\n\nfunc Stub() {}\n")

	out := filepath.Join(t.TempDir(), "play", "api.json")
	assert.NoError(t, writeAPI([]string{dir}, out))
	data, err := os.ReadFile(out)
	assert.NoError(t, err)
	var api map[string][]APIEntry
	assert.NoError(t, json.Unmarshal(data, &api))
	assert.Equal(t, 1, len(api))
	entries := api["example.com/yaroze"]

	byName := map[string]APIEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	assert.Equal(t, 4, len(entries), entries)
	assert.Equal(t, "const MODE_PAL = 1", byName["MODE_PAL"].Sig)
	assert.Equal(t, "Video modes.\n", byName["MODE_PAL"].Doc)
	assert.Equal(t, "type BOX struct", byName["BOX"].Sig)
	assert.Equal(t, []APIField{{"X", "int16"}, {"Y", "int16"}}, byName["BOX"].Fields)
	assert.Equal(t, 1, len(byName["BOX"].Methods))
	assert.Equal(t, "func (b *BOX) Draw(pri uint16)", byName["BOX"].Methods[0].Sig)
	assert.Equal(t, "func NewBOX() *BOX", byName["NewBOX"].Sig)
	assert.Equal(t, "NewBOX returns a box.\n", byName["NewBOX"].Doc)
}
