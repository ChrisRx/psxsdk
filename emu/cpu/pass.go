package cpu

import "encoding/binary"

// MaxPassSteps is the most steps a trial pass runs before giving up on
// finding a loop.
const MaxPassSteps = 32

// Pass is a trial pass of a loop: what TryPass found one costs, step by step.
// It also holds the copy of the CPU the trial runs on, kept here so that a
// trial allocates nothing.
type Pass struct {
	// Cycles[:Steps] is what each step cost.
	Steps  int
	Cycles [MaxPassSteps]uint32

	cpu    CPU
	charge uint32
	cost   cost
}

// TryPass reports whether the CPU is in a quiet loop: whether the steps from
// here come back here with the CPU just as it is now, and touch nothing but
// the registers on the way, no device, no coprocessor and no store. If so,
// a pass is a function of the registers and RAM that gives back the same
// registers, so every pass after it goes the same way until something other
// than the CPU changes RAM or raises an interrupt.
//
// The steps are real ones, run by Run on a copy of the CPU, so that there is
// no second account here of what an instruction does or costs. Each is checked
// before it runs. The copy has no bus, since a checked step doesn't use one,
// and its own place for the window to charge, so nothing outside it changes.
//
// A step Run would stop before, for k's hook or a breakpoint, ends the trial,
// so that skipping passes can't pass over one.
//
// Under the cost model the copy has its own copy of the model, and the pass
// is a loop only if it costs the same every time: it missed nothing in the
// instruction cache and waited for nothing, so that the model is left as it
// was and every pass after it costs what the trial did.
func (c *CPU) TryPass(p *Pass, k *Clock) bool {
	if c.irqPending() {
		return false
	}
	t := &p.cpu
	*t = *c
	t.bus = nil
	t.window.Charge = &p.charge
	if c.cost != nil {
		p.cost = *c.cost
		t.cost = &p.cost
	}
	for i := range p.Cycles {
		if !t.quietNext(k) {
			return false
		}
		t.Run(1, nil)
		p.Cycles[i] = t.LastCycles
		if t.PC == c.PC && t.NextPC == c.NextPC && t.branch == c.branch {
			p.Steps = i + 1
			return t.R == c.R && t.loadD == c.loadD && t.loadV == c.loadV &&
				(c.cost == nil || p.cost.steady(c.cost))
		}
	}
	return false
}

// quietNext reports whether the next instruction is one a quiet loop can
// have, at this point, fetched from RAM: register arithmetic that cannot
// overflow into an exception, a branch or jump that doesn't link, or a load
// from RAM.
func (c *CPU) quietNext(k *Clock) bool {
	a, ok := c.window.inWindow(c.PC)
	if !ok || c.PC&3 != 0 || k.Hooked && c.PC&k.HookMask == k.Hook || k.Breaks != nil && k.Breaks.Has(c.PC) {
		return false
	}
	op := binary.LittleEndian.Uint32(c.window.Mem[a&uint32(len(c.window.Mem)-1):])
	switch op >> 26 {
	case 0x00:
		switch op & 0x3f {
		case 0x00, 0x02, 0x03, 0x04, 0x06, 0x07, // the shifts
			0x21, 0x23, 0x24, 0x25, 0x26, 0x27, 0x2a, 0x2b: // addu to sltu
			return true
		}
		return false
	case 0x01: // bltz and bgez, not bltzal and bgezal
		return op>>17&0xf != 0x8
	case 0x02, 0x04, 0x05, 0x06, 0x07, // j, beq, bne, blez, bgtz
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f: // addiu to lui
		return true
	case 0x20, 0x21, 0x23, 0x24, 0x25: // lb, lh, lw, lbu, lhu
		addr := c.R[op>>21&0x1f] + uint32(int32(int16(op)))
		_, ram := c.window.inWindow(addr)
		return ram && addr&(accessSize[op>>26&3]-1) == 0
	}
	return false
}
