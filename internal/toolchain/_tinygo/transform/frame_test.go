package transform_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

// program is a configuration for a program with the packages named, those
// the loader would find outside GOROOT, besides package main.
func program(packages ...string) *compileopts.Config {
	config := &compileopts.Config{
		Options:         &compileopts.Options{},
		Target:          &compileopts.TargetSpec{},
		ProgramPackages: map[string]bool{"main": true},
	}
	for _, pkg := range packages {
		config.ProgramPackages[pkg] = true
	}
	return config
}

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
	for _, err := range transform.CheckFrameAllocs(mod, program("go.x/yaroze", "go.x/lib")) {
		got = append(got, err.Error())
	}
	const tail = " allocates each time the frame loop runs it"
	const fix = ", and nothing is ever freed: make what it makes once, before the loop, and reuse it"
	want := []string{
		"lib.go:50:7: this" + tail + ", in the function literal in main.main" + fix,
		"main.go:12:8: this" + tail + fix,
		"main.go:13:16: strconv.Itoa" + tail + fix,
		"main.go:14:9: joining strings with +" + tail + fix,
		"main.go:15:12: lib.Load" + tail + ", at lib.go:30" + fix,
		"main.go:21:7: this" + tail + fix,
		"main.go:23:4: this" + tail + fix,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestCheckFrameAllocsHot checks that a hot build's slots, through which
// each function of package main is called, do not make main's own body a
// function value that a callback in the library's VSync might call.
func TestCheckFrameAllocsHot(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/framehot.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()
	for _, err := range transform.CheckFrameAllocs(mod, program()) {
		t.Error(err)
	}
}

// TestCheckFrameAllocsPanic checks that what the runtime allocates to print
// a panic's value, before it stops the program, is not the frame loop's,
// in a program that keeps an error whose message allocates; and that what
// the program's own code allocates on the way to a panic still is, whether
// or not the program recovers, since one that does could run it every
// frame.
func TestCheckFrameAllocsPanic(t *testing.T) {
	t.Parallel()
	const tail = " allocates each time the frame loop runs it"
	const fix = ", and nothing is ever freed: make what it makes once, before the loop, and reuse it"
	want := []string{
		"main.go:26:10: this" + tail + fix,
		"main.go:28:12: lib.Check" + tail + ", at lib.go:20" + fix,
	}
	for _, test := range []struct {
		name     string
		recovers bool
	}{
		{
			name: "stops",
		},
		{
			name:     "recovers",
			recovers: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := llvm.NewContext()
			defer ctx.Dispose()
			buf, err := llvm.NewMemoryBufferFromFile("testdata/framepanic.ll")
			if err != nil {
				t.Fatal(err)
			}
			mod, err := ctx.ParseIR(buf)
			if err != nil {
				t.Fatal(err)
			}
			defer mod.Dispose()
			if test.recovers {
				// The function a call of recover becomes.
				llvm.AddFunction(mod, "runtime._recover", llvm.FunctionType(ctx.VoidType(), nil, false))
			}
			var got []string
			for _, err := range transform.CheckFrameAllocs(mod, program("go.x/input", "go.x/lib")) {
				got = append(got, err.Error())
			}
			if !slices.Equal(got, want) {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// TestCheckFrameAllocsModule checks that the packages of a module whose
// path has no domain, as the playground's module play has, are the
// program's own, though the path looks like one of the standard library's:
// an allocation in one is reported where package main calls it with the
// line it is at, and one that is made once and kept is not reported. A
// package of the standard library is still one, slash or no slash.
func TestCheckFrameAllocsModule(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/framemodule.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()

	var got []string
	for _, err := range transform.CheckFrameAllocs(mod, program("play/level")) {
		got = append(got, err.Error())
	}
	const tail = " allocates each time the frame loop runs it"
	const fix = ", and nothing is ever freed: make what it makes once, before the loop, and reuse it"
	want := []string{
		"main.go:11:19: level.Make" + tail + ", at level.go:14" + fix,
		"main.go:12:28: hex.EncodeToString" + tail + fix,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
