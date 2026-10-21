package toolchain

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

// TestWasmOpt checks the level a build runs at against the levels the
// sysroot's cache holds: nyc's own holds z and 2, the playground's only z,
// and one from before the levels were recorded says nothing.
func TestWasmOpt(t *testing.T) {
	for _, c := range []struct {
		cached []string
		asked  string
		want   string
		err    string
	}{
		{cached: []string{"z", "2"}, asked: "", want: "2"},
		{cached: []string{"z", "2"}, asked: "z", want: "z"},
		{cached: []string{"z"}, asked: "", want: "z"},
		{cached: []string{"z"}, asked: "z", want: "z"},
		{cached: []string{"z"}, asked: "2", err: "compiled only at -opt=z, so it cannot build at -opt=2"},
		{cached: []string{"z", "2"}, asked: "s", err: "compiled only at -opt=z and -opt=2"},
		{cached: nil, asked: "", want: "2"},
		{cached: nil, asked: "s", want: "s"},
	} {
		w := &Wasm{env: sysrootEnv{Opts: c.cached}}
		got, err := w.opt(&Build{Opt: c.asked, Stderr: io.Discard})
		if c.err != "" {
			assert.Error(t, c.err, err, "%v %q", c.cached, c.asked)
			continue
		}
		assert.Equal(t, c.want, assert.Ok(t, got, err), "%v %q", c.cached, c.asked)
	}
}

// TestWasmKeepsModule builds a program twice with one Wasm, which keeps
// the compiler's module loaded between builds, and expects the second
// build to be quicker and to write the same executable. It needs the
// compiler: NYC_WASM naming a directory with tinygo.wasm and
// sysroot.tar.gz, as the site's play/ directory is.
func TestWasmKeepsModule(t *testing.T) {
	if os.Getenv("NYC_WASM") == "" {
		t.Skip("NYC_WASM is not set")
	}
	w, err := NewWasm()
	assert.NoError(t, err)
	ctx := context.Background()
	defer w.Close(ctx)
	dir := t.TempDir()
	var took [2]time.Duration
	var elfs [2][]byte
	for i := range took {
		out := filepath.Join(dir, "prog.elf")
		var log bytes.Buffer
		start := time.Now()
		err := w.Compile(ctx, &Build{
			Package: filepath.Join("..", "..", "examples", "square"),
			Opt:     "z",
			Output:  out,
			Stderr:  &log,
		})
		took[i] = time.Since(start)
		assert.NoError(t, err, log.String())
		elfs[i], err = os.ReadFile(out)
		assert.NoError(t, err)
	}
	assert.Equal(t, elfs[0], elfs[1])
	if took[1] > took[0] {
		t.Errorf("the second build took %s, the first %s", took[1], took[0])
	}
	t.Logf("first build %s, second %s", took[0], took[1])
}
