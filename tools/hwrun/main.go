// Command hwrun runs programs on a real Net Yaroze for the hardware session,
// which spec 35 describes, keeping what each prints in a file. A program with a
// file is skipped, so a session can stop and go on later.
//
//	go run ./tools/hwrun -d /dev/ttyUSB0 -library sony  -o results .build/serial/hwprobe.elf
//	go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -o results .build/serial/hwprobe.elf
//
// With -library clean it uploads the clean image with each program.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/internal/cmd/hwrun"
	"go.chrisrx.dev/psx/serial"
)

func main() {
	device := flag.String("d", "", "serial device, such as /dev/ttyUSB0")
	baud := flag.Int("b", 115200, "baud rate")
	library := flag.String("library", "sony", "the library to run on: sony, as the boot disc left it, or clean")
	imagePath := flag.String("image", "link/libps.exe.gz", "the clean image")
	out := flag.String("o", "results", "directory for the output")
	watch := flag.Duration("watch", 60*time.Second, "how long a program runs if it prints no end mark")
	flag.Parse()
	if err := run(*device, *baud, *library, *imagePath, *out, *watch, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "hwrun:", err)
		os.Exit(1)
	}
}

func run(device string, baud int, library, imagePath, out string, watch time.Duration, programs []string) error {
	if device == "" || len(programs) == 0 {
		return fmt.Errorf("give -d and at least one program")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	s := &hwrun.Session{
		Library: library,
		Out:     out,
		Watch:   watch,
		Log:     os.Stdout,
		Reset: func(why string) error {
			fmt.Printf("\n%s. Press reset on the console, then press Enter here.\n", why)
			_, err := bufio.NewReader(os.Stdin).ReadString('\n')
			return err
		},
	}
	switch library {
	case "sony":
	case "clean":
		image, err := openImage(imagePath)
		if err != nil {
			return err
		}
		s.Image = image
	default:
		return fmt.Errorf("unknown library %q", library)
	}

	port, err := serial.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	port.Timeout = 30 * time.Second
	s.Port = port

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("reaching the monitor: %w", err)
	}
	return s.Run(ctx, programs)
}

// openImage reads a resident image, gzipped as it is in link.
func openImage(path string) (*serial.Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(z)
	if err != nil {
		return nil, err
	}
	exe, err := psx.ParseFile(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return serial.EXEProgram(exe), nil
}
