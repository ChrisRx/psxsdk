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

// StoreUint32 writes the word at p, every time it is called.
func StoreUint32(p *uint32, v uint32) { volatile.StoreUint32(p, v) }

// LoadUint16 reads the half-word at p, every time it is called.
//
//go:inline
func LoadUint16(p *uint16) uint16 { return volatile.LoadUint16(p) }

// StoreUint16 writes the half-word at p, every time it is called.
func StoreUint16(p *uint16, v uint16) { volatile.StoreUint16(p, v) }

// LoadUint8 reads the byte at p, every time it is called.
//
//go:inline
func LoadUint8(p *uint8) uint8 { return volatile.LoadUint8(p) }

// StoreUint8 writes the byte at p, every time it is called.
func StoreUint8(p *uint8, v uint8) { volatile.StoreUint8(p, v) }
