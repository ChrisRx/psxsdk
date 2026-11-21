package yaroze

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"debug/elf"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-dap"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/internal/ecoffelf"
	"go.chrisrx.dev/psx/internal/gdbserver"
	"go.chrisrx.dev/psx/link"
)

// The debuggers yaroze debug runs itself: GDB for WebAssembly on a C
// program, and Delve's debug adapter on a Go one.

// served is a program in the emulator, served to debuggers until the test
// ends.
type served struct {
	server *gdbserver.Server
	addr   string
}

// serve serves target on a loopback port until the test ends.
func serve(t *testing.T, target gdbserver.Target) served {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := gdbserver.New(target)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Serve(ctx, l)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return served{s, l.Addr().String()}
}

// output is what a debugger has printed, written by it and read by the
// test meanwhile.
type output struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.Write(p)
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.String()
}

// waitFor waits for the output to have want in it n times.
func (o *output) waitFor(t *testing.T, want string, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for strings.Count(o.String(), want) < n {
		if time.Now().After(deadline) {
			t.Fatalf("no %q after a minute:\n%s", want, o.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestDebugGDB runs GDB for WebAssembly as yaroze debug does, on a program
// GCC 2.95.3 built, through its ELF with DWARF, and has it stop at a
// breakpoint, then stops the running program as the terminal's Control-C
// does. YAROZE_GDB names the gdb.wasm, rather than the test downloading it.
func TestDebugGDB(t *testing.T) {
	if os.Getenv("YAROZE_GDB") == "" {
		t.Skip("YAROZE_GDB names no gdb.wasm")
	}
	const prog = "../../../format/ecoff/testdata/debug/steps"
	f, err := ecoff.Open(prog)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer f.Close()
	data, err := ecoffelf.Convert(f)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	elfPath := filepath.Join(t.TempDir(), "steps.elf")
	assert.NoError(t, os.WriteFile(elfPath, data, 0o644))
	exe, err := link.Runnable(prog)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m, err := emu.New(openbios.Image)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.NoError(t, m.SideloadEXE(exe, 50_000_000)) {
		t.FailNow()
	}
	e := gdbserver.NewEmulator(m)
	e.Fast = true
	if !assert.NoError(t, e.RunTo(f.Entry)) {
		t.FailNow()
	}
	s := serve(t, e)

	stdin, commands := io.Pipe()
	var out output
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	go func() { done <- runGDB(ctx, s.server, s.addr, elfPath, nil, stdin, &out, &out) }()
	send := func(lines ...string) {
		t.Helper()
		_, err := io.WriteString(commands, strings.Join(lines, "\n")+"\n")
		assert.NoError(t, err)
	}

	send("break tick", "continue")
	out.waitFor(t, "Breakpoint 1, tick (n=", 1)
	send("bt", "delete", "continue")
	out.waitFor(t, "Continuing.", 2)
	// What the terminal's Control-C does while the program runs.
	time.Sleep(100 * time.Millisecond)
	assert.NoError(t, s.server.Interrupt())
	out.waitFor(t, "SIGINT", 1)
	send("bt", "kill", "quit")
	commands.Close()
	assert.NoError(t, <-done)
	t.Log(out.String())
	for _, want := range []string{
		"Breakpoint 1, tick (n=",
		"in main () at steps.c:45",
		"Program received signal SIGINT",
	} {
		assert.Equal(t, true, strings.Contains(out.String(), want), want)
	}
	// Stopped in the program, not in the kernel's interrupt handler, the
	// backtrace reaches main.
	_, after, _ := strings.Cut(out.String(), "Program received signal SIGINT")
	assert.Equal(t, false, strings.Contains(after, "in ?? ()"), after)
	assert.Equal(t, true, strings.Contains(after, "main () at steps.c:4"), after)
}

// TestDebugDAP has Delve's debug adapter, as yaroze debug --dap serves it,
// stop a Go program at a line and read a variable, as an editor would.
func TestDebugDAP(t *testing.T) {
	builds := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../../../.build"), "debug")
	prog, err := filepath.Abs(filepath.Join(builds, "steps.elf"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if _, err := os.Stat(prog); err != nil {
		t.Skipf("no build of steps (task build:debug): %v", err)
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(filepath.Join(builds, "steps.cue")))
	f, err := elf.Open(prog)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	f.Close()
	e := gdbserver.NewEmulator(m.Machine)
	e.Fast = true
	if !assert.NoError(t, e.RunTo(uint32(f.Entry))) {
		t.FailNow()
	}
	s := serve(t, e)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- serveDAP(ctx, l, s.addr, prog) }()

	conn, err := net.Dial("tcp", l.Addr().String())
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer conn.Close()
	c := &dapClient{t: t, conn: conn, r: bufio.NewReader(conn)}
	source, err := filepath.Abs("../../../tests/testdata/debug/steps/main.go")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	c.send(&dap.InitializeRequest{Request: c.request("initialize"), Arguments: dap.InitializeRequestArguments{
		AdapterID:       "go",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:      "path",
	}})
	c.wait(func(m dap.Message) bool { _, ok := m.(*dap.InitializeResponse); return ok })
	c.send(&dap.LaunchRequest{Request: c.request("launch"), Arguments: []byte(`{"request":"launch"}`)})
	c.wait(func(m dap.Message) bool { _, ok := m.(*dap.InitializedEvent); return ok })
	c.send(&dap.SetBreakpointsRequest{Request: c.request("setBreakpoints"), Arguments: dap.SetBreakpointsArguments{
		Source:      dap.Source{Path: source},
		Breakpoints: []dap.SourceBreakpoint{{Line: 34}},
	}})
	bps := c.wait(func(m dap.Message) bool { _, ok := m.(*dap.SetBreakpointsResponse); return ok }).(*dap.SetBreakpointsResponse)
	assert.Equal(t, 1, len(bps.Body.Breakpoints))
	if len(bps.Body.Breakpoints) != 1 || !bps.Body.Breakpoints[0].Verified {
		t.Fatalf("breakpoints %+v", bps.Body.Breakpoints)
	}
	c.send(&dap.ConfigurationDoneRequest{Request: c.request("configurationDone")})
	stopped := c.wait(func(m dap.Message) bool { _, ok := m.(*dap.StoppedEvent); return ok }).(*dap.StoppedEvent)
	assert.Equal(t, "breakpoint", stopped.Body.Reason)

	c.send(&dap.StackTraceRequest{Request: c.request("stackTrace"), Arguments: dap.StackTraceArguments{ThreadId: stopped.Body.ThreadId, Levels: 4}})
	st := c.wait(func(m dap.Message) bool { _, ok := m.(*dap.StackTraceResponse); return ok }).(*dap.StackTraceResponse)
	if len(st.Body.StackFrames) == 0 {
		t.Fatal("no stack frames")
	}
	assert.Equal(t, 34, st.Body.StackFrames[0].Line)
	c.send(&dap.EvaluateRequest{Request: c.request("evaluate"), Arguments: dap.EvaluateArguments{
		Expression: "main.frames",
		FrameId:    st.Body.StackFrames[0].Id,
		Context:    "watch",
	}})
	ev := c.wait(func(m dap.Message) bool { _, ok := m.(*dap.EvaluateResponse); return ok }).(*dap.EvaluateResponse)
	assert.Equal(t, "0", ev.Body.Result)
	c.send(&dap.DisconnectRequest{Request: c.request("disconnect"), Arguments: &dap.DisconnectArguments{}})
	c.wait(func(m dap.Message) bool { _, ok := m.(*dap.DisconnectResponse); return ok })
	assert.NoError(t, <-served)
}

// dapClient speaks the debug adapter protocol to a server, as an editor.
type dapClient struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
	seq  int
}

func (c *dapClient) request(command string) dap.Request {
	c.seq++
	return dap.Request{ProtocolMessage: dap.ProtocolMessage{Seq: c.seq, Type: "request"}, Command: command}
}

func (c *dapClient) send(m dap.Message) {
	c.t.Helper()
	if !assert.NoError(c.t, dap.WriteProtocolMessage(c.conn, m)) {
		c.t.FailNow()
	}
}

// wait reads messages until one that match accepts, failing on a failed
// response.
func (c *dapClient) wait(match func(dap.Message) bool) dap.Message {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(time.Minute))
	for {
		m, err := dap.ReadProtocolMessage(c.r)
		if !assert.NoError(c.t, err) {
			c.t.FailNow()
		}
		if r, ok := m.(dap.ResponseMessage); ok && !r.GetResponse().Success {
			c.t.Fatalf("%s failed: %s", r.GetResponse().Command, r.GetResponse().Message)
		}
		if er, ok := m.(*dap.ErrorResponse); ok {
			c.t.Fatalf("%s failed: %s", er.Command, er.Message)
		}
		if match(m) {
			return m
		}
	}
}
