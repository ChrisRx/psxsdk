package probetest

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/x/assert"
)

var kanjiSourceDisc = flag.String("kanjisource", "", "the cue sheet of probes/kanjisource on the clean image, as probe.sh builds it, for TestKanjiNoGlyph")

// noGlyph is the codes of the non-kanji range that probes/kanjisource tries and
// that have no glyph: unassigned cells and trail bytes Shift JIS does not use.
var noGlyph = []string{
	"000081ad", "000081b0", "000081c0", "000081fd", "00008200", "0000823f", "00008240",
	"00008259", "0000827f", "000082f2", "000082fd", "00008397", "00008461",
}

// TestKanjiNoGlyph runs probes/kanjisource on the clean image (spec 51). Krom2Tim
// and the Kanji flush draw a code with no glyph as the 30 bytes before the ROM's
// non-kanji table, as Sony's library does: zeros on OpenBIOS, so a blank cell
// there, and the end of a copyright line on the BIOS in PSX_BIOS, if that is
// set and holds one. Krom2Tim2 draws a blank cell on either, and the cell after
// a flush's last character is zeros whatever the stack held.
func TestKanjiNoGlyph(t *testing.T) {
	if *kanjiSourceDisc == "" {
		t.Skip("-kanjisource names the probe's disc")
	}
	open := filepath.Join(t.TempDir(), "openbios.bin")
	if !assert.NoError(t, os.WriteFile(open, openbios.Image, 0o644)) {
		t.FailNow()
	}
	t.Run("openbios", func(t *testing.T) {
		out := kanjiSource(t, open)
		assert.Equal(t, true, out.romZero, "OpenBIOS has zeros before its font")
		out.check(t)
	})
	t.Run("bios", func(t *testing.T) {
		if os.Getenv("PSX_BIOS") == "" {
			t.Skip("PSX_BIOS is not set")
		}
		out := kanjiSource(t, emutest.BIOSFromEnv(t))
		if out.romZero {
			t.Log("this BIOS has zeros before its font, as OpenBIOS has")
		}
		out.check(t)
	})
}

// kanjiSourceOut is what a run of probes/kanjisource printed.
type kanjiSourceOut struct {
	// romZero is whether the 30 bytes before the non-kanji table are zeros.
	romZero bool

	// krom and krom2 are what Krom2Tim and Krom2Tim2 drew for each code, and
	// cells the lines of cells under each flush's label.
	krom, krom2 map[string]string
	cells       map[string][]string
}

func kanjiSource(t *testing.T, bios string) kanjiSourceOut {
	t.Helper()
	m := emutest.Boot(t, emutest.BIOS(bios), emutest.Disc(*kanjiSourceDisc), emutest.AllowFaults())
	last := m.Frames() + 3000
	for step := 0; m.Frames() < last; step++ {
		// m.TTY copies the text, so look for the end only now and then.
		if step%4096 == 0 && strings.Contains(m.TTY(), "\nEND") {
			break
		}
		m.Step()
	}
	out := kanjiSourceOut{
		krom:  map[string]string{},
		krom2: map[string]string{},
		cells: map[string][]string{},
	}
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	if !strings.Contains(tty, "\nEND") {
		t.Fatalf("the probe did not end: %q", tty[max(0, len(tty)-400):])
	}
	label := ""
	for _, l := range strings.Split(tty, "\n") {
		switch {
		case strings.HasPrefix(l, "rom before non-kanji:"):
			out.romZero = strings.Trim(strings.TrimPrefix(l, "rom before non-kanji:"), " 0") == ""
		case strings.HasPrefix(l, "Krom2Tim "), strings.HasPrefix(l, "Krom2Tim2 "):
			f := strings.SplitN(l, " ", 5)
			if len(f) < 5 {
				continue
			}
			if f[0] == "Krom2Tim" {
				out.krom[f[1]] = f[3] + " " + f[4]
			} else {
				out.krom2[f[1]] = f[3] + " " + f[4]
			}
		case strings.HasPrefix(l, "flush "):
			label, _, _ = strings.Cut(strings.TrimPrefix(l, "flush "), ":")
		case strings.HasPrefix(l, "  cell "):
			out.cells[label] = append(out.cells[label], strings.TrimSpace(l))
		}
	}
	return out
}

func (out kanjiSourceOut) check(t *testing.T) {
	t.Helper()
	want := "0 the 30 bytes before the non-kanji table"
	if out.romZero {
		want = "0 blank"
	}
	for _, c := range noGlyph {
		assert.Equal(t, want, out.krom[c], "Krom2Tim "+c)
		assert.Equal(t, "0 blank", out.krom2[c], "Krom2Tim2 "+c)
	}
	// The full-width space has a glyph, and it is blank.
	assert.Equal(t, "0 blank", out.krom["00008140"])
	assert.Equal(t, "0 the 30 bytes before the kanji table", out.krom["0000897f"])

	// A stream of a hiragana, 81ADh and a hiragana: cell 1 is the code's, and
	// cell 3, after the last character, is zeros.
	const zeros = " stack 0 zero 32 other 0"
	cells := out.cells["empty cell 81ad between two"]
	assert.Equal(t, 4, len(cells))
	if len(cells) == 4 {
		assert.Equal(t, out.romZero, cells[1] == "cell 1:"+zeros, cells[1])
		assert.Equal(t, "cell 3:"+zeros, cells[3])
	}
	cells = out.cells["1 char"]
	assert.Equal(t, 2, len(cells))
	if len(cells) == 2 {
		assert.Equal(t, "cell 1:"+zeros, cells[1])
	}
}
