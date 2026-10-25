// Command obj2tmd converts Wavefront OBJ models, which most 3D tools export,
// into TMD files the library draws with GsSortObject4. It is also yaroze's
// tmd command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/obj2tmd"
)

func main() {
	if err := obj2tmd.Command("obj2tmd").Execute(); err != nil {
		log.Fatal(err)
	}
}
