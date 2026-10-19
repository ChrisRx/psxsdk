//go:build !yaroze

package scene

// setUp is the library's part of Load, which there is no library for on a
// host; the fields parse filled are what a test looks at.
func (s *Scene) setUp() {}
