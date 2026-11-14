package transform_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

func TestCheckKeeps(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/keeps.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()

	config := &compileopts.Config{Options: &compileopts.Options{}, Target: &compileopts.TargetSpec{GC: "leaking"}}
	var got []string
	for _, err := range transform.CheckKeeps(mod, config) {
		// No debug information: a position only where tinygo.pos gives one.
		got = append(got, strings.TrimPrefix(err.Error(), "-: "))
	}
	slices.Sort(got)
	want := []string{
		"GsSetWorkBase keeps outpacketp, but the variable passed is on the stack of onStack, which returns: make it a package-level variable",
		"GsSetWorkBase keeps outpacketp, so the variable passed is allocated each time frame runs here, and never freed: make it a package-level variable",
		"GsSetWorkBase keeps outpacketp, so the variable passed is allocated each time helper runs here, and never freed: make it a package-level variable",
		"GsSetWorkBase keeps outpacketp, so the variable passed is allocated each time main.main runs here, and never freed: make it a package-level variable",
		"GsSetWorkBase, called through wrapBase, keeps outpacketp, so the variable passed is allocated each time frame2 runs here, and never freed: make it a package-level variable",
		"LoadImage goes on with p until DrawSync, but the variable passed is on the stack of syncedOnOnePath, which can return first: call DrawSync before it does, or make the variable a package-level one",
		"LoadImage goes on with p until DrawSync, but the variable passed is on the stack of unsynced, which can return first: call DrawSync before it does, or make the variable a package-level one",
		"main.go:12:3: LoadImage, called through wrapLoad, goes on with p until DrawSync, but the variable passed is on the stack of throughWrapper, which can return first: call DrawSync before it does, or make the variable a package-level one",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
