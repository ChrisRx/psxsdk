//go:build yaroze && fidelity && !cleanimage

package cleanlib

// The static fidelity build, compared with a reference: not laid out as the
// resident image (spec 30, "One more tag"). A linked-in library's first
// ResetGraph does the image's start-up work and has its time from the
// reference.

//go:inline
func imagePad(c uint32) {}

// padReadingStart starts the kernel's reading of the controllers and leaves
// the vertical blank's acknowledgement to the library, which the driver would
// otherwise do itself, before the library's handler counted it.
//
//go:inline
func padReadingStart() {
	libps_biosB(biosStartPad, 0, 0, 0, 0)
	libps_biosB(biosClearPad, 0, 0, 0, 0)
}
