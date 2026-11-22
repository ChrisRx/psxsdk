package gdbserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"go.chrisrx.dev/psx/internal/ttyout"
)

// packetSize is the longest packet the server takes, which bounds how much
// memory the debugger asks for at a time.
const packetSize = 0x1000

// ErrKilled is what Serve returns when the debugger kills the program,
// GDB's kill command.
var ErrKilled = errors.New("gdbserver: the debugger killed the program")

// A Server serves one Target to a debugger at a time.
type Server struct {
	Target Target

	// Stdout receives what the program prints while no debugger is there
	// to be sent it.
	Stdout io.Writer

	// Logf, if set, is given each packet as it comes and goes.
	Logf func(format string, args ...any)

	// own says the server keeps the software breakpoints itself, the
	// target having none, and breaks is those: the word each replaced
	// while the program runs, by address.
	own    bool
	breaks map[uint32]*softBreak
	// placed is the breakpoints the target keeps, by address, for saying
	// which kind a stop is at.
	placed map[uint32]int

	regs  [NumRegs]uint32
	known [NumRegs]bool
	last  Stop

	// running is the stop of a program left running, when the debugger
	// detached or its connection went, which the next one waits for.
	running chan result

	mu     sync.Mutex // guards client and going
	client *conn
	going  bool // the program runs, so a debugger takes its output
	// features is what the debugger said it supports.
	swbreak, hwbreak bool
}

type softBreak struct {
	saved   [4]byte
	planted bool
}

type result struct {
	stop Stop
	err  error
}

// New returns a Server for t, which is stopped.
func New(t Target) *Server {
	s := &Server{
		Target: t,
		breaks: map[uint32]*softBreak{},
		placed: map[uint32]int{},
		last:   Stop{Signal: SIGTRAP},
	}
	t.SetOutput(ttyout.Program(output{s}))
	return s
}

func (s *Server) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// output is where the target writes what the program prints: to the
// debugger, in O packets, while there is one and the program runs, and to
// Stdout otherwise.
type output struct{ s *Server }

func (o output) Write(p []byte) (int, error) {
	s := o.s
	s.mu.Lock()
	c, going := s.client, s.going
	s.mu.Unlock()
	if c != nil && going {
		// An O packet is hexadecimal, so half a packet of text at most.
		for b := p; len(b) > 0; {
			n := min(len(b), packetSize/2-8)
			c.write([]byte("O" + hex.EncodeToString(b[:n])))
			b = b[n:]
		}
	} else if s.Stdout != nil {
		s.Stdout.Write(p)
	}
	return len(p), nil
}

// Serve serves the debuggers that connect to l, one after another, until
// ctx is done or one kills the program. A debugger that detaches leaves
// the program running, for the next to stop.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	stop := context.AfterFunc(ctx, func() { l.Close() })
	defer stop()
	// A program left running has the target until ctx stops it.
	defer func() {
		if ctx.Err() != nil && s.running != nil {
			<-s.running
		}
	}()
	for {
		nc, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		// A debugger in the middle of a session is cut off when ctx is
		// done, which a read of its connection would not otherwise see.
		cut := context.AfterFunc(ctx, func() { nc.Close() })
		err = s.Session(ctx, nc)
		cut()
		nc.Close()
		if errors.Is(err, ErrKilled) || ctx.Err() != nil {
			return err
		}
		s.logf("the debugger left: %v", err)
	}
}

// Session serves one debugger on rw until it detaches or goes.
func (s *Server) Session(ctx context.Context, rw io.ReadWriter) error {
	c := newConn(rw, s.logf)
	events := make(chan event)
	failed := make(chan error, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			ev, err := c.read()
			if err != nil {
				failed <- err
				return
			}
			select {
			case events <- ev:
			case <-done:
				return
			}
		}
	}()

	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.client = nil
		s.mu.Unlock()
	}()

	// A program the last debugger left running is stopped for this one.
	if s.running != nil {
		if err := s.Target.Interrupt(); err != nil {
			if s.Stdout != nil {
				fmt.Fprintf(s.Stdout, "yaroze debug: a debugger connected, but %v\n", err)
			}
			return err
		}
		// The debugger's first packets wait their turn meanwhile.
		if _, err := s.wait(ctx, nil, failed); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failed:
			return err
		case ev := <-events:
			if ev.interrupt {
				continue // nothing runs
			}
			reply, act := s.handle(ctx, c, ev.packet)
			switch act {
			case actNone:
				if err := c.write([]byte(reply)); err != nil {
					return err
				}
			case actStep, actContinue:
				s.start(ctx, act == actStep)
				stop, err := s.wait(ctx, events, failed)
				if err != nil {
					return err
				}
				if err := c.write([]byte(s.stopReply(ctx, stop))); err != nil {
					return err
				}
			case actDetach:
				clear(s.breaks)
				s.clearPlaced()
				s.start(ctx, false)
				c.write([]byte("OK"))
				return nil
			case actKill:
				return ErrKilled
			}
		}
	}
}

// Interrupt stops the program if it is running, as the debugger's
// Control-C does, which the debugger is then told of: for a host that
// takes the Control-C itself, the terminal's interrupt key when the
// debugger runs in its process. It does nothing to a stopped program.
func (s *Server) Interrupt() error {
	s.mu.Lock()
	going := s.going
	s.mu.Unlock()
	if !going {
		return nil
	}
	return s.Target.Interrupt()
}

// start starts the program going, for a step or until it stops, and
// leaves its stop to come on running.
func (s *Server) start(ctx context.Context, step bool) {
	s.known = [NumRegs]bool{}
	s.running = make(chan result, 1)
	s.mu.Lock()
	s.going = true
	s.mu.Unlock()
	go func(done chan<- result) {
		stop, err := s.resume(ctx, step)
		done <- result{stop, err}
	}(s.running)
}

// wait waits for the running program to stop, passing the debugger's
// Control-C on to the target meanwhile. A target that fails is reported to
// the debugger as stopped, aborted, with the error as the program's
// output: the debugger is still there to be told.
func (s *Server) wait(ctx context.Context, events <-chan event, failed <-chan error) (Stop, error) {
	for {
		select {
		case r := <-s.running:
			s.running = nil
			if r.err != nil {
				s.logf("the target failed: %v", r.err)
				output{s}.Write([]byte("yaroze debug: " + r.err.Error() + "\n"))
				r.stop = Stop{Signal: SIGABRT}
			}
			s.mu.Lock()
			s.going = false
			s.mu.Unlock()
			s.last = r.stop
			return r.stop, nil
		case ev := <-events:
			if ev.interrupt {
				if err := s.Target.Interrupt(); err != nil {
					output{s}.Write([]byte("yaroze debug: " + err.Error() + "\n"))
				}
			}
		case err := <-failed:
			return Stop{}, err
		case <-ctx.Done():
			return Stop{}, ctx.Err()
		}
	}
}

// resume runs the program for a step, or until it stops, and returns why
// it stopped. The server's own breakpoints are in the program's memory
// only while it runs: written before it goes on and taken out once it has
// stopped, so that the debugger reads the program as it is. A breakpoint
// where the program is, or in the delay slot of the branch it is at, would
// stop it again at once, so the program is first stepped past it.
func (s *Server) resume(ctx context.Context, step bool) (Stop, error) {
	t := s.Target
	if step {
		if err := t.Step(ctx); err != nil {
			return Stop{}, err
		}
		return t.Wait(ctx)
	}
	if len(s.breaks) > 0 {
		pc, err := t.ReadRegister(ctx, RegPC)
		if err != nil {
			return Stop{}, err
		}
		over := s.breaks[pc] != nil
		if !over && s.breaks[pc+4] != nil {
			b, err := t.ReadMemory(ctx, pc, 4)
			if err != nil {
				return Stop{}, err
			}
			over = isBranch(binary.LittleEndian.Uint32(b))
		}
		if over {
			if err := t.Step(ctx); err != nil {
				return Stop{}, err
			}
			stop, err := t.Wait(ctx)
			if err != nil || stop.Signal != SIGTRAP {
				return stop, err
			}
			if pc, err = t.ReadRegister(ctx, RegPC); err != nil {
				return Stop{}, err
			}
			if s.breaks[pc] != nil {
				return stop, nil // the next instruction has one too
			}
		}
		if err := s.plant(ctx); err != nil {
			return Stop{}, errors.Join(err, s.unplant(ctx))
		}
	}
	if err := t.Continue(ctx); err != nil {
		return Stop{}, errors.Join(err, s.unplant(ctx))
	}
	stop, err := t.Wait(ctx)
	return stop, errors.Join(err, s.unplant(ctx))
}

// plant writes the server's breakpoints into the program.
func (s *Server) plant(ctx context.Context) error {
	for addr, b := range s.breaks {
		saved, err := s.Target.ReadMemory(ctx, addr, 4)
		if err != nil {
			return err
		}
		copy(b.saved[:], saved)
		if err := s.Target.WriteMemory(ctx, addr, binary.LittleEndian.AppendUint32(nil, breakInsn)); err != nil {
			return err
		}
		b.planted = true
	}
	return nil
}

// unplant puts back the words the breakpoints replaced.
func (s *Server) unplant(ctx context.Context) error {
	var errs error
	for addr, b := range s.breaks {
		if !b.planted {
			continue
		}
		if err := s.Target.WriteMemory(ctx, addr, b.saved[:]); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		b.planted = false
	}
	return errs
}

func (s *Server) clearPlaced() {
	if t, ok := s.Target.(Breakpointer); ok {
		for addr, kind := range s.placed {
			t.ClearBreakpoint(kind, addr, 4)
		}
	}
	clear(s.placed)
}

// reg returns a register of the stopped program, read once a stop.
func (s *Server) reg(ctx context.Context, n int) (uint32, error) {
	if !s.known[n] {
		v, err := s.Target.ReadRegister(ctx, n)
		if err != nil {
			return 0, err
		}
		s.regs[n], s.known[n] = v, true
	}
	return s.regs[n], nil
}

func (s *Server) setReg(ctx context.Context, n int, v uint32) error {
	if s.known[n] && s.regs[n] == v {
		return nil
	}
	if err := s.Target.WriteRegister(ctx, n, v); err != nil {
		return err
	}
	s.regs[n], s.known[n] = v, true
	return nil
}

func hexWord(v uint32) string {
	return hex.EncodeToString(binary.LittleEndian.AppendUint32(nil, v))
}

// stopReply is the packet that says the program stopped, with the
// registers a debugger reads first, the pc, the stack pointer and the
// frame pointer, to save it asking, and which kind of breakpoint it is at,
// if the debugger said it knows those.
func (s *Server) stopReply(ctx context.Context, stop Stop) string {
	var b strings.Builder
	fmt.Fprintf(&b, "T%02x", stop.Signal)
	for _, n := range []int{RegPC, 29, 30} {
		v, err := s.reg(ctx, n)
		if err != nil {
			return fmt.Sprintf("S%02x", stop.Signal)
		}
		fmt.Fprintf(&b, "%02x:%s;", n, hexWord(v))
	}
	b.WriteString("thread:1;")
	if stop.Signal == SIGTRAP {
		pc := s.regs[RegPC]
		kind, placed := s.placed[pc]
		switch {
		case s.swbreak && (s.breaks[pc] != nil || placed && kind == BreakSoftware):
			b.WriteString("swbreak:;")
		case s.hwbreak && placed && kind == BreakHardware:
			b.WriteString("hwbreak:;")
		}
	}
	return b.String()
}

// What the server does after a packet, beyond replying.
const (
	actNone = iota
	actStep
	actContinue
	actDetach
	actKill
)

const (
	replyOK  = "OK"
	replyErr = "E01"

	// unavailable is a register the target does not have, in a reply.
	unavailable = "xxxxxxxx"
)

// handle does what a packet asks and returns the reply, or what to do in
// place of one. An empty reply says the packet is not supported, which is
// how a debugger learns what a server has.
func (s *Server) handle(ctx context.Context, c *conn, p []byte) (string, int) {
	if len(p) == 0 {
		return "", actNone
	}
	t := s.Target
	arg := string(p[1:])
	switch p[0] {
	case '?':
		return s.stopReply(ctx, s.last), actNone
	case 'g':
		var b strings.Builder
		for n := range NumRegs {
			v, err := s.reg(ctx, n)
			if err != nil {
				return replyErr, actNone
			}
			b.WriteString(hexWord(v))
		}
		b.WriteString(strings.Repeat(unavailable, fpuRegs))
		return b.String(), actNone
	case 'G':
		// The registers there are, which come first: the rest is the
		// floating point unit's.
		if len(arg) < NumRegs*8 {
			return replyErr, actNone
		}
		data, err := hex.DecodeString(arg[:NumRegs*8])
		if err != nil {
			return replyErr, actNone
		}
		for n := range NumRegs {
			if err := s.setReg(ctx, n, binary.LittleEndian.Uint32(data[n*4:])); err != nil {
				return replyErr, actNone
			}
		}
		return replyOK, actNone
	case 'p':
		n, err := strconv.ParseUint(arg, 16, 32)
		switch {
		case err != nil || n >= NumRegs+fpuRegs:
			return replyErr, actNone
		case n >= NumRegs:
			return unavailable, actNone
		}
		v, err := s.reg(ctx, int(n))
		if err != nil {
			return replyErr, actNone
		}
		return hexWord(v), actNone
	case 'P':
		num, val, _ := strings.Cut(arg, "=")
		n, err := strconv.ParseUint(num, 16, 32)
		data, err2 := hex.DecodeString(val)
		if err != nil || err2 != nil || n >= NumRegs+fpuRegs || len(data) != 4 {
			return replyErr, actNone
		}
		if n >= NumRegs {
			return replyOK, actNone // nowhere to keep it
		}
		if err := s.setReg(ctx, int(n), binary.LittleEndian.Uint32(data)); err != nil {
			return replyErr, actNone
		}
		return replyOK, actNone
	case 'm':
		addr, n, ok := addrLen(arg)
		if !ok || !accessible(addr, n) {
			return replyErr, actNone
		}
		data, err := t.ReadMemory(ctx, addr, n)
		if err != nil {
			return replyErr, actNone
		}
		return hex.EncodeToString(data), actNone
	case 'M', 'X':
		head, body, _ := bytes.Cut(p[1:], []byte(":"))
		addr, n, ok := addrLen(string(head))
		if !ok {
			return replyErr, actNone
		}
		var data []byte
		if p[0] == 'X' {
			data = unescape(body)
		} else if d, err := hex.DecodeString(string(body)); err == nil {
			data = d
		}
		if len(data) != n || !accessible(addr, n) {
			return replyErr, actNone
		}
		if n > 0 {
			if err := t.WriteMemory(ctx, addr, data); err != nil {
				return replyErr, actNone
			}
		}
		return replyOK, actNone
	case 'c', 's', 'C', 'S':
		// An address after c or s, or after the signal of C or S and a
		// semicolon, is where to go on from.
		at := arg
		if p[0] == 'C' || p[0] == 'S' {
			_, at, _ = strings.Cut(arg, ";")
		}
		if at != "" {
			pc, err := strconv.ParseUint(at, 16, 64)
			if err != nil || s.setReg(ctx, RegPC, uint32(pc)) != nil {
				return replyErr, actNone
			}
		}
		if p[0] == 's' || p[0] == 'S' {
			return "", actStep
		}
		return "", actContinue
	case 'Z', 'z':
		return s.breakpoint(p[0] == 'Z', arg), actNone
	case 'D':
		return "", actDetach
	case 'k':
		return "", actKill
	case 'H', 'T':
		return replyOK, actNone
	case 'v':
		switch {
		case arg == "Cont?":
			return "vCont;c;C;s;S", actNone
		case strings.HasPrefix(arg, "Cont;"):
			// One thread, so the first action is the only one.
			switch arg[len("Cont;")] {
			case 's', 'S':
				return "", actStep
			case 'c', 'C':
				return "", actContinue
			}
			return replyErr, actNone
		case strings.HasPrefix(arg, "Kill"):
			c.write([]byte(replyOK))
			return "", actKill
		}
	case 'q':
		return s.query(arg), actNone
	case 'Q':
		if arg == "StartNoAckMode" {
			// From the next packet on, which may come as soon as the
			// reply has gone: this one was acknowledged as it was read.
			c.noAck.Store(true)
			return replyOK, actNone
		}
	}
	return "", actNone
}

// addrLen parses the "addr,length" of the memory packets.
func addrLen(s string) (uint32, int, bool) {
	a, l, ok := strings.Cut(s, ",")
	addr, err := strconv.ParseUint(a, 16, 64)
	n, err2 := strconv.ParseUint(l, 16, 32)
	return uint32(addr), int(n), ok && err == nil && err2 == nil && n <= packetSize
}

// breakpoint sets or clears a breakpoint: "kind,addr,size". The target
// keeps it if it has that kind, and otherwise a software breakpoint is the
// server's own.
func (s *Server) breakpoint(set bool, arg string) string {
	f := strings.Split(arg, ",")
	if len(f) < 3 {
		return replyErr
	}
	kind, err := strconv.Atoi(f[0])
	addr64, err2 := strconv.ParseUint(f[1], 16, 64)
	size, err3 := strconv.ParseUint(strings.SplitN(f[2], ";", 2)[0], 16, 32)
	if err != nil || err2 != nil || err3 != nil {
		return replyErr
	}
	addr := uint32(addr64)
	if t, ok := s.Target.(Breakpointer); ok {
		if set {
			err = t.SetBreakpoint(kind, addr, int(size))
		} else {
			err = t.ClearBreakpoint(kind, addr, int(size))
		}
		switch {
		case errors.Is(err, ErrUnsupported):
			return ""
		case err != nil:
			return replyErr
		}
		if set {
			s.placed[addr] = kind
		} else {
			delete(s.placed, addr)
		}
		return replyOK
	}
	if kind != BreakSoftware {
		return ""
	}
	if !inRAM(addr) {
		return replyErr
	}
	if set {
		if s.breaks[addr] == nil {
			s.breaks[addr] = &softBreak{}
		}
	} else {
		delete(s.breaks, addr)
	}
	return replyOK
}

// query answers the q packets: what the server supports, the one thread
// there is, and the target description. There is no memory map: GDB keeps
// a MIPS's addresses sign-extended to 64 bits, which no region in one
// would match.
func (s *Server) query(arg string) string {
	name, rest, _ := strings.Cut(arg, ":")
	switch name {
	case "Supported":
		s.swbreak = strings.Contains(rest, "swbreak+")
		s.hwbreak = strings.Contains(rest, "hwbreak+")
		reply := fmt.Sprintf("PacketSize=%x;QStartNoAckMode+;qXfer:features:read+;vContSupported+;swbreak+", packetSize)
		if _, ok := s.Target.(Breakpointer); ok {
			reply += ";hwbreak+"
		}
		return reply
	case "Xfer":
		f := strings.Split(rest, ":")
		if len(f) != 4 || f[1] != "read" {
			return ""
		}
		var doc string
		switch {
		case f[0] == "features" && f[2] == "target.xml":
			doc = targetXML
		case f[0] == "features":
			return "E00"
		default:
			return ""
		}
		off, n, ok := addrLen(f[3])
		if !ok {
			return replyErr
		}
		if int(off) >= len(doc) {
			return "l"
		}
		chunk := doc[off:]
		more := "l"
		if len(chunk) > n {
			chunk, more = chunk[:n], "m"
		}
		return more + string(escape([]byte(chunk)))
	case "C":
		return "QC1"
	case "fThreadInfo":
		return "m1"
	case "sThreadInfo":
		return "l"
	case "Attached":
		return "1"
	case "Symbol":
		return replyOK
	}
	return ""
}
