package emu

import (
	"errors"

	"go.chrisrx.dev/psx/emu/cpu"
)

// ErrBreak is what Run, RunFrame and Step return when a breakpoint's
// function stops the run. The machine is stopped before the instruction at
// the breakpoint, which the next run executes without stopping there again.
var ErrBreak = errors.New("stopped at a breakpoint")

// A Break is called before the instruction at its address runs, with the
// machine stopped there: the CPU's PC is the address, its registers are as
// the program left them, and the bus reads and writes memory. It returns
// whether to stop the run, which then returns ErrBreak.
//
// A breakpoint inside the vertical blank wait the machine skips through is
// passed over with the loop, as are those in the BIOS.
type Break func(m *Machine) (stop bool)

// breaks is the machine's breakpoints: the addresses, as the CPU tests
// them, and the function at each, by its address in RAM.
type breaks struct {
	set cpu.Breaks
	fns map[uint32]Break

	// resumeAt is the address a stop left the CPU at, and resuming that
	// the next run has not yet passed it, so that it does not stop there
	// again.
	resumeAt uint32
	resuming bool
}

// SetBreak sets a breakpoint: f is called before the instruction at addr
// runs, at any of the addresses RAM is seen through. A breakpoint already
// at addr is replaced. An address outside RAM is not kept.
func (m *Machine) SetBreak(addr uint32, f Break) {
	if addr&0x1fffffff >= RAMSize || f == nil {
		return
	}
	if m.breaks.fns == nil {
		m.breaks.fns = map[uint32]Break{}
	}
	m.breaks.set.Set(addr)
	m.breaks.fns[addr&0x1fffffff] = f
}

// ClearBreak takes away the breakpoint at addr, if there is one.
func (m *Machine) ClearBreak(addr uint32) {
	m.breaks.set.Clear(addr)
	delete(m.breaks.fns, addr&0x1fffffff)
}

// Breaks returns the addresses of the breakpoints set, in RAM's own
// segment, in no particular order.
func (m *Machine) Breaks() []uint32 {
	out := make([]uint32, 0, len(m.breaks.fns))
	for addr := range m.breaks.fns {
		out = append(out, addr|0x80000000)
	}
	return out
}

// atBreak calls the breakpoint at the CPU's PC, if there is one that the
// run has not just stopped at, and reports whether it asked to stop.
func (m *Machine) atBreak() bool {
	pc := m.CPU.PC
	if m.breaks.resuming {
		m.breaks.resuming = false
		if pc == m.breaks.resumeAt {
			return false
		}
	}
	f := m.breaks.fns[pc&0x1fffffff]
	if f == nil || !f(m) {
		return false
	}
	m.breaks.resumeAt, m.breaks.resuming = pc, true
	return true
}
