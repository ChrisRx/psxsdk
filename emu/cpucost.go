package emu

// The CPU charges 2 cycles for every instruction, as psxe does, where a
// console's instructions cost from 1 cycle, running from the instruction
// cache, to 33, loading from the BIOS ROM. Under SetCPUCost the CPU charges
// what a console's do instead, as measured on one (cpu.CPU.SetCost says
// how), so that a test can see how long a library's code takes.
//
// A DMA holds the bus while it runs, which stops the CPU at its next access
// to anything but the instruction cache and the scratchpad. Only channel 6
// (OTC) was measured, and only it holds the bus: the GPU, MDEC, CD-ROM and
// SPU channels still take no time.

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
