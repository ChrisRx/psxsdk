package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"

	"go.chrisrx.dev/psx/format/ecoff"
)

// AsmOptions controls assembly generation.
type AsmOptions struct {
	// Source names the object the assembly was generated from. It is only
	// used in the output header comment.
	Source string

	// Annotate adds the original instruction encoding as a trailing comment
	// on every relocated instruction.
	Annotate bool
}

// sectionClasses maps an ECOFF storage class to the section a symbol with that
// class is defined in.
var sectionClasses = map[ecoff.StorageClass]string{
	ecoff.SC_TEXT:   ecoff.S_TEXT,
	ecoff.SC_DATA:   ecoff.S_DATA,
	ecoff.SC_BSS:    ecoff.S_BSS,
	ecoff.SC_SDATA:  ecoff.S_SDATA,
	ecoff.SC_SBSS:   ecoff.S_SBSS,
	ecoff.SC_RDATA:  ecoff.S_RDATA,
	ecoff.SC_INIT:   ecoff.S_INIT,
	ecoff.SC_FINI:   ecoff.S_FINI,
	ecoff.SC_RCONST: ecoff.S_RCONST,
}

// elfSections maps ECOFF section names onto the directives used to open the
// equivalent section in an ELF object.
var elfSections = map[string]string{
	ecoff.S_TEXT:   "\t.text",
	ecoff.S_DATA:   "\t.data",
	ecoff.S_BSS:    "\t.section .bss,\"aw\",@nobits",
	ecoff.S_SDATA:  "\t.section .sdata,\"aw\",@progbits",
	ecoff.S_SBSS:   "\t.section .sbss,\"aw\",@nobits",
	ecoff.S_RDATA:  "\t.section .rodata,\"a\",@progbits",
	ecoff.S_RCONST: "\t.section .rodata,\"a\",@progbits",
	ecoff.S_LIT4:   "\t.section .rodata,\"a\",@progbits",
	ecoff.S_LIT8:   "\t.section .rodata,\"a\",@progbits",
	ecoff.S_INIT:   "\t.section .init,\"ax\",@progbits",
	ecoff.S_FINI:   "\t.section .fini,\"ax\",@progbits",
}

// regNames holds the o32 ABI register names, indexed by register number.
var regNames = [32]string{
	"$zero", "$at", "$v0", "$v1", "$a0", "$a1", "$a2", "$a3",
	"$t0", "$t1", "$t2", "$t3", "$t4", "$t5", "$t6", "$t7",
	"$s0", "$s1", "$s2", "$s3", "$s4", "$s5", "$s6", "$s7",
	"$t8", "$t9", "$k0", "$k1", "$gp", "$sp", "$fp", "$ra",
}

// immOpcodes names the I-type opcodes that can carry a REFHI or REFLO
// relocation. The bool records whether the instruction uses base+offset
// addressing, which changes how the relocated operand is spelled.
var immOpcodes = map[uint32]struct {
	name   string
	memory bool
}{
	0x08: {"addi", false},
	0x09: {"addiu", false},
	0x0a: {"slti", false},
	0x0b: {"sltiu", false},
	0x0c: {"andi", false},
	0x0d: {"ori", false},
	0x0e: {"xori", false},
	0x0f: {"lui", false},
	0x20: {"lb", true},
	0x21: {"lh", true},
	0x22: {"lwl", true},
	0x23: {"lw", true},
	0x24: {"lbu", true},
	0x25: {"lhu", true},
	0x26: {"lwr", true},
	0x28: {"sb", true},
	0x29: {"sh", true},
	0x2a: {"swl", true},
	0x2b: {"sw", true},
	0x2e: {"swr", true},
}

// markerSymbols are symbols GCC 2.x emitted to record how a file was
// compiled. They sit at offset 0 of .text and are not labels for anything.
var markerSymbols = map[string]bool{
	"gcc2_compiled.":   true,
	"__gnu_compiled_c": true,
}

// A labelSet records the names defined at each offset of a section.
type labelSet struct {
	names  map[uint32][]string
	global map[string]bool
}

func (l *labelSet) add(off uint32, name string) {
	for _, n := range l.names[off] {
		if n == name {
			return
		}
	}
	l.names[off] = append(l.names[off], name)
}

// emitName returns the name a label is written out under.
//
// Labels that are not global are given a .L prefix so that the assembler
// leaves them out of the symbol table entirely. That is not only tidiness: on
// MIPS, gas emits symbols in the order it encounters them without partitioning
// locals from globals, so an object mixing named local labels with references
// to undefined externals ends up with locals positioned after the symbol
// table's sh_info boundary. That violates the ELF requirement that all local
// symbols precede the globals. GNU ld tolerates it, but lld rejects the object
// outright with "symbol has invalid binding: 0". Keeping internal labels out of
// the table sidesteps it. The same construct assembled for x86 is ordered
// correctly, so this is specific to the MIPS target.
func (l *labelSet) emitName(name string) string {
	if l.global[name] {
		return name
	}
	return ".L" + name
}

// sectionBase returns the local label used to refer to the start of a section.
func sectionBase(name string) string {
	return ".Lsect" + strings.ReplaceAll(name, ".", "_")
}

// asmWriter renders a single ECOFF object as assembly.
type asmWriter struct {
	f    *ecoff.File
	opts AsmOptions
	bo   binary.ByteOrder

	// labels maps an ECOFF section name to the labels defined within it.
	labels map[string]*labelSet

	// addends holds the resolved addend for each relocation in the section
	// currently being written.
	addends map[*ecoff.Relocation]uint32

	// bases maps an ECOFF section name to the virtual address the object
	// assigns it. Symbol values and section-relative relocation addends are
	// addresses in that object-wide space rather than section offsets, so
	// this is what converts between the two.
	bases map[string]uint32
}

// offsetIn converts an object-space address to an offset within a section.
func (a *asmWriter) offsetIn(sect string, addr uint32) uint32 {
	return addr - a.bases[sect]
}

// WriteAsm writes GNU as source for an ECOFF object to w.
//
// Section contents are emitted verbatim as .word directives, except where a
// relocation applies. Relocated storage units are respelled symbolically so
// that the assembler and linker recreate the original relocation rather than
// baking in the placeholder value the ECOFF object carries. Recovering those
// references is the whole point: a plain disassembly of a relocatable object
// renders every external call as "jal 0" and every symbol address as 0.
func WriteAsm(w io.Writer, f *ecoff.File, opts AsmOptions) error {
	a := &asmWriter{
		f:      f,
		opts:   opts,
		bo:     f.TargetByteOrder(),
		labels: make(map[string]*labelSet),
		bases:  make(map[string]uint32),
	}
	for _, s := range f.Sections {
		a.bases[s.SectionName()] = s.VirtualAddress
	}
	a.collectLabels()
	return a.write(w)
}

// collectLabels builds the per-section label map from both symbol tables.
//
// The two tables carry different information. Internal labels (branch targets,
// static data) are described properly only by the local table, while the
// external table is what records which names are global. Labels also leak into
// the external table with a nil type and storage class, so placement comes
// from the local table wherever it is available.
func (a *asmWriter) collectLabels() {
	for _, s := range a.f.Sections {
		a.labels[s.SectionName()] = &labelSet{
			names:  make(map[uint32][]string),
			global: make(map[string]bool),
		}
	}
	place := func(sym *ecoff.Symbol, global bool) {
		sect, ok := sectionClasses[ecoff.StorageClass(sym.StorageClass)]
		if !ok {
			return
		}
		ls, ok := a.labels[sect]
		if !ok {
			return
		}
		if sym.Name == "" || strings.HasPrefix(sym.Name, "$") || markerSymbols[sym.Name] {
			// Compiler-internal labels such as $LC0 and $L13 are referenced
			// only through relocations, which are resolved against the
			// section base instead.
			return
		}
		ls.add(a.offsetIn(sect, sym.Value), sym.Name)
		if global {
			ls.global[sym.Name] = true
		}
	}
	for _, s := range a.f.LocalSymbols {
		switch s.Type {
		case ecoff.ST_PROC, ecoff.ST_STATIC_PROC, ecoff.ST_LABEL, ecoff.ST_STATIC:
			place(s, false)
		}
	}
	for _, s := range a.f.ExternalSymbols {
		if sc := ecoff.StorageClass(s.StorageClass); sc == ecoff.SC_UNDEFINED || sc == ecoff.SC_ABS {
			continue
		}
		place(&s.Symbol, true)
	}
}

func (a *asmWriter) write(w io.Writer) error {
	src := a.opts.Source
	if src == "" {
		src = "an ECOFF object"
	}
	fmt.Fprintf(w, "# Generated by ecoff2asm from %s. Do not edit.\n", src)
	fmt.Fprintf(w, "#\n# %s\n\n", a.f)
	fmt.Fprintf(w, "\t.set noreorder\n\t.set noat\n\n")

	// Undefined externals need no declaration in gas, but listing them
	// documents what the object expects the link to supply.
	var undef []string
	for _, s := range a.f.ExternalSymbols {
		if ecoff.StorageClass(s.StorageClass) == ecoff.SC_UNDEFINED && s.Name != "" && !strings.HasPrefix(s.Name, "$") {
			undef = append(undef, s.Name)
		}
	}
	if len(undef) > 0 {
		slices.Sort(undef)
		fmt.Fprintf(w, "# Undefined references: %s\n\n", strings.Join(undef, ", "))
	}

	if err := a.writeAbsoluteSymbols(w); err != nil {
		return err
	}

	for _, s := range a.f.Sections {
		if err := a.writeSection(w, s); err != nil {
			return err
		}
	}
	return nil
}

// writeAbsoluteSymbols emits definitions for symbols that live at a fixed
// address rather than in any section.
//
// It has to be an object rather than a link script. A script assignment is
// applied only after the linker has scanned the input archives, so a reference
// to, say, __extendsfdf2 pulls libgcc's own copy into the image even though
// the address is about to be overridden. Linking libgcc is otherwise
// desirable, since it is the only source of the 64-bit integer and bit
// counting helpers an R3000 has no instructions for, so the resident
// definitions have to be in place before the scan happens. That is what the
// original symusr.o achieved by being an archive member.
//
// The symbols are absolute, as they were in symusr.o, which takes lld: GNU ld
// does not apply the MIPS PC-region masking to an absolute symbol, so a jal to
// one fails with "relocation truncated to fit: R_MIPS_26". They used to be
// offsets into an empty section that the link script placed at the resident
// library, but that section counted as allocated, so size(1) reported the
// library's 420KB as part of every program.
func (a *asmWriter) writeAbsoluteSymbols(w io.Writer) error {
	syms := AbsoluteSymbols(a.f)
	if len(syms) == 0 {
		return nil
	}

	fmt.Fprintf(w, "# %d absolute symbols: entry points in the resident library.\n", len(syms))
	fmt.Fprintf(w, "#\n")
	fmt.Fprintf(w, "# The definitions are weak so that a program bringing its own copy of\n")
	fmt.Fprintf(w, "# one of them wins instead of colliding. That is what the resident\n")
	fmt.Fprintf(w, "# library amounts to: a default C library, which a runtime with its own\n")
	fmt.Fprintf(w, "# allocator is entitled to replace. A weak definition is still a\n")
	fmt.Fprintf(w, "# definition, so it does not reopen the question of archive members\n")
	fmt.Fprintf(w, "# being pulled in to satisfy these names.\n\n")

	for _, s := range syms {
		fmt.Fprintf(w, "\t.weak %s\n\t.set %s, 0x%08X\n", s.Name, s.Name, s.Address)
	}
	fmt.Fprint(w, "\n")
	return nil
}

func (a *asmWriter) writeSection(w io.Writer, s *ecoff.Section) error {
	name := s.SectionName()
	ls := a.labels[name]
	// A section with no contents and no labels contributes nothing. Zero-sized
	// .bss still matters if symbols are defined in it.
	if s.Size == 0 && (ls == nil || len(ls.names) == 0) {
		return nil
	}
	directive, ok := elfSections[name]
	if !ok {
		return fmt.Errorf("no ELF equivalent for section %q", name)
	}

	fmt.Fprintf(w, "%s\n\t.align 2\n", directive)
	fmt.Fprintf(w, "%s:\n", sectionBase(name))
	for _, n := range sortedNames(ls) {
		if ls.global[n] {
			fmt.Fprintf(w, "\t.globl %s\n", n)
		}
	}

	// .bss and .sbss have no file contents; reserve space instead.
	if s.Offset == 0 || name == ecoff.S_BSS || name == ecoff.S_SBSS {
		a.writeLabels(w, ls, 0)
		if n := bssSize(s); n > 0 {
			fmt.Fprintf(w, "\t.space %d\n", n)
		}
		fmt.Fprint(w, "\n")
		return nil
	}

	data, err := s.Data()
	if err != nil {
		return fmt.Errorf("reading section %s: %v", name, err)
	}
	addends, err := a.computeAddends(s.Relocations, data)
	if err != nil {
		return fmt.Errorf("section %s: %v", name, err)
	}
	a.addends = addends
	relocs := indexRelocations(s.Relocations)

	if name == ecoff.S_TEXT {
		err = a.writeText(w, name, data, relocs, ls)
	} else {
		err = a.writeData(w, name, data, relocs, ls)
	}
	if err != nil {
		return err
	}
	fmt.Fprint(w, "\n")
	return nil
}

// writeText emits one line per instruction word.
func (a *asmWriter) writeText(w io.Writer, sect string, data []byte, relocs map[uint32][]*ecoff.Relocation, ls *labelSet) error {
	for off := uint32(0); int(off) < len(data); off += 4 {
		a.writeLabels(w, ls, off)
		word := a.bo.Uint32(data[off:])
		rels := relocs[off]
		if len(rels) == 0 {
			fmt.Fprintf(w, "\t.word 0x%08X\n", word)
			continue
		}
		if len(rels) > 1 {
			return fmt.Errorf("%s+0x%X: %d relocations on one word", sect, off, len(rels))
		}
		line, err := a.relocatedInstruction(word, rels[0])
		if err != nil {
			return fmt.Errorf("%s+0x%X: %v", sect, off, err)
		}
		if a.opts.Annotate {
			fmt.Fprintf(w, "\t%-40s # 0x%08X %s\n", line, word, rels[0].Type)
		} else {
			fmt.Fprintf(w, "\t%s\n", line)
		}
	}
	return nil
}

// writeData emits .word directives, collapsing long runs of zeroes.
func (a *asmWriter) writeData(w io.Writer, sect string, data []byte, relocs map[uint32][]*ecoff.Relocation, ls *labelSet) error {
	const minZeroRun = 16
	for off := uint32(0); int(off) < len(data); {
		a.writeLabels(w, ls, off)
		if rels := relocs[off]; len(rels) > 0 {
			if len(rels) > 1 {
				return fmt.Errorf("%s+0x%X: %d relocations on one word", sect, off, len(rels))
			}
			line, size, err := a.relocatedDatum(rels[0])
			if err != nil {
				return fmt.Errorf("%s+0x%X: %v", sect, off, err)
			}
			fmt.Fprintf(w, "\t%s\n", line)
			off += size
			continue
		}
		// Collapse a zero run, but never across a label or relocation.
		if run := zeroRun(data, off, relocs, ls); run >= minZeroRun {
			fmt.Fprintf(w, "\t.space %d\n", run)
			off += run
			continue
		}
		if int(off)+4 <= len(data) {
			fmt.Fprintf(w, "\t.word 0x%08X\n", a.bo.Uint32(data[off:]))
			off += 4
			continue
		}
		fmt.Fprintf(w, "\t.byte 0x%02X\n", data[off])
		off++
	}
	return nil
}

// relocatedInstruction respells one relocated instruction word symbolically.
func (a *asmWriter) relocatedInstruction(word uint32, r *ecoff.Relocation) (string, error) {
	expr := a.target(r, a.addends[r])
	switch r.Type {
	case ecoff.R_JMPADDR:
		switch op := word >> 26; op {
		case 0x02:
			return "j " + expr, nil
		case 0x03:
			return "jal " + expr, nil
		default:
			return "", fmt.Errorf("JMPADDR on opcode 0x%02X", op)
		}
	case ecoff.R_REFHI:
		return a.immInstruction(word, fmt.Sprintf("%%hi(%s)", expr))
	case ecoff.R_REFLO:
		return a.immInstruction(word, fmt.Sprintf("%%lo(%s)", expr))
	case ecoff.R_GPREL, ecoff.R_LITERAL:
		return a.immInstruction(word, fmt.Sprintf("%%gp_rel(%s)", expr))
	case ecoff.R_REFWORD:
		return fmt.Sprintf(".word %s", expr), nil
	}
	return "", fmt.Errorf("unexpected relocation %s in .text", r.Type)
}

// immInstruction formats an I-type instruction with a symbolic immediate.
func (a *asmWriter) immInstruction(word uint32, imm string) (string, error) {
	op := word >> 26
	info, ok := immOpcodes[op]
	if !ok {
		return "", fmt.Errorf("relocated immediate on opcode 0x%02X", op)
	}
	rs := regNames[(word>>21)&0x1f]
	rt := regNames[(word>>16)&0x1f]
	switch {
	case info.memory:
		return fmt.Sprintf("%s %s, %s(%s)", info.name, rt, imm, rs), nil
	case info.name == "lui":
		return fmt.Sprintf("lui %s, %s", rt, imm), nil
	default:
		return fmt.Sprintf("%s %s, %s, %s", info.name, rt, rs, imm), nil
	}
}

// relocatedDatum respells one relocated data word symbolically, returning the
// number of bytes consumed.
func (a *asmWriter) relocatedDatum(r *ecoff.Relocation) (string, uint32, error) {
	expr := a.target(r, a.addends[r])
	switch r.Type {
	case ecoff.R_REFWORD:
		return fmt.Sprintf(".word %s", expr), 4, nil
	case ecoff.R_REFHALF:
		return fmt.Sprintf(".short %s", expr), 2, nil
	}
	return "", 0, fmt.Errorf("unexpected relocation %s in data", r.Type)
}

// computeAddends resolves the addend carried by every relocation in a section.
//
// ECOFF is a REL format: addends live in the relocated storage unit rather
// than in the relocation entry. A 32-bit reference is therefore split across a
// REFHI/REFLO pair, with the high instruction holding bits 16-31 and the low
// instruction a sign-extended bits 0-15, so neither half can be interpreted on
// its own. Such pairs are emitted as consecutive relocation table entries.
func (a *asmWriter) computeAddends(relocs []*ecoff.Relocation, data []byte) (map[*ecoff.Relocation]uint32, error) {
	imm := func(off uint32) (uint32, error) {
		if int(off)+4 > len(data) {
			return 0, fmt.Errorf("relocation at 0x%X past end of section", off)
		}
		return a.bo.Uint32(data[off:]) & 0xffff, nil
	}
	addends := make(map[*ecoff.Relocation]uint32, len(relocs))
	for i := range relocs {
		r := relocs[i]
		switch r.Type {
		case ecoff.R_REFHI:
			if i+1 >= len(relocs) || relocs[i+1].Type != ecoff.R_REFLO {
				return nil, fmt.Errorf("REFHI at 0x%X has no following REFLO", r.Address)
			}
			lo := relocs[i+1]
			hiv, err := imm(r.Address)
			if err != nil {
				return nil, err
			}
			lov, err := imm(lo.Address)
			if err != nil {
				return nil, err
			}
			v := uint32(int32(hiv<<16) + int32(int16(lov)))
			addends[r], addends[lo] = v, v
			i++
		case ecoff.R_REFLO, ecoff.R_GPREL, ecoff.R_LITERAL:
			// A REFLO reached here has no preceding REFHI, so it carries the
			// whole addend itself.
			v, err := imm(r.Address)
			if err != nil {
				return nil, err
			}
			addends[r] = uint32(int32(int16(v)))
		case ecoff.R_JMPADDR:
			if int(r.Address)+4 > len(data) {
				return nil, fmt.Errorf("relocation at 0x%X past end of section", r.Address)
			}
			addends[r] = (a.bo.Uint32(data[r.Address:]) & 0x03ffffff) << 2
		case ecoff.R_REFWORD:
			if int(r.Address)+4 > len(data) {
				return nil, fmt.Errorf("relocation at 0x%X past end of section", r.Address)
			}
			addends[r] = a.bo.Uint32(data[r.Address:])
		case ecoff.R_REFHALF:
			if int(r.Address)+2 > len(data) {
				return nil, fmt.Errorf("relocation at 0x%X past end of section", r.Address)
			}
			addends[r] = uint32(a.bo.Uint16(data[r.Address:]))
		}
	}
	return addends, nil
}

// target renders what a relocation refers to, with its addend applied.
func (a *asmWriter) target(r *ecoff.Relocation, addend uint32) string {
	if r.External {
		name := a.f.Target(r)
		if addend == 0 {
			return name
		}
		return fmt.Sprintf("%s+0x%X", name, addend)
	}
	if r.SymbolIndex == ecoff.RS_ABS {
		return fmt.Sprintf("0x%X", addend)
	}
	// A section-relative addend is an address in the object's own address
	// space, so it has to be rebased onto the target section.
	sect := ecoff.RelocationSectionName(r.SymbolIndex)
	off := a.offsetIn(sect, addend)
	// Prefer a real label at the target offset over base+offset arithmetic;
	// it keeps internal branches and data references readable.
	if ls, ok := a.labels[sect]; ok {
		if names := ls.names[off]; len(names) > 0 {
			return ls.emitName(names[0])
		}
	}
	if off == 0 {
		return sectionBase(sect)
	}
	return fmt.Sprintf("%s+0x%X", sectionBase(sect), off)
}

func (a *asmWriter) writeLabels(w io.Writer, ls *labelSet, off uint32) {
	if ls == nil {
		return
	}
	for _, n := range ls.names[off] {
		fmt.Fprintf(w, "%s:\n", ls.emitName(n))
	}
}

func sortedNames(ls *labelSet) []string {
	if ls == nil {
		return nil
	}
	var names []string
	for _, ns := range ls.names {
		names = append(names, ns...)
	}
	slices.Sort(names)
	return names
}

// bssSize returns the size of a .bss style section. Such sections have no file
// contents, so Size is cleared during parsing and the header size is what
// records how much memory to reserve.
func bssSize(s *ecoff.Section) int64 {
	return int64(s.MemSize)
}

// indexRelocations groups relocations by the offset they apply to.
func indexRelocations(relocs []*ecoff.Relocation) map[uint32][]*ecoff.Relocation {
	m := make(map[uint32][]*ecoff.Relocation, len(relocs))
	for _, r := range relocs {
		m[r.Address] = append(m[r.Address], r)
	}
	return m
}

// zeroRun returns the length of the run of zero bytes starting at off that is
// uninterrupted by a label or relocation.
func zeroRun(data []byte, off uint32, relocs map[uint32][]*ecoff.Relocation, ls *labelSet) uint32 {
	var n uint32
	for int(off+n) < len(data) {
		at := off + n
		if data[at] != 0 {
			break
		}
		if n > 0 {
			if len(relocs[at]) > 0 {
				break
			}
			if ls != nil && len(ls.names[at]) > 0 {
				break
			}
		}
		n++
	}
	return n
}
