package vag

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/internal/pcm"
)

// snr returns the signal to noise ratio of got against want in decibels,
// over the length of want.
func snr(want, got []int16) float64 {
	var sig, noise float64
	for i, w := range want {
		g := 0.0
		if i < len(got) {
			g = float64(got[i])
		}
		sig += float64(w) * float64(w)
		d := float64(w) - g
		noise += d * d
	}
	if noise == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(sig/noise)
}

// flags returns the blocks' flags, one character each.
func flags(data []byte) string {
	var b strings.Builder
	for i := 0; i+BlockSize <= len(data); i += BlockSize {
		b.WriteByte('0' + data[i+1])
	}
	return b.String()
}

func sine(n int, period, amp float64) []int16 {
	s := make([]int16, n)
	for i := range s {
		s[i] = int16(amp * math.Sin(2*math.Pi*float64(i)/period))
	}
	return s
}

func TestEncodeSine(t *testing.T) {
	in := sine(28*40, 37.5, 12000)
	data := Encode(in, Options{})
	assert.Equal(t, (40+2)*BlockSize, len(data))
	assert.Equal(t, make([]byte, BlockSize), data[:BlockSize])
	last := data[len(data)-2*BlockSize:]
	assert.Equal(t, []byte{FlagEnd, FlagEnd | FlagRepeat | FlagStart}, []byte{last[1], last[BlockSize+1]})
	out := Samples(data[BlockSize:])
	if q := snr(in, out); q < 30 {
		t.Errorf("SNR %.1f dB, want at least 30", q)
	}
}

func TestEncodeLoop(t *testing.T) {
	in := sine(28*10, 28, 8000)
	data := Encode(in, Options{Loop: &[2]int{28 * 4, 28 * 10}})
	var flags []byte
	for b := BlockSize; b < len(data); b += BlockSize {
		flags = append(flags, data[b+1])
	}
	assert.Equal(t, []byte{2, 2, 2, 2, 6, 2, 2, 2, 2, 3}, flags)
}

func TestHeader(t *testing.T) {
	v := &VAG{Rate: 22050, Name: "boing", Data: Encode(sine(100, 20, 1000), Options{})}
	back, err := Decode(v.Bytes())
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, v, back)
}

// TestArchive encodes the WAV and AIFF files in the Net Yaroze websites'
// archive that sit beside a VAG of the same name, which a member made with
// Sony's aiff2vag, and compares how close each decodes to the source. The
// encoders differ in their arithmetic, so the blocks are not the same byte
// for byte, but the sound should be as good. PSX_WEBARCHIVE is the
// archive's site directory.
func TestArchive(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	var pairs, worse, loops, loopsSame int
	var gain float64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		ext := strings.ToLower(filepath.Ext(p))
		if err != nil || d.IsDir() || (ext != ".wav" && ext != ".aif" && ext != ".aiff") {
			return err
		}
		stem := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(p)))
		// The VAG is beside its source or, in SCEA's sound packs, in a
		// VAGs directory beside the AIFFs one.
		var golden []byte
		for _, dir := range []string{filepath.Dir(p), filepath.Join(filepath.Dir(p), "..", "VAGs")} {
			ents, _ := os.ReadDir(dir)
			for _, e := range ents {
				if strings.ToLower(e.Name()) == stem+".vag" {
					golden, _ = os.ReadFile(filepath.Join(dir, e.Name()))
				}
			}
		}
		if golden == nil {
			return nil
		}
		want, err := Decode(golden)
		if err != nil {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		snd, err := pcm.Read(b)
		if err != nil {
			t.Logf("%s: %v", p, err)
			return nil
		}
		if snd.Loop != nil || want.Data[BlockSize+1]&FlagRepeat != 0 {
			// A looped sound: the blocks should carry the same flags,
			// the loop starting and ending where Sony's do.
			loops++
			ours := Encode(snd.Samples, Options{Loop: snd.Loop})
			if flags(ours) == flags(want.Data) {
				loopsSame++
			} else {
				t.Logf("%s: loop %v, flags\n%q, Sony's\n%q", p, snd.Loop, flags(ours), flags(want.Data))
			}
			return nil
		}
		pairs++
		ours := snr(snd.Samples, Samples(Encode(snd.Samples, Options{})[BlockSize:]))
		sony := snr(snd.Samples, Samples(want.Data[BlockSize:]))
		gain += ours - sony
		if ours < sony-1 {
			worse++
			t.Logf("%s: %.1f dB, Sony's %.1f dB", p, ours, sony)
		}
		return nil
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Logf("%d of %d sounds encode at least as well as Sony's, within a decibel, %.1f dB better on average", pairs-worse, pairs, gain/float64(max(pairs, 1)))
	t.Logf("%d of %d looped sounds have Sony's flags", loopsSame, loops)
	if pairs == 0 || worse > 0 || loopsSame < loops {
		t.Errorf("%d of %d sounds encode worse than Sony's", worse, pairs)
	}
}
