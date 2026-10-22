// Command mkdisc builds a PlayStation disc image from a directory, the
// files placed on it and the program that boots it. It is also yaroze's iso
// command.
package main

import (
	"fmt"
	"os"

	"go.chrisrx.dev/psx/internal/mkdisc"
)

func main() {
	cmd := mkdisc.Command("mkdisc")
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "mkdisc: %v\n", err)
		os.Exit(1)
	}
}
