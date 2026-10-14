package iso9660

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// An Entry is a file on the disc. Contents are supplied by Open so that a
// large file never has to be held in memory.
type Entry struct {
	// Name is the identifier as it appears on the disc, including the
	// ";1" version suffix.
	Name string

	// Size is the length of the file in bytes.
	Size uint32

	// Open returns the file's contents.
	Open func() (io.ReadCloser, error)

	// Modified is the file's modification time. It is recorded only when
	// Options.FileTimes is set, so that an image is reproducible from the
	// tree alone by default.
	Modified time.Time

	// Raw marks a file whose contents are whole Mode 2 sector payloads of
	// RawSectorSize bytes each, rather than a byte stream to be packed into
	// logical blocks. XA audio and STR video are produced in that form,
	// each sector carrying the subheader that says which form it is and
	// which interleaved channel it belongs to, so the data has to be
	// recorded as it stands.
	//
	// Such a file needs a BlockWriter that can record raw sectors. Writing
	// one to a plain .iso of 2048-byte blocks is not possible and is an
	// error rather than a silent truncation.
	Raw bool

	// LBA is filled in during layout.
	LBA uint32
}

// sectors is the number of disc sectors the contents occupy. A zero length
// file still takes one, so that its recorded address points at something
// real.
func (e *Entry) sectors() uint32 {
	unit := uint32(LogicalBlockSize)
	if e.Raw {
		unit = RawSectorSize
	}
	return max((e.Size+unit-1)/unit, 1)
}

// recordedSize is the length to state in the directory record.
//
// For a raw file this is not the length of the file: its sectors hold 2336
// bytes each, but a directory record counts 2048-byte logical blocks, and a
// reader that divided a 2336-byte-per-sector length by 2048 would ask for
// more sectors than the file has. So the record states the extent the file
// occupies and a program reading the stream takes the rest from the
// subheaders, which is the convention PlayStation discs use.
func (e *Entry) recordedSize() uint32 {
	if e.Raw {
		return e.sectors() * LogicalBlockSize
	}
	return e.Size
}

// A Dir is a directory on the disc.
type Dir struct {
	// Name is the identifier as it appears on the disc. It is empty for
	// the root.
	Name string

	Dirs    []*Dir
	Entries []*Entry

	// Modified is the directory's modification time, recorded under the
	// same condition as Entry.Modified.
	Modified time.Time

	// Filled in during layout.
	LBA    uint32
	Size   uint32
	parent *Dir
	number uint16 // 1-based index in the path table
}

// AddFile appends a file backed by a path on the host filesystem.
func (d *Dir) AddFile(name, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Size() > 0xffffffff {
		return fmt.Errorf("%s: %d bytes is larger than a disc file can be", path, fi.Size())
	}
	d.Entries = append(d.Entries, &Entry{
		Name:     FileIdentifier(name),
		Size:     uint32(fi.Size()),
		Open:     func() (io.ReadCloser, error) { return os.Open(path) },
		Modified: fi.ModTime(),
	})
	return nil
}

// AddDir appends and returns a subdirectory.
func (d *Dir) AddDir(name string) *Dir {
	sub := &Dir{Name: NormalizeName(name), parent: d}
	d.Dirs = append(d.Dirs, sub)
	return sub
}

// FromDirectory builds a disc tree mirroring a directory on the host
// filesystem. This is the common case: a program plus the data it loads, laid
// out on the disc the same way it is laid out in the source tree.
func FromDirectory(root string) (*Dir, error) {
	d := &Dir{}
	if err := addTree(d, root); err != nil {
		return nil, err
	}
	return d, nil
}

func addTree(d *Dir, path string) error {
	ents, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		d.Modified = fi.ModTime()
	}
	// Read order is already sorted by name, but the disc requires it, so be
	// explicit rather than relying on it.
	slices.SortFunc(ents, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	for _, e := range ents {
		full := filepath.Join(path, e.Name())
		switch {
		case e.IsDir():
			if err := addTree(d.AddDir(e.Name()), full); err != nil {
				return err
			}
		case e.Type().IsRegular():
			if err := d.AddFile(e.Name(), full); err != nil {
				return err
			}
		default:
			// Symlinks, devices and sockets have no meaning on a disc.
			return fmt.Errorf("%s: not a regular file or directory", full)
		}
	}
	return nil
}

// walk visits the tree breadth first, which is the order the path table
// requires: every directory appears after its parent, so a parent's table
// index is always known by the time a child needs it.
func (d *Dir) walk() []*Dir {
	out := []*Dir{d}
	for i := 0; i < len(out); i++ {
		cur := out[i]
		// Sorting keeps the path table and directory records in the order
		// the standard requires, and makes output reproducible.
		slices.SortFunc(cur.Dirs, func(a, b *Dir) int {
			return strings.Compare(a.Name, b.Name)
		})
		slices.SortFunc(cur.Entries, func(a, b *Entry) int {
			return strings.Compare(a.Name, b.Name)
		})
		out = append(out, cur.Dirs...)
	}
	for i, dir := range out {
		dir.number = uint16(i + 1)
	}
	return out
}

// recordLen returns the size of a directory record for an identifier of the
// given length, with sysUse bytes of system use area after it. The identifier
// is followed by a padding byte when it would otherwise leave the record an
// odd number of bytes long, so that the system use area starts on an even
// boundary and the record as a whole is even.
func recordLen(idLen int, sysUse uint32) uint32 {
	n := DirRecordFixedLen + idLen
	if n%2 != 0 {
		n++
	}
	return uint32(n) + sysUse
}

// contentSize computes how many bytes of directory records a directory needs,
// including the "." and ".." entries that begin it.
//
// A record may not straddle a logical block boundary, so a record that would
// not fit in what remains of the current block starts the next one instead.
// That padding has to be accounted for here or the computed size will be too
// small and the records will overrun into whatever follows.
func (d *Dir) contentSize(sysUse uint32) uint32 {
	var total, used uint32
	add := func(n uint32) {
		if used+n > LogicalBlockSize {
			total += LogicalBlockSize - used
			used = 0
		}
		used += n
		total += n
	}
	add(recordLen(1, sysUse)) // "."
	add(recordLen(1, sysUse)) // ".."
	for _, sub := range d.Dirs {
		add(recordLen(len(sub.Name), sysUse))
	}
	for _, e := range d.Entries {
		add(recordLen(len(e.Name), sysUse))
	}
	return total
}

// pathTableSize returns the size in bytes of one path table.
func pathTableSize(dirs []*Dir) uint32 {
	var n uint32
	for _, d := range dirs {
		idLen := len(d.Name)
		if idLen == 0 {
			idLen = 1 // the root is recorded as a single zero byte
		}
		size := uint32(8 + idLen)
		if size%2 != 0 {
			size++
		}
		n += size
	}
	return n
}

// A record is the directory entry to encode for one file or directory.
type record struct {
	id    string
	lba   uint32
	size  uint32
	isDir bool
	// raw marks a file of Mode 2 sector payloads, which the XA attributes
	// report as Form 2.
	raw bool
	// mod is the modification time to record, already resolved against
	// Options.Created and Options.FileTimes by the caller.
	mod time.Time
	// sysUse is the length of the system use area to append: xaSysUseLen
	// for an XA disc, zero for plain ISO 9660.
	sysUse uint32
}

// writeDirRecord encodes one directory record into b and returns its length.
func writeDirRecord(b []byte, r record) uint32 {
	id := r.id
	n := recordLen(len(id), r.sysUse)
	clear(b[:n])

	b[0] = byte(n)
	b[1] = 0 // no extended attribute record
	putBoth32(b[2:], r.lba)
	putBoth32(b[10:], r.size)
	putDirDateTime(b[18:], r.mod)
	if r.isDir {
		b[25] = fileFlagDirectory
	}
	b[26] = 0 // file unit size, zero when not interleaved
	b[27] = 0 // interleave gap size
	putBoth16(b[28:], 1)
	b[32] = byte(len(id))
	copy(b[33:], id)

	if r.sysUse == xaSysUseLen {
		writeXA(b[n-xaSysUseLen:n], xaAttributes(r))
	}
	return n
}

// xaAttributes is what a record claims about its contents in the XA
// attributes field.
func xaAttributes(r record) uint16 {
	switch {
	case r.isDir:
		return xaAttrDir
	case r.raw:
		// A raw file's sectors are the streamed kind, which is what Form 2
		// means here. The claim is about the file as a whole; a reader
		// handling the stream goes by each sector's own subheader, which is
		// the only thing that can describe a file holding both forms.
		return xaAttrForm2
	default:
		return xaAttrFile
	}
}
