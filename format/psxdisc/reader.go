package psxdisc

import (
	"fmt"
	"io"
	"os"

	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
)

// A RawReader reads an image of raw 2352-byte sectors, the .bin half of a
// .bin and .cue pair.
type RawReader struct {
	r       io.ReaderAt
	sectors uint32
	buf     []byte
}

// NewRawReader returns a BlockReader over an image of raw sectors. size is
// the length of the image in bytes.
func NewRawReader(r io.ReaderAt, size int64) (*RawReader, error) {
	if size%cdrom.SectorSize != 0 {
		return nil, fmt.Errorf("image is %d bytes, which is not a whole number of %d-byte sectors",
			size, cdrom.SectorSize)
	}
	return &RawReader{
		r:       r,
		sectors: uint32(size / cdrom.SectorSize),
		buf:     make([]byte, cdrom.SectorSize),
	}, nil
}

// sector reads and checks one raw sector.
func (r *RawReader) sector(lba uint32) ([]byte, error) {
	if lba >= r.sectors {
		return nil, fmt.Errorf("block %d is past the end of the image, which holds %d", lba, r.sectors)
	}
	if _, err := r.r.ReadAt(r.buf, int64(lba)*cdrom.SectorSize); err != nil {
		return nil, fmt.Errorf("reading sector %d: %w", lba, err)
	}
	return r.buf, nil
}

// ReadBlock implements iso9660.BlockReader.
func (r *RawReader) ReadBlock(lba uint32, b []byte) error {
	sector, err := r.sector(lba)
	if err != nil {
		return err
	}
	data, err := cdrom.Form1Data(sector)
	if err != nil {
		return fmt.Errorf("block %d: %w", lba, err)
	}
	if len(b) != len(data) {
		return fmt.Errorf("block %d: destination is %d bytes, want %d", lba, len(b), len(data))
	}
	copy(b, data)
	return nil
}

// ReadSector implements iso9660.BlockReader.
func (r *RawReader) ReadSector(lba uint32, b []byte) error {
	sector, err := r.sector(lba)
	if err != nil {
		return err
	}
	payload, err := cdrom.Mode2Payload(sector)
	if err != nil {
		return fmt.Errorf("block %d: %w", lba, err)
	}
	if len(b) != len(payload) {
		return fmt.Errorf("block %d: destination is %d bytes, want %d", lba, len(b), len(payload))
	}
	copy(b, payload)
	return nil
}

// An ISOReader reads an image of plain 2048-byte blocks.
type ISOReader struct {
	r      io.ReaderAt
	blocks uint32
}

// NewISOReader returns a BlockReader over a .iso. size is the length of the
// image in bytes.
func NewISOReader(r io.ReaderAt, size int64) (*ISOReader, error) {
	if size%iso9660.LogicalBlockSize != 0 {
		return nil, fmt.Errorf("image is %d bytes, which is not a whole number of %d-byte blocks",
			size, iso9660.LogicalBlockSize)
	}
	return &ISOReader{r: r, blocks: uint32(size / iso9660.LogicalBlockSize)}, nil
}

// ReadBlock implements iso9660.BlockReader.
func (i *ISOReader) ReadBlock(lba uint32, b []byte) error {
	if lba >= i.blocks {
		return fmt.Errorf("block %d is past the end of the image, which holds %d", lba, i.blocks)
	}
	if len(b) != iso9660.LogicalBlockSize {
		return fmt.Errorf("block %d: destination is %d bytes, want %d", lba, len(b), iso9660.LogicalBlockSize)
	}
	if _, err := i.r.ReadAt(b, int64(lba)*iso9660.LogicalBlockSize); err != nil {
		return fmt.Errorf("reading block %d: %w", lba, err)
	}
	return nil
}

// ReadSector implements iso9660.BlockReader.
func (i *ISOReader) ReadSector(lba uint32, b []byte) error {
	// The subheaders are the part a 2048-byte image threw away, and a
	// stream without them is unidentifiable, so there is nothing to return.
	return fmt.Errorf("block %d holds a raw Mode 2 sector, which a 2048-byte image did not keep", lba)
}

// Open reads the filesystem of an image file, working out from its length
// whether it holds raw sectors or plain blocks.
//
// The two are told apart by size alone: an image of raw sectors is a multiple
// of 2352 bytes and one of logical blocks is a multiple of 2048. Where both
// would fit, the sync pattern at the start of the first sector settles it,
// since a 2048-byte image has no room for one.
func Open(path string) (*iso9660.Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	br, err := blockReader(f, fi.Size())
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	r, err := iso9660.NewReader(br)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, f, nil
}

func blockReader(r io.ReaderAt, size int64) (iso9660.BlockReader, error) {
	raw := size%cdrom.SectorSize == 0
	iso := size%iso9660.LogicalBlockSize == 0
	if raw && iso {
		// Both lengths fit, so look at what is actually there.
		var head [12]byte
		if _, err := r.ReadAt(head[:], 0); err != nil {
			return nil, err
		}
		raw = head[0] == 0x00 && head[1] == 0xff && head[11] == 0x00
		iso = !raw
	}
	switch {
	case raw:
		return NewRawReader(r, size)
	case iso:
		return NewISOReader(r, size)
	}
	return nil, fmt.Errorf("image is %d bytes, neither a multiple of %d for raw sectors nor of %d for logical blocks",
		size, cdrom.SectorSize, iso9660.LogicalBlockSize)
}
