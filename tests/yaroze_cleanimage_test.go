package tests

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/internal/cmd/hwrun"
	"go.chrisrx.dev/psx/serial"
)

// monitorStart is where the boot disc's monitor sits, which the clean image
// must end below.
const monitorStart = 0x80068000

// cleanImage returns the clean resident image as an upload would send it.
func cleanImage(t *testing.T) *serial.Program {
	t.Helper()
	f, err := os.Open("../link/libps.exe.gz")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	data, err := io.ReadAll(z)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	exe, err := psx.ParseFile(bytes.NewReader(data))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return serial.EXEProgram(exe)
}

// TestYarozeCleanImageInPlace records why the image can't go straight over
// Sony's: the monitor stops answering partway, since it needs that library
// while it runs.
func TestYarozeCleanImageInPlace(t *testing.T) {
	_, port, mon := sonyMonitor(t)
	port.Timeout = 5 * time.Second
	if err := mon.WriteSegments(t.Context(), cleanImage(t).Segments...); err == nil {
		t.Error("the monitor survived its library being overwritten, so the loader isn't needed")
	}
}

// hwSession runs the hardware session's probe through hwrun as the session
// will, from the boot disc's monitor, and returns what it wrote. It runs a second
// time on the same boot, which only needs a monitor that comes back, and the
// reset the emulated console can't do fails the test.
func hwSession(t *testing.T, library string, image *serial.Program, out string) (first, second string) {
	t.Helper()
	m, port, _ := sonyMonitor(t)
	port.Timeout = 60 * time.Second
	s := &hwrun.Session{
		Port:    port,
		Library: library,
		Out:     out,
		Watch:   60 * time.Second,
		Log:     io.Discard,
		Reset:   func(why string) error { return errors.New("reset asked for: " + why) },
	}
	if image != nil {
		monitor, err := serial.OpenMonitor(serialProgram(t, "monitor"), 115200)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		s.TakeOver, err = serial.TakeOver(monitor, image.Segments[0])
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		s.Marker = binary.LittleEndian.Uint32(monitor.Segments[0].Data)
	}
	if !assert.NoError(t, s.Start(t.Context())) {
		t.FailNow()
	}
	path := filepath.Join(out, "hwprobe-"+library+".txt")
	program := []hwrun.Job{hwrun.NewJob(serialProgram(t, "hwprobe"))}
	if !assert.NoError(t, s.Run(t.Context(), program)) {
		t.FailNow()
	}
	data, err := os.ReadFile(path)
	assert.NoError(t, err)

	// A program with a file is skipped, which is how a session goes on later.
	before, _ := os.Stat(path)
	assert.NoError(t, s.Run(t.Context(), program))
	after, _ := os.Stat(path)
	assert.Equal(t, before.ModTime(), after.ModTime(), "a finished program shouldn't run again")

	// Starting again on the same boot finds the library in place.
	if image != nil {
		var log bytes.Buffer
		s.Log = &log
		assert.NoError(t, s.Start(t.Context()))
		assert.Equal(t, true, strings.Contains(log.String(), "already in place"), log.String())
		s.Log = io.Discard
		seg := image.Segments[0]
		for i, b := range seg.Data[:4096] {
			if got := m.Bus.Read8(seg.Addr + uint32(i)); got != b {
				t.Fatalf("byte %d of the image, at %08X, is %02X and not %02X", i, seg.Addr+uint32(i), got, b)
			}
		}
	}

	assert.NoError(t, os.Remove(path))
	assert.NoError(t, s.Run(t.Context(), program))
	again, err := os.ReadFile(path)
	assert.NoError(t, err)
	return string(data), string(again)
}

// readings keeps the lines of the probe's that carry results.
func readings(out string) map[string]string {
	lines := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && slices.Contains([]string{"T", "S", "F", "H"}, f[0]) {
			lines[f[1]] = strings.Join(f[2:], " ")
		}
	}
	return lines
}

// TestYarozeCleanImage is the hardware session (spec 35) rehearsed: hwrun from
// the boot disc's own monitor, running the session's probe under Sony's library,
// and then, after replacing them, on the clean library and our monitor, twice
// each with no reset. The two pictures must hash the same. The emulator's
// readings go in PSX_HW_OUT, as the baseline to compare a console's with.
func TestYarozeCleanImage(t *testing.T) {
	image := cleanImage(t)
	seg := image.Segments[0]
	assert.Equal(t, true, seg.Addr+uint32(len(seg.Data)) <= monitorStart, "the image must end below the monitor")
	out := cmp.Or(os.Getenv("PSX_HW_OUT"), t.TempDir())
	for _, name := range []string{"sony", "clean"} {
		os.Remove(filepath.Join(out, "hwprobe-"+name+".txt"))
	}
	sony, sonyAgain := hwSession(t, "sony", nil, out)
	clean, cleanAgain := hwSession(t, "clean", image, out)
	assert.Equal(t, readings(sony)["scene"], readings(sonyAgain)["scene"])
	assert.Equal(t, readings(clean)["scene"], readings(cleanAgain)["scene"])
	a, b := readings(sony), readings(clean)
	if len(a) == 0 {
		t.Fatalf("no readings: %q", sony)
	}
	assert.Equal(t, a["scene"], b["scene"], "the picture should hash the same")
	assert.Equal(t, len(a), len(b))
	for _, n := range slices.Sorted(maps.Keys(a)) {
		t.Logf("%-20s sony %-40s clean %s", n, a[n], b[n])
	}
}

// TestYarozeArchiveJob runs an original program with its data files as the
// hardware session would, on each library, and checks it's still running at the
// end with no crash. PSX_HW_ARCHIVE names the archive's programs.tsv.
func TestYarozeArchiveJob(t *testing.T) {
	tsv := os.Getenv("PSX_HW_ARCHIVE")
	if tsv == "" {
		t.Skip("PSX_HW_ARCHIVE is not set")
	}
	jobs, err := hwrun.ArchiveJobs(tsv, []string{cmp.Or(os.Getenv("PSX_HW_PROGRAM"), "scee-topten-adven")})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	image := cleanImage(t)
	for _, library := range []string{"sony", "clean"} {
		t.Run(library, func(t *testing.T) {
			m, port, _ := sonyMonitor(t)
			port.Timeout = 60 * time.Second
			errReset := errors.New("reset asked for")
			s := &hwrun.Session{
				Port:    port,
				Library: library,
				Out:     t.TempDir(),
				Watch:   20 * time.Second,
				Log:     io.Discard,
				Reset:   func(why string) error { return errReset },
			}
			if library == "clean" {
				monitor, err := serial.OpenMonitor(serialProgram(t, "monitor"), 115200)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				s.TakeOver, err = serial.TakeOver(monitor, image.Segments[0])
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				s.Marker = binary.LittleEndian.Uint32(monitor.Segments[0].Data)
			}
			if !assert.NoError(t, s.Start(t.Context())) {
				t.FailNow()
			}

			// It never returns, so it ends by asking for a reset.
			if err := s.Run(t.Context(), jobs); !errors.Is(err, errReset) {
				t.Errorf("the program should end by asking for a reset, not %v", err)
			}
			data, err := os.ReadFile(filepath.Join(s.Out, jobs[0].Name+"-"+library+".txt"))
			assert.NoError(t, err)
			t.Logf("pc %08x, printed: %.300q", m.CPU.PC, string(data))
			if m.CPU.PC>>16 == 0xbfc0 || m.CPU.PC < 0x10000 {
				t.Logf("pc in the BIOS: %08x, epc %08x, cause %08x", m.CPU.PC, m.CPU.COP0[14], m.CPU.COP0[13])
			}
		})
	}
}
