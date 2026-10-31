// Package sioload uploads an executable to a Net Yaroze over the serial
// cable, as SIOCONS's load did, a PS-X EXE, ECOFF or ELF, and with --exec
// runs it. It is the sioload tool.
package sioload

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	xcontext "go.chrisrx.dev/x/context"
	"go.chrisrx.dev/x/errors"

	"go.chrisrx.dev/psx/serial"
)

type options struct {
	BaudRate   int
	DeviceName string
	Exec       bool
	Stdout     bool
}

// Command returns the command, as use.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:          use + " [flags] <file>",
		Short:        "upload an executable to a Net Yaroze over the serial cable",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), &opts, args)
		},
	}
	cmd.Flags().IntVarP(&opts.BaudRate, "baud", "b", 115200, "baud rate")
	cmd.Flags().StringVarP(&opts.DeviceName, "device-name", "d", "", "serial device name (e.g. /dev/ttyUSB0)")
	cmd.Flags().BoolVar(&opts.Exec, "exec", false, "execute uploaded file")
	cmd.Flags().BoolVar(&opts.Stdout, "stdout", false, "output response to stdout")
	return cmd
}

func run(ctx context.Context, opts *options, args []string) error {
	prog, err := serial.OpenProgram(args[0])
	if err != nil {
		return err
	}
	if opts.DeviceName == "" {
		ports, err := serial.Devices()
		if err != nil {
			return err
		}
		if len(ports) == 0 {
			return errors.New("cannot find serial devices")
		}
		opts.DeviceName = ports[0]
	}
	// Under the shutdown context the tool runs with, the first Ctrl-C
	// cancels ctx, which stops the upload between bytes and closes the port,
	// and a second kills the process.
	xcontext.AddHandler(ctx, func() {})
	port, err := serial.Open(opts.DeviceName, opts.BaudRate)
	if err != nil {
		return err
	}
	defer port.Close()
	if opts.Stdout {
		port.Output = os.Stdout
	}

	m, err := serial.NewMonitor(ctx, port)
	if err != nil {
		return err
	}
	if err := m.ClearScreen(ctx); err != nil {
		return err
	}
	if err := m.Load(ctx, prog); err != nil {
		return err
	}
	if opts.Exec {
		return m.Run(ctx)
	}
	return nil
}
