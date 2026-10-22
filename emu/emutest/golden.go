package emutest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/openbios"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite golden images from what the tests produce")

// DefaultBIOS is the name of the BIOS the tests ship with, whose golden
// images are the ones in testdata itself.
const DefaultBIOS = "openbios"

// BIOSName names a BIOS image, which is the directory under testdata its
// golden images are kept in: openbios for the image the tests ship with;
// for one of Sony's, sony- and the version and region its ROM states, as
// "sony-3.0a" for "System ROM Version 3.0 11/18/96 A"; and for anything
// else, bios- and the first eight hex digits of its SHA-256, so that an
// image with no name of its own still has one that stays the same.
func BIOSName(image []byte) string {
	if bytes.Equal(image, openbios.Image) {
		return DefaultBIOS
	}
	// Sony's ROMs state their version near the end, NUL terminated.
	const off = 0x7ff32
	if len(image) > off+48 {
		s := image[off : off+48]
		if i := bytes.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		if rest, ok := strings.CutPrefix(string(s), "System ROM Version "); ok {
			if f := strings.Fields(rest); len(f) >= 3 {
				return "sony-" + strings.ToLower(f[0]+f[2])
			}
		}
	}
	sum := sha256.Sum256(image)
	return "bios-" + hex.EncodeToString(sum[:4])
}

// Golden compares the display with the golden image called name, as the
// package's Golden does, for the BIOS the machine booted: testdata/name
// for the default BIOS, and for another, testdata/BIOS/name where that
// BIOS has an image of its own, else testdata/name.
//
// What a screen shows can depend on the BIOS, in the count of a frame's
// syncs it reports, say, so a BIOS other than the default has a directory
// of its own, named by BIOSName, holding the images where it differs from
// the default. Run with -update-golden under that BIOS to record them: an
// image the same as the default's is not written, and one that was is
// removed, so the directory stays a list of the differences.
func (m *Machine) Golden(name string) {
	m.tb.Helper()
	golden(m.tb, m.Display(), m.bios, name)
}

// Golden compares an image with the golden one at testdata/name, which must
// be a PNG, failing the test if any pixel differs. With -update-golden it
// writes the image there instead.
//
// On a mismatch the image the test produced is written beside the golden
// one, as name with .actual.png in place of .png, to compare by eye.
func Golden(tb testing.TB, img image.Image, name string) {
	tb.Helper()
	golden(tb, img, DefaultBIOS, name)
}

// golden is Golden for the named BIOS, with the default's image standing in
// for one the BIOS has not got.
func golden(tb testing.TB, img image.Image, bios, name string) {
	tb.Helper()
	def := filepath.Join("testdata", name)
	path := def
	if bios != "" && bios != DefaultBIOS {
		path = filepath.Join("testdata", bios, name)
	}

	if *updateGolden {
		if path != def {
			if want, err := readPNG(def); err == nil && same(img, want) {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					tb.Fatal(err)
				}
				return
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := writePNG(path, img); err != nil {
			tb.Fatal(err)
		}
		return
	}

	want, err := readPNG(path)
	fellBack := false
	if os.IsNotExist(err) && path != def {
		want, err = readPNG(def)
		fellBack = true
	}
	if os.IsNotExist(err) {
		tb.Fatalf("no golden image %s; run with -update-golden to create it", path)
	}
	if err != nil {
		tb.Fatal(err)
	}

	got, wanted := rgba(img), rgba(want)
	if got.Rect != wanted.Rect {
		tb.Fatalf("image is %v, golden image %s is %v", got.Rect, path, wanted.Rect)
	}
	if bytes.Equal(got.Pix, wanted.Pix) {
		return
	}

	n := 0
	first := image.Point{-1, -1}
	for y := got.Rect.Min.Y; y < got.Rect.Max.Y; y++ {
		for x := got.Rect.Min.X; x < got.Rect.Max.X; x++ {
			if got.RGBAAt(x, y) != wanted.RGBAAt(x, y) {
				if n == 0 {
					first = image.Pt(x, y)
				}
				n++
			}
		}
	}
	actual := path[:len(path)-len(filepath.Ext(path))] + ".actual.png"
	if err := os.MkdirAll(filepath.Dir(actual), 0o755); err != nil {
		tb.Logf("making %s: %v", filepath.Dir(actual), err)
	} else if err := writePNG(actual, img); err != nil {
		tb.Logf("writing %s: %v", actual, err)
	}
	compared := def
	note := ""
	if fellBack {
		note = fmt.Sprintf(", which stands in for %s, since the %s BIOS has no image of its own; -update-golden under it records one", path, bios)
	} else {
		compared = path
	}
	tb.Fatalf("%d pixels differ from %s%s, the first at %v; the image is in %s",
		n, compared, note, first, actual)
}

// same reports whether two images are the same size and pixels.
func same(a, b image.Image) bool {
	ra, rb := rgba(a), rgba(b)
	return ra.Rect == rb.Rect && bytes.Equal(ra.Pix, rb.Pix)
}

func rgba(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	r := image.NewRGBA(img.Bounds())
	draw.Draw(r, r.Rect, img, img.Bounds().Min, draw.Src)
	return r
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
