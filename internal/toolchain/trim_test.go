package toolchain

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

// TestTrimCache makes a cache of compilers used at various times and
// expects the trim to remove those not used for five days, other than the
// one in use, and to run once a day.
func TestTrimCache(t *testing.T) {
	cache := t.TempDir()
	now := time.Now()
	add := func(name string, used time.Time, marked bool) string {
		dir := filepath.Join(cache, name)
		assert.NoError(t, os.Mkdir(dir, 0o777))
		if marked {
			assert.NoError(t, os.WriteFile(filepath.Join(dir, usedMark), nil, 0o666))
			assert.NoError(t, os.Chtimes(filepath.Join(dir, usedMark), used, used))
		} else {
			assert.NoError(t, os.Chtimes(dir, used, used))
		}
		return dir
	}
	keep := add("wasm-current", now.Add(-10*24*time.Hour), true)
	old := add("wasm-old", now.Add(-6*24*time.Hour), true)
	unmarked := add("wasm-unmarked", now.Add(-6*24*time.Hour), false)
	leftover := add("wasm-tmp-abc", now.Add(-6*24*time.Hour), false)
	fresh := add("wasm-fresh", now.Add(-4*24*time.Hour), true)
	other := add("tinygo-overlay", now.Add(-30*24*time.Hour), false)

	trimCache(cache, keep, now)
	for _, dir := range []string{old, unmarked, leftover} {
		_, err := os.Stat(dir)
		assert.Equal(t, true, os.IsNotExist(err), "%s should have been removed", dir)
	}
	for _, dir := range []string{keep, fresh, other} {
		_, err := os.Stat(dir)
		assert.NoError(t, err, "%s should have been kept", dir)
	}

	// Not again the same day, even for one now old enough.
	late := add("wasm-late", now.Add(-6*24*time.Hour), true)
	trimCache(cache, keep, now.Add(time.Hour))
	_, err := os.Stat(late)
	assert.NoError(t, err)
	trimCache(cache, keep, now.Add(25*time.Hour))
	_, err = os.Stat(late)
	assert.Equal(t, true, os.IsNotExist(err))
}

// TestMarkUsed expects the mark to be made and then left alone within the
// hour.
func TestMarkUsed(t *testing.T) {
	dir := t.TempDir()
	markUsed(dir)
	mark := filepath.Join(dir, usedMark)
	fi, err := os.Stat(mark)
	assert.NoError(t, err)
	then := time.Now().Add(-30 * time.Minute)
	assert.NoError(t, os.Chtimes(mark, then, then))
	markUsed(dir)
	fi, err = os.Stat(mark)
	assert.NoError(t, err)
	assert.Equal(t, true, fi.ModTime().Before(time.Now().Add(-29*time.Minute)), "marked again within the hour")
	then = time.Now().Add(-2 * time.Hour)
	assert.NoError(t, os.Chtimes(mark, then, then))
	markUsed(dir)
	fi, err = os.Stat(mark)
	assert.NoError(t, err)
	assert.Equal(t, true, fi.ModTime().After(time.Now().Add(-time.Minute)), "not marked after an hour")
}
