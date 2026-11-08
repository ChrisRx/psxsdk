// Command gen writes zz_generated_keeps.go, the table of what package
// yaroze's functions keep, from the package's source: go generate in
// internal/keepcheck runs it.
package main

import (
	"log"
	"os"

	"go.chrisrx.dev/psx/internal/keepcheck"
)

func main() {
	t, err := keepcheck.Derive("../../yaroze")
	if err != nil {
		log.Fatal(err)
	}
	src, err := t.Source()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("zz_generated_keeps.go", src, 0o644); err != nil {
		log.Fatal(err)
	}
}
