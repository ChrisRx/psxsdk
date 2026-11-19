package project

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/psx"
)

// files writes a project's files into dir: a name ending in .exe, or named
// MAIN, is a small PSX-EXE linked above the library, anything else holds
// its own name.
func files(t *testing.T, dir string, names map[string]string) {
	t.Helper()
	for name, content := range names {
		path := filepath.Join(dir, filepath.FromSlash(name))
		assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		if content == "exe" {
			exe := psx.New(0x80100000, 0x80100000, make([]byte, 2048))
			exe.StackAddr = 0x801fff00
			assert.NoError(t, exe.WriteFile(path))
			continue
		}
		assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func TestOpenDir(t *testing.T) {
	dir := t.TempDir()
	files(t, dir, map[string]string{
		"MAIN":     "exe",
		"DATA.TIM": "data",
		"auto":     "local dload data.tim 80090000\nlocal load main\ngo\n",
	})
	p, err := Open(dir, Options{})
	assert.NoError(t, err)
	defer p.Close()
	assert.Equal(t, filepath.Join(dir, "MAIN"), p.Program)
	assert.Equal(t, 1, len(p.Loads))
	assert.Equal(t, dir, p.Dir)
	exe, err := p.Executable()
	assert.NoError(t, err)
	text := exe.Section("text")
	assert.Equal(t, []byte("data"), text.Data[0x80090000-text.Addr:][:4], "the load is in the image")
}

func TestOpenZip(t *testing.T) {
	src := t.TempDir()
	files(t, src, map[string]string{
		"game/MAIN":     "exe",
		"game/DATA.TIM": "data",
		"game/AUTO":     "local dload data.tim 80090000\nlocal load main\ngo\n",
	})
	path := filepath.Join(t.TempDir(), "game.zip")
	writeZip(t, path, src)
	p, err := Open(path, Options{})
	assert.NoError(t, err)
	assert.Equal(t, "MAIN", p.Name())
	assert.Equal(t, "game", filepath.Base(p.Dir))
	tmp := p.tmp
	assert.NoError(t, p.Close())
	_, err = os.Stat(tmp)
	assert.Equal(t, true, os.IsNotExist(err), "Close removes what was unpacked")

	hasCue, err := ZipHasCue(path)
	assert.NoError(t, err)
	assert.Equal(t, false, hasCue)
}

func TestOpenChoices(t *testing.T) {
	dir := t.TempDir()
	files(t, dir, map[string]string{
		"a/MAIN": "exe",
		"a/auto": "local load main\ngo\n",
		"b/MAIN": "exe",
		"b/AUTO": "local load main\ngo\n",
	})
	_, err := Open(dir, Options{})
	assert.Equal(t, true, err != nil && strings.Contains(err.Error(), "a/auto, b/AUTO"), err)
	p, err := Open(dir, Options{Auto: "B/auto"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "b", "MAIN"), p.Program)

	// No auto file: the lone executable.
	lone := t.TempDir()
	files(t, lone, map[string]string{"x/GAME.EXE": "exe", "x/README": "text"})
	p, err = Open(lone, Options{})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(lone, "x", "GAME.EXE"), p.Program)
	assert.Equal(t, "", p.Auto)

	files(t, lone, map[string]string{"y/OTHER.EXE": "exe"})
	_, err = Open(lone, Options{})
	assert.Equal(t, true, err != nil && strings.Contains(err.Error(), "x/GAME.EXE, y/OTHER.EXE"), err)
}

// An executable given alone takes its files from an auto file beside it
// that loads it.
func TestOpenProgram(t *testing.T) {
	dir := t.TempDir()
	files(t, dir, map[string]string{
		"MAIN":     "exe",
		"OTHER":    "exe",
		"DATA.TIM": "data",
		"auto":     "local dload data.tim 80090000\nlocal load main\ngo\n",
	})
	p, err := Open(filepath.Join(dir, "MAIN"), Options{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(p.Loads))
	p, err = Open(filepath.Join(dir, "OTHER"), Options{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(p.Loads), "the auto file loads another program")
}

func writeZip(t *testing.T, path, dir string) {
	t.Helper()
	out, err := os.Create(path)
	assert.NoError(t, err)
	w := zip.NewWriter(out)
	assert.NoError(t, filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		f, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}))
	assert.NoError(t, w.Close())
	assert.NoError(t, out.Close())
}

// SIOCONS ran auto, and an auto.bat beside it was for the earlier tools.
func TestOpenAutoBat(t *testing.T) {
	dir := t.TempDir()
	files(t, dir, map[string]string{
		"MAIN":     "exe",
		"TEST":     "exe",
		"AUTO":     "local load main\ngo\n",
		"AUTO.BAT": "load -ntest\ngo\n",
	})
	p, err := Open(dir, Options{})
	assert.NoError(t, err)
	assert.Equal(t, "MAIN", p.Name())
	p, err = Open(dir, Options{Auto: "auto.bat"})
	assert.NoError(t, err)
	assert.Equal(t, "TEST", p.Name())
	p, err = Open(dir, Options{Program: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "TEST", p.Name())
}
