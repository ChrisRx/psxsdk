package toolchain

import (
	"io"
	"testing"

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
