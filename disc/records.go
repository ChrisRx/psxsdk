//go:build !yaroze

package disc

// The host half of the package: finding the records Register left in an
// executable, and filling them in.

import (
	"encoding/binary"
)

// A Record is one registration found in a program's image: what Register
// was called with, and where the File lies so that it can be patched.
type Record struct {
	// Name is the path on the disc, as the program wrote it.
	Name string

	// Src is the host file, as the program wrote it, relative to whatever
	// directory the caller takes the program's files to be in.
	Src string

	// Offset is where the record begins in the image it was found in.
	Offset int
}

// Find returns the registrations in image, which is the loadable part of a
// program with its first byte at address base: for a PSX-EXE, everything
// after the 2048-byte header, at the header's text address.
//
// There is no table of records to go to, since each is wherever the compiler
// put the variable it initializes, so the image is scanned for the magic word
// at every word boundary. A match is only taken for a record when the two
// strings it points at lie inside the image and look like paths, which no
// instruction or other data is going to satisfy by accident. Records are
// returned in the order they appear.
func Find(image []byte, base uint32) []Record {
	var out []Record
	for off := 0; off+recordSize <= len(image); off += 4 {
		if binary.LittleEndian.Uint32(image[off:]) != magic {
			continue
		}
		name, ok := str(image, base, image[off+offName:], maxName)
		if !ok {
			continue
		}
		src, ok := str(image, base, image[off+offSrc:], maxSrc)
		if !ok {
			continue
		}
		out = append(out, Record{Name: name, Src: src, Offset: off})
		off += recordSize - 4
	}
	return out
}

// Lengths past which a string is taken for something other than a path. A
// disc path is bounded by ISO 9660 itself; a host path is not, but one longer
// than this is a stray match rather than a file anyone named.
const (
	maxName = 255
	maxSrc  = 1024
)

// str reads the string whose pointer and length are at b, if it lies within
// the image and consists of printable ASCII.
func str(image []byte, base uint32, b []byte, limit int) (string, bool) {
	ptr := binary.LittleEndian.Uint32(b)
	n := binary.LittleEndian.Uint32(b[4:])
	if n == 0 || n > uint32(limit) || ptr < base {
		return "", false
	}
	start := uint64(ptr - base)
	if start+uint64(n) > uint64(len(image)) {
		return "", false
	}
	s := image[start : start+uint64(n)]
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return "", false
		}
	}
	return string(s), true
}

// Patch writes into image, the one r was found in, the sector the file was
// placed at and its length, where the program's File reads them.
func (r Record) Patch(image []byte, sector, size uint32) {
	binary.LittleEndian.PutUint32(image[r.Offset+offSector:], sector)
	binary.LittleEndian.PutUint32(image[r.Offset+offSize:], size)
}
