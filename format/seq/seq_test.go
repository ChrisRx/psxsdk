package seq

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// smf builds a format 1 MIDI file from tracks of raw event bytes.
func smf(division uint16, tracks ...[]byte) []byte {
	b := []byte{'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 1, 0, byte(len(tracks)), byte(division >> 8), byte(division)}
	for _, t := range tracks {
		n := len(t)
		b = append(b, 'M', 'T', 'r', 'k', byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		b = append(b, t...)
	}
	return b
}

func TestConvert(t *testing.T) {
	in := smf(480,
		[]byte{
			0x00, 0xff, 0x03, 0x02, 'h', 'i', // name: dropped
			0x00, 0xff, 0x58, 0x04, 0x03, 0x02, 0x18, 0x08, // 3/4: the header
			0x00, 0xff, 0x51, 0x03, 0x07, 0xa1, 0x20, // 120 bpm: the header
			0x83, 0x60, 0xff, 0x51, 0x03, 0x0f, 0x42, 0x40, // 60 bpm at 480
			0x00, 0xff, 0x2f, 0x00,
		},
		[]byte{
			0x00, 0xc0, 0x05,
			0x00, 0xb0, 0x07, 0x64, // volume: kept
			0x00, 0x01, 0x40, // modulation, under running status: dropped
			0x00, 0x90, 0x3c, 0x40,
			0x83, 0x60, 0x80, 0x3c, 0x40, // note off at 480: note on, velocity 0
			0x00, 0xff, 0x2f, 0x00,
		},
	)
	out, dropped, err := Convert(in, Options{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, []byte{
		'p', 'Q', 'E', 'S', 0, 0, 0, 1, 0x01, 0xe0, 0x07, 0xa1, 0x20, 0x03, 0x02,
		0x00, 0xc0, 0x05,
		0x00, 0xb0, 0x07, 0x64,
		0x00, 0x90, 0x3c, 0x40,
		0x83, 0x60, 0xff, 0x51, 0x0f, 0x42, 0x40,
		0x00, 0x90, 0x3c, 0x00,
		0x00, 0xff, 0x2f, 0x00,
	}, out)
	assert.Equal(t, Dropped{Controls: map[byte]int{1: 1}, Metas: map[byte]int{3: 1}}, dropped)
}

// TestArchive converts the MIDI files in the Net Yaroze websites' archive
// that sit beside a SEQ of the same name, which a member made with Sony's
// smf2seq, and compares them. Sony released two versions: SCEA's, whose
// files Convert matches byte for byte, and SCEI's, which takes events at the
// same time from the tracks in a different order and writes the end twice.
// Every file must play the same, the same events at the same times, but for
// two things members did that Convert does not: one ran smf2seq with -B,
// dropping bank select, and one file's sequencer-specific meta event came
// out as a bare FF 7F, which Convert drops. PSX_WEBARCHIVE is the archive's
// site directory.
func TestArchive(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	var pairs, same, plays int
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".mid") {
			return err
		}
		stem := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(p)))
		var golden []byte
		ents, _ := os.ReadDir(filepath.Dir(p))
		for _, e := range ents {
			if strings.ToLower(e.Name()) == stem+".seq" {
				golden, _ = os.ReadFile(filepath.Join(filepath.Dir(p), e.Name()))
			}
		}
		if golden == nil {
			return nil
		}
		in, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		pairs++
		out, _, err := Convert(in, Options{})
		if err != nil {
			t.Logf("%s: %v", p, err)
			return nil
		}
		if string(out) == string(golden) {
			same++
			plays++
			return nil
		}
		gh, ge, err := Decode(golden)
		if err != nil {
			t.Logf("%s: Sony's: %v", p, err)
			return nil
		}
		oh, oe, err := Decode(out)
		if !assert.NoError(t, err) {
			return nil
		}
		if gh != oh {
			t.Logf("%s: header %+v, Sony's %+v", p, oh, gh)
			return nil
		}
		if g, o := byTime(ge), byTime(oe); !maps.EqualFunc(g, o, slices.Equal) {
			for tm := range g {
				if !slices.Equal(g[tm], o[tm]) {
					t.Logf("%s: at %d %q, Sony's %q", p, tm, o[tm], g[tm])
					break
				}
			}
			return nil
		}
		plays++
		return nil
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Logf("%d of %d MIDI files convert to their SEQ byte for byte, %d play the same", same, pairs, plays)
	if pairs == 0 || plays < pairs || same < 5 {
		t.Errorf("%d of %d MIDI files play differently from their SEQ, %d match byte for byte", pairs-plays, pairs, same)
	}
}

// byTime groups events by time, each time's sorted, leaving out bank
// select and anything but a tempo change among meta events.
func byTime(evs []Event) map[int][]string {
	m := map[int][]string{}
	for _, e := range evs {
		if e.Data[0] == 0xff && e.Data[1] != 0x51 || e.Data[0]>>4 == 0xb && e.Data[1] == 0 {
			continue
		}
		m[e.Time] = append(m[e.Time], fmt.Sprintf("% x", e.Data))
	}
	for _, v := range m {
		slices.Sort(v)
	}
	return m
}
