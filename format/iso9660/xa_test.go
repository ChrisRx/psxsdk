package iso9660

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// walkRecords returns every directory record in a directory's extent, in
// order, as a reader would find them.
func walkRecords(t *testing.T, m *memWriter, d *Dir) [][]byte {
	t.Helper()
	var out [][]byte
	for off := uint32(0); off < d.Size; {
		b := m.blocks[d.LBA+off/LogicalBlockSize][off%LogicalBlockSize:]
		if b[0] == 0 {
			off += LogicalBlockSize - off%LogicalBlockSize
			continue
		}
		n := uint32(b[0])
		out = append(out, b[:n])
		off += n
	}
	return out
}

// recordID is the file identifier of a record, with the "." and ".." entries
// rendered as they are usually written.
func recordID(rec []byte) string {
	id := string(rec[33 : 33+rec[32]])
	switch id {
	case "\x00":
		return "."
	case "\x01":
		return ".."
	}
	return id
}

// xaOf returns the XA block of a record, which sits at its end.
func xaOf(rec []byte) []byte {
	return rec[len(rec)-xaSysUseLen:]
}

func TestXASignature(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"))
	_, m := build(t, root)

	pvd := m.blocks[PVDSector]
	if got := string(pvd[xaSignatureOffset : xaSignatureOffset+8]); got != "CD-XA001" {
		t.Errorf("volume descriptor signature at %d = %q, want CD-XA001", xaSignatureOffset, got)
	}

	// The signature lives in the application use area, so it must not have
	// landed on top of a field the standard defines.
	if pvd[881] != 1 {
		t.Errorf("file structure version = %d, want 1", pvd[881])
	}
	if xaSignatureOffset < 883 {
		t.Errorf("signature at %d is outside the application use area", xaSignatureOffset)
	}
}

func TestXADirectoryRecords(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"))
	xa := entry("music.xa", strings.Repeat("\x00", RawSectorSize*2))
	xa.Raw = true
	root.Entries = append(root.Entries, xa)
	root.AddDir("data")

	_, m := build(t, root)

	want := map[string]uint16{
		".":          xaAttrDir,
		"..":         xaAttrDir,
		"DATA":       xaAttrDir,
		"MAIN.EXE;1": xaAttrFile,
		"MUSIC.XA;1": xaAttrForm2,
	}
	seen := map[string]bool{}
	for _, rec := range walkRecords(t, m, root) {
		id := recordID(rec)
		x := xaOf(rec)
		if string(x[6:8]) != "XA" {
			t.Errorf("%s: system use area is not an XA entry: % x", id, x)
			continue
		}
		attr, ok := want[id]
		if !ok {
			t.Errorf("unexpected record %q", id)
			continue
		}
		if got := binary.BigEndian.Uint16(x[4:]); got != attr {
			t.Errorf("%s: attributes = %#04x, want %#04x", id, got, attr)
		}
		// A record must stay an even number of bytes with the XA block on
		// the end, or every record after it is misaligned.
		if len(rec)%2 != 0 {
			t.Errorf("%s: record is %d bytes, which is odd", id, len(rec))
		}
		seen[id] = true
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("no record for %q", id)
		}
	}

	// The documented values, as a guard against the bit definitions above
	// drifting.
	if xaAttrDir != 0x8d55 || xaAttrFile != 0x0d55 {
		t.Errorf("attributes are %#04x for a directory and %#04x for a file, want 0x8d55 and 0x0d55",
			xaAttrDir, xaAttrFile)
	}
}

// Without the extension the records go back to being plain ISO 9660, which is
// what makes an image comparable with another implementation's.
func TestOmitXA(t *testing.T) {
	root := &Dir{}
	root.Entries = append(root.Entries, entry("main.exe", "hello"))

	w, err := NewWriter(root, Options{VolumeID: "TEST", OmitXA: true})
	if err != nil {
		t.Fatal(err)
	}
	m := &memWriter{}
	if err := w.WriteTo(m); err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(m.blocks[PVDSector], []byte("CD-XA001")) {
		t.Error("volume descriptor still carries the XA signature")
	}
	for _, rec := range walkRecords(t, m, root) {
		if n := int(rec[0]); n != DirRecordFixedLen+int(rec[32])+(DirRecordFixedLen+int(rec[32]))%2 {
			t.Errorf("%s: record is %d bytes, which leaves room for a system use area", recordID(rec), n)
		}
	}
}

// A raw file is written sector for sector, and its record reports the extent
// it occupies rather than the length of the file.
func TestRawEntry(t *testing.T) {
	// Two sectors of distinguishable contents.
	payload := strings.Repeat("A", RawSectorSize) + strings.Repeat("B", RawSectorSize)
	root := &Dir{}
	xa := entry("music.xa", payload)
	xa.Raw = true
	root.Entries = append(root.Entries, xa)

	w, m := build(t, root)

	if got := xa.sectors(); got != 2 {
		t.Fatalf("file occupies %d sectors, want 2", got)
	}
	if got := xa.recordedSize(); got != 2*LogicalBlockSize {
		t.Errorf("recorded size = %d, want %d", got, 2*LogicalBlockSize)
	}
	for i := range xa.sectors() {
		if !m.raw[xa.LBA+i] {
			t.Errorf("block %d was not emitted as a raw sector", xa.LBA+i)
		}
		if got := len(m.blocks[xa.LBA+i]); got != RawSectorSize {
			t.Errorf("block %d is %d bytes, want %d", xa.LBA+i, got, RawSectorSize)
		}
	}
	if got := string(m.blocks[xa.LBA][:1]); got != "A" {
		t.Errorf("first sector starts with %q", got)
	}
	if got := string(m.blocks[xa.LBA+1][:1]); got != "B" {
		t.Errorf("second sector starts with %q", got)
	}

	// The layout has to agree with what was emitted, and a raw file's
	// sectors are larger than a block, so a count taken in blocks would run
	// the rest of the image off the end.
	if w.TotalBlocks() != xa.LBA+2 {
		t.Errorf("image is %d blocks, want %d", w.TotalBlocks(), xa.LBA+2)
	}
}

// A file that does not hold a whole number of sectors is not the stream it was
// taken for, and padding it would hand the console a sector with no subheader.
func TestRawEntryRejectsPartialSector(t *testing.T) {
	root := &Dir{}
	xa := entry("music.xa", strings.Repeat("A", RawSectorSize+7))
	xa.Raw = true
	root.Entries = append(root.Entries, xa)

	if _, err := NewWriter(root, Options{}); err == nil {
		t.Fatal("writer accepted a raw file that is not a whole number of sectors")
	}
}

// The end of a file is marked so that a reader can see where one stops
// without going back to the filesystem.
func TestEndOfFileMarks(t *testing.T) {
	root := &Dir{}
	big := strings.Repeat("x", LogicalBlockSize*2+10) // three blocks
	root.Entries = append(root.Entries,
		entry("one.dat", "small"),
		entry("big.dat", big),
	)
	_, m := build(t, root)

	byName := map[string]*Entry{}
	for _, e := range root.Entries {
		byName[e.Name] = e
	}
	for _, e := range root.Entries {
		last := e.LBA + e.sectors() - 1
		for i := range e.sectors() {
			lba := e.LBA + i
			if got := m.eof[lba]; got != (lba == last) {
				t.Errorf("%s: block %d end of file = %v, want %v", e.Name, lba, got, lba == last)
			}
		}
	}

	// A directory's records are a file too, and the volume structure is not.
	if !m.eof[root.LBA+blocks(root.Size)-1] {
		t.Error("the last block of the root directory is not marked")
	}
	if m.eof[PVDSector] {
		t.Error("the volume descriptor is marked as the end of a file")
	}
}

// Zero length files are the case where sizing in sectors and sizing in bytes
// disagree most.
func TestRawEmptyEntry(t *testing.T) {
	root := &Dir{}
	xa := &Entry{
		Name: FileIdentifier("empty.xa"),
		Raw:  true,
		Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil },
	}
	root.Entries = append(root.Entries, xa)

	_, m := build(t, root)
	if xa.LBA == 0 {
		t.Error("empty raw file was not assigned an address")
	}
	if got := len(m.blocks[xa.LBA]); got != RawSectorSize {
		t.Errorf("its sector is %d bytes, want %d", got, RawSectorSize)
	}
}
