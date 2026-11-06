package psx

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestEXEParseFile(t *testing.T) {
	gz, err := os.Open("../../link/libps.exe.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	r, err := gzip.NewReader(gz)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	f, err := ParseFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", f)
}
