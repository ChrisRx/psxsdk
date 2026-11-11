package emu

// FillMemory fills RAM, the scratchpad and VRAM with bytes made from seed,
// for a machine about to boot. The emulator otherwise starts with all three
// cleared, where a console starts with whatever they held: a reset clears
// RAM from 800A0000h up and leaves the scratchpad and VRAM as they were
// (exp/cleanlib/spec/35-hardware-session.md). A program that reads memory it
// never wrote shows it, as different runs with different seeds. The same
// seed fills the same bytes.
func (m *Machine) FillMemory(seed uint64) {
	x := seed | 1
	next := func() uint64 {
		// xorshift64*
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x * 0x2545f4914f6cdd1d
	}
	fill := func(b []byte) {
		for i := 0; i < len(b); i += 8 {
			v := next()
			for j := 0; j < 8 && i+j < len(b); j++ {
				b[i+j] = byte(v >> (8 * j))
			}
		}
	}
	fill(m.ram.memory)
	fill(m.scratch)
	for i := range m.gpu.vram {
		m.gpu.vram[i] = uint16(next())
	}
}
