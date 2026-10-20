package iso9660

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

// A BlockReader supplies the blocks of an image.
//
// It is the counterpart of BlockWriter: the same filesystem code reads an
// image whether it came from a .bin of raw sectors or a plain .iso, and only
// the reader knows which.
type BlockReader interface {
	// ReadBlock fills b, which is LogicalBlockSize bytes, with the user
	// data of the block at lba.
	ReadBlock(lba uint32, b []byte) error

	// ReadSector fills b, which is RawSectorSize bytes, with the whole
	// Mode 2 payload at lba, subheader included. An image that does not
	// record sectors cannot do this and should say so.
	ReadSector(lba uint32, b []byte) error
}

// A Volume is what the primary volume descriptor says about an image.
type Volume struct {
	SystemID    string
	VolumeID    string
	Publisher   string
	Preparer    string
	Application string

	// Created is the volume date, and is zero when the image records none.
	Created time.Time

	// Blocks is the length of the volume in logical blocks, which for an
	// image built here is the whole data track.
	Blocks uint32

	// XA reports the CD-XA signature in the descriptor, which every
	// PlayStation disc carries.
	XA bool
}

// A Reader reads the filesystem of an image.
type Reader struct {
	br BlockReader

	// Volume describes the image.
	Volume Volume

	// Root is the directory tree, with every LBA and size filled in and
	// every Entry ready to be opened. It is the same tree a Writer takes,
	// so an image read here can be written back out.
	Root *Dir
}

// maxDepth is how deep the tree may go before the image is assumed to be
// malformed. ISO 9660 allows eight levels; the limit here is looser, and only
// exists so that a record pointing back at its own parent cannot be followed
// forever.
const maxDepth = 32

// NewReader reads the volume descriptor and the whole directory tree.
func NewReader(br BlockReader) (*Reader, error) {
	r := &Reader{br: br}

	pvd := make([]byte, LogicalBlockSize)
	if err := br.ReadBlock(PVDSector, pvd); err != nil {
		return nil, fmt.Errorf("reading the volume descriptor: %w", err)
	}
	if string(pvd[1:6]) != "CD001" {
		return nil, fmt.Errorf("no ISO 9660 volume descriptor at block %d, so this is not a disc image", PVDSector)
	}
	if pvd[0] != volumeDescriptorPrimary {
		return nil, fmt.Errorf("the descriptor at block %d is type %d, not a primary volume descriptor", PVDSector, pvd[0])
	}

	r.Volume = Volume{
		SystemID:    trimField(pvd[8:40]),
		VolumeID:    trimField(pvd[40:72]),
		Publisher:   trimField(pvd[318:446]),
		Preparer:    trimField(pvd[446:574]),
		Application: trimField(pvd[574:702]),
		Created:     parseDecDateTime(pvd[813:830]),
		Blocks:      binary.LittleEndian.Uint32(pvd[80:]),
		XA:          string(pvd[xaSignatureOffset:xaSignatureOffset+8]) == "CD-XA001",
	}

	// The root's own record is stored in the descriptor, and everything
	// else hangs off it.
	root := parseDirRecord(pvd[156:190])
	if root == nil {
		return nil, fmt.Errorf("the volume descriptor holds no root directory record")
	}
	r.Root = &Dir{LBA: root.lba, Size: root.size, Modified: root.mod}
	if err := r.readDir(r.Root, 0, map[uint32]bool{r.Root.LBA: true}); err != nil {
		return nil, err
	}
	return r, nil
}

// readDir fills in one directory from its extent, and then its children.
func (r *Reader) readDir(d *Dir, depth int, seen map[uint32]bool) error {
	if depth > maxDepth {
		return fmt.Errorf("directory tree is more than %d levels deep, so a record points back into it", maxDepth)
	}
	// A directory's extent is a whole number of blocks, and one that claims
	// to be longer than the volume is not a directory at all. Checking
	// before allocating keeps a malformed image from asking for the length
	// field as memory.
	if d.Size == 0 || d.Size%LogicalBlockSize != 0 {
		return fmt.Errorf("directory at block %d says it is %d bytes, which is not a whole number of blocks", d.LBA, d.Size)
	}
	if end := uint64(d.LBA) + uint64(blocks(d.Size)); r.Volume.Blocks != 0 && end > uint64(r.Volume.Blocks) {
		return fmt.Errorf("directory at block %d runs to block %d, past the %d the volume holds", d.LBA, end, r.Volume.Blocks)
	}

	extent := make([]byte, d.Size)
	for i := range blocks(d.Size) {
		if err := r.br.ReadBlock(d.LBA+i, extent[i*LogicalBlockSize:(i+1)*LogicalBlockSize]); err != nil {
			return fmt.Errorf("reading directory at block %d: %w", d.LBA, err)
		}
	}

	for off := uint32(0); off < d.Size; {
		// A zero length byte means the rest of the block is the padding
		// that keeps a record from straddling a boundary.
		if extent[off] == 0 {
			off += LogicalBlockSize - off%LogicalBlockSize
			continue
		}
		n := uint32(extent[off])
		if off%LogicalBlockSize+n > LogicalBlockSize || off+n > uint32(len(extent)) {
			return fmt.Errorf("directory at block %d has a %d byte record at offset %d, which runs past its block",
				d.LBA, n, off)
		}
		rec := parseDirRecord(extent[off : off+n])
		off += n
		if rec == nil {
			continue
		}

		switch rec.id {
		case "\x00", "\x01":
			// "." and ".." are rebuilt from the tree itself.
			continue
		}

		if rec.isDir {
			if seen[rec.lba] {
				return fmt.Errorf("directory %s points at block %d, which is already in the tree", rec.id, rec.lba)
			}
			seen[rec.lba] = true
			sub := &Dir{
				Name:     rec.id,
				LBA:      rec.lba,
				Size:     rec.size,
				Modified: rec.mod,
				parent:   d,
			}
			d.Dirs = append(d.Dirs, sub)
			if err := r.readDir(sub, depth+1, seen); err != nil {
				return err
			}
			continue
		}

		e := &Entry{
			Name:     rec.id,
			Size:     rec.size,
			Modified: rec.mod,
			LBA:      rec.lba,
			Raw:      rec.raw,
		}
		if e.Raw {
			// The record counts logical blocks even for a stream whose
			// sectors hold 2336 bytes, so the length of the file is the
			// extent it occupies measured in those.
			e.Size = (rec.size / LogicalBlockSize) * RawSectorSize
		}
		e.Open = r.opener(e)
		d.Entries = append(d.Entries, e)
	}
	return nil
}

// opener returns an Open function reading the entry out of the image, so that
// a tree read here can be handed straight back to a Writer.
func (r *Reader) opener(e *Entry) func() (io.ReadCloser, error) {
	lba, size, raw := e.LBA, e.Size, e.Raw
	return func() (io.ReadCloser, error) {
		return &extentReader{br: r.br, lba: lba, remaining: size, raw: raw}, nil
	}
}

// An extentReader reads a file's contents out of an image a sector at a time,
// so that a large file is never held in memory.
type extentReader struct {
	br        BlockReader
	lba       uint32
	remaining uint32
	raw       bool

	buf  []byte
	pos  int
	fill int
}

func (e *extentReader) Read(p []byte) (int, error) {
	if e.pos >= e.fill {
		if e.remaining == 0 {
			return 0, io.EOF
		}
		unit := uint32(LogicalBlockSize)
		read := e.br.ReadBlock
		if e.raw {
			unit = RawSectorSize
			read = e.br.ReadSector
		}
		if e.buf == nil {
			e.buf = make([]byte, unit)
		}
		if err := read(e.lba, e.buf); err != nil {
			return 0, err
		}
		e.lba++
		// The last block of a file is only partly the file's, so the
		// padding the writer put there is not handed back.
		e.fill = int(min(unit, e.remaining))
		e.remaining -= uint32(e.fill)
		e.pos = 0
	}
	n := copy(p, e.buf[e.pos:e.fill])
	e.pos += n
	return n, nil
}

func (e *extentReader) Close() error { return nil }

// A parsedRecord is one directory record as read back.
type parsedRecord struct {
	id    string
	lba   uint32
	size  uint32
	isDir bool
	raw   bool
	mod   time.Time
}

// parseDirRecord reads one directory record. It returns nil for a record too
// short to hold its own fields, which is how the padding at the end of a
// block appears.
func parseDirRecord(b []byte) *parsedRecord {
	if len(b) < DirRecordFixedLen+1 {
		return nil
	}
	idLen := int(b[32])
	if DirRecordFixedLen+idLen > len(b) {
		return nil
	}
	r := &parsedRecord{
		id:    string(b[33 : 33+idLen]),
		lba:   binary.LittleEndian.Uint32(b[2:]),
		size:  binary.LittleEndian.Uint32(b[10:]),
		isDir: b[25]&fileFlagDirectory != 0,
		mod:   parseDirDateTime(b[18:25]),
	}

	// What is left over after the identifier is the system use area, and
	// the XA block in it is the only thing on the disc that says a file
	// holds streamed sectors rather than a byte stream. Without it a stream
	// is indistinguishable from data, which is the reason the extension
	// exists.
	sys := b[recordLen(idLen, 0):]
	if len(sys) >= xaSysUseLen && string(sys[6:8]) == "XA" {
		attr := uint16(sys[4])<<8 | uint16(sys[5])
		r.raw = attr&xaForm2 != 0
	}
	return r
}

// trimField reads one of the descriptor's space padded text fields.
func trimField(b []byte) string {
	return strings.TrimRight(string(b), " \x00")
}

// parseDirDateTime reads the compact 7-byte form in a directory record. An
// all-zero field means the date was not recorded.
func parseDirDateTime(b []byte) time.Time {
	if b[0] == 0 && b[1] == 0 && b[2] == 0 {
		return time.Time{}
	}
	return time.Date(1900+int(b[0]), time.Month(b[1]), int(b[2]),
		int(b[3]), int(b[4]), int(b[5]), 0, zoneOf(b[6]))
}

// parseDecDateTime reads the 17-byte digits form used for volume dates. The
// digits are local time and the last byte says which local, so the two
// together are an instant.
func parseDecDateTime(b []byte) time.Time {
	s := string(b[:16])
	if s == "0000000000000000" || strings.TrimSpace(s) == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation("20060102150405", s[:14], zoneOf(b[16]))
	if err != nil {
		return time.Time{}
	}
	return t
}

// zoneOf reads the offset from GMT that both date forms end with, counted in
// 15 minute intervals and signed.
func zoneOf(b byte) *time.Location {
	return time.FixedZone("", int(int8(b))*15*60)
}
