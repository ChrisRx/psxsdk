//go:build yaroze

package disc

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/discrecord"
)

// The record a File is on the console, where a pointer and a string's
// length are 32 bits, is what mkpsxiso looks for; these fail the build if it
// drifts from what discrecord says. On a host the type is wider and nothing
// reads it as a record.
var (
	_ = [1]struct{}{}[unsafe.Sizeof(File{})-discrecord.Size]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.Name)-discrecord.OffName]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.src)-discrecord.OffSrc]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.sector)-discrecord.OffSector]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.size)-discrecord.OffSize]
	_ = [1]struct{}{}[unsafe.Offsetof(File{}.dst)-discrecord.OffDst]
)
