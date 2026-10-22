//go:build !linux

package toolchain

import "os"

// newRestorer reads the snapshot into the module's memory. head is unused:
// the whole snapshot is copied after instantiation.
func newRestorer(f *os.File, size, head int64) restorer {
	return &copyRestorer{f: f, size: size}
}
