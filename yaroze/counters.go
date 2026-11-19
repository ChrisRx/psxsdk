package yaroze

// The CPU's root counters, three timers a program may use: each counts
// to 65535 and round again, so a span is the difference of two
// readings. The fourth, the vertical blanks, is the library's, which
// FieldCount and LinesSinceBlank read.

// A Counter is one of the root counters.
type Counter int

const (
	// DotClock counts the GPU's dot clock.
	DotClock Counter = 0

	// Lines counts the television's horizontal lines, 15734 a second on
	// NTSC and 15625 on PAL.
	Lines Counter = 1

	// SystemClock counts the system clock divided by eight, 4233600 a
	// second.
	SystemClock Counter = 2
)

// Start starts the counter from 0.
func (c Counter) Start() { startCounter(c) }

// Reset puts the counter back to 0.
func (c Counter) Reset() { resetCounter(c) }

// Read returns the counter's count.
func (c Counter) Read() int { return readCounter(c) }

// FieldCount is how many vertical blanks have come since the library
// started: sixty a second on an NTSC console and fifty on a PAL one,
// which keeps a clock without a counter.
func FieldCount() int { return fieldCount() }

// LinesSinceBlank is how many lines the television has drawn since the
// last vertical blank, which read when a frame's work is done says how
// much of the frame it took: a field is 263 lines on NTSC and 313 on
// PAL, and work that takes more misses the next blank.
func LinesSinceBlank() int { return linesSinceBlank() }
