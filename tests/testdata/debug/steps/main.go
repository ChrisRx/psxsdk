// The program the debugger tests run under GDB: a frame loop that counts,
// with functions that are not inlined to break in and step through, a
// branch to step over, globals to print, and a line printed each second.
package main

import "go.chrisrx.dev/psx/yaroze"

var screen = yaroze.Screen{Background: yaroze.Color{R: 60, G: 60, B: 120}}

var (
	frames int
	total  int
	odd    int
)

//go:noinline
func add(a, b int) int {
	return a + b
}

//go:noinline
func tick(n int) int {
	total = add(total, n)
	if n&1 != 0 {
		odd++
	}
	return total
}

func main() {
	println("debug: start")
	screen.Open()
	for range screen.Frames() {
		frames++
		tick(frames)
		if frames%60 == 0 {
			println("debug: second", frames/60)
		}
	}
}
