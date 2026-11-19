// Command hwrun runs programs on a real Net Yaroze for the hardware session,
// which spec 35 describes, keeping what each prints in a file. A program with a
// file is skipped, so a session can stop and go on later.
//
//	go run ./tools/hwrun -d /dev/ttyUSB0 -library sony  -o results .build/serial/hwprobe.elf
//	go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -o results .build/serial/hwprobe.elf
//
// With -archive, the arguments are names from a programs.tsv, which brings each
// program's data files as its AUTO batch does:
//
//	go run ./tools/hwrun -d /dev/ttyUSB0 -archive .build/carchive/programs.tsv demos-scee-tunelfin
//
// With -library clean it first replaces the boot disc's monitor and library
// with the clean ones, once per boot. Programs then return to that monitor.
// That monitor can also stop a running program and let it go on, which -peek
// does at the times it gives, saving the registers and the -read regions:
//
//	go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -peek 20s,30s -read 80010000:256 prog.elf
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/internal/cmd/hwrun"
	"go.chrisrx.dev/psx/serial"
)

func main() {
	device := flag.String("d", "", "serial device, such as /dev/ttyUSB0")
	baud := flag.Int("b", 115200, "baud rate")
	library := flag.String("library", "sony", "the library to run on: sony, as the boot disc left it; clean; or sony-ourmon, Sony's as the boot disc left it under the clean monitor")
	imagePath := flag.String("image", "link/libps.exe.gz", "the clean image")
	monitorPath := flag.String("monitor", ".build/serial/monitor.elf", "the clean monitor, from task build:serial")
	out := flag.String("o", "results", "directory for the output")
	archive := flag.String("archive", "", "a programs.tsv whose names the arguments are")
	watch := flag.Duration("watch", 60*time.Second, "how long a program runs if it prints no end mark")
	peeks := flag.String("peek", "", "times into each run to stop the program and save its registers and -read regions, as 20s,30s (clean library only)")
	reads := flag.String("read", "", "memory to save at each peek, as ADDR:SIZE,..., the address in hexadecimal")
	flag.Parse()
	if err := run(*device, *baud, *library, *imagePath, *monitorPath, *archive, *out, *watch, *peeks, *reads, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "hwrun:", err)
		os.Exit(1)
	}
}

func run(device string, baud int, library, imagePath, monitorPath, archive, out string, watch time.Duration, peeks, reads string, programs []string) error {
	if device == "" || len(programs) == 0 {
		return fmt.Errorf("give -d and at least one program")
	}
	var peekAt []time.Duration
	for p := range strings.SplitSeq(peeks, ",") {
		if p == "" {
			continue
		}
		d, err := time.ParseDuration(p)
		if err != nil {
			return fmt.Errorf("-peek: %w", err)
		}
		peekAt = append(peekAt, d)
	}
	if len(peekAt) > 0 && library == "sony" {
		return fmt.Errorf("-peek needs the clean monitor (-library clean or sony-ourmon): only it stops a program")
	}
	var regions []hwrun.Region
	for r := range strings.SplitSeq(reads, ",") {
		if r == "" {
			continue
		}
		region, err := hwrun.ParseRegion(r)
		if err != nil {
			return fmt.Errorf("-read: %w", err)
		}
		regions = append(regions, region)
	}
	jobs := make([]hwrun.Job, 0, len(programs))
	if archive != "" {
		var err error
		if jobs, err = hwrun.ArchiveJobs(archive, programs); err != nil {
			return err
		}
	} else {
		for _, path := range programs {
			jobs = append(jobs, hwrun.NewJob(path))
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	s := &hwrun.Session{
		Library: library,
		Out:     out,
		Watch:   watch,
		Peeks:   peekAt,
		Reads:   regions,
		Log:     os.Stdout,
		Reset: func(why string) error {
			fmt.Printf("\n%s. Press reset on the console, then press Enter here.\n", why)
			_, err := bufio.NewReader(os.Stdin).ReadString('\n')
			return err
		},
	}
	// Both libraries need the clean monitor's first word: a Sony run checks
	// that the clean monitor isn't up.
	monitor, err := serial.OpenMonitor(monitorPath, uint32(baud))
	if err != nil {
		return err
	}
	s.Marker = binary.LittleEndian.Uint32(monitor.Segments[0].Data)
	switch library {
	case "sony":
	case "clean":
		image, err := openImage(imagePath)
		if err != nil {
			return err
		}
		s.TakeOver, err = serial.TakeOver(monitor, image.Segments[0])
		if err != nil {
			return err
		}
		if dir, err := os.UserCacheDir(); err == nil {
			s.Stamp = filepath.Join(dir, "psx", "hwrun-in-place")
		}
	case "sony-ourmon":
		// Sony's library as the boot disc started it, under the clean
		// monitor, for comparing the two libraries under one monitor.
		s.TakeOver, err = serial.TakeOverMonitor(monitor)
		if err != nil {
			return err
		}
		if dir, err := os.UserCacheDir(); err == nil {
			s.Stamp = filepath.Join(dir, "psx", "hwrun-in-place")
		}
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
	return s.Run(ctx, jobs)
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
