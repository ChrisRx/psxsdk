package transform_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

func TestCheckFrameAllocs(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/frame.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()

	var got []string
	for _, err := range transform.CheckFrameAllocs(mod) {
		got = append(got, err.Error())
	}
	const tail = " allocates each time the frame loop runs it"
	const fix = ", and nothing is ever freed: make what it makes once, before the loop, and reuse it"
	want := []string{
		"lib.go:50:7: this" + tail + ", in the function literal in main.main" + fix,
		"main.go:12:8: this" + tail + fix,
		"main.go:13:16: strconv.Itoa" + tail + fix,
		"main.go:14:9: joining strings with +" + tail + fix,
		"main.go:15:12: lib.Load" + tail + fix,
		"main.go:21:7: this" + tail + fix,
		"main.go:23:4: this" + tail + fix,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
