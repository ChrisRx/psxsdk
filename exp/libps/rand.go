//go:build yaroze

package libps

// seed is the C library's random seed.
//
// libps 0x80062CEC
var seed uint32

// rand is the C library's linear congruential generator: the seed steps
// by 1103515245 and 12345, and the result is bits 16 to 30 of it.
//
// libps 0x80043B30, 48 bytes
//
//export rand
func rand() int32 {
	seed = seed*0x41c64e6d + 12345
	return int32(seed>>16) & 0x7fff
}

// srand sets the seed.
//
// libps 0x80043B60, 16 bytes
//
//export srand
func srand(s uint32) {
	seed = s
}

// Rand and Srand are rand and srand for other packages, which cannot
// reach the unexported names the C library's names were given to.
func Rand() int32    { return rand() }
func Srand(s uint32) { srand(s) }
