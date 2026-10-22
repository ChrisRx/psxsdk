package toolchain

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The cache directory gets a compiler per yaroze build and per rebuild of what
// YAROZE_WASM names: the module unpacked, its sysroot, and wazero's compiled
// code, about 270MB each. Nothing but the one this yaroze runs is needed, so
// they are trimmed the way the go command trims its build cache: a compiler
// is marked used at most once an hour, and once a day the ones not used for
// five days are removed.
const (
	usedMark     = ".used"
	trimMark     = "trim.txt"
	usedInterval = time.Hour
	trimInterval = 24 * time.Hour
	trimLimit    = 5 * 24 * time.Hour
)

// touch sets path's modification time to now, creating it if need be,
// unless it was set within interval, so that a file used often is written
// rarely.
func touch(path string, interval time.Duration) {
	if fi, err := os.Stat(path); err == nil && time.Since(fi.ModTime()) < interval {
		return
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666); err == nil {
		f.Close()
	}
	now := time.Now()
	os.Chtimes(path, now, now)
}

// markUsed records that the compiler in dir was used.
func markUsed(dir string) { touch(filepath.Join(dir, usedMark), usedInterval) }

// trimCache removes the compilers in cache not used for trimLimit, other
// than keep, the one in use. It does so at most once per trimInterval,
// recording when in trimMark.
func trimCache(cache, keep string, now time.Time) {
	mark := filepath.Join(cache, trimMark)
	if data, err := os.ReadFile(mark); err == nil {
		if t, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil && now.Sub(time.Unix(t, 0)) < trimInterval {
			return
		}
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		return
	}
	for _, e := range entries {
		dir := filepath.Join(cache, e.Name())
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "wasm-") || dir == keep {
			continue
		}
		if now.Sub(lastUsed(dir)) > trimLimit {
			os.RemoveAll(dir)
		}
	}
	os.WriteFile(mark, []byte(strconv.FormatInt(now.Unix(), 10)+"\n"), 0o666)
}

// lastUsed is when the compiler in dir was last marked used, or the
// directory's own modification time for one from before the marks.
func lastUsed(dir string) time.Time {
	if fi, err := os.Stat(filepath.Join(dir, usedMark)); err == nil {
		return fi.ModTime()
	}
	if fi, err := os.Stat(dir); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}
