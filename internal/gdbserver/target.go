// Package gdbserver is a server for GDB's remote serial protocol, so that
// GDB, or any debugger that speaks the protocol, debugs a program on a
// PlayStation: yaroze debug is its command.
//
// A Server speaks the protocol to the debugger and a Target does what it
// asks of the program. There are two: HSS, the monitor's remote debugger on
// a console's serial cable, and Emulator, a machine of package emu run
// in-process, with no monitor in its memory.
//
// The protocol's side is the R3000's: 38 registers numbered as GDB's MIPS
// targets always have, which are also HSS's numbers, a target description
// with no floating point unit, and breakpoints that are a break
// instruction over a saved word for a target with none of its own.
package gdbserver

import (
	"context"
	"errors"
	"io"
)

// The registers, as GDB numbers a 32-bit MIPS's and HSS does: the 32
// general registers first.
const (
	RegSR       = 32
	RegLO       = 33
	RegHI       = 34
	RegBadVAddr = 35
	RegCause    = 36
	RegPC       = 37

	NumRegs = 38
)

// The signals a program stops for, in GDB's numbers.
const (
	SIGINT  = 2
	SIGILL  = 4
	SIGTRAP = 5
	SIGABRT = 6
	SIGFPE  = 8
	SIGBUS  = 10
)

// A Stop is why a program stopped.
type Stop struct {
	Signal int
}

// A Target is a program a Server debugs. It starts stopped. Its methods are
// called one at a time, except Interrupt.
// Killer is a Target that does something with the program when the
// debugger kills it, rather than leave it as it is.
type Killer interface {
	Kill(ctx context.Context) error
}

type Target interface {
	// ReadRegister and WriteRegister read and set a register of the stopped
	// program, by its number.
	ReadRegister(ctx context.Context, n int) (uint32, error)
	WriteRegister(ctx context.Context, n int, v uint32) error

	// ReadMemory and WriteMemory read and write the stopped program's
	// memory.
	ReadMemory(ctx context.Context, addr uint32, n int) ([]byte, error)
	WriteMemory(ctx context.Context, addr uint32, data []byte) error

	// Step starts the program for one instruction, and its delay slot's
	// with it if it is a jump or a branch, and Continue lets it run. Wait
	// then waits for it to stop.
	Step(ctx context.Context) error
	Continue(ctx context.Context) error
	Wait(ctx context.Context) (Stop, error)

	// Interrupt asks the running program to stop, which Wait then reports.
	// It may be called while Wait waits, from another goroutine.
	Interrupt() error

	// SetOutput sets where what the program prints goes, which is written
	// while Wait waits.
	SetOutput(w io.Writer)
}

// The kinds of breakpoint, as the protocol's Z packets number them.
const (
	BreakSoftware = iota
	BreakHardware
	WatchWrite
	WatchRead
	WatchAccess
)

// ErrUnsupported is what a Breakpointer returns for a kind of breakpoint
// it does not have.
var ErrUnsupported = errors.New("gdbserver: not supported by the target")

// A Breakpointer is a Target with breakpoints of its own. The Server keeps
// a breakpoint of a kind the target has there, and without one, keeps
// software breakpoints itself, as break instructions it writes over the
// program for as long as it runs.
type Breakpointer interface {
	SetBreakpoint(kind int, addr uint32, size int) error
	ClearBreakpoint(kind int, addr uint32, size int) error
}
