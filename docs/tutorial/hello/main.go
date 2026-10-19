// Hello: a line of text on the screen.
package main

import "go.chrisrx.dev/psx/yaroze"

// The screen, 320 by 240, cleared to teal every frame.
var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	for range screen.Frames() {
		yaroze.Print("hello from go")
	}
}
