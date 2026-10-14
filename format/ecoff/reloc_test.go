package ecoff

import "testing"

// TestRelocations pins down the relocation table decoding.
//
// The testdata objects are MIPSEL-BE: big-endian headers wrapping
// little-endian MIPS code, which is what Sony's own compiler produced. Their
// packed relocation bitfields do not use the big-endian layout in binutils
// include/coff/mips.h, and decoding them that way halves every relocation
// type rather than failing, so a regression here would be quiet.
func TestRelocations(t *testing.T) {
	tests := []struct {
		file    string
		section string
		want    []struct {
			addr     uint32
			typ      RelocationType
			target   string
			external bool
		}
	}{{
		// puts() loads the address of its "<NULL>" string out of .rdata with
		// a REFHI/REFLO pair, branches internally, and calls putchar.
		file:    "puts.o",
		section: S_TEXT,
		want: []struct {
			addr     uint32
			typ      RelocationType
			target   string
			external bool
		}{
			{0x14, R_REFHI, S_RDATA, false},
			{0x1c, R_REFLO, S_RDATA, false},
			{0x18, R_JMPADDR, S_TEXT, false},
			{0x20, R_JMPADDR, "putchar", true},
		},
	}, {
		file:    "video.o",
		section: S_TEXT,
		want: []struct {
			addr     uint32
			typ      RelocationType
			target   string
			external bool
		}{
			{0x1c, R_JMPADDR, "SsSetTickMode", true},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			f, err := Open("testdata/" + tt.file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			var got []*Relocation
			for _, s := range f.Sections {
				if s.SectionName() == tt.section {
					got = s.Relocations
					break
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d relocations in %s, want %d", len(got), tt.section, len(tt.want))
			}
			for i, w := range tt.want {
				r := got[i]
				if r.Address != w.addr {
					t.Errorf("reloc %d: address 0x%X, want 0x%X", i, r.Address, w.addr)
				}
				if r.Type != w.typ {
					t.Errorf("reloc %d at 0x%X: type %v, want %v", i, w.addr, r.Type, w.typ)
				}
				if r.External != w.external {
					t.Errorf("reloc %d at 0x%X: external %v, want %v", i, w.addr, r.External, w.external)
				}
				if target := f.Target(r); target != w.target {
					t.Errorf("reloc %d at 0x%X: target %q, want %q", i, w.addr, target, w.target)
				}
			}
		})
	}
}

// TestTargetByteOrder checks that header byte order and target byte order are
// kept apart. Reading section contents with the header's byte order would
// byte-swap every instruction in the objects Sony shipped.
func TestTargetByteOrder(t *testing.T) {
	f, err := Open("testdata/puts.o")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := f.ByteOrder().String(); got != "BigEndian" {
		t.Errorf("header byte order is %s, want BigEndian", got)
	}
	if got := f.TargetByteOrder().String(); got != "LittleEndian" {
		t.Errorf("target byte order is %s, want LittleEndian", got)
	}

	// puts+0x14 is "lui $s0, ..." which encodes as 0x3C10xxxx. Read with the
	// header's big-endian order it would come out as 0x0000103C.
	var text *Section
	for _, s := range f.Sections {
		if s.SectionName() == S_TEXT {
			text = s
			break
		}
	}
	if text == nil {
		t.Fatal("no .text section")
	}
	data, err := text.Data()
	if err != nil {
		t.Fatal(err)
	}
	if got := f.TargetByteOrder().Uint32(data[0x14:]); got>>26 != 0x0f {
		t.Errorf("word at .text+0x14 is 0x%08X, want opcode 0x0F (lui)", got)
	}
}
