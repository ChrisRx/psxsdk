package yaroze

import (
	"bytes"
	"cmp"
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/internal/ecoffelf"
	"go.chrisrx.dev/psx/internal/gdbserver"
	"go.chrisrx.dev/psx/internal/project"
	"go.chrisrx.dev/psx/internal/psxplay"
	"go.chrisrx.dev/psx/internal/toolchain"
	"go.chrisrx.dev/psx/internal/ttyout"
	"go.chrisrx.dev/psx/link"
	"go.chrisrx.dev/psx/serial"
)

// bootBudget bounds the instructions the BIOS may take to reach its shell,
// where an executable is loaded.
const bootBudget = 50_000_000

// debugFlags are the debug command's own flags.
type debugFlags struct {
	discFlags
	listen    string
	port      string
	baud      int
	start     string
	play      psxplay.Options
	headless  bool
	fast      bool
	packets   bool
	serveOnly bool   // only serve the remote protocol, as for GDB
	dap       string // serve Delve's debug adapter protocol here instead of its prompt
}

// A debugged program: what to run, and the ELF that holds its symbols.
type debugged struct {
	disc  string    // a cue sheet for the drive
	exe   *psx.File // an executable to load when the BIOS reaches its shell
	elf   string    // the ELF GDB reads, if there is one
	file  string    // the program's file, for uploading it
	entry uint32    // where the program is first stopped, if known
	title string
}

func debugCommand() *cobra.Command {
	var f debugFlags
	cmd := &cobra.Command{
		Use:   "debug [flags] [package | program | disc.cue]",
		Short: "run a program stopped and debug it, with Delve here or GDB",
		Long: `debug runs a program stopped at its first instruction and debugs it. A
Go program is debugged with Delve, built in: its prompt opens here, or with
--dap its debug adapter protocol is served for an editor, which needs no
more in its launch configuration than to connect. Any other program, in C
or C++ or an ECOFF, is debugged with GDB, whose prompt opens here: GDB for
WebAssembly (gdb-wasi), downloaded the first time and kept in the cache
directory, or the gdb.wasm YAROZE_GDB names. Control-C stops the running
program, and two within half a second end debug.

Either way the program is served with GDB's remote protocol too, which GDB
speaks with target remote and other debuggers do too. With --serve that is
all debug does, and it says what to type in GDB.

Given a package, the current directory's if nothing is given, it builds it
as build does with --debug, which keeps the line tables and the variables'
places, and writes the ELF that holds them, which GDB reads, to the
package's name with .elf, or the file --elf names. Given a program built
already, an ELF from build --debug or a PSX-EXE, it debugs that, and given
a disc's cue sheet, that disc, with --elf naming the ELF of the program on
it, for where it starts: without one the machine is stopped at the BIOS's
first instruction.

The program runs in the emulator, in a window as run has it, on a disc
with its files as disc builds it, or with --headless with no window. The
debugger works the emulated machine itself: a step is one instruction, and
a breakpoint writes nothing into the program.

With --port the program is on a console, at the other end of the serial
cable, with the monitor at its prompt: the Net Yaroze boot disc's, or the
clean one. The package is built for the library the console has in memory
(--libps resident), uploaded as sio does it, and debugged through the
monitor's remote debugger, the one the Net Yaroze's own gdb used. --start
says how it is started:

  auto    go on the clean monitor and rdb on Sony's, which yaroze asks the
          monitor about. This is the default.
  go      with the monitor's go command and a breakpoint at the first
          instruction. The clean monitor stops such a program for the
          debugger, there and whenever Control-C asks.
  rdb     with the monitor's rdb command, which Sony's monitor has too.
          The program cannot be stopped once it runs, short of a
          breakpoint or a fault.
  attach  nothing is uploaded: the program the clean monitor's go started
          earlier is stopped where it is.

  yaroze debug ./mygame
  yaroze debug --headless mygame.elf
  yaroze debug --port /dev/ttyUSB0,115200 ./mygame`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer f.close()
			arg := "."
			if len(args) == 1 {
				arg = args[0]
			}
			if f.port != "" {
				return f.debugSerial(cmd, arg, len(args) == 1)
			}
			return f.debugEmulator(cmd, arg)
		},
	}
	f.discFlags.register(cmd)
	fl := cmd.Flags()
	fl.Lookup("debug").Hidden = true // always on here
	fl.Lookup("elf").Usage = "the ELF to write when building a package, the package's name with .elf if not given, or the ELF of the program on a disc given"
	fl.StringVar(&f.listen, "listen", "localhost:1234", "the address to wait for the debugger on")
	fl.StringVarP(&f.port, "port", "p", "", "DEVICE[,BAUD]: debug on the console on this serial device, through its monitor, rather than in the emulator")
	fl.IntVarP(&f.baud, "baud", "B", 0, "the serial device's speed, 9600 if neither this nor --port gives one")
	fl.StringVar(&f.start, "start", gdbserver.StartAuto, "with --port, how the program is started: auto, go, rdb or attach")
	// The window's flags, as run has them, less those for what debug does
	// its own way.
	f.play.AddFlags(fl)
	for _, name := range []string{"exe", "fast-start", "break"} {
		fl.Lookup(name).Hidden = true
	}
	fl.BoolVar(&f.headless, "headless", false, "run the emulator with no window")
	fl.BoolVar(&f.fast, "fast", false, "with --headless, run as fast as the host does rather than at a console's pace")
	fl.BoolVar(&f.packets, "packets", false, "print each packet to and from the debugger")
	fl.BoolVar(&f.serveOnly, "serve", false, "only wait for a debugger such as GDB, without debugging a Go program with Delve here")
	fl.StringVar(&f.dap, "dap", "", "for a Go program, serve Delve's debug adapter protocol on this address, for an editor, in place of its prompt")
	return cmd
}

// prepare makes arg into what to debug: a package built, into dir for what
// is not kept, or a program or a disc as it is. resident says the program
// is for a console's monitor, built over the library in its memory, with
// no disc.
func (f *debugFlags) prepare(ctx context.Context, arg, dir string, resident bool) (*debugged, error) {
	d := &debugged{title: "yaroze debug - " + filepath.Base(arg)}
	ext := filepath.Ext(arg)
	if fi, err := os.Stat(arg); err != nil {
		return nil, err
	} else if fi.IsDir() {
		if !isBuildable(arg) {
			return nil, fmt.Errorf("%s has no Go package or C project to build", arg)
		}
		name, err := pkgName(arg)
		if err != nil {
			return nil, err
		}
		f.debug = true
		d.elf = cmp.Or(f.elf, name+".elf")
		d.file = d.elf
		d.title = "yaroze debug - " + name
		if resident {
			f.libps = "resident"
			if err := f.build(ctx, arg, "", d.elf); err != nil {
				return nil, err
			}
		} else {
			f.elf = d.elf
			if err := f.disc(ctx, arg, filepath.Join(dir, name+".bin"), io.Discard); err != nil {
				return nil, err
			}
			d.disc = filepath.Join(dir, name+".cue")
		}
	} else if strings.EqualFold(ext, ".cue") || strings.EqualFold(ext, ".zip") {
		if ok, err := project.ZipHasCue(arg); strings.EqualFold(ext, ".zip") && (err != nil || !ok) {
			return nil, cmp.Or(err, fmt.Errorf("%s holds no cue sheet", arg))
		}
		d.disc, d.elf = arg, f.elf
	} else {
		data, err := os.ReadFile(arg)
		if err != nil {
			return nil, err
		}
		d.file = arg
		// An ECOFF, as GCC 2.95.3 builds, has its symbols in a form current
		// debuggers cannot read: they read an ELF with DWARF made from it
		// instead, written beside it, and the program runs as it is.
		if f.elf == "" {
			if conv, err := debugELFOf(arg, data); err != nil {
				return nil, err
			} else if conv != "" {
				f.elf = conv
			}
		}
		switch {
		case resident && link.IsExecutable(data) && !link.IsPSXEXE(data):
			// Uploaded as it is, over the console's library.
			d.elf = cmp.Or(f.elf, arg)
		case link.IsPSXEXE(data):
			if d.exe, err = psx.Open(arg); err != nil {
				return nil, err
			}
			d.elf, d.entry = f.elf, d.exe.PC0
		case link.IsExecutable(data):
			if d.exe, err = link.Runnable(arg, f.loads...); err != nil {
				return nil, err
			}
			img, err := link.LoadImage(arg)
			if err != nil {
				return nil, err
			}
			d.elf, d.entry = cmp.Or(f.elf, arg), img.Entry
		default:
			return nil, fmt.Errorf("%s is not a package, a disc, a PSX-EXE, an ECOFF or an ELF", arg)
		}
	}
	// Where a program on a disc, or one a console starts, begins is in its
	// ELF, if what holds its symbols is one.
	if d.elf != "" && d.entry == 0 {
		if e, err := elf.Open(d.elf); err == nil {
			d.entry = uint32(e.Entry)
			e.Close()
		} else if d.disc != "" {
			return nil, err
		}
	}
	return d, nil
}

// serve waits for debuggers on the listen address and serves them t. A Go
// program is debugged with Delve here, and any other with GDB here, unless
// --serve says to only wait, having said what to type in GDB. options are
// more of GDB's arguments, before its target remote.
func (f *debugFlags) serve(ctx context.Context, t gdbserver.Target, d *debugged, where string, options ...string) error {
	l, err := net.Listen("tcp", f.listen)
	if err != nil {
		return err
	}
	defer l.Close()
	s := gdbserver.New(t)
	s.Stdout = os.Stdout
	if f.packets {
		s.Logf = func(format string, args ...any) { fmt.Fprintf(os.Stderr, "gdb: "+format+"\n", args...) }
	}
	pc, _ := t.ReadRegister(ctx, gdbserver.RegPC)
	if !f.serveOnly && d.elf != "" && isGoProgram(d.elf) {
		return f.serveDelve(ctx, s, l, d, pc, where)
	}
	if !f.serveOnly && f.dap == "" {
		err := f.serveGDB(ctx, s, l, d, pc, where, options)
		if !errors.Is(err, toolchain.ErrNoGDB) {
			return err
		}
		fmt.Fprintf(os.Stderr, "yaroze: %v\n", err)
	}
	var shown strings.Builder
	for i := 0; i+1 < len(options); i += 2 {
		fmt.Fprintf(&shown, " %s '%s'", options[i], options[i+1])
	}
	fmt.Fprintf(os.Stderr, `yaroze: the program is stopped at %08x, %s
yaroze: waiting for a debugger on %s; in another terminal:

    gdb %s%s -ex 'target remote %s'

`, pc, where, l.Addr(), cmp.Or(d.elf, "PROGRAM.elf"), shown.String(), l.Addr())
	if d.elf == "" {
		fmt.Fprintln(os.Stderr, "yaroze: with no ELF given (--elf), GDB has no symbols or source lines")
	}
	err = s.Serve(ctx, l)
	if errors.Is(err, gdbserver.ErrKilled) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// debugEmulator debugs arg in the emulator.
func (f *debugFlags) debugEmulator(cmd *cobra.Command, arg string) error {
	ctx := cmd.Context()
	tmp, err := os.MkdirTemp("", "yaroze-debug-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	d, err := f.prepare(ctx, arg, tmp, false)
	if err != nil {
		return err
	}

	// attach makes the machine the debugger's, run to the program's first
	// instruction.
	attach := func(m *emu.Machine) (*gdbserver.Emulator, error) {
		e := gdbserver.NewEmulator(m)
		e.Fast = f.fast
		if d.entry != 0 && m.CPU.PC != d.entry {
			if err := e.RunTo(d.entry); err != nil {
				return nil, err
			}
		}
		return e, nil
	}
	if f.headless {
		bios := openbios.Image
		if f.play.BIOS != "" {
			if bios, err = os.ReadFile(f.play.BIOS); err != nil {
				return err
			}
		}
		m, err := emu.New(bios)
		if err != nil {
			return err
		}
		// What the BIOS prints as it boots, with --bios-log; the server
		// takes the output over once the program is stopped at its start.
		tty := ttyout.New(os.Stdout, f.play.BIOSLog)
		m.TTY = tty
		if d.disc != "" {
			disc, err := emu.OpenDisc(d.disc)
			if err != nil {
				return err
			}
			defer disc.Close()
			m.InsertDisc(disc)
		}
		if f.play.Card != "" {
			card, err := emu.OpenMemoryCard(f.play.Card)
			if err != nil {
				return err
			}
			m.InsertCard(1, card)
			defer func() { os.WriteFile(f.play.Card, card.Data(), 0o644) }()
		}
		if d.exe != nil {
			if err := m.SideloadEXE(d.exe, bootBudget); err != nil {
				return err
			}
			tty.Start()
		}
		e, err := attach(m)
		if err != nil {
			return err
		}
		return f.serve(ctx, e, d, "in the emulator")
	}

	play := f.play
	play.Disc, play.Title, play.Breaks = d.disc, d.title, nil
	if d.exe != nil {
		play.EXE = filepath.Join(tmp, "PROGRAM.EXE")
		if err := d.exe.WriteFile(play.EXE); err != nil {
			return err
		}
	}
	// The window has the main goroutine, and runs the machine's frames; the
	// server has its own, and closes the window when it is done.
	served := make(chan error, 1)
	play.Debug = func(m *emu.Machine) (func() error, error) {
		e, err := attach(m)
		if err != nil {
			return nil, err
		}
		e.Driven = true
		go func() { served <- f.serve(ctx, e, d, "in the emulator") }()
		return func() error {
			select {
			case err := <-served:
				served <- err
				return psxplay.ErrClose
			default:
			}
			e.RunFrame()
			return nil
		}, nil
	}
	if err := psxplay.Run(play); err != nil {
		return err
	}
	select {
	case err := <-served:
		return err
	default:
		return nil
	}
}

// debugSerial debugs on the console on the serial port: arg, uploaded,
// if given is set or the program is to be started, or the program there
// already.
func (f *debugFlags) debugSerial(cmd *cobra.Command, arg string, given bool) error {
	ctx := cmd.Context()
	device, baud := f.port, 9600
	if dev, b, ok := strings.Cut(f.port, ","); ok {
		device = dev
		if n, err := strconv.Atoi(b); err == nil {
			baud = n
		}
	}
	baud = cmp.Or(f.baud, baud)

	d := &debugged{elf: f.elf}
	var prog *serial.Program
	if f.start != gdbserver.StartAttach && (given || f.start != gdbserver.StartRDB) {
		tmp, err := os.MkdirTemp("", "yaroze-debug-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if d, err = f.prepare(ctx, arg, tmp, true); err != nil {
			return err
		}
		if d.disc != "" {
			return errors.New("a disc cannot be uploaded to a console: give the program")
		}
		if prog, err = serial.OpenProgram(d.file); err != nil {
			return err
		}
	}

	port, err := serial.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	if prog != nil {
		fmt.Fprintf(os.Stderr, "yaroze: uploading to %s at %d bps\n", device, baud)
	}
	t, err := gdbserver.StartHSS(ctx, port, prog, f.start)
	if err != nil {
		return err
	}
	if f.start == gdbserver.StartAuto && !t.Stoppable() {
		fmt.Fprintln(os.Stderr, "yaroze: the monitor is Sony's, so the program runs until a breakpoint or a fault: Control-C cannot stop it")
	}
	// A register is a packet each way on the cable, and GDB reads them all
	// at each stop, which at 9600 bps takes longer than the two seconds it
	// waits for an answer by default.
	return f.serve(ctx, t, d, "on the console at "+device, "-ex", "set remotetimeout 30")
}

// debugELFOf writes the ELF with DWARF for the ECOFF program at path, whose
// contents are data, and returns its name: the program's with .elf added.
// It returns "" for a program that is no ECOFF.
func debugELFOf(path string, data []byte) (string, error) {
	f, err := ecoff.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", nil
	}
	conv, err := ecoffelf.Convert(f)
	if err != nil {
		return "", fmt.Errorf("%s: making an ELF a debugger reads: %w", path, err)
	}
	out := path + ".elf"
	if err := os.WriteFile(out, conv, 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// serveDelve debugs a Go program with Delve, in this process: its prompt
// in this terminal, or with --dap its debug adapter protocol for an editor.
// Delve reaches the program through the server on l, which another
// debugger can connect to as well, and ends it when it quits.
// serveGDB serves the debugger s on l while GDB runs here on d, until GDB
// ends.
func (f *debugFlags) serveGDB(ctx context.Context, s *gdbserver.Server, l net.Listener, d *debugged, pc uint32, where string, options []string) error {
	// Before serving, so that with no GDB to be had the program is served
	// for another.
	if _, _, err := toolchain.GDB(ctx, os.Stderr); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx, l) }()
	addr := l.Addr().String()
	fmt.Fprintf(os.Stderr, "yaroze: the program is stopped at %08x, %s, and GDB debugs it; another GDB can connect on %s\n", pc, where, addr)
	if d.elf == "" {
		fmt.Fprintln(os.Stderr, "yaroze: with no ELF given (--elf), GDB has no symbols or source lines")
	}
	err := runGDB(ctx, s, addr, d.elf, options, os.Stdin, os.Stdout, os.Stderr)
	cancel()
	if serr := <-served; err == nil && serr != nil && !errors.Is(serr, gdbserver.ErrKilled) && !errors.Is(serr, context.Canceled) {
		err = serr
	}
	return err
}

func (f *debugFlags) serveDelve(ctx context.Context, s *gdbserver.Server, l net.Listener, d *debugged, pc uint32, where string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx, l) }()
	addr := l.Addr().String()
	fmt.Fprintf(os.Stderr, "yaroze: the program is stopped at %08x, %s, and Delve debugs it; GDB can connect on %s too\n", pc, where, addr)
	var err error
	if f.dap != "" {
		var dl net.Listener
		if dl, err = net.Listen("tcp", f.dap); err == nil {
			err = serveDAP(ctx, dl, addr, d.elf)
		}
	} else {
		err = runDelve(ctx, addr, d.elf)
	}
	cancel()
	if serr := <-served; err == nil && serr != nil && !errors.Is(serr, gdbserver.ErrKilled) && !errors.Is(serr, context.Canceled) {
		err = serr
	}
	return err
}
