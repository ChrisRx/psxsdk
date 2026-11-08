// The program of v1 after an edit: the frame loop's body prints something
// else and a new function gives the colour, speed starts at 5, which turns
// the background orange, and frames keeps counting from where v1 left it,
// since a patch keeps a variable the program has written.
package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}

var frames int

var speed uint32 = 5

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	for range screen.Frames() {
		frames++
		if volatile.LoadUint32(&speed) > 3 {
			screen.Background = orange()
		}
		yaroze.Print("version two")
	}
}

func orange() yaroze.CVECTOR { return yaroze.CVECTOR{R: 200, G: 100, B: 0} }
