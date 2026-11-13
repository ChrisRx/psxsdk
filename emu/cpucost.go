package emu

// The CPU charges 2 cycles for every instruction, as psxe does, where a
// console's instructions cost from 1 cycle, running from the instruction
// cache, to 33, loading from the BIOS ROM. Under SetCPUCost the CPU charges
// what a console's do instead, as measured on one (cpu.CPU.SetCost says
// how), so that a test can see how long a library's code takes.
//
// A DMA holds the bus while it runs, which stops the CPU at its next access
// to anything but the instruction cache and the scratchpad. Channel 6 (OTC)
// holds it for its whole transfer. An image going to the GPU by channel 2
// in blocks, under SetDrawTime, which gives it its time, holds it for the
// start of each block (cpu.CPU.HoldBlocks). The other transfers still take
// the CPU no time: a packet list, an image from the GPU, and the MDEC,
// CD-ROM and SPU channels.

// otcHoldPerMille is the cycles, in thousandths, a word of channel 6's
// transfer holds the bus under SetCPUCost: clearing 1,024 words takes 1,097
// cycles on a console (exp/cleanlib/spec/35-hardware-session.md, cpucost2,
// "otc alone").
const otcHoldPerMille = 1070

// SetCPUCost sets whether the CPU charges what a console's instructions
// cost, as described above. It is off unless a test turns it on, and with
// it off nothing about the emulation changes. Turning it on starts with the
// instruction cache empty.
func (m *Machine) SetCPUCost(on bool) { m.CPU.SetCost(on) }

// imageOpenTenths is the tenths of a cycle of each block of an image going
// to the GPU for which the bus is open to the CPU under SetCPUCost: 6
// cycles, whatever the block's size (cpu.CPU.HoldBlocks has the console's
// figures, for blocks of 16 words; other sizes were not measured). The
// console's figures put it between a port read's 5 cycles and a load's 6.
// At 6 a second load from RAM, which could start only as the block ends,
// never does, and a second read of a port always does, whatever part of a
// cycle the block's start falls on.
const imageOpenTenths = 60
