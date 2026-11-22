package gdbserver

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.chrisrx.dev/psx/serial"
)

// How an HSS target's program is started.
const (
	// StartRDB uploads the program and starts the monitor's remote
	// debugger with its rdb command, which stops before the program's
	// first instruction. Both Sony's monitor and the clean one have it.
	// The program cannot be stopped once it runs: it stops at a
	// breakpoint or a fault.
	StartRDB = "rdb"

	// StartGo uploads the program and starts it with the monitor's go
	// command, with a breakpoint at its first instruction. The clean
	// monitor stops a program it started so for the debugger, there and
	// whenever it is sent a Control-B, which is what Interrupt sends.
	StartGo = "go"

	// StartAttach stops the program the clean monitor's go command
	// started before, with a Control-B.
	StartAttach = "attach"

	// StartAuto asks the monitor which it is (serial.Monitor.Clean) and
	// starts the program with go on the clean monitor, which can stop it,
	// and with rdb on Sony's, which cannot.
	StartAuto = "auto"
)

// waitTurn is how long HSS waits for the console at a time, between which
// it looks for an Interrupt to send.
const waitTurn = 100 * time.Millisecond

// HSS is a program on a console, or in an emulated one, debugged through
// the monitor's remote debugger on the serial cable, in HSS packets
// (serial.Debugger). The registers' numbers are HSS's own.
type HSS struct {
	d    *serial.Debugger
	port *serial.Port

	// stoppable says a Control-B stops the program: the clean monitor's go
	// started it.
	stoppable bool
	interrupt atomic.Bool
}

// StartHSS starts prog on the console at the other end of port, whose
// monitor waits at its prompt, as start says, and returns it stopped. prog
// may be nil with StartRDB, for the program the monitor has loaded already,
// and is not used with StartAttach. Stoppable then says whether Interrupt
// can stop it.
func StartHSS(ctx context.Context, port *serial.Port, prog *serial.Program, start string) (*HSS, error) {
	h := &HSS{port: port}
	if start == StartAttach {
		h.stoppable = true
		h.d = serial.NewDebugger(port)
		if _, err := port.Write([]byte{0x02}); err != nil {
			return nil, err
		}
		if _, err := h.d.Wait(ctx); err != nil {
			return nil, fmt.Errorf("no program stopped for a Control-B, which only the clean monitor's go command lets one do: %w", err)
		}
		return h, nil
	}
	mon, err := serial.NewMonitor(ctx, port)
	if err != nil {
		return nil, err
	}
	if _, err := mon.Command(ctx, ""); err != nil {
		return nil, fmt.Errorf("the monitor does not answer: %w", err)
	}
	if start == StartAuto {
		start = StartRDB
		features, _, err := mon.Clean(ctx)
		if err != nil {
			return nil, err
		}
		if slices.Contains(features, "stop") && prog != nil {
			start = StartGo
		}
	}
	if prog != nil {
		if err := mon.Load(ctx, prog); err != nil {
			return nil, err
		}
	}
	switch start {
	case StartRDB:
		if h.d, err = mon.RemoteDebugger(ctx); err != nil {
			return nil, err
		}
		if prog != nil {
			// rdb starts with the stored registers, which Load set.
			if err := h.d.WriteRegister(ctx, serial.HSSRegEPC, prog.PC); err != nil {
				return nil, err
			}
		}
	case StartGo:
		if prog == nil {
			return nil, errors.New("starting with go needs the program, for where it starts")
		}
		// The word at the program's first instruction, as the monitor's dw
		// shows it: "80010000:27bdffe8  ".
		reply, err := mon.Command(ctx, fmt.Sprintf("dw %x 4", prog.PC))
		if err != nil {
			return nil, err
		}
		_, after, _ := strings.Cut(reply, fmt.Sprintf("%08x:", prog.PC))
		word, _, _ := strings.Cut(after, " ")
		orig, err := strconv.ParseUint(word, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("the monitor's dw answered %q", reply)
		}
		if _, err := mon.Command(ctx, fmt.Sprintf("sw %x %x", prog.PC, breakInsn)); err != nil {
			return nil, err
		}
		if err := mon.Run(ctx); err != nil {
			return nil, err
		}
		h.stoppable = true
		h.d = serial.NewDebugger(port)
		if _, err := h.d.Wait(ctx); err != nil {
			return nil, fmt.Errorf("the program did not stop at its first instruction, which it does only on the clean monitor (Sony's needs --start rdb): %w", err)
		}
		if err := h.d.WriteMemory(ctx, prog.PC, binary.LittleEndian.AppendUint32(nil, uint32(orig))); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%q is no way to start a program: auto, go, rdb or attach", start)
	}
	return h, nil
}

// Stoppable reports whether Interrupt stops the program: whether the
// clean monitor's go command started it.
func (h *HSS) Stoppable() bool { return h.stoppable }

func (h *HSS) SetOutput(w io.Writer) {
	// Under rdb the program's text comes in SEND packets, and under go as
	// it is, between the packets.
	h.d.Output = w
	if h.stoppable {
		h.d.Text = w
	}
}

func (h *HSS) ReadRegister(ctx context.Context, n int) (uint32, error) {
	// r0 is zero, and HSS has no number 35, the bad address.
	if n == 0 || n == RegBadVAddr {
		return 0, nil
	}
	return h.d.ReadRegister(ctx, n)
}

func (h *HSS) WriteRegister(ctx context.Context, n int, v uint32) error {
	if n == 0 || n == RegBadVAddr {
		return nil
	}
	return h.d.WriteRegister(ctx, n, v)
}

func (h *HSS) ReadMemory(ctx context.Context, addr uint32, n int) ([]byte, error) {
	return h.d.ReadMemory(ctx, addr, n)
}

func (h *HSS) WriteMemory(ctx context.Context, addr uint32, data []byte) error {
	return h.d.WriteMemory(ctx, addr, data)
}

func (h *HSS) Step(ctx context.Context) error     { return h.d.Step(ctx) }
func (h *HSS) Continue(ctx context.Context) error { return h.d.Continue(ctx) }

// Interrupt has Wait send the monitor a Control-B, rather than sending it
// here: the port is Wait's while it waits, and an emulated one is not safe
// to use from two goroutines.
func (h *HSS) Interrupt() error {
	if !h.stoppable {
		return errors.New("the program cannot be stopped: the monitor's remote debugger reads nothing while it runs, so it runs until a breakpoint or a fault (the clean monitor's go can stop one)")
	}
	h.interrupt.Store(true)
	return nil
}

// Wait waits for the program to stop, in turns short enough to send an
// Interrupt soon after it is asked for.
func (h *HSS) Wait(ctx context.Context) (Stop, error) {
	for {
		if h.interrupt.Swap(false) {
			if _, err := h.port.Write([]byte{0x02}); err != nil {
				return Stop{}, err
			}
		}
		turn, cancel := context.WithTimeout(ctx, waitTurn)
		sig, err := h.d.Wait(turn)
		cancel()
		switch {
		case err == nil:
			return Stop{Signal: int(sig)}, nil
		case ctx.Err() != nil:
			return Stop{}, ctx.Err()
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, serial.ErrTimeout):
			continue
		}
		return Stop{}, err
	}
}
