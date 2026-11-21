package ecoff

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestDebug reads testdata/debug/steps, built by GCC 2.95.3 from steps.c
// with -g and scale.c without (see its build.sh).
func TestDebug(t *testing.T) {
	f, err := Open("testdata/debug/steps")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer f.Close()
	d, err := f.Debug()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	files := map[string]*SourceFile{}
	for _, sf := range d.Files {
		files[sf.Name] = sf
	}

	steps := files["steps.c"]
	if steps == nil {
		t.Fatalf("no steps.c among %v", d.Files)
	}
	procs := map[string]*Procedure{}
	for _, p := range steps.Procedures {
		procs[p.Name] = p
	}
	// Each procedure has its symbol's address, the one the linker gave it.
	addrs := map[string]uint32{}
	for _, s := range f.ExternalSymbols {
		addrs[s.Name] = s.Value
	}
	for _, name := range []string{"tick", "main"} {
		if procs[name] == nil {
			t.Errorf("no procedure %s", name)
		} else {
			assert.Equal(t, addrs[name], procs[name].Address, name)
			assert.Equal(t, false, procs[name].Static, name)
		}
	}
	for _, name := range []string{"add", "move"} {
		if procs[name] == nil {
			t.Errorf("no procedure %s", name)
		} else {
			assert.Equal(t, true, procs[name].Static, name)
		}
	}
	// tick calls others, so it keeps a frame and saves the return address.
	if p := procs["tick"]; p != nil {
		assert.Equal(t, true, p.FrameOffset > 0)
		assert.Equal(t, uint32(1<<31), p.RegisterMask&(1<<31))
		assert.Equal(t, uint16(29), p.FrameRegister)
		assert.Equal(t, uint16(31), p.PCRegister)
	}

	// The stabs: the directory and the file, the first two.
	var so []string
	for _, s := range steps.Symbols {
		if s.IsStab() && s.Stab() == 0x64 {
			so = append(so, s.Name)
		}
	}
	assert.Equal(t, []string{"/tmp/ecoff-debug/", "steps.c"}, so)

	// scale.c was built without -g: a procedure, no stabs.
	scale := files["scale.c"]
	if scale == nil || len(scale.Procedures) != 1 {
		t.Fatalf("scale.c: want one procedure, have %+v", scale)
	}
	assert.Equal(t, "scale", scale.Procedures[0].Name)
	assert.Equal(t, addrs["scale"], scale.Procedures[0].Address)
	for _, s := range scale.Symbols {
		assert.Equal(t, false, s.IsStab(), s.Name)
	}
}

func TestDecodeLines(t *testing.T) {
	// Line 10 for two instructions, line 13 for one, then a jump back of
	// 200 lines through the two-byte escape, for sixteen.
	b := []byte{0x01, 0x30, 0x8f, 0xff, 0x38}
	assert.Equal(t, []Line{
		{Address: 0x1000, Line: 10},
		{Address: 0x1008, Line: 13},
		{Address: 0x100c, Line: -187},
	}, decodeLines(b, 0x1000, 10))
}
