package emu

import (
	"encoding/binary"
	"fmt"
)

// device is something mapped into the physical address space. Offsets are
// relative to where it is mapped.
type device interface {
	Read32(off uint32) uint32
	Read16(off uint32) uint16
	Read8(off uint32) uint8
	Write32(off, v uint32)
	Write16(off uint32, v uint16)
	Write8(off uint32, v uint8)
}

// mapping places a device in the physical address space. delay is the
// cycles an access to it costs.
type mapping struct {
	base, size uint32
	delay      uint32
	dev        device
}

// The address space is decoded a page at a time. A page that one mapping
// covers, or none, decodes in one lookup. The few that are shared, among
// the small I/O devices, are decoded again at a finer grain, to which every
// mapping in them is aligned.
const (
	pageShift  = 16
	pages      = 1 << (32 - pageShift)
	grainShift = 2
	grains     = 1 << (pageShift - grainShift)

	// pageShared marks a page entry as the index of a fine table rather
	// than of a mapping.
	pageShared = 0x8000
)

// Bus decodes addresses to devices the way psxe does: the segment is masked
// off, then the mappings are tried in order. The order is worked out ahead
// of time, into tables that give the first mapping holding each address.
type Bus struct {
	maps   []mapping
	cycles uint32

	// page holds, for each page, 0 for nothing mapped, a mapping's index
	// plus one, or pageShared and the index of the page's fine table,
	// which holds a mapping's index plus one, or 0, for each grain.
	page *[pages]uint16
	fine []*[grains]uint8

	// ram is where every access goes without decoding, and code the other
	// places Fetch does.
	ram  window
	code []window

	// sioCtrl is the one register psxe answers for SIO1, which it does
	// not otherwise emulate. Nothing ever writes it.
	sioCtrl uint16
}

// segmentMask strips the segment from a virtual address, indexed by its top
// three bits. KUSEG passes through, KSEG0 and KSEG1 map onto the first
// 512MB, and KSEG2 is left alone.
var segmentMask = [8]uint32{
	0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff,
	0x7fffffff, 0x1fffffff, 0xffffffff, 0xffffffff,
}

func (b *Bus) find(addr uint32) (*mapping, uint32, bool) {
	i := uint32(b.page[addr>>pageShift])
	if i&pageShared != 0 {
		i = uint32(b.fine[i&^pageShared][addr>>grainShift&(grains-1)])
	}
	if i == 0 {
		return nil, 0, false
	}
	m := &b.maps[i-1]
	return m, addr - m.base, true
}

// setMaps installs the memory map, in order of precedence.
func (b *Bus) setMaps(maps []mapping) {
	if len(maps) > 0xff {
		panic(fmt.Sprintf("bus: %d mappings is more than a fine table can index", len(maps)))
	}
	b.maps = maps
	b.page = new([pages]uint16)
	b.fine = nil

	// first returns the index plus one of the first mapping holding addr.
	first := func(addr uint32) int {
		for i, m := range maps {
			if addr-m.base < m.size {
				return i + 1
			}
		}
		return 0
	}
	for p := range uint32(pages) {
		start, end := uint64(p)<<pageShift, uint64(p+1)<<pageShift
		// The page decodes to one mapping if the first that reaches into
		// it covers all of it.
		shared := false
		for _, m := range maps {
			base, limit := uint64(m.base), uint64(m.base)+uint64(m.size)
			if base >= end || limit <= start {
				continue
			}
			shared = base > start || limit < end
			break
		}
		if !shared {
			b.page[p] = uint16(first(uint32(start)))
			continue
		}

		for _, m := range maps {
			if m.base&(1<<grainShift-1) != 0 || m.size&(1<<grainShift-1) != 0 {
				panic(fmt.Sprintf("bus: mapping at %08x of %x bytes is not aligned to the %d-byte grain", m.base, m.size, 1<<grainShift))
			}
		}
		t := new([grains]uint8)
		for g := range uint32(grains) {
			t[g] = uint8(first(uint32(start) + g<<grainShift))
		}
		b.page[p] = pageShared | uint16(len(b.fine))
		b.fine = append(b.fine, t)
	}
}

// window is memory read and written directly, standing for the mapping
// that decodes every address from base to base+size.
type window struct {
	base, size uint32
	delay      uint32
	mem        memory
}

// window returns a window onto mem from base to base+size, which must be
// what the mapping there reads and writes.
func (b *Bus) window(base, size uint32, mem memory) window {
	m, _, ok := b.find(base)
	for a := uint64(base); ok && a < uint64(base)+uint64(size); a += 1 << grainShift {
		n, _, found := b.find(uint32(a))
		ok = found && n == m
	}
	if !ok {
		panic(fmt.Sprintf("bus: %08x to %08x is not one mapping", base, uint64(base)+uint64(size)))
	}
	return window{base: base, size: size, delay: m.delay, mem: mem}
}

// direct installs the windows: ram for every access, and code for fetching
// instructions only. Accesses through them cost what decoding would.
func (b *Bus) direct(ram window, code ...window) {
	b.ram = ram
	b.code = code
}

// Fetch reads an instruction. It costs what a 32-bit read costs, and
// leaves nothing charged after it.
//
// In psxe only the instruction fetch is ever charged: data accesses set the
// count too, but the next fetch overwrites it before anything reads it. An
// unmapped fetch charges what the last access did.
func (b *Bus) Fetch(addr uint32) (insn, cycles uint32) {
	a := addr & segmentMask[addr>>29]
	if off := a - b.ram.base; off < b.ram.size {
		b.cycles = 0
		return b.ram.mem.Read32(off), b.ram.delay
	}
	for i := range b.code {
		if w := &b.code[i]; a-w.base < w.size {
			b.cycles = 0
			return w.mem.Read32(a - w.base), w.delay
		}
	}
	insn = b.Read32(addr)
	cycles, b.cycles = b.cycles, 0
	return insn, cycles
}

func (b *Bus) Read32(addr uint32) uint32 {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		return b.ram.mem.Read32(off)
	}
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		return m.dev.Read32(off)
	}
	// An unmapped 32-bit read leaves the cycle count as it was.
	return 0
}

func (b *Bus) Read16(addr uint32) uint16 {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		return b.ram.mem.Read16(off)
	}
	b.cycles = 2
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		return m.dev.Read16(off)
	}
	switch addr {
	case 0x1f80105a:
		return b.sioCtrl
	case 0x1f801054:
		return 0x05 // SIO1 status: transmitter ready and idle
	case 0x1f400004:
		return 0xc8
	case 0x1f400006:
		return 0x1fe0
	}
	return 0
}

func (b *Bus) Read8(addr uint32) uint8 {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		return b.ram.mem.Read8(off)
	}
	b.cycles = 2
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		return m.dev.Read8(off)
	}
	return 0
}

func (b *Bus) Write32(addr, v uint32) {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		b.ram.mem.Write32(off, v)
		return
	}
	b.cycles = 0
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		m.dev.Write32(off, v)
	}
}

func (b *Bus) Write16(addr uint32, v uint16) {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		b.ram.mem.Write16(off, v)
		return
	}
	b.cycles = 0
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		m.dev.Write16(off, v)
	}
}

func (b *Bus) Write8(addr uint32, v uint8) {
	addr &= segmentMask[addr>>29]
	if off := addr - b.ram.base; off < b.ram.size {
		b.cycles = b.ram.delay
		b.ram.mem.Write8(off, v)
		return
	}
	b.cycles = 0
	if m, off, ok := b.find(addr); ok {
		b.cycles = m.delay
		m.dev.Write8(off, v)
	}
}

// memory is plain little-endian storage. Offsets are wrapped to its size,
// which must be a power of two, so it repeats through a larger window.
type memory []byte

func (m memory) Read32(off uint32) uint32 {
	off &= uint32(len(m) - 1)
	return binary.LittleEndian.Uint32(m[off:])
}

func (m memory) Read16(off uint32) uint16 {
	off &= uint32(len(m) - 1)
	return binary.LittleEndian.Uint16(m[off:])
}

func (m memory) Read8(off uint32) uint8 {
	return m[off&uint32(len(m)-1)]
}

func (m memory) Write32(off, v uint32) {
	off &= uint32(len(m) - 1)
	binary.LittleEndian.PutUint32(m[off:], v)
}

func (m memory) Write16(off uint32, v uint16) {
	off &= uint32(len(m) - 1)
	binary.LittleEndian.PutUint16(m[off:], v)
}

func (m memory) Write8(off uint32, v uint8) {
	m[off&uint32(len(m)-1)] = v
}

// rom is memory that ignores writes.
type rom struct{ memory }

func (rom) Write32(uint32, uint32) {}
func (rom) Write16(uint32, uint16) {}
func (rom) Write8(uint32, uint8)   {}

// unmapped answers every read with zero and ignores writes. It stands in for
// the devices not emulated yet.
type unmapped struct{}

func (unmapped) Read32(uint32) uint32   { return 0 }
func (unmapped) Read16(uint32) uint16   { return 0 }
func (unmapped) Read8(uint32) uint8     { return 0 }
func (unmapped) Write32(uint32, uint32) {}
func (unmapped) Write16(uint32, uint16) {}
func (unmapped) Write8(uint32, uint8)   {}
