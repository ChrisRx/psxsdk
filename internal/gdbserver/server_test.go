package gdbserver

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

// The fake target's instructions, beyond the nop every other word is: a
// jump, and a system call that prints.
const (
	fakeBase  = 0x80010000
	fakeSize  = 0x100
	fakePrint = 0x0000000c
)

func jump(to uint32) uint32 { return 2<<26 | to&0x0fffffff>>2 }

// fake is a target with a little memory and a CPU of four instructions,
// which stops as a console's does: at a break, for a trap, with the pc at
// it, or at the jump before it when the break is in a delay slot.
type fake struct {
	mem       [fakeSize]byte
	regs      [NumRegs]uint32
	out       io.Writer
	going     bool
	interrupt atomic.Bool
	reads     int
}

func newFake(code ...uint32) *fake {
	f := &fake{}
	for i, w := range code {
		binary.LittleEndian.PutUint32(f.mem[i*4:], w)
	}
	f.regs[RegPC] = fakeBase
	return f
}

func (f *fake) word(addr uint32) uint32 { return binary.LittleEndian.Uint32(f.mem[addr-fakeBase:]) }

func (f *fake) SetOutput(w io.Writer) { f.out = w }

func (f *fake) ReadRegister(ctx context.Context, n int) (uint32, error) {
	f.reads++
	return f.regs[n], nil
}

func (f *fake) WriteRegister(ctx context.Context, n int, v uint32) error {
	f.regs[n] = v
	return nil
}

func (f *fake) ReadMemory(ctx context.Context, addr uint32, n int) ([]byte, error) {
	if addr < fakeBase || addr+uint32(n) > fakeBase+fakeSize {
		return nil, fmt.Errorf("no memory at %08x", addr)
	}
	return append([]byte(nil), f.mem[addr-fakeBase:][:n]...), nil
}

func (f *fake) WriteMemory(ctx context.Context, addr uint32, data []byte) error {
	if addr < fakeBase || addr+uint32(len(data)) > fakeBase+fakeSize {
		return fmt.Errorf("no memory at %08x", addr)
	}
	copy(f.mem[addr-fakeBase:], data)
	return nil
}

// run runs an instruction, and a jump's delay slot with it, and reports
// whether it stopped at a break in place of running it.
func (f *fake) run() (stopped bool) {
	pc := f.regs[RegPC]
	insn := f.word(pc)
	next := pc + 4
	if isBranch(insn) {
		if f.word(pc+4) == breakInsn {
			return true
		}
		next = pc&0xf0000000 | insn&0x03ffffff<<2
	}
	switch insn {
	case breakInsn:
		return true
	case fakePrint:
		f.out.Write([]byte("tick\n"))
	}
	f.regs[RegPC] = next
	return false
}

func (f *fake) Step(ctx context.Context) error {
	f.going = false
	f.run()
	return nil
}

func (f *fake) Continue(ctx context.Context) error {
	f.going = true
	return nil
}

func (f *fake) Interrupt() error {
	f.interrupt.Store(true)
	return nil
}

func (f *fake) Wait(ctx context.Context) (Stop, error) {
	for f.going {
		if f.interrupt.Swap(false) {
			return Stop{Signal: SIGINT}, nil
		}
		if f.run() {
			break
		}
		if err := ctx.Err(); err != nil {
			return Stop{}, err
		}
		time.Sleep(10 * time.Microsecond)
	}
	return Stop{Signal: SIGTRAP}, nil
}

// client is a debugger's end of a connection.
type client struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

// setup serves target to a client on a pipe, and returns the client.
func setup(t *testing.T, target Target) (*Server, *client) {
	t.Helper()
	s := New(target)
	return s, connect(t, s)
}

// connect starts a session of s with a new client.
func connect(t *testing.T, s *Server) *client {
	t.Helper()
	a, b := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Session(ctx, b)
		b.Close()
	}()
	t.Cleanup(func() {
		a.Close()
		cancel()
		<-done
	})
	a.SetDeadline(time.Now().Add(10 * time.Second))
	return &client{
		t:    t,
		conn: a,
		r:    bufio.NewReader(a),
	}
}

// send sends a packet and waits for its acknowledgment.
func (c *client) send(payload string) {
	c.t.Helper()
	_, err := fmt.Fprintf(c.conn, "$%s#%02x", payload, checksum([]byte(payload)))
	assert.NoError(c.t, err)
	b, err := c.r.ReadByte()
	assert.NoError(c.t, err)
	assert.Equal(c.t, byte('+'), b, "the acknowledgment of "+payload)
}

// receive returns the next packet's payload, acknowledged.
func (c *client) receive() string {
	c.t.Helper()
	b, err := c.r.ReadByte()
	if !assert.NoError(c.t, err) {
		c.t.FailNow()
	}
	assert.Equal(c.t, byte('$'), b)
	payload, err := c.r.ReadString('#')
	assert.NoError(c.t, err)
	var sum [2]byte
	_, err = io.ReadFull(c.r, sum[:])
	assert.NoError(c.t, err)
	payload = strings.TrimSuffix(payload, "#")
	assert.Equal(c.t, fmt.Sprintf("%02x", checksum([]byte(payload))), string(sum[:]))
	c.conn.Write([]byte("+"))
	return payload
}

// ask sends a packet and returns the reply.
func (c *client) ask(payload string) string {
	c.t.Helper()
	c.send(payload)
	return c.receive()
}

// pc is the pc a stop reply gives.
func pc(t *testing.T, reply string) uint32 {
	t.Helper()
	_, v, ok := strings.Cut(reply, "25:")
	if !ok || len(v) < 8 {
		t.Fatalf("no pc in %q", reply)
	}
	b, err := hex.DecodeString(v[:8])
	assert.NoError(t, err)
	return binary.LittleEndian.Uint32(b)
}

func TestQueries(t *testing.T) {
	_, c := setup(t, newFake())
	reply := c.ask("qSupported:swbreak+;hwbreak+")
	assert.Equal(t, true, strings.Contains(reply, "qXfer:features:read+"), reply)
	assert.Equal(t, false, strings.Contains(reply, "hwbreak+"), "a target with no breakpoints of its own")
	assert.Equal(t, "vCont;c;C;s;S", c.ask("vCont?"))
	assert.Equal(t, "", c.ask("vMustReplyEmpty"))
	assert.Equal(t, "QC1", c.ask("qC"))
	assert.Equal(t, "m1", c.ask("qfThreadInfo"))
	assert.Equal(t, "l", c.ask("qsThreadInfo"))
	assert.Equal(t, "1", c.ask("qAttached"))
	assert.Equal(t, true, strings.HasPrefix(c.ask("?"), "T05"))

	// The description comes in the pieces asked for.
	var doc string
	for {
		reply := c.ask(fmt.Sprintf("qXfer:features:read:target.xml:%x,200", len(doc)))
		doc += reply[1:]
		if reply[0] == 'l' {
			break
		}
		assert.Equal(t, byte('m'), reply[0])
	}
	assert.Equal(t, targetXML, doc)
	assert.Equal(t, true, strings.Contains(doc, "<architecture>mips:3000</architecture>"))
	assert.Equal(t, 72, strings.Count(doc, "<reg "), "the 38 registers, and the 34 of the unit GDB insists on")
}

func TestChecksum(t *testing.T) {
	_, c := setup(t, newFake())
	c.conn.Write([]byte("$qC#00"))
	b, err := c.r.ReadByte()
	assert.NoError(t, err)
	assert.Equal(t, byte('-'), b, "a packet whose sum is wrong")
	assert.Equal(t, "QC1", c.ask("qC"))

	// With acknowledgments off, there are none, and sums are not checked.
	assert.Equal(t, "OK", c.ask("QStartNoAckMode"))
	c.conn.Write([]byte("$qC#00"))
	assert.Equal(t, "QC1", c.receive())
}

func TestRegisters(t *testing.T) {
	f := newFake()
	f.regs[4], f.regs[RegSR] = 0x12345678, 0x40000401
	_, c := setup(t, f)
	g := c.ask("g")
	assert.Equal(t, (NumRegs+fpuRegs)*8, len(g))
	assert.Equal(t, "78563412", g[4*8:5*8])
	assert.Equal(t, "00000180", g[RegPC*8:RegPC*8+8])
	assert.Equal(t, strings.Repeat("x", fpuRegs*8), g[NumRegs*8:])
	assert.Equal(t, "01040040", c.ask("p20"))
	assert.Equal(t, "xxxxxxxx", c.ask("p26"), "f0")
	assert.Equal(t, "E01", c.ask("p48"))

	assert.Equal(t, "OK", c.ask("P5=efbeadde"))
	assert.Equal(t, uint32(0xdeadbeef), f.regs[5])
	assert.Equal(t, "efbeadde", c.ask("p5"))

	// A register is read from the target once a stop.
	reads := f.reads
	c.ask("g")
	c.ask("p4")
	assert.Equal(t, reads, f.reads)

	g = g[:RegPC*8] + "04000180" + g[RegPC*8+8:]
	assert.Equal(t, "OK", c.ask("G"+g))
	assert.Equal(t, uint32(fakeBase+4), f.regs[RegPC])
}

func TestMemory(t *testing.T) {
	f := newFake(0x11223344, 0x55667788)
	_, c := setup(t, f)
	assert.Equal(t, "4433221188776655", c.ask("m80010000,8"))
	assert.Equal(t, "OK", c.ask("M80010004,2:abcd"))
	assert.Equal(t, uint32(0x5566cdab), f.word(fakeBase+4))
	// A binary write, with the bytes the framing would trip on escaped.
	assert.Equal(t, "OK", c.ask("X80010008,4:\x01}\x03}\x04}]"))
	assert.Equal(t, []byte{0x01, '#', '$', '}'}, f.mem[8:12])
	assert.Equal(t, "OK", c.ask("X80010008,0:"), "the probe GDB sends first")

	// Memory a console has none at is refused before the target is asked.
	assert.Equal(t, "E01", c.ask("m0,4"), "RAM the fake does not have")
	assert.Equal(t, "E01", c.ask("m40000000,4"))
	assert.Equal(t, "E01", c.ask("m801ffffe,4"), "over the end of RAM")
	assert.Equal(t, "E01", c.ask("M40000000,1:00"))
}

func TestStepAndBreakpoint(t *testing.T) {
	// 0: nop, 4: nop, 8: nop, c: jump to 0, 10: its delay slot.
	f := newFake(0, 0, 0, jump(fakeBase), 0)
	_, c := setup(t, f)
	c.ask("qSupported:swbreak+")
	assert.Equal(t, uint32(fakeBase+4), pc(t, c.ask("s")))
	assert.Equal(t, uint32(fakeBase+8), pc(t, c.ask("vCont;s:1")))

	assert.Equal(t, "", c.ask("Z1,80010004,4"), "no hardware breakpoints")
	assert.Equal(t, "", c.ask("Z2,80010004,4"), "no watchpoints")
	assert.Equal(t, "E01", c.ask("Z0,40000000,4"), "not in RAM")
	assert.Equal(t, "OK", c.ask("Z0,80010004,4"))
	assert.Equal(t, uint32(0), f.word(fakeBase+4), "nothing is written while the program is stopped")

	reply := c.ask("c")
	assert.Equal(t, uint32(fakeBase+4), pc(t, reply))
	assert.Equal(t, true, strings.HasPrefix(reply, "T05") && strings.HasSuffix(reply, "swbreak:;"), reply)
	assert.Equal(t, uint32(0), f.word(fakeBase+4), "and it is taken out again at the stop")
	assert.Equal(t, "00000000", c.ask("m80010004,4"))

	// Going on from a breakpoint steps past it first, round to it again.
	assert.Equal(t, uint32(fakeBase+4), pc(t, c.ask("c")))
	// A breakpoint on the next instruction too is hit by that step.
	assert.Equal(t, "OK", c.ask("Z0,80010008,4"))
	assert.Equal(t, uint32(fakeBase+8), pc(t, c.ask("c")))
	assert.Equal(t, "OK", c.ask("z0,80010008,4"))
	assert.Equal(t, "OK", c.ask("z0,80010004,4"))

	// c and s take an address to go on from.
	assert.Equal(t, uint32(fakeBase+4), pc(t, c.ask("s80010000")))
}

// TestDelaySlotBreakpoint puts a breakpoint in a jump's delay slot, where
// the CPU reports the jump as the place it stopped: going on has to step
// the two together, or it would stop there for ever.
func TestDelaySlotBreakpoint(t *testing.T) {
	f := newFake(0, 0, 0, jump(fakeBase), 0)
	_, c := setup(t, f)
	assert.Equal(t, "OK", c.ask("Z0,80010010,4"))
	assert.Equal(t, uint32(fakeBase+0xc), pc(t, c.ask("c")), "stopped at the jump")
	assert.Equal(t, "OK", c.ask("Z0,80010004,4"))
	assert.Equal(t, uint32(fakeBase+4), pc(t, c.ask("c")), "past the slot, to the next breakpoint")
	assert.Equal(t, uint32(fakeBase+0xc), pc(t, c.ask("c")))
}

func TestInterruptAndOutput(t *testing.T) {
	// A loop that prints each time round.
	f := newFake(fakePrint, jump(fakeBase), 0)
	_, c := setup(t, f)
	c.send("c")
	assert.Equal(t, "O"+hex.EncodeToString([]byte("tick\n")), c.receive())
	c.conn.Write([]byte{0x03})
	var reply string
	for reply = c.receive(); strings.HasPrefix(reply, "O"); reply = c.receive() {
	}
	assert.Equal(t, true, strings.HasPrefix(reply, "T02"), reply)
	assert.Equal(t, reply, c.ask("?"))
}

// TestDetach leaves the program running, with the breakpoints gone, and
// the next debugger finds it stopped for it.
func TestDetach(t *testing.T) {
	f := newFake(0, jump(fakeBase), 0)
	s, c := setup(t, f)
	var printed strings.Builder
	s.Stdout = &printed
	assert.Equal(t, "OK", c.ask("Z0,80010000,4"))
	assert.Equal(t, "OK", c.ask("D"))
	c.conn.Close()

	c = connect(t, s)
	reply := c.ask("?")
	assert.Equal(t, true, strings.HasPrefix(reply, "T02"), reply)
	assert.Equal(t, uint32(0), f.word(fakeBase))
}
