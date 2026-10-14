// Package psxdisc writes PlayStation disc images.
//
// A disc image is a .bin of raw 2352-byte CD-ROM sectors plus a .cue sheet
// naming the tracks. That pairing exists because a PlayStation disc is not
// just a filesystem: the format can carry CD audio and streamed XA alongside
// the data track, and none of that fits in a flat .iso of 2048-byte blocks.
package psxdisc

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"

	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
)

// A RawWriter wraps each logical block in a Mode 2 Form 1 sector, producing
// the 2352-byte sectors of a .bin image.
type RawWriter struct {
	w    io.Writer
	next uint32
	buf  []byte
}

// NewRawWriter returns a BlockWriter that emits raw CD-ROM sectors.
func NewRawWriter(w io.Writer) *RawWriter {
	return &RawWriter{w: w, buf: make([]byte, cdrom.SectorSize)}
}

// WriteBlock implements iso9660.BlockWriter.
func (r *RawWriter) WriteBlock(lba uint32, data []byte) error {
	// Blocks must arrive in order: a sector's address is encoded in its own
	// header, so emitting them out of order would silently produce an image
	// whose contents and addressing disagree.
	if lba != r.next {
		return fmt.Errorf("block %d arrived out of order, expected %d", lba, r.next)
	}
	if err := cdrom.EncodeForm1(r.buf, lba, cdrom.DataSubheader, data); err != nil {
		return err
	}
	if _, err := r.w.Write(r.buf); err != nil {
		return err
	}
	r.next++
	return nil
}

// An ISOWriter emits plain 2048-byte logical blocks, producing a .iso rather
// than a raw image. It cannot represent audio tracks or Form 2 data, but it
// is what ordinary ISO 9660 tools can read, which makes it useful for
// checking an image against an independent implementation.
type ISOWriter struct {
	w    io.Writer
	next uint32
}

// NewISOWriter returns a BlockWriter that emits 2048-byte blocks.
func NewISOWriter(w io.Writer) *ISOWriter {
	return &ISOWriter{w: w}
}

// WriteBlock implements iso9660.BlockWriter.
func (i *ISOWriter) WriteBlock(lba uint32, data []byte) error {
	if lba != i.next {
		return fmt.Errorf("block %d arrived out of order, expected %d", lba, i.next)
	}
	if len(data) != iso9660.LogicalBlockSize {
		return fmt.Errorf("block %d is %d bytes, want %d", lba, len(data), iso9660.LogicalBlockSize)
	}
	if _, err := i.w.Write(data); err != nil {
		return err
	}
	i.next++
	return nil
}

// LicenseStringSector is where the disc identification string lives, within
// the 16 sector system area that precedes the filesystem.
const LicenseStringSector = 4

// identificationString marks a disc as a PlayStation disc. Emulators read it
// to decide which system a given image belongs to; without it they cannot
// tell a PlayStation disc from any other CD-XA disc and will refuse to load
// the image at all.
//
// This is disc format identification and nothing more. An unmodified console
// does not read it to decide whether to boot: that check is on the wobble
// encoded in a pressed disc's lead-in groove, which is a physical property no
// file can reproduce. Booting on real hardware additionally wants the licence
// data proper, which holds the logo shown at startup, is Sony's, and has to
// be supplied by whoever owns a copy.
const identificationString = "          Licensed  by          Sony Computer Entertainment Inc."

// SystemArea returns the 32KB that precedes the filesystem, carrying the
// identification string so that emulators recognise the image.
func SystemArea() []byte {
	area := make([]byte, iso9660.SystemAreaSectors*iso9660.LogicalBlockSize)
	copy(area[LicenseStringSector*iso9660.LogicalBlockSize:], identificationString)
	return area
}

// WriteCueSheet writes a .cue describing a single Mode 2 data track, which is
// what a disc holding only a filesystem needs. binName should be the file
// name of the .bin as it sits beside the .cue, not a full path.
func WriteCueSheet(w io.Writer, binName string) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "FILE %q BINARY\n", filepath.Base(binName))
	fmt.Fprintf(bw, "  TRACK 01 MODE2/%d\n", cdrom.SectorSize)
	fmt.Fprintf(bw, "    INDEX 01 00:00:00\n")
	return bw.Flush()
}
