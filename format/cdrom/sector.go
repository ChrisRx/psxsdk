// Package cdrom implements the raw CD-ROM sector encoding used by PlayStation
// discs.
//
// A PlayStation disc is not a plain ISO 9660 image. Its sectors are the full
// 2352-byte raw form carrying sync, header, subheader, user data and two
// layers of error detection and correction, in the CD-ROM XA Mode 2 layout:
//
//	Form 1  2048 bytes of user data, with EDC and ECC. Used for the
//	        filesystem and for anything that must be read exactly.
//	Form 2  2324 bytes of user data, with EDC but no ECC, so a damaged
//	        sector is passed through rather than retried. Used for streamed
//	        audio and video, where a glitch beats a stall.
//
// This is why a disc image is distributed as a .bin of 2352-byte sectors with
// a .cue alongside, rather than as a 2048-byte-per-sector .iso.
package cdrom

import (
	"bytes"
	"fmt"
)

const (
	// SectorSize is the size of a raw CD-ROM sector.
	SectorSize = 2352

	// Form1DataSize is the user data carried by a Mode 2 Form 1 sector.
	Form1DataSize = 2048

	// Form2DataSize is the user data carried by a Mode 2 Form 2 sector.
	Form2DataSize = 2324

	// Mode2PayloadSize is everything a Mode 2 sector holds after its sync
	// and address header: both copies of the subheader, the user data and
	// the error detection, 2336 bytes in all. It is derived from the layout
	// rather than written out so that the two cannot disagree.
	//
	// XA audio and STR video are produced as a stream of these, subheaders
	// included, which is why such a file is a whole number of 2336-byte
	// units rather than a plain byte stream.
	Mode2PayloadSize = SectorSize - subheaderOffset
)

const (
	// LeadInSectors is the 2 second pregap before logical sector zero. CD
	// addressing counts from the physical start of the disc, so converting
	// a logical block address to a timecode adds it.
	LeadInSectors = 150
)

// Sector layout offsets.
const (
	syncOffset      = 0
	headerOffset    = 12
	subheaderOffset = 16
	dataOffset      = 24

	form1EDCOffset = 2072 // after 2048 bytes of user data
	form2EDCOffset = 2348 // after 2324 bytes of user data
	eccPOffset     = 2076
	eccQOffset     = 2248
)

// syncPattern begins every raw sector.
var syncPattern = [12]byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}

// Submode bits in the subheader, describing how a sector should be handled.
type Submode uint8

const (
	SubmodeEndOfRecord Submode = 1 << 0
	SubmodeVideo       Submode = 1 << 1
	SubmodeAudio       Submode = 1 << 2
	SubmodeData        Submode = 1 << 3
	SubmodeTrigger     Submode = 1 << 4
	SubmodeForm2       Submode = 1 << 5
	SubmodeRealTime    Submode = 1 << 6
	SubmodeEndOfFile   Submode = 1 << 7
)

// A Subheader describes a Mode 2 sector's contents. It is stored twice in
// every sector so that a reader can recover it from either copy.
type Subheader struct {
	// File and Channel group sectors into interleaved streams. Ordinary
	// data uses zero for both.
	File    byte
	Channel byte

	Submode Submode

	// Coding describes the sample format of an audio sector and is zero
	// for data.
	Coding byte
}

// DataSubheader is the subheader for an ordinary filesystem data sector.
var DataSubheader = Subheader{Submode: SubmodeData}

// toBCD encodes a value below 100 as two binary coded decimal digits.
func toBCD(v uint32) byte {
	return byte((v/10)<<4 | (v % 10))
}

// MSF converts a logical block address to the minute/second/frame timecode
// stored in a sector header. There are 75 frames per second, and addressing
// starts two seconds before logical sector zero.
func MSF(lba uint32) (minute, second, frame byte) {
	abs := lba + LeadInSectors
	return toBCD(abs / (75 * 60)), toBCD((abs / 75) % 60), toBCD(abs % 75)
}

// EncodeForm1 writes a Mode 2 Form 1 sector to dst, which must be SectorSize
// bytes. data is the user payload and is zero padded if shorter than
// Form1DataSize.
func EncodeForm1(dst []byte, lba uint32, sh Subheader, data []byte) error {
	if len(dst) != SectorSize {
		return fmt.Errorf("sector buffer is %d bytes, want %d", len(dst), SectorSize)
	}
	if len(data) > Form1DataSize {
		return fmt.Errorf("%d bytes of data, more than a Form 1 sector's %d", len(data), Form1DataSize)
	}
	writeHeader(dst, lba, sh)
	copy(dst[dataOffset:dataOffset+Form1DataSize], data)
	clear(dst[dataOffset+len(data) : dataOffset+Form1DataSize])

	// The EDC covers the subheader and user data. The ECC then covers the
	// header, subheader, data and EDC together, so it has to come second.
	putUint32LE(dst[form1EDCOffset:], edc(dst[subheaderOffset:form1EDCOffset]))
	generateECC(dst)
	return nil
}

// EncodeForm2 writes a Mode 2 Form 2 sector to dst. Form 2 has no ECC: the
// space is given over to user data instead, on the basis that the streamed
// media it carries is better off with a corrupt sample than a read retry.
func EncodeForm2(dst []byte, lba uint32, sh Subheader, data []byte) error {
	if len(dst) != SectorSize {
		return fmt.Errorf("sector buffer is %d bytes, want %d", len(dst), SectorSize)
	}
	if len(data) > Form2DataSize {
		return fmt.Errorf("%d bytes of data, more than a Form 2 sector's %d", len(data), Form2DataSize)
	}
	sh.Submode |= SubmodeForm2
	writeHeader(dst, lba, sh)
	copy(dst[dataOffset:dataOffset+Form2DataSize], data)
	clear(dst[dataOffset+len(data) : dataOffset+Form2DataSize])
	putUint32LE(dst[form2EDCOffset:], edc(dst[subheaderOffset:form2EDCOffset]))
	return nil
}

// EncodeRaw writes a sector whose Mode 2 payload is supplied whole:
// subheader, user data and error detection exactly as they are to appear on
// the disc. Only the sync pattern and the address header are added.
//
// This is how XA audio and STR video have to be written. Their sectors carry
// per-sector subheaders that say which form each one is and which interleaved
// channel it belongs to, so re-encoding them from a payload would mean
// inventing information the stream already carries. Nothing is recomputed,
// not even the EDC: a stream whose sectors are deliberately Form 2 is one
// where a damaged sector is meant to be passed through, and silently
// rewriting its error detection would misrepresent the source.
func EncodeRaw(dst []byte, lba uint32, payload []byte) error {
	if len(dst) != SectorSize {
		return fmt.Errorf("sector buffer is %d bytes, want %d", len(dst), SectorSize)
	}
	if len(payload) != Mode2PayloadSize {
		return fmt.Errorf("payload is %d bytes, want a whole Mode 2 payload of %d", len(payload), Mode2PayloadSize)
	}
	writeSyncHeader(dst, lba)
	copy(dst[subheaderOffset:], payload)
	return nil
}

// Mode2Payload returns the Mode 2 payload of a raw sector: the subheader,
// user data and error detection, with only the sync pattern and address
// header left behind. This is the form XA audio and STR video are stored in,
// so it is what reading one back out of an image gives.
func Mode2Payload(sector []byte) ([]byte, error) {
	if err := checkSector(sector); err != nil {
		return nil, err
	}
	return sector[subheaderOffset:], nil
}

// Form1Data returns the user data of a Mode 2 Form 1 sector.
//
// The EDC is checked as well as the layout. A sector of a filesystem is
// exactly the kind that is meant to be read correctly or not at all, and a
// reader that passed a failing one through would report a file that the
// console itself would refuse.
func Form1Data(sector []byte) ([]byte, error) {
	if err := checkSector(sector); err != nil {
		return nil, err
	}
	if Submode(sector[subheaderOffset+2])&SubmodeForm2 != 0 {
		return nil, fmt.Errorf("sector is Form 2, which holds %d bytes of streamed data rather than %d of user data",
			Form2DataSize, Form1DataSize)
	}
	stored := uint32(sector[form1EDCOffset]) | uint32(sector[form1EDCOffset+1])<<8 |
		uint32(sector[form1EDCOffset+2])<<16 | uint32(sector[form1EDCOffset+3])<<24
	if got := edc(sector[subheaderOffset:form1EDCOffset]); got != stored {
		return nil, fmt.Errorf("error detection is %#08x but the contents give %#08x, so the sector is damaged", stored, got)
	}
	return sector[dataOffset : dataOffset+Form1DataSize], nil
}

// DecodeSubheader returns a sector's subheader, which says what the sector
// holds and which interleaved stream it belongs to.
func DecodeSubheader(sector []byte) (Subheader, error) {
	if err := checkSector(sector); err != nil {
		return Subheader{}, err
	}
	return Subheader{
		File:    sector[subheaderOffset+0],
		Channel: sector[subheaderOffset+1],
		Submode: Submode(sector[subheaderOffset+2]),
		Coding:  sector[subheaderOffset+3],
	}, nil
}

// checkSector rejects anything that is not a Mode 2 sector before its
// contents are believed. The sync pattern is fixed and the mode byte has one
// acceptable value, so either being wrong means the bytes are not a sector at
// all: the wrong file, or an image read at the wrong offset.
func checkSector(sector []byte) error {
	if len(sector) != SectorSize {
		return fmt.Errorf("sector is %d bytes, want %d", len(sector), SectorSize)
	}
	if !bytes.Equal(sector[syncOffset:syncOffset+len(syncPattern)], syncPattern[:]) {
		return fmt.Errorf("sector does not begin with a sync pattern, so it is not a raw CD-ROM sector")
	}
	if mode := sector[headerOffset+3]; mode != 2 {
		return fmt.Errorf("sector is mode %d, and only mode 2 carries a PlayStation filesystem", mode)
	}
	return nil
}

// writeSyncHeader fills in the sync pattern and the address header, which
// every raw sector begins with regardless of what follows.
func writeSyncHeader(dst []byte, lba uint32) {
	copy(dst[syncOffset:], syncPattern[:])

	m, s, f := MSF(lba)
	dst[headerOffset+0] = m
	dst[headerOffset+1] = s
	dst[headerOffset+2] = f
	dst[headerOffset+3] = 2 // mode 2
}

// writeHeader fills in the sync pattern, address header and both copies of
// the subheader.
func writeHeader(dst []byte, lba uint32, sh Subheader) {
	writeSyncHeader(dst, lba)
	for _, off := range [2]int{subheaderOffset, subheaderOffset + 4} {
		dst[off+0] = sh.File
		dst[off+1] = sh.Channel
		dst[off+2] = byte(sh.Submode)
		dst[off+3] = sh.Coding
	}
}

func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
