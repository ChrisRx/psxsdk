package transform_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

func TestCheckCallbacks(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/callback.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()

	var got []string
	for _, err := range transform.CheckCallbacks(mod) {
		got = append(got, err.Error())
	}
	const alloc = ", which the vertical blank interrupt runs: the allocator is not to be entered from an interrupt, so allocate before installing it"
	want := []string{
		"main.go:14:7: this allocates in main.bad" + alloc,
		"main.go:15:23: strconv.Itoa allocates in main.bad" + alloc,
		"main.go:20:17: DrawSync waits, in main.helper, which the vertical blank interrupt runs with interrupts off, so what it waits for never comes: wait in the frame loop instead",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
