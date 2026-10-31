// Command sioload uploads an executable to a Net Yaroze over the serial
// cable, as SIOCONS's load did, a PS-X EXE, ECOFF or ELF, and with --exec
// runs it.
package main

import (
	"log"

	"go.chrisrx.dev/x/context"

	"go.chrisrx.dev/psx/internal/cmd/sioload"
)

func main() {
	ctx := context.Shutdown()
	if err := sioload.Command("sioload").ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
