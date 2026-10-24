package tim

import (
	"bytes"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	_ "go.chrisrx.dev/psx/internal/bmp"
)

func TestRoundTrip(t *testing.T) {
	p := image.NewPaletted(image.Rect(0, 0, 5, 2), color.Palette{
		color.NRGBA{},                       // transparent
		color.NRGBA{A: 255},                 // black
		color.NRGBA{R: 255, A: 255},         // red
		color.NRGBA{G: 255, B: 255, A: 255}, // cyan
		color.NRGBA{R: 8, G: 16, B: 24, A: 255},
	})
	copy(p.Pix, []uint8{0, 1, 2, 3, 4, 4, 3, 2, 1, 0})
	tm, err := Encode(p, Options{Mode: Auto, X: 640, Y: 0, CLUTX: 0, CLUTY: 480})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, Mode4, tm.Mode)
	assert.Equal(t, 2, tm.Pixels.W) // five pixels pad to eight, two units
	assert.Equal(t, []uint16{0x0000, 0x8000, 0x001f, 0x7fe0, 0x0c41}, tm.CLUT.Data[:5])
	assert.Equal(t, uint16(480<<6), tm.CBA())
	assert.Equal(t, uint16(10), tm.TPage())

	back, err := Decode(tm.Bytes())
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, tm, back)
	for y := range 2 {
		for x := range 5 {
			assert.Equal(t, int(p.Pix[y*5+x]), back.Index(x, y))
		}
	}
}

func TestBlackTransparent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{A: 255})
	img.Set(1, 0, color.NRGBA{R: 255, A: 255})
	tm, err := Encode(img, Options{Mode: Mode16, Black: BlackTransparent, Semi: true})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, []uint16{0x0000, 0x801f}, tm.Pixels.Data)
}

func TestQuantize(t *testing.T) {
	// A gradient of 64 grays reduced to 16 colors keeps its ends and
	// keeps the order of what it reduces.
	img := image.NewNRGBA(image.Rect(0, 0, 64, 1))
	for x := range 64 {
		img.Set(x, 0, color.NRGBA{R: uint8(x * 4), G: uint8(x * 4), B: uint8(x * 4), A: 255})
	}
	tm, err := Encode(img, Options{Mode: Mode4})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	last := -1
	for x := range 64 {
		c := int(tm.Color(x, 0, 0) & 31)
		if c < last {
			t.Fatalf("pixel %d is darker than the one before it: %d < %d", x, c, last)
		}
		last = c
	}
	assert.Equal(t, true, int(tm.Color(0, 0, 0)&31) <= 1)
	assert.Equal(t, true, last >= 28)
}

// TestArchive encodes the BMPs in the Net Yaroze websites' archive that sit
// beside a TIM of the same name and compares the colors, pixel by pixel,
// with the TIM the member made from it with Sony's tools, which narrowed
// each component to 5 bits by dropping the low 3 as Encode does. Many BMPs
// were edited after their TIM was made, or were not its source at all, so
// the test counts those rather than failing on them, and fails when fewer
// agree than did when it was written. PSX_WEBARCHIVE is the
// archive's site directory.
func TestArchive(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	var pairs, same int
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".bmp") {
			return err
		}
		// The names were typed on DOS, so the TIM's may differ in case.
		stem := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(p)))
		var golden []byte
		ents, _ := os.ReadDir(filepath.Dir(p))
		for _, e := range ents {
			if strings.ToLower(e.Name()) == stem+".tim" {
				golden, _ = os.ReadFile(filepath.Join(filepath.Dir(p), e.Name()))
			}
		}
		if golden == nil {
			return nil
		}
		want, err := Decode(golden)
		if err != nil {
			return nil
		}
		f, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		img, _, err := image.Decode(bytes.NewReader(f))
		if err != nil {
			t.Logf("%s: %v", p, err)
			return nil
		}
		pairs++
		o := Options{Mode: want.Mode, X: want.Pixels.X, Y: want.Pixels.Y}
		if want.CLUT != nil {
			o.CLUTX, o.CLUTY = want.CLUT.X, want.CLUT.Y
		}
		got, err := Encode(img, o)
		if err != nil {
			t.Logf("%s: %v", p, err)
			return nil
		}
		if got.Width() < want.Width() || got.Height() != want.Height() {
			t.Logf("%s: %dx%d, want %dx%d", p, got.Width(), got.Height(), want.Width(), want.Height())
			return nil
		}
		for y := range want.Height() {
			for x := range img.Bounds().Dx() {
				if got.Color(x, y, 0)&0x7fff != want.Color(x, y, 0)&0x7fff {
					t.Logf("%s: (%d,%d) is %#04x, want %#04x", p, x, y, got.Color(x, y, 0), want.Color(x, y, 0))
					return nil
				}
			}
		}
		same++
		return nil
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Logf("%d of %d BMPs encode to their TIM's colors", same, pairs)
	// 56 is how many agree: the rest are SCEE tutorial and demo images
	// whose BMPs are not what their TIMs were made from, as the colors
	// show, and a few that were edited afterwards.
	if same < 56 {
		t.Errorf("only %d of %d BMPs encode to their TIM's colors", same, pairs)
	}
}
