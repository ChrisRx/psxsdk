// An empty program to start from: the screen, cleared every frame, and a
// line of text, which is all a program needs to show that it runs.
//
// The playground's New project offers it beside jumpcube and quest, for a
// program written from nothing, and it is the program the guide's Getting
// started shows.
package main

import "go.chrisrx.dev/psx/yaroze"

// The screen, 320 by 240, cleared to teal every frame.
var screen = yaroze.Screen{Background: yaroze.Color{R: 60, G: 120, B: 120}}

func main() {
	screen.Open()
	screen.Font(16, 16, 256, 64, 512)
	for range screen.Frames() {
		yaroze.Print("hello from go")
	}
}
