// v3 after an edit: a new allocation in main before the old one, kept by a
// package variable rather than captured, since a patch cannot change what
// the running loop body captures, and the body counts by two and turns the
// background orange.
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

type extra struct{ hits [8]uint32 }

var frames uint32

var kept *extra

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	kept = &extra{}
	s := &state{step: 1}
	for range screen.Frames() {
		s.count += 2
		volatile.StoreUint32(&frames, s.count)
		screen.Background = yaroze.CVECTOR{R: 200, G: 100, B: 0}
		yaroze.Print("version four")
	}
}
