// Command mkscene cooks a scene, written as JSON by the playground's scene
// editor or by hand, into the file a program reads with package scene. It is
// also yaroze's scene command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/mkscene"
)

func main() {
	if err := mkscene.Command("mkscene").Execute(); err != nil {
		log.Fatal(err)
	}
}
