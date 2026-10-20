package iso9660

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// A Block is one block of the image, with what a disc needs to know in order
// to record it.
type Block struct {
	// LBA is the address of the block. Blocks are emitted in ascending
	// order with no gaps.
	LBA uint32

	// Data is the contents: LogicalBlockSize bytes, or a whole
	// RawSectorSize payload when Raw is set.
	Data []byte

	// Raw marks Data as a complete Mode 2 sector payload, subheader and
	// error detection included, to be recorded as it stands rather than
	// wrapped in a freshly encoded sector. See Entry.Raw.
	Raw bool

	// EndOfFile marks the last block of a file's contents or of a
	// directory's records. A Mode 2 sector records this in its subheader,
	// which is how a reader knows a file has ended without consulting the
	// filesystem again.
	EndOfFile bool
}

// A BlockWriter receives the blocks of the image in order.
//
// Keeping emission behind this interface is what lets the same filesystem be
// written either as a plain .iso of 2048-byte blocks or as the raw 2352-byte
// sectors a PlayStation disc actually uses, without the filesystem code
// knowing which. A writer that cannot record raw sectors should reject a
// Block with Raw set rather than write something approximate.
type BlockWriter interface {
	WriteBlock(Block) error
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

	// Created is the volume timestamp, and the timestamp recorded for every
	// file unless FileTimes is set. Leaving it zero produces a reproducible
	// image.
	Created time.Time

	// FileTimes records each file's own modification time in its directory
	// record rather than Created. It is off by default because an image
	// then depends on the mtimes of the tree it was built from, which
	// differ between checkouts.
	FileTimes bool

	// OmitXA leaves out the CD-XA extension: the "CD-XA001" signature in
	// the volume descriptor and the attribute block on every directory
	// record. A PlayStation disc carries both, so this is only useful for
	// producing something a plain ISO 9660 implementation can be compared
	// against byte for byte.
	OmitXA bool

	// Padding is a run of empty blocks to leave after the last file, counted
	// in the volume like any other block but reachable through no directory
	// record.
	//
	// A drive does not stop at the sector it was asked for: it reads ahead,
	// and the console's own CD code asks for whole sectors past the end of a
	// file. When the last file ends at the last sector of the track there is
	// nothing there to read, and the read fails rather than returning the
	// data that was wanted. Pressed discs leave a run-out for this reason,
	// which is what psxdisc.StandardPadding is.
	//
	// Zero writes no padding, which keeps an image as small as its contents
	// and is fine for anything only ever read by an emulator.
	Padding uint32

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
	// The volume identifier is a d-character field, so the same folding
	// that applies to file names applies here. Doing it in the library
	// keeps a stray lowercase letter or dot out of the descriptor whatever
	// the caller passes.
	o.VolumeID = NormalizeName(o.VolumeID)
}

// A Writer lays out and emits an ISO 9660 image.
type Writer struct {
	root *Dir
	opts Options

	dirs []*Dir

	// sysUse is the system use area appended to every directory record,
	// which is the XA attribute block unless it was turned off.
	sysUse uint32

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
	if len(opts.VolumeID) > MaxVolumeIDLen {
		return nil, fmt.Errorf("volume id %q is %d characters, more than the %d a volume identifier holds",
			opts.VolumeID, len(opts.VolumeID), MaxVolumeIDLen)
	}

	w := &Writer{root: root, opts: opts}
	if !opts.OmitXA {
		w.sysUse = xaSysUseLen
	}
	w.dirs = root.walk()

	// A raw file is a sequence of whole sectors, so a length that is not a
	// multiple of one is not a short final sector but a sign that the file
	// is not what it was taken for: an ordinary data file given a .XA name,
	// or a stream truncated in transit.
	for _, d := range w.dirs {
		for _, e := range d.Entries {
			if e.Raw && e.Size%RawSectorSize != 0 {
				return nil, fmt.Errorf("%s: %d bytes is not a whole number of %d-byte Mode 2 sectors, so it is not a raw stream",
					e.Name, e.Size, RawSectorSize)
			}
		}
	}

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
		// A directory's recorded length is a whole number of logical
		// blocks, unlike a file's. The records themselves stop short of
		// that and the rest is zeroed, which is the same padding a reader
		// already has to step over inside the extent.
		d.Size = blocks(d.contentSize(w.sysUse)) * LogicalBlockSize
		d.LBA = lba
		lba += blocks(d.Size)
	}
	for _, d := range w.dirs {
		for _, e := range d.Entries {
			e.LBA = lba
			lba += e.sectors()
		}
	}
	lba += opts.Padding

	w.totalBlocks = lba
	return w, nil
}

// TotalBlocks is the size of the image in logical blocks.
func (w *Writer) TotalBlocks() uint32 { return w.totalBlocks }

// recordTime is the timestamp to record for a file or directory. A
// modification time is only honoured when the caller asked for it, so the
// default image depends on nothing but the tree's contents.
func (w *Writer) recordTime(mod time.Time) time.Time {
	if w.opts.FileTimes && !mod.IsZero() {
		return mod
	}
	return w.opts.Created
}

// WriteTo emits the image, one logical block at a time and in order.
func (w *Writer) WriteTo(bw BlockWriter) error {
	block := make([]byte, LogicalBlockSize)
	lba := uint32(0)

	emit := func(data []byte, endOfFile bool) error {
		if err := bw.WriteBlock(Block{LBA: lba, Data: data, EndOfFile: endOfFile}); err != nil {
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
		if err := emit(block, false); err != nil {
			return err
		}
	}

	clear(block)
	w.writePVD(block)
	if err := emit(block, false); err != nil {
		return err
	}

	clear(block)
	block[0] = volumeDescriptorTerminator
	copy(block[1:], "CD001")
	block[6] = 1
	if err := emit(block, false); err != nil {
		return err
	}

	for _, bigEndian := range []bool{false, true} {
		table := w.pathTable(bigEndian)
		for len(table) > 0 {
			clear(block)
			n := copy(block, table)
			table = table[n:]
			// The volume descriptors and the path tables are structure
			// rather than files, and have no directory record to be the end
			// of, so they go unmarked.
			if err := emit(block, false); err != nil {
				return err
			}
		}
	}

	// A directory's records are a file as far as the disc is concerned, so
	// its last block is marked as the end of one.
	for _, d := range w.dirs {
		records := w.dirRecords(d)
		for len(records) > 0 {
			clear(block)
			n := copy(block, records)
			records = records[n:]
			if err := emit(block, len(records) == 0); err != nil {
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

	// The run-out. These belong to no file, so they are not the end of one.
	for range w.opts.Padding {
		clear(block)
		if err := emit(block, false); err != nil {
			return err
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

	// A raw file's contents are already sectors, so they are passed through
	// a sector at a time. Everything else is a byte stream packed into
	// logical blocks, with the last one zero padded.
	unit := uint32(LogicalBlockSize)
	if e.Raw {
		unit = RawSectorSize
	}

	block := make([]byte, unit)
	total := e.sectors()
	for i := range total {
		clear(block)
		if _, err := io.ReadFull(rc, block); err != nil &&
			err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("reading %s: %w", e.Name, err)
		}
		b := Block{
			LBA:       *lba,
			Data:      block,
			Raw:       e.Raw,
			EndOfFile: i == total-1,
		}
		if err := bw.WriteBlock(b); err != nil {
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
	binary.LittleEndian.PutUint32(b[140:], w.lPathTableLBA)
	binary.LittleEndian.PutUint32(b[144:], 0) // no optional L path table
	binary.BigEndian.PutUint32(b[148:], w.mPathTableLBA)
	binary.BigEndian.PutUint32(b[152:], 0) // no optional M path table

	// The root directory record is stored inline rather than referenced, in
	// a fixed 34-byte field. There is no room for a system use area, so the
	// XA block goes only on the records in the directories themselves.
	writeDirRecord(b[156:190], record{
		id:    "\x00",
		lba:   w.root.LBA,
		size:  w.root.Size,
		isDir: true,
		mod:   w.recordTime(w.root.Modified),
	})

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

	// The application use area is otherwise unused, and an XA disc puts its
	// signature in the middle of it.
	if !w.opts.OmitXA {
		copy(b[xaSignatureOffset:], "CD-XA001")
	}
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
			binary.BigEndian.PutUint32(rec[2:], d.LBA)
			binary.BigEndian.PutUint16(rec[6:], parent)
		} else {
			binary.LittleEndian.PutUint32(rec[2:], d.LBA)
			binary.LittleEndian.PutUint16(rec[6:], parent)
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
	put := func(r record) {
		r.sysUse = w.sysUse
		r.mod = w.recordTime(r.mod)
		n := recordLen(len(r.id), r.sysUse)
		if off%LogicalBlockSize+n > LogicalBlockSize {
			off += LogicalBlockSize - off%LogicalBlockSize
		}
		writeDirRecord(out[off:], r)
		off += n
	}

	parent := d.parent
	if parent == nil {
		parent = d
	}
	// "." and ".."
	put(record{id: "\x00", lba: d.LBA, size: d.Size, isDir: true, mod: d.Modified})
	put(record{id: "\x01", lba: parent.LBA, size: parent.Size, isDir: true, mod: parent.Modified})
	for _, sub := range d.Dirs {
		put(record{id: sub.Name, lba: sub.LBA, size: sub.Size, isDir: true, mod: sub.Modified})
	}
	for _, e := range d.Entries {
		put(record{id: e.Name, lba: e.LBA, size: e.recordedSize(), raw: e.Raw, mod: e.Modified})
	}
	return out
}
