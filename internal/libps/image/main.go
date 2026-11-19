//go:build yaroze

// Command image is the clean library, internal/libps, laid out as the resident
// library's image, for programs built for that library to run on in place of
// Sony's. make.sh builds it. Everything the image does is in internal/libps:
// the tooling here adds a jump at each entry point and nothing else.
package main

import "go.chrisrx.dev/psx/internal/libps"

// setup is what the resident library's init routine does once the runtime
// has started.
//
//export libps_image_setup
func setup() {
	libps.Startup()
}

func main() {}
