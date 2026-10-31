package ps

// Rand returns the next of the library's random numbers, 0 to 32767: the
// C library's generator, which Seed starts.
func Rand() int { return randNext() }

// Seed starts the random numbers from s.
func Seed(s uint32) { randSeed(s) }
