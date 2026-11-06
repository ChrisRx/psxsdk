//go:build yaroze && fidelity && cleanimage

package cleanlib

// What differs on the resident image from the static fidelity build (spec 30,
// "One more tag"): its start and its GsInitGraph are timed against Sony's
// routine (spec 21, "Start-up").

// imagePad spends c cycles on the resident image only.
//
//go:inline
func imagePad(c uint32) { spinCycles(c) }

// padReadingStart is empty on the resident image: Sony's routine leaves the
// kernel's reading of the controllers to the card driver, which comes next, and
// cardStart has it done (spec 07, step 3, and spec 21).
//
//go:inline
func padReadingStart() {}
