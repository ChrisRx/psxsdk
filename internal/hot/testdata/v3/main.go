// The second hot reload test's program: its count kept in a struct made in
// main, which the compiler makes a global, rather than in a package-level
// variable. v4 is the same program after an edit.
package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}

type state struct {
	count uint32
	step  uint32
}

// frames mirrors the count, for the test to read by its symbol.
var frames uint32

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	s := &state{step: 1}
	for range screen.Frames() {
		s.count += s.step
		volatile.StoreUint32(&frames, s.count)
		yaroze.Print("version three")
	}
}
