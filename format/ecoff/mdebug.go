package ecoff

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Debug is a file's symbolic information grouped by the source file it came
// from: what the MIPS assembler and linker keep beside the flat symbol lists
// ("mdebug"), and what debuggers read. Each source file has its own local
// symbols, strings, procedures and line numbers. The layouts are binutils'
// include/coff/mips.h, and the line numbers are decoded as GDB's
// mdebugread.c does.
type Debug struct {
	Files []*SourceFile
}

// A SourceFile is one source file's part of the symbolic information: a file
// descriptor and what it owns.
type SourceFile struct {
	// Name is the file's name as the compiler gave it, often without a
	// directory.
	Name string

	// Address is where the file's code starts.
	Address uint32

	// Language is the file descriptor's language code: 1 is C.
	Language uint8

	// Symbols are the file's local symbols, in order, their names read
	// from the file's own strings. Stabs among them have their type in
	// Stab (see Symbol.Stab).
	Symbols []*Symbol

	// Procedures are the file's procedures, in the order of their
	// descriptors.
	Procedures []*Procedure
}

// A Procedure is a procedure descriptor: where a function starts, how its
// frame is laid out, and its line numbers.
type Procedure struct {
	// Name and Address are those of the procedure's symbol.
	Name    string
	Address uint32

	// Static reports whether the procedure is local to its file.
	Static bool

	// RegisterMask has a bit set for each general register the procedure
	// saves, and RegisterOffset is where the highest of them is saved,
	// relative to the virtual frame pointer (the stack pointer on entry).
	// The others are saved below it, a word each, in register order.
	RegisterMask   uint32
	RegisterOffset int32

	// FrameOffset is the frame's size, FrameRegister the register the
	// frame is addressed from (29, the stack pointer, for GCC's code), and
	// PCRegister the register holding the return address (31).
	FrameOffset   int32
	FrameRegister uint16
	PCRegister    uint16

	// LineLow and LineHigh are the procedure's first and last lines, -1
	// if it has none.
	LineLow, LineHigh int32

	// Lines map the procedure's instructions to source lines, in the order
	// encoded: the address of each run of instructions and its line.
	Lines []Line
}

// A Line is the source line of the instructions from Address on.
type Line struct {
	Address uint32
	Line    int32
}

// IsStab reports whether s holds a stab rather than an ECOFF symbol: GCC
// writes its stabs debugging information as local symbols with this mark
// in their index (binutils' ECOFF_IS_STAB).
func (s *Symbol) IsStab() bool { return s.SectionIndex&0xfff00 == stabMark }

// Stab is the stab type of a symbol IsStab reports true for, such as 0x24
// (N_FUN) or 0x64 (N_SO).
func (s *Symbol) Stab() uint8 { return uint8(s.SectionIndex - stabMark) }

const stabMark = 0x8f300

// External record sizes, from binutils' include/coff/mips.h.
const (
	fdrSize = 72
	pdrSize = 52
	symSize = 12
)

const (
	indexNil = 0xfffff
	ilineNil = -1
)

// Debug reads the file's symbolic information. It returns nil and no error
// for a file without any.
func (f *File) Debug() (*Debug, error) {
	h := f.SymbolicHeader
	if h == nil || h.FileDescriptorLength == 0 {
		return nil, nil
	}
	read := func(off, n int32) ([]byte, error) {
		if n <= 0 {
			return nil, nil
		}
		b := make([]byte, n)
		if _, err := f.r.ReadAt(b, int64(off)); err != nil && err != io.EOF {
			return nil, err
		}
		return b, nil
	}
	fdrs, err := read(h.FileDescriptorOffset, h.FileDescriptorLength*fdrSize)
	if err != nil {
		return nil, fmt.Errorf("reading the file descriptors: %w", err)
	}
	pdrs, err := read(h.ProceduresOffset, h.ProceduresCount*pdrSize)
	if err != nil {
		return nil, fmt.Errorf("reading the procedure descriptors: %w", err)
	}
	syms, err := read(h.LocalSymbolsOffset, h.LocalSymbolsCount*symSize)
	if err != nil {
		return nil, fmt.Errorf("reading the local symbols: %w", err)
	}
	strs, err := read(h.LocalStringsOffset, h.LocalStringsLength)
	if err != nil {
		return nil, fmt.Errorf("reading the local strings: %w", err)
	}
	lines, err := read(h.LineNumbersOffset, h.LineNumbersLength)
	if err != nil {
		return nil, fmt.Errorf("reading the line numbers: %w", err)
	}

	bo := f.byteOrder
	d := new(Debug)
	for i := range int(h.FileDescriptorLength) {
		r := fdrs[i*fdrSize : (i+1)*fdrSize]
		fd := fileDescriptor{
			adr:          bo.Uint32(r[0:]),
			rss:          int32(bo.Uint32(r[4:])),
			issBase:      int32(bo.Uint32(r[8:])),
			cbSs:         int32(bo.Uint32(r[12:])),
			isymBase:     int32(bo.Uint32(r[16:])),
			csym:         int32(bo.Uint32(r[20:])),
			ipdFirst:     int32(bo.Uint16(r[40:])),
			cpd:          int32(bo.Uint16(r[42:])),
			bits1:        r[60],
			cbLineOffset: int32(bo.Uint32(r[64:])),
			cbLine:       int32(bo.Uint32(r[68:])),
		}
		// The language is the low five bits of the first bit field byte
		// little-endian, the high five big-endian.
		sf := &SourceFile{Address: fd.adr}
		if bo == binary.LittleEndian {
			sf.Language = fd.bits1 & 0x1f
		} else {
			sf.Language = fd.bits1 >> 3
		}
		if fd.issBase < 0 || fd.issBase > int32(len(strs)) {
			return nil, fmt.Errorf("file descriptor %d: strings at %d, past the table's %d bytes", i, fd.issBase, len(strs))
		}
		fstrs := strs[fd.issBase:min(int(fd.issBase+fd.cbSs), len(strs))]
		sf.Name, _ = getString(fstrs, int(fd.rss))

		// The file's local symbols.
		for j := range int(fd.csym) {
			k := int(fd.isymBase) + j
			if k < 0 || (k+1)*symSize > len(syms) {
				return nil, fmt.Errorf("%s: symbol %d is past the table", sf.Name, k)
			}
			sym := f.decodeSymbol(syms[k*symSize : (k+1)*symSize])
			sym.Name, _ = getString(fstrs, int(sym.Index))
			sf.Symbols = append(sf.Symbols, sym)
		}

		// The file's procedures, and their lines, which are encoded one
		// procedure after another from the file's start in the table.
		type pdr struct {
			isym, iline, lnLow, lnHigh, cbLineOffset int32
			p                                        *Procedure
		}
		var procs []pdr
		for j := range int(fd.cpd) {
			k := int(fd.ipdFirst) + j
			if (k+1)*pdrSize > len(pdrs) {
				return nil, fmt.Errorf("%s: procedure %d is past the table", sf.Name, k)
			}
			r := pdrs[k*pdrSize : (k+1)*pdrSize]
			pd := pdr{
				isym:         int32(bo.Uint32(r[4:])),
				iline:        int32(bo.Uint32(r[8:])),
				lnLow:        int32(bo.Uint32(r[40:])),
				lnHigh:       int32(bo.Uint32(r[44:])),
				cbLineOffset: int32(bo.Uint32(r[48:])),
			}
			p := &Procedure{
				Address:        bo.Uint32(r[0:]),
				RegisterMask:   bo.Uint32(r[12:]),
				RegisterOffset: int32(bo.Uint32(r[16:])),
				FrameOffset:    int32(bo.Uint32(r[32:])),
				FrameRegister:  bo.Uint16(r[36:]),
				PCRegister:     bo.Uint16(r[38:]),
				LineLow:        pd.lnLow,
				LineHigh:       pd.lnHigh,
			}
			// The procedure's symbol has its name and, in a linked
			// program, its address.
			if pd.isym != indexNil && int(pd.isym) < len(sf.Symbols) {
				s := sf.Symbols[pd.isym]
				p.Name, p.Address = s.Name, s.Value
				p.Static = s.Type == ST_STATIC_PROC
			}
			pd.p = p
			procs = append(procs, pd)
			sf.Procedures = append(sf.Procedures, p)
		}
		if fd.cbLine > 0 {
			base := int(fd.cbLineOffset)
			for j, pd := range procs {
				if pd.iline == ilineNil || pd.lnLow == -1 || pd.lnHigh == -1 {
					continue
				}
				end := int(fd.cbLine)
				if j+1 < len(procs) {
					end = int(procs[j+1].cbLineOffset)
				}
				start, stop := base+int(pd.cbLineOffset), base+end
				if start < 0 || stop > len(lines) || start > stop {
					return nil, fmt.Errorf("%s: %s: line numbers %d to %d are past the table's %d bytes", sf.Name, pd.p.Name, start, stop, len(lines))
				}
				pd.p.Lines = decodeLines(lines[start:stop], pd.p.Address, pd.lnLow)
			}
		}
		d.Files = append(d.Files, sf)
	}
	return d, nil
}

type fileDescriptor struct {
	adr                                uint32
	rss, issBase, cbSs, isymBase, csym int32
	ipdFirst, cpd                      int32
	bits1                              byte
	cbLineOffset, cbLine               int32
}

// decodeSymbol reads a local symbol record, as NewFile does.
func (f *File) decodeSymbol(r []byte) *Symbol {
	bo := f.byteOrder
	bits := bo.Uint32(r[8:])
	s := &Symbol{Index: bo.Uint32(r[0:]), Value: bo.Uint32(r[4:])}
	if bo == binary.LittleEndian {
		s.Type = SymbolType(extractBits(bits, 0, 6))
		s.StorageClass = extractBits(bits, 6, 5)
		s.SectionIndex = extractBits(bits, 12, 20)
	} else {
		s.Type = SymbolType(extractBits(bits, 26, 6))
		s.StorageClass = extractBits(bits, 21, 5)
		s.SectionIndex = extractBits(bits, 0, 20)
	}
	return s
}

// decodeLines decodes a procedure's compressed line numbers. Each byte holds
// a signed line delta in its high four bits and the number of instructions
// less one in its low four; a delta of -8 means the delta is in the next two
// bytes instead, big-endian, whatever the file's byte order. The first delta
// is from the procedure's first line.
func decodeLines(b []byte, addr uint32, line int32) []Line {
	var out []Line
	for i := 0; i < len(b); {
		count := int32(b[i] & 0x0f)
		delta := int32(b[i] >> 4)
		i++
		if delta >= 8 {
			delta -= 16
		}
		if delta == -8 {
			if i+2 > len(b) {
				break
			}
			delta = int32(int16(uint16(b[i])<<8 | uint16(b[i+1])))
			i += 2
		}
		line += delta
		out = append(out, Line{Address: addr, Line: line})
		addr += uint32(count+1) * 4
	}
	return out
}
