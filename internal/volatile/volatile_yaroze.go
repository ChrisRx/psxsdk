//go:build yaroze

package volatile

import "runtime/volatile"

// On the console these are TinyGo's, which the compiler leaves alone.

// A Register32 is a word of memory-mapped hardware, read and written as
// is, every time.
type Register32 = volatile.Register32

// LoadUint32 reads the word at p, every time it is called.
//
//go:inline
func LoadUint32(p *uint32) uint32 { return volatile.LoadUint32(p) }
