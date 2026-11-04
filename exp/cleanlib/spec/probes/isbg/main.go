//go:build yaroze

// Command isbg is a probe for the spec: two print windows over a teal
// screen, the left opened with isbg 0 and the right with isbg 1, to see
// which one the library clears to black. Text goes to the right one, the
// last opened; FntFlush(-1) draws both. Built for the resident library it
// shows Sony's behavior.
package main

import "go.chrisrx.dev/psx/yaroze"

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}
	text   [16]byte
)

func main() {
	screen.Open()
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 32, 128, 64, 0, 256)
	yaroze.FntOpen(176, 32, 128, 64, 1, 256)
	for {
		screen.Begin()
		yaroze.FntPrint(yaroze.CString(text[:], "RIGHT"))
		screen.End()
		yaroze.FntFlush(-1)
	}
}
