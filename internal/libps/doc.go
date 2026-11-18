//go:build yaroze

// Package libps is the Net Yaroze library in Go, written clean-room:
// from Sony's public manuals, public hardware documentation, this
// repository's emulator, and the resident library's behavior observed
// from outside, never from its code. CLEANROOM.md says what may and may
// not be read while working on it; spec/ holds the behavior to implement.
//
// Each function is exported under its C name, as the resident library's
// entry points were, so that a program built for the playstation-yaroze-go
// target with the cleanlib tag links against it.
package libps

//go:generate go run ./gen/font
//go:generate go run ./gen/sine
