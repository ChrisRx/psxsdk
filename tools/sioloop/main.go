// Command sioloop checks a USB serial adapter with no console attached, for
// the serial cable to a Net Yaroze. Join pins 2 and 3 of its DB9 and pins 7
// and 8, keeping the two apart, then plug it in and run it:
//
//	go run ./tools/sioloop -d /dev/ttyUSB0
//
// It reports whether the adapter returns every byte, whether its CTS follows
// its RTS, and how long each of those takes, which limits what an upload can
// do. It exits with 1 if the adapter fails a check.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.chrisrx.dev/psx/serial"
)

func main() {
	device := flag.String("d", "", "serial device, such as /dev/ttyUSB0 (default: the first found)")
	baud := flag.Int("b", 115200, "baud rate")
	flag.Parse()
	if err := run(*device, *baud); err != nil {
		fmt.Fprintln(os.Stderr, "sioloop:", err)
		os.Exit(1)
	}
}

func run(device string, baud int) error {
	if device == "" {
		devices, err := serial.Devices()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			return fmt.Errorf("no serial devices found")
		}
		device = devices[0]
	}
	fmt.Printf("%s at %d baud\n", device, baud)

	port, err := serial.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	port.Timeout = 2 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	l, err := serial.RunLoopback(ctx, port)

	fmt.Printf("bytes back:      %d of %d, %d wrong\n", l.Received, l.Sent, l.Wrong)
	fmt.Printf("CTS follows RTS: %v\n", l.Follows)
	if err != nil {
		return err
	}
	fmt.Printf("CTS lag:         %v median, %v longest\n", l.CTSLag, l.CTSMax)
	fmt.Printf("CTS read:        %v\n", l.CTSRead)
	fmt.Printf("RTS pulse:       %v\n", l.Pulse)
	fmt.Printf("round trip:      %v\n", l.RoundTrip)
	fmt.Printf("paced 2 KB:      %v, came back right: %v\n", l.BlockTime, l.BlockOK)

	// With the pause after each block's reply, as an upload has it.
	block := l.BlockTime + serial.DevicePacing.BlockStart + serial.DevicePacing.Reply
	fmt.Printf("upload estimate: %.1f KB/s, 346 KB in %.0f s\n", 2.0/block.Seconds(), 346/(2.0/block.Seconds()))
	if !l.OK() {
		return fmt.Errorf("the adapter failed a check")
	}
	fmt.Println("all checks passed")
	return nil
}
