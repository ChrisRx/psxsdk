package ecoff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEcoffParseFile(t *testing.T) {
	data, err := os.ReadFile(sonyFixture(t, "main-ecoff"))
	if err != nil {
		t.Fatal(err)
	}

	f, err := NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", f)
}

// sonyFixture is the path of a file built from Sony's SDK, which is not in this
// repository: PSX_SONY_TESTDATA names a private copy, and the test skips without it.
func sonyFixture(t *testing.T, name string) string {
	t.Helper()
	dir := os.Getenv("PSX_SONY_TESTDATA")
	if dir == "" {
		t.Skip("PSX_SONY_TESTDATA is not set")
	}
	return filepath.Join(dir, name)
}
