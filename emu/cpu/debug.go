package cpu

// What a debugger needs of the CPU between steps, beyond its exported
// state: the registers as the program will next see them, and a way to
// move the PC.

// Reg returns general register n as the next instruction but one reads it:
// a load still on its way to the register has landed.
func (c *CPU) Reg(n int) uint32 {
	if c.loadD != 0 && int(c.loadD) == n {
		return c.loadV
	}
	return c.R[n]
}

// SetReg sets general register n, over any load on its way to it.
func (c *CPU) SetReg(n int, v uint32) {
	if n == 0 {
		return
	}
	if int(c.loadD) == n {
		c.loadD, c.loadV = 0, 0xffffffff
	}
	c.R[n] = v
}

// Jump makes pc the next instruction, as a jump whose delay slot has run
// does.
func (c *CPU) Jump(pc uint32) {
	c.PC, c.NextPC = pc, pc+4
	c.branch = false
}

// InDelaySlot reports whether the next instruction is in the delay slot of
// the branch or jump the last step ran.
func (c *CPU) InDelaySlot() bool { return c.branch }
