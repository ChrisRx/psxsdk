package ecoffelf

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/ecoff"
)

const (
	fixture     = "../../format/ecoff/testdata/debug/steps"
	varsFixture = "../../format/ecoff/testdata/debug/vars"
)

func convert(t *testing.T) (*ecoff.File, *elf.File, []byte) {
	return convertPath(t, fixture)
}

func convertPath(t *testing.T, path string) (*ecoff.File, *elf.File, []byte) {
	t.Helper()
	f, err := ecoff.Open(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { f.Close() })
	data, err := Convert(f)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	e, err := elf.NewFile(bytes.NewReader(data))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return f, e, data
}

// TestSections checks the ELF holds the program as the ECOFF does: each
// section at its address with the same contents, and the same entry point.
func TestSections(t *testing.T) {
	f, e, _ := convert(t)
	assert.Equal(t, uint64(f.Entry), e.Entry)
	assert.Equal(t, elf.EM_MIPS, e.Machine)
	for _, s := range f.Sections {
		es := e.Section(s.SectionName())
		if es == nil {
			t.Errorf("no section %s", s.SectionName())
			continue
		}
		assert.Equal(t, uint64(s.VirtualAddress), es.Addr, s.SectionName())
		if s.Size == 0 {
			continue
		}
		want, _ := s.Data()
		got, err := es.Data()
		assert.NoError(t, err)
		assert.Equal(t, want, got, s.SectionName())
	}
}

// TestSymbols checks functions get their procedures' sizes, statics stay
// local, and one name at the entry point is a function.
func TestSymbols(t *testing.T) {
	_, e, _ := convert(t)
	syms, err := e.Symbols()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	byName := map[string]elf.Symbol{}
	for _, s := range syms {
		byName[s.Name] = s
	}
	for name, bind := range map[string]elf.SymBind{
		"tick": elf.STB_GLOBAL, "main": elf.STB_GLOBAL, "_start": elf.STB_GLOBAL, "scale": elf.STB_GLOBAL,
		"add": elf.STB_LOCAL, "move": elf.STB_LOCAL,
	} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("no symbol %s", name)
			continue
		}
		assert.Equal(t, elf.STT_FUNC, elf.ST_TYPE(s.Info), name)
		assert.Equal(t, bind, elf.ST_BIND(s.Info), name)
		assert.Equal(t, true, s.Size > 0 && s.Size%4 == 0, name)
	}
	// _start is at the text's start, with the linker's _ftext: only _start
	// is a function there.
	for _, name := range []string{"_ftext"} {
		assert.Equal(t, elf.STT_NOTYPE, elf.ST_TYPE(byName[name].Info), name)
	}
	assert.Equal(t, elf.STT_OBJECT, elf.ST_TYPE(byName["ball"].Info))
	assert.Equal(t, elf.STB_LOCAL, elf.ST_BIND(byName["ball"].Info))
}

// TestLines checks the line table: each line of tick is at an address in
// tick, in the compile unit of steps.c, in the directory it was built in.
func TestLines(t *testing.T) {
	_, e, _ := convert(t)
	d, err := e.DWARF()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	syms, _ := e.Symbols()
	var tick elf.Symbol
	for _, s := range syms {
		if s.Name == "tick" {
			tick = s
		}
	}
	r := d.Reader()
	lines := map[int]uint64{}
	for {
		ent, err := r.Next()
		if !assert.NoError(t, err) || ent == nil {
			break
		}
		if ent.Tag != dwarf.TagCompileUnit {
			continue
		}
		if ent.Val(dwarf.AttrName) != "steps.c" {
			continue
		}
		assert.Equal(t, "/tmp/ecoff-debug", ent.Val(dwarf.AttrCompDir))
		lr, err := d.LineReader(ent)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var le dwarf.LineEntry
		for lr.Next(&le) == nil {
			if le.Address >= tick.Value && le.Address < tick.Value+tick.Size {
				if _, ok := lines[le.Line]; !ok {
					lines[le.Line] = le.Address
				}
				assert.Equal(t, true, strings.HasSuffix(le.File.Name, "steps.c"), le.File.Name)
			}
		}
	}
	// tick's body, steps.c lines 35 to 38.
	for _, l := range []int{35, 36, 37, 38} {
		_, ok := lines[l]
		assert.Equal(t, true, ok, "line", l)
	}
}

// TestVerify has llvm-dwarfdump check the DWARF, when it is on the path, and
// checks tick's frame is described.
func TestVerify(t *testing.T) {
	path, err := exec.LookPath("llvm-dwarfdump")
	if err != nil {
		t.Skip("no llvm-dwarfdump on the path")
	}
	_, _, data := convert(t)
	out := filepath.Join(t.TempDir(), "steps.elf")
	assert.NoError(t, os.WriteFile(out, data, 0o644))
	got, err := exec.Command(path, "--verify", out).CombinedOutput()
	assert.NoError(t, err, string(got))
	assert.Equal(t, true, strings.Contains(string(got), "No errors."), string(got))
	frames, err := exec.Command(path, "--debug-frame", out).CombinedOutput()
	assert.NoError(t, err, string(frames))
	assert.Equal(t, true, strings.Contains(string(frames), "DW_CFA_def_cfa_offset"), string(frames))
	assert.Equal(t, true, strings.Contains(string(frames), "DW_CFA_offset: RA"), string(frames))
}

// TestTypes reads the types and variables of testdata/debug/vars, built at
// -O0, back from the DWARF.
func TestTypes(t *testing.T) {
	_, e, _ := convertPath(t, varsFixture)
	d, err := e.DWARF()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	r := d.Reader()
	structs := map[string]*dwarf.StructType{}
	vars := map[string]dwarf.Type{}
	locs := map[string][]byte{}
	for {
		ent, err := r.Next()
		if !assert.NoError(t, err) || ent == nil {
			break
		}
		name, _ := ent.Val(dwarf.AttrName).(string)
		switch ent.Tag {
		case dwarf.TagStructType, dwarf.TagUnionType:
			typ, err := d.Type(ent.Offset)
			if assert.NoError(t, err) {
				structs[name] = typ.(*dwarf.StructType)
			}
		case dwarf.TagVariable, dwarf.TagFormalParameter:
			if off, ok := ent.Val(dwarf.AttrType).(dwarf.Offset); ok {
				typ, err := d.Type(off)
				if assert.NoError(t, err) {
					vars[name] = typ
				}
			}
			if loc, ok := ent.Val(dwarf.AttrLocation).([]byte); ok {
				locs[name] = loc
			}
		}
	}

	shape := structs["shape"]
	if shape == nil {
		t.Fatalf("no struct shape")
	}
	assert.Equal(t, int64(64), shape.ByteSize)
	offsets := map[string]int64{}
	for _, f := range shape.Field {
		offsets[f.Name] = f.ByteOffset
	}
	assert.Equal(t, map[string]int64{
		"pts": 0, "color": 12, "name": 16, "scale": 20, "weight": 24, "big": 32,
		"ubig": 40, "fl": 48, "w": 56, "fn": 60,
	}, offsets)

	// The bit fields: a is three bits at 0, b five at 3.
	flags := structs["flags"]
	if flags == nil {
		t.Fatalf("no struct flags")
	}
	bits := map[string][2]int64{}
	for _, f := range flags.Field {
		bits[f.Name] = [2]int64{f.DataBitOffset, f.BitSize}
	}
	assert.Equal(t, [2]int64{0, 3}, bits["a"])
	assert.Equal(t, [2]int64{3, 5}, bits["b"])
	assert.Equal(t, "union", structs["word"].Kind)

	assert.Equal(t, "*struct node", vars["n"].String())
	assert.Equal(t, "[3]struct node", vars["nodes"].String())
	assert.Equal(t, "char", vars["tag"].String())

	// Stack variables are at offsets from the frame base, the CFA: walk's
	// parameters in its caller's home area, its locals below.
	assert.Equal(t, []byte{opFbreg, 0}, locs["n"])
	assert.Equal(t, []byte{opFbreg, 4}, locs["limit"])
	assert.Equal(t, []byte{opFbreg, 0x68}, locs["sum"]) // -24
}

// TestPrologueEnd checks where a debugger breaks on a function: at its
// first line after the prologue, and at -O0, where GCC 2.95.3 labels the
// opening brace before storing the arguments, after those stores, so the
// arguments read right.
func TestPrologueEnd(t *testing.T) {
	for _, c := range []struct {
		path, file, fn string
		line           int
	}{
		{fixture, "steps.c", "tick", 35},    // -O2
		{varsFixture, "vars.c", "walk", 55}, // -O0
	} {
		_, e, _ := convertPath(t, c.path)
		d, err := e.DWARF()
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		syms, _ := e.Symbols()
		var fn elf.Symbol
		for _, s := range syms {
			if s.Name == c.fn {
				fn = s
			}
		}
		end := -1
		r := d.Reader()
		for {
			ent, err := r.Next()
			if !assert.NoError(t, err) || ent == nil {
				break
			}
			if ent.Tag != dwarf.TagCompileUnit || ent.Val(dwarf.AttrName) != c.file {
				continue
			}
			lr, err := d.LineReader(ent)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			var le dwarf.LineEntry
			for lr.Next(&le) == nil {
				if le.PrologueEnd && le.Address >= fn.Value && le.Address < fn.Value+fn.Size {
					end = le.Line
				}
			}
		}
		assert.Equal(t, c.line, end, c.fn)
	}
}
