package serial

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"

	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/format/psx"
)

// defaultStack is where a program's stack starts when its file does not
// say, as SIOCONS has it: 16 bytes below the top of the console's 2 MB of
// RAM.
const defaultStack = 0x801ffff0

// Program is an executable as Load uploads it: the bytes to write into RAM
// and the registers to start it with.
type Program struct {
	Segments []Segment

	// PC is where the program starts, GP its gp, which Load leaves alone if
	// it is 0, and SP the top of its stack.
	PC, GP, SP uint32
}

// Segment is bytes to write into RAM at an address, and where they came
// from, such as the section's name, for a loader to say.
type Segment struct {
	Name string
	Addr uint32
	Data []byte
}

// OpenProgram reads an executable in any of the forms LoadFile takes,
// which its first bytes tell apart: a PS-X EXE, an ECOFF executable or an
// ELF. SIOCONS also takes S-records, which are not supported.
func OpenProgram(path string) (*Program, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch {
	case bytes.HasPrefix(data, []byte("PS-X EXE")):
		f, err := psx.ParseFile(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return EXEProgram(f), nil
	case bytes.HasPrefix(data, []byte{0x62, 0x01}): // MIPSEL ECOFF
		f, err := ecoff.NewFile(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return ECOFFProgram(f)
	case bytes.HasPrefix(data, []byte(elf.ELFMAG)):
		f, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return ELFProgram(f)
	case len(data) > 1 && data[0] == 'S' && data[1] >= '0' && data[1] <= '9':
		return nil, fmt.Errorf("serial: %s is S-records, which are not supported", path)
	}
	return nil, fmt.Errorf("serial: %s is not a PS-X EXE, ECOFF or ELF executable", path)
}

// EXEProgram is a PS-X EXE as SIOCONS loads it: its text at its address,
// started at its PC0 with its GP0, and its stack at the end of the one the
// header gives, if it gives one. The data and BSS the header describes are
// not touched.
func EXEProgram(f *psx.File) *Program {
	sp := uint32(defaultStack)
	if f.StackAddr+f.StackSize != 0 {
		sp = f.StackAddr + f.StackSize
	}
	text := f.Section("text").Data
	return &Program{
		Segments: []Segment{{Name: "text", Addr: f.TextAddr, Data: text[:min(len(text), int(f.TextSize))]}},
		PC:       f.PC0,
		GP:       f.GP0,
		SP:       sp,
	}
}

// ECOFF section flags, for the sections SIOCONS loads.
const (
	stypText  = 0x20
	stypData  = 0x40
	stypRdata = 0x100
	stypSdata = 0x200
)

// ECOFFProgram is an ECOFF executable as SIOCONS loads it: its .text,
// .rdata, .data and .sdata sections, each at its physical address, started
// at its entry point with its gp value, and the stack at defaultStack.
func ECOFFProgram(f *ecoff.File) (*Program, error) {
	p := &Program{PC: f.Entry, GP: f.GpValue, SP: defaultStack}
	for _, s := range f.Sections {
		if !loaded(s.SectionName(), s.Flags) {
			continue
		}
		data, err := s.Data()
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			p.Segments = append(p.Segments, Segment{Name: s.SectionName(), Addr: s.PhysicalAddress, Data: data})
		}
	}
	return p, nil
}

// loaded reports whether SIOCONS loads a section: one of four, with both
// its name and its flag.
func loaded(name string, flags int32) bool {
	switch name {
	case ".text":
		return flags&stypText != 0
	case ".rdata":
		return flags&stypRdata != 0
	case ".data":
		return flags&stypData != 0
	case ".sdata":
		return flags&stypSdata != 0
	}
	return false
}

// ELFProgram is a program linked as an ELF, such as yaroze build writes
// with -o NAME.elf: the program alone, for the library the boot disc left
// resident. Its loadable segments' bytes go at their addresses, it starts
// at its entry point with gp at _gp if it has the symbol, which a program
// of ours sets itself anyway, and the stack is at defaultStack.
func ELFProgram(f *elf.File) (*Program, error) {
	p := &Program{PC: uint32(f.Entry), SP: defaultStack}
	for _, prog := range f.Progs {
		if prog.Type != elf.PT_LOAD || prog.Filesz == 0 {
			continue
		}
		data := make([]byte, prog.Filesz)
		if _, err := prog.ReadAt(data, 0); err != nil {
			return nil, err
		}
		p.Segments = append(p.Segments, Segment{Name: fmt.Sprintf("seg%d", len(p.Segments)), Addr: uint32(prog.Vaddr), Data: data})
	}
	if syms, err := f.Symbols(); err == nil {
		for _, s := range syms {
			if s.Name == "_gp" {
				p.GP = uint32(s.Value)
			}
		}
	}
	return p, nil
}
