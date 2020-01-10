package psx

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestEXEParseFile(t *testing.T) {
	data, err := ioutil.ReadFile(filepath.Join("testdata", "psx.exe"))
	if err != nil {
		t.Fatal(err)
	}

	f, err := ParseFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", f)
}
