package probetest

import (
	"debug/elf"
	"encoding/binary"
	"sort"
	"strings"
	"testing"
)

// noHeapLevels is the compiler's optimization levels and where make.sh
// leaves the library built at each: level 2 is the image itself, and
// OPT=N make.sh builds the others.
var noHeapLevels = []struct {
	opt, elf string
}{
	{"0", "../../../../.build/cleanimage-opt0/image.elf"},
	{"1", "../../../../.build/cleanimage-opt1/image.elf"},
	{"2", "../../../../.build/cleanimage/image.elf"},
	{"s", "../../../../.build/cleanimage-opts/image.elf"},
	{"z", "../../../../.build/cleanimage-optz/image.elf"},
}

// TestNoHeapAnyOpt checks that the library allocates nothing at any
// optimization level (spec 59): a program that links it statically is built
// at the level it asks for, and nothing allocated on this target is freed. It
// reads the library as make.sh built it at each level, the whole of it and
// not only what one program reaches, and fails for every function of the
// library that calls runtime.alloc, or that calls a function of the runtime
// or the standard library that does. A level that has not been built is
// skipped: OPT=0, 1, s and z each want a run of internal/libps/image/make.sh.
//
// Level 0 is listed and not failed. There the compiler does not look at where
// a pointer goes, so every local whose address is taken is on the heap, and it
// builds the map of operands that device.AsmFull takes at run time: the
// library cannot be free of allocations at that level, whatever its source
// (spec 59, Results).
//
// Only direct calls count (jal): a call through a register is not followed.
func TestNoHeapAnyOpt(t *testing.T) {
	for _, l := range noHeapLevels {
		t.Run("opt="+l.opt, func(t *testing.T) {
			f, err := elf.Open(l.elf)
			if err != nil {
				t.Skipf("no library at this level: run OPT=%s internal/libps/image/make.sh", l.opt)
			}
			defer f.Close()
			allocs := libraryAllocs(t, f)
			if l.opt == "0" {
				for _, a := range allocs {
					t.Logf("%s allocates: it calls %s", a.fn, a.callee)
				}
				t.Skipf("level 0 is not checked: %d calls that allocate", len(allocs))
			}
			for _, a := range allocs {
				t.Errorf("%s allocates: it calls %s", a.fn, a.callee)
			}
		})
	}
}

// libraryAlloc is a function of the library and the allocating function it
// calls: runtime.alloc itself, or one outside the library that reaches it.
type libraryAlloc struct {
	fn, callee string
}

// inLibrary reports whether the function named is the library's or the
// image's own: an exported C name, or one of a Go package that is not the
// runtime's or the standard library's.
func inLibrary(name string) bool {
	return !strings.Contains(name, ".") || strings.Contains(name, "internal/libps") || strings.HasPrefix(name, "main.")
}

// libraryAllocs lists the library's functions in f that allocate, each once
// for each allocating function it calls, in order of name.
func libraryAllocs(t *testing.T, f *elf.File) []libraryAlloc {
	syms, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	type fn struct {
		name      string
		addr, end uint32
	}
	var fns []fn
	alloc, found := uint32(0), false
	for _, s := range syms {
		if elf.ST_TYPE(s.Info) != elf.STT_FUNC {
			continue
		}
		fns = append(fns, fn{s.Name, uint32(s.Value), uint32(s.Value + s.Size)})
		if s.Name == "runtime.alloc" {
			alloc, found = uint32(s.Value), true
		}
	}
	if !found {
		// The runtime's alloc is never inlined, so it is linked in exactly
		// when something allocates.
		return nil
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].addr < fns[j].addr })
	at := func(pc uint32) (string, bool) {
		i := sort.Search(len(fns), func(i int) bool { return fns[i].addr > pc }) - 1
		if i < 0 || pc >= fns[i].end {
			return "", false
		}
		return fns[i].name, true
	}

	// calls is each function's direct calls, by the callee's name.
	calls := map[string]map[string]bool{}
	for _, s := range f.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_EXECINSTR == 0 {
			continue
		}
		code, err := s.Data()
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i+4 <= len(code); i += 4 {
			w := binary.LittleEndian.Uint32(code[i:])
			if w>>26 != 3 { // jal
				continue
			}
			pc := uint32(s.Addr) + uint32(i)
			target := (pc+4)&0xf0000000 | w&0x03ffffff<<2
			caller, ok := at(pc)
			if !ok {
				continue
			}
			callee := "runtime.alloc"
			if target != alloc {
				if callee, ok = at(target); !ok {
					continue
				}
			}
			if calls[caller] == nil {
				calls[caller] = map[string]bool{}
			}
			calls[caller][callee] = true
		}
	}

	// allocates reports whether a function outside the library reaches
	// runtime.alloc without coming back through the library.
	reaches := map[string]bool{"runtime.alloc": true}
	seen := map[string]bool{}
	var allocates func(name string) bool
	allocates = func(name string) bool {
		if reaches[name] {
			return true
		}
		if seen[name] || inLibrary(name) {
			return false
		}
		seen[name] = true
		for callee := range calls[name] {
			if allocates(callee) {
				reaches[name] = true
				return true
			}
		}
		return false
	}
	var out []libraryAlloc
	for caller, callees := range calls {
		if !inLibrary(caller) {
			continue
		}
		for callee := range callees {
			if allocates(callee) {
				out = append(out, libraryAlloc{caller, callee})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].fn != out[j].fn {
			return out[i].fn < out[j].fn
		}
		return out[i].callee < out[j].callee
	})
	return out
}
