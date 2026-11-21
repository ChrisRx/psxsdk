package ecoffelf

import (
	"bytes"
	"encoding/binary"
)

// DWARF constants this writer uses, from the DWARF 4 standard.
const (
	tagCompileUnit = 0x11
	tagSubprogram  = 0x2e

	atName      = 0x03
	atStmtList  = 0x10
	atLowPC     = 0x11
	atHighPC    = 0x12
	atLanguage  = 0x13
	atCompDir   = 0x1b
	atProducer  = 0x25
	atExternal  = 0x3f
	atFrameBase = 0x40

	formAddr      = 0x01
	formData2     = 0x05
	formData4     = 0x06
	formString    = 0x08
	formFlag      = 0x0c
	formSecOffset = 0x17
	formExprloc   = 0x18

	langC89      = 0x0001
	langCPlus    = 0x0004
	langMipsAsm  = 0x8001
	opCallFrame  = 0x9c
	childrenYes  = 1
	childrenNo   = 0
	abbrevCU     = 1
	abbrevSubprg = 2
)

// Line program opcodes.
const (
	lnsCopy        = 1
	lnsAdvancePC   = 2
	lnsAdvanceLine = 3
	lnsPrologueEnd = 10
	lneEndSequence = 1
	lneSetAddress  = 2
)

// Call frame instructions.
const (
	cfaAdvanceLoc4      = 0x04
	cfaDefCFA           = 0x0c
	cfaDefCFARegister   = 0x0d
	cfaDefCFAOffset     = 0x0e
	cfaOffsetExtended   = 0x05
	cfaOffset           = 0x80 // | register
	cfaUndefined        = 0x07
	cfaValOffset        = 0x14
	cfaNop              = 0x00
	regSP, regFP, regRA = 29, 30, 31
)

// A unit is one compile unit: a source file and its functions.
type unit struct {
	name, dir string
	language  uint16
	funcs     []function

	// decls are the unit's types and file-level variables.
	decls []*die
}

// A function is a subprogram: its extent, its lines and its frame.
type function struct {
	name     string
	addr     uint32
	size     uint32
	external bool
	lines    []lineEntry
	frame    []frameRule

	// bodyStart is where the function's body starts: its first line
	// label, past any stores of its arguments to the stack that follow it.
	bodyStart uint32

	// outermost is set for the function the program starts at, which no
	// function called: its return address is undefined, which ends a
	// debugger's backtrace there.
	outermost bool

	// ret is the type the function returns, nil for none, and body its
	// parameters, variables and blocks, from the stabs.
	ret  *die
	body []*die
}

type lineEntry struct {
	addr uint32
	line int32
}

// A frameRule says how the frame stands from addr on: the CFA is the stack
// pointer, or the frame pointer, plus cfa, and each saved register is at
// the CFA plus its offset.
type frameRule struct {
	addr  uint32
	reg   uint8 // regSP or regFP
	cfa   uint32
	saved map[uint8]int32
}

type dwarfSections struct {
	abbrev, info, line, frame []byte
}

func uleb(b *bytes.Buffer, v uint64) {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			c |= 0x80
		}
		b.WriteByte(c)
		if v == 0 {
			return
		}
	}
}

func sleb(b *bytes.Buffer, v int64) {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		done := (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0)
		if !done {
			c |= 0x80
		}
		b.WriteByte(c)
		if done {
			return
		}
	}
}

func cstring(b *bytes.Buffer, s string) {
	b.WriteString(s)
	b.WriteByte(0)
}

func u16(b *bytes.Buffer, v uint16) { binary.Write(b, binary.LittleEndian, v) }
func u32(b *bytes.Buffer, v uint32) { binary.Write(b, binary.LittleEndian, v) }

// writeDWARF writes the units' debugging information: a compile unit and
// line program for each, and frame information for every function, and
// for the functions in frameOnly, which have no unit.
func writeDWARF(units []unit, frameOnly []function, producer string) dwarfSections {
	enc := newEncoder()
	var info, line bytes.Buffer
	for _, u := range units {
		low, high := u.funcs[0].addr, u.funcs[0].addr+u.funcs[0].size
		for _, f := range u.funcs {
			low, high = min(low, f.addr), max(high, f.addr+f.size)
		}
		stmt := uint32(line.Len())
		writeLines(&line, u)

		cu := &die{tag: tagCompileUnit}
		cu.add(atName, formString, u.name)
		cu.add(atCompDir, formString, u.dir)
		cu.add(atProducer, formString, producer)
		cu.add(atLanguage, formData2, uint32(u.language))
		cu.add(atLowPC, formAddr, low)
		cu.add(atHighPC, formData4, high-low)
		cu.add(atStmtList, formSecOffset, stmt)
		for _, f := range u.funcs {
			sp := cu.child(&die{tag: tagSubprogram})
			sp.add(atName, formString, f.name)
			sp.add(atExternal, formFlag, f.external)
			sp.add(atLowPC, formAddr, f.addr)
			sp.add(atHighPC, formData4, f.size)
			sp.add(atFrameBase, formExprloc, []byte{opCallFrame})
			if f.ret != nil {
				sp.add(atType, formRef4, f.ret)
			}
			sp.children = append(sp.children, f.body...)
		}
		cu.children = append(cu.children, u.decls...)
		info.Write(enc.unit(cu))
	}
	enc.abbrev.WriteByte(0)

	return dwarfSections{
		abbrev: enc.abbrev.Bytes(),
		info:   info.Bytes(),
		line:   line.Bytes(),
		frame:  writeFrames(append(allFuncs(units), frameOnly...)),
	}
}

// writeLines writes u's line program, DWARF 4, a sequence for each function
// with lines. The file is u's name in its compilation directory.
func writeLines(out *bytes.Buffer, u unit) {
	var hdr bytes.Buffer
	hdr.WriteByte(4) // minimum instruction length
	hdr.WriteByte(1) // maximum operations per instruction
	hdr.WriteByte(1) // default is_stmt
	hdr.WriteByte(0xfb)
	hdr.WriteByte(14) // line range
	hdr.WriteByte(13) // opcode base
	hdr.Write([]byte{0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1})
	hdr.WriteByte(0) // no include directories but the compilation one
	cstring(&hdr, u.name)
	hdr.Write([]byte{0, 0, 0}) // directory 0, no time, no length
	hdr.WriteByte(0)

	var prog bytes.Buffer
	for _, f := range u.funcs {
		if len(f.lines) == 0 {
			continue
		}
		prog.WriteByte(0)
		uleb(&prog, 5)
		prog.WriteByte(lneSetAddress)
		u32(&prog, f.addr)
		addr, ln := f.addr, int32(1)
		// GCC's first line label is where it put the function's first
		// line, after the prologue. The function's start gets that line
		// too, and the first label from the body's start on is marked as
		// the prologue's end, which is where a debugger breaks on the
		// function.
		lines := f.lines
		end := -1
		if lines[0].addr > f.addr {
			lines = append([]lineEntry{{addr: f.addr, line: lines[0].line}}, lines...)
			end = 1
			for i := 1; i < len(lines); i++ {
				if lines[i].addr >= f.bodyStart {
					end = i
					break
				}
			}
		}
		for i, e := range lines {
			if e.addr < addr {
				continue // out of order: the program can't go back
			}
			if e.addr != addr {
				prog.WriteByte(lnsAdvancePC)
				uleb(&prog, uint64(e.addr-addr)/4)
				addr = e.addr
			}
			if e.line != ln {
				prog.WriteByte(lnsAdvanceLine)
				sleb(&prog, int64(e.line-ln))
				ln = e.line
			}
			if i == end {
				prog.WriteByte(lnsPrologueEnd)
			}
			prog.WriteByte(lnsCopy)
		}
		if end := f.addr + f.size; end > addr {
			prog.WriteByte(lnsAdvancePC)
			uleb(&prog, uint64(end-addr)/4)
		}
		prog.WriteByte(0)
		uleb(&prog, 1)
		prog.WriteByte(lneEndSequence)
	}

	var unitBody bytes.Buffer
	u16(&unitBody, 4)
	u32(&unitBody, uint32(hdr.Len()))
	unitBody.Write(hdr.Bytes())
	unitBody.Write(prog.Bytes())
	u32(out, uint32(unitBody.Len()))
	out.Write(unitBody.Bytes())
}

// writeFrames writes .debug_frame: one CIE, in which the CFA is the stack
// pointer and the return address is in ra, and an FDE for each function
// that says how its prologue changes that.
func writeFrames(funcs []function) []byte {
	var out bytes.Buffer
	var cie bytes.Buffer
	u32(&cie, 0xffffffff) // CIE id
	cie.WriteByte(1)      // version
	cie.WriteByte(0)      // no augmentation
	uleb(&cie, 4)         // code alignment
	sleb(&cie, -4)        // data alignment
	cie.WriteByte(regRA)  // return address register
	cie.WriteByte(cfaDefCFA)
	uleb(&cie, regSP)
	uleb(&cie, 0)
	// The caller's stack pointer is the CFA, the stack pointer on entry.
	cie.WriteByte(cfaValOffset)
	uleb(&cie, regSP)
	uleb(&cie, 0)
	for (cie.Len()+4)%4 != 0 {
		cie.WriteByte(cfaNop)
	}
	u32(&out, uint32(cie.Len()))
	out.Write(cie.Bytes())

	for _, f := range funcs {
		var fde bytes.Buffer
		u32(&fde, 0) // the CIE at offset 0
		u32(&fde, f.addr)
		u32(&fde, f.size)
		if f.outermost {
			fde.WriteByte(cfaUndefined)
			uleb(&fde, regRA)
		}
		loc := f.addr
		prev := frameRule{reg: regSP}
		for _, r := range f.frame {
			if r.addr > loc {
				fde.WriteByte(cfaAdvanceLoc4)
				u32(&fde, (r.addr-loc)/4)
				loc = r.addr
			}
			if r.reg != prev.reg {
				fde.WriteByte(cfaDefCFA)
				uleb(&fde, uint64(r.reg))
				uleb(&fde, uint64(r.cfa))
			} else if r.cfa != prev.cfa {
				fde.WriteByte(cfaDefCFAOffset)
				uleb(&fde, uint64(r.cfa))
			}
			for _, reg := range sortedRegs(r.saved) {
				off := r.saved[reg]
				if old, ok := prev.saved[reg]; ok && old == off {
					continue
				}
				if f.outermost && reg == regRA {
					continue // there is no caller to return to
				}
				// Saved at CFA+off; factored by the data alignment, -4.
				fde.WriteByte(cfaOffset | reg)
				uleb(&fde, uint64(-off/4))
			}
			prev = r
		}
		for (fde.Len()+4)%4 != 0 {
			fde.WriteByte(cfaNop)
		}
		u32(&out, uint32(fde.Len()))
		out.Write(fde.Bytes())
	}
	return out.Bytes()
}

func allFuncs(units []unit) []function {
	var fs []function
	for _, u := range units {
		fs = append(fs, u.funcs...)
	}
	return fs
}

func sortedRegs(m map[uint8]int32) []uint8 {
	var regs []uint8
	for r := range 32 {
		if _, ok := m[uint8(r)]; ok {
			regs = append(regs, uint8(r))
		}
	}
	return regs
}
