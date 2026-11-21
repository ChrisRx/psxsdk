// Package ecoffelf converts an ECOFF Net Yaroze program into an ELF with
// DWARF, so that any current debugger can read its symbols: GDB from 16 on
// crashes on the stabs GCC 2.95.3 writes into ECOFF, and LLDB never read
// them.
//
// The ELF has the program's sections at their addresses, with the same
// contents, so it describes the program the console runs: the program
// itself is not changed, only the file the debugger reads. It has an ELF
// symbol table, with each function's size, and the resident library's
// entry points as functions in a section of their own, so that a debugger
// breaks on them by name. Its DWARF has a compile unit for each source file
// with a function for each procedure, the lines GCC recorded, and the frame
// each function's prologue sets up, from which debuggers unwind.
//
// The stabs of a file compiled with -g give its types, its global and
// static variables, and each function's result, parameters and locals, with
// the blocks they are in. A file compiled without -g has its functions,
// their sizes and frames, and nothing more.
package ecoffelf

import (
	"cmp"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.chrisrx.dev/psx/format/ecoff"
)

// Resident is the range the Net Yaroze's resident library is loaded at, in
// which a program's absolute symbols are the library's entry points.
const (
	residentStart = 0x80010000
	residentEnd   = 0x80090000
)

// stab types used here, from binutils' include/aout/stab.def.
const (
	stabSO = 0x64 // source file name, or its directory
)

// Convert returns the ELF with DWARF for the ECOFF program f.
func Convert(f *ecoff.File) ([]byte, error) {
	if f.TargetByteOrder() != binary.LittleEndian {
		return nil, fmt.Errorf("the program is big-endian: only the PlayStation's little-endian MIPS is converted")
	}
	out := &elfFile{entry: f.Entry, flags: 0x1001} // noreorder, o32, MIPS I

	// The program's sections, where it loads them.
	var text []byte
	var textAddr uint32
	bySection := map[string]int{}
	for _, s := range f.Sections {
		name := s.SectionName()
		es := &section{name: name, addr: s.VirtualAddress, align: 16, flags: elf.SHF_ALLOC}
		switch name {
		case ecoff.S_TEXT, ecoff.S_INIT, ecoff.S_FINI:
			es.flags |= elf.SHF_EXECINSTR
		case ecoff.S_RDATA, ecoff.S_LIT4, ecoff.S_LIT8, ecoff.S_RCONST:
		default:
			es.flags |= elf.SHF_WRITE
		}
		if s.Size == 0 {
			es.typ = elf.SHT_NOBITS
			es.size = uint32(s.MemSize)
		} else {
			data, err := s.Data()
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", name, err)
			}
			es.typ = elf.SHT_PROGBITS
			es.data = data
		}
		bySection[name] = out.add(es)
		if name == ecoff.S_TEXT {
			text, textAddr = es.data, es.addr
		}
	}
	sectionAt := func(addr uint32) int {
		for _, s := range out.sections {
			if s.flags&elf.SHF_ALLOC != 0 && addr >= s.addr && addr < s.addr+max(uint32(len(s.data)), s.size) {
				return s.index
			}
		}
		return int(elf.SHN_ABS)
	}

	d, err := f.Debug()
	if err != nil {
		return nil, err
	}

	// The resident library's entry points: absolute symbols in its range,
	// which become functions in a section covering it, each as long as the
	// gap to the next.
	var resident []*ecoff.ExternalSymbol
	for _, s := range f.ExternalSymbols {
		if ecoff.StorageClass(s.StorageClass) == ecoff.SC_ABS && s.Value >= residentStart && s.Value < residentEnd {
			resident = append(resident, s)
		}
	}
	slices.SortFunc(resident, func(a, b *ecoff.ExternalSymbol) int { return cmp.Compare(a.Value, b.Value) })
	if len(resident) > 0 {
		lo := resident[0].Value
		libps := out.add(&section{name: ".libps", typ: elf.SHT_NOBITS, flags: elf.SHF_ALLOC | elf.SHF_EXECINSTR, addr: lo, size: residentEnd - lo, align: 4})
		for i, s := range resident {
			end := uint32(residentEnd)
			if i+1 < len(resident) {
				end = resident[i+1].Value
			}
			if end == s.Value {
				// Two names for one entry point: both get the size.
				for j := i + 1; j < len(resident) && resident[j].Value == s.Value; j++ {
					end = residentEnd
					if j+1 < len(resident) {
						end = resident[j+1].Value
					}
				}
			}
			out.symbols = append(out.symbols, symbol{name: s.Name, value: s.Value, size: end - s.Value, typ: elf.STT_FUNC, bind: elf.STB_GLOBAL, section: libps})
		}
	}

	// The program's own symbols. Functions take their sizes from their
	// procedures; anything else in a section, from the next symbol.
	funcSize := map[uint32]uint32{}
	var units []unit
	globals := map[string]uint32{}
	for _, s := range f.ExternalSymbols {
		if isData(s.StorageClass) {
			globals[s.Name] = s.Value
		}
	}
	if d != nil {
		for _, sf := range d.Files {
			u := convertFile(sf, text, textAddr)
			if err := describe(&u, sf, globals); err != nil {
				return nil, fmt.Errorf("%s: %w", sf.Name, err)
			}
			for _, fn := range u.funcs {
				funcSize[fn.addr] = fn.size
				bind := elf.STB_LOCAL
				if fn.external {
					bind = elf.STB_GLOBAL
				}
				out.symbols = append(out.symbols, symbol{name: fn.name, value: fn.addr, size: fn.size, typ: elf.STT_FUNC, bind: bind, section: sectionAt(fn.addr)})
			}
			if len(u.funcs) > 0 {
				units = append(units, u)
			}
			// File-local data, such as statics.
			for _, s := range sf.Symbols {
				if s.IsStab() || s.Type != ecoff.ST_STATIC || !isData(s.StorageClass) {
					continue
				}
				out.symbols = append(out.symbols, symbol{name: s.Name, value: s.Value, typ: elf.STT_OBJECT, bind: elf.STB_LOCAL, section: sectionAt(s.Value)})
			}
		}
	}
	// Several names can share an address, such as the entry point's
	// _start, the startup code's eprol and the linker's _ftext. A debugger
	// names the address after one of them, so only one is a function: the
	// procedure there, or else the entry point's _start, or else the first.
	// The others become local labels, still there to break on.
	claimed := map[uint32]bool{}
	for _, s := range out.symbols {
		if s.typ == elf.STT_FUNC {
			claimed[s.value] = true
		}
	}
	for _, s := range f.ExternalSymbols {
		if ecoff.StorageClass(s.StorageClass) == ecoff.SC_TEXT && s.Value == f.Entry && s.Name == "_start" {
			claimed[s.Value] = false // reserved for _start below
		}
	}
	for _, s := range f.ExternalSymbols {
		sc := ecoff.StorageClass(s.StorageClass)
		switch {
		case sc == ecoff.SC_ABS && s.Value >= residentStart && s.Value < residentEnd:
			// Done above.
		case sc == ecoff.SC_ABS:
			out.symbols = append(out.symbols, symbol{name: s.Name, value: s.Value, typ: elf.STT_NOTYPE, bind: elf.STB_GLOBAL, section: int(elf.SHN_ABS)})
		case sc == ecoff.SC_TEXT:
			if _, ok := funcSize[s.Value]; ok && hasFunc(out.symbols, s.Name, s.Value) {
				continue
			}
			sym := symbol{name: s.Name, value: s.Value, typ: elf.STT_FUNC, bind: elf.STB_GLOBAL, section: sectionAt(s.Value)}
			taken, seen := claimed[s.Value]
			switch {
			case linkerMarkers[s.Name]:
				sym.typ, sym.bind = elf.STT_NOTYPE, elf.STB_LOCAL
			case seen && taken:
				sym.typ, sym.bind = elf.STT_NOTYPE, elf.STB_LOCAL
			case seen && !taken && s.Name != "_start":
				sym.typ, sym.bind = elf.STT_NOTYPE, elf.STB_LOCAL
			default:
				claimed[s.Value] = true
			}
			out.symbols = append(out.symbols, sym)
		case isData(s.StorageClass):
			out.symbols = append(out.symbols, symbol{name: s.Name, value: s.Value, typ: elf.STT_OBJECT, bind: elf.STB_GLOBAL, section: sectionAt(s.Value)})
		}
	}
	sizeFromNext(out.symbols)

	// The SDK's startup code has no procedure, but is where the program
	// starts: frame information for it alone ends a backtrace there. It is
	// only for code with no procedure, which has no locals to show: a
	// function whose return address is undefined is one GDB will not find
	// the frame base of.
	var frameOnly []function
	if _, ok := funcSize[f.Entry]; !ok {
		for _, s := range out.symbols {
			if s.typ == elf.STT_FUNC && s.value == f.Entry && s.size > 0 {
				frameOnly = append(frameOnly, function{name: s.name, addr: s.value, size: s.size, outermost: true})
				break
			}
		}
	}

	dw := writeDWARF(units, frameOnly, "GCC for mipsel-ecoff, converted from ECOFF by yaroze")
	out.add(&section{name: ".debug_abbrev", typ: elf.SHT_PROGBITS, data: dw.abbrev, align: 1})
	out.add(&section{name: ".debug_info", typ: elf.SHT_PROGBITS, data: dw.info, align: 1})
	out.add(&section{name: ".debug_line", typ: elf.SHT_PROGBITS, data: dw.line, align: 1})
	out.add(&section{name: ".debug_frame", typ: elf.SHT_PROGBITS, data: dw.frame, align: 4})
	return out.bytes(), nil
}

// linkerMarkers are the names the linker gives section boundaries, which
// are no functions.
var linkerMarkers = map[string]bool{
	"_ftext": true, "etext": true, "_etext": true,
	"_fdata": true, "edata": true, "_edata": true,
	"_fbss": true, "end": true, "_end": true,
}

func isData(sc uint32) bool {
	switch ecoff.StorageClass(sc) {
	case ecoff.SC_DATA, ecoff.SC_BSS, ecoff.SC_SDATA, ecoff.SC_SBSS, ecoff.SC_RDATA, ecoff.SC_COMMON, ecoff.SC_SCOMMON:
		return true
	}
	return false
}

func hasFunc(syms []symbol, name string, addr uint32) bool {
	for _, s := range syms {
		if s.typ == elf.STT_FUNC && s.name == name && s.value == addr {
			return true
		}
	}
	return false
}

// sizeFromNext gives each symbol in a loaded section without a size the
// distance to the next symbol after it in the same section.
func sizeFromNext(syms []symbol) {
	order := make([]int, len(syms))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return cmp.Compare(syms[a].value, syms[b].value) })
	for k, i := range order {
		s := &syms[i]
		if s.size != 0 || s.section == int(elf.SHN_ABS) {
			continue
		}
		for _, j := range order[k+1:] {
			if syms[j].section == s.section && syms[j].value > s.value {
				s.size = syms[j].value - s.value
				break
			}
		}
	}
}

// convertFile makes a source file's compile unit: its name and directory
// from the stabs if it has them, and a function for each procedure with
// its size, lines and frame.
func convertFile(sf *ecoff.SourceFile, text []byte, textAddr uint32) unit {
	u := unit{name: sf.Name, language: langC89}
	switch ext := path.Ext(sf.Name); {
	case strings.EqualFold(ext, ".s"):
		u.language = langMipsAsm
	case ext == ".cc" || ext == ".cpp" || ext == ".cxx" || ext == ".C" || ext == ".c++":
		u.language = langCPlus
	}
	// N_SO: the directory, ending in a slash, then the file.
	for _, s := range sf.Symbols {
		if !s.IsStab() || s.Stab() != stabSO {
			continue
		}
		if strings.HasSuffix(s.Name, "/") {
			u.dir = strings.TrimSuffix(s.Name, "/")
		} else if s.Name != "" {
			u.name = s.Name
		}
	}
	if u.dir == "" && path.IsAbs(u.name) {
		u.dir = path.Dir(u.name)
	}

	// Each procedure's size is the value of the end symbol that closes its
	// symbol, and its lines are the $LM labels GCC puts at each line's
	// first instruction, with the line in the label's index.
	for pi, p := range sf.Procedures {
		fn := function{name: p.Name, addr: p.Address, external: !p.Static}
		if u.language == langCPlus {
			if d, ok := demangle(p.Name); ok {
				fn.name = d
			}
		}
		// The procedure's own symbols run from its symbol to the end
		// symbol that closes it.
		start, end := -1, -1
		for i, s := range sf.Symbols {
			if (s.Type == ecoff.ST_PROC || s.Type == ecoff.ST_STATIC_PROC) && s.Value == p.Address && s.Name == p.Name && !s.IsStab() {
				start = i
				break
			}
		}
		if start >= 0 {
			for i, s := range sf.Symbols[start+1:] {
				if s.Type == ecoff.ST_END && s.Name == p.Name && !s.IsStab() {
					fn.size = s.Value
					end = start + 1 + i
					break
				}
			}
		}
		if fn.size == 0 && pi+1 < len(sf.Procedures) && sf.Procedures[pi+1].Address > p.Address {
			fn.size = sf.Procedures[pi+1].Address - p.Address
		}
		for _, l := range p.Lines {
			fn.lines = append(fn.lines, lineEntry{addr: l.Address, line: l.Line})
		}
		// The line labels are among the procedure's own symbols: the one
		// for its closing brace can be at its end, which is the next
		// function's start.
		if len(fn.lines) == 0 && start >= 0 && end > start {
			for _, s := range sf.Symbols[start+1 : end] {
				if s.IsStab() || s.Type != ecoff.ST_LABEL || !strings.HasPrefix(s.Name, "$LM") {
					continue
				}
				if s.Value >= fn.addr && s.Value < fn.addr+fn.size {
					fn.lines = append(fn.lines, lineEntry{addr: s.Value, line: int32(s.SectionIndex)})
				}
			}
			slices.SortStableFunc(fn.lines, func(a, b lineEntry) int { return cmp.Compare(a.addr, b.addr) })
		}
		fn.frame = scanPrologue(text, textAddr, fn.addr, fn.size, p)
		if len(fn.lines) > 0 {
			fn.bodyStart = argStoresEnd(text, textAddr, fn.lines[0].addr, fn.addr+fn.size)
		}
		u.funcs = append(u.funcs, fn)
	}
	return u
}

// scanPrologue follows a function's frame as GCC writes it. In the
// prologue the stack pointer is lowered by the frame's size, the saved
// registers stored into the frame, and the frame pointer set from the stack
// pointer if the function uses one; each changes the frame from the
// instruction after it, and the prologue ends at the first branch or jump.
// After it, an epilogue that sets the stack pointer back from the frame
// pointer, before it reloads the frame pointer, makes the frame the stack
// pointer's again; after a return and its delay slot, any code that follows
// is in the body's frame.
func scanPrologue(text []byte, textAddr, addr, size uint32, p *ecoff.Procedure) []frameRule {
	cur := frameRule{reg: regSP, saved: map[uint8]int32{}}
	var rules []frameRule
	emit := func(at uint32, r frameRule) {
		c := frameRule{addr: at, reg: r.reg, cfa: r.cfa, saved: map[uint8]int32{}}
		for k, v := range r.saved {
			c.saved[k] = v
		}
		rules = append(rules, c)
	}
	insAt := func(pc uint32) (uint32, bool) {
		off := int(pc - textAddr)
		if off < 0 || off+4 > len(text) {
			return 0, false
		}
		return binary.LittleEndian.Uint32(text[off:]), true
	}
	pc := addr
	for ; pc < addr+size && pc < addr+64*4; pc += 4 {
		ins, ok := insAt(pc)
		if !ok {
			return rules
		}
		op, rs, rt := ins>>26, uint8(ins>>21&31), uint8(ins>>16&31)
		imm := int32(int16(ins))
		switch {
		case op == 0x09 && rs == regSP && rt == regSP && imm < 0: // addiu sp, sp, -N
			cur.cfa = uint32(-imm)
			emit(pc+4, cur)
			continue
		case op == 0x2b && rs == regSP && cur.cfa != 0: // sw rt, off(sp)
			cur.saved[rt] = imm - int32(cur.cfa)
			emit(pc+4, cur)
			continue
		case isMove(ins, regFP, regSP): // move fp, sp
			cur.reg = regFP
			emit(pc+4, cur)
			continue
		}
		if isBranch(ins) {
			break
		}
	}
	body := cur
	if body.reg != regFP {
		return rules
	}
	for ; pc+4 < addr+size; pc += 4 {
		ins, ok := insAt(pc)
		if !ok {
			break
		}
		switch {
		case isMove(ins, regSP, regFP): // move sp, fp: the epilogue
			emit(pc+4, frameRule{reg: regSP, cfa: body.cfa, saved: body.saved})
		case ins == 0x03e00008: // jr ra
			if pc+8 < addr+size {
				emit(pc+8, body)
			}
		}
	}
	return rules
}

// isMove reports whether ins is move dst, src, as GCC writes it: addu dst,
// src, zero.
func isMove(ins uint32, dst, src uint8) bool {
	return ins>>26 == 0 && ins&0x3f == 0x21 && uint8(ins>>11&31) == dst && uint8(ins>>21&31) == src && uint8(ins>>16&31) == 0
}

// isBranch reports whether ins is a branch or jump.
func isBranch(ins uint32) bool {
	op := ins >> 26
	switch op {
	case 0x00:
		fn := ins & 0x3f
		return fn == 0x08 || fn == 0x09 // jr, jalr
	case 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
		return true
	}
	return false
}

// argStoresEnd returns the address after the stores of argument registers
// to the stack that start at addr: GCC 2.95.3 at -O0 puts its first line
// label, for the opening brace, before them, so the arguments are not
// where their stabs say until they are done. GDB's own prologue analysis
// counts them as the prologue too.
func argStoresEnd(text []byte, textAddr, addr, end uint32) uint32 {
	for ; addr+4 <= end && addr >= textAddr && int(addr-textAddr)+4 <= len(text); addr += 4 {
		w := binary.LittleEndian.Uint32(text[addr-textAddr:])
		op, base, rt := w>>26, w>>21&31, w>>16&31
		store := op == 0x28 || op == 0x29 || op == 0x2b // sb, sh, sw
		if !store || rt < 4 || rt > 7 || (base != 29 && base != 30) {
			break
		}
	}
	return addr
}
