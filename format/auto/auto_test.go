package auto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestParseSiocons(t *testing.T) {
	b, err := Parse(strings.NewReader("local dload data\\Sprite.TIM 80090000\r\n" +
		"local dload a.seq 0x801079c9 b.vh 801f71e0\r\n" +
		"local echo loading\r\n" +
		"local load -s main\r\n" +
		"local\r\n" +
		"go\r\n"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"main"}, b.Programs)
	assert.Equal(t, []Load{
		{File: `data\Sprite.TIM`, Addr: 0x80090000},
		{File: "a.seq", Addr: 0x801079c9},
		{File: "b.vh", Addr: 0x801f71e0},
	}, b.Loads)
	assert.Equal(t, true, b.Go)
	assert.Equal(t, []string{"local echo loading"}, b.Skipped)
}

// The earlier tools' batch files, which some samples kept.
func TestParseDOS(t *testing.T) {
	b, err := Parse(strings.NewReader("@echo off\r\ndload -nrsd\\wall0.tmd -a80090000\r\nload -ntest\r\ngo\r\n"))
	assert.NoError(t, err)
	assert.Equal(t, "test", b.Program())
	assert.Equal(t, []Load{{File: `rsd\wall0.tmd`, Addr: 0x80090000}}, b.Loads)
	assert.Equal(t, []string{"@echo off"}, b.Skipped)
}

func TestParseErrors(t *testing.T) {
	for _, text := range []string{"local dload a.tim\n", "local load\n", "dload -na.tim -azz\n"} {
		_, err := Parse(strings.NewReader(text))
		assert.Equal(t, true, err != nil, text)
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(dir, "Data"), 0o755))
	for _, name := range []string{"MAIN", "Data/SPRITE.TIM"} {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte{1}, 0o644))
	}
	path := filepath.Join(dir, "AUTO")
	assert.NoError(t, os.WriteFile(path, []byte("local dload data\\sprite.tim 80090000\nlocal dload gone.tim 800a0000\nlocal load main\ngo\n"), 0o644))
	b, err := Read(path)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "MAIN"), b.Program())
	assert.Equal(t, []Load{{File: filepath.Join(dir, "Data", "SPRITE.TIM"), Addr: 0x80090000}}, b.Loads)
	assert.Equal(t, []string{"gone.tim"}, b.Missing)
	assert.Equal(t, dir, b.Dir)
}
