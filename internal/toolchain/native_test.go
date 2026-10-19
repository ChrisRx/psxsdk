package toolchain

import (
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestOverlay lays the fork's files over a small TinyGo root: the fork's
// files are real, directories on the way to them and everything under src
// are real directories, and everything else is a link into the root.
func TestOverlay(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"src/runtime/runtime.go",
		"src/runtime/interrupt/interrupt.go",
		"src/unicode/utf8/utf8.go",
		"src/device/arm/arm.go",
		"targets/wasip1.json",
		"lib/picolibc/README",
	} {
		p := filepath.Join(root, f)
		assert.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		assert.NoError(t, os.WriteFile(p, []byte(f), 0o644))
	}
	dst := t.TempDir()
	assert.NoError(t, overlay(root, dst))

	kind := func(rel string) string {
		info, err := os.Lstat(filepath.Join(dst, rel))
		if err != nil {
			return "missing"
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return "link"
		case info.IsDir():
			return "dir"
		}
		return "file"
	}
	for rel, want := range map[string]string{
		"targets":                            "dir",
		"targets/wasip1.json":                "link",
		"targets/playstation-yaroze.json":    "file",
		"targets/playstation-yaroze-gte.S":   "file",
		"src":                                "dir",
		"src/runtime":                        "dir",
		"src/runtime/runtime.go":             "link",
		"src/runtime/runtime_yaroze.go":      "file",
		"src/runtime/interrupt":              "dir",
		"src/runtime/interrupt/interrupt.go": "link",
		"src/unicode":                        "dir",
		"src/unicode/utf8":                   "dir",
		"src/unicode/utf8/utf8.go":           "link",
		"src/device/arm/arm.go":              "link",
		"lib":                                "link",
	} {
		assert.Equal(t, want, kind(rel), rel)
	}

	data, err := os.ReadFile(filepath.Join(dst, "src/unicode/utf8/utf8.go"))
	assert.Equal(t, "src/unicode/utf8/utf8.go", string(assert.Ok(t, data, err)))
	data, err = os.ReadFile(filepath.Join(dst, "targets/playstation-yaroze.json"))
	want, _ := fork.ReadFile("_tinygo/targets/playstation-yaroze.json")
	assert.Equal(t, string(want), string(assert.Ok(t, data, err)))
}
