// Package hot loads hot reload patches into a running program: the main
// package's functions compiled afresh, as the compiler builds them with
// -hot-base against the ELF of a program built with -hot (see
// transform/hot.go in the TinyGo tree under internal/toolchain/_tinygo).
//
// A program built with -hot calls each of its main package's functions
// through a slot, a word holding the function's address, and reserves a
// region of memory above its own for patches. A patch is an ELF linked at
// an address in that region, holding the new functions and a table,
// __hot_patch, of the slots to point at them. Apply copies the patch in,
// zeroes what it leaves uninitialized, and writes the slots, which is safe
// only between frames: ApplyAt does it the next time the program calls
// the library's VSync, where every frame of the main package's code has
// returned.
package hot

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"go.chrisrx.dev/psx/emu"
)

// A Patch is a hot reload patch read from its ELF.
type Patch struct {
	// Start and End are the memory the patch occupies: its code and data,
	// which Loads fill, and what follows up to End, which is zeroed.
	Start, End uint32
	// Loads are the patch's bytes, by address.
	Loads []Load
	// Slots are the words to write, each the address of a slot in the
	// running program and the address of the function to put there.
	Slots []Slot
	// Inits are the variables whose initial value changed: each is given
	// New if it still holds Old, which is the running program not having
	// written it since it started.
	Inits []Init
	// Safe is the address of the library's VSync, where the program is
	// between frames.
	Safe uint32
}

// A Load is bytes to put in memory at an address.
type Load struct {
	Addr uint32
	Data []byte
}

// A Slot is a word of the running program to write.
type Slot struct {
	Addr, Value uint32
}

// An Init is a variable of the running program with its initial bytes in
// the program as it was built, and as the patch would have them.
type Init struct {
	Addr     uint32
	Old, New []byte
}

// The globals the compiler emits for the loader.
const (
	tableName = "__hot_patch"
	initsName = "__hot_inits"
)

// Parse reads a patch from its ELF.
func Parse(data []byte) (*Patch, error) {
	f, err := elf.NewFile(bytesReaderAt(data))
	if err != nil {
		return nil, fmt.Errorf("hot: reading the patch: %w", err)
	}
	defer f.Close()
	// The sections, rather than the segments: the linker puts the ELF's
	// own headers in a segment of their own, which is not the patch's.
	p := &Patch{}
	for _, sec := range f.Sections {
		if sec.Flags&elf.SHF_ALLOC == 0 || sec.Size == 0 {
			continue
		}
		if sec.Type != elf.SHT_NOBITS {
			b, err := sec.Data()
			if err != nil {
				return nil, fmt.Errorf("hot: reading the patch's %s: %w", sec.Name, err)
			}
			p.Loads = append(p.Loads, Load{Addr: uint32(sec.Addr), Data: b})
		}
		if p.Start == 0 || uint32(sec.Addr) < p.Start {
			p.Start = uint32(sec.Addr)
		}
		if end := uint32(sec.Addr + sec.Size); end > p.End {
			p.End = end
		}
	}
	if len(p.Loads) == 0 {
		return nil, errors.New("hot: the patch has nothing to load")
	}
	sort.Slice(p.Loads, func(i, j int) bool { return p.Loads[i].Addr < p.Loads[j].Addr })

	syms, err := f.Symbols()
	if err != nil {
		return nil, fmt.Errorf("hot: reading the patch's symbols: %w", err)
	}
	var table, inits *elf.Symbol
	for i := range syms {
		switch syms[i].Name {
		case tableName:
			table = &syms[i]
		case initsName:
			inits = &syms[i]
		}
	}
	if table == nil {
		return nil, fmt.Errorf("hot: the patch has no %s: it was not built with -hot-base", tableName)
	}
	head := p.read(uint32(table.Value), 8)
	if head == nil {
		return nil, fmt.Errorf("hot: %s lies outside the patch", tableName)
	}
	p.Safe = binary.LittleEndian.Uint32(head)
	n := binary.LittleEndian.Uint32(head[4:])
	entries := p.read(uint32(table.Value)+8, 8*n)
	if entries == nil {
		return nil, fmt.Errorf("hot: %s lists %d slots, past the patch's end", tableName, n)
	}
	for i := range n {
		e := entries[8*i:]
		p.Slots = append(p.Slots, Slot{Addr: binary.LittleEndian.Uint32(e), Value: binary.LittleEndian.Uint32(e[4:])})
	}
	if inits != nil {
		head := p.read(uint32(inits.Value), 4)
		if head == nil {
			return nil, fmt.Errorf("hot: %s lies outside the patch", initsName)
		}
		n := binary.LittleEndian.Uint32(head)
		entries := p.read(uint32(inits.Value)+4, 16*n)
		if entries == nil {
			return nil, fmt.Errorf("hot: %s lists %d variables, past the patch's end", initsName, n)
		}
		for i := range n {
			e := entries[16*i:]
			addr, size := binary.LittleEndian.Uint32(e), binary.LittleEndian.Uint32(e[4:])
			old, now := p.read(binary.LittleEndian.Uint32(e[8:]), size), p.read(binary.LittleEndian.Uint32(e[12:]), size)
			if old == nil || now == nil {
				return nil, fmt.Errorf("hot: the initial values of the variable at %08x lie outside the patch", addr)
			}
			if bytes.Equal(old, now) {
				continue
			}
			p.Inits = append(p.Inits, Init{Addr: addr, Old: old, New: now})
		}
	}
	return p, nil
}

// read returns n bytes of the patch at addr, or nil.
func (p *Patch) read(addr, n uint32) []byte {
	for _, l := range p.Loads {
		if addr >= l.Addr && addr+n <= l.Addr+uint32(len(l.Data)) {
			return l.Data[addr-l.Addr : addr-l.Addr+n]
		}
	}
	return nil
}

// Size is how much of the region the patch takes, including what it
// zeroes.
func (p *Patch) Size() uint32 { return p.End - p.Start }

// Apply loads the patch into m now, gives each variable the program has
// not written its new initial value, and writes the slots. The program
// must be between frames: see ApplyAt. It returns how many variables were
// given new values.
func (p *Patch) Apply(m *emu.Machine) int {
	for addr := p.Start; addr < p.End; addr += 4 {
		m.Bus.Write32(addr, 0)
	}
	for _, l := range p.Loads {
		for i, b := range l.Data {
			m.Bus.Write8(l.Addr+uint32(i), b)
		}
	}
	reset := 0
	for _, in := range p.Inits {
		untouched := true
		for i, b := range in.Old {
			if m.Bus.Read8(in.Addr+uint32(i)) != b {
				untouched = false
				break
			}
		}
		if !untouched {
			continue
		}
		for i, b := range in.New {
			m.Bus.Write8(in.Addr+uint32(i), b)
		}
		reset++
	}
	for _, s := range p.Slots {
		m.Bus.Write32(s.Addr, s.Value)
	}
	return reset
}

// ApplyAt applies the patch the next time the program reaches the
// library's VSync, where every function of the main package has returned
// for the frame, and then calls done, if it is not nil. The breakpoint it
// sets at VSync is cleared afterwards, so a program with a breakpoint of
// its own there loses it.
func (p *Patch) ApplyAt(m *emu.Machine, done func()) {
	m.SetBreak(p.Safe, func(m *emu.Machine) bool {
		m.ClearBreak(p.Safe)
		p.Apply(m)
		if done != nil {
			done()
		}
		return false
	})
}

// Region reads where a program built with -hot keeps its patches from its
// ELF: the region's start and end, or an error for a program not built
// with -hot. A program so large that it leaves no room has end at start.
func Region(data []byte) (start, end uint32, err error) {
	f, err := elf.NewFile(bytesReaderAt(data))
	if err != nil {
		return 0, 0, fmt.Errorf("hot: reading the program: %w", err)
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return 0, 0, fmt.Errorf("hot: reading the program's symbols: %w", err)
	}
	for _, s := range syms {
		switch s.Name {
		case "__hot_start":
			start = uint32(s.Value)
		case "__hot_end":
			end = uint32(s.Value)
		}
	}
	if start == 0 || end < start {
		return 0, 0, errors.New("hot: the program was not built with -hot: it has no region for patches")
	}
	return start, end, nil
}
