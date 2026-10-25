package cpu

import (
	"encoding/binary"
	"math/rand/v2"

	"go.chrisrx.dev/psx/format/psx"
)

// The GTE exercise program, which TestGTE runs here and psxe runs to make
// testdata/gte.golden.
//
// For each case the program writes a value to every writable GTE register,
// runs one command, and stores all 64 registers to RAM. The cases cover
// every command with each combination of its shift and saturation bits,
// and MVMVA with every combination of matrix, vector and translation, with
// register values drawn at several scales so that both the saturating and
// the ordinary paths are taken. The values come from a fixed seed, so the
// program is the same every time it is built.

// Addresses, in KSEG0.
const (
	gteCodeAddr   = 0x80010000
	gteInputAddr  = 0x80080000
	gteOutputAddr = 0x80100000

	// gteRegs is how many registers each case stores: all 32 data and 32
	// control registers, a word each.
	gteRegs = 64
)

// gteProgram is the built program.
type gteProgram struct {
	// code is the code and its input tables, from gteCodeAddr.
	code []byte
	// entry is where the program starts.
	entry uint32
	// done is the address the program loops at once it has finished.
	done uint32
	// cases is how many cases it runs; their results are gteRegs words
	// each, from gteOutputAddr.
	cases int
}

// gteCommands are the commands other than MVMVA, by function field.
var gteCommands = []uint32{
	0x01, 0x06, 0x0c, 0x10, 0x11, 0x13, 0x14, 0x16, 0x1b, 0x1c, 0x1e,
	0x20, 0x28, 0x29, 0x2a, 0x2d, 0x2e, 0x30, 0x3d, 0x3e, 0x3f,
}

const mvmva = 0x12

// gteSkipped are the data registers the program does not write: SXYP,
// whose write pushes the screen coordinate FIFO; IRGB, whose write replaces
// IR1-IR3; and the read-only ORGB and LZCR.
var gteSkipped = map[int]bool{15: true, 28: true, 29: true, 31: true}

// Register numbers, beyond those in cpu_test.go.
const (
	a0 = 4
	a1 = 5
	ra = 31
)

func sw(rt uint32, off int16, base uint32) uint32 { return iType(0x2b, base, rt, off) }
func jal(target uint32) uint32                    { return 0x03<<26 | (target>>2)&0x3ffffff }
func jr(rs uint32) uint32                         { return rType(rs, 0, 0, 0x08) }

// cop2 moves: mfc2, cfc2, mtc2 and ctc2, by the rs field.
func cop2Move(rs, rt, rd uint32) uint32 { return 0x12<<26 | rs<<21 | rt<<16 | rd<<11 }

// buildGTEProgram builds the program.
func buildGTEProgram() *gteProgram {
	type testCase struct {
		op     uint32
		inputs [gteRegs]uint32
	}
	rng := rand.New(rand.NewPCG(0x5053_5847, 0x4745_5445))

	var cases []testCase
	add := func(op uint32) {
		cases = append(cases, testCase{op: 0x4a000000 | op, inputs: gteInputs(rng)})
	}
	for _, fn := range gteCommands {
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
	// in. The input tables go at gteInputAddr.
	var code []uint32
	pc := func() uint32 { return gteCodeAddr + uint32(4*len(code)) }

	// load: GTE registers from the table at a0.
	load := pc()
	for r := range uint32(gteRegs) {
		if r < 32 && gteSkipped[int(r)] {
			continue
		}
		code = append(code, lw(t0, int16(4*r), a0), nop)
		if r < 32 {
			code = append(code, cop2Move(4, t0, r)) // mtc2
		} else {
			code = append(code, cop2Move(6, t0, r-32)) // ctc2
		}
	}
	code = append(code, jr(ra), nop)

	// store: every GTE register to the table at a1.
	store := pc()
	for r := range uint32(gteRegs) {
		if r < 32 {
			code = append(code, cop2Move(0, t0, r)) // mfc2
		} else {
			code = append(code, cop2Move(2, t0, r-32)) // cfc2
		}
		code = append(code, nop, sw(t0, int16(4*r), a1))
	}
	code = append(code, jr(ra), nop)

	entry := pc()
	for i, c := range cases {
		in := gteInputAddr + uint32(i*gteRegs*4)
		out := gteOutputAddr + uint32(i*gteRegs*4)
		code = append(code,
			lui(a0, uint16(in>>16)), ori(a0, a0, uint16(in)),
			jal(load), nop,
			c.op,
			lui(a1, uint16(out>>16)), ori(a1, a1, uint16(out)),
			jal(store), nop,
		)
	}
	done := pc()
	code = append(code, j(done), nop)

	size := gteInputAddr - gteCodeAddr + len(cases)*gteRegs*4
	buf := make([]byte, size)
	for i, insn := range code {
		binary.LittleEndian.PutUint32(buf[4*i:], insn)
	}
	for i, c := range cases {
		for r, v := range c.inputs {
			binary.LittleEndian.PutUint32(buf[gteInputAddr-gteCodeAddr+(i*gteRegs+r)*4:], v)
		}
	}
	return &gteProgram{code: buf, entry: entry, done: done, cases: len(cases)}
}

// gteInputs draws a value for every register, at one of several scales:
// full 32-bit values, which mostly saturate, and smaller ones, packed as
// two 16-bit halves, which mostly do not.
func gteInputs(rng *rand.Rand) [gteRegs]uint32 {
	var in [gteRegs]uint32
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

// exe wraps the program as a PSX-EXE.
func (p *gteProgram) exe() *psx.File {
	data := p.code
	if pad := len(data) % 2048; pad != 0 {
		data = append(data, make([]byte, 2048-pad)...)
	}
	exe := psx.New(gteCodeAddr, p.entry, data)
	exe.StackAddr = 0x801ffff0
	return exe
}
