package cpu

// breakRange is the memory breakpoints can be set in: the console's RAM,
// at any of the addresses it is seen through, which is where a program's
// code is. The BIOS is left out.
const breakRange = 2 << 20

// Breaks is a set of addresses Run stops before: a bit for every word of
// RAM, so that having any number of breakpoints costs each step the same,
// one load and one test.
type Breaks struct {
	bits [breakRange / 4 / 64]uint64
	n    int
}

// index returns the bit for addr, and whether it has one.
func index(addr uint32) (word, bit uint32, ok bool) {
	off := addr & 0x1fffffff
	if off >= breakRange {
		return 0, 0, false
	}
	off >>= 2
	return off / 64, off % 64, true
}

// Set adds addr to the set. An address outside RAM is not kept.
func (b *Breaks) Set(addr uint32) {
	w, bit, ok := index(addr)
	if !ok || b.bits[w]&(1<<bit) != 0 {
		return
	}
	b.bits[w] |= 1 << bit
	b.n++
}

// Clear takes addr out of the set.
func (b *Breaks) Clear(addr uint32) {
	w, bit, ok := index(addr)
	if !ok || b.bits[w]&(1<<bit) == 0 {
		return
	}
	b.bits[w] &^= 1 << bit
	b.n--
}

// Has reports whether addr is in the set.
func (b *Breaks) Has(addr uint32) bool {
	off := addr & 0x1fffffff
	if off >= breakRange {
		return false
	}
	off >>= 2
	return b.bits[off/64]&(1<<(off%64)) != 0
}

// Len returns how many addresses are in the set.
func (b *Breaks) Len() int { return b.n }
