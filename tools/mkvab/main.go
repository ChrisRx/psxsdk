// Command mkvab builds a VAB sound bank from a definition file and VAG
// files, or reads a bank back into a definition file: what Sony's mkvab did,
// with vabsplit's work as an option. It is also yaroze's vab command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/mkvab"
)

func main() {
	if err := mkvab.Command("mkvab").Execute(); err != nil {
		log.Fatal(err)
	}
}
