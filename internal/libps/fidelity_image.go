//go:build yaroze && fidelity && cleanimage

package libps

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

// The remainders of Sony's printf's time between writes in the library's own lines,
// past what ttyPrintf's own work takes. From the calls of the kernel's write in a
// trace, Sony's takes 582 cycles a plain character, 1,020 to a conversion's first
// digit, 578 to each of the others, 618 to a plain character after a conversion
// and 1,154 for the two writes of a "\n", and ttyPrintf's work comes to 580, 636,
// 554, 594 and 1,160.
func ttyPlainPad()     {}
func ttyFirstDigPad()  { spin(31) }
func ttyDigitPad()     { spin(1) }
func ttyAfterConvPad() { spin(1) }
func ttyNewlinePad()   {}

// libTablePad brings the library's first kernel calls to Sony's spacing: 680
// cycles from SetCustomExitFromException to the kernel's _96_remove.
func libTablePad() { spinCycles(220) }

// cdStartPad is empty: Sony's masks the interrupts here, with no system call.
//
//go:inline
func cdStartPad() {}
