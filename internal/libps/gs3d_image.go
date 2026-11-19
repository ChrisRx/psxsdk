//go:build yaroze && cleanimage

package libps

// GsInitCoordinate2 is gs3d_static.go's on the resident image, in image/clib.S: the
// compiler would begin it with a branch for a nil base, and the entry's first
// instruction has to go in the delay of the image's jump (spec 33). A nil base
// writes to memory at 0, as Sony's does.
//
//export GsInitCoordinate2
func GsInitCoordinate2(super, base *GsCOORDINATE2)
