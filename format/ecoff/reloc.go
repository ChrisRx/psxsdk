package ecoff

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RelocationType represents the r_type field of an ECOFF relocation entry.
type RelocationType uint8

const (
	R_IGNORE  RelocationType = 0 /* no relocation */
	R_REFHALF RelocationType = 1 /* 16-bit reference */
	R_REFWORD RelocationType = 2 /* 32-bit reference */
	R_JMPADDR RelocationType = 3 /* 26-bit jump target */
	R_REFHI   RelocationType = 4 /* high 16 bits of a 32-bit reference */
	R_REFLO   RelocationType = 5 /* low 16 bits of a 32-bit reference */
	R_GPREL   RelocationType = 6 /* gp-relative 16-bit reference */
	R_LITERAL RelocationType = 7 /* gp-relative reference into .lit{4,8} */
)

func (t RelocationType) String() string {
	switch t {
	case R_IGNORE:
		return "IGNORE"
	case R_REFHALF:
		return "REFHALF"
	case R_REFWORD:
		return "REFWORD"
	case R_JMPADDR:
		return "JMPADDR"
	case R_REFHI:
		return "REFHI"
	case R_REFLO:
		return "REFLO"
	case R_GPREL:
		return "GPREL"
	case R_LITERAL:
		return "LITERAL"
	}
	return fmt.Sprintf("UNKNOWN(%d)", uint8(t))
}

// Relocation section numbers. When the r_extern bit of a relocation is unset,
// the symbol index field holds one of these values instead, naming the section
// the reference is relative to.
const (
	RS_NONE   = 0
	RS_TEXT   = 1
	RS_RDATA  = 2
	RS_DATA   = 3
	RS_SDATA  = 4
	RS_SBSS   = 5
	RS_BSS    = 6
	RS_INIT   = 7
	RS_LIT8   = 8
	RS_LIT4   = 9
	RS_XDATA  = 10
	RS_PDATA  = 11
	RS_FINI   = 12
	RS_LITA   = 13
	RS_ABS    = 14
	RS_RCONST = 15
)

var relocSectionNames = [...]string{
	RS_NONE:   "none",
	RS_TEXT:   S_TEXT,
	RS_RDATA:  S_RDATA,
	RS_DATA:   S_DATA,
	RS_SDATA:  S_SDATA,
	RS_SBSS:   S_SBSS,
	RS_BSS:    S_BSS,
	RS_INIT:   S_INIT,
	RS_LIT8:   S_LIT8,
	RS_LIT4:   S_LIT4,
	RS_XDATA:  S_XDATA,
	RS_PDATA:  S_PDATA,
	RS_FINI:   S_FINI,
	RS_LITA:   S_LITA,
	RS_ABS:    "*ABS*",
	RS_RCONST: S_RCONST,
}

// RelocationSectionName returns the section name for a relocation section
// number.
func RelocationSectionName(n uint32) string {
	if n < uint32(len(relocSectionNames)) {
		return relocSectionNames[n]
	}
	return fmt.Sprintf("section(%d)", n)
}

// RelocationSize is the size in bytes of an on-disk ECOFF relocation entry.
const RelocationSize = 8

// A Relocation represents a single entry in an ECOFF relocation table.
//
// On disk an entry is a 32-bit virtual address followed by four bytes of
// packed fields. The packing differs between big- and little-endian files, so
// the symbol index, type and extern flag are unpacked at parse time.
type Relocation struct {
	// Address is the offset within the containing section of the storage
	// unit being relocated.
	Address uint32

	// SymbolIndex is an index into the external symbol table when External
	// is true, and a relocation section number otherwise.
	SymbolIndex uint32

	// Type is the kind of reference being relocated.
	Type RelocationType

	// External reports whether SymbolIndex names an external symbol rather
	// than a section.
	External bool
}

func (r *Relocation) String() string {
	target := RelocationSectionName(r.SymbolIndex)
	if r.External {
		target = fmt.Sprintf("extern(%d)", r.SymbolIndex)
	}
	return fmt.Sprintf("0x%08X %-8s %s", r.Address, r.Type, target)
}

// readRelocations parses NumRelocations entries starting at RelocationsOffset.
func readRelocations(r io.ReaderAt, bo binary.ByteOrder, h SectionHeader) ([]*Relocation, error) {
	if h.NumRelocations == 0 || h.RelocationsOffset == 0 {
		return nil, nil
	}
	buf := make([]byte, RelocationSize*int(h.NumRelocations))
	if _, err := r.ReadAt(buf, int64(h.RelocationsOffset)); err != nil {
		return nil, err
	}
	relocs := make([]*Relocation, 0, h.NumRelocations)
	for i := 0; i < len(buf); i += RelocationSize {
		e := buf[i : i+RelocationSize]
		rel := &Relocation{Address: bo.Uint32(e)}
		b := e[4:8]
		// The trailing byte packs the bitfields
		//
		//	unsigned r_symndx : 24, r_reserved : 3, r_type : 4, r_extern : 1;
		//
		// so its layout follows the byte order of the host that wrote the
		// file. Little-endian matches RELOC_BITS3_*_LITTLE in binutils
		// include/coff/mips.h.
		//
		// Big-endian does not: binutils expects r_reserved to be 2 bits wide
		// (type mask 0x3C, extern mask 0x02), but the Net Yaroze libps.a
		// objects use the 3-bit r_reserved above, putting type at 0x3E and
		// extern at 0x01. Decoding those files the binutils way silently
		// halves every relocation type.
		switch bo {
		case binary.LittleEndian:
			rel.SymbolIndex = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
			rel.Type = RelocationType((b[3] & 0x78) >> 3)
			rel.External = b[3]&0x80 != 0
		default:
			rel.SymbolIndex = uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
			rel.Type = RelocationType((b[3] & 0x3e) >> 1)
			rel.External = b[3]&0x01 != 0
		}
		relocs = append(relocs, rel)
	}
	return relocs, nil
}

// Target returns a human readable name for what a relocation refers to,
// resolving external symbol indices against the file's external symbol table.
func (f *File) Target(r *Relocation) string {
	if !r.External {
		return RelocationSectionName(r.SymbolIndex)
	}
	if int(r.SymbolIndex) < len(f.ExternalSymbols) {
		return f.ExternalSymbols[r.SymbolIndex].Name
	}
	return fmt.Sprintf("extern(%d)", r.SymbolIndex)
}
