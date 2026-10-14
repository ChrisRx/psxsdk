package psxdisc

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
)

// block is the kind of block the filesystem hands down, with 2048 bytes of
// contents unless it is a raw sector.
func block(lba uint32, fill byte) iso9660.Block {
	return iso9660.Block{LBA: lba, Data: bytes.Repeat([]byte{fill}, iso9660.LogicalBlockSize)}
}

// The two packages size raw entries with constants of their own, and a
// disagreement would misplace every sector after the first raw file.
func TestRawSectorSizesAgree(t *testing.T) {
	if iso9660.RawSectorSize != cdrom.Mode2PayloadSize {
		t.Errorf("iso9660 uses %d bytes for a raw sector but cdrom uses %d",
			iso9660.RawSectorSize, cdrom.Mode2PayloadSize)
	}
}

func TestRawWriterSubmode(t *testing.T) {
	var buf bytes.Buffer
	w := NewRawWriter(&buf)

	b := block(0, 'a')
	if err := w.WriteBlock(b); err != nil {
		t.Fatal(err)
	}
	b = block(1, 'b')
	b.EndOfFile = true
	if err := w.WriteBlock(b); err != nil {
		t.Fatal(err)
	}

	if got := buf.Len(); got != 2*cdrom.SectorSize {
		t.Fatalf("wrote %d bytes, want %d", got, 2*cdrom.SectorSize)
	}
	image := buf.Bytes()

	// Both copies of the subheader must say the same thing, and the last
	// sector of a file is the 0x89 a pressed disc carries: data, end of
	// record and end of file.
	for i, want := range []byte{0x08, 0x89} {
		sector := image[i*cdrom.SectorSize:]
		for _, off := range []int{16, 20} {
			if got := sector[off+2]; got != want {
				t.Errorf("sector %d: submode at %d = %#02x, want %#02x", i, off, got, want)
			}
		}
		if got := sector[userDataOffset]; got != 'a'+byte(i) {
			t.Errorf("sector %d: data starts with %q", i, got)
		}
	}
}

// userDataOffset is where a sector's user data begins, after the sync
// pattern, address header and both copies of the subheader.
const userDataOffset = 24

// A raw sector is recorded as it stands. Re-encoding it would mean replacing
// the subheader the stream carries with one invented here.
func TestRawWriterPassesSectorsThrough(t *testing.T) {
	payload := make([]byte, iso9660.RawSectorSize)
	for i := range payload {
		payload[i] = byte(i)
	}

	var buf bytes.Buffer
	w := NewRawWriter(&buf)
	if err := w.WriteBlock(iso9660.Block{LBA: 0, Data: payload, Raw: true}); err != nil {
		t.Fatal(err)
	}

	sector := buf.Bytes()
	if !bytes.Equal(sector[16:], payload) {
		t.Error("the payload was not recorded as it was given")
	}
	// Only the sync pattern and the address are added.
	if !bytes.Equal(sector[:12], []byte{0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0}) {
		t.Errorf("sync pattern = % x", sector[:12])
	}
	if sector[15] != 2 {
		t.Errorf("mode = %d, want 2", sector[15])
	}
}

func TestWritersRejectOutOfOrderBlocks(t *testing.T) {
	for _, tt := range []struct {
		name string
		make func(io.Writer) iso9660.BlockWriter
	}{
		{"raw", func(w io.Writer) iso9660.BlockWriter { return NewRawWriter(w) }},
		{"iso", func(w io.Writer) iso9660.BlockWriter { return NewISOWriter(w) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.make(io.Discard)
			if err := w.WriteBlock(block(0, 0)); err != nil {
				t.Fatal(err)
			}
			if err := w.WriteBlock(block(2, 0)); err == nil {
				t.Error("a gap in the addresses was accepted")
			}
		})
	}
}

// A 2048-byte image has nowhere to put a subheader, so a stream cannot go in
// one and saying so beats writing 2324 bytes of unidentifiable data.
func TestISOWriterRejectsRawSectors(t *testing.T) {
	w := NewISOWriter(io.Discard)
	err := w.WriteBlock(iso9660.Block{LBA: 0, Data: make([]byte, iso9660.RawSectorSize), Raw: true})
	if err == nil {
		t.Fatal("a raw sector was accepted into a 2048-byte image")
	}
	if !strings.Contains(err.Error(), "raw") {
		t.Errorf("error does not say what the trouble is: %v", err)
	}
}

func TestSystemArea(t *testing.T) {
	area := SystemArea()
	if got := len(area); got != iso9660.SystemAreaSectors*iso9660.LogicalBlockSize {
		t.Fatalf("system area is %d bytes, want %d", got, iso9660.SystemAreaSectors*iso9660.LogicalBlockSize)
	}
	at := LicenseStringSector * iso9660.LogicalBlockSize
	if got := string(area[at : at+len(identificationString)]); got != identificationString {
		t.Errorf("identification string at sector %d = %q", LicenseStringSector, got)
	}
	// It is the only thing in there; the licence data proper is the user's
	// to supply.
	if n := bytes.Count(area, []byte("Sony")); n != 1 {
		t.Errorf("the string appears %d times", n)
	}
}
