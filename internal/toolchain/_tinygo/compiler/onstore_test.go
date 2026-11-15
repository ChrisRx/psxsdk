package compiler

import (
	"go/types"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/interp"
	"github.com/tinygo-org/tinygo/loader"
	"tinygo.org/x/go-llvm"
)

// compileOnstore compiles testdata/file with the onstore experiment on or
// off, as package path, which is let use the pragma unless outside is set.
func compileOnstore(t *testing.T, file string, on, outside bool) (llvm.Module, []error) {
	t.Helper()
	options := &compileopts.Options{Target: "wasm"}
	target, err := compileopts.LoadTarget(options)
	if err != nil {
		t.Fatal(err)
	}
	config := &compileopts.Config{Options: options, Target: target}
	compilerConfig := &Config{
		Triple:            config.Triple(),
		Features:          config.Features(),
		ABI:               config.ABI(),
		GOOS:              config.GOOS(),
		GOARCH:            config.GOARCH(),
		CodeModel:         config.CodeModel(),
		RelocationModel:   config.RelocationModel(),
		Scheduler:         config.Scheduler(),
		DefaultStackSize:  config.StackSize(),
		NeedsStackObjects: config.NeedsStackObjects(),
		PanicUnwind:       config.PanicUnwind(),
		Onstore:           on,
	}
	machine, err := NewTargetMachine(compilerConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer machine.Dispose()
	lprogram, err := loader.Load(config, "./testdata/"+file, types.Config{Sizes: Sizes(machine)})
	if err != nil {
		t.Fatal(err)
	}
	if err := lprogram.Parse(); err != nil {
		t.Fatal(err)
	}
	program := lprogram.LoadSSA()
	pkg := lprogram.MainPkg()
	ssaPkg := program.Package(pkg.Pkg)
	ssaPkg.Build()
	if !outside {
		saved := onstorePackages
		onstorePackages = pkg.Pkg.Path()
		defer func() { onstorePackages = saved }()
	}
	return CompilePackage(file, pkg, ssaPkg, machine, compilerConfig, false)
}

// packCalls counts the calls of a pack method in the function named.
func packCalls(t *testing.T, mod llvm.Module, name string) int {
	t.Helper()
	fn := mod.NamedFunction("main." + name)
	if fn.IsNil() {
		t.Fatalf("no function main.%s", name)
	}
	n := 0
	for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
		for in := bb.FirstInstruction(); !in.IsNil(); in = llvm.NextInstruction(in) {
			if in.IsACallInst().IsNil() {
				continue
			}
			if callee := in.CalledValue(); !callee.IsAFunction().IsNil() && strings.HasSuffix(callee.Name(), ".pack") {
				n++
			}
		}
	}
	return n
}

func TestOnstore(t *testing.T) {
	mod, errs := compileOnstore(t, "onstore.go", true, false)
	for _, err := range errs {
		t.Error(err)
	}
	if len(errs) > 0 {
		return
	}
	defer mod.Dispose()
	if err := llvm.VerifyModule(mod, llvm.PrintMessageAction); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fn    string
		calls int
	}{
		{"storeOne", 1},
		{"storeTwo", 1},
		{"storeSplit", 3},
		{"storeIndex", 1},
		{"storeEmbedded", 1},
		{"storeNested", 1},
		{"storeUnwatched", 1},
		{"storeCopy", 0},
		{"read", 0},
	} {
		if got := packCalls(t, mod, tc.fn); got != tc.calls {
			t.Errorf("%s calls pack %d times, want %d", tc.fn, got, tc.calls)
		}
	}
	if fn := mod.NamedFunction("(*main.S).pack"); fn.IsNil() || fn.GetEnumFunctionAttribute(llvm.AttributeKindID("alwaysinline")).IsNil() {
		t.Error("pack is not marked alwaysinline")
	}

	// The package's initializer is run at compile time, as the build does,
	// and ship's word is in its initial value.
	if err := interp.RunFunc(mod.NamedFunction("main.init"), 10*time.Second, 1000, false); err != nil {
		t.Fatal(err)
	}
	ship := mod.NamedGlobal("main.ship")
	if ship.IsNil() {
		t.Fatal("no global main.ship")
	}
	init := ship.Initializer()
	if init.IsNil() || init.Type().TypeKind() != llvm.StructTypeKind {
		t.Fatalf("ship has no struct initializer: %s", ship)
	}
	const want = 5<<28 | 2<<24 | 1<<6
	if word := init.Operand(0); word.IsAConstantInt().IsNil() || word.ZExtValue() != want {
		t.Errorf("ship's word is %s, want %#x", word, want)
	}
}

func TestOnstoreOff(t *testing.T) {
	mod, errs := compileOnstore(t, "onstore.go", false, false)
	for _, err := range errs {
		t.Error(err)
	}
	if len(errs) > 0 {
		return
	}
	defer mod.Dispose()
	for _, fn := range []string{"storeOne", "storeTwo", "storeSplit", "storeIndex", "storeEmbedded", "storeNested", "storeUnwatched"} {
		if got := packCalls(t, mod, fn); got != 0 {
			t.Errorf("%s calls pack %d times with the experiment off", fn, got)
		}
	}
}

func TestOnstoreErrors(t *testing.T) {
	_, errs := compileOnstore(t, "onstore-errors.go", true, false)
	var got []string
	for _, err := range errs {
		msg := err.Error()
		if _, after, ok := strings.Cut(msg, "onstore-errors.go:"); ok {
			msg = after
		}
		got = append(got, msg)
	}
	slices.Sort(got)
	want := []string{
		"24:5: the address of S.Blend, or of something in it, is used for more than a store or a load, and a store through it would not call pack, which //go:onstore runs after each store to the field: store to the field directly, or build with -experiment=noonstore",
		"29:15: the address of S.Tex, or of something in it, is used for more than a store or a load, and a store through it would not call pack, which //go:onstore runs after each store to the field: store to the field directly, or build with -experiment=noonstore",
		"36:2: the address of S.Tex, or of something in it, is used for more than a store or a load, and a store through it would not call pack, which //go:onstore runs after each store to the field: store to the field directly, or build with -experiment=noonstore",
		"48:15: //go:onstore names y, which is not a field of the receiver's type",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestOnstoreOutside(t *testing.T) {
	_, errs := compileOnstore(t, "onstore.go", true, true)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "//go:onstore is an experiment only packages under go.chrisrx.dev/psx may use") {
		t.Errorf("got %v, want the pragma refused outside go.chrisrx.dev/psx", errs)
	}
}
