// Command psxplay runs the emulator in a window, with the keyboard or a
// gamepad as the pad in port 1 and the SPU's sound on the speakers. It is
// the desktop counterpart of the player on the site, which runs the same
// emulator in a browser, and yaroze run runs a disc in it too.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/psxplay"
)

func main() {
	if err := psxplay.Command("psxplay").Execute(); err != nil {
		log.Fatal(err)
	}
}
