package tests

import (
	"cmp"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// The R3000 has no load interlock: the instruction after a load still sees
// the register's old value. LLVM's delay slot filler once put a load into a
// branch's slot when the instruction the branch reaches read what it
// loaded, and the cursor in examples/pads stuck at the screen's edge. These
// tests read every built program's machine code for a load whose result
// the next instruction to run reads, through a delay slot to the branch's
// target and its fall-through as well as in a straight line.

// TestNoLoadUseHazards scans the prog.elf of each disc under .build, or
// under PSX_DISCS, and names each load the next instruction reads.
func TestNoLoadUseHazards(t *testing.T) {
	elfs, err := filepath.Glob(filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../.build"), "*", "prog.elf"))
	assert.NoError(t, err)
	if len(elfs) == 0 {
		t.Skip("no discs built")
	}
	for _, path := range elfs {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			hazards, err := scanLoadUseHazards(path)
			assert.NoError(t, err)
			assert.Equal(t, []string(nil), hazards, path)
		})
	}
}

// TestLoadUseHazardScanner checks the scanner finds the code that broke
// examples/pads, and passes it once a nop follows the load.
func TestLoadUseHazardScanner(t *testing.T) {
	code := []uint32{
		0x14200002, // 8001a70c bnez $1, 8001a718
		0x8ca3e5b4, // 8001a710 lw $3, -0x1a4c($5), in the slot
		0x24020137, // 8001a714 addiu $2, $zero, 0x137
		0x0083082a, // 8001a718 slt $1, $4, $3
	}
	assert.Equal(t, []string{
		"8001a710 lw $3 is read by 8001a718, the target of 8001a70c",
	}, loadUseHazards(code, 0x8001a70c, nil))

	// In a straight line, and the fall-through of a branch.
	code = []uint32{
		0x8ca3e5b4, // lw $3, -0x1a4c($5)
		0x00641021, // addu $2, $3, $4
		0x10200003, // beqz $1, +3
		0x8ca3e5b4, // lw $3, -0x1a4c($5), in the slot
		0x00031040, // sll $2, $3, 1
	}
	assert.Equal(t, []string{
		"00001000 lw $3 is read by 00001004",
		"0000100c lw $3 is read by 00001010, the fall-through of 00001008",
	}, loadUseHazards(code, 0x1000, nil))

	// A nop in between, a write of the register rather than a read, and an
	// lwl and lwr pair, which the processor merges without a wait.
	code = []uint32{
		0x8ca3e5b4, // lw $3, -0x1a4c($5)
		0x00000000, // nop
		0x00641021, // addu $2, $3, $4
		0x8ca3e5b4, // lw $3, -0x1a4c($5)
		0x24030001, // addiu $3, $zero, 1
		0x88a30003, // lwl $3, 3($5)
		0x98a30000, // lwr $3, 0($5)
	}
	assert.Equal(t, []string(nil), loadUseHazards(code, 0x1000, nil))
}

// scanLoadUseHazards reads the executable sections of the ELF at path.
func scanLoadUseHazards(path string) ([]string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, _ := f.Symbols()
	var funcs []elf.Symbol
	for _, s := range syms {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC && s.Value != 0 {
			funcs = append(funcs, s)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Value < funcs[j].Value })
	var hazards []string
	for _, s := range f.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_EXECINSTR == 0 {
			continue
		}
		data, err := s.Data()
		if err != nil {
			return nil, err
		}
		code := make([]uint32, len(data)/4)
		for i := range code {
			code[i] = binary.LittleEndian.Uint32(data[i*4:])
		}
		hazards = append(hazards, loadUseHazards(code, uint32(s.Addr), funcs)...)
	}
	return hazards, nil
}

// loadUseHazards returns a line for each load in code, which starts at
// base, whose destination register the next instruction to run reads. A
// load in a branch's delay slot is checked against the first instruction
// at the branch's target and, unless the branch always goes, the one it
// falls through to. Lines name the function from funcs, sorted by address,
// when there are any.
func loadUseHazards(code []uint32, base uint32, funcs []elf.Symbol) []string {
	at := func(addr uint32) (uint32, bool) {
		i := (addr - base) / 4
		if addr < base || addr%4 != 0 || int(i) >= len(code) {
			return 0, false
		}
		return code[i], true
	}
	where := func(addr uint32) string {
		i := sort.Search(len(funcs), func(i int) bool { return funcs[i].Value > uint64(addr) })
		if i == 0 {
			return ""
		}
		s := funcs[i-1]
		if s.Size != 0 && uint64(addr) >= s.Value+s.Size {
			return ""
		}
		return fmt.Sprintf(" (%s+%#x)", s.Name, uint64(addr)-s.Value)
	}
	var hazards []string
	for i, insn := range code {
		addr := base + uint32(i)*4
		reg, name := loadDest(insn)
		// The compiler never allocates $k0 and $k1: a load into one is
		// written by hand, as in the clean library's probes, which time the
		// load delay on purpose.
		if reg == 0 || reg == 26 || reg == 27 {
			continue
		}
		type next struct {
			addr uint32
			why  string
		}
		var nexts []next
		if prev, ok := at(addr - 4); ok && i > 0 && isBranch(prev) {
			br := addr - 4
			if target, ok := branchTarget(prev, br); ok {
				nexts = append(nexts, next{target, fmt.Sprintf(", the target of %08x", br)})
			}
			if !alwaysGoes(prev) {
				nexts = append(nexts, next{addr + 4, fmt.Sprintf(", the fall-through of %08x", br)})
			}
		} else {
			nexts = append(nexts, next{addr + 4, ""})
		}
		for _, n := range nexts {
			if insn, ok := at(n.addr); ok && reads(insn, reg) {
				hazards = append(hazards, fmt.Sprintf("%08x %s $%d is read by %08x%s%s",
					addr, name, reg, n.addr, n.why, where(addr)))
			}
		}
	}
	return hazards
}

// loadDest returns the register a load writes, with the load's name, or 0
// if insn is not a load with a delay: the memory loads, and the moves from
// coprocessor 0 and the GTE.
func loadDest(insn uint32) (int, string) {
	rt := int(insn >> 16 & 31)
	switch op := insn >> 26; op {
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26:
		return rt, [...]string{"lb", "lh", "lwl", "lw", "lbu", "lhu", "lwr"}[op-0x20]
	case 0x10, 0x12:
		switch insn >> 21 & 31 {
		case 0:
			return rt, fmt.Sprintf("mfc%d", op&3)
		case 2:
			return rt, fmt.Sprintf("cfc%d", op&3)
		}
	}
	return 0, ""
}

// reads reports whether insn reads general register reg, which is not 0.
func reads(insn uint32, reg int) bool {
	rs, rt := int(insn>>21&31), int(insn>>16&31)
	switch op := insn >> 26; op {
	case 0x00: // SPECIAL
		switch funct := insn & 63; {
		case funct == 0x00 || funct == 0x02 || funct == 0x03: // sll, srl, sra
			return rt == reg
		case funct == 0x08 || funct == 0x09 || funct == 0x11 || funct == 0x13: // jr, jalr, mthi, mtlo
			return rs == reg
		case funct == 0x0c || funct == 0x0d || funct == 0x10 || funct == 0x12: // syscall, break, mfhi, mflo
			return false
		default: // the shifts by register, multiply and divide, and arithmetic
			return rs == reg || rt == reg
		}
	case 0x02, 0x03: // j, jal
		return false
	case 0x04, 0x05: // beq, bne
		return rs == reg || rt == reg
	case 0x10, 0x12: // COP0, COP2
		switch insn >> 21 & 31 {
		case 4, 6: // mtc, ctc
			return rt == reg
		}
		return false
	case 0x22, 0x26: // lwl, lwr merge into rt without waiting for a load before
		return rs == reg
	case 0x28, 0x29, 0x2a, 0x2b, 0x2e: // stores
		return rs == reg || rt == reg
	case 0x0f: // lui
		return false
	default: // REGIMM, blez, bgtz, the immediates, loads, lwc2, swc2
		return rs == reg
	}
}

// isBranch reports whether insn is a branch or jump, which has a delay slot.
func isBranch(insn uint32) bool {
	switch op := insn >> 26; op {
	case 0x00:
		funct := insn & 63
		return funct == 0x08 || funct == 0x09
	case 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
		return true
	}
	return false
}

// branchTarget returns where the branch or jump at pc goes, if it is known.
func branchTarget(insn, pc uint32) (uint32, bool) {
	switch insn >> 26 {
	case 0x00: // jr, jalr
		return 0, false
	case 0x02, 0x03:
		return (pc+4)&0xf0000000 | (insn&0x03ffffff)<<2, true
	default:
		return pc + 4 + uint32(int32(int16(insn))<<2), true
	}
}

// alwaysGoes reports whether the instruction after a branch's delay slot
// never runs straight after it: jumps, b (beq $0, $0), and the calls, after
// which it runs only once the callee returns.
func alwaysGoes(insn uint32) bool {
	switch op := insn >> 26; op {
	case 0x00, 0x02, 0x03:
		return true
	case 0x01: // bltzal, bgezal link
		return insn>>16&0x10 != 0 || insn>>21&31 == 0 && insn>>16&31 == 1
	case 0x04:
		return insn>>21&31 == 0 && insn>>16&31 == 0
	}
	return false
}
