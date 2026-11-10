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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

	// softResetGap is the time between Control-Cs, softResetTries how many are
	// sent, and softResetBoot how long the disc then gets to boot, less than
	// bootTimeout since it's only a wait before asking for the button.
	softResetGap   = 500 * time.Millisecond
	softResetTries = 10
	softResetBoot  = 90 * time.Second
)

// Job is a program to run, with any files its batch loads into RAM first.
type Job struct {
	// Name names the output file.
	Name string

	// Path is the executable.
	Path string

	// Loads are the files to put in RAM, as "PATH@ADDRESS", in hexadecimal.
	Loads []string
}

// NewJob is a job for an executable on its own, named after its file.
func NewJob(path string) Job {
	return Job{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path}
}

// ArchiveJobs reads the jobs named from a programs.tsv, as
// exp/cleanlib/image/archive.py writes it from each program's AUTO batch: its
// name, directory, executable, and files to load, separated by spaces.
func ArchiveJobs(path string, names []string) ([]Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	all := map[string]Job{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			continue
		}
		all[f[0]] = Job{Name: f[0], Path: f[2], Loads: strings.Fields(f[3])}
	}
	jobs := make([]Job, 0, len(names))
	for _, name := range names {
		job, ok := all[name]
		if !ok {
			return nil, fmt.Errorf("%s has no program %s", path, name)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// program reads what's uploaded for a job.
func (j Job) program() (*serial.Program, error) {
	prog, err := serial.OpenProgram(j.Path)
	if err != nil {
		return nil, err
	}
	for _, load := range j.Loads {
		path, at, ok := strings.Cut(load, "@")
		if !ok {
			return nil, fmt.Errorf("%q has no address", load)
		}
		addr, err := strconv.ParseUint(at, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", load, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		prog.Segments = append(prog.Segments, serial.Segment{Name: filepath.Base(path), Addr: uint32(addr), Data: data})
	}
	return prog, nil
}

// Session is a run of programs on one console.
type Session struct {
	Port *serial.Port

	// TakeOver is what replaces the boot disc's monitor and library, from
	// serial.TakeOver, or nil to stay on them. Marker is the first word of the
	// monitor it brings, which tells if it's already there.
	TakeOver *serial.Program
	Marker   uint32

	// Stamp is a file that names what was last uploaded, by a hash of
	// TakeOver. With the clean monitor already there, Start goes on with it
	// only if the stamp names this TakeOver, and otherwise resets the console
	// and uploads it, so that a console left with another image (Sony's,
	// through -image, or an older build) isn't taken for this one. Empty
	// skips the check.
	Stamp string

	// Library names the library in the output files' names.
	Library string

	// Out is the directory for the output files.
	Out string

	// Watch is how long a program runs before it counts as done with no end
	// mark, which is how original programs finish here.
	Watch time.Duration

	// Peeks are times into each program's run, in order, at which it is
	// stopped (the clean monitor's Control-B, Monitor.Stop), its registers
	// and the memory in Reads saved, and let go on. Only the clean monitor
	// can do it.
	Peeks []time.Duration
	Reads []Region

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
		if s.stamped() {
			fmt.Fprintln(s.Log, "the clean library is already in place")
			return nil
		}
		return s.reset(ctx, "the clean monitor is up with another library image")
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
	if s.Stamp != "" {
		if err := os.MkdirAll(filepath.Dir(s.Stamp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(s.Stamp, []byte(s.takeOverHash()+"\n"), 0o644); err != nil {
			return err
		}
	}
	s.mon, err = serial.NewMonitor(ctx, s.Port)
	return err
}

// takeOverHash is a hash of what TakeOver uploads.
func (s *Session) takeOverHash() string {
	h := sha256.New()
	for _, seg := range s.TakeOver.Segments {
		fmt.Fprintf(h, "%08x %d\n", seg.Addr, len(seg.Data))
		h.Write(seg.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// stamped reports whether the stamp names this TakeOver, or there is no stamp
// to check.
func (s *Session) stamped() bool {
	if s.Stamp == "" {
		return true
	}
	b, err := os.ReadFile(s.Stamp)
	return err == nil && strings.TrimSpace(string(b)) == s.takeOverHash()
}

// Probe returns the monitor's reply to reading the word Start's check reads.
func (s *Session) Probe(ctx context.Context) string {
	reply, _ := s.mon.Command(ctx, fmt.Sprintf("dw %08x 4", monitorStart))
	return reply
}

// Run runs each program.
func (s *Session) Run(ctx context.Context, jobs []Job) error {
	for _, job := range jobs {
		if err := s.one(ctx, job); err != nil {
			return fmt.Errorf("%s: %w", job.Name, err)
		}
	}
	return nil
}

// outPath is where a program's output goes.
func (s *Session) outPath(job Job) string {
	return filepath.Join(s.Out, job.Name+"-"+s.Library+".txt")
}

func (s *Session) one(ctx context.Context, job Job) error {
	out := s.outPath(job)
	path := job.Name
	if _, err := os.Stat(out); err == nil {
		fmt.Fprintf(s.Log, "%s: already done, in %s\n", path, out)
		return nil
	}
	prog, err := job.program()
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

	ended, err := s.watch(ctx, job)
	if err != nil {
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
	return s.reset(ctx, "the monitor won't come back from "+path)
}

// Region is memory to save at a peek.
type Region struct {
	Addr uint32
	Size int
}

// ParseRegion reads a region as "ADDR:SIZE", the address in hexadecimal and
// the size in bytes, decimal or 0x-prefixed.
func ParseRegion(s string) (Region, error) {
	a, n, ok := strings.Cut(s, ":")
	if !ok {
		return Region{}, fmt.Errorf("region %q is not ADDR:SIZE", s)
	}
	addr, err := strconv.ParseUint(strings.TrimPrefix(a, "0x"), 16, 32)
	if err != nil {
		return Region{}, fmt.Errorf("region %q: %w", s, err)
	}
	size, err := strconv.ParseUint(n, 0, 31)
	if err != nil {
		return Region{}, fmt.Errorf("region %q: %w", s, err)
	}
	return Region{Addr: uint32(addr), Size: int(size)}, nil
}

// watch waits for the running program to print its end mark, for up to
// Watch, stopping it at each of Peeks on the way. It reports whether the
// mark came.
func (s *Session) watch(ctx context.Context, job Job) (bool, error) {
	timeout := s.Port.Timeout
	defer func() { s.Port.Timeout = timeout }()
	start := time.Now()
	for i, at := range append(s.Peeks, s.Watch) {
		if at > s.Watch {
			at = s.Watch
		}
		s.Port.Timeout = max(at-time.Since(start), time.Millisecond)
		_, err := s.Port.ReadUntil(ctx, endMark)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, serial.ErrTimeout) {
			return false, err
		}
		if i == len(s.Peeks) || at == s.Watch {
			return false, nil
		}
		s.Port.Timeout = timeout
		if err := s.peek(ctx, job, i+1, time.Since(start)); err != nil {
			return false, fmt.Errorf("peek %d: %w", i+1, err)
		}
	}
	return false, nil
}

// peek stops the program, saves its registers and Reads into the output
// directory, and lets it go on.
func (s *Session) peek(ctx context.Context, job Job, n int, at time.Duration) error {
	stopped := time.Now()
	d, err := s.mon.Stop(ctx)
	if err != nil {
		return err
	}
	base := filepath.Join(s.Out, fmt.Sprintf("%s-%s-peek%d", job.Name, s.Library, n))
	var regs strings.Builder
	fmt.Fprintf(&regs, "peek %d at %v, signal %d\n", n, at.Round(time.Millisecond), d.Signal)
	for r := range 38 {
		v, err := d.ReadRegister(ctx, r)
		if err != nil {
			return err
		}
		fmt.Fprintf(&regs, "r%d %08x\n", r, v)
	}
	if err := os.WriteFile(base+"-regs.txt", []byte(regs.String()), 0o644); err != nil {
		return err
	}
	for _, r := range s.Reads {
		b, err := d.ReadMemory(ctx, r.Addr, r.Size)
		if err != nil {
			return err
		}
		if err := os.WriteFile(fmt.Sprintf("%s-%08x.bin", base, r.Addr), b, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(s.Log, "%s: peek %d at %v, stopped for %v\n", job.Name, n, at.Round(time.Millisecond), time.Since(stopped).Round(time.Millisecond))
	return d.Continue(ctx)
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

// softReset asks the clean monitor's handler to reset the console, by sending
// Control-C until it has rebooted, and reports whether the boot disc's monitor
// came back. The program may take a byte for itself if it's printing as the
// byte comes, so it sends more than one. It can't help a console with its
// interrupts off.
func (s *Session) softReset(ctx context.Context) bool {
	if s.TakeOver == nil {
		return false
	}
	fmt.Fprintln(s.Log, "resetting the console from the serial port")
	timeout := s.Port.Timeout
	defer func() { s.Port.Timeout = timeout }()
	s.Port.Timeout = softResetGap
	back := false
	for i := 0; i < softResetTries && !back; i++ {
		if _, err := s.Port.Write([]byte{0x03}); err != nil {
			return false
		}
		_, err := s.Port.ReadUntil(ctx, greeting)
		back = err == nil
	}
	s.Port.Timeout = softResetBoot
	if !back {
		if _, err := s.Port.ReadUntil(ctx, greeting); err != nil {
			return false
		}
	}
	_, err := s.Port.ReadUntil(ctx, ">>")
	return err == nil
}

// reset gets the console back, by itself if it can and otherwise by asking for
// the reset button, and waits for the monitor.
func (s *Session) reset(ctx context.Context, why string) error {
	if s.softReset(ctx) {
		return s.Start(ctx)
	}
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
