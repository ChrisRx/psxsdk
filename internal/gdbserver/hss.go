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
	// starts the program with go on the clean monitor, which can stop it.
	// Sony's, which cannot, is first replaced with the clean one when
	// HSSOptions.TakeOver says how, and otherwise the program is started
	// with rdb.
	StartAuto = "auto"
)

// HSSOptions say how StartHSS starts a program.
type HSSOptions struct {
	// Start is how: StartAuto, StartGo, StartRDB or StartAttach.
	Start string

	// TakeOver, if not nil, is the clean monitor as serial.TakeOverMonitor
	// uploads it in place of Sony's, keeping Sony's library as the boot
	// left it. StartAuto uploads it when it finds Sony's monitor.
	TakeOver *serial.Program

	// Log, if not nil, is told of a take-over as it goes.
	Log io.Writer
}

// takeOverTimeout is how long the clean monitor gets to greet the host
// once the take-over is uploaded: the loader copies it into place in a
// moment.
const takeOverTimeout = 10 * time.Second

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
	returned  bool
	interrupt atomic.Bool
}

// StartHSS starts prog on the console at the other end of port, whose
// monitor waits at its prompt, as opts say, and returns it stopped. prog
// may be nil with StartRDB, for the program the monitor has loaded already,
// and is not used with StartAttach. Stoppable then says whether Interrupt
// can stop it.
func StartHSS(ctx context.Context, port *serial.Port, prog *serial.Program, opts HSSOptions) (*HSS, error) {
	start := opts.Start
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
		if !slices.Contains(features, "stop") && opts.TakeOver != nil && prog != nil {
			if mon, err = takeOver(ctx, port, mon, opts); err != nil {
				return nil, err
			}
			features = []string{"stop"}
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

// takeOver puts the clean monitor in place of Sony's, at its prompt on
// the other end of port, and returns the clean one once it is at its own.
func takeOver(ctx context.Context, port *serial.Port, mon *serial.Monitor, opts HSSOptions) (*serial.Monitor, error) {
	if opts.Log != nil {
		size := 0
		for _, seg := range opts.TakeOver.Segments {
			size += len(seg.Data)
		}
		fmt.Fprintf(opts.Log, "yaroze: the monitor is Sony's, which cannot stop a running program: uploading the clean monitor in its place, %d KB (--take-over=false keeps Sony's)\n", size/1024)
	}
	if err := mon.Load(ctx, opts.TakeOver); err != nil {
		return nil, fmt.Errorf("uploading the clean monitor: %w", err)
	}
	if err := mon.Run(ctx); err != nil {
		return nil, err
	}
	timeout := port.Timeout
	port.Timeout = takeOverTimeout
	defer func() { port.Timeout = timeout }()
	// The clean monitor greets the host as it starts, as Sony's does.
	if _, err := port.ReadUntil(ctx, "Communication baud rate"); err != nil {
		return nil, fmt.Errorf("the clean monitor did not start: %w", err)
	}
	mon, err := serial.NewMonitor(ctx, port)
	if err != nil {
		return nil, err
	}
	if _, err := mon.Command(ctx, ""); err != nil {
		return nil, fmt.Errorf("the clean monitor does not answer: %w", err)
	}
	if _, ok, err := mon.Clean(ctx); err != nil || !ok {
		return nil, fmt.Errorf("the monitor that started is not the clean one (%v)", err)
	}
	return mon, nil
}

// Where the clean monitor starts again when a program returns to it: its
// start, on its own stack at the top of its space (serial/monitor's run and
// target.json).
const (
	monitorStart = 0x80068000
	monitorStack = 0x8008fff0
)

// Kill sends the console back to the clean monitor's prompt, when its go
// started the program: the program, stopped, carries on from the
// monitor's start, as the monitor does when a program returns to it.
// Sony's remote debugger has no way back, and the program is left as it
// is.
func (h *HSS) Kill(ctx context.Context) error {
	if !h.stoppable {
		return nil
	}
	if err := h.d.WriteRegister(ctx, 29, monitorStack); err != nil {
		return err
	}
	if err := h.d.WriteRegister(ctx, serial.HSSRegEPC, monitorStart); err != nil {
		return err
	}
	if err := h.d.Continue(ctx); err != nil {
		return err
	}
	h.returned = true
	return nil
}

// Returned reports whether Kill sent the console back to the monitor.
func (h *HSS) Returned() bool { return h.returned }

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
