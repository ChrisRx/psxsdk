//go:build yaroze && !cleanimage

package libps

// The image has __addsf3 and a program's __divdf3 in assembly, at Sony's time for what they
// are given (image/clib.S, spec 34).

//export __addsf3
func softAddsf3(a, b uint32) uint32 { return sAdd(a, b) }

//export libps_c___divdf3
func programDivdf3(a, b uint64) uint64 { return softDivdf3(a, b) }
