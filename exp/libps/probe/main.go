// Probe: the smallest program the Go library runs, which proves the build
// switch: it prints through the library's putchar and stops. println is
// the runtime's, which writes through putchar; yaroze.Print is FntPrint,
// which the probe does not have. The yaroze package is imported for its
// side: under the Go library's target, it is what links the library in.
package main

import _ "go.chrisrx.dev/psx/yaroze"

func main() {
	println("probe ok")
	for {
	}
}
