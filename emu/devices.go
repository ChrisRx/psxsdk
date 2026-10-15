package emu

import "io"

// ram is main memory: 2MB, mirrored through the 8MB window psxe maps it
// in. When RAM_SIZE says the window holds 8MB of which only the first 4MB
// is populated, reads above 4MB return all ones.
type ram struct {
	memory
	mc2 *mc2
}

func (r *ram) unpopulated(off uint32) bool {
	return off >= 0x400000 && (r.mc2.ramSize>>9)&7 == 3
}

func (r *ram) Read32(off uint32) uint32 {
	if r.unpopulated(off) {
		return 0xffffffff
	}
	return r.memory.Read32(off)
}

func (r *ram) Read16(off uint32) uint16 {
	if r.unpopulated(off) {
		return 0xffff
	}
	return r.memory.Read16(off)
}

func (r *ram) Read8(off uint32) uint8 {
	if r.unpopulated(off) {
		return 0xff
	}
	return r.memory.Read8(off)
}

// registers is a device of 32-bit registers that read back what was written
// to them. psxe ignores the narrower accesses, which read as zero.
type registers []uint32

func (r registers) Read32(off uint32) uint32 {
	if i := off / 4; off%4 == 0 && int(i) < len(r) {
		return r[i]
	}
	return 0
}

func (r registers) Write32(off, v uint32) {
	if i := off / 4; off%4 == 0 && int(i) < len(r) {
		r[i] = v
	}
}

func (registers) Read16(uint32) uint16   { return 0 }
func (registers) Read8(uint32) uint8     { return 0 }
func (registers) Write16(uint32, uint16) {}
func (registers) Write8(uint32, uint8)   {}

// newMC1 returns the memory control registers at 1F801000: the expansion
// base addresses and the delay/size of each region, at their usual values
// after reset.
func newMC1() registers {
	return registers{
		0x1f000000, // expansion 1 base
		0x1f802000, // expansion 2 base
		0x0013243f, // expansion 1 delay/size
		0x00003022, // expansion 3 delay/size
		0x0013243f, // BIOS delay/size
		0x200931e1, // SPU delay/size
		0x00020843, // CD-ROM delay/size
		0x00070777, // expansion 2 delay/size
		0x00031125, // common delay
	}
}

// mc2 is RAM_SIZE at 1F801060, which sets how RAM is laid out in its window.
type mc2 struct {
	registers
	ramSize uint32
}

func newMC2() *mc2 {
	m := &mc2{registers: make(registers, 1)}
	m.ramSize = 0x00000b88 // 2MB, mirrored through the first 8MB
	m.registers[0] = m.ramSize
	return m
}

func (m *mc2) Write32(off, v uint32) {
	m.registers.Write32(off, v)
	if off == 0 {
		m.ramSize = v
	}
}

// exp2 is expansion region 2, where a development board's serial console
// and the BIOS's POST code port sit. psxe gives it a console with nothing to
// receive, and drops what the BIOS sends.
type exp2 struct {
	atcStat, atcRx uint8
	tty            io.Writer
}

const (
	exp2ATCStat = 0x00
	exp2ATCData = 0x02
)

func (e *exp2) Read32(uint32) uint32 { return 0 }
func (e *exp2) Read16(uint32) uint16 { return 0 }

func (e *exp2) Read8(off uint32) uint8 {
	switch off {
	case exp2ATCStat:
		return e.atcStat | 8
	case exp2ATCData:
		e.atcStat &= 0xef
		return e.atcRx
	}
	return 0
}

func (e *exp2) Write32(uint32, uint32) {}
func (e *exp2) Write16(uint32, uint16) {}

func (e *exp2) Write8(off uint32, v uint8) {
	if off == exp2ATCData && e.tty != nil {
		e.tty.Write([]byte{v})
	}
}

// ic is the interrupt controller at 1F801070: I_STAT, the interrupts raised
// and not yet acknowledged, and I_MASK, the ones allowed through to the CPU.
type ic struct {
	stat, mask uint32
	setIRQ     func(bool)
}

// Interrupt sources, as I_STAT bits.
const (
	irqVBlank = 1 << 0
	irqGPU    = 1 << 1
	irqCDROM  = 1 << 2
	irqDMA    = 1 << 3
	irqTimer0 = 1 << 4
	irqTimer1 = 1 << 5
	irqTimer2 = 1 << 6
	irqPad    = 1 << 7
	irqSIO    = 1 << 8
	irqSPU    = 1 << 9
)

// raise sets an interrupt's bit in I_STAT. The CPU's line goes up if the
// interrupt is unmasked; it only comes down again when the program writes
// I_STAT or I_MASK.
func (c *ic) raise(irq uint32) {
	c.stat |= irq
	if c.mask&c.stat != 0 {
		c.setIRQ(true)
	}
}

// update recomputes the CPU's line after a write.
func (c *ic) update() {
	c.setIRQ(c.stat&c.mask != 0)
}

func (c *ic) Read32(off uint32) uint32 {
	switch off {
	case 0:
		return c.stat
	case 4:
		return c.mask
	}
	return 0
}

func (c *ic) Read16(off uint32) uint16 {
	switch off {
	case 0:
		return uint16(c.stat)
	case 2:
		return uint16(c.stat >> 16)
	case 4:
		return uint16(c.mask)
	case 6:
		return uint16(c.mask >> 16)
	}
	return 0
}

func (c *ic) Read8(off uint32) uint8 {
	switch {
	case off < 4:
		return uint8(c.stat >> (8 * off))
	case off < 8:
		return uint8(c.mask >> (8 * (off - 4)))
	}
	return 0
}

func (c *ic) Write32(off, v uint32) {
	switch off {
	case 0:
		c.stat &= v // writing 0 to a bit acknowledges it
	case 4:
		c.mask = v
	}
	c.update()
}

// The narrower writes are psxe's: the value is shifted into place but the
// rest of the register is not preserved, so acknowledging through the upper
// half of I_STAT also clears the lower half, and writing part of I_MASK
// clears the rest of it.

func (c *ic) Write16(off uint32, v uint16) {
	switch off {
	case 0, 2:
		c.stat &= uint32(v) << (8 * off)
	case 4, 6:
		c.mask = uint32(v) << (8 * (off - 4))
	}
	c.update()
}

func (c *ic) Write8(off uint32, v uint8) {
	switch {
	case off < 4:
		c.stat &= uint32(v) << (8 * off)
	case off < 8:
		c.mask = uint32(v) << (8 * (off - 4))
	}
	c.update()
}
