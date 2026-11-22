package tests

import (
	"bytes"
	"cmp"
	"context"
	"debug/elf"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/internal/ecoffelf"
	"go.chrisrx.dev/psx/internal/gdbserver"
	"go.chrisrx.dev/psx/link"
	"go.chrisrx.dev/psx/serial"
)

// GDB debugging a Go program through yaroze debug's server
// (internal/gdbserver), on each of its targets: the emulated machine itself,
// and the monitor's remote debugger on an emulated serial cable. The
// program is tests/testdata/debug/steps, as task build:debug builds it,
// with DWARF.

// gdbCommand returns the gdb to run, PSX_GDB or the one on the path, or
// skips the test without one that knows the MIPS: nixpkgs' does (nix shell
// nixpkgs#gdb).
func gdbCommand(t *testing.T) string {
	t.Helper()
	path := cmp.Or(os.Getenv("PSX_GDB"), "gdb")
	path, err := exec.LookPath(path)
	if err != nil {
		t.Skip("no gdb: PSX_GDB names one, or put one on the path")
	}
	if out, err := exec.Command(path, "-nx", "-batch", "-ex", "set architecture mips:3000").CombinedOutput(); err != nil {
		t.Skipf("%s does not know the MIPS: %s", path, out)
	}
	return path
}

// debugProgram returns the path of a build of the steps program under
// .build/debug, or skips the test if it has not been built.
func debugProgram(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../.build"), "debug", name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no build of %s: %v", name, err)
	}
	return path
}

// serveGDB serves target to debuggers on a port of the loopback address,
// which it returns, until the test ends.
func serveGDB(t *testing.T, target gdbserver.Target) string {
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
	return l.Addr().String()
}

// output is what gdb has printed, written by the command and read by the
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

// runGDB runs gdb on the program's ELF against the server at addr, with
// commands as its script, and returns what it printed. If interruptAt is
// not empty, gdb is sent an interrupt, as Control-C at its prompt is, once
// it has printed that.
func runGDB(t *testing.T, elf, addr string, commands []string, interruptAt string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "script.gdb")
	assert.NoError(t, os.WriteFile(script, []byte(strings.Join(commands, "\n")+"\n"), 0o644))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, gdbCommand(t), "-nx", "-batch",
		"-ex", "set pagination off",
		"-ex", "set confirm off",
		"-ex", "set remotetimeout 30",
		"-ex", "target remote "+addr,
		"-x", script, elf)
	var out output
	cmd.Stdout, cmd.Stderr = &out, &out
	if !assert.NoError(t, cmd.Start()) {
		t.FailNow()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for interruptAt != "" {
		select {
		case err := <-done:
			t.Fatalf("gdb ended before printing %q: %v\n%s", interruptAt, err, out.String())
		case <-time.After(20 * time.Millisecond):
		}
		if strings.Contains(out.String(), interruptAt) {
			assert.NoError(t, cmd.Process.Signal(syscall.SIGINT))
			break
		}
	}
	if err := <-done; err != nil {
		t.Errorf("gdb: %v", err)
	}
	return out.String()
}

// The script every target is put through, and what gdb must print for
// it, in order. The lines are main.go's, in tests/testdata/debug/steps.
var (
	gdbScript = []string{
		// A breakpoint on a line, a backtrace through the inlined frames,
		// and a global.
		"break main.go:34",
		"continue",
		"bt",
		"print 'main.frames'",
		// Over a line, to a line in a function, into the function it
		// calls and out again.
		"next",
		"tbreak main.go:23",
		"continue",
		"step",
		"finish",
		// Over the branch an instruction at a time: the first frame is
		// odd, so it is not taken.
		"stepi",
		"stepi",
		"echo odd:\\n",
		"info line *$pc",
		"finish",
		"print 'main.total'",
		// A breakpoint on a function, and the branch again, taken.
		"delete",
		"break main.tick",
		"continue",
		"print n",
		"tbreak main.go:24",
		"continue",
		"stepi",
		"stepi",
		"echo even:\\n",
		"info line *$pc",
		// Memory written, and what the program prints.
		"set var 'main.frames' = 58",
		"delete",
		"break main.go:37",
		"continue",
		"print 'main.frames'",
		"delete",
		"break main.go:34",
		"continue",
		"print 'main.frames'",
		"delete",
	}
	gdbWant = []string{
		"Breakpoint 1, main.main$1 (",
		"main.go:34",
		"frames++",
		"#0  main.main$1 (",
		"in main.main ()",
		"runtime.run ()",
		"$1 = 0",
		"tick(frames)",
		"main.tick (n=1) at",
		"total = add(total, n)",
		"main.add (a=0, b=1) at",
		"return a + b",
		"main.tick (n=1) at",
		"if n&1 != 0 {",
		"odd:\n",
		"Line 25 of",
		"main.main$1 (",
		"if frames%60 == 0 {",
		"$2 = 1",
		"main.tick (n=2)",
		"$3 = 2",
		"even:\n",
		"Line 27 of",
		"println(\"debug: second\", frames/60)",
		"$4 = 60",
		"debug: second 1",
		"$5 = 60",
	}

	// gdbInterrupt goes on the end for a target that can stop a running
	// program: gdb is interrupted once the program has printed again.
	gdbInterrupt = []string{
		"continue",
		"print 'main.frames' > 100",
		"bt",
		"detach",
	}
	gdbInterruptWant = []string{
		"debug: second 2",
		"Program received signal SIGINT",
		"$6 = 1",
	}
)

// inOrder checks that out holds each of want, one after another.
func inOrder(t *testing.T, out string, want []string) {
	t.Helper()
	rest := out
	for _, w := range want {
		i := strings.Index(rest, w)
		if i < 0 {
			t.Errorf("gdb did not print %q, after what came before it:\n%s", w, out)
			return
		}
		rest = rest[i+len(w):]
	}
}

func TestGDBEmulator(t *testing.T) {
	parallel(t)
	gdbCommand(t)
	prog := debugProgram(t, "steps.elf")
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(debugProgram(t, "steps.cue")))
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
	addr := serveGDB(t, e)
	out := runGDB(t, prog, addr, append(gdbScript, gdbInterrupt...), "debug: second 2")
	t.Log(out)
	// The library is the program's own here, with its lines, so the
	// backtrace from where the interrupt found it, waiting for the
	// vertical blank, reaches main.
	inOrder(t, out, append(append(gdbWant, gdbInterruptWant...), "libps_vsync_wait (", "in main.main ()", "runtime.run ()"))
}

// TestGDBMonitor debugs through the clean monitor, the program started
// with its go command, which a Control-B stops.
func TestGDBMonitor(t *testing.T) {
	parallel(t)
	gdbCommand(t)
	path := debugProgram(t, "steps-resident.elf")
	prog, err := serial.OpenProgram(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	_, port, _ := ourMonitor(t)
	h, err := gdbserver.StartHSS(t.Context(), port, prog, gdbserver.HSSOptions{Start: gdbserver.StartGo})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	addr := serveGDB(t, h)
	out := runGDB(t, path, addr, append(gdbScript, gdbInterrupt...), "debug: second 2")
	t.Log(out)
	inOrder(t, out, append(gdbWant, gdbInterruptWant...))
}

// TestStartAuto starts a program on each monitor as yaroze debug does by
// default: with go on ours, which can stop it, and with rdb on Sony's. Either
// way the program is stopped at its first instruction.
func TestStartAuto(t *testing.T) {
	path := debugProgram(t, "steps-resident.elf")
	eachMonitor(t, func(t *testing.T, boot boot) {
		prog, err := serial.OpenProgram(path)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		_, port, _ := boot(t)
		h, err := gdbserver.StartHSS(t.Context(), port, prog, gdbserver.HSSOptions{Start: gdbserver.StartAuto})
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		assert.Equal(t, strings.HasSuffix(t.Name(), "/ours"), h.Stoppable())
		pc, err := h.ReadRegister(t.Context(), gdbserver.RegPC)
		assert.NoError(t, err)
		assert.Equal(t, prog.PC, pc)
	})
}

// TestStartTakeOver starts a program from Sony's monitor as yaroze debug
// --port does by default: the clean monitor is uploaded in its place, with
// Sony's library kept, and the program started with go, which a Control-B
// then stops while it runs.
func TestStartTakeOver(t *testing.T) {
	path := debugProgram(t, "steps-resident.elf")
	prog, err := serial.OpenProgram(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	monitor, err := serial.OpenMonitor(serialProgram(t, "monitor"), 115200)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	takeOver, err := serial.TakeOverMonitor(monitor)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	_, port, _ := sonyMonitor(t)
	var log strings.Builder
	h, err := gdbserver.StartHSS(t.Context(), port, prog, gdbserver.HSSOptions{
		Start:    gdbserver.StartAuto,
		TakeOver: takeOver,
		Log:      &log,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, true, strings.Contains(log.String(), "uploading the clean monitor"), log.String())
	assert.Equal(t, true, h.Stoppable())
	pc, err := h.ReadRegister(t.Context(), gdbserver.RegPC)
	assert.NoError(t, err)
	assert.Equal(t, prog.PC, pc)

	// Let it run, and stop it.
	var out strings.Builder
	h.SetOutput(&out)
	assert.NoError(t, h.Continue(t.Context()))
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	_, err = h.Wait(ctx)
	cancel()
	assert.Equal(t, true, errors.Is(err, context.DeadlineExceeded), err)
	assert.NoError(t, h.Interrupt())
	stop, err := h.Wait(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, gdbserver.SIGINT, stop.Signal)
	assert.Equal(t, true, strings.Contains(out.String(), "debug: start"), out.String())
}

// TestGDBRemoteDebugger debugs through each monitor's rdb, which Sony's has
// too. A program it runs cannot be interrupted.
func TestGDBRemoteDebugger(t *testing.T) {
	gdbCommand(t)
	path := debugProgram(t, "steps-resident.elf")
	eachMonitor(t, func(t *testing.T, boot boot) {
		prog, err := serial.OpenProgram(path)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		_, port, _ := boot(t)
		h, err := gdbserver.StartHSS(t.Context(), port, prog, gdbserver.HSSOptions{Start: gdbserver.StartRDB})
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		addr := serveGDB(t, h)
		out := runGDB(t, path, addr, append(gdbScript, "detach"), "")
		t.Log(out)
		inOrder(t, out, gdbWant)
	})
}

// TestGDBECOFF debugs a program GCC 2.95.3 built as ECOFF through the ELF
// with DWARF that yaroze debug makes from it (internal/ecoffelf), as GDB
// from 16 on crashes on the ECOFF's own stabs. The program is the original,
// side-loaded; only GDB reads the ELF. Its scale.c was built without -g, so
// GDB unwinds from it on the frame information alone.
func TestGDBECOFF(t *testing.T) {
	parallel(t)
	gdbCommand(t)
	addr, path := serveECOFF(t, "../format/ecoff/testdata/debug/steps")
	out := runGDB(t, path, addr, []string{
		"break tick",
		"continue",
		"bt",
		"break scale",
		"continue",
		"bt",
		"finish",
		"next",
		"info symbol $pc",
		"kill",
	}, "")
	t.Log(out)
	inOrder(t, out, []string{
		// Where GCC put tick's first line, after its prologue.
		"Breakpoint 1, tick (n=", ") at steps.c:35",
		"#0  tick (n=", ") at steps.c:35",
		"in main () at steps.c:45",
		// scale has no lines, but has a frame to unwind through.
		"Breakpoint 2, ",
		"in scale ()",
		"in tick (n=", ") at steps.c:35",
		"in main () at steps.c:45",
		"tick (n=", ") at steps.c:36",
	})
	assert.Equal(t, false, strings.Contains(out, "can't find the start of the function"), out)
}

// TestGDBECOFFVariables reads the types and variables of a program GCC
// 2.95.3 built at -O0 with GDB, through the converted ELF: locals in nested
// blocks, parameters, a caller's locals, globals of every kind of type.
func TestGDBECOFFVariables(t *testing.T) {
	parallel(t)
	gdbCommand(t)
	addr, path := serveECOFF(t, "../format/ecoff/testdata/debug/vars")
	out := runGDB(t, path, addr, []string{
		// In walk's second time round its loop, in the innermost block.
		"break vars.c:63 if count == 1",
		"continue",
		"info locals",
		"print *n",
		"print limit",
		"up",
		"info locals",
		"print shape",
		"print answer",
		"ptype struct flags",
		"kill",
	}, "")
	t.Log(out)
	inOrder(t, out, []string{
		"Breakpoint 1, walk (n=", "limit=3) at vars.c:63",
		"tag = 98 'b'", "v = 4", "sum = 99", "count = 1",
		"{next = 0x", "value = 2}",
		"= 3",
		"in main () at vars.c:82",
		"i = 3", "local = {x = 7, y = 8}",
		"{pts = {{x = 1, y = 2}, {x = 3, y = 4}, {x = 5, y = 6}}, color = GREEN, name = ", "\"tri\"",
		"scale = 1.5, weight = 2.25, big = -5, ubig = 7, fl = {a = 5, b = 17, c = -3}",
		"= 42",
		"unsigned int a : 3;", "unsigned int b : 5;", "int c;",
	})
}

// serveECOFF converts the ECOFF program at prog, side-loads the original
// into an emulator run to its entry point, as yaroze debug --headless does,
// and serves it. It returns the address and the converted ELF's path.
func serveECOFF(t *testing.T, prog string) (string, string) {
	t.Helper()
	f, err := ecoff.Open(prog)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer f.Close()
	data, err := ecoffelf.Convert(f)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	path := filepath.Join(t.TempDir(), filepath.Base(prog)+".elf")
	assert.NoError(t, os.WriteFile(path, data, 0o644))
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
	return serveGDB(t, e), path
}
