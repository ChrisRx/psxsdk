package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// sysrootTar is a sysroot of one file, for unpack.
func sysrootTar(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	assert.NoError(t, tw.WriteHeader(&tar.Header{Name: "sysroot/env.json", Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}))
	_, err := tw.Write([]byte(content))
	assert.NoError(t, err)
	assert.NoError(t, tw.Close())
	assert.NoError(t, zw.Close())
	return buf.Bytes()
}

// TestUnpackConcurrent has many processes' worth of unpacking of the same
// compiler run at once: every one succeeds, one copy is what is left, it
// is complete, and the cache holds nothing else of theirs.
func TestUnpackConcurrent(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "wasm-key")
	module := func() ([]byte, error) { return []byte("module"), nil }
	sysroot := func() ([]byte, error) { return sysrootTar(t, `{"target":"x"}`), nil }

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = unpack(cache, dir, module, sysroot)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		assert.NoError(t, err, "unpack %d", i)
	}
	assert.Equal(t, true, complete(dir))
	data, err := os.ReadFile(filepath.Join(dir, "sysroot", "env.json"))
	assert.NoError(t, err)
	assert.Equal(t, `{"target":"x"}`, string(data))
	entries, err := os.ReadDir(cache)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(entries), "the copies made too late are gone")
}

// TestUnpackKeepsWhatIsThere: a compiler already in place is left as it
// is, with whatever a process running from it has put there, and one
// that was cut short, with no mark, is replaced.
func TestUnpackKeepsWhatIsThere(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "wasm-key")
	module := func() ([]byte, error) { return []byte("module"), nil }
	sysroot := func() ([]byte, error) { return sysrootTar(t, "new"), nil }

	assert.NoError(t, os.MkdirAll(filepath.Join(dir, "tmp-running"), 0o777))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "tinygo.wasm"), []byte("old"), 0o666))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, completeMark), nil, 0o666))
	assert.NoError(t, unpack(cache, dir, module, sysroot))
	data, err := os.ReadFile(filepath.Join(dir, "tinygo.wasm"))
	assert.NoError(t, err)
	assert.Equal(t, "old", string(data), "a complete compiler is not replaced")
	_, err = os.Stat(filepath.Join(dir, "tmp-running"))
	assert.NoError(t, err, "what a process running from it made is kept")

	assert.NoError(t, os.Remove(filepath.Join(dir, completeMark)))
	assert.NoError(t, unpack(cache, dir, module, sysroot))
	data, err = os.ReadFile(filepath.Join(dir, "tinygo.wasm"))
	assert.NoError(t, err)
	assert.Equal(t, "module", string(data), "one cut short is replaced")
	assert.Equal(t, true, complete(dir))
}
