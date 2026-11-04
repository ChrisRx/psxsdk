//go:build yaroze

// Command isbg1 is a probe for the spec: one print window, opened with
// isbg 1, over a teal screen, to see whether the library draws a
// background behind it. Built for the resident library it shows Sony's
// behavior.
package main

import "go.chrisrx.dev/psx/yaroze"

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}
	text   [16]byte
)

func main() {
	screen.Open()
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(176, 32, 128, 64, 1, 256)
	for {
		screen.Begin()
		yaroze.FntPrint(yaroze.CString(text[:], "RIGHT"))
		screen.End()
		yaroze.FntFlush(-1)
	}
}
