package yaroze

// Random returns the next of the library's random numbers, 0 to 32767,
// as an int: Rand's generator, which Seed and Srand start.
func Random() int { return randNext() }

// Seed starts the random numbers from s, as Srand does.
func Seed(s int) { randSeed(s) }
