//go:build yaroze && !fidelity

package cleanlib

// A program's static build, with nothing of Sony's time or leftover memory to
// match: fidelity.go's operations, empty, for the compiler to remove (spec 30).

//go:inline
func spinCycles(c uint32) {}

//go:inline
func spin(n uint32) {}

//go:inline
func pace(c uint32) {}

//go:inline
func space(n int) {}

//go:inline
func spaceWith(n int, v uint32) {}

//go:inline
func spaceApart(n int, v uint32) {}

//go:inline
func spaceAt(p *uint32) {}

//go:inline
func delay(n uint32) {}

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

// The flush's working memory is what it uses, and the callee's argument home.
const (
	kanjiFrameBytes = kanjiImageWords*4 + kanjiDrawn*8
	kanjiFrameExtra = 16
)

//go:inline
func clearKromStack(frame uint32) {}

//go:inline
func leaveKromGlyph(frame uint32, x, y int, addr uintptr) {}

//go:inline
func leaveKromGlyphs(frame uint32, s *byte) {}

//go:inline
func leaveKromImage(frame uint32, t *uint32, shape kromShape) {}
