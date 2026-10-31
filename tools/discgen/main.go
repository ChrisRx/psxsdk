// Command discgen writes the Go declarations that put a program's data
// files on its disc, for go:generate:
//
//	//go:generate go run go.chrisrx.dev/psx/tools/discgen data
//
// Package go.chrisrx.dev/psx/internal/discgen says what it writes. It is
// also yaroze's generate command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/cmd/discgen"
)

func main() {
	if err := discgen.Command("discgen").Execute(); err != nil {
		log.Fatal(err)
	}
}
