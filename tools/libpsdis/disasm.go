package main

import "fmt"

// Kinds of instruction, for the annotations: what a listing can say about
// an instruction depends on what it reaches for.
const (
	insnPlain     = iota
	insnJump      // j: Target is where
	insnCall      // jal: Target is where
	insnBranch    // a conditional branch: Target is where
	insnLui       // lui: the high half of an address, finished by the next instruction
	insnLoadStore // a load or store: Base and Offset say through what
	insnJumpReg   // jr, jalr: through a register
)

// An Insn is one decoded instruction.
type Insn struct {
	Mnemonic string
	Operands string
	Kind     int
	Target   uint32 // for a jump, call or branch
	Rs, Rt   int    // the source registers, -1 if none
	Rd       int    // the destination register, -1 if none
	Imm      int32  // the sign-extended immediate, for lui/addiu/ori pairs and loads
}

var regs = [32]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
	"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
	"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
}

// Decode decodes the MIPS I instruction word at addr: the R3000A's set,
// with the coprocessor 2 moves and commands the library uses. A word it
// does not know is ".word".
func Decode(addr, word uint32) Insn {
	op := word >> 26
	rs := int(word>>21) & 31
	rt := int(word>>16) & 31
	rd := int(word>>11) & 31
	sa := int(word>>6) & 31
	fn := word & 63
	imm := int32(int16(word & 0xffff))
	uimm := word & 0xffff
	in := Insn{Rs: rs, Rt: rt, Rd: -1, Imm: imm}
	branch := func(name string, ops string) Insn {
		in.Mnemonic = name
		in.Kind = insnBranch
		in.Target = addr + 4 + uint32(imm)<<2
		in.Operands = fmt.Sprintf("%s0x%08x", ops, in.Target)
		return in
	}
	switch op {
	case 0:
		in.Rd = rd
		switch fn {
		case 0:
			if word == 0 {
				return Insn{Mnemonic: "nop", Rs: -1, Rt: -1, Rd: -1}
			}
			return in.r3("sll", regs[rd], regs[rt], fmt.Sprint(sa))
		case 2:
			return in.r3("srl", regs[rd], regs[rt], fmt.Sprint(sa))
		case 3:
			return in.r3("sra", regs[rd], regs[rt], fmt.Sprint(sa))
		case 4:
			return in.r3("sllv", regs[rd], regs[rt], regs[rs])
		case 6:
			return in.r3("srlv", regs[rd], regs[rt], regs[rs])
		case 7:
			return in.r3("srav", regs[rd], regs[rt], regs[rs])
		case 8:
			in.Mnemonic, in.Operands, in.Kind, in.Rd = "jr", regs[rs], insnJumpReg, -1
			return in
		case 9:
			in.Mnemonic, in.Kind = "jalr", insnJumpReg
			if rd == 31 {
				in.Operands = regs[rs]
			} else {
				in.Operands = regs[rd] + ", " + regs[rs]
			}
			return in
		case 12:
			in.Mnemonic, in.Operands, in.Rd = "syscall", fmt.Sprint((word>>6)&0xfffff), -1
			return in
		case 13:
			in.Mnemonic, in.Operands, in.Rd = "break", fmt.Sprint((word>>6)&0xfffff), -1
			return in
		case 16:
			return in.r3("mfhi", regs[rd], "", "")
		case 17:
			return in.r3("mthi", regs[rs], "", "")
		case 18:
			return in.r3("mflo", regs[rd], "", "")
		case 19:
			return in.r3("mtlo", regs[rs], "", "")
		case 24:
			return in.r3("mult", regs[rs], regs[rt], "")
		case 25:
			return in.r3("multu", regs[rs], regs[rt], "")
		case 26:
			return in.r3("div", regs[rs], regs[rt], "")
		case 27:
			return in.r3("divu", regs[rs], regs[rt], "")
		case 32:
			return in.r3("add", regs[rd], regs[rs], regs[rt])
		case 33:
			if rt == 0 {
				return in.r3("move", regs[rd], regs[rs], "")
			}
			return in.r3("addu", regs[rd], regs[rs], regs[rt])
		case 34:
			return in.r3("sub", regs[rd], regs[rs], regs[rt])
		case 35:
			return in.r3("subu", regs[rd], regs[rs], regs[rt])
		case 36:
			return in.r3("and", regs[rd], regs[rs], regs[rt])
		case 37:
			return in.r3("or", regs[rd], regs[rs], regs[rt])
		case 38:
			return in.r3("xor", regs[rd], regs[rs], regs[rt])
		case 39:
			return in.r3("nor", regs[rd], regs[rs], regs[rt])
		case 42:
			return in.r3("slt", regs[rd], regs[rs], regs[rt])
		case 43:
			return in.r3("sltu", regs[rd], regs[rs], regs[rt])
		}
	case 1:
		switch rt {
		case 0:
			return branch("bltz", regs[rs]+", ")
		case 1:
			return branch("bgez", regs[rs]+", ")
		case 16:
			return branch("bltzal", regs[rs]+", ")
		case 17:
			return branch("bgezal", regs[rs]+", ")
		}
	case 2, 3:
		in.Target = (addr+4)&0xf0000000 | (word&0x3ffffff)<<2
		in.Operands = fmt.Sprintf("0x%08x", in.Target)
		in.Rs, in.Rt = -1, -1
		if op == 2 {
			in.Mnemonic, in.Kind = "j", insnJump
		} else {
			in.Mnemonic, in.Kind = "jal", insnCall
		}
		return in
	case 4:
		return branch("beq", regs[rs]+", "+regs[rt]+", ")
	case 5:
		return branch("bne", regs[rs]+", "+regs[rt]+", ")
	case 6:
		return branch("blez", regs[rs]+", ")
	case 7:
		return branch("bgtz", regs[rs]+", ")
	case 8:
		return in.imm("addi", regs[rt], regs[rs], fmt.Sprint(imm))
	case 9:
		if rs == 0 {
			return in.imm("li", regs[rt], "", fmt.Sprint(imm))
		}
		return in.imm("addiu", regs[rt], regs[rs], fmt.Sprint(imm))
	case 10:
		return in.imm("slti", regs[rt], regs[rs], fmt.Sprint(imm))
	case 11:
		return in.imm("sltiu", regs[rt], regs[rs], fmt.Sprint(imm))
	case 12:
		return in.imm("andi", regs[rt], regs[rs], fmt.Sprintf("0x%x", uimm))
	case 13:
		if rs == 0 {
			return in.imm("li", regs[rt], "", fmt.Sprintf("0x%x", uimm))
		}
		return in.imm("ori", regs[rt], regs[rs], fmt.Sprintf("0x%x", uimm))
	case 14:
		return in.imm("xori", regs[rt], regs[rs], fmt.Sprintf("0x%x", uimm))
	case 15:
		in.Mnemonic, in.Kind, in.Rd, in.Rs = "lui", insnLui, rt, -1
		in.Operands = fmt.Sprintf("%s, 0x%x", regs[rt], uimm)
		return in
	case 16, 18:
		cop := "0"
		if op == 18 {
			cop = "2"
		}
		switch rs {
		case 0:
			return in.r3("mfc"+cop, regs[rt], "$"+fmt.Sprint(rd), "")
		case 2:
			return in.r3("cfc"+cop, regs[rt], "$"+fmt.Sprint(rd), "")
		case 4:
			return in.r3("mtc"+cop, regs[rt], "$"+fmt.Sprint(rd), "")
		case 6:
			return in.r3("ctc"+cop, regs[rt], "$"+fmt.Sprint(rd), "")
		}
		if rs&16 != 0 {
			if op == 16 && word&0x3f == 16 {
				return Insn{Mnemonic: "rfe", Rs: -1, Rt: -1, Rd: -1}
			}
			in.Mnemonic, in.Operands, in.Rs, in.Rt = "cop"+cop, fmt.Sprintf("0x%x", word&0x1ffffff), -1, -1
			return in
		}
	case 32:
		return in.mem("lb", word)
	case 33:
		return in.mem("lh", word)
	case 34:
		return in.mem("lwl", word)
	case 35:
		return in.mem("lw", word)
	case 36:
		return in.mem("lbu", word)
	case 37:
		return in.mem("lhu", word)
	case 38:
		return in.mem("lwr", word)
	case 40:
		return in.mem("sb", word)
	case 41:
		return in.mem("sh", word)
	case 42:
		return in.mem("swl", word)
	case 43:
		return in.mem("sw", word)
	case 46:
		return in.mem("swr", word)
	case 50:
		return in.mem("lwc2", word)
	case 58:
		return in.mem("swc2", word)
	}
	return Insn{Mnemonic: ".word", Operands: fmt.Sprintf("0x%08x", word), Rs: -1, Rt: -1, Rd: -1}
}

// r3 finishes a register-form instruction with up to three operands.
func (in Insn) r3(name, a, b, c string) Insn {
	in.Mnemonic = name
	in.Operands = a
	if b != "" {
		in.Operands += ", " + b
	}
	if c != "" {
		in.Operands += ", " + c
	}
	return in
}

// imm finishes an immediate-form instruction, whose destination is rt.
func (in Insn) imm(name, dst, src, imm string) Insn {
	in.Rd = in.Rt
	in.Mnemonic = name
	in.Operands = dst
	if src != "" {
		in.Operands += ", " + src
	}
	in.Operands += ", " + imm
	return in
}

// mem finishes a load or store. The coprocessor forms name their register
// by number.
func (in Insn) mem(name string, word uint32) Insn {
	in.Mnemonic = name
	in.Kind = insnLoadStore
	reg := regs[in.Rt]
	if name[len(name)-1] == '2' {
		reg = "$" + fmt.Sprint(in.Rt)
	}
	if name[0] == 'l' {
		in.Rd = in.Rt
	}
	in.Operands = fmt.Sprintf("%s, %d(%s)", reg, in.Imm, regs[in.Rs])
	return in
}
