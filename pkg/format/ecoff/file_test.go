package ecoff

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestEcoffParseFile(t *testing.T) {
	data, err := ioutil.ReadFile(filepath.Join("testdata", "main-ecoff"))
	if err != nil {
		t.Fatal(err)
	}

	f, err := NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", f)
}
