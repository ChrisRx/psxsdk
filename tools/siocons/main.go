// Command siocons talks to a Net Yaroze over the serial cable, as Sony's
// SIOCONS did: a terminal on the console's monitor, with SIOCONS's local
// commands on the function keys and in batch files. It is also yaroze's
// sio command.
package main

import (
	"log"

	"go.chrisrx.dev/x/context"

	"go.chrisrx.dev/psx/internal/cmd/siocons"
)

func main() {
	ctx := context.Shutdown()
	if err := siocons.Command("siocons").ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
