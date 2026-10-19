package mkpsxiso

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxdisc"
)

const textAddr = 0x80010000

// A reg is one registration to write into a test executable: a disc path,
// a host path, and the size in bytes of the buffer Into gave it, or 0.
type reg struct {
	name, src string
	buf       uint32
}

// exe writes a PSX-EXE whose text holds a record for each registration,
// laid out as package disc describes, and returns its path.
func exe(t *testing.T, dir string, regs ...reg) string {
	t.Helper()
	var text []byte
	word := func(v uint32) { text = binary.LittleEndian.AppendUint32(text, v) }
	for _, reg := range regs {
		nameAddr := textAddr + uint32(len(text))
		text = append(text, reg.name...)
		srcAddr := textAddr + uint32(len(text))
		text = append(text, reg.src...)
		for len(text)%4 != 0 {
			text = append(text, 0)
		}
		word(0x44585350) // "PSXD"
		word(nameAddr)
		word(uint32(len(reg.name)))
		word(srcAddr)
		word(uint32(len(reg.src)))
		word(0)
		word(0)
		if reg.buf == 0 {
			word(0)
			word(0)
			word(0)
		} else {
			word(0x80150000)
			word(reg.buf / 4)
			word(reg.buf / 4)
		}
	}
	text = append(text, make([]byte, 2048-len(text)%2048)...)

	f := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:    psx.ExecutableSignature,
			PC0:      textAddr,
			TextAddr: textAddr,
			TextSize: uint32(len(text)),
		},
		Sections: []*psx.Section{{Name: "text", Addr: textAddr, Data: text}},
	}
	path := filepath.Join(dir, "prog.exe")
	assert.NoError(t, f.WriteFile(path))
	return path
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readAll(t *testing.T, e *iso9660.Entry) []byte {
	t.Helper()
	rc, err := e.Open()
	assert.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	assert.NoError(t, err)
	return data
}

// placeExe puts the executable in the root of a tree and returns the tree.
func placeExe(t *testing.T, root *iso9660.Dir, prog string) *iso9660.Dir {
	t.Helper()
	_, err := place(root, "MAIN.EXE", prog)
	assert.NoError(t, err)
	return root
}

// TestRegisteredFiles builds an image from an executable that registers its
// files and reads it back: the files are on the disc at their paths, and the
// executable on the disc says where.
func TestRegisteredFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "assets/wall0.tmd", "wall zero")
	write(t, dir, "assets/stone.tim", "stone")
	prog := exe(t, dir,
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "stone.tim", src: "assets/stone.tim"},
		// The same file registered again, as a second package might.
		reg{name: "data/wall0.tmd", src: "assets/wall0.tmd"},
	)

	root := placeExe(t, &iso9660.Dir{}, prog)
	p, err := loadProgram(root, "main.exe", dir)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(p.files))
	assert.Equal(t, p.files[0].entry, p.files[2].entry)

	w, err := iso9660.NewWriter(root, iso9660.Options{})
	assert.NoError(t, err)
	p.patch()
	var buf bytes.Buffer
	assert.NoError(t, w.WriteTo(psxdisc.NewISOWriter(&buf)))

	br, err := psxdisc.NewISOReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	assert.NoError(t, err)
	r, err := iso9660.NewReader(br)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r.Root.Dirs))
	data := r.Root.Dirs[0]
	assert.Equal(t, "DATA", data.Name)
	wall := data.Find("WALL0.TMD")
	stone := r.Root.Find("STONE.TIM")
	assert.Equal(t, "wall zero", string(readAll(t, wall)))
	assert.Equal(t, "stone", string(readAll(t, stone)))

	// The executable on the disc carries the patched records.
	image := readAll(t, r.Root.Find("MAIN.EXE"))[psx.HeaderSize:]
	recs := disc.Find(image, textAddr)
	assert.Equal(t, 3, len(recs))
	for i, want := range []*iso9660.Entry{wall, stone, wall} {
		sector := binary.LittleEndian.Uint32(image[recs[i].Offset+20:])
		size := binary.LittleEndian.Uint32(image[recs[i].Offset+24:])
		assert.Equal(t, want.LBA, sector, "record %d", i)
		assert.Equal(t, want.Size, size, "record %d", i)
	}
}

func TestRegisteredFileConflicts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "assets/wall0.tmd", "wall zero")
	write(t, dir, "assets/other.tmd", "other")
	write(t, dir, "disc/DATA/WALL0.TMD", "from the tree")

	// A path the tree already holds.
	prog := exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"})
	tree, err := iso9660.FromDirectory(filepath.Join(dir, "disc"))
	assert.NoError(t, err)
	_, err = loadProgram(placeExe(t, tree, prog), "MAIN.EXE", dir)
	assert.Error(t, "WALL0.TMD;1 is already on the disc", err)

	// One path from two host files.
	prog = exe(t, dir,
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "DATA/WALL0.TMD", src: "assets/other.tmd"},
	)
	_, err = loadProgram(placeExe(t, &iso9660.Dir{}, prog), "MAIN.EXE", dir)
	assert.Error(t, "from both assets/wall0.tmd and assets/other.tmd", err)

	// A host file that is not there.
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/missing.tmd"})
	_, err = loadProgram(placeExe(t, &iso9660.Dir{}, prog), "MAIN.EXE", dir)
	assert.Error(t, "missing.tmd", err)

	// A buffer the file does not fit: nine bytes need a sector, and a
	// buffer of a sector holds it, but one of half a sector does not.
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd", buf: 2048})
	_, err = loadProgram(placeExe(t, &iso9660.Dir{}, prog), "MAIN.EXE", dir)
	assert.NoError(t, err)
	prog = exe(t, dir, reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd", buf: 1024})
	_, err = loadProgram(placeExe(t, &iso9660.Dir{}, prog), "MAIN.EXE", dir)
	assert.Error(t, "assets/wall0.tmd is 9 bytes, 1 sectors, but the buffer it is read into holds 1024", err)
	// The second registration of a file is checked too.
	prog = exe(t, dir,
		reg{name: "DATA/WALL0.TMD", src: "assets/wall0.tmd"},
		reg{name: "data/wall0.tmd", src: "assets/wall0.tmd", buf: 1024},
	)
	_, err = loadProgram(placeExe(t, &iso9660.Dir{}, prog), "MAIN.EXE", dir)
	assert.Error(t, "holds 1024", err)

	// No executable of that name registers nothing, and is not an error.
	p, err := loadProgram(&iso9660.Dir{}, "MAIN.EXE", dir)
	assert.NoError(t, err)
	assert.Equal(t, (*program)(nil), p)
}

func TestPlace(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a", "a")
	host := filepath.Join(dir, "a")

	root := &iso9660.Dir{}
	e, err := place(root, "data/sound/sample1.seq", host)
	assert.NoError(t, err)
	assert.Equal(t, "SAMPLE1.SEQ;1", e.Name)
	assert.Equal(t, "SOUND", root.Dirs[0].Dirs[0].Name)

	// A second file in the same directory joins it rather than making another.
	_, err = place(root, `\DATA\SOUND\SAMPLE2.SEQ;1`, host)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(root.Dirs))
	assert.Equal(t, 1, len(root.Dirs[0].Dirs))
	assert.Equal(t, 2, len(root.Dirs[0].Dirs[0].Entries))

	_, err = place(root, "data/sound/sample1.seq", host)
	assert.Error(t, "SAMPLE1.SEQ;1 is already on the disc", err)
	_, err = place(root, "data//x", host)
	assert.Error(t, "is not a path on the disc", err)
	_, err = place(root, "", host)
	assert.Error(t, "is not a path on the disc", err)
}
