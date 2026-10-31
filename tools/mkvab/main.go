// Command mkvab builds a VAB sound bank from a definition file and VAG
// files, reads a bank back into a definition file, or splits a bank into
// its VH and VB: what Sony's mkvab and vabsplit did. It is also yaroze's vab
// command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/cmd/mkvab"
)

func main() {
	if err := mkvab.Command("mkvab").Execute(); err != nil {
		log.Fatal(err)
	}
}
