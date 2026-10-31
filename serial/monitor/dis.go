package main

// dis: the monitor's disassembler. It names things as the boot disc's
// monitor does, which the tests check against what that one prints: its
// own names for what is not an instruction, the COP0 register names, and
// the GTE, coprocessor 2, decoded as if it were the floating point unit.

var gpr = [32]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
	"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
	"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
}

// cop0Names are the COP0 registers' names, where they have one.
var cop0Names = [16]string{0: "index", 1: "random", 2: "tlblo", 4: "context", 8: "badvaddr", 10: "tlbhi", 12: "sr", 13: "cause", 14: "epc"}

// cop0Ops are COP0's operations' names, where they have one.
var cop0Ops = [32]string{1: "tlbr", 2: "tlbwi", 6: "tlbwr", 8: "tlbp", 16: "rfe"}

// The floating point operations and formats coprocessor 2 is decoded as.
var fpOps = [64]string{0: "fadd", 1: "fsub", 2: "fmpy", 3: "fdiv", 4: "fsqrt", 5: "fabs", 6: "fmov", 0x20: "fcvts", 0x21: "fcvtd", 0x22: "fcvte", 0x24: "fcvtw"}
var fpFormats = [16]string{0: "s", 1: "d", 2: "e", 4: "w"}

var specials = [64]string{
	0: "sll", 2: "srl", 3: "sra", 4: "sllv", 6: "srlv", 7: "srav", 8: "jr", 9: "jalr",
	12: "syscall", 13: "break", 14: "vcall", 16: "mfhi", 17: "mthi", 18: "mflo", 19: "mtlo",
	24: "mult", 25: "multu", 26: "div", 27: "divu",
	32: "add", 33: "addu", 34: "sub", 35: "subu", 36: "and", 37: "or", 38: "xor", 39: "nor", 42: "slt", 43: "sltu",
}

var regimms = [32]string{0: "bltz", 1: "bgez", 16: "bltzal", 17: "bgezal"}

var opcodes = [64]string{
	2: "j", 3: "jal", 4: "beq", 5: "bne", 6: "blez", 7: "bgtz",
	8: "addi", 9: "addiu", 10: "slti", 11: "sltiu", 12: "andi", 13: "ori", 14: "xori", 15: "lui",
	32: "lb", 33: "lh", 34: "lwl", 35: "lw", 36: "lbu", 37: "lhu", 38: "lwr", 39: "ld",
	40: "sb", 41: "sh", 42: "swl", 43: "sw", 46: "swr", 47: "sd",
	48: "lwc0", 49: "lwc1", 50: "lwc2", 51: "lwc3", 52: "ldc0", 53: "ldc1", 54: "ldc2", 55: "ldc3",
	56: "swc0", 57: "swc1", 58: "swc2", 59: "swc3", 60: "sdc0", 61: "sdc1", 62: "sdc2", 63: "sdc3",
}

// insn is where an instruction's text is made: its mnemonic and operands.
type insn struct {
	mnem, ops [40]byte
	m, o      int
}

func (in *insn) mn(s string) { in.m += copy(in.mnem[in.m:], s) }
func (in *insn) op(s string) { in.o += copy(in.ops[in.o:], s) }
func (in *insn) opc(c byte)  { in.ops[in.o] = c; in.o++ }

// hexNum writes v as 0x and its hexadecimal digits, without leading zeros.
func (in *insn) hexNum(v uint32) {
	in.op("0x")
	n := 1
	for x := v >> 4; x != 0; x >>= 4 {
		n++
	}
	for i := n - 1; i >= 0; i-- {
		in.opc("0123456789abcdef"[v>>(4*i)&15])
	}
}

func (in *insn) octal2(v uint32) {
	in.mnem[in.m] = byte('0' + v>>3&7)
	in.mnem[in.m+1] = byte('0' + v&7)
	in.m += 2
}

func (in *insn) hex1(b *[40]byte, n *int, v uint32) {
	b[*n] = "0123456789abcdef"[v&15]
	*n++
}

func (in *insn) decimal(v uint32) {
	if v >= 10 {
		in.decimal(v / 10)
	}
	in.opc(byte('0' + v%10))
}

// regs writes registers, a comma between each: one, two or three, as
// many as are not -1. It takes them one by one, as a variadic call would
// take memory for each.
func (in *insn) regs(a, b, c int32) {
	in.op(gpr[a])
	if b >= 0 {
		in.opc(',')
		in.op(gpr[b])
	}
	if c >= 0 {
		in.opc(',')
		in.op(gpr[c])
	}
}

func (in *insn) fpr(r uint32) {
	in.opc('f')
	in.decimal(r)
}

func (in *insn) cop0Reg(r uint32) {
	if r < 16 && cop0Names[r] != "" {
		in.op(cop0Names[r])
		return
	}
	in.op("c0r")
	in.decimal(r)
}

// disassemble decodes w, at pc, into in, and reports whether it is a jump
// or branch, which has a delay slot.
func disassemble(in *insn, pc, w uint32) bool {
	op, rs, rt, rd, sa, funct := w>>26, w>>21&31, w>>16&31, w>>11&31, w>>6&31, w&63
	imm := w & 0xffff
	simm := uint32(int32(int16(imm)))
	target := pc + 4 + simm<<2
	switch op {
	case 0:
		return disSpecial(in, w, rs, rt, rd, sa, funct)
	case 1:
		if regimms[rt] != "" {
			in.mn(regimms[rt])
		} else {
			in.mn("bcond")
			in.octal2(rt)
		}
		in.regs(int32(rs), -1, -1)
		in.opc(',')
		in.hexNum(target)
		return true
	case 2, 3:
		in.mn(opcodes[op])
		in.hexNum((pc+4)&0xf0000000 | w&0x3ffffff<<2)
		return true
	case 4, 5:
		if op == 4 && rs == 0 && rt == 0 {
			in.mn("b")
		} else {
			in.mn(opcodes[op])
			in.regs(int32(rs), int32(rt), -1)
			in.opc(',')
		}
		in.hexNum(target)
		return true
	case 6, 7:
		in.mn(opcodes[op])
		in.regs(int32(rs), -1, -1)
		in.opc(',')
		in.hexNum(target)
		return true
	case 8, 9, 10, 11:
		in.mn(opcodes[op])
		in.regs(int32(rt), int32(rs), -1)
		in.opc(',')
		in.hexNum(simm)
	case 12, 13, 14:
		in.mn(opcodes[op])
		in.regs(int32(rt), int32(rs), -1)
		in.opc(',')
		in.hexNum(imm)
	case 15:
		in.mn("lui")
		in.regs(int32(rt), -1, -1)
		in.opc(',')
		in.hexNum(imm)
	case 16:
		return disCop0(in, target, rs, rt, rd, funct)
	case 18:
		return disFP(in, target, rs, rt, rd, sa, funct)
	case 17, 19:
		in.mn("cop")
		in.mnem[in.m] = byte('0' + op - 16)
		in.m++
		in.regs(int32(rt), int32(rs), -1)
		in.opc(',')
		in.hexNum(imm)
	case 32, 33, 35, 36, 37, 39, 40, 41, 43, 47:
		in.mn(opcodes[op])
		in.regs(int32(rt), -1, -1)
		in.memory(simm, rs)
	case 49, 53, 57, 61:
		in.mn(opcodes[op])
		in.fpr(rt)
		in.memory(simm, rs)
	default:
		if opcodes[op] != "" {
			in.mn(opcodes[op])
		} else {
			in.mn("op")
			in.hex1(&in.mnem, &in.m, op>>2)
			in.hex1(&in.mnem, &in.m, op<<2)
		}
		in.regs(int32(rt), int32(rs), -1)
		in.opc(',')
		in.hexNum(simm)
	}
	return false
}

func (in *insn) memory(off, base uint32) {
	in.opc(',')
	in.hexNum(off)
	in.opc('(')
	in.op(gpr[base])
	in.opc(')')
}

func disSpecial(in *insn, w, rs, rt, rd, sa, funct uint32) bool {
	name := specials[funct]
	switch {
	case w == 0:
		in.mn("nop")
	case funct == 33 && rt == 0:
		in.mn("move")
		in.regs(int32(rd), int32(rs), -1)
	case name == "":
		in.mn("spec")
		in.octal2(funct)
		in.regs(int32(rd), int32(rs), int32(rt))
	case funct < 4:
		in.mn(name)
		in.regs(int32(rd), int32(rt), -1)
		in.opc(',')
		in.decimal(sa)
	case funct < 8:
		in.mn(name)
		in.regs(int32(rd), int32(rt), int32(rs))
	case funct == 8 || funct == 9:
		in.mn(name)
		in.regs(int32(rs), -1, -1)
		return true
	case funct == 12:
		in.mn(name)
	case funct == 13 || funct == 14:
		in.mn(name)
		in.hexNum(w >> 6 & 0xfffff)
	case funct == 16 || funct == 18:
		in.mn(name)
		in.regs(int32(rd), -1, -1)
	case funct == 17 || funct == 19:
		in.mn(name)
		in.regs(int32(rs), -1, -1)
	case funct < 32:
		in.mn(name)
		in.regs(int32(rs), int32(rt), -1)
	default:
		in.mn(name)
		in.regs(int32(rd), int32(rs), int32(rt))
	}
	return false
}

func disCop0(in *insn, target, rs, rt, rd, funct uint32) bool {
	switch rs {
	case 0, 4:
		if rs == 0 {
			in.mn("mfc0")
		} else {
			in.mn("mtc0")
		}
		in.regs(int32(rt), -1, -1)
		in.opc(',')
		in.cop0Reg(rd)
		return false
	case 8:
		in.mn("bc0")
		in.bc(rt, target)
		return true
	}
	in.mn("c0")
	switch {
	case funct >= 32:
		in.cop0Reg(funct - 32)
	case cop0Ops[funct] != "":
		in.op(cop0Ops[funct])
	default:
		in.op("c0op")
		in.decimal(funct)
	}
	return false
}

func (in *insn) bc(rt, target uint32) {
	if rt&1 != 0 {
		in.mn("t")
	} else {
		in.mn("f")
	}
	in.hexNum(target)
}

func disFP(in *insn, target, rs, rt, rd, sa, funct uint32) bool {
	switch rs {
	case 0, 4:
		if rs == 0 {
			in.mn("mfc1")
		} else {
			in.mn("mtc1")
		}
		in.regs(int32(rt), -1, -1)
		in.opc(',')
		in.fpr(rd)
		return false
	case 8:
		in.mn("bc1")
		in.bc(rt, target)
		return true
	}
	switch {
	case fpOps[funct] != "":
		in.mn(fpOps[funct])
	case funct >= 0x30:
		in.mn("fcmp")
		in.hex1(&in.mnem, &in.m, funct)
	default:
		in.mn("fop")
		in.hex1(&in.mnem, &in.m, funct>>4)
		in.hex1(&in.mnem, &in.m, funct)
	}
	in.mn(".")
	if f := fpFormats[rs&15]; f != "" {
		in.mn(f)
	} else {
		in.mn("fmt")
		in.hex1(&in.mnem, &in.m, rs)
	}
	in.fpr(sa)
	in.opc(',')
	in.fpr(rd)
	in.opc(',')
	in.fpr(rt)
	return false
}

// putInsn shows an instruction as dis does: its address and word, then its
// mnemonic and operands, each at the next of the tab stops every eight
// columns.
func putInsn(pc uint32) bool {
	in := &decoded
	*in = insn{}
	w := load(pc, 4)
	branch := disassemble(in, pc, w)
	line.putHex(pc)
	line.putc(':')
	line.putHex(w)
	line.puts("       ") // from column 17 to 24
	line.putBytes(in.mnem[:in.m])
	if in.o > 0 {
		for col := 24 + in.m; ; {
			line.putc(' ')
			col++
			if col%8 == 0 {
				break
			}
		}
		line.putBytes(in.ops[:in.o])
	}
	line.puts("\r\n")
	return branch
}

// disNext is where dis goes on from without an address.
var disNext uint32

// dis is di and dis: as many instructions as the count says, ten if it is
// not given, from the address or where the last stopped. A jump or branch
// is shown with its delay slot, and the slot's own if that is one too, and
// the next instruction counted is the one after the first slot.
func dis(args [][]byte) {
	pc := disNext
	if len(args) > 0 {
		pc = hex(args[0]) &^ 3
	}
	n := uint32(10)
	if len(args) > 1 {
		n = hex(args[1])
	}
	for range n {
		branch := putInsn(pc)
		for a := pc + 4; branch; a += 4 {
			branch = putInsn(a)
		}
		if putsBranch(pc) {
			pc += 8
		} else {
			pc += 4
		}
	}
	disNext = pc
}

// putsBranch reports whether the instruction at pc is a jump or branch.
func putsBranch(pc uint32) bool {
	decoded = insn{}
	return disassemble(&decoded, pc, load(pc, 4))
}

// decoded is where instructions are decoded, one at a time: kept rather
// than made for each, which would take memory the heap never gives back.
var decoded insn
