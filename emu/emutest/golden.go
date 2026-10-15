package emutest

import (
	"bytes"
	"flag"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite golden images from what the tests produce")

// Golden compares an image with the golden one at testdata/name, which must
// be a PNG, failing the test if any pixel differs. With -update-golden it
// writes the image there instead.
//
// On a mismatch the image the test produced is written beside the golden
// one, as name with .actual.png in place of .png, to compare by eye.
func Golden(tb testing.TB, img image.Image, name string) {
	tb.Helper()
	path := filepath.Join("testdata", name)

	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := writePNG(path, img); err != nil {
			tb.Fatal(err)
		}
		return
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		tb.Fatalf("no golden image %s; run with -update-golden to create it", path)
	}
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		tb.Fatalf("%s: %v", path, err)
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
	if err := writePNG(actual, img); err != nil {
		tb.Logf("writing %s: %v", actual, err)
	}
	tb.Fatalf("%d pixels differ from %s, the first at %v; the image is in %s",
		n, path, first, actual)
}

func rgba(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	r := image.NewRGBA(img.Bounds())
	draw.Draw(r, r.Rect, img, img.Bounds().Min, draw.Src)
	return r
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
