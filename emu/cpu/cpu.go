// Package cpu emulates the PlayStation's R3000A, with its system control
// coprocessor (COP0) and geometry transformation engine (GTE, COP2).
//
// It is written to match psxe instruction for instruction, so that the two
// can be compared with a trace (see format/psxtrace) until the first point
// they disagree. That includes psxe's cycle accounting and the places where
// psxe departs from the hardware; those are marked where they occur, so they
// can be revisited once something better than psxe is the reference.
package cpu

import "encoding/binary"

// Bus is the CPU's view of memory.
type Bus interface {
	Read32(addr uint32) uint32
	Read16(addr uint32) uint16
	Read8(addr uint32) uint8
	Write32(addr, v uint32)
	Write16(addr uint32, v uint16)
	Write8(addr uint32, v uint8)

	// Fetch reads the instruction at addr and returns it with the cycles
	// the fetch cost.
	Fetch(addr uint32) (insn, cycles uint32)
}

// COP0 register numbers.
const (
	BPC      = 3
	BDA      = 5
	JumpDest = 6
	DCIC     = 7
	BadVAddr = 8
	BDAM     = 9
	BPCM     = 11
	SR       = 12
	Cause    = 13
	EPC      = 14
	PRID     = 15
)

// Status register bits.
const (
	srIEc = 1 << 0  // interrupts enabled
	srIM2 = 1 << 10 // the interrupt controller's line, in SR and Cause
	srIsc = 1 << 16 // cache isolated: stores do not reach memory
	srBEV = 1 << 22 // exception vectors in ROM
)

// Exception codes, as stored in bits 2-6 of Cause.
const (
	ExcInt     = 0x00
	ExcAdEL    = 0x04
	ExcAdES    = 0x05
	ExcSyscall = 0x08
	ExcBp      = 0x09
	ExcRI      = 0x0a
	ExcCpU     = 0x0b
	ExcOv      = 0x0c

	// NoException is Excode after a step that took none.
	NoException = 0xff
)

// cop0WriteMask is which bits of each COP0 register mtc0 can change.
var cop0WriteMask = [16]uint32{
	BPC:  0xffffffff,
	BDA:  0xffffffff,
	DCIC: 0xffc0f03f,
	BDAM: 0xffffffff,
	BPCM: 0xffffffff,
	SR:   0xffffffff,
	// Only the two software interrupt bits of Cause are writable.
	Cause: 0x00000300,
}

// CPU is the R3000A's state.
type CPU struct {
	R      [32]uint32
	HI, LO uint32
	COP0   [16]uint32
	GTE    GTE

	// PC is the next instruction to fetch and NextPC the one after it,
	// which a jump or branch changes to make its delay slot.
	PC, NextPC uint32

	// SavedPC and Opcode are the instruction the last step fetched.
	SavedPC uint32
	Opcode  uint32

	// LastCycles is what the last step cost, and TotalCycles the sum.
	LastCycles  uint32
	TotalCycles uint64

	// Excode is the exception the last step took, or NoException.
	Excode uint8

	// A load's result arrives one instruction late. loadD is the register
	// it is bound for and loadV the value; loadD 0 means none is pending,
	// since a load into $zero has no effect anyway.
	loadD, loadV uint32

	branch, delaySlot, branchTaken bool

	bus    Bus
	window Window
}

// Window is memory the CPU fetches instructions from directly, rather than
// through the bus: physical addresses below Size, in the segments that map
// onto them, which read as Mem, its length a power of two, and cost Delay.
// It must read what the bus's Fetch would. Fetch leaves nothing charged on
// the bus, so a fetch through the window sets *Charge to zero as well.
type Window struct {
	Mem         []byte
	Size, Delay uint32
	Charge      *uint32
}

// SetWindow sets the memory the CPU fetches from directly. It survives a
// reset.
func (c *CPU) SetWindow(w Window) {
	if len(w.Mem)&(len(w.Mem)-1) != 0 {
		panic("cpu: window memory is not a power of two")
	}
	c.window = w
}

// inWindow reports whether the instruction at pc is fetched through the
// window, and from what offset. KUSEG's first 512MB, KSEG0 and KSEG1, the
// segments 0, 4 and 5 by the top three bits, map onto physical memory from
// 0.
func (w *Window) inWindow(pc uint32) (uint32, bool) {
	a := pc & 0x1fffffff
	return a, 0x31>>(pc>>29)&1 != 0 && a < w.Size
}

// New returns a CPU in its reset state, about to fetch from the BIOS.
func New(bus Bus) *CPU {
	c := &CPU{bus: bus}
	c.Reset()
	return c
}

// Reset puts the CPU in its reset state.
func (c *CPU) Reset() {
	*c = CPU{bus: c.bus, window: c.window}
	c.PC = 0xbfc00000
	c.NextPC = c.PC + 4
	c.COP0[SR] = 0x10900000
	c.COP0[PRID] = 0x00000002
	c.Excode = NoException
}

// SetIRQ raises or lowers the interrupt controller's line into the CPU.
func (c *CPU) SetIRQ(pending bool) {
	if pending {
		c.COP0[Cause] |= srIM2
	} else {
		c.COP0[Cause] &^= srIM2
	}
}

// IRQPending reports whether the next step takes an interrupt.
func (c *CPU) IRQPending() bool { return c.irqPending() }

func (c *CPU) irqPending() bool {
	return c.COP0[SR]&srIEc != 0 && c.COP0[SR]&c.COP0[Cause]&0x700 != 0
}

// pendingLoad lets a delayed load land.
func (c *CPU) pendingLoad() {
	c.R[c.loadD] = c.loadV
	c.R[0] = 0
	c.loadV = 0xffffffff
	c.loadD = 0
}

func (c *CPU) exception(code uint32) {
	c.Excode = uint8(code)

	c.COP0[Cause] = c.COP0[Cause]&0xffffff80 | code<<2
	c.COP0[EPC] = c.SavedPC
	if c.delaySlot {
		c.COP0[EPC] -= 4
		c.COP0[Cause] |= 0x80000000
	}

	// Push the interrupt-enable and kernel-mode pairs down the stack.
	mode := c.COP0[SR] & 0x3f
	c.COP0[SR] = c.COP0[SR]&0xffffffc0 | (mode<<2)&0x3f

	if c.COP0[SR]&srBEV != 0 {
		c.PC = 0xbfc00180
	} else {
		c.PC = 0x80000080
	}
	c.NextPC = c.PC + 4
}

// Clock is what the machine's devices count off the CPU's steps, which Run
// keeps up to date itself for as long as no device has anything to do, so
// that it only hands back to the machine when one does. The pointers are
// into the devices' own state, so Run sees any change a step makes to them.
type Clock struct {
	// Steps counts the steps. Run stops at StepsDue.
	Steps, StepsDue *uint64

	// Run stops when TotalCycles reaches any of Due.
	Due [3]*uint64

	// Video advances by VideoRate for each cycle, rounded to a float32 at
	// every step. Run stops at the step that would take it to VideoEdge.
	Video, VideoEdge *float32
	VideoRate        float32

	// Sample counts cycles. Run stops at the step that would take it to
	// SampleCycles.
	Sample       *uint32
	SampleCycles uint32

	// Run stops after every step while Busy is true.
	Busy *bool

	// With Hooked, Run stops before an instruction whose address, masked
	// with HookMask, is Hook. The mask leaves the segment out.
	Hook, HookMask uint32
	Hooked         bool

	// Run stops before an instruction at an address in Breaks, if it is
	// not nil.
	Breaks *Breaks

	// Run stops before the instruction at Spin, the head of a loop the
	// machine can skip through.
	Spin uint32

	// Run reads the rest once, when it starts. A device that changes any
	// of it, or fails, sets Wake, and Run stops after that step so the
	// machine can read it again.
	Wake *bool
}

// Step fetches and executes one instruction, or takes an interrupt in its
// place.
func (c *CPU) Step() { c.Run(1, nil) }

// Run steps up to n times and returns how many it ran. Every step but the
// last is followed by what k counts, done as the machine would; Run stops
// early at a step after which the machine has more to do, and leaves the
// last step's counting to the machine, so that it can do it all in order.
// k is not used when n is 1.
//
// The steps are run here rather than by calling a function for each, which
// costs a good deal under wasm.
func (c *CPU) Run(n uint64, k *Clock) uint64 {
	// The step at which the devices next have something to do: due in
	// cycles, for the drive, the pad, the serial port and the SPU's sample,
	// and stepsDue in steps, for the timers, or straight away while DMA is
	// busy.
	var total0, due, stepsDue uint64
	if n > 1 {
		total0 = c.TotalCycles
		due = min(*k.Due[0], *k.Due[1], *k.Due[2])
		if *k.Sample < k.SampleCycles {
			due = min(due, total0+uint64(k.SampleCycles-*k.Sample))
		} else {
			due = total0
		}
		stepsDue = 1
		if !*k.Busy && *k.StepsDue > *k.Steps {
			stepsDue = *k.StepsDue - *k.Steps
		}
		*k.Wake = false
	}

	i := uint64(1)
	for ; ; i++ {
		c.LastCycles = 0
		c.Excode = NoException

		c.SavedPC = c.PC
		c.delaySlot = c.branch
		c.branch = false
		c.branchTaken = false

		// psxe takes the address error and then carries on to fetch from the
		// vector within the same step.
		if c.SavedPC&3 != 0 {
			c.COP0[BadVAddr] = c.SavedPC
			c.exception(ExcAdEL)
		}

		// The window is checked here rather than in a function of its own,
		// which would be too big to inline.
		if a, ok := c.window.inWindow(c.PC); ok {
			*c.window.Charge = 0
			c.Opcode = binary.LittleEndian.Uint32(c.window.Mem[a&uint32(len(c.window.Mem)-1):])
			c.LastCycles = c.window.Delay
		} else {
			c.Opcode, c.LastCycles = c.bus.Fetch(c.PC)
		}

		c.PC = c.NextPC
		c.NextPC += 4

		if c.irqPending() {
			// A GTE command is not interrupted: it completes, and the
			// interrupt is taken after it. Any other instruction is not
			// executed, and a pending load stays pending across the
			// exception.
			if c.Opcode&0xfe000000 == 0x4a000000 {
				c.pendingLoad()
				c.LastCycles += c.GTE.Command(c.Opcode)
			}
			c.TotalCycles += uint64(c.LastCycles)
			c.R[0] = 0
			c.exception(ExcInt)
		} else {
			// The frequent instructions are run here, so that they cost
			// no calls; execute runs the rest. Each reads its operands,
			// then lets a pending load land, then writes its result, so
			// an instruction in a load's delay slot sees the old value
			// and its own result wins over the load's.
			cycles, ok := uint32(2), true
			switch op := c.Opcode >> 26; op {
			case 0x00:
				switch funct := c.Opcode & 0x3f; funct {
				case 0x00, 0x02, 0x03, 0x04, 0x06, 0x07:
					// sll, srl and sra by the shift amount field, and with bit 2
					// set, sllv, srlv and srav by the low five bits of rs.
					t := c.R[c.rt()]
					s := c.shamt()
					if funct&4 != 0 {
						s = c.R[c.rs()] & 0x1f
					}
					c.pendingLoad()
					c.R[c.rd()] = shiftOp(funct, t, s)
				case 0x21, 0x23, 0x24, 0x25, 0x26, 0x27, 0x2a, 0x2b:
					s, t := c.R[c.rs()], c.R[c.rt()]
					c.pendingLoad()
					c.R[c.rd()] = aluOp(funct, s, t)
				default:
					cycles, ok = c.execute()
				}
			case 0x04, 0x05, 0x06, 0x07: // beq, bne, blez, bgtz
				c.branch = true
				c.branchTaken = false
				s, t := c.R[c.rs()], c.R[c.rt()]
				c.pendingLoad()
				if branchCond(op, s, t) {
					c.branchTo(c.simm() << 2)
				}
			case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e:
				s := c.R[c.rs()]
				c.pendingLoad()
				c.R[c.rt()] = immOp(op, s, c.Opcode)
			case 0x0f: // lui
				c.pendingLoad()
				c.R[c.rt()] = c.imm() << 16
			case 0x20, 0x21, 0x23, 0x24, 0x25:
				// lb, lh, lw, lbu and lhu. A load into the register a pending
				// load is bound for replaces it rather than letting it land first.
				// A misaligned address raises the exception without reading, and
				// leaves that pending load still pending.
				t := c.rt()
				addr := c.R[c.rs()] + c.simm()
				if c.loadD != t {
					c.pendingLoad()
				}
				if addr&(accessSize[op&3]-1) != 0 {
					c.COP0[BadVAddr] = addr
					c.exception(ExcAdEL)
					break
				}
				c.loadD = t
				if a, ok := c.window.inWindow(addr); ok {
					// Read here rather than in a function, which wasm would
					// not inline.
					*c.window.Charge = c.window.Delay
					m := c.window.Mem[a&uint32(len(c.window.Mem)-1):]
					switch op {
					case 0x20:
						c.loadV = uint32(int32(int8(m[0])))
					case 0x21:
						c.loadV = uint32(int32(int16(binary.LittleEndian.Uint16(m))))
					case 0x23:
						c.loadV = binary.LittleEndian.Uint32(m)
					case 0x24:
						c.loadV = uint32(m[0])
					case 0x25:
						c.loadV = uint32(binary.LittleEndian.Uint16(m))
					}
					break
				}
				switch op {
				case 0x20:
					c.loadV = uint32(int32(int8(c.bus.Read8(addr))))
				case 0x21:
					c.loadV = uint32(int32(int16(c.bus.Read16(addr))))
				case 0x23:
					c.loadV = c.bus.Read32(addr)
				case 0x24:
					c.loadV = uint32(c.bus.Read8(addr))
				case 0x25:
					c.loadV = uint32(c.bus.Read16(addr))
				}
			case 0x28, 0x29, 0x2b:
				// sb, sh and sw. A store while the cache is isolated goes to the
				// cache rather than memory, which nothing emulates, so it is
				// dropped.
				s, t := c.R[c.rs()], c.R[c.rt()]
				addr := s + c.simm()
				c.pendingLoad()
				if c.COP0[SR]&srIsc != 0 {
					break
				}
				if addr&(accessSize[op&3]-1) != 0 {
					c.COP0[BadVAddr] = addr
					c.exception(ExcAdES)
					break
				}
				if a, ok := c.window.inWindow(addr); ok {
					*c.window.Charge = c.window.Delay
					m := c.window.Mem[a&uint32(len(c.window.Mem)-1):]
					switch op {
					case 0x28:
						m[0] = uint8(t)
					case 0x29:
						binary.LittleEndian.PutUint16(m, uint16(t))
					default:
						binary.LittleEndian.PutUint32(m, t)
					}
					break
				}
				switch op {
				case 0x28:
					c.bus.Write8(addr, uint8(t))
				case 0x29:
					c.bus.Write16(addr, uint16(t))
				case 0x2b:
					c.bus.Write32(addr, t)
				}
			default:
				cycles, ok = c.execute()
			}
			if !ok {
				c.exception(ExcRI)
			}

			c.LastCycles += cycles
			c.TotalCycles += uint64(c.LastCycles)
			c.R[0] = 0
		}

		if i >= n {
			break
		}
		video := float32(*k.Video + float32(float32(c.LastCycles)*k.VideoRate))
		if c.TotalCycles >= due || video >= *k.VideoEdge || i >= stepsDue || *k.Wake ||
			k.Hooked && c.PC&k.HookMask == k.Hook || c.PC == k.Spin ||
			k.Breaks != nil && k.Breaks.Has(c.PC) {
			break
		}
		*k.Video = video
	}
	if n > 1 {
		// Every step but the last is counted here; the machine counts the
		// last.
		*k.Steps += i - 1
		*k.Sample += uint32(c.TotalCycles - uint64(c.LastCycles) - total0)
	}
	return i
}
