//go:build yaroze

// Package libps is the Net Yaroze library in Go, written clean-room:
// from Sony's public manuals, public hardware documentation, this
// repository's emulator, and the resident library's behavior observed
// from outside, never from its code. CLEANROOM.md says what may and may
// not be read while working on it; spec/ holds the behavior to implement.
//
// Each function is exported under its C name, as the resident library's
// entry points were, so that a program built for the playstation-yaroze-clean
// target, the default, links against it statically. The target sets the
// cleanlib tag. Built as the resident image (task image), the library is
// what a program built with --libps resident is laid over.
package libps

//go:generate go run ./gen/font
//go:generate go run ./gen/sine
