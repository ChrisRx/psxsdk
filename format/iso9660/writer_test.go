package iso9660

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// memWriter collects the emitted blocks so a test can read the image back.
type memWriter struct {
	blocks [][]byte
	raw    map[uint32]bool
	eof    map[uint32]bool
}

func (m *memWriter) WriteBlock(b Block) error {
	if int(b.LBA) != len(m.blocks) {
		return io.ErrUnexpectedEOF
	}
	m.blocks = append(m.blocks, bytes.Clone(b.Data))
	if b.Raw {
		if m.raw == nil {
			m.raw = map[uint32]bool{}
		}
		m.raw[b.LBA] = true
	}
	if b.EndOfFile {
		if m.eof == nil {
			m.eof = map[uint32]bool{}
		}
		m.eof[b.LBA] = true
	}
	return nil
}

func entry(name, content string) *Entry {
	return &Entry{
		Name: FileIdentifier(name),
		Size: uint32(len(content)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
}

func build(t *testing.T, root *Dir) (*Writer, *memWriter) {
	t.Helper()
	w, err := NewWriter(root, Options{VolumeID: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}
	if got := uint32(len(m.blocks)); got != w.TotalBlocks() {
		t.Fatalf("emitted %d blocks, laid out %d", got, w.TotalBlocks())
	}
	return w, m
}

func TestFileIdentifier(t *testing.T) {
	tests := []struct{ in, want string }{
		{"main.exe", "MAIN.EXE;1"},
		{"MAIN.EXE", "MAIN.EXE;1"},
		{"sample1.seq", "SAMPLE1.SEQ;1"},
		{"read me.txt", "READ_ME.TXT;1"},
		{"noext", "NOEXT;1"},
		{"a-b.c+d", "A_B.C_D;1"},
	}
	for _, tt := range tests {
		if got := FileIdentifier(tt.in); got != tt.want {
			t.Errorf("FileIdentifier(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPVD(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"))
	w, m := build(t, root)

	pvd := m.blocks[PVDSector]
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" || pvd[6] != 1 {
		t.Fatalf("not a primary volume descriptor: %x", pvd[:8])
	}
	if got := strings.TrimRight(string(pvd[40:72]), " "); got != "TEST" {
		t.Errorf("volume id = %q, want TEST", got)
	}
	if got := strings.TrimRight(string(pvd[8:40]), " "); got != "PLAYSTATION" {
		t.Errorf("system id = %q, want PLAYSTATION", got)
	}

	// Both-endian fields must agree, which is the easiest thing to get
	// wrong and the easiest to check.
	for _, f := range []struct {
		name string
		off  int
	}{
		{"volume space size", 80},
		{"path table size", 132},
	} {
		le := binary.LittleEndian.Uint32(pvd[f.off:])
		be := binary.BigEndian.Uint32(pvd[f.off+4:])
		if le != be {
			t.Errorf("%s: little-endian %d but big-endian %d", f.name, le, be)
		}
	}
	if got := binary.LittleEndian.Uint32(pvd[80:]); got != w.TotalBlocks() {
		t.Errorf("volume space size = %d, want %d", got, w.TotalBlocks())
	}
	if got := binary.LittleEndian.Uint16(pvd[128:]); got != LogicalBlockSize {
		t.Errorf("logical block size = %d, want %d", got, LogicalBlockSize)
	}

	// The descriptor set terminator follows immediately.
	term := m.blocks[PVDSector+1]
	if term[0] != 255 || string(term[1:6]) != "CD001" {
		t.Errorf("missing volume descriptor set terminator: %x", term[:8])
	}
}

// Directory records may not straddle a block boundary. A directory holding
// enough entries to fill more than one block is the case where a size
// computed without accounting for that padding goes wrong, and the records
// then overrun into the next structure.
func TestDirectoryRecordsRespectBlockBoundaries(t *testing.T) {
	root := &Dir{}
	for i := range 200 {
		root.Entries = append(root.Entries, entry(
			strings.Repeat("F", 6)+string(rune('A'+i%26))+string(rune('A'+i/26))+".DAT", ""))
	}
	w, m := build(t, root)

	if w.root.Size <= LogicalBlockSize {
		t.Fatalf("test needs a multi-block directory, got %d bytes", w.root.Size)
	}

	// Walk the records exactly as a reader would: a zero length byte means
	// the rest of this block is padding, so advance to the next one.
	var seen int
	for off := uint32(0); off < w.root.Size; {
		b := m.blocks[w.root.LBA+off/LogicalBlockSize][off%LogicalBlockSize:]
		if b[0] == 0 {
			off += LogicalBlockSize - off%LogicalBlockSize
			continue
		}
		n := uint32(b[0])
		if off%LogicalBlockSize+n > LogicalBlockSize {
			t.Fatalf("record at offset %d is %d bytes and crosses a block boundary", off, n)
		}
		seen++
		off += n
	}
	// 200 files plus "." and "..".
	if seen != 202 {
		t.Errorf("found %d directory records, want 202", seen)
	}
}

func TestFileContentsAndAddresses(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries,
		entry("small.txt", "hello world"),
		entry("empty.dat", ""),
	)
	// A file that spans several blocks exercises the read loop's padding.
	big := strings.Repeat("0123456789", 700) // 7000 bytes, four blocks
	root.Entries = append(root.Entries, entry("big.bin", big))

	sub := root.AddDir("data")
	sub.Entries = append(sub.Entries, entry("nested.txt", "nested"))

	_, m := build(t, root)

	check := func(e *Entry, want string) {
		t.Helper()
		var got []byte
		for i := range max(blocks(e.Size), 1) {
			got = append(got, m.blocks[e.LBA+i]...)
		}
		if string(got[:len(want)]) != want {
			t.Errorf("%s: contents do not match", e.Name)
		}
		// Everything after the payload must be zero padding.
		for i, b := range got[len(want):] {
			if b != 0 {
				t.Fatalf("%s: byte %d past the payload is %#x", e.Name, i, b)
			}
		}
	}

	byName := map[string]*Entry{}
	for _, e := range root.Entries {
		byName[e.Name] = e
	}
	check(byName["SMALL.TXT;1"], "hello world")
	check(byName["EMPTY.DAT;1"], "")
	check(byName["BIG.BIN;1"], big)
	check(sub.Entries[0], "nested")

	// A zero length file still has to be given a real address.
	if byName["EMPTY.DAT;1"].LBA == 0 {
		t.Error("empty file was not assigned an address")
	}
}

// The run-out after the last file is what keeps a drive's read-ahead from
// running off the end of the track, so it has to be real blocks in the image
// and inside the volume rather than a number in a header.
func TestPadding(t *testing.T) {
	const padding = 150

	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"))

	bare, err := NewWriter(root, Options{VolumeID: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	unpadded := bare.TotalBlocks()

	w, err := NewWriter(root, Options{VolumeID: "TEST", Padding: padding})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.TotalBlocks(); got != unpadded+padding {
		t.Errorf("padded image is %d blocks, want %d", got, unpadded+padding)
	}

	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}
	if got := uint32(len(m.blocks)); got != unpadded+padding {
		t.Fatalf("emitted %d blocks, want %d", got, unpadded+padding)
	}

	// The last file still ends where it did, and everything after it is
	// empty.
	last := root.Entries[0]
	first := last.LBA + last.sectors()
	for lba := first; lba < uint32(len(m.blocks)); lba++ {
		if !bytes.Equal(m.blocks[lba], make([]byte, LogicalBlockSize)) {
			t.Fatalf("block %d of the run-out is not empty", lba)
		}
		if m.eof[lba] {
			t.Errorf("block %d of the run-out is marked as the end of a file", lba)
		}
	}

	// A reader must see a volume that covers the padding, or its own bounds
	// checks will disagree with the image it was given.
	pvd := m.blocks[PVDSector]
	if got := binary.LittleEndian.Uint32(pvd[80:]); got != w.TotalBlocks() {
		t.Errorf("volume space size = %d, want %d", got, w.TotalBlocks())
	}
	r, err := NewReader(memReader{m})
	if err != nil {
		t.Fatal(err)
	}
	if r.Volume.Blocks != w.TotalBlocks() {
		t.Errorf("read back %d blocks, want %d", r.Volume.Blocks, w.TotalBlocks())
	}
	if len(r.Root.Entries) != 1 {
		t.Errorf("read %d files, want 1", len(r.Root.Entries))
	}
}

// Every directory must appear in the path table with the correct parent, or a
// drive that uses the table instead of walking the tree will not find it.
func TestPathTable(t *testing.T) {
	root := &Dir{}
	data := root.AddDir("data")
	data.AddDir("sound")
	root.AddDir("gfx")

	w, m := build(t, root)

	table := m.blocks[w.lPathTableLBA]
	type rec struct {
		name   string
		parent uint16
	}
	var got []rec
	for off := uint32(0); off < w.pathTableLen; {
		idLen := uint32(table[off])
		name := string(table[off+8 : off+8+idLen])
		if name == "\x00" {
			name = "/"
		}
		got = append(got, rec{name, binary.LittleEndian.Uint16(table[off+6:])})
		n := 8 + idLen
		if n%2 != 0 {
			n++
		}
		off += n
	}

	// Breadth first, so parents always precede their children.
	want := []rec{{"/", 1}, {"DATA", 1}, {"GFX", 1}, {"SOUND", 2}}
	if len(got) != len(want) {
		t.Fatalf("path table has %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}
