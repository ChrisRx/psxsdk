package cpu

import "math/bits"

// Instruction fields.
func (c *CPU) rs() uint32    { return (c.Opcode >> 21) & 0x1f }
func (c *CPU) rt() uint32    { return (c.Opcode >> 16) & 0x1f }
func (c *CPU) rd() uint32    { return (c.Opcode >> 11) & 0x1f }
func (c *CPU) shamt() uint32 { return (c.Opcode >> 6) & 0x1f }
func (c *CPU) imm() uint32   { return c.Opcode & 0xffff }
func (c *CPU) simm() uint32  { return uint32(int32(int16(c.Opcode))) }

// execute runs an instruction Step does not run itself, and returns what it
// cost. ok is false for an instruction psxe does not recognise, which Step
// turns into a reserved instruction exception.
//
// Step runs the frequent instructions, shifts, ALU operations, branches,
// immediates, lui, loads and stores, in its own switch, so that they cost
// no calls; this is everything else. Every instruction costs 2 cycles
// beyond the fetch, except the GTE commands, which cost what Command says.
func (c *CPU) execute() (cycles uint32, ok bool) {
	switch c.Opcode >> 26 {
	case 0x00:
		switch c.Opcode & 0x3f {
		case 0x08:
			c.opJR()
		case 0x09:
			c.opJALR()
		case 0x0c:
			c.pendingLoad()
			c.exception(ExcSyscall)
		case 0x0d:
			c.pendingLoad()
			c.exception(ExcBp)
		case 0x10:
			c.pendingLoad()
			c.R[c.rd()] = c.HI
		case 0x11:
			c.pendingLoad()
			c.HI = c.R[c.rs()]
		case 0x12:
			c.pendingLoad()
			c.R[c.rd()] = c.LO
		case 0x13:
			c.pendingLoad()
			c.LO = c.R[c.rs()]
		case 0x18:
			c.opMult()
		case 0x19:
			c.opMultu()
		case 0x1a:
			c.opDiv()
		case 0x1b:
			c.opDivu()
		case 0x20:
			c.opAdd()
		case 0x22:
			c.opSub()
		default:
			return 0, false
		}
	case 0x01:
		c.opRegimm()
	case 0x02:
		c.opJ(false)
	case 0x03:
		c.opJ(true)
	case 0x08:
		c.opAddi()
	case 0x10:
		switch c.rs() {
		case 0x00: // mfc0
			c.pendingLoad()
			c.loadV = c.COP0[c.rd()&0xf]
			c.loadD = c.rt()
		case 0x04: // mtc0
			t := c.R[c.rt()]
			c.pendingLoad()
			d := c.rd() & 0xf
			c.COP0[d] = t & cop0WriteMask[d]
		case 0x10: // rfe
			c.pendingLoad()
			mode := c.COP0[SR] & 0x3f
			c.COP0[SR] = c.COP0[SR]&0xfffffff0 | mode>>2
		default:
			return 0, false
		}
	case 0x12:
		switch c.rs() {
		case 0x00: // mfc2
			c.pendingLoad()
			c.loadV = c.GTE.Read(c.rd())
			c.loadD = c.rt()
		case 0x02: // cfc2
			c.pendingLoad()
			c.loadV = c.GTE.Read(c.rd() + 32)
			c.loadD = c.rt()
		case 0x04: // mtc2
			t := c.R[c.rt()]
			c.pendingLoad()
			c.GTE.Write(c.rd(), t)
		case 0x06: // ctc2
			t := c.R[c.rt()]
			c.pendingLoad()
			c.GTE.Write(c.rd()+32, t)
		default:
			c.pendingLoad()
			n := c.GTE.Command(c.Opcode)
			if n == 0 {
				return 0, false
			}
			return n, true
		}
	case 0x22:
		c.opLWL()
	case 0x26:
		c.opLWR()
	case 0x2a:
		c.opSWL()
	case 0x2e:
		c.opSWR()
	case 0x30, 0x31, 0x33, 0x38, 0x39, 0x3b:
		// lwc0, lwc1, lwc3, swc0, swc1, swc3. psxe does not apply a
		// pending load first.
		c.exception(ExcCpU)
	case 0x32:
		c.opLWC2()
	case 0x3a:
		c.opSWC2()
	default:
		return 0, false
	}
	return 2, true
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// Every instruction reads its operands, then lets a pending load land, then
// writes its result, so an instruction in a load's delay slot sees the old
// value and its own result wins over the load's.

// aluOp is addu, subu, and, or, xor, nor, slt and sltu, by function field.
func aluOp(funct, s, t uint32) uint32 {
	switch funct {
	case 0x21:
		return s + t
	case 0x23:
		return s - t
	case 0x24:
		return s & t
	case 0x25:
		return s | t
	case 0x26:
		return s ^ t
	case 0x27:
		return ^(s | t)
	case 0x2a:
		return b2u(int32(s) < int32(t))
	}
	return b2u(s < t) // 0x2b
}

// immOp is addiu, slti, sltiu, andi, ori and xori, by opcode, on rs's value
// and the instruction's immediate, sign-extended or not as each takes it.
func immOp(op, s, insn uint32) uint32 {
	simm, imm := uint32(int32(int16(insn))), insn&0xffff
	switch op {
	case 0x09:
		return s + simm
	case 0x0a:
		return b2u(int32(s) < int32(simm))
	case 0x0b:
		return b2u(s < simm)
	case 0x0c:
		return s & imm
	case 0x0d:
		return s | imm
	}
	return s ^ imm // 0x0e
}

// shiftOp is sll, srl and sra, by function field, with bit 2, which picks
// the variable forms, already taken into account in s.
func shiftOp(funct, t, s uint32) uint32 {
	switch funct & 3 {
	case 0:
		return t << s
	case 2:
		return t >> s
	}
	return uint32(int32(t) >> s) // 3
}

// branchCond is whether beq, bne, blez or bgtz, by opcode, is taken.
func branchCond(op, s, t uint32) bool {
	switch op {
	case 0x04:
		return s == t
	case 0x05:
		return s != t
	case 0x06:
		return int32(s) <= 0
	}
	return int32(s) > 0 // 0x07
}

func (c *CPU) opAdd() {
	s, t := c.R[c.rs()], c.R[c.rt()]
	c.pendingLoad()
	r := s + t
	if (s^r)&(t^r)&0x80000000 != 0 {
		c.exception(ExcOv)
		return
	}
	c.R[c.rd()] = r
}

func (c *CPU) opSub() {
	s, t := c.R[c.rs()], c.R[c.rt()]
	c.pendingLoad()
	r := s - t
	if (s^t)&(s^r)&0x80000000 != 0 {
		c.exception(ExcOv)
		return
	}
	c.R[c.rd()] = r
}

func (c *CPU) opAddi() {
	s := c.R[c.rs()]
	c.pendingLoad()
	i := c.simm()
	r := s + i
	if (s^r)&(i^r)&0x80000000 != 0 {
		c.exception(ExcOv)
		return
	}
	c.R[c.rt()] = r
}

func (c *CPU) opMult() {
	s, t := int64(int32(c.R[c.rs()])), int64(int32(c.R[c.rt()]))
	c.pendingLoad()
	r := uint64(s * t)
	c.HI, c.LO = uint32(r>>32), uint32(r)
}

func (c *CPU) opMultu() {
	s, t := c.R[c.rs()], c.R[c.rt()]
	c.pendingLoad()
	hi, lo := bits.Mul32(s, t)
	c.HI, c.LO = hi, lo
}

func (c *CPU) opDiv() {
	s, t := int32(c.R[c.rs()]), int32(c.R[c.rt()])
	c.pendingLoad()
	switch {
	case t == 0:
		c.HI = uint32(s)
		if s >= 0 {
			c.LO = 0xffffffff
		} else {
			c.LO = 1
		}
	case uint32(s) == 0x80000000 && t == -1:
		c.HI, c.LO = 0, 0x80000000
	default:
		c.HI, c.LO = uint32(s%t), uint32(s/t)
	}
}

func (c *CPU) opDivu() {
	s, t := c.R[c.rs()], c.R[c.rt()]
	c.pendingLoad()
	if t == 0 {
		c.HI, c.LO = s, 0xffffffff
		return
	}
	c.HI, c.LO = s%t, s/t
}

// branchTo jumps relative to the delay slot. PC has already moved on to the
// delay slot by the time an instruction executes, and NextPC past it.
func (c *CPU) branchTo(offset uint32) {
	c.NextPC = c.NextPC + offset - 4
	c.branch = true
	c.branchTaken = true
}

// opRegimm is bltz, bgez, bltzal and bgezal. Bit 16 picks bgez over bltz,
// and rt 0x10 or 0x11 adds the link, which happens whether or not the branch
// is taken. psxe treats every other rt as plain bltz or bgez.
func (c *CPU) opRegimm() {
	c.branch = true
	c.branchTaken = false

	rt := c.rt()
	s := int32(c.R[c.rs()])
	c.pendingLoad()

	if rt == 0x10 || rt == 0x11 {
		c.R[31] = c.NextPC
	}
	if ge := rt&1 != 0; (ge && s >= 0) || (!ge && s < 0) {
		c.branchTo(c.simm() << 2)
	}
}

func (c *CPU) opJ(link bool) {
	c.branch = true
	c.pendingLoad()
	if link {
		c.R[31] = c.NextPC
	}
	c.NextPC = c.NextPC&0xf0000000 | (c.Opcode&0x3ffffff)<<2
}

func (c *CPU) opJR() {
	c.branch = true
	s := c.R[c.rs()]
	c.pendingLoad()
	c.NextPC = s
}

func (c *CPU) opJALR() {
	c.branch = true
	s := c.R[c.rs()]
	c.pendingLoad()
	c.R[c.rd()] = c.NextPC
	c.NextPC = s
}

// accessSize is the bytes a load or store accesses, by the low two bits of
// its opcode.
var accessSize = [4]uint32{1, 2, 0, 4}

// opLWL and opLWR merge part of a word into the register's current value,
// or into the value of a load still pending for it, which is how a pair of
// them loads an unaligned word with no delay between them.
func (c *CPU) opLWL() {
	rt := c.rt()
	s := c.R[c.rs()]
	t := c.R[rt]
	addr := s + c.simm()
	load := c.bus.Read32(addr &^ 3)
	if rt == c.loadD {
		t = c.loadV
	} else {
		c.pendingLoad()
	}
	shift := (addr & 3) << 3
	mask := uint32(0x00ffffff) >> shift
	c.loadD = rt
	c.loadV = t&mask | load<<(24-shift)
}

func (c *CPU) opLWR() {
	rt := c.rt()
	s := c.R[c.rs()]
	t := c.R[rt]
	addr := s + c.simm()
	load := c.bus.Read32(addr &^ 3)
	if rt == c.loadD {
		t = c.loadV
	} else {
		c.pendingLoad()
	}
	shift := (addr & 3) << 3
	mask := uint32(0xffffff00) << (24 - shift)
	c.loadD = rt
	c.loadV = t&mask | load>>shift
}

// opSWL and opSWR differ from the other stores in psxe in two ways: they read
// rt after the pending load has landed rather than before, and they write
// through even while the cache is isolated.
func (c *CPU) opSWL() {
	s := c.R[c.rs()]
	c.pendingLoad()
	addr := s + c.simm()
	aligned := addr &^ 3
	v := c.bus.Read32(aligned)
	t := c.R[c.rt()]
	switch addr & 3 {
	case 0:
		v = v&0xffffff00 | t>>24
	case 1:
		v = v&0xffff0000 | t>>16
	case 2:
		v = v&0xff000000 | t>>8
	case 3:
		v = t
	}
	c.bus.Write32(aligned, v)
}

func (c *CPU) opSWR() {
	s := c.R[c.rs()]
	c.pendingLoad()
	addr := s + c.simm()
	aligned := addr &^ 3
	v := c.bus.Read32(aligned)
	t := c.R[c.rt()]
	switch addr & 3 {
	case 0:
		v = t
	case 1:
		v = v&0x000000ff | t<<8
	case 2:
		v = v&0x0000ffff | t<<16
	case 3:
		v = v&0x00ffffff | t<<24
	}
	c.bus.Write32(aligned, v)
}

func (c *CPU) opLWC2() {
	addr := c.R[c.rs()] + c.simm()
	c.pendingLoad()
	if addr&3 != 0 {
		c.exception(ExcAdEL)
		return
	}
	c.GTE.Write(c.rt(), c.bus.Read32(addr))
}

func (c *CPU) opSWC2() {
	addr := c.R[c.rs()] + c.simm()
	c.pendingLoad()
	if c.COP0[SR]&srIsc != 0 {
		return
	}
	if addr&3 != 0 {
		c.exception(ExcAdES)
		return
	}
	c.bus.Write32(addr, c.GTE.Read(c.rt()))
}
