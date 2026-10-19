package vab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/vag"
)

func TestEnvelope(t *testing.T) {
	// What Sony's mkvab made of the envelopes in the definitions it was
	// given, read from the banks it wrote.
	for _, c := range []struct {
		e            Envelope
		adsr1, adsr2 uint16
	}{
		{Envelope{AR: 127, DR: 0, SR: 0, RR: 31, SL: 15}, 0x80ff, 0xdfe0},
		{Envelope{AR: 127, DR: 15, SR: 0, RR: 31, SL: 15}, 0x800f, 0xdfe0},
		{Envelope{AR: 127, DR: 0, SR: 0, RR: 0, SL: 15}, 0x80ff, 0xdfff},
	} {
		a1, a2 := c.e.Words()
		assert.Equal(t, [2]uint16{c.adsr1, c.adsr2}, [2]uint16{a1, a2})
		assert.Equal(t, c.e, EnvelopeOf(a1, a2))
	}
	for _, e := range []Envelope{{AR: 90, DR: 3, SR: 40, RR: 18, SL: 9}, {AR: 1, DR: 14, SR: -100, RR: 5, SL: 0}} {
		assert.Equal(t, e, EnvelopeOf(e.Words()))
	}
}

const def = `VabHdr
	form = 'VABp'
	ver = 0x7
	id = 0
	fsize = 0
	ps = 2
	ts = 3
	vs = 2

ProgAtr 0
	tones = 2
	mvol = 100
	mpan = 64

ProgAtr 5
	tones = 1
	mvol = 127
	mpan = 32

VagAtr 0 0
	vol = 110
	center = 60
	min = 0
	max = 59
	ar = 127
	dr = 0
	sr = 10-
	rr = 20
	sl = 15
	prog = 0
	vag = 1

VagAtr 0 1
	center = 72
	min = 60
	max = 127
	prog = 0
	vag = 2

VagAtr 5 0
	mode = 4
	center = 48
	prog = 5
	vag = 2
`

func TestDef(t *testing.T) {
	v, err := ParseDef(def)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, 2, len(v.Tones))
	assert.Equal(t, int16(5), v.Tones[1][0].Program)
	assert.Equal(t, Envelope{AR: 127, SR: -10, RR: 20, SL: 15}, EnvelopeOf(v.Tones[0][0].ADSR1, v.Tones[0][0].ADSR2))
	assert.Equal(t, 2, v.MaxVAG())
	v.VAGs = [][]byte{make([]byte, 32), make([]byte, 48)}

	back, err := ParseDef(v.Def())
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	back.VAGs = v.VAGs
	assert.Equal(t, v, back)

	vh, vb := v.VH(), v.VB()
	assert.Equal(t, HeaderSize+MaxPrograms*ProgramSize+2*MaxTones*ToneSize+512, len(vh))
	n, err := HeaderLen(vh)
	assert.Equal(t, len(vh), n)
	assert.NoError(t, err)
	decoded, err := Decode(vh, vb)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, v, decoded)
}

// TestArchive builds the banks in the Net Yaroze websites' archive whose
// definition file, VAGs and mkvab command line survive beside the VH and VB
// Sony's mkvab and vabsplit made, and compares them. Sony's mkvab wrote
// version 0x20 whatever the definition said, and left the tones' unused
// bytes as whatever was in its memory, so those are set aside; the rest,
// and the body, must be the same. PSX_WEBARCHIVE is the archive's site
// directory.
func TestArchive(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	for _, c := range []struct {
		dir, def, out string
		vags          []string
	}{
		{"yaroze_scea/demos/scei/BOMB96.ZIP.x/BOMB/DATA", "BOMB.DEF", "BOMB", []string{"DUMMY.VAG", "BOMB.VAG"}},
		{"yaroze_scea/demos/scea/snddemo.zip.x", "WAVES.DEF", "WAVES", []string{"SINE.VAG", "SQUARE.VAG", "TRIANGLE.VAG"}},
		{"yaroze/ftp/scei/sample/sound/wavplay.zip.x/DAT", "WAV.DEF", "NEW", []string{"SHUTTER.VAG"}},
	} {
		t.Run(c.out, func(t *testing.T) {
			dir := filepath.Join(root, c.dir)
			text, err := os.ReadFile(filepath.Join(dir, c.def))
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			v, err := ParseDef(string(text))
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			for _, name := range c.vags {
				b, err := os.ReadFile(filepath.Join(dir, name))
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				g, err := vag.Decode(b)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				v.VAGs = append(v.VAGs, g.Data)
			}
			wantVH, err := os.ReadFile(filepath.Join(dir, c.out+".VH"))
			assert.NoError(t, err)
			wantVB, err := os.ReadFile(filepath.Join(dir, c.out+".VB"))
			assert.NoError(t, err)
			want, err := Decode(wantVH, wantVB)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			v.Version = want.Version
			for i := range want.Tones {
				for j := range want.Tones[i] {
					w := &want.Tones[i][j]
					if v.Tones[i][j].VAG == 0 {
						*w = Tone{} // a slot mkvab left as it found it
						continue
					}
					w.VibW, w.VibT, w.PorW, w.PorT = 0, 0, 0, 0
				}
			}
			assert.Equal(t, want, v)
			assert.Equal(t, wantVB, v.VB())
			assert.Equal(t, len(wantVH), len(v.VH()))
			assert.Equal(t, true, strings.HasPrefix(string(v.VH()), string(wantVH[:16])))
		})
	}
}
