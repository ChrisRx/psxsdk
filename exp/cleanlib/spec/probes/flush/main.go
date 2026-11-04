//go:build yaroze

// Command flush is a probe for the spec: two print windows, both opened
// with isbg 1, text printed to the first, each flushed by its own id, to
// see whether an empty stream draws its background and where FntPrint's
// text goes. Built for the resident library it shows Sony's behavior.
package main

import "go.chrisrx.dev/psx/yaroze"

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}
	text   [16]byte
)

func main() {
	screen.Open()
	yaroze.FntLoad(960, 256)
	a := yaroze.FntOpen(16, 32, 128, 64, 1, 256)
	b := yaroze.FntOpen(176, 32, 128, 64, 1, 256)
	for {
		screen.Begin()
		yaroze.FntPrint(yaroze.CString(text[:], "TEXT"))
		screen.End()
		yaroze.FntFlush(a)
		yaroze.FntFlush(b)
	}
}
