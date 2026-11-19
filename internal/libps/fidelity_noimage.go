//go:build yaroze && fidelity && !cleanimage

package libps

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

// The remainders of the reference's printf's time between writes, past what
// ttyPrintf's own work takes, in spin's loops. The reference (blank-ref, named
// hello-ref when examples/blank was the tutorial's hello, measured with iotrace)
// spends 134 cycles before a plain character, 658 before a conversion's first
// digit and 114 before each of the others, 166 before a plain character after a
// conversion and 146 before the "\r" of a "\n", and 40 between the two.
func ttyPlainPad()     { spin(4) }
func ttyFirstDigPad()  { spin(46) }
func ttyDigitPad()     { spin(4) }
func ttyAfterConvPad() { spin(1) }
func ttyNewlinePad()   { spin(3) }

// libTablePad is the time the reference takes to clear a 64-byte table a byte at a
// time (512 cycles) and make the stores around it, measured with iotrace on
// blank-ref (then hello-ref).
func libTablePad() { spinCycles(576) }

// cdStartPad is the time of the two system calls of a critical section the
// reference has where Sony's doesn't.
func cdStartPad() { spinCycles(850) }
