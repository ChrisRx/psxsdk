package libps_test

import (
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// The twin program (exp/libps/twin) calls each original and its Go
// version on the same inputs and reports on the TTY; see its package
// comment. task build:disc NAME=twin DIR=exp/libps/twin builds it.

// bootTwin boots the twin program, sets its mode, runs it to the end of
// its report, and fails on any FAIL line, quoting it.
func bootTwin(t *testing.T, mode int32) *emutest.Machine {
	t.Helper()
	m := boot(t, "twin")
	// A button is held from the start, so that the pad buffers the
	// script prints hold something.
	m.Pad(1).Press(emu.ButtonCross)
	m.RunUntilTTY("twin ready", 60*emutest.Second)
	m.Bus.Write32(m.Sym("main.mode"), uint32(mode))
	m.RunUntilTTY("twin done", 120*emutest.Second)
	for _, line := range strings.Split(m.TTY(), "\n") {
		if strings.HasPrefix(line, "FAIL ") {
			t.Error(line)
		}
	}
	noComplaints(t, m)
	return m
}

// TestTwinInPlace is the in-place cases: every one reports ok.
func TestTwinInPlace(t *testing.T) {
	m := bootTwin(t, 0)
	for _, last := range []string{"ok rand 31", "ok GetTPage 3 3 1024 512", "ok GetVideoMode 1", "ok CdPosToInt 330000", "ok StartRCnt mask 3", "ok GetRCnt 2"} {
		assert.Equal(t, true, strings.Contains(m.TTY(), last), "the cases ran to %s: %s", last, m.TTY())
	}
}

// TestTwinHardware runs the hardware script under the originals and under
// the Go versions and compares the two machines: video memory, the GPU's
// state, the interrupt mask, and the values the script prints.
func TestTwinHardware(t *testing.T) {
	orig := bootTwin(t, 0)
	gone := bootTwin(t, 1)
	assert.Equal(t, gpuState(orig), gpuState(gone), "GPU state")
	assert.Equal(t, orig.Bus.Read32(0x1f801074), gone.Bus.Read32(0x1f801074), "interrupt mask")
	lines := printed(orig.TTY())
	assert.Equal(t, lines, printed(gone.TTY()), "what the script printed")
	for _, want := range []string{"vsync ", "pad ", "long ", "ticks 3", "dispenv", "clear 0 ", "ot 3 7 ", "clear1 ", "font sprites", "font open 1", "bg sum 0 ", "bg sum 1 ", "bg ot ", "cdinit 1", "search true ", "readfile 5000", "readsync 0 ", "data ok ", "setloc 1", "read 1", "readsync0 0 ", "nop 1 ", "cdtrack 2", "cdplay 0", "cdplaying 0", "cdstop -1", "cdstopped -1", "sndread true", "vab 0 ", "seq 0", "spu play 240 eos 1", "spu stop eos 0", "spu close", "obj lit ", "obj fog ", "obj off alon ", "obj div1 ", "obj div2 fog ", "lines ", "clear 0", "clear2 0", "move 0", "move0 -1", "store 0", "stored ", "kanji open 0", "kanji print ", "kanji flush ", "kanji bad true", "krom2tim 0 ", "krom2tim wide 0 ", "krom2tim2 -1 ", "krom2rawadd "} {
		found := false
		for _, line := range lines {
			found = found || strings.HasPrefix(line, want)
		}
		assert.Equal(t, true, found, "the script printed a %q line", want)
	}
	want, got := orig.VRAM(), gone.VRAM()
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("VRAM differs first at %d,%d: original %04x, Go %04x", i%1024, i/1024, want[i], got[i])
		}
	}
}

// gpuState is the machine's GPU state less the field being shown, bit 31
// of the status, which is a matter of when the script ended.
func gpuState(m *emutest.Machine) string {
	s := m.GPUState()
	i := strings.LastIndex(s, "stat ")
	if i < 0 || len(s) < i+6 {
		return s
	}
	digit := s[i+5]
	if digit >= '8' && digit <= '9' || digit >= 'a' && digit <= 'f' {
		digit -= 8
		if digit > '9' && digit < 'a' {
			digit = digit - 'a' + '2' + 7
		}
	}
	return s[:i+5] + string(digit) + s[i+6:]
}

// printed is everything printed after the program's own output begins,
// less the case reports: the script's values, and whatever the library
// or the kernel said along the way, which has to be the same too, apart
// from two kernel lines that mark known differences:
//
//   - the pad driver's banner (Sony's kernel adds its version), printed
//     when the Go library starts the kernel's pad service, which the
//     resident library left to the boot disc's own program;
//   - OpenBIOS's note of the GTE kernel patch, which the original's
//     InitGeom applies through a kernel call OpenBIOS recognizes, where
//     the Go library writes nothing under OpenBIOS (see patchGTE).
func printed(tty string) []string {
	var out []string
	if i := strings.Index(tty, "twin ready"); i >= 0 {
		tty = tty[i:]
	}
	for _, line := range strings.Split(tty, "\n") {
		word, _, _ := strings.Cut(line, " ")
		if word == "ok" || word == "FAIL" || word == "twin" || line == "" {
			continue
		}
		if strings.HasPrefix(line, "PS-X Control PAD Driver") || strings.HasPrefix(line, "Found C0 patch hash") {
			continue
		}
		out = append(out, line)
	}
	return out
}
