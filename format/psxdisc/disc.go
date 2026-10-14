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
func (r *RawWriter) WriteBlock(b iso9660.Block) error {
	// Blocks must arrive in order: a sector's address is encoded in its own
	// header, so emitting them out of order would silently produce an image
	// whose contents and addressing disagree.
	if b.LBA != r.next {
		return fmt.Errorf("block %d arrived out of order, expected %d", b.LBA, r.next)
	}
	if b.Raw {
		// The payload is already a sector's worth of subheader, data and
		// error detection, so only the address is ours to supply.
		if err := cdrom.EncodeRaw(r.buf, b.LBA, b.Data); err != nil {
			return err
		}
	} else {
		sh := cdrom.DataSubheader
		if b.EndOfFile {
			// The last sector of a file ends both the file and the record
			// within it, which is the 0x89 submode seen on pressed discs.
			sh.Submode |= cdrom.SubmodeEndOfFile | cdrom.SubmodeEndOfRecord
		}
		if err := cdrom.EncodeForm1(r.buf, b.LBA, sh, b.Data); err != nil {
			return err
		}
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
func (i *ISOWriter) WriteBlock(b iso9660.Block) error {
	if b.LBA != i.next {
		return fmt.Errorf("block %d arrived out of order, expected %d", b.LBA, i.next)
	}
	if b.Raw {
		// There is nowhere to put a subheader in a 2048-byte block, and
		// dropping it would turn a stream into 2324 bytes of noise with no
		// way to tell what it was.
		return fmt.Errorf("block %d holds a raw Mode 2 sector, which a 2048-byte image cannot represent", b.LBA)
	}
	if len(b.Data) != iso9660.LogicalBlockSize {
		return fmt.Errorf("block %d is %d bytes, want %d", b.LBA, len(b.Data), iso9660.LogicalBlockSize)
	}
	if _, err := i.w.Write(b.Data); err != nil {
		return err
	}
	i.next++
	return nil
}

// StandardPadding is the run-out to leave after the last file of the data
// track, in sectors, for iso9660.Options.Padding.
//
// Two seconds of sectors, the same margin as the pause between tracks. The
// console reads a file by asking for whole sectors and the drive reads ahead
// of what it was asked for, so a file that ends at the last sector of the
// track is a file the console cannot read all of: the request runs into the
// lead-out and fails. The cost of avoiding that is 300KB of zeros, which is
// what a pressed disc carries there too.
const StandardPadding = 2 * 75

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

// WriteCueSheet writes the .cue describing a written image's tracks.
// binName should be the file name of the .bin as it sits beside the .cue, not
// a full path, since that is where a cue sheet is read from.
func WriteCueSheet(w io.Writer, binName string, l Layout) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "FILE %q BINARY\n", filepath.Base(binName))
	for _, t := range l.Tracks {
		if t.Audio {
			fmt.Fprintf(bw, "  TRACK %02d AUDIO\n", t.Number)
		} else {
			fmt.Fprintf(bw, "  TRACK %02d MODE2/%d\n", t.Number, cdrom.SectorSize)
		}
		// A pause that is recorded in the file is given as index zero, so
		// that a player seeking to the track lands on the samples and a
		// program copying the disc still finds the gap where it was.
		if t.Pregap != t.Start {
			fmt.Fprintf(bw, "    INDEX 00 %s\n", msf(t.Pregap))
		}
		fmt.Fprintf(bw, "    INDEX 01 %s\n", msf(t.Start))
	}
	return bw.Flush()
}

// msf formats a sector as the timecode a cue sheet uses. Positions in a cue
// sheet are offsets into the file it names, so unlike the timecode in a
// sector header they do not include the two second lead-in.
func msf(sector uint32) string {
	return fmt.Sprintf("%02d:%02d:%02d", sector/(75*60), (sector/75)%60, sector%75)
}
