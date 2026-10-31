// The program the serial tests upload to see what the monitor does when a
// program crashes: it prints a line, then loads a word from an odd
// address, which raises an address error exception.
package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

func main() {
	println("crashing")
	volatile.LoadUint32((*uint32)(yaroze.Pointer(0x80100001)))
	println("survived")
}
