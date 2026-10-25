// Command sioload uploads an ECOFF executable to a Net Yaroze over the
// serial cable, as siocons did, and with --exec runs it. It is also yaroze's
// sio command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/sioload"
)

func main() {
	if err := sioload.Command("sioload").Execute(); err != nil {
		log.Fatal(err)
	}
}
