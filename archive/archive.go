// Package archive reads archives: one file on the disc holding many, each
// stored as it is or packed with deflate, which the program unpacks into
// memory of its own as it needs them. A program registers the archive
// with package disc, loads it whole, and opens it:
//
//	var level = disc.Register("ASSETS/LEVEL.ARC", "assets/LEVEL.ARC")
//
//	a, err := archive.Open(level.Data())
//	e, ok := a.Lookup("CUBE.TMD")
//	words, err := e.Unpack(buf[:])
//
// An entry stored as it is can be used where it lies, without a copy,
// through Data, which is how a texture or a model is best kept when the
// archive stays in memory. Entries are packed one by one, so a program
// unpacks only what it wants, when it wants it: Terra Incognita's
// pattern, three archives resident and pieces unpacked as the game needed
// them.
//
// The format is mkarc's, described in package format/archive, which writes
// it. The deflate here decodes at about a third to a whole megabyte a
// second on the console, at or above what a double speed drive reads, so
// a packed file costs no more time to load than the raw one and takes a
// third to a half of the space. docs/topics/compression.md has the
// measurements the choice was made on.
package archive

import (
	"errors"
	"unsafe"
)

// ID is the first word of an archive: "ARC" and a 1.
const ID = 0x01435241

// Method is how an entry is held.
type Method uint32

const (
	// Stored entries are their file's bytes as they are, word aligned.
	Stored Method = 0
	// Deflate entries are a raw deflate stream (RFC 1951) of the file.
	Deflate Method = 1
)

// The layout: a header of four words, then an entry of eight words each,
// then the entries' bytes, each starting on a word.
const (
	HeaderSize = 16
	EntrySize  = 32
	NameSize   = 16
)

var (
	ErrFormat  = errors.New("archive: not an archive")
	ErrCorrupt = errors.New("archive: the entry's packed bytes do not unpack")
	ErrSpace   = errors.New("archive: the destination is too small for the entry")
	ErrMethod  = errors.New("archive: the entry is packed in a way this program cannot read")
)

// An Archive is an archive opened in memory.
type Archive struct {
	data []byte
	n    int
}

// Open opens an archive from the words package disc loaded it into. The
// words are kept, not copied: the entries point into them.
func Open(words []uint32) (Archive, error) {
	if len(words) == 0 {
		return Archive{}, ErrFormat
	}
	return OpenBytes(unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), 4*len(words)))
}

// OpenBytes opens an archive held in bytes, as a host program has it.
// Data on a stored entry needs the bytes word aligned, as the disc's are.
func OpenBytes(b []byte) (Archive, error) {
	if len(b) < HeaderSize || u32(b, 0) != ID {
		return Archive{}, ErrFormat
	}
	n := int(u32(b, 4))
	size := int(u32(b, 8))
	if n < 0 || size < HeaderSize+n*EntrySize || size > len(b) {
		return Archive{}, ErrFormat
	}
	a := Archive{data: b[:size], n: n}
	for i := 0; i < n; i++ {
		e := a.At(i)
		off := int(u32(b, HeaderSize+i*EntrySize+28))
		if off&3 != 0 || off < HeaderSize+n*EntrySize || off+len(e.packed) > size || (e.Method == Stored && len(e.packed) != e.Size) {
			return Archive{}, ErrFormat
		}
	}
	return a, nil
}

func u32(b []byte, off int) uint32 {
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

// Len is how many entries the archive holds.
func (a Archive) Len() int { return a.n }

// Size is the archive's bytes, header, table and entries together.
func (a Archive) Size() int { return len(a.data) }

// An Entry is one file in an archive.
type Entry struct {
	// Name is the file's name as the disc would have it: capitals, up to
	// eight and three.
	Name string
	// Method is how the entry is held.
	Method Method
	// Size is the file's size unpacked.
	Size   int
	packed []byte
}

// At returns entry i, which must be less than Len.
func (a Archive) At(i int) Entry {
	rec := a.data[HeaderSize+i*EntrySize : HeaderSize+(i+1)*EntrySize]
	n := 0
	for n < NameSize && rec[n] != 0 {
		n++
	}
	var name string
	if n > 0 {
		name = unsafe.String(&rec[0], n)
	}
	off := int(u32(rec, 28))
	packed := int(u32(rec, 24))
	return Entry{
		Name:   name,
		Method: Method(u32(rec, 16)),
		Size:   int(u32(rec, 20)),
		packed: a.data[off : off+packed],
	}
}

// Lookup finds the entry with the name, as the disc would have it.
func (a Archive) Lookup(name string) (Entry, bool) {
	for i := 0; i < a.n; i++ {
		if e := a.At(i); e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Packed is the entry's bytes as the archive holds them.
func (e Entry) Packed() []byte { return e.packed }

// Words is how many words Unpack needs for the entry.
func (e Entry) Words() int { return (e.Size + 3) / 4 }

// Data is a stored entry's bytes where they lie in the archive, as words,
// with no copy: a TIM or a TMD the library can take as it is while the
// archive stays in memory. It is nil for a packed entry, which needs
// Unpack, and for an archive whose bytes are not word aligned.
func (e Entry) Data() []uint32 {
	if e.Method != Stored || e.Size == 0 || uintptr(unsafe.Pointer(&e.packed[0]))&3 != 0 {
		return nil
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(&e.packed[0])), e.Words())
}

// Unpack writes the entry's file into dst, which needs Words words, and
// returns the words it filled.
func (e Entry) Unpack(dst []uint32) ([]uint32, error) {
	if len(dst) < e.Words() {
		return nil, ErrSpace
	}
	if e.Size == 0 {
		return dst[:0], nil
	}
	out := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), e.Size)
	switch e.Method {
	case Stored:
		copy(out, e.packed)
	case Deflate:
		if Inflate(out, e.packed) != e.Size {
			return nil, ErrCorrupt
		}
	default:
		return nil, ErrMethod
	}
	return dst[:e.Words()], nil
}

// Load unpacks the entry into new memory.
func (e Entry) Load() ([]uint32, error) {
	return e.Unpack(make([]uint32, e.Words()))
}
