package binutils

import (
	"bytes"
	"debug/elf"
	"fmt"
)

// MIPS II trap-on-condition instructions, in the SPECIAL opcode's function
// field. These are the ones LLVM emits that a MIPS I core does not have.
var trapFuncs = map[uint32]string{
	0x30: "tge",
	0x31: "tgeu",
	0x32: "tlt",
	0x33: "tltu",
	0x34: "teq",
	0x36: "tne",
}

// A Trap is a MIPS II trap instruction found in an object.
type Trap struct {
	Section string // the section it is in
	Offset  int    // its offset in the section
	Pos     int    // its offset in the file
	Name    string // its mnemonic: teq, tne, tge, tgeu, tlt or tltu
	Word    uint32 // its encoding
}

// MIPS1Traps finds the MIPS II trap instructions in the executable sections
// of data, an ELF object for MIPS.
//
// LLVM's MIPS I support emits a trap after an integer division to catch
// division by zero. The trap family arrived with MIPS II, so on the R3000A
// the encoding is not an instruction at all, and the console takes a
// reserved-instruction exception and stops. GCC branches around a break
// instead, and clang does with -mno-check-zero-division, which is an LLVM
// backend flag rather than a target feature and so cannot be reached
// through TinyGo. RemoveTraps makes the same trade afterwards: a trap
// becomes a nop, nothing moves, and division by zero has an undefined
// result rather than trapping.
func MIPS1Traps(data []byte) ([]Trap, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if f.Machine != elf.EM_MIPS && f.Machine != elf.EM_MIPS_RS3_LE {
		return nil, fmt.Errorf("machine is %v, not MIPS", f.Machine)
	}
	bo := f.ByteOrder
	var traps []Trap
	for _, s := range f.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_EXECINSTR == 0 {
			continue
		}
		base := int(s.Offset)
		if base+int(s.Size) > len(data) {
			return nil, fmt.Errorf("section %s runs past the end of the file", s.Name)
		}
		for off := 0; off+4 <= int(s.Size); off += 4 {
			word := bo.Uint32(data[base+off:])
			// SPECIAL opcode with a trap function code.
			if word>>26 != 0 {
				continue
			}
			if name, ok := trapFuncs[word&0x3f]; ok {
				traps = append(traps, Trap{Section: s.Name, Offset: off, Pos: base + off, Name: name, Word: word})
			}
		}
	}
	return traps, nil
}

// RemoveTraps replaces each of traps, found in data by MIPS1Traps, with a
// nop, which is all zeros in either byte order.
func RemoveTraps(data []byte, traps []Trap) {
	for _, t := range traps {
		clear(data[t.Pos : t.Pos+4])
	}
}
