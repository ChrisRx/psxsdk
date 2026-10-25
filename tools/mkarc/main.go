// Command mkarc packs files into one archive for the disc, which a program
// reads with package archive. It is also yaroze's arc command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/mkarc"
)

func main() {
	if err := mkarc.Command("mkarc").Execute(); err != nil {
		log.Fatal(err)
	}
}
