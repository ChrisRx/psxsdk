package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/ChrisRx/psxsdk/pkg/format/ecoff"
	"github.com/ChrisRx/psxsdk/pkg/yaroze"
	"github.com/spf13/cobra"
	"go.bug.st/serial"
)

type sioLoadOpts struct {
	BaudRate   int
	DeviceName string
	Exec       bool
	Stdout     bool
}

func NewSIOLoadCommand() *cobra.Command {
	o := &sioLoadOpts{}
	cmd := &cobra.Command{
		Use:  "sioload [flags] <file>",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			f, err := ecoff.Open(args[0])
			if err != nil {
				log.Fatal(err)
			}
			w := ioutil.Discard
			if o.Stdout {
				w = os.Stdout
			}
			if o.DeviceName == "" {
				ports, err := serial.GetPortsList()
				if err != nil {
					log.Fatal(err)
				}
				if len(ports) == 0 {
					log.Fatal("cannot find serial devices")
				}
				o.DeviceName = ports[0]
			}
			c, err := yaroze.OpenPort(&yaroze.PortConfig{
				BaudRate:   o.BaudRate,
				DeviceName: o.DeviceName,
				Output:     w,
			})
			if err != nil {
				log.Fatal(err)
			}
			defer c.Close()

			if err := c.ClearScreen(); err != nil {
				log.Fatal(err)
			}

			if err := c.Load(f); err != nil {
				log.Fatal(err)
			}

			if o.Exec {
				if err := c.Go(); err != nil {
					log.Fatal(err)
				}
			}
		},
	}
	cmd.Flags().IntVarP(&o.BaudRate, "baud", "b", 115200, "baud rate")
	cmd.Flags().StringVarP(&o.DeviceName, "device-name", "d", "", "serial device name (e.g. /dev/ttyUSB0)")
	cmd.Flags().BoolVar(&o.Exec, "exec", false, "execute uploaded file")
	cmd.Flags().BoolVar(&o.Stdout, "stdout", false, "output response to stdout")
	return cmd
}

func main() {
	if err := NewSIOLoadCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
