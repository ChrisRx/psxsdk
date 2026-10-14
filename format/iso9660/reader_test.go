package iso9660

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// memReader reads back what a memWriter collected, so the filesystem can be
// read without a disc underneath it.
type memReader struct{ m *memWriter }

func (r memReader) block(lba uint32, b []byte, want int) error {
	if int(lba) >= len(r.m.blocks) {
		return fmt.Errorf("block %d is past the end of the image", lba)
	}
	got := r.m.blocks[lba]
	if len(got) != want {
		return fmt.Errorf("block %d is %d bytes, want %d", lba, len(got), want)
	}
	copy(b, got)
	return nil
}

func (r memReader) ReadBlock(lba uint32, b []byte) error {
	return r.block(lba, b, LogicalBlockSize)
}

func (r memReader) ReadSector(lba uint32, b []byte) error {
	return r.block(lba, b, RawSectorSize)
}

// contents reads an entry back in full.
func contents(t *testing.T, e *Entry) []byte {
	t.Helper()
	rc, err := e.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("%s: %v", e.Name, err)
	}
	return b
}

// flatten lists a tree as paths, so two trees can be compared without caring
// how they are nested.
func flatten(d *Dir, prefix string, out map[string]*Entry) {
	for _, e := range d.Entries {
		out[prefix+"/"+e.Name] = e
	}
	for _, sub := range d.Dirs {
		flatten(sub, prefix+"/"+sub.Name, out)
	}
}

func TestReadBackTree(t *testing.T) {
	root := &Dir{}
	big := strings.Repeat("0123456789", 700) // 7000 bytes, four blocks
	root.Entries = append(root.Entries,
		entry("main.exe", "hello world"),
		entry("big.bin", big),
		entry("empty.dat", ""),
	)
	xa := entry("music.xa", strings.Repeat("S", RawSectorSize*3))
	xa.Raw = true
	root.Entries = append(root.Entries, xa)

	sound := root.AddDir("data").AddDir("sound")
	sound.Entries = append(sound.Entries, entry("sample1.seq", "seq"))

	opts := Options{
		VolumeID:    "TESTDISC",
		Publisher:   "PUBLISHER",
		Preparer:    "PREPARER",
		Application: "APPLICATION",
		Created:     time.Date(2026, 10, 14, 12, 30, 15, 0, time.UTC),
	}
	w, err := NewWriter(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(memReader{m})
	if err != nil {
		t.Fatal(err)
	}

	if r.Volume.VolumeID != "TESTDISC" {
		t.Errorf("volume id = %q", r.Volume.VolumeID)
	}
	if r.Volume.Publisher != "PUBLISHER" || r.Volume.Preparer != "PREPARER" || r.Volume.Application != "APPLICATION" {
		t.Errorf("descriptor text = %q, %q, %q", r.Volume.Publisher, r.Volume.Preparer, r.Volume.Application)
	}
	if !r.Volume.Created.Equal(opts.Created) {
		t.Errorf("created = %s, want %s", r.Volume.Created, opts.Created)
	}
	if r.Volume.Blocks != w.TotalBlocks() {
		t.Errorf("volume is %d blocks, want %d", r.Volume.Blocks, w.TotalBlocks())
	}
	if !r.Volume.XA {
		t.Error("the CD-XA signature was not read back")
	}

	want := map[string]*Entry{}
	flatten(root, "", want)
	got := map[string]*Entry{}
	flatten(r.Root, "", got)

	if len(got) != len(want) {
		t.Fatalf("read %d files, want %d: %v", len(got), len(want), got)
	}
	for path, e := range want {
		back, ok := got[path]
		if !ok {
			t.Errorf("%s is missing from the image", path)
			continue
		}
		if back.Size != e.Size {
			t.Errorf("%s: read %d bytes, want %d", path, back.Size, e.Size)
		}
		if back.LBA != e.LBA {
			t.Errorf("%s: read address %d, want %d", path, back.LBA, e.LBA)
		}
		// A stream is only distinguishable from data by its attributes,
		// which is what makes writing them worth the bytes.
		if back.Raw != e.Raw {
			t.Errorf("%s: read raw = %v, want %v", path, back.Raw, e.Raw)
		}
		if a, b := contents(t, back), contents(t, e); !bytes.Equal(a, b) {
			t.Errorf("%s: contents differ: read %d bytes, want %d", path, len(a), len(b))
		}
	}
}

// An image read back and written again is the same image. This is the check
// that the reader and the writer agree about every field, rather than only
// the ones a test thought to look at.
func TestReadBackReproducesImage(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"), entry("data.bin", strings.Repeat("x", 5000)))
	xa := entry("music.xa", strings.Repeat("S", RawSectorSize*2))
	xa.Raw = true
	root.Entries = append(root.Entries, xa)
	data := root.AddDir("data")
	data.Entries = append(data.Entries, entry("nested.txt", "nested"))

	opts := Options{VolumeID: "TESTDISC", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	first := &memWriter{}
	w, err := NewWriter(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteTo(first); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(memReader{first})
	if err != nil {
		t.Fatal(err)
	}

	// The tree read back carries its own timestamps, so FileTimes has to be
	// set for them to be written again where the originals were.
	again := opts
	again.FileTimes = true
	w2, err := NewWriter(r.Root, again)
	if err != nil {
		t.Fatal(err)
	}
	second := &memWriter{}
	if err := w2.WriteTo(second); err != nil {
		t.Fatal(err)
	}

	if len(second.blocks) != len(first.blocks) {
		t.Fatalf("rewritten image is %d blocks, want %d", len(second.blocks), len(first.blocks))
	}
	for i := range first.blocks {
		if !bytes.Equal(first.blocks[i], second.blocks[i]) {
			t.Fatalf("block %d differs after a round trip", i)
		}
	}
}

// Timestamps survive a round trip, which is the only reason to record them.
//
// The fields hold local time and an offset from GMT, so a time away from GMT
// is the case that catches an image which records the digits of one zone and
// the offset of another: it reads back as an instant moved by however far the
// machine that built it was from GMT.
func TestReadBackFileTimes(t *testing.T) {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("EDT", -4*60*60),
		time.FixedZone("ACST", 9*60*60+30*60), // not a whole number of hours
	}
	for _, zone := range zones {
		mod := time.Date(2026, 3, 4, 14, 56, 42, 0, zone)
		root := &Dir{}
		e := entry("main.exe", "hello")
		e.Modified = mod
		root.Entries = append(root.Entries, e)

		w, err := NewWriter(root, Options{VolumeID: "TEST", FileTimes: true, Created: mod})
		if err != nil {
			t.Fatal(err)
		}
		m := &memWriter{}
		if err := w.WriteTo(m); err != nil {
			t.Fatal(err)
		}

		r, err := NewReader(memReader{m})
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Root.Entries[0].Modified; !got.Equal(mod) {
			t.Errorf("%s: modification time = %s, want %s", zone, got, mod)
		}
		if got := r.Volume.Created; !got.Equal(mod) {
			t.Errorf("%s: volume date = %s, want %s", zone, got, mod)
		}
	}

	mod := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	root := &Dir{}
	e := entry("main.exe", "hello")
	e.Modified = mod
	root.Entries = append(root.Entries, e)

	// Without FileTimes the records carry the volume date, and a zero
	// volume date is recorded as no date rather than as the year 1900.
	w, err := NewWriter(root, Options{VolumeID: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(memReader{m})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Root.Entries[0].Modified; !got.IsZero() {
		t.Errorf("modification time = %s, want none", got)
	}
}

func TestReaderRejectsNonImages(t *testing.T) {
	// Blocks of nothing, so there is no descriptor where one should be.
	m := &memWriter{}
	for i := range PVDSector + 2 {
		if err := m.WriteBlock(Block{LBA: uint32(i), Data: make([]byte, LogicalBlockSize)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewReader(memReader{m}); err == nil {
		t.Error("a volume with no descriptor was accepted")
	}

	// A descriptor of the wrong type is a readable image that is not the
	// one being looked for.
	m.blocks[PVDSector][0] = volumeDescriptorTerminator
	copy(m.blocks[PVDSector][1:], "CD001")
	if _, err := NewReader(memReader{m}); err == nil {
		t.Error("a descriptor set terminator was taken for a primary volume descriptor")
	}
}

// A directory whose record points back into the tree would otherwise be
// followed until the stack ran out.
func TestReaderRejectsCircularTree(t *testing.T) {
	root := &Dir{}
	sub := root.AddDir("data")
	sub.Entries = append(sub.Entries, entry("a.dat", "a"))
	w, err := NewWriter(root, Options{VolumeID: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}

	// Point the subdirectory's record at the root's extent.
	extent := m.blocks[root.LBA]
	for off := 0; off < len(extent) && extent[off] != 0; off += int(extent[off]) {
		rec := extent[off : off+int(extent[off])]
		if string(rec[33:33+rec[32]]) == "DATA" {
			putBoth32(rec[2:], root.LBA)
			break
		}
	}
	if _, err := NewReader(memReader{m}); err == nil {
		t.Error("a directory pointing at its own parent was followed")
	}
}
