package libps_test

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/link"
)

// The tests run programs in the emulator from discs task build:disc and
// build:disc:go write under .build, or the directory PSX_DISCS names:
// NAME/NAME.cue with prog.elf beside it. A test skips when its disc is
// not built, so a passing run is one that says ok, not SKIP.

func disc(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../../.build"), name, name+".cue")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no disc for %s: %v", name, err)
	}
	return path
}

// boot boots the BIOS with the named disc in the drive and the program's
// symbols loaded.
func boot(t *testing.T, name string) *emutest.Machine {
	t.Helper()
	cue := disc(t, name)
	return emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc(cue),
		emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")),
	)
}

// complaints are what the library prints when something it waited for
// never came. A run that prints one is a run where the library lost an
// interrupt or the GPU, whatever the picture looks like afterwards.
var complaints = []string{"VSync: timeout", "GPU timeout", "intr timeout", "unexpected interrupt"}

// noComplaints fails the test on any of the library's complaints in the
// machine's output.
func noComplaints(t *testing.T, m *emutest.Machine) {
	t.Helper()
	for _, c := range complaints {
		if strings.Contains(m.TTY(), c) {
			t.Errorf("the library printed %q", c)
		}
	}
}

// elfBytes reads the program's ELF beside the disc.
func elfBytes(t *testing.T, cue string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(cue), "prog.elf"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return data
}

// TestProbe runs the probe under the Go library: it prints through the
// library's putchar, and the program was linked where the resident image
// used to be, with nothing under it.
func TestProbe(t *testing.T) {
	cue := disc(t, "probe-go")
	m := boot(t, "probe-go")
	m.RunUntilTTY("probe ok", 60*emutest.Second)
	img, err := link.ReadImage(bytes.NewReader(elfBytes(t, cue)))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, uint32(0x80010000), img.Addr, "the program loads where the resident image was")
}
