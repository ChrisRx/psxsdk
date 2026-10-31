//go:build yaroze

// Package libps is the Net Yaroze library in Go: a reimplementation of the
// resident libps.exe that the boot disc left in RAM and that eco2exe
// combines into every program, linked into the program instead under the
// playstation-yaroze-go target (yaroze --libps go).
//
// Each function is exported under its C name, so the bindings in package
// yaroze, which declare the same names without bodies, resolve to these
// when the resident address table is left out of the link. The package
// imports nothing of yaroze: yaroze imports it, under the golibps tag.
//
// Every function and every variable that stands for the library's state
// says where it came from, as "libps 0xADDR, N bytes", the original's
// address in the image, so that a reader can go from the Go back to the
// disassembly (tools/libpsdis). Behavior is the original's, checked by
// the twin-call tests in this directory, which run each function and its
// original on the same inputs in the emulator; where the original and the
// manuals disagree, the Go follows the manuals and the test says so.
//
// This is an experiment; docs/architecture/libps-go.md describes it.
package libps
