// Package gteprog builds a program that exercises the GTE, for comparing an
// emulator's GTE with another's.
//
// For each case the program writes a value to every writable GTE register,
// runs one command, and stores all 64 registers to RAM. The cases cover
// every command with each combination of its shift and saturation bits,
// and MVMVA with every combination of matrix, vector and translation, with
// register values drawn at several scales so that both the saturating and
// the ordinary paths are taken. The values come from a fixed seed, so the
// program is the same every time it is built.
package gteprog

import (
	"encoding/binary"
	"math/rand/v2"

	"go.chrisrx.dev/psx/format/psx"
)

// Addresses, in KSEG0.
const (
	CodeAddr   = 0x80010000
	inputAddr  = 0x80080000
	OutputAddr = 0x80100000

	// Regs is how many registers each case stores: all 32 data and 32
	// control registers, a word each.
	Regs = 64
)

// Program is the built program.
type Program struct {
	// Code is the code and its input tables, from CodeAddr.
	Code []byte
	// Entry is where the program starts.
	Entry uint32
	// Done is the address the program loops at once it has finished.
	Done uint32
	// Cases is how many cases it runs; their results are Regs words
	// each, from OutputAddr.
	Cases int
}

// Commands, by function field, with whether they have the sf and lm bits.
var commands = []uint32{
	0x01, 0x06, 0x0c, 0x10, 0x11, 0x13, 0x14, 0x16, 0x1b, 0x1c, 0x1e,
	0x20, 0x28, 0x29, 0x2a, 0x2d, 0x2e, 0x30, 0x3d, 0x3e, 0x3f,
}

const mvmva = 0x12

// skipped are the data registers the program does not write: SXYP, whose
// write pushes the screen coordinate FIFO; IRGB, whose write replaces
// IR1-IR3; and the read-only ORGB and LZCR.
var skipped = map[int]bool{15: true, 28: true, 29: true, 31: true}

// Instruction encodings.
const (
	t0 = 8
	a0 = 4
	a1 = 5
	ra = 31
)

func lui(rt, imm uint32) uint32      { return 0x0f<<26 | rt<<16 | imm&0xffff }
func ori(rt, rs, imm uint32) uint32  { return 0x0d<<26 | rs<<21 | rt<<16 | imm&0xffff }
func lw(rt, off, base uint32) uint32 { return 0x23<<26 | base<<21 | rt<<16 | off&0xffff }
func sw(rt, off, base uint32) uint32 { return 0x2b<<26 | base<<21 | rt<<16 | off&0xffff }
func jal(target uint32) uint32       { return 0x03<<26 | target>>2&0x3ffffff }
func j(target uint32) uint32         { return 0x02<<26 | target>>2&0x3ffffff }
func jr(rs uint32) uint32            { return rs<<21 | 0x08 }

// cop2 moves: mfc2, cfc2, mtc2 and ctc2, by the rs field.
func cop2Move(rs, rt, rd uint32) uint32 { return 0x12<<26 | rs<<21 | rt<<16 | rd<<11 }

const nop = 0

// Build builds the program.
func Build() *Program {
	type testCase struct {
		op     uint32
		inputs [Regs]uint32
	}
	rng := rand.New(rand.NewPCG(0x5053_5847, 0x4745_5445))

	var cases []testCase
	add := func(op uint32) {
		cases = append(cases, testCase{op: 0x4a000000 | op, inputs: inputs(rng)})
	}
	for _, fn := range commands {
		for _, sf := range []uint32{0, 1} {
			for _, lm := range []uint32{0, 1} {
				for range 3 {
					add(sf<<19 | lm<<10 | fn)
				}
			}
		}
	}
	for mx := range uint32(4) {
		for v := range uint32(4) {
			for cv := range uint32(4) {
				for _, sf := range []uint32{0, 1} {
					for _, lm := range []uint32{0, 1} {
						add(sf<<19 | mx<<17 | v<<15 | cv<<13 | lm<<10 | mvmva)
					}
				}
			}
		}
	}

	// The code: the two subroutines, then the cases, then the loop to stop
	// in. The input tables go at inputAddr.
	var code []uint32
	pc := func() uint32 { return CodeAddr + uint32(4*len(code)) }

	// load: GTE registers from the table at a0.
	load := pc()
	for r := range uint32(Regs) {
		if r < 32 && skipped[int(r)] {
			continue
		}
		code = append(code, lw(t0, 4*r, a0), nop)
		if r < 32 {
			code = append(code, cop2Move(4, t0, r)) // mtc2
		} else {
			code = append(code, cop2Move(6, t0, r-32)) // ctc2
		}
	}
	code = append(code, jr(ra), nop)

	// store: every GTE register to the table at a1.
	store := pc()
	for r := range uint32(Regs) {
		if r < 32 {
			code = append(code, cop2Move(0, t0, r)) // mfc2
		} else {
			code = append(code, cop2Move(2, t0, r-32)) // cfc2
		}
		code = append(code, nop, sw(t0, 4*r, a1))
	}
	code = append(code, jr(ra), nop)

	entry := pc()
	for i, c := range cases {
		in := inputAddr + uint32(i*Regs*4)
		out := OutputAddr + uint32(i*Regs*4)
		code = append(code,
			lui(a0, in>>16), ori(a0, a0, in),
			jal(load), nop,
			c.op,
			lui(a1, out>>16), ori(a1, a1, out),
			jal(store), nop,
		)
	}
	done := pc()
	code = append(code, j(done), nop)

	size := inputAddr - CodeAddr + len(cases)*Regs*4
	buf := make([]byte, size)
	for i, insn := range code {
		binary.LittleEndian.PutUint32(buf[4*i:], insn)
	}
	for i, c := range cases {
		for r, v := range c.inputs {
			binary.LittleEndian.PutUint32(buf[inputAddr-CodeAddr+(i*Regs+r)*4:], v)
		}
	}
	return &Program{Code: buf, Entry: entry, Done: done, Cases: len(cases)}
}

// inputs draws a value for every register, at one of several scales: full
// 32-bit values, which mostly saturate, and smaller ones, packed as two
// 16-bit halves, which mostly do not.
func inputs(rng *rand.Rand) [Regs]uint32 {
	var in [Regs]uint32
	limit := []int32{0, 0x7fff, 0x1000, 0x100}[rng.IntN(4)]
	half := func() uint32 {
		return uint32(uint16(rng.Int32N(2*limit+1) - limit))
	}
	for r := range in {
		if limit == 0 {
			in[r] = rng.Uint32()
		} else {
			in[r] = half() | half()<<16
		}
	}
	return in
}

// EXE wraps the program as a PSX-EXE.
func (p *Program) EXE() *psx.File {
	data := p.Code
	if pad := len(data) % 2048; pad != 0 {
		data = append(data, make([]byte, 2048-pad)...)
	}
	exe := psx.New(CodeAddr, p.Entry, data)
	exe.StackAddr = 0x801ffff0
	return exe
}
