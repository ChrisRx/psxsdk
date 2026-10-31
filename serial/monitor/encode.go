package main

// MIPS registers and instructions, for the code the monitor writes: the
// kernel hook's dispatch, the thunks it calls the kernel through, and the
// debugger's exception handler and resume code.
const (
	zero, a0, t0, t1, t2 = 0, 4, 8, 9, 10
	k0, k1, sp           = 26, 27, 29
	nop                  = 0
)

// The coprocessor 0 registers the debugger saves.
const (
	cop0SR    = 12
	cop0Cause = 13
	cop0EPC   = 14
)

func addiu(rt, rs uint32, imm int32) uint32 { return 9<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff }
func lui(rt, imm uint32) uint32             { return 15<<26 | rt<<16 | imm&0xffff }
func jr(rs uint32) uint32                   { return rs<<21 | 8 }
func beq(rs, rt uint32, off int) uint32     { return 4<<26 | rs<<21 | rt<<16 | uint32(off)&0xffff }
func bne(rs, rt uint32, off int) uint32     { return 5<<26 | rs<<21 | rt<<16 | uint32(off)&0xffff }
func andi(rt, rs, imm uint32) uint32        { return 12<<26 | rs<<21 | rt<<16 | imm&0xffff }
func lw(rt, base uint32, off int32) uint32  { return 0x23<<26 | base<<21 | rt<<16 | uint32(off)&0xffff }
func sw(rt, base uint32, off int32) uint32  { return 0x2b<<26 | base<<21 | rt<<16 | uint32(off)&0xffff }
func addu(rd, rs, rt uint32) uint32         { return rs<<21 | rt<<16 | rd<<11 | 0x21 }
func mfhi(rd uint32) uint32                 { return rd<<11 | 0x10 }
func mthi(rs uint32) uint32                 { return rs<<21 | 0x11 }
func mflo(rd uint32) uint32                 { return rd<<11 | 0x12 }
func mtlo(rs uint32) uint32                 { return rs<<21 | 0x13 }
func mfc0(rt, rd uint32) uint32             { return 0x10<<26 | rt<<16 | rd<<11 }
func mtc0(rt, rd uint32) uint32             { return 0x10<<26 | 4<<21 | rt<<16 | rd<<11 }

const rfe = 0x42000010
