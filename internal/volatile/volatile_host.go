//go:build !yaroze

package volatile

// On a host there is no hardware behind the memory and nothing patches the
// executable, so plain loads and stores do.

// A Register32 is a word of memory-mapped hardware, read and written as
// is, every time.
type Register32 struct {
	Reg uint32
}

// Get reads the register.
func (r *Register32) Get() uint32 { return r.Reg }

// Set writes the register.
func (r *Register32) Set(v uint32) { r.Reg = v }

// LoadUint32 reads the word at p, every time it is called.
func LoadUint32(p *uint32) uint32 { return *p }

// StoreUint32 writes the word at p, every time it is called.
func StoreUint32(p *uint32, v uint32) { *p = v }

// LoadUint16 reads the half-word at p, every time it is called.
func LoadUint16(p *uint16) uint16 { return *p }

// StoreUint16 writes the half-word at p, every time it is called.
func StoreUint16(p *uint16, v uint16) { *p = v }

// LoadUint8 reads the byte at p, every time it is called.
func LoadUint8(p *uint8) uint8 { return *p }

// StoreUint8 writes the byte at p, every time it is called.
func StoreUint8(p *uint8, v uint8) { *p = v }
