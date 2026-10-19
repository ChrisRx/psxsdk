package emu

import "fmt"

// LoadRAM copies data into RAM at addr, which may be given in any of RAM's
// segments: what siocons' "local dload" did for a Net Yaroze program's
// data.
func (m *Machine) LoadRAM(addr uint32, data []byte) error {
	off := addr & 0x1fffffff
	if int(off)+len(data) > len(m.ram.memory) {
		return fmt.Errorf("%d bytes at %08x do not fit in RAM", len(data), addr)
	}
	copy(m.ram.memory[off:], data)
	return nil
}
