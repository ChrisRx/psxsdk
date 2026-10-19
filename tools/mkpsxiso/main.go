// Command mkpsxiso builds a PlayStation disc image from a directory. It is
// also nyc's iso command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/mkpsxiso"
)

func main() {
	if err := mkpsxiso.Command("mkpsxiso").Execute(); err != nil {
		log.Fatal(err)
	}
}
