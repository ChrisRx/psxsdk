// +build ignore

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

func main() {
	data, err := ioutil.ReadFile("files/libps.exe")
	if err != nil {
		log.Fatal(err)
	}
	var in bytes.Buffer
	w := gzip.NewWriter(&in)
	if _, err := w.Write(data); err != nil {
		log.Fatal(err)
	}
	w.Close()

	var sb strings.Builder
	for _, b := range in.Bytes() {
		sb.WriteString(fmt.Sprintf("\\x%02x", b))
	}
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		log.Fatal(err)
	}
	var out bytes.Buffer
	if err := t.Execute(&out, map[string]string{
		"PackageName": "yaroze",
		"Data":        sb.String(),
	}); err != nil {
		log.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("template directory not found")
	}
	if err := ioutil.WriteFile(filepath.Join(filepath.Dir(filename), "zz_generated.files.go"), out.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

const tmpl = `// Code generated DO NOT EDIT
package {{ .PackageName }}

import (
	"bytes"
	"compress/gzip"

	"github.com/ChrisRx/psxsdk/pkg/format/psx"
)

var Libps *psx.File

func init() {
	var err error
	r, err := gzip.NewReader(bytes.NewReader(libps))
	if err != nil {
		panic(err)
	}
	defer r.Close()
	
	Libps, err = psx.ParseFile(r)
	if err != nil {
		panic(err)
	}

}

var libps = []byte("{{ .Data }}")`
