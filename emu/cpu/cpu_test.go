package cpu

import (
	"encoding/binary"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// testBus is 2MB of RAM at physical 0 and 64KB of ROM at physical
// 1FC00000, where the CPU starts.
type testBus struct {
	ram [2 << 20]byte
	rom [64 << 10]byte
}

func (b *testBus) at(addr uint32) []byte {
	addr &= 0x1fffffff
	if addr >= 0x1fc00000 {
		return b.rom[(addr-0x1fc00000)&(uint32(len(b.rom))-1):]
	}
	return b.ram[addr&(uint32(len(b.ram))-1):]
}

func (b *testBus) Read32(a uint32) uint32     { return binary.LittleEndian.Uint32(b.at(a)) }
func (b *testBus) Read16(a uint32) uint16     { return binary.LittleEndian.Uint16(b.at(a)) }
func (b *testBus) Read8(a uint32) uint8       { return b.at(a)[0] }
func (b *testBus) Write32(a, v uint32)        { binary.LittleEndian.PutUint32(b.at(a), v) }
func (b *testBus) Write16(a uint32, v uint16) { binary.LittleEndian.PutUint16(b.at(a), v) }
func (b *testBus) Write8(a uint32, v uint8)   { b.at(a)[0] = v }

func (b *testBus) Fetch(a uint32) (insn, cycles uint32) { return b.Read32(a), 0 }

// run loads a program at the reset vector and steps through it.
func run(t *testing.T, bus *testBus, steps int, program ...uint32) *CPU {
	t.Helper()
	for i, insn := range program {
		binary.LittleEndian.PutUint32(bus.rom[4*i:], insn)
	}
	c := New(bus)
	for range steps {
		c.Step()
	}
	return c
}

// Register numbers.
const (
	zero = 0
	t0   = 8
	t1   = 9
	t2   = 10
	t3   = 11
)

func iType(op, rs, rt uint32, imm int16) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(uint16(imm))
}

func rType(rs, rt, rd, funct uint32) uint32 {
	return rs<<21 | rt<<16 | rd<<11 | funct
}

func addiu(rt, rs uint32, imm int16) uint32        { return iType(0x09, rs, rt, imm) }
func addi(rt, rs uint32, imm int16) uint32         { return iType(0x08, rs, rt, imm) }
func lui(rt uint32, imm uint16) uint32             { return iType(0x0f, 0, rt, int16(imm)) }
func ori(rt, rs uint32, imm uint16) uint32         { return iType(0x0d, rs, rt, int16(imm)) }
func lw(rt uint32, off int16, base uint32) uint32  { return iType(0x23, base, rt, off) }
func lwl(rt uint32, off int16, base uint32) uint32 { return iType(0x22, base, rt, off) }
func lwr(rt uint32, off int16, base uint32) uint32 { return iType(0x26, base, rt, off) }
func beq(rs, rt uint32, off int16) uint32          { return iType(0x04, rs, rt, off) }
func addu(rd, rs, rt uint32) uint32                { return rType(rs, rt, rd, 0x21) }
func j(target uint32) uint32                       { return 0x02<<26 | (target>>2)&0x3ffffff }

const (
	nop     = 0
	syscall = 0x0000000c
)

func TestLoadDelaySlot(t *testing.T) {
	bus := &testBus{}
	binary.LittleEndian.PutUint32(bus.ram[0x100:], 0x12345678)

	c := run(t, bus, 4,
		addiu(t1, zero, 1),
		lw(t1, 0x100, zero),
		addu(t2, t1, zero), // in the delay slot: sees the old value
		addu(t3, t1, zero), // after it: sees the loaded one
	)
	assert.Equal(t, uint32(1), c.R[t2], "the instruction in the load delay slot")
	assert.Equal(t, uint32(0x12345678), c.R[t3], "the instruction after the delay slot")
}

func TestBranchDelaySlot(t *testing.T) {
	c := run(t, &testBus{}, 3,
		beq(zero, zero, 2),
		addiu(t0, zero, 1), // delay slot: runs
		addiu(t1, zero, 1), // skipped
		addiu(t2, zero, 1), // the target
	)
	assert.Equal(t, [3]uint32{1, 0, 1}, [3]uint32{c.R[t0], c.R[t1], c.R[t2]})
}

func TestSyscall(t *testing.T) {
	bus := &testBus{}
	binary.LittleEndian.PutUint32(bus.rom[0:], syscall)
	c := New(bus)
	c.COP0[SR] |= srIEc
	c.Step()

	assert.Equal(t, struct {
		Excode      uint8
		PC, EPC     uint32
		Cause, Mode uint32
	}{
		Excode: ExcSyscall,
		PC:     0x80000080,
		EPC:    0xbfc00000,
		Cause:  ExcSyscall << 2,
		Mode:   0x04, // interrupts were enabled, and that was pushed
	}, struct {
		Excode      uint8
		PC, EPC     uint32
		Cause, Mode uint32
	}{c.Excode, c.PC, c.COP0[EPC], c.COP0[Cause], c.COP0[SR] & 0x3f})
}

func TestExceptionInDelaySlot(t *testing.T) {
	c := run(t, &testBus{}, 2,
		j(0xbfc00100),
		syscall,
	)
	assert.Equal(t, uint32(0xbfc00000), c.COP0[EPC], "EPC should point at the jump, so the jump is rerun")
	assert.Equal(t, uint32(0x80000000), c.COP0[Cause]&0x80000000, "Cause should have BD set")
}

func TestInterruptReplacesInstruction(t *testing.T) {
	bus := &testBus{}
	binary.LittleEndian.PutUint32(bus.rom[0:], addiu(t0, zero, 1))
	c := New(bus)
	c.COP0[SR] |= srIEc | srIM2
	c.SetIRQ(true)
	c.Step()

	assert.Equal(t, uint8(ExcInt), c.Excode)
	assert.Equal(t, uint32(0), c.R[t0], "the interrupted instruction should not have run")
	assert.Equal(t, uint32(0xbfc00000), c.COP0[EPC])
}

func TestUnalignedLoadPair(t *testing.T) {
	bus := &testBus{}
	copy(bus.ram[0x100:], []byte{0, 1, 2, 3, 4, 5, 6, 7})

	c := run(t, bus, 4,
		addiu(t1, zero, 0x100),
		lwr(t0, 1, t1),
		lwl(t0, 4, t1), // merges into the lwr's pending result
		nop,
	)
	assert.Equal(t, uint32(0x04030201), c.R[t0])
}

func TestOverflow(t *testing.T) {
	c := run(t, &testBus{}, 4,
		lui(t0, 0x7fff),
		ori(t0, t0, 0xffff), // 0x7fffffff
		addiu(t1, zero, 5),
		addi(t1, t0, 1),
	)
	assert.Equal(t, uint8(ExcOv), c.Excode)
	assert.Equal(t, uint32(5), c.R[t1], "rt should be left as it was")

	c = run(t, &testBus{}, 3,
		lui(t0, 0x7fff),
		ori(t0, t0, 0xfffe),
		addi(t1, t0, 1),
	)
	assert.Equal(t, uint8(NoException), c.Excode)
	assert.Equal(t, uint32(0x7fffffff), c.R[t1])
}

func TestLZCR(t *testing.T) {
	for _, tc := range []struct {
		lzcs uint32
		lzcr uint32
	}{
		{0x00ffffff, 8},
		{0xff000000, 8},
		{0x00000001, 31},
		{0x00000000, 32},
		{0xffffffff, 32},
	} {
		var g GTE
		g.Write(30, tc.lzcs)
		assert.Equal(t, tc.lzcr, g.Read(31), "LZCS %#08x", tc.lzcs)
	}
}
