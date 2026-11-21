package gdbserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/cpu"
)

// frameBudget bounds the instructions a frame runs looking for its
// vertical blank, and startBudget the frames a program may take to reach
// the address it is first stopped at.
const (
	frameBudget = 4_000_000
	startBudget = 60 * 60
)

// frameTime is how long a frame lasts on a console, which Wait paces a
// machine nothing else runs to.
const frameTime = time.Second / 60

// vector is where the CPU goes for an exception once the kernel is up.
const vector = 0x80000080

// Emulator is a program in an emulated machine, debugged by running the
// machine itself: a step is one instruction, a breakpoint is the
// machine's own and writes nothing into the program, and the registers
// and memory are the machine's.
//
// The machine runs a frame at a time. Wait runs the frames, paced to a
// console's unless Fast is set. With Driven set something else does, by
// calling RunFrame at its own pace, such as a window that shows the
// picture, and Wait only waits.
type Emulator struct {
	// Fast runs the frames as fast as the host does.
	Fast bool

	// Driven says RunFrame is called from elsewhere, as often as a frame
	// should run.
	Driven bool

	mu      sync.Mutex // guards the machine and what follows
	m       *emu.Machine
	running bool
	fault   int             // the signal of the fault the vector's breakpoint caught
	user    map[uint32]bool // the debugger's breakpoints

	interrupt atomic.Bool
	stopped   chan result
}

// NewEmulator returns m as a Target, stopped where it is. A fault the
// program takes from here on, an address error or a reserved instruction,
// or a break instruction it runs, stops it at the instruction, as it stops
// a program under the monitor's debugger, in place of going on into the
// kernel's handler.
func NewEmulator(m *emu.Machine) *Emulator {
	e := &Emulator{
		m:       m,
		user:    map[uint32]bool{},
		stopped: make(chan result, 1),
	}
	m.SetBreak(vector, e.atVector)
	return e
}

// RunTo runs the machine until it is about to run the instruction at addr,
// as fast as the host does: to a program's first instruction, from the
// BIOS's start.
func (e *Emulator) RunTo(addr uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.m.SetBreak(addr, func(*emu.Machine) bool { return true })
	defer e.restoreBreak(addr)
	for range startBudget {
		_, err := e.m.RunFrame(frameBudget)
		switch {
		case err == nil:
			continue
		case !errors.Is(err, emu.ErrBreak):
			return err
		case e.m.CPU.PC == addr:
			return nil
		}
		return fmt.Errorf("the program stopped at %08x, for signal %d, before it reached %08x", e.m.CPU.PC, e.fault, addr)
	}
	return fmt.Errorf("the program did not reach %08x in %d frames", addr, startBudget)
}

// restoreBreak leaves the machine's breakpoint at addr as the debugger and
// the Emulator have it.
func (e *Emulator) restoreBreak(addr uint32) {
	switch {
	case addr == vector:
		e.m.SetBreak(vector, e.atVector)
	case e.user[addr]:
		e.m.SetBreak(addr, func(*emu.Machine) bool { return true })
	default:
		e.m.ClearBreak(addr)
	}
}

// atVector is the breakpoint at the exception vector. An interrupt or a
// system call goes on to the kernel. Anything else stops the program, put
// back as it was before the instruction: the CPU at it again, or at the
// branch before it when it is in a delay slot, and the status register's
// mode bits as they were. The cause and the bad address stay, for the
// debugger to show.
func (e *Emulator) atVector(m *emu.Machine) bool {
	c := m.CPU
	switch code := c.COP0[cpu.Cause] >> 2 & 31; code {
	case cpu.ExcInt, cpu.ExcSyscall:
		return e.user[vector]
	case cpu.ExcBp:
		e.fault = SIGTRAP
	case cpu.ExcRI, cpu.ExcCpU:
		e.fault = SIGILL
	case cpu.ExcOv:
		e.fault = SIGFPE
	default:
		e.fault = SIGBUS
	}
	sr := c.COP0[cpu.SR]
	c.COP0[cpu.SR] = sr&^0xf | sr>>2&0xf
	c.Jump(c.COP0[cpu.EPC])
	return true
}

func (e *Emulator) SetOutput(w io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.m.TTY = w
}

func (e *Emulator) ReadRegister(ctx context.Context, n int) (uint32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.m.CPU
	switch {
	case n < 32:
		return c.Reg(n), nil
	case n == RegSR:
		return c.COP0[cpu.SR], nil
	case n == RegLO:
		return c.LO, nil
	case n == RegHI:
		return c.HI, nil
	case n == RegBadVAddr:
		return c.COP0[cpu.BadVAddr], nil
	case n == RegCause:
		return c.COP0[cpu.Cause], nil
	case n == RegPC:
		return c.PC, nil
	}
	return 0, fmt.Errorf("no register %d", n)
}

func (e *Emulator) WriteRegister(ctx context.Context, n int, v uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.m.CPU
	switch {
	case n < 32:
		c.SetReg(n, v)
	case n == RegSR:
		c.COP0[cpu.SR] = v
	case n == RegLO:
		c.LO = v
	case n == RegHI:
		c.HI = v
	case n == RegBadVAddr:
		c.COP0[cpu.BadVAddr] = v
	case n == RegCause:
		c.COP0[cpu.Cause] = v
	case n == RegPC:
		if v != c.PC {
			c.Jump(v)
		}
	default:
		return fmt.Errorf("no register %d", n)
	}
	return nil
}

func (e *Emulator) ReadMemory(ctx context.Context, addr uint32, n int) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	data := make([]byte, n)
	for i := range data {
		data[i] = e.m.Bus.Read8(addr + uint32(i))
	}
	return data, nil
}

func (e *Emulator) WriteMemory(ctx context.Context, addr uint32, data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, b := range data {
		e.m.Bus.Write8(addr+uint32(i), b)
	}
	return nil
}

// SetBreakpoint sets a breakpoint of the machine's own, for a software
// breakpoint and a hardware one alike: neither writes anything into the
// program, so either can be set before the program is loaded. The machine
// has no watchpoints.
func (e *Emulator) SetBreakpoint(kind int, addr uint32, size int) error {
	if kind != BreakSoftware && kind != BreakHardware {
		return ErrUnsupported
	}
	if !inRAM(addr) {
		return fmt.Errorf("%08x is not an instruction in RAM", addr)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.user[addr] = true
	e.restoreBreak(addr)
	return nil
}

func (e *Emulator) ClearBreakpoint(kind int, addr uint32, size int) error {
	if kind != BreakSoftware && kind != BreakHardware {
		return ErrUnsupported
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.user, addr)
	e.restoreBreak(addr)
	return nil
}

// one runs one instruction, the debugger's breakpoint on it, if it has
// one, left out: the program is going on from there. A fault it takes is
// caught as the vector's breakpoint would catch it, and reported.
func (e *Emulator) one() (fault bool, err error) {
	pc := e.m.CPU.PC
	if e.user[pc] {
		e.m.ClearBreak(pc)
		defer e.restoreBreak(pc)
	}
	if err := e.m.Step(); err != nil && !errors.Is(err, emu.ErrBreak) {
		return false, err
	}
	return e.m.CPU.PC == vector && e.m.CPU.Excode != cpu.NoException && e.atVector(e.m) && e.fault != 0, nil
}

// Step runs one instruction, and the one in its delay slot if it is a jump
// or a branch, so that the program never rests between the two, where a
// debugger that works out the next instruction for itself would have it
// wrong. An interrupt taken in the instruction's place is a step too, to
// the kernel's handler.
func (e *Emulator) Step(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fault = 0
	stop := Stop{Signal: SIGTRAP}
	for range 2 {
		fault, err := e.one()
		if err != nil {
			return err
		}
		if fault {
			stop.Signal = e.fault
		}
		if fault || !e.m.CPU.InDelaySlot() {
			break
		}
	}
	e.stopped <- result{stop: stop}
	return nil
}

// Continue lets the program run, which RunFrame then does. The
// instruction it is at runs here, past any breakpoint on it.
func (e *Emulator) Continue(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fault = 0
	e.interrupt.Store(false)
	if e.user[e.m.CPU.PC] {
		fault, err := e.one()
		if err != nil {
			return err
		}
		if fault {
			e.stopped <- result{stop: Stop{Signal: e.fault}}
			return nil
		}
	}
	e.running = true
	return nil
}

func (e *Emulator) Interrupt() error {
	e.interrupt.Store(true)
	return nil
}

// RunFrame runs the machine to its next vertical blank, if the program is
// running, and reports whether it is. A program that stops in the frame
// is Wait's to report.
func (e *Emulator) RunFrame() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return false
	}
	var r result
	if e.interrupt.Swap(false) {
		r.stop.Signal = SIGINT
		e.settle()
	} else {
		_, err := e.m.RunFrame(frameBudget)
		switch {
		case err == nil:
			return true
		case errors.Is(err, emu.ErrBreak):
			r.stop.Signal = SIGTRAP
			if e.fault != 0 {
				r.stop.Signal = e.fault
			}
		default:
			r.err = err
		}
	}
	e.running = false
	e.stopped <- r
	return false
}

// settle leaves a program the debugger interrupted where a debugger can
// make sense of it. It is taken out of a delay slot, as Step does. A frame
// ends at the vertical blank, whose interrupt the CPU has just taken, and
// the step out of a delay slot can take the interrupt in the instruction's
// place, so either way it may be at the kernel's handler: it is put back
// at the instruction the interrupt came before, with the interrupt still
// waiting, as the monitor leaves a program it stops for a Control-B. For
// an interrupt in a delay slot that is the branch, which runs again, as on
// the hardware.
func (e *Emulator) settle() {
	c := e.m.CPU
	if c.InDelaySlot() && !e.user[c.PC] {
		e.m.Step()
	}
	if c.PC == vector && c.Excode == cpu.ExcInt && !e.user[vector] {
		sr := c.COP0[cpu.SR]
		c.COP0[cpu.SR] = sr&^0xf | sr>>2&0xf
		c.Jump(c.COP0[cpu.EPC])
	}
}

// Wait waits for the program to stop, running its frames meanwhile unless
// the machine is Driven.
func (e *Emulator) Wait(ctx context.Context) (Stop, error) {
	next := time.Now()
	for {
		select {
		case r := <-e.stopped:
			return r.stop, r.err
		case <-ctx.Done():
			return Stop{}, ctx.Err()
		default:
		}
		if e.Driven {
			select {
			case r := <-e.stopped:
				return r.stop, r.err
			case <-ctx.Done():
				return Stop{}, ctx.Err()
			}
		}
		switch {
		case !e.RunFrame():
			// Stopped, which is there to report at once.
			select {
			case r := <-e.stopped:
				return r.stop, r.err
			case <-time.After(time.Millisecond):
			}
		case !e.Fast:
			next = next.Add(frameTime)
			if d := time.Until(next); d > 0 {
				time.Sleep(d)
			} else {
				next = time.Now()
			}
		}
	}
}
