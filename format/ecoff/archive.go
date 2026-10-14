package ecoff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ArchiveMagic is the global header of a Unix ar archive.
const ArchiveMagic = "!<arch>\n"

// archiveHeaderSize is the size in bytes of a per-member ar header.
const archiveHeaderSize = 60

// ErrNotArchive is returned when a file does not begin with ArchiveMagic.
var ErrNotArchive = errors.New("not an ar archive")

// An ArchiveMember is a single named object within an archive.
type ArchiveMember struct {
	Name string

	// Data is the raw member contents.
	Data []byte

	// File is the parsed ECOFF object, or nil for members that are not
	// ECOFF objects (such as the archive symbol table).
	File *File
}

// An Archive represents a Unix ar archive of ECOFF objects.
type Archive struct {
	Members []*ArchiveMember

	closer io.Closer
}

// OpenArchive opens the named ar archive.
func OpenArchive(name string) (*Archive, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	a, err := NewArchive(data)
	if err != nil {
		f.Close()
		return nil, err
	}
	a.closer = f
	return a, nil
}

// NewArchive parses an ar archive from data.
//
// Members that cannot be parsed as ECOFF objects are retained with a nil File,
// which covers the archive symbol table. The Net Yaroze libps.a names its
// symbol table member "__________ELEL_" rather than the more usual "/" or
// "__.SYMDEF", so members are classified by content and not by name.
func NewArchive(data []byte) (*Archive, error) {
	if !bytes.HasPrefix(data, []byte(ArchiveMagic)) {
		return nil, ErrNotArchive
	}
	a := new(Archive)
	for off := len(ArchiveMagic); off+archiveHeaderSize <= len(data); {
		hdr := data[off : off+archiveHeaderSize]
		size, err := strconv.Atoi(strings.TrimSpace(string(hdr[48:58])))
		if err != nil {
			return nil, fmt.Errorf("invalid member size at offset %d: %v", off, err)
		}
		start := off + archiveHeaderSize
		if size < 0 || start+size > len(data) {
			return nil, fmt.Errorf("member at offset %d extends past end of archive", off)
		}
		m := &ArchiveMember{
			Name: strings.TrimSuffix(strings.TrimSpace(string(hdr[0:16])), "/"),
			Data: data[start : start+size],
		}
		// A member that is not an ECOFF object is kept as raw data. Only
		// NewFile can tell the difference, so a parse failure is not fatal.
		if fi, err := NewFile(bytes.NewReader(m.Data)); err == nil {
			m.File = fi
		}
		a.Members = append(a.Members, m)
		// Members are padded to an even offset.
		off = start + size + size%2
	}
	return a, nil
}

// Objects returns only the members that parsed as ECOFF objects.
func (a *Archive) Objects() []*ArchiveMember {
	members := make([]*ArchiveMember, 0, len(a.Members))
	for _, m := range a.Members {
		if m.File != nil {
			members = append(members, m)
		}
	}
	return members
}

func (a *Archive) Close() error {
	var err error
	if a.closer != nil {
		err = a.closer.Close()
		a.closer = nil
	}
	return err
}
