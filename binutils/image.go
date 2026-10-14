package binutils

import (
	"debug/elf"
	"fmt"
	"os"

	"go.chrisrx.dev/psx/format/ecoff"
)

// An Image is a flat memory image of a program together with the address it
// loads at and the address to begin executing.
//
// This is all a Net Yaroze executable ever really was. The boot disk's serial
// loader is handed bytes, a destination and an entry point; the container they
// arrived in is not part of the picture. That is why the SDK's choice of ECOFF
// does not constrain anything downstream, and why a program built as
// mipsel-none-elf can be loaded and converted by exactly the same tools.
type Image struct {
	// Entry is the address execution begins at.
	Entry uint32

	// Addr is the address the image loads at.
	Addr uint32

	// Data is the contents of the image, with any gaps between sections
	// zero filled.
	Data []byte
}

// Size returns the length of the image in bytes.
func (i *Image) Size() uint32 {
	return uint32(len(i.Data))
}

// LoadImage reads a flat image from an executable, which may be either ECOFF
// or ELF.
func LoadImage(path string) (*Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// debug/elf is strict about the magic, so it is the cheaper check and
	// cannot be confused by an ECOFF file.
	if ef, err := elf.NewFile(f); err == nil {
		defer ef.Close()
		return elfImage(ef)
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	ec, err := ecoff.NewFile(f)
	if err != nil {
		return nil, fmt.Errorf("%s: not an ECOFF or ELF executable", path)
	}
	return &Image{
		Entry: ec.Entry,
		Addr:  ec.TextStart,
		Data:  ec.Data(),
	}, nil
}

// elfImage flattens the loadable segments of an ELF executable.
func elfImage(f *elf.File) (*Image, error) {
	if f.Type != elf.ET_EXEC {
		return nil, fmt.Errorf("ELF file is %v, not an executable", f.Type)
	}
	if f.Machine != elf.EM_MIPS && f.Machine != elf.EM_MIPS_RS3_LE {
		return nil, fmt.Errorf("ELF machine is %v, not MIPS", f.Machine)
	}
	if f.ByteOrder.String() != "LittleEndian" {
		return nil, fmt.Errorf("ELF file is big-endian; the PlayStation R3000A is little-endian")
	}

	// Only PT_LOAD segments with file contents end up in the image. A .bss
	// segment has a memory size but no file size, and the startup code
	// clears it, so there is nothing to carry.
	var lo, hi uint64
	first := true
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Filesz == 0 {
			continue
		}
		if first || p.Vaddr < lo {
			lo = p.Vaddr
		}
		if first || p.Vaddr+p.Filesz > hi {
			hi = p.Vaddr + p.Filesz
		}
		first = false
	}
	if first {
		return nil, fmt.Errorf("ELF file has no loadable segments with contents")
	}
	if hi-lo > 2*1024*1024 {
		return nil, fmt.Errorf("image spans %d bytes, more than the console's 2MB of RAM", hi-lo)
	}

	data := make([]byte, hi-lo)
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Filesz == 0 {
			continue
		}
		b := make([]byte, p.Filesz)
		if _, err := p.ReadAt(b, 0); err != nil {
			return nil, fmt.Errorf("reading segment at 0x%X: %v", p.Vaddr, err)
		}
		copy(data[p.Vaddr-lo:], b)
	}
	return &Image{
		Entry: uint32(f.Entry),
		Addr:  uint32(lo),
		Data:  data,
	}, nil
}
