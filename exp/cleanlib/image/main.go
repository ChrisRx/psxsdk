//go:build yaroze

// Command image is the clean library, exp/cleanlib, laid out as the resident
// library's image, for programs built for that library to run on in place of
// Sony's. make.sh builds it. Everything the image does is in exp/cleanlib:
// the tooling here adds a jump at each entry point and nothing else.
package main

import "go.chrisrx.dev/psx/exp/cleanlib"

// setup is what the resident library's init routine does once the runtime
// has started.
//
//export libps_image_setup
func setup() {
	cleanlib.Startup()
}

func main() {}
