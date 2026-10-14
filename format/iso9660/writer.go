package iso9660

import (
	"fmt"
	"io"
	"time"
)

// A BlockWriter receives the 2048-byte logical blocks of the image, in
// ascending order with no gaps.
//
// Keeping emission behind this interface is what lets the same filesystem be
// written either as a plain .iso of 2048-byte blocks or as the raw 2352-byte
// sectors a PlayStation disc actually uses, without the filesystem code
// knowing which.
type BlockWriter interface {
	WriteBlock(lba uint32, data []byte) error
}

// Options configure the volume descriptor.
type Options struct {
	// VolumeID is the disc label.
	VolumeID string

	// SystemID identifies the target system. PlayStation discs use
	// "PLAYSTATION", and the console's own boot code does not care, but
	// tools that inspect the image do.
	SystemID string

	Publisher   string
	Preparer    string
	Application string

	// Created is the volume timestamp. Leaving it zero produces a
	// reproducible image.
	Created time.Time

	// SystemArea is written to the first 32KB. On a real PlayStation this
	// is where the licence data goes; an emulator does not need it.
	SystemArea []byte
}

func (o *Options) defaults() {
	if o.SystemID == "" {
		o.SystemID = "PLAYSTATION"
	}
	if o.VolumeID == "" {
		o.VolumeID = "UNTITLED"
	}
}

// A Writer lays out and emits an ISO 9660 image.
type Writer struct {
	root *Dir
	opts Options

	dirs []*Dir

	lPathTableLBA uint32
	mPathTableLBA uint32
	pathTableLen  uint32
	totalBlocks   uint32
}

// NewWriter assigns addresses to every directory and file in the tree. After
// it returns, each Dir and Entry knows its LBA, so a caller can report the
// layout or build a SYSTEM.CNF that refers to it.
func NewWriter(root *Dir, opts Options) (*Writer, error) {
	opts.defaults()
	if len(opts.SystemArea) > SystemAreaSectors*LogicalBlockSize {
		return nil, fmt.Errorf("system area is %d bytes, more than the %d available",
			len(opts.SystemArea), SystemAreaSectors*LogicalBlockSize)
	}

	w := &Writer{root: root, opts: opts}
	w.dirs = root.walk()

	// The volume descriptors occupy the two sectors after the system area,
	// then the path tables, then directory records, then file data. Nothing
	// requires this order, but it is the conventional one and it keeps the
	// structural data at the start of the disc where seeks are cheapest.
	lba := uint32(PVDSector) + 2

	w.pathTableLen = pathTableSize(w.dirs)
	ptBlocks := blocks(w.pathTableLen)
	w.lPathTableLBA = lba
	lba += ptBlocks
	w.mPathTableLBA = lba
	lba += ptBlocks

	for _, d := range w.dirs {
		d.Size = d.contentSize()
		d.LBA = lba
		lba += blocks(d.Size)
	}
	for _, d := range w.dirs {
		for _, e := range d.Entries {
			e.LBA = lba
			// A zero length file still occupies a block, so that its
			// recorded address points at something real.
			lba += max(blocks(e.Size), 1)
		}
	}

	w.totalBlocks = lba
	return w, nil
}

// TotalBlocks is the size of the image in logical blocks.
func (w *Writer) TotalBlocks() uint32 { return w.totalBlocks }

// WriteTo emits the image, one logical block at a time and in order.
func (w *Writer) WriteTo(bw BlockWriter) error {
	block := make([]byte, LogicalBlockSize)
	lba := uint32(0)

	emit := func(data []byte) error {
		if err := bw.WriteBlock(lba, data); err != nil {
			return fmt.Errorf("writing block %d: %w", lba, err)
		}
		lba++
		return nil
	}

	// System area.
	for range SystemAreaSectors {
		clear(block)
		if off := int(lba) * LogicalBlockSize; off < len(w.opts.SystemArea) {
			copy(block, w.opts.SystemArea[off:])
		}
		if err := emit(block); err != nil {
			return err
		}
	}

	clear(block)
	w.writePVD(block)
	if err := emit(block); err != nil {
		return err
	}

	clear(block)
	block[0] = volumeDescriptorTerminator
	copy(block[1:], "CD001")
	block[6] = 1
	if err := emit(block); err != nil {
		return err
	}

	for _, bigEndian := range []bool{false, true} {
		table := w.pathTable(bigEndian)
		for len(table) > 0 {
			clear(block)
			n := copy(block, table)
			table = table[n:]
			if err := emit(block); err != nil {
				return err
			}
		}
	}

	for _, d := range w.dirs {
		records := w.dirRecords(d)
		for len(records) > 0 {
			clear(block)
			n := copy(block, records)
			records = records[n:]
			if err := emit(block); err != nil {
				return err
			}
		}
	}

	for _, d := range w.dirs {
		for _, e := range d.Entries {
			if err := w.writeFile(bw, &lba, e); err != nil {
				return err
			}
		}
	}

	if lba != w.totalBlocks {
		return fmt.Errorf("wrote %d blocks but laid out %d", lba, w.totalBlocks)
	}
	return nil
}

func (w *Writer) writeFile(bw BlockWriter, lba *uint32, e *Entry) error {
	rc, err := e.Open()
	if err != nil {
		return fmt.Errorf("opening %s: %w", e.Name, err)
	}
	defer rc.Close()

	block := make([]byte, LogicalBlockSize)
	remaining := max(blocks(e.Size), 1)
	for range remaining {
		clear(block)
		if _, err := io.ReadFull(rc, block); err != nil &&
			err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("reading %s: %w", e.Name, err)
		}
		if err := bw.WriteBlock(*lba, block); err != nil {
			return fmt.Errorf("writing %s at block %d: %w", e.Name, *lba, err)
		}
		*lba++
	}
	return nil
}

// writePVD fills in the primary volume descriptor.
func (w *Writer) writePVD(b []byte) {
	b[0] = volumeDescriptorPrimary
	copy(b[1:], "CD001")
	b[6] = 1

	padString(b[8:40], w.opts.SystemID)
	padString(b[40:72], w.opts.VolumeID)
	putBoth32(b[80:], w.totalBlocks)
	putBoth16(b[120:], 1) // volume set size
	putBoth16(b[124:], 1) // volume sequence number
	putBoth16(b[128:], LogicalBlockSize)
	putBoth32(b[132:], w.pathTableLen)
	putLE32(b[140:], w.lPathTableLBA)
	putLE32(b[144:], 0) // no optional L path table
	putBE32(b[148:], w.mPathTableLBA)
	putBE32(b[152:], 0) // no optional M path table

	// The root directory record is stored inline rather than referenced.
	writeDirRecord(b[156:190], "\x00", w.root.LBA, w.root.Size, true, w.opts.Created)

	padString(b[190:318], "")
	padString(b[318:446], w.opts.Publisher)
	padString(b[446:574], w.opts.Preparer)
	padString(b[574:702], w.opts.Application)
	padString(b[702:739], "")
	padString(b[739:776], "")
	padString(b[776:813], "")

	putDecDateTime(b[813:830], w.opts.Created)
	putDecDateTime(b[830:847], w.opts.Created)
	putDecDateTime(b[847:864], time.Time{}) // never expires
	putDecDateTime(b[864:881], w.opts.Created)

	b[881] = 1 // file structure version
}

// pathTable renders the flat directory index, in either byte order.
func (w *Writer) pathTable(bigEndian bool) []byte {
	out := make([]byte, 0, w.pathTableLen)
	for _, d := range w.dirs {
		id := d.Name
		if id == "" {
			id = "\x00" // the root's identifier is a single zero byte
		}
		rec := make([]byte, 8+len(id))
		rec[0] = byte(len(id))
		rec[1] = 0
		parent := d.number
		if d.parent != nil {
			parent = d.parent.number
		}
		if bigEndian {
			putBE32(rec[2:], d.LBA)
			putBE16(rec[6:], parent)
		} else {
			putLE32(rec[2:], d.LBA)
			putLE16(rec[6:], parent)
		}
		copy(rec[8:], id)
		if len(rec)%2 != 0 {
			rec = append(rec, 0)
		}
		out = append(out, rec...)
	}
	return out
}

// dirRecords renders the records held in one directory.
func (w *Writer) dirRecords(d *Dir) []byte {
	out := make([]byte, d.Size)
	var off uint32

	// A record may not cross a block boundary; skip to the next one rather
	// than split it. contentSize accounts for the same padding.
	put := func(id string, lba, size uint32, isDir bool) {
		n := recordLen(len(id))
		if off%LogicalBlockSize+n > LogicalBlockSize {
			off += LogicalBlockSize - off%LogicalBlockSize
		}
		writeDirRecord(out[off:], id, lba, size, isDir, w.opts.Created)
		off += n
	}

	parent := d.parent
	if parent == nil {
		parent = d
	}
	put("\x00", d.LBA, d.Size, true)           // "."
	put("\x01", parent.LBA, parent.Size, true) // ".."
	for _, sub := range d.Dirs {
		put(sub.Name, sub.LBA, sub.Size, true)
	}
	for _, e := range d.Entries {
		put(e.Name, e.LBA, e.Size, false)
	}
	return out
}
