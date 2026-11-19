package transform_test

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		transform.HotStubs(mod, &compileopts.Config{
			Options:         options,
			Target:          &compileopts.TargetSpec{},
			ProgramPackages: map[string]bool{"main": true},
		})
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

// TestHotKeep checks which variables the keeper holds as declared: those
// of the program's own packages, which are the ones outside GOROOT, and
// not those whose paths have a domain. A module named with one word, as
// go mod init mygame names it and as the playground's play is, has
// packages such as mygame/level, whose variables a patch of package main
// has to find as the running program has them. The standard library's are
// left to the optimizer.
func TestHotKeep(t *testing.T) {
	t.Parallel()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	buf, err := llvm.NewMemoryBufferFromFile("testdata/hotkeep.ll")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := ctx.ParseIR(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Dispose()
	config := program("mygame/level", "go.x/yaroze", "gopkg.in/yaml.v3")
	config.Options.Hot = true
	config.Options.Opt = "z"
	transform.HotStubs(mod, config)

	keeper := mod.NamedFunction("__hot_keep")
	if keeper.IsNil() {
		t.Fatal("no __hot_keep")
	}
	var got []string
	for inst := keeper.EntryBasicBlock().FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
		if inst.IsALoadInst().IsNil() {
			continue
		}
		if !inst.IsVolatile() {
			t.Errorf("the load of %s is not volatile", inst.Operand(0).Name())
		}
		got = append(got, inst.Operand(0).Name())
	}
	slices.Sort(got)
	want := []string{
		"go.x/yaroze.frame",
		"gopkg.in/yaml.v3.depth",
		"main$pack",
		"main.main$once.64@main.go:12:9",
		"main.score",
		"mygame/level.Count",
		"mygame/level.tiles",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the keeper holds:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// writeBase writes an ELF that has nothing but absolute symbols, as the
// link script of a program defines them, with the values given, and
// returns its path.
func writeBase(t *testing.T, symbols map[string]uint32) string {
	t.Helper()
	strtab := []byte{0}
	syms := []elf.Sym32{{}}
	for name, value := range symbols {
		syms = append(syms, elf.Sym32{
			Name:  uint32(len(strtab)),
			Value: value,
			Info:  elf.ST_INFO(elf.STB_GLOBAL, elf.STT_NOTYPE),
			Shndx: uint16(elf.SHN_ABS),
		})
		strtab = append(append(strtab, name...), 0)
	}
	shstrtab := []byte("\x00.symtab\x00.strtab\x00.shstrtab\x00")
	const headerSize, sectionSize, symSize = 52, 40, 16
	symOff := uint32(headerSize)
	strOff := symOff + uint32(len(syms))*symSize
	shstrOff := strOff + uint32(len(strtab))
	shOff := shstrOff + uint32(len(shstrtab))
	header := elf.Header32{
		Type:      uint16(elf.ET_EXEC),
		Machine:   uint16(elf.EM_MIPS),
		Version:   uint32(elf.EV_CURRENT),
		Shoff:     shOff,
		Ehsize:    headerSize,
		Shentsize: sectionSize,
		Shnum:     4,
		Shstrndx:  3,
	}
	copy(header.Ident[:], elf.ELFMAG)
	header.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS32)
	header.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	header.Ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	sections := []elf.Section32{
		{},
		{
			Name:    1,
			Type:    uint32(elf.SHT_SYMTAB),
			Off:     symOff,
			Size:    uint32(len(syms)) * symSize,
			Link:    2,
			Info:    1,
			Entsize: symSize,
		},
		{
			Name: 9,
			Type: uint32(elf.SHT_STRTAB),
			Off:  strOff,
			Size: uint32(len(strtab)),
		},
		{
			Name: 17,
			Type: uint32(elf.SHT_STRTAB),
			Off:  shstrOff,
			Size: uint32(len(shstrtab)),
		},
	}
	var buf bytes.Buffer
	for _, part := range []any{header, syms, strtab, shstrtab, sections} {
		if err := binary.Write(&buf, binary.LittleEndian, part); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "base.elf")
	if err := os.WriteFile(path, buf.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestHotExperiments checks that a program built with -hot says which of
// the compiler's experiments it was built with, and that HotBind refuses a
// patch built with others, or for a program that does not say.
func TestHotExperiments(t *testing.T) {
	t.Parallel()
	config := func(experiment, base string) *compileopts.Config {
		return &compileopts.Config{
			Options: &compileopts.Options{
				Experiment: experiment,
				HotBase:    base,
			},
			Target: &compileopts.TargetSpec{},
		}
	}
	for experiment, want := range map[string]string{
		"":          "__hot_experiment_onstore = 1;\n",
		"onstore":   "__hot_experiment_onstore = 1;\n",
		"noonstore": "__hot_experiment_onstore = 0;\n",
	} {
		if got := transform.HotExperiments(config(experiment, "")); got != want {
			t.Errorf("-experiment=%s: the link script is %q, want %q", experiment, got, want)
		}
	}

	const (
		both  = "hot: the patch is built with the experiments "
		older = "hot: the running program does not say which of the compiler's experiments it was built with"
	)
	tests := []struct {
		name       string
		base       map[string]uint32 // the base's symbols, besides VSync
		experiment string            // the patch's -experiment
		want       string            // what the error starts with, or "" for none
	}{
		{
			name: "both on",
			base: map[string]uint32{"__hot_experiment_onstore": 1},
		},
		{
			name:       "both off",
			base:       map[string]uint32{"__hot_experiment_onstore": 0},
			experiment: "noonstore",
		},
		{
			name:       "base on, patch off",
			base:       map[string]uint32{"__hot_experiment_onstore": 1},
			experiment: "noonstore",
			want:       both + "noonstore and the running program was built with onstore:",
		},
		{
			name: "base off, patch on",
			base: map[string]uint32{"__hot_experiment_onstore": 0},
			want: both + "onstore and the running program was built with noonstore:",
		},
		{
			name: "an experiment this compiler has not",
			base: map[string]uint32{
				"__hot_experiment_onstore": 1,
				"__hot_experiment_other":   0,
			},
			want: both + "onstore and the running program was built with onstore,noother:",
		},
		{
			name: "a base from an older compiler",
			want: older,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			symbols := map[string]uint32{"VSync": 0x80100000}
			for name, value := range test.base {
				symbols[name] = value
			}
			ctx := llvm.NewContext()
			defer ctx.Dispose()
			mod := ctx.NewModule("patch")
			defer mod.Dispose()
			mod.SetDataLayout("e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64")
			err := transform.HotBind(mod, config(test.experiment, writeBase(t, symbols)))
			switch {
			case test.want == "" && err != nil:
				t.Errorf("got the error %q, want none", err)
			case test.want != "" && err == nil:
				t.Errorf("got no error, want %q", test.want)
			case test.want != "" && !strings.HasPrefix(err.Error(), test.want):
				t.Errorf("got the error %q, want %q", err, test.want)
			}
		})
	}
}
