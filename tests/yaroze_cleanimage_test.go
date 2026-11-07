package tests

import (
	"bytes"
	"cmp"
	"compress/gzip"
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
// will, and returns what it wrote. The emulated console can't be reset, so a
// request for one ends the session, and only the clean library should make one.
func hwSession(t *testing.T, library string, image *serial.Program, out string) string {
	t.Helper()
	_, port, _ := sonyMonitor(t)
	port.Timeout = 60 * time.Second
	errReset := errors.New("reset asked for")
	s := &hwrun.Session{
		Port:    port,
		Image:   image,
		Library: library,
		Out:     out,
		Watch:   60 * time.Second,
		Log:     io.Discard,
		Reset:   func(why string) error { return errReset },
	}
	assert.NoError(t, s.Start(t.Context()))
	err := s.Run(t.Context(), []string{serialProgram(t, "hwprobe")})
	if image == nil {
		assert.NoError(t, err)
	} else if !errors.Is(err, errReset) {
		t.Errorf("the clean library should end by asking for a reset, not %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "hwprobe-"+library+".txt"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A program with a file is skipped, which is how a session goes on later.
	s.Reset = func(why string) error { t.Errorf("a finished program ran again: %s", why); return nil }
	assert.NoError(t, s.Run(t.Context(), []string{serialProgram(t, "hwprobe")}))
	return string(data)
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

// TestYarozeCleanImage is the hardware session (spec 35) rehearsed: hwrun on the
// boot disc's own monitor, running the session's probe under Sony's library and
// then on the clean image, set side by side. The two pictures must hash the
// same. The emulator's readings go in PSX_HW_OUT, as the baseline to compare a
// console's with.
func TestYarozeCleanImage(t *testing.T) {
	image := cleanImage(t)
	seg := image.Segments[0]
	assert.Equal(t, true, seg.Addr+uint32(len(seg.Data)) <= monitorStart, "the image must end below the monitor")
	out := cmp.Or(os.Getenv("PSX_HW_OUT"), t.TempDir())
	for _, name := range []string{"sony", "clean"} {
		os.Remove(filepath.Join(out, "hwprobe-"+name+".txt"))
	}
	sony := hwSession(t, "sony", nil, out)
	clean := hwSession(t, "clean", image, out)
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
