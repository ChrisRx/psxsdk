package toolchain

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

// TestWasmOpt checks the level a build runs at against the levels the
// sysroot's cache holds: yaroze's own holds z and 2, the playground's only z,
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

// TestWasmKeepsModule builds a program twice with one Wasm under wazero,
// which keeps the compiler's module loaded between builds, and expects
// the second build to be quicker and to write the same executable. It
// needs the compiler: YAROZE_WASM naming a directory with tinygo.wasm and
// sysroot.tar.gz, as the site's play/ directory is.
func TestWasmKeepsModule(t *testing.T) {
	if os.Getenv("YAROZE_WASM") == "" {
		t.Skip("YAROZE_WASM is not set")
	}
	t.Setenv("TMPDIR", t.TempDir()) // no snapshot
	w, err := New("wazero")
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

// TestWasmSnapshot builds with one Wasm, which keeps a snapshot of the
// compiler's memory, then with another, which expects to restore it, build
// the same executable from it, more quickly, and build a package the
// snapshot never saw. It needs the compiler, as TestWasmKeepsModule does.
func TestWasmSnapshot(t *testing.T) {
	if os.Getenv("YAROZE_WASM") == "" {
		t.Skip("YAROZE_WASM is not set")
	}
	t.Setenv("TMPDIR", t.TempDir())
	ctx := context.Background()
	dir := t.TempDir()
	build := func(w *Wasm, pkg, out string) ([]byte, time.Duration) {
		var log bytes.Buffer
		start := time.Now()
		err := w.Compile(ctx, &Build{
			Package: filepath.Join("..", "..", "examples", pkg),
			Opt:     "z",
			Output:  filepath.Join(dir, out),
			Stderr:  &log,
			Log:     func(format string, args ...any) { t.Logf(format, args...) },
		})
		took := time.Since(start)
		assert.NoError(t, err, log.String())
		elf, err := os.ReadFile(filepath.Join(dir, out))
		assert.NoError(t, err)
		return elf, took
	}

	first, err := New("wazero")
	assert.NoError(t, err)
	rt := first.rt.(*wazeroRuntime)
	assert.Equal(t, true, rt.snapshot != "", "no snapshot path: %v", rt.snapshotErr)
	elf1, took1 := build(first, "square", "square1.elf")
	assert.Equal(t, false, rt.restored)
	assert.Equal(t, true, rt.saved)
	_, err = os.Stat(rt.snapshot)
	assert.NoError(t, err)
	first.Close(ctx)

	second, err := New("wazero")
	assert.NoError(t, err)
	defer second.Close(ctx)
	elf2, took2 := build(second, "square", "square2.elf")
	assert.Equal(t, true, second.rt.(*wazeroRuntime).restored)
	assert.Equal(t, elf1, elf2)
	build(second, "dino", "dino.elf")
	if took2 > took1 {
		t.Errorf("the build from the snapshot took %s, the first %s", took2, took1)
	}
	t.Logf("first build %s, from the snapshot %s", took1, took2)
}

// TestWasmtime builds a program under the wasmtime command and under
// wazero, and expects the same executable. It needs the compiler, as
// TestWasmKeepsModule does, and a wasmtime command.
func TestWasmtime(t *testing.T) {
	if os.Getenv("YAROZE_WASM") == "" {
		t.Skip("YAROZE_WASM is not set")
	}
	if _, err := exec.LookPath("wasmtime"); err != nil {
		t.Skip("there is no wasmtime command")
	}
	t.Setenv("TMPDIR", t.TempDir())
	ctx := context.Background()
	dir := t.TempDir()
	var elfs [2][]byte
	for i, runtime := range []string{"wasmtime", "wazero"} {
		w, err := New(runtime)
		assert.NoError(t, err)
		var log bytes.Buffer
		start := time.Now()
		err = w.Compile(ctx, &Build{
			Package: filepath.Join("..", "..", "examples", "square"),
			Opt:     "z",
			Output:  filepath.Join(dir, runtime+".elf"),
			Stderr:  &log,
			Log:     func(format string, args ...any) { t.Logf(format, args...) },
		})
		t.Logf("%s: built in %s", w.Runtime(), time.Since(start))
		assert.NoError(t, err, log.String())
		elfs[i], err = os.ReadFile(filepath.Join(dir, runtime+".elf"))
		assert.NoError(t, err)
		w.Close(ctx)
	}
	assert.Equal(t, elfs[0], elfs[1])
}
