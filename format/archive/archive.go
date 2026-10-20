// Package archive writes the archives the runtime package archive reads:
// one file holding many, each stored as it is or packed with deflate, for
// mkarc and the playground.
//
// The file is little-endian words. A header of four: the ID, "ARC" and a
// 1; the number of entries; the size of the whole file; and a zero. Then
// an entry of eight words each: the name, sixteen bytes padded with
// zeros, as the disc would have it; the method, 0 stored or 1 deflate;
// the size unpacked; the size as held; and the offset of its bytes from
// the start of the file, a multiple of four. The entries' bytes follow in
// the table's order, each padded to a word. A stored entry is therefore
// word aligned, and a program can use it where it lies.
//
// Deflate is the one packing, chosen over LZ4 and the LZSS of the time
// on measurements of the examples' data and of both decoders on the
// console, in docs/topics/compression.md: it packs textures and models a
// quarter to a third smaller than LZ4, and unpacks at or above the speed
// a double speed drive reads, so nothing loads slower packed. A file that
// deflate shrinks by less than a tenth, as a sound bank or a movie is, is
// stored.
package archive

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"strings"

	"go.chrisrx.dev/psx/archive"
	"go.chrisrx.dev/psx/format/iso9660"
)

// The reader's constants, for a writer's convenience.
const (
	ID         = archive.ID
	HeaderSize = archive.HeaderSize
	EntrySize  = archive.EntrySize
	NameSize   = archive.NameSize
)

// StoreUnder is the saving, as a fraction, below which a file is stored
// rather than packed: the time to unpack it would buy nothing.
const StoreUnder = 0.1

// An Entry is a file to go into an archive.
type Entry struct {
	// Name is the entry's name, which Name makes from a file's path.
	Name string
	// Data is the file's bytes.
	Data []byte
	// Store keeps the file as it is whatever it would pack to.
	Store bool
}

// Name is the entry name a file at the path gets: its base name as the
// disc would have it, in capitals, letters, digits and underscores, the
// stem cut to eight and the extension to three.
func Name(p string) string {
	return iso9660.ShortName(path.Base(strings.ReplaceAll(p, "\\", "/")))
}

// Pack writes an archive of the entries, in their order, each packed
// with deflate unless that saves less than StoreUnder of it or the entry
// says Store.
func Pack(entries []Entry) ([]byte, error) {
	seen := map[string]bool{}
	for i, e := range entries {
		switch {
		case e.Name == "":
			return nil, fmt.Errorf("entry %d has no name", i)
		case len(e.Name) >= NameSize:
			return nil, fmt.Errorf("%s: the name is longer than %d bytes", e.Name, NameSize-1)
		case strings.ContainsAny(e.Name, "/\\"):
			return nil, fmt.Errorf("%s: an archive is flat; the name cannot hold a path", e.Name)
		case seen[e.Name]:
			return nil, fmt.Errorf("%s: two entries have the name", e.Name)
		}
		seen[e.Name] = true
	}
	var out bytes.Buffer
	var hdr [HeaderSize]byte
	binary.LittleEndian.PutUint32(hdr[0:], ID)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(len(entries)))
	out.Write(hdr[:])
	table := make([]byte, EntrySize*len(entries))
	out.Write(table)
	for i, e := range entries {
		method := archive.Stored
		held := e.Data
		if !e.Store && len(e.Data) > 0 {
			var b bytes.Buffer
			w, err := flate.NewWriter(&b, flate.BestCompression)
			if err != nil {
				return nil, err
			}
			w.Write(e.Data)
			w.Close()
			if float64(len(e.Data)-b.Len()) >= StoreUnder*float64(len(e.Data)) {
				method = archive.Deflate
				held = b.Bytes()
			}
		}
		rec := table[i*EntrySize : (i+1)*EntrySize]
		copy(rec[:NameSize], e.Name)
		binary.LittleEndian.PutUint32(rec[16:], uint32(method))
		binary.LittleEndian.PutUint32(rec[20:], uint32(len(e.Data)))
		binary.LittleEndian.PutUint32(rec[24:], uint32(len(held)))
		binary.LittleEndian.PutUint32(rec[28:], uint32(out.Len()))
		out.Write(held)
		for out.Len()&3 != 0 {
			out.WriteByte(0)
		}
	}
	b := out.Bytes()
	copy(b[HeaderSize:], table)
	binary.LittleEndian.PutUint32(b[8:], uint32(len(b)))
	if _, err := archive.OpenBytes(b); err != nil {
		return nil, errors.New("archive: the archive written does not read back")
	}
	return b, nil
}
