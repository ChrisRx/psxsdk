// Package hwrun runs programs on a real Net Yaroze for the hardware session
// (spec 35), and keeps what each prints. It is the hwrun tool.
//
// Each program's output goes in a file of its own, and one that has a file
// is skipped, so a session can stop and go on later. A console stuck in a
// program can't be told to stop, so hwrun asks for the reset button when the
// monitor doesn't come back. For the clean library it first replaces the boot
// disc's monitor and library with ours, once per boot, and programs return to
// that.
package hwrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.chrisrx.dev/psx/serial"
)

const (
	// endMark is the line a program prints when it has said all it will.
	endMark = "END"

	// greeting is what the monitor prints as it starts.
	greeting = "Communication baud rate"

	// monitorStart is where a monitor starts, the boot disc's or ours.
	monitorStart = 0x80068000

	// bootTimeout is how long a console gets to boot its disc after a reset.
	bootTimeout = 3 * time.Minute
)

// Session is a run of programs on one console.
type Session struct {
	Port *serial.Port

	// TakeOver is what replaces the boot disc's monitor and library, from
	// serial.TakeOver, or nil to stay on them. Marker is the first word of the
	// monitor it brings, which tells if it's already there.
	TakeOver *serial.Program
	Marker   uint32

	// Library names the library in the output files' names.
	Library string

	// Out is the directory for the output files.
	Out string

	// Watch is how long a program runs before it counts as done with no end
	// mark, which is how original programs finish here.
	Watch time.Duration

	// Reset asks the person at the console to press reset, and returns when
	// they have.
	Reset func(why string) error

	// Log is for progress.
	Log io.Writer

	mon *serial.Monitor
}

// Start connects to the monitor, and takes over from it if the session should
// and that hasn't happened on this boot.
func (s *Session) Start(ctx context.Context) error {
	mon, err := serial.NewMonitor(ctx, s.Port)
	if err != nil {
		return err
	}
	s.mon = mon
	if _, err := mon.Command(ctx, ""); err != nil {
		return err
	}
	if s.TakeOver == nil {
		return nil
	}
	reply, err := mon.Command(ctx, fmt.Sprintf("dw %08x 4", monitorStart))
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(reply), fmt.Sprintf(":%08x", s.Marker)) {
		fmt.Fprintln(s.Log, "the clean library is already in place")
		return nil
	}
	start := time.Now()
	fmt.Fprintln(s.Log, "uploading the clean library and monitor")
	if err := mon.Load(ctx, s.TakeOver); err != nil {
		return fmt.Errorf("uploading the clean library: %w", err)
	}
	if err := mon.Run(ctx); err != nil {
		return err
	}
	if !s.backOnItsOwn(ctx) {
		return errors.New("the clean monitor didn't start")
	}
	fmt.Fprintf(s.Log, "the clean library is in place, after %v\n", time.Since(start).Round(time.Second))
	s.mon, err = serial.NewMonitor(ctx, s.Port)
	return err
}

// Run runs each program.
func (s *Session) Run(ctx context.Context, programs []string) error {
	for _, path := range programs {
		if err := s.one(ctx, path); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// outPath is where a program's output goes.
func (s *Session) outPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return filepath.Join(s.Out, name+"-"+s.Library+".txt")
}

func (s *Session) one(ctx context.Context, path string) error {
	out := s.outPath(path)
	if _, err := os.Stat(out); err == nil {
		fmt.Fprintf(s.Log, "%s: already done, in %s\n", path, out)
		return nil
	}
	prog, err := serial.OpenProgram(path)
	if err != nil {
		return err
	}

	start := time.Now()
	fmt.Fprintf(s.Log, "%s: uploading\n", path)
	if err := s.mon.Load(ctx, prog); err != nil {
		return s.afterFailure(ctx, fmt.Errorf("uploading: %w", err))
	}
	fmt.Fprintf(s.Log, "%s: uploaded in %v, running\n", path, time.Since(start).Round(time.Second))

	var text strings.Builder
	s.Port.Output = io.MultiWriter(&text, s.Log)
	defer func() { s.Port.Output = io.Discard }()
	if err := s.mon.Run(ctx); err != nil {
		return s.afterFailure(ctx, fmt.Errorf("starting: %w", err))
	}

	timeout := s.Port.Timeout
	s.Port.Timeout = s.Watch
	_, err = s.Port.ReadUntil(ctx, endMark)
	s.Port.Timeout = timeout
	ended := err == nil
	if err != nil && !errors.Is(err, serial.ErrTimeout) {
		return err
	}
	if !ended {
		text.WriteString("\nhwrun: no end mark in " + s.Watch.String() + "\n")
	}
	if err := os.WriteFile(out, []byte(text.String()), 0o644); err != nil {
		return err
	}
	s.Port.Output = io.Discard

	if ended && s.backOnItsOwn(ctx) {
		return nil
	}
	return s.reset(ctx, "the monitor won't come back from "+filepath.Base(path))
}

// backOnItsOwn reports whether the monitor restarted after a program returned.
func (s *Session) backOnItsOwn(ctx context.Context) bool {
	if _, err := s.Port.ReadUntil(ctx, greeting); err != nil {
		return false
	}
	_, err := s.Port.ReadUntil(ctx, ">>")
	return err == nil
}

// afterFailure asks for a reset after a step failed, so that the program can be
// tried again on the next run, and returns err.
func (s *Session) afterFailure(ctx context.Context, err error) error {
	fmt.Fprintf(s.Log, "failed: %v\n", err)
	return errors.Join(err, s.reset(ctx, "a step failed"))
}

// reset asks for the reset button and waits for the monitor.
func (s *Session) reset(ctx context.Context, why string) error {
	if err := s.Reset(why); err != nil {
		return err
	}
	timeout := s.Port.Timeout
	s.Port.Timeout = bootTimeout
	defer func() { s.Port.Timeout = timeout }()
	if _, err := s.Port.ReadUntil(ctx, greeting); err != nil {
		return fmt.Errorf("waiting for the monitor after a reset: %w", err)
	}
	if _, err := s.Port.ReadUntil(ctx, ">>"); err != nil {
		return err
	}
	return s.Start(ctx)
}
