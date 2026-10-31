// The program the serial tests upload to print to the host: println goes
// through the resident library's putchar, which the monitor sends down the
// cable. It prints a greeting, then a count each second, and frames counts
// the frames for a test to wait on.
package main

import "go.chrisrx.dev/psx/yaroze"

var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}

var frames int

func main() {
	println("hello from the console")
	screen.Open()
	for range screen.Frames() {
		frames++
		if frames%60 == 0 {
			println("second", frames/60)
		}
	}
}
