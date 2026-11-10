//go:build yaroze && !fidelity

package cleanlib

// A program's static build, with nothing of Sony's time or leftover memory to
// match: fidelity.go's operations, empty, for the compiler to remove (spec 30).

const spinTurn = 12

//go:inline
func spinCycles(c uint32) {}

//go:inline
func spin(n uint32) {}

//go:inline
func pace(c uint32) {}

//go:inline
func space(n int) {}

//go:inline
func spaceRun(n *uint32) {}

//go:inline
func spaceWith(n int, v uint32) {}

// imageBlocksWait waits for a queued image's blocks to go, without Sony's
// accesses between (fidelity.go).
//
//go:inline
func imageBlocksWait() {
	for dmaGPUControl.Get()&dmaBusy != 0 {
	}
}

// imageBlank leaves a vertical blank raised while the images went out to the
// interrupt after them (fidelity.go).
//
//go:inline
func imageBlank() {}

//go:inline
func spaceApart(n int, v uint32) {}

//go:inline
func spaceAt(p *uint32) {}

//go:inline
func nops(n int) {}

//go:inline
func delay(n uint32) {}

//go:inline
func padPow(x float64) {}

//go:inline
func padPowInt() {}

//go:inline
func kromCharTime(c uint16) {}

//go:inline
func tickVoiceTime(sv *sndVoice, e uint16, settled, live *uint32) {}

//go:inline
func tickWordsTime(settled, written, live uint32) {}

//go:inline
func imagePad(c uint32) {}

//go:inline
func libTablePad() {}

//go:inline
func cdStartPad() {}

//go:inline
func ttyPlainPad() {}

//go:inline
func ttyFirstDigPad() {}

//go:inline
func ttyDigitPad() {}

//go:inline
func ttyAfterConvPad() {}

//go:inline
func ttyNewlinePad() {}

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

// padAdd and the rest are what the arithmetic spends for Sony's time (fidelity.go).
func padAdd(a, b, r uint64) {}
func padMul(a, b uint64)    {}
func padDiv(q uint64)       {}
func padFix(a uint64)       {}
func padFloat()             {}
func padSqrt(x uint64)      {}
func padExtend()            {}
func padTrunc()             {}
func padSin(x uint64)       {}
