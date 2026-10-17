//go:build yaroze

package disc

import "unsafe"

// The record a File is on the console, where a pointer and a string's
// length are 32 bits, is what mkpsxiso looks for; these fail the build if it
// drifts from what layout.go says. On a host the type is wider and nothing
// reads it as a record.
var (
	_ = [1]struct{}{}[unsafe.Sizeof(File{})-recordSize]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.Name)-offName]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.src)-offSrc]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.sector)-offSector]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.size)-offSize]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.dst)-offDst]
)
