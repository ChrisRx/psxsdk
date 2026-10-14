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

	// LBA is filled in during layout.
	LBA uint32
}

// A Dir is a directory on the disc.
type Dir struct {
	// Name is the identifier as it appears on the disc. It is empty for
	// the root.
	Name string

	Dirs    []*Dir
	Entries []*Entry

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
		Name: FileIdentifier(name),
		Size: uint32(fi.Size()),
		Open: func() (io.ReadCloser, error) { return os.Open(path) },
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
// given length. Records are padded to an even length.
func recordLen(idLen int) uint32 {
	n := DirRecordFixedLen + idLen
	if n%2 != 0 {
		n++
	}
	return uint32(n)
}

// contentSize computes how many bytes of directory records a directory needs,
// including the "." and ".." entries that begin it.
//
// A record may not straddle a logical block boundary, so a record that would
// not fit in what remains of the current block starts the next one instead.
// That padding has to be accounted for here or the computed size will be too
// small and the records will overrun into whatever follows.
func (d *Dir) contentSize() uint32 {
	var total, used uint32
	add := func(n uint32) {
		if used+n > LogicalBlockSize {
			total += LogicalBlockSize - used
			used = 0
		}
		used += n
		total += n
	}
	add(recordLen(1)) // "."
	add(recordLen(1)) // ".."
	for _, sub := range d.Dirs {
		add(recordLen(len(sub.Name)))
	}
	for _, e := range d.Entries {
		add(recordLen(len(e.Name)))
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

// writeDirRecord encodes one directory record into b and returns its length.
func writeDirRecord(b []byte, id string, lba, size uint32, isDir bool, t time.Time) uint32 {
	n := recordLen(len(id))
	clear(b[:n])

	b[0] = byte(n)
	b[1] = 0 // no extended attribute record
	putBoth32(b[2:], lba)
	putBoth32(b[10:], size)
	putDirDateTime(b[18:], t)
	if isDir {
		b[25] = fileFlagDirectory
	}
	b[26] = 0 // file unit size, zero when not interleaved
	b[27] = 0 // interleave gap size
	putBoth16(b[28:], 1)
	b[32] = byte(len(id))
	copy(b[33:], id)
	return n
}
