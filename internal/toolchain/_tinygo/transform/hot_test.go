package transform_test

import (
	"testing"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

// TestHotCaptures checks what HotStubs records of how main's function
// literals unpack their contexts: a variable in the program, and an
// attribute in a patch, for HotBind to compare with it.
func TestHotCaptures(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"main.main$1": "captures: gep {ptr, ptr}",
		"main.main$2": "captures: load i32",
	}
	for _, patch := range []bool{false, true} {
		ctx := llvm.NewContext()
		buf, err := llvm.NewMemoryBufferFromFile("testdata/hotcaptures.ll")
		if err != nil {
			t.Fatal(err)
		}
		mod, err := ctx.ParseIR(buf)
		if err != nil {
			t.Fatal(err)
		}
		options := &compileopts.Options{Hot: !patch, Opt: "z"}
		if patch {
			options.HotBase = "base.elf"
		}
		transform.HotStubs(mod, &compileopts.Config{Options: options, Target: &compileopts.TargetSpec{}})
		for name, layout := range want {
			var got string
			if patch {
				got = mod.NamedFunction(name+"$body").GetStringAttributeAtIndex(-1, "tinygo-captures").GetStringValue()
			} else {
				g := mod.NamedGlobal(name + "$captures")
				if g.IsNil() {
					t.Fatalf("no %s$captures", name)
				}
				got = g.Initializer().ConstGetAsString()
				got = got[:len(got)-1] // its NUL
			}
			if got != layout {
				t.Errorf("patch %v: %s: got %q, want %q", patch, name, got, layout)
			}
		}
		if !mod.NamedGlobal("main.update$1$captures").IsNil() || !mod.NamedFunction("main.update$1$body").GetStringAttributeAtIndex(-1, "tinygo-captures").IsNil() {
			t.Errorf("patch %v: main.update$1 should not be recorded", patch)
		}
		mod.Dispose()
		ctx.Dispose()
	}
}
