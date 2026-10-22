//go:build unix

package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestSnapshotPath expects the snapshot directory to be refused unless it
// is a directory of this user's alone.
func TestSnapshotPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	w := &Wasm{dir: filepath.Join(tmp, "wasm-abc")}
	dir := filepath.Join(tmp, "nyc"+userSuffix())

	t.Run("made", func(t *testing.T) {
		path, err := snapshotPath(w)
		assert.NoError(t, err)
		assert.Equal(t, true, strings.HasPrefix(path, dir+string(filepath.Separator)), path)
		assert.Equal(t, true, strings.HasPrefix(filepath.Base(path), "wasm-abc-"), path)
		fi, err := os.Stat(dir)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
	})

	t.Run("open to others", func(t *testing.T) {
		assert.NoError(t, os.Chmod(dir, 0o755))
		_, err := snapshotPath(w)
		assert.Error(t, "is not a directory of this user's alone", err)
		assert.NoError(t, os.Chmod(dir, 0o700))
		_, err = snapshotPath(w)
		assert.NoError(t, err)
	})

	t.Run("a link", func(t *testing.T) {
		assert.NoError(t, os.Remove(dir))
		assert.NoError(t, os.Symlink(t.TempDir(), dir))
		_, err := snapshotPath(w)
		assert.Error(t, "is not a directory of this user's alone", err)
	})
}

// TestOpenSnapshot expects a snapshot to be refused unless it is a regular
// file of this user's alone, of a memory's size.
func TestOpenSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.mem")
	_, _, err := openSnapshot(path)
	assert.Equal(t, true, os.IsNotExist(err))

	assert.NoError(t, os.WriteFile(path, make([]byte, wasmPage), 0o644))
	_, _, err = openSnapshot(path)
	assert.Error(t, "not a regular file of this user's alone", err)

	assert.NoError(t, os.Chmod(path, 0o600))
	f, size, err := openSnapshot(path)
	assert.NoError(t, err)
	assert.Equal(t, int64(wasmPage), size)
	f.Close()

	assert.NoError(t, os.WriteFile(path, make([]byte, 100), 0o600))
	_, _, err = openSnapshot(path)
	assert.Error(t, "100 bytes is not a memory's size", err)
}
