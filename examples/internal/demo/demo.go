// Package demo holds what the examples ported from Sony's Net Yaroze demos
// share: loading their data files off the disc, and the TMD layouts from
// their tmd.h. None of it is part of the resident library, which is why it
// is not in package yaroze.
//
// Like the examples, nothing here allocates after startup.
package demo

import (
	"go.chrisrx.dev/psx/disc"
)

// readTimeout is how many frames one CD read may take before it is given up
// on, so a missing or unreadable disc shows up as a message rather than a
// hang.
const readTimeout = 300

// Load reads files off the disc into the memory each was registered with,
// calling progress, if it is not nil, before each. It returns the name of
// the first that could not be found or read, or "" if they all arrived.
//
// On a Net Yaroze the files were sent from the PC by the loader's batch
// script, to addresses the script chose; here each is on the disc, put
// there by mkdisc from the declaration that registers it, and read into
// the program's own memory:
//
//	var wall0Data [(112 + 2047) / 2048 * 512]uint32
//	var wall0 = disc.Register("ASSETS/WALL0.TMD", "assets/WALL0.TMD").Into(wall0Data[:])
func Load(files []*disc.File, progress func(name string)) string {
	for _, f := range files {
		if progress != nil {
			progress(f.Name)
		}
		if !f.Load() || !disc.Wait(readTimeout) {
			return f.Name
		}
	}
	return ""
}
