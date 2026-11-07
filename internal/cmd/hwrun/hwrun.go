// Package hwrun runs programs on a real Net Yaroze for the hardware session
// (spec 35), and keeps what each prints. It is the hwrun tool.
//
// Each program's output goes in a file of its own, and one that has a file
// is skipped, so a session can stop and go on later. A console stuck in a
// program can't be told to stop, so hwrun asks for the reset button when the
// monitor doesn't come back, which on the clean library is after every
// program: the boot disc's monitor doesn't survive one returning there.
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

	// bootTimeout is how long a console gets to boot its disc after a reset.
	bootTimeout = 3 * time.Minute
)

// Session is a run of programs on one console.
type Session struct {
	Port *serial.Port

	// Image is the resident library to run on, uploaded with each program, or
	// nil for the one the boot disc left.
	Image *serial.Program

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

// Start connects to the monitor.
func (s *Session) Start(ctx context.Context) error {
	mon, err := serial.NewMonitor(ctx, s.Port)
	if err != nil {
		return err
	}
	s.mon = mon
	_, err = mon.Command(ctx, "")
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
	if s.Image != nil {
		prog, err = serial.Relocated(prog, s.Image.Segments...)
		if err != nil {
			return err
		}
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

	if s.Image == nil && ended && s.backOnItsOwn(ctx) {
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
