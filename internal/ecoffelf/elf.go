package ecoffelf

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"sort"
)

// elfFile builds a 32-bit little-endian MIPS ELF: sections, a symbol table
// and program headers for the loaded sections.
type elfFile struct {
	entry    uint32
	flags    uint32
	sections []*section
	symbols  []symbol
}

type section struct {
	name  string
	typ   elf.SectionType
	flags elf.SectionFlag
	addr  uint32
	data  []byte // nil for NOBITS
	size  uint32 // the size in memory, for NOBITS
	align uint32
	link  uint32
	info  uint32
	entsz uint32
	index int // set by add
}

type symbol struct {
	name    string
	value   uint32
	size    uint32
	typ     elf.SymType
	bind    elf.SymBind
	section int // a section index, or SHN_ABS
}

// add adds s and returns its section index.
func (f *elfFile) add(s *section) int {
	s.index = len(f.sections) + 1
	f.sections = append(f.sections, s)
	return s.index
}

// bytes lays the file out: the header, the program headers, each section's
// contents, then the section headers.
func (f *elfFile) bytes() []byte {
	le := binary.LittleEndian

	// The symbol table, locals first as ELF requires.
	syms := append([]symbol(nil), f.symbols...)
	sort.SliceStable(syms, func(i, j int) bool {
		return syms[i].bind == elf.STB_LOCAL && syms[j].bind != elf.STB_LOCAL
	})
	var strtab bytes.Buffer
	strtab.WriteByte(0)
	var symtab bytes.Buffer
	symtab.Write(make([]byte, 16))
	firstGlobal := 1
	for i, s := range syms {
		name := uint32(0)
		if s.name != "" {
			name = uint32(strtab.Len())
			strtab.WriteString(s.name)
			strtab.WriteByte(0)
		}
		var e [16]byte
		le.PutUint32(e[0:], name)
		le.PutUint32(e[4:], s.value)
		le.PutUint32(e[8:], s.size)
		e[12] = byte(s.bind)<<4 | byte(s.typ)&0xf
		le.PutUint16(e[14:], uint16(s.section))
		symtab.Write(e[:])
		if s.bind == elf.STB_LOCAL {
			firstGlobal = i + 2
		}
	}
	strIndex := len(f.sections) + 2
	f.add(&section{name: ".symtab", typ: elf.SHT_SYMTAB, data: symtab.Bytes(), align: 4, link: uint32(strIndex), info: uint32(firstGlobal), entsz: 16})
	f.add(&section{name: ".strtab", typ: elf.SHT_STRTAB, data: strtab.Bytes(), align: 1})

	var shstrtab bytes.Buffer
	shstrtab.WriteByte(0)
	names := make([]uint32, len(f.sections)+1)
	for _, s := range f.sections {
		names[s.index] = uint32(shstrtab.Len())
		shstrtab.WriteString(s.name)
		shstrtab.WriteByte(0)
	}
	names = append(names, uint32(shstrtab.Len()))
	shstrtab.WriteString(".shstrtab")
	shstrtab.WriteByte(0)
	f.add(&section{name: ".shstrtab", typ: elf.SHT_STRTAB, data: shstrtab.Bytes(), align: 1})

	// A program header for each loaded section with contents.
	var loads []*section
	for _, s := range f.sections {
		if s.flags&elf.SHF_ALLOC != 0 && s.typ == elf.SHT_PROGBITS {
			loads = append(loads, s)
		}
	}
	const ehsize, phentsize, shentsize = 52, 32, 40
	off := uint32(ehsize + phentsize*len(loads))
	offsets := make([]uint32, len(f.sections)+1)
	for _, s := range f.sections {
		if s.data == nil {
			offsets[s.index] = off
			continue
		}
		if s.align > 1 {
			off = (off + s.align - 1) &^ (s.align - 1)
		}
		offsets[s.index] = off
		off += uint32(len(s.data))
	}
	shoff := (off + 3) &^ 3

	out := make([]byte, shoff+shentsize*uint32(len(f.sections)+1))
	copy(out[0:], []byte{0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS32), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)})
	le.PutUint16(out[16:], uint16(elf.ET_EXEC))
	le.PutUint16(out[18:], uint16(elf.EM_MIPS))
	le.PutUint32(out[20:], uint32(elf.EV_CURRENT))
	le.PutUint32(out[24:], f.entry)
	le.PutUint32(out[28:], ehsize)
	le.PutUint32(out[32:], shoff)
	le.PutUint32(out[36:], f.flags)
	le.PutUint16(out[40:], ehsize)
	le.PutUint16(out[42:], phentsize)
	le.PutUint16(out[44:], uint16(len(loads)))
	le.PutUint16(out[46:], shentsize)
	le.PutUint16(out[48:], uint16(len(f.sections)+1))
	le.PutUint16(out[50:], uint16(len(f.sections)))

	for i, s := range loads {
		p := out[ehsize+phentsize*i:]
		le.PutUint32(p[0:], uint32(elf.PT_LOAD))
		le.PutUint32(p[4:], offsets[s.index])
		le.PutUint32(p[8:], s.addr)
		le.PutUint32(p[12:], s.addr)
		le.PutUint32(p[16:], uint32(len(s.data)))
		le.PutUint32(p[20:], uint32(len(s.data)))
		flags := elf.PF_R
		if s.flags&elf.SHF_WRITE != 0 {
			flags |= elf.PF_W
		}
		if s.flags&elf.SHF_EXECINSTR != 0 {
			flags |= elf.PF_X
		}
		le.PutUint32(p[24:], uint32(flags))
		le.PutUint32(p[28:], max(s.align, 1))
	}
	for _, s := range f.sections {
		copy(out[offsets[s.index]:], s.data)
		h := out[shoff+shentsize*uint32(s.index):]
		size := uint32(len(s.data))
		if s.data == nil {
			size = s.size
		}
		le.PutUint32(h[0:], names[s.index])
		le.PutUint32(h[4:], uint32(s.typ))
		le.PutUint32(h[8:], uint32(s.flags))
		le.PutUint32(h[12:], s.addr)
		le.PutUint32(h[16:], offsets[s.index])
		le.PutUint32(h[20:], size)
		le.PutUint32(h[24:], s.link)
		le.PutUint32(h[28:], s.info)
		le.PutUint32(h[32:], max(s.align, 1))
		le.PutUint32(h[36:], s.entsz)
	}
	return out
}
