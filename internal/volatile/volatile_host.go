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
