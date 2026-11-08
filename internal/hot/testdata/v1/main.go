// The program the hot reload test starts from: a line of text on teal.
// v2 is the same program after an edit, which the test applies to this
// one while it runs.
package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}

var frames int

// speed is never written, so a patch whose initial value differs gives
// it the new one; frames is written every frame, so a patch leaves it.
var speed uint32 = 1

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	for range screen.Frames() {
		frames++
		if volatile.LoadUint32(&speed) > 3 {
			screen.Background = yaroze.CVECTOR{R: 200, G: 100, B: 0}
		}
		yaroze.Print("version one")
	}
}
