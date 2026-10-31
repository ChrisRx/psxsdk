package siocons

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	xcontext "go.chrisrx.dev/x/context"
	"golang.org/x/term"

	"go.chrisrx.dev/psx/serial"
)

// banner is what SIOCONS printed as it started, in its version's color.
const banner = "\033[36;1m" +
	"siocons -- PlayStation debug system console program\n" +
	"   for DTLH3000 1996/05/10 00:00:03\n" +
	"   type \033[7m F1 \033[0;36;1m  ----> display help\n" +
	"   when hung up try type \033[7m ESC \033[0m\n"

type options struct {
	port string
	baud int
}

// Command returns the command, as use: siocons as a tool of its own, and
// sio in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:   use + " [flags] [auto file]",
		Short: "talk to a Net Yaroze over the serial cable, as SIOCONS did",
		Long: `The console Sony's Net Yaroze came with for its serial cable: what is typed
goes to the monitor on the console, and what it and the programs it runs
send comes back. The function keys run local commands, which F1 lists:
F4 loads a program, a PS-X EXE, an ECOFF executable or an ELF, F9 F4 loads
data at an address, F10 F4 saves memory to a file, F3 runs a batch file
of lines for the monitor and local commands, F5 logs, F2 runs a command on
the host, F8 edits lines at the monitor's prompt with a history, and F10
F2 or Escape quits. A batch file given runs first.

The serial device and speed come from DTLH3000, DEVICE[,BAUD], as SIOCONS
took its port from it, or -p in the same form, with -B for the speed, and
the first serial device at 9600 bps otherwise. The history is kept in
siocons.his, or the file SIOCONSHIS names.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			auto := ""
			if len(args) == 1 {
				auto = args[0]
			}
			return run(cmd, &opts, auto)
		},
	}
	cmd.Flags().StringVarP(&opts.port, "port", "p", os.Getenv("DTLH3000"), "DEVICE[,BAUD]: the serial device, and its speed")
	cmd.Flags().IntVarP(&opts.baud, "baud", "B", 0, "the serial device's speed, 9600 if neither this nor --port gives one")
	return cmd
}

func run(cmd *cobra.Command, opts *options, auto string) error {
	device, baud := opts.port, 9600
	if d, b, ok := strings.Cut(opts.port, ","); ok {
		device = d
		if n, err := strconv.Atoi(b); err == nil {
			baud = n
		}
	}
	baud = cmp.Or(opts.baud, baud)
	if device == "" {
		devices, err := serial.Devices()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			return errors.New("cannot find serial devices")
		}
		device = devices[0]
	}
	fmt.Print(banner)
	fmt.Printf(" BAUDRATE = %d\n", baud)

	port, err := serial.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	mon, err := serial.NewMonitor(cmd.Context(), port)
	if err != nil {
		return err
	}

	fd := int(os.Stdin.Fd())
	var cooked *term.State
	if term.IsTerminal(fd) {
		if cooked, err = term.MakeRaw(fd); err != nil {
			return err
		}
		defer term.Restore(fd, cooked)
	}
	keys := make(chan Key, 64)
	go readKeys(os.Stdin, keys)
	c := New(port, mon, os.Stdout, keys)
	c.Shell = func(line string) error { return shell(fd, cooked, line) }

	hist := cmp.Or(os.Getenv("SIOCONSHIS"), "siocons.his")
	if f, err := os.Open(hist); err == nil {
		c.LoadHistory(f)
		f.Close()
	}
	defer func() {
		if f, err := os.Create(hist); err == nil {
			c.SaveHistory(f)
			f.Close()
		}
	}()
	// Control-C is a key here, which goes to the monitor, so the first
	// signal to stop cancels the run instead of ending the process at once.
	xcontext.AddHandler(cmd.Context(), func() {})
	err = c.Run(cmd.Context(), auto)
	if errors.Is(err, errTerminate) {
		return nil
	}
	return err
}

// shell runs a command line on the host, with the terminal as it was
// before SIOCONS took it over, cooked, as SIOCONS's dos command did under
// DOS.
func shell(fd int, cooked *term.State, line string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", line)
	} else {
		cmd = exec.Command(cmp.Or(os.Getenv("SHELL"), "/bin/sh"), "-c", line)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if cooked != nil {
		if raw, err := term.GetState(fd); err == nil {
			defer term.Restore(fd, raw)
		}
		term.Restore(fd, cooked)
	}
	return cmd.Run()
}
