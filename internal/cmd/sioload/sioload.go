// Package sioload uploads an ECOFF executable to a Net Yaroze over the
// serial cable, as siocons did, and with --exec runs it. It is the sioload
// tool and yaroze's sio command.
package sioload

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.bug.st/serial"
	"go.chrisrx.dev/psx/format/ecoff"
)

type options struct {
	BaudRate   int
	DeviceName string
	Exec       bool
	Stdout     bool
}

// Command returns the command, as use: sioload as a tool of its own, and
// sio in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:          use + " [flags] <file>",
		Short:        "upload an executable to a Net Yaroze over the serial cable",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&opts, args)
		},
	}
	cmd.Flags().IntVarP(&opts.BaudRate, "baud", "b", 115200, "baud rate")
	cmd.Flags().StringVarP(&opts.DeviceName, "device-name", "d", "", "serial device name (e.g. /dev/ttyUSB0)")
	cmd.Flags().BoolVar(&opts.Exec, "exec", false, "execute uploaded file")
	cmd.Flags().BoolVar(&opts.Stdout, "stdout", false, "output response to stdout")
	return cmd
}

func run(opts *options, args []string) error {
	f, err := ecoff.Open(args[0])
	if err != nil {
		return err
	}
	w := io.Discard
	if opts.Stdout {
		w = os.Stdout
	}
	if opts.DeviceName == "" {
		ports, err := serial.GetPortsList()
		if err != nil {
			return err
		}
		if len(ports) == 0 {
			return errors.New("cannot find serial devices")
		}
		opts.DeviceName = ports[0]
	}
	c, err := OpenPort(&PortConfig{
		BaudRate:   opts.BaudRate,
		DeviceName: opts.DeviceName,
		Output:     w,
	})
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.ClearScreen(); err != nil {
		return err
	}
	if err := c.Load(f); err != nil {
		return err
	}
	if opts.Exec {
		return c.Go()
	}
	return nil
}
