package mkdisc

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxdisc"
	"go.chrisrx.dev/psx/internal/discrecord"
)

const textAddr = 0x80010000

// A reg is a registration to put in a test executable: the disc path, the
// host path, and the bytes of the buffer Into gave it, or 0.
type reg struct {
	name, src string
	buf       uint32
}

// exe writes a PSX-EXE to dir whose text is a record per registration, in
// the layout discrecord spells out, and returns its path.
func exe(t *testing.T, dir string, regs ...reg) string {
	t.Helper()
	var text []byte
	word := func(v uint32) { text = binary.LittleEndian.AppendUint32(text, v) }
	str := func(s string) (addr, n uint32) {
		addr = textAddr + uint32(len(text))
		text = append(text, s...)
		for len(text)%4 != 0 {
			text = append(text, 0)
		}
		return addr, uint32(len(s))
	}
	for _, r := range regs {
		nameAddr, nameLen := str(r.name)
		srcAddr, srcLen := str(r.src)
		word(discrecord.Magic)
		word(nameAddr)
		word(nameLen)
		word(srcAddr)
		word(srcLen)
		word(0) // sector
		word(0) // size
		if r.buf == 0 {
			word(0)
			word(0)
			word(0)
		} else {
			word(0x80100000)
			word(r.buf / 4)
			word(r.buf / 4)
		}
	}
	text = append(text, make([]byte, 2048-len(text)%2048)...)
	f := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:     psx.ExecutableSignature,
			PC0:       textAddr,
			TextAddr:  textAddr,
			TextSize:  uint32(len(text)),
			StackAddr: 0x801fff00,
		},
		Sections: []*psx.Section{{Name: "text", Addr: textAddr, Data: text}},
	}
	path := filepath.Join(dir, "prog.exe")
	assert.NoError(t, f.WriteFile(path))
	return path
}

func write(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}

func readAll(t *testing.T, e *iso9660.Entry) []byte {
	t.Helper()
	if e == nil {
		t.Fatal("no such file on the disc")
	}
	rc, err := e.Open()
	assert.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	assert.NoError(t, err)
	return data
}

// records reads the sector and size words of every record in an executable
// as it is on the disc, by host path.
func records(t *testing.T, image []byte) map[string][2]uint32 {
	t.Helper()
	text := image[psx.HeaderSize:]
	got := map[string][2]uint32{}
	for _, r := range discrecord.Find(text, textAddr) {
		got[r.Name+"<"+r.Src] = [2]uint32{
			binary.LittleEndian.Uint32(text[r.Offset+discrecord.OffSector:]),
			binary.LittleEndian.Uint32(text[r.Offset+discrecord.OffSize:]),
		}
	}
	return got
}

func TestSplit(t *testing.T) {
	for _, tc := range []struct {
		name string
		dirs []string
		base string
	}{
		{"MAIN.EXE", []string{}, "MAIN.EXE"},
		{"data/sound/sample1.seq", []string{"data", "sound"}, "sample1.seq"},
		{`\DATA\SOUND\SAMPLE2.SEQ;1`, []string{"DATA", "SOUND"}, "SAMPLE2.SEQ"},
		{"/x", []string{}, "x"},
	} {
		dirs, base, err := split(tc.name)
		assert.NoError(t, err, tc.name)
		assert.Equal(t, tc.dirs, dirs, tc.name)
		assert.Equal(t, tc.base, base, tc.name)
	}
	for _, name := range []string{"", "/", "data//x", "data/", ";1", "//x"} {
		_, _, err := split(name)
		assert.Error(t, "is not a path on the disc", err, name)
	}
}

func TestPlace(t *testing.T) {
	dir := t.TempDir()
	host := write(t, dir, "a", []byte("a"))

	root := &iso9660.Dir{}
	e, err := place(root, "data/sound/sample1.seq", host)
	assert.NoError(t, err)
	assert.Equal(t, "SAMPLE1.SEQ;1", e.Name)
	assert.Equal(t, "DATA", root.Dirs[0].Name)
	assert.Equal(t, "SOUND", root.Dirs[0].Dirs[0].Name)

	// The same directory, written as a program would, takes the second
	// file rather than getting a twin.
	_, err = place(root, `\DATA\SOUND\SAMPLE2.SEQ;1`, host)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(root.Dirs))
	assert.Equal(t, 1, len(root.Dirs[0].Dirs))
	assert.Equal(t, 2, len(root.Dirs[0].Dirs[0].Entries))

	_, err = place(root, "Data/Sound/Sample1.SEQ", host)
	assert.Error(t, regexp.QuoteMeta(`\DATA\SOUND\SAMPLE1.SEQ;1 is already on the disc`), err)

	found, dirs, err := lookup(root, "data/sound/sample2.seq;1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"data", "sound"}, dirs)
	assert.Equal(t, "SAMPLE2.SEQ;1", found.Name)
	found, _, err = lookup(root, "data/none/x")
	assert.NoError(t, err)
	assert.Equal(t, (*iso9660.Entry)(nil), found)
}

func TestSystemCNF(t *testing.T) {
	assert.Equal(t, "BOOT = cdrom:\\MAIN.EXE;1\r\nTCB = 4\r\nEVENT = 10\r\nSTACK = 801FF000\r\n",
		string(systemCNF(`\MAIN.EXE;1`, DefaultStack)))
	assert.Equal(t, "BOOT = cdrom:\\BIN\\GAME.EXE;1\r\nTCB = 4\r\nEVENT = 10\r\nSTACK = 801FFFF0\r\n",
		string(systemCNF(`\BIN\GAME.EXE;1`, 0x801ffff0)))
}

// TestBuild writes a disc from a directory, a placed file, a program that
// registers files and tracks, and a given track, and reads it back.
func TestBuild(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tree/readme.txt", []byte("hello"))
	write(t, dir, "tree/music.xa", make([]byte, 2*cdrom.Mode2PayloadSize))
	write(t, dir, "assets/wall0.tmd", []byte("wall zero"))
	write(t, dir, "assets/stone.tim", []byte("stone"))
	sector := make([]byte, psxdisc.AudioSectorSize)
	given := write(t, dir, "given.raw", sector)
	write(t, dir, "audio/one.raw", append(append([]byte{}, sector...), sector...))
	write(t, dir, "audio/two.raw", sector)
	prog := exe(t, dir,
		reg{name: disc.TrackName, src: "audio/one.raw"},
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "stone.tim", src: "assets/stone.tim"},
		reg{name: disc.TrackName, src: "audio/two.raw"},
		reg{name: "data/wall0.tmd", src: "assets/wall0.tmd"}, // the same file, from another package
		reg{name: disc.TrackName, src: "audio/one.raw"},      // the same track too
	)

	opts := NewOptions()
	opts.Output = filepath.Join(dir, "out.cue")
	opts.Dir = filepath.Join(dir, "tree")
	opts.Files = []string{"MAIN.EXE=" + prog}
	opts.Boot = "main.exe"
	opts.Src = dir
	opts.Audio = []string{given}
	opts.VolumeID = "test"
	opts.Verbose = true
	var out bytes.Buffer
	opts.Stdout = &out
	if !assert.NoError(t, Build(opts)) {
		t.FailNow()
	}
	t.Log(out.String())

	r, closer, err := psxdisc.Open(filepath.Join(dir, "out.bin"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer closer.Close()
	assert.Equal(t, "TEST", r.Volume.VolumeID)
	assert.Equal(t, "PLAYSTATION", r.Volume.SystemID)
	assert.Equal(t, true, r.Volume.XA)

	assert.Equal(t, "hello", string(readAll(t, r.Root.Find("README.TXT"))))
	assert.Equal(t, "BOOT = cdrom:\\MAIN.EXE;1\r\nTCB = 4\r\nEVENT = 10\r\nSTACK = 801FF000\r\n",
		string(readAll(t, r.Root.Find("SYSTEM.CNF"))))
	music := r.Root.Find("MUSIC.XA")
	assert.Equal(t, true, music.Raw, "a .xa is a stream")
	assert.Equal(t, false, r.Root.Find("README.TXT").Raw)
	assert.Equal(t, 1, len(r.Root.Dirs))
	data := r.Root.Dirs[0]
	assert.Equal(t, "DATA", data.Name)
	wall := data.Find("WALL0.TMD")
	stone := r.Root.Find("STONE.TIM")
	assert.Equal(t, "wall zero", string(readAll(t, wall)))
	assert.Equal(t, "stone", string(readAll(t, stone)))

	// The executable on the disc says where each file is and which track
	// each track became: the given one is 2, the program's follow.
	got := records(t, readAll(t, r.Root.Find("MAIN.EXE")))
	assert.Equal(t, [2]uint32{wall.LBA, wall.Size}, got["DATA/WALL0.TMD<assets/wall0.tmd"])
	assert.Equal(t, [2]uint32{wall.LBA, wall.Size}, got["data/wall0.tmd<assets/wall0.tmd"])
	assert.Equal(t, [2]uint32{stone.LBA, stone.Size}, got["stone.tim<assets/stone.tim"])
	assert.Equal(t, [2]uint32{3, 2}, got[disc.TrackName+"<audio/one.raw"])
	assert.Equal(t, [2]uint32{4, 1}, got[disc.TrackName+"<audio/two.raw"])

	cue, err := os.ReadFile(opts.Output)
	assert.NoError(t, err)
	assert.Equal(t, `FILE "out.bin" BINARY`, strings.SplitN(string(cue), "\n", 2)[0])
	assert.Equal(t, 3, strings.Count(string(cue), "AUDIO"), "the given track and the program's two")

	// The run-out is there: the data track is the volume, and the first
	// pause follows it.
	assert.Equal(t, true, strings.Contains(out.String(), "150 of them run-out"), out.String())
	assert.Equal(t, true, strings.Contains(out.String(), `\DATA\WALL0.TMD;1`), out.String())
	assert.Equal(t, true, strings.Contains(out.String(), "registered from assets/wall0.tmd"), out.String())
}

func TestBuildErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "assets/wall0.tmd", []byte("wall zero"))
	write(t, dir, "assets/other.tmd", []byte("other"))
	write(t, dir, "disc/DATA/WALL0.TMD", []byte("from the tree"))
	write(t, dir, "nine.xa", []byte("nine byte"))

	build := func(prog string, set func(*Options)) error {
		opts := NewOptions()
		opts.Output = filepath.Join(dir, "out.bin")
		opts.Files = []string{"MAIN.EXE=" + prog}
		opts.Boot = "MAIN.EXE"
		opts.Src = dir
		opts.Stdout = io.Discard
		if set != nil {
			set(&opts)
		}
		return Build(opts)
	}

	// A path the tree already holds.
	prog := exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"})
	err := build(prog, func(o *Options) { o.Dir = filepath.Join(dir, "disc") })
	assert.Error(t, regexp.QuoteMeta(`\DATA\WALL0.TMD;1 is already on the disc`), err)

	// One path from two host files.
	prog = exe(t, dir,
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "data/wall0.tmd", src: "assets/other.tmd"},
	)
	assert.Error(t, regexp.QuoteMeta(`\DATA\WALL0.TMD;1 is registered from both assets/wall0.tmd and assets/other.tmd`), build(prog, nil))

	// A host file that is not there.
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/missing.tmd"})
	assert.Error(t, "missing.tmd", build(prog, nil))

	// A buffer the file does not fit: nine bytes take a sector, so a
	// buffer of one holds them and one of half a sector does not, on the
	// second registration as on the first.
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd", buf: 2048})
	assert.NoError(t, build(prog, nil))
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd", buf: 1024})
	assert.Error(t, "assets/wall0.tmd is 9 bytes, 1 sectors, but the buffer it is read into holds 1024", build(prog, nil))
	prog = exe(t, dir,
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "data/wall0.tmd", src: "assets/wall0.tmd", buf: 1024},
	)
	assert.Error(t, "holds 1024", build(prog, nil))

	// Nothing to boot.
	prog = exe(t, dir)
	assert.Error(t, "GAME.EXE is not on the disc", build(prog, func(o *Options) { o.Boot = "GAME.EXE" }))

	// A stream that is not whole sectors, and a pattern that is not one.
	assert.Error(t, "not a whole number of 2336-byte Mode 2 sectors", build(prog, func(o *Options) {
		o.Files = append(o.Files, "NINE.XA="+filepath.Join(dir, "nine.xa"))
	}))
	assert.Error(t, "syntax error in pattern", build(prog, func(o *Options) { o.Raw = []string{"[x"} }))
	assert.NoError(t, build(prog, func(o *Options) {
		o.Files = append(o.Files, "NINE.XA="+filepath.Join(dir, "nine.xa"))
		o.Raw = nil
	}), "with no stream names a .xa is a file like any other")

	// Not DISC=HOST.
	assert.Error(t, `"x" is not DISC=HOST`, build(prog, func(o *Options) { o.Files = []string{"x"} }))

	// An output that could not be finished is not left behind.
	_, err = os.Stat(filepath.Join(dir, "out.bin"))
	assert.NoError(t, err, "the last build succeeded")
	assert.Error(t, "is already on the disc", build(prog, func(o *Options) {
		o.Files = append(o.Files, "MAIN.EXE="+prog)
	}))
}

// TestISO writes plain blocks, which an audio track cannot go on.
func TestISO(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tree/a.txt", []byte("a"))
	opts := NewOptions()
	opts.Output = filepath.Join(dir, "out.iso")
	opts.Dir = filepath.Join(dir, "tree")
	opts.ISO = true
	opts.NoXA = true
	opts.Padding = 0
	opts.Stdout = io.Discard
	assert.NoError(t, Build(opts))

	r, closer, err := psxdisc.Open(opts.Output)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer closer.Close()
	assert.Equal(t, false, r.Volume.XA)
	assert.Equal(t, "a", string(readAll(t, r.Root.Find("A.TXT"))))
	fi, err := os.Stat(opts.Output)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), fi.Size()%iso9660.LogicalBlockSize)
	assert.Equal(t, int64(r.Volume.Blocks)*iso9660.LogicalBlockSize, fi.Size())

	opts.Audio = []string{write(t, dir, "a.raw", make([]byte, psxdisc.AudioSectorSize))}
	assert.Error(t, "cannot hold", Build(opts))
}

func TestNames(t *testing.T) {
	for _, tc := range [][3]string{
		{"game.bin", "game.bin", "game.cue"},
		{"out/game.cue", "out/game.bin", "out/game.cue"},
		{"game.CUE", "game.bin", "game.CUE"},
		{"game", "game", "game.cue"},
	} {
		bin, cue := names(tc[0])
		assert.Equal(t, tc[1], bin, tc[0])
		assert.Equal(t, tc[2], cue, tc[0])
	}
}
