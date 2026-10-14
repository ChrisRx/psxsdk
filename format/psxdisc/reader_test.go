package psxdisc

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
)

// discTree is a filesystem with one of everything a disc holds: a file that
// spans blocks, an empty one, a nested directory, and a stream of raw
// sectors.
func discTree() *iso9660.Dir {
	file := func(name, content string) *iso9660.Entry {
		return &iso9660.Entry{
			Name: iso9660.FileIdentifier(name),
			Size: uint32(len(content)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(content)), nil
			},
		}
	}

	root := &iso9660.Dir{}
	root.Entries = append(root.Entries,
		file("main.exe", strings.Repeat("E", 5000)),
		file("empty.dat", ""),
	)
	xa := file("music.xa", strings.Repeat("S", iso9660.RawSectorSize*2))
	xa.Raw = true
	root.Entries = append(root.Entries, xa)

	sound := root.AddDir("data").AddDir("sound")
	sound.Entries = append(sound.Entries, file("sample1.seq", "sequence"))
	return root
}

// writeImage writes a whole image, the way the tool does, and returns it.
func writeImage(t *testing.T, root *iso9660.Dir, audio ...*AudioTrack) ([]byte, Layout) {
	t.Helper()
	w, err := iso9660.NewWriter(root, iso9660.Options{
		VolumeID:   "TESTDISC",
		SystemArea: SystemArea(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	l, err := (&Image{Data: w, Audio: audio}).Write(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), l
}

// An image written as sectors reads back through the sector layer as the tree
// that went in. This is the whole path: filesystem to sectors and back.
func TestRoundTripRawImage(t *testing.T) {
	root := discTree()
	image, _ := writeImage(t, root)

	br, err := NewRawReader(bytes.NewReader(image), int64(len(image)))
	if err != nil {
		t.Fatal(err)
	}
	r, err := iso9660.NewReader(br)
	if err != nil {
		t.Fatal(err)
	}

	if r.Volume.VolumeID != "TESTDISC" {
		t.Errorf("volume id = %q", r.Volume.VolumeID)
	}

	type found struct {
		size uint32
		raw  bool
		body string
	}
	got := map[string]found{}
	var walk func(d *iso9660.Dir, prefix string)
	walk = func(d *iso9660.Dir, prefix string) {
		for _, e := range d.Entries {
			rc, err := e.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("%s: %v", e.Name, err)
			}
			got[prefix+"/"+e.Name] = found{e.Size, e.Raw, string(b)}
		}
		for _, sub := range d.Dirs {
			walk(sub, prefix+"/"+sub.Name)
		}
	}
	walk(r.Root, "")

	want := map[string]found{
		"/MAIN.EXE;1":               {5000, false, strings.Repeat("E", 5000)},
		"/EMPTY.DAT;1":              {0, false, ""},
		"/MUSIC.XA;1":               {iso9660.RawSectorSize * 2, true, strings.Repeat("S", iso9660.RawSectorSize*2)},
		"/DATA/SOUND/SAMPLE1.SEQ;1": {8, false, "sequence"},
	}
	if len(got) != len(want) {
		t.Fatalf("read %d files, want %d", len(got), len(want))
	}
	for path, w := range want {
		g, ok := got[path]
		if !ok {
			t.Errorf("%s is missing", path)
			continue
		}
		if g.size != w.size || g.raw != w.raw || g.body != w.body {
			t.Errorf("%s: read %d bytes raw=%v, want %d bytes raw=%v", path, g.size, g.raw, w.size, w.raw)
		}
	}
}

// Audio tracks sit after the filesystem, and reading the filesystem must not
// be affected by them.
func TestRoundTripIgnoresAudioTracks(t *testing.T) {
	root := discTree()
	image, l := writeImage(t, root, audioFromBytes("music.wav", samples(1000)))

	br, err := NewRawReader(bytes.NewReader(image), int64(len(image)))
	if err != nil {
		t.Fatal(err)
	}
	r, err := iso9660.NewReader(br)
	if err != nil {
		t.Fatal(err)
	}
	// The volume ends where the data track does, not where the image does.
	if r.Volume.Blocks != l.Tracks[0].Sectors {
		t.Errorf("volume is %d blocks but the data track is %d sectors", r.Volume.Blocks, l.Tracks[0].Sectors)
	}
	if l.Sectors() <= r.Volume.Blocks {
		t.Error("the audio track was not written after the filesystem")
	}
	if len(r.Root.Entries) == 0 {
		t.Error("no files were read")
	}
}

// A sector of a filesystem is meant to be read correctly or not at all, so a
// damaged one is reported rather than passed on.
func TestRawReaderDetectsDamage(t *testing.T) {
	root := discTree()
	image, _ := writeImage(t, root)

	// Flip a bit in the user data of the block holding the volume
	// descriptor, leaving its error detection describing what used to be
	// there.
	damaged := bytes.Clone(image)
	damaged[iso9660.PVDSector*cdrom.SectorSize+24+100] ^= 0x01

	br, err := NewRawReader(bytes.NewReader(damaged), int64(len(damaged)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = iso9660.NewReader(br)
	if err == nil {
		t.Fatal("a damaged sector was read as if it were intact")
	}
	if !strings.Contains(err.Error(), "damaged") {
		t.Errorf("error does not say the sector is damaged: %v", err)
	}

	// The sync pattern is what says these bytes are a sector at all.
	noSync := bytes.Clone(image)
	noSync[iso9660.PVDSector*cdrom.SectorSize+1] = 0
	br, err = NewRawReader(bytes.NewReader(noSync), int64(len(noSync)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := iso9660.NewReader(br); err == nil {
		t.Error("a sector with no sync pattern was accepted")
	}
}

// An image of plain blocks is the other thing a reader has to cope with, and
// the filesystem code should not be able to tell the difference.
func TestRoundTripISOImage(t *testing.T) {
	root := &iso9660.Dir{}
	root.Entries = append(root.Entries, &iso9660.Entry{
		Name: iso9660.FileIdentifier("main.exe"),
		Size: 5,
		Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("hello")), nil },
	})

	w, err := iso9660.NewWriter(root, iso9660.Options{VolumeID: "PLAIN"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := w.WriteTo(NewISOWriter(&buf)); err != nil {
		t.Fatal(err)
	}

	br, err := NewISOReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	r, err := iso9660.NewReader(br)
	if err != nil {
		t.Fatal(err)
	}
	if r.Volume.VolumeID != "PLAIN" {
		t.Errorf("volume id = %q", r.Volume.VolumeID)
	}
	rc, err := r.Root.Entries[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Errorf("contents = %q", b)
	}
}

// Which kind of image a file holds follows from the file, since a tool is
// given a path and nothing else.
func TestOpenDetectsImageFormat(t *testing.T) {
	dir := t.TempDir()
	root := discTree()
	raw, _ := writeImage(t, root)

	rawPath := filepath.Join(dir, "disc.bin")
	if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	r, closer, err := Open(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	if r.Volume.VolumeID != "TESTDISC" {
		t.Errorf("raw image: volume id = %q", r.Volume.VolumeID)
	}
	if !r.Volume.XA {
		t.Error("raw image: the XA signature was not read")
	}

	// A .iso of the same filesystem, which has no raw entries in it.
	plain := &iso9660.Dir{}
	plain.Entries = append(plain.Entries, &iso9660.Entry{
		Name: iso9660.FileIdentifier("main.exe"),
		Size: 5,
		Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("hello")), nil },
	})
	w, err := iso9660.NewWriter(plain, iso9660.Options{VolumeID: "PLAIN"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := w.WriteTo(NewISOWriter(&buf)); err != nil {
		t.Fatal(err)
	}
	isoPath := filepath.Join(dir, "disc.iso")
	if err := os.WriteFile(isoPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	r2, closer2, err := Open(isoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer closer2.Close()
	if r2.Volume.VolumeID != "PLAIN" {
		t.Errorf("iso image: volume id = %q", r2.Volume.VolumeID)
	}

	// Something that is neither.
	oddPath := filepath.Join(dir, "odd.bin")
	if err := os.WriteFile(oddPath, make([]byte, 1234), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(oddPath); err == nil {
		t.Error("a file that is not a disc image was opened")
	}
}

// A stream cannot be read out of an image that never held the subheaders.
func TestISOReaderRejectsSectorReads(t *testing.T) {
	r, err := NewISOReader(bytes.NewReader(make([]byte, iso9660.LogicalBlockSize)), iso9660.LogicalBlockSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReadSector(0, make([]byte, iso9660.RawSectorSize)); err == nil {
		t.Error("a 2048-byte image returned a raw sector")
	}
}
