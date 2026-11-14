//go:build yaroze

package yaroze

// The SPU's master volume is zero until something sets it, which left every
// program that forgot SsSetMVol silent. It starts at full instead, before
// main runs; a program that wants it lower, or silent to fade in, sets it
// as before.
func init() {
	SsSetMVol(127, 127)
}
