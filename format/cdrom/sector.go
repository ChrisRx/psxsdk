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

import "fmt"

const (
	// SectorSize is the size of a raw CD-ROM sector.
	SectorSize = 2352

	// Form1DataSize is the user data carried by a Mode 2 Form 1 sector.
	Form1DataSize = 2048

	// Form2DataSize is the user data carried by a Mode 2 Form 2 sector.
	Form2DataSize = 2324

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

// writeHeader fills in the sync pattern, address header and both copies of
// the subheader.
func writeHeader(dst []byte, lba uint32, sh Subheader) {
	copy(dst[syncOffset:], syncPattern[:])

	m, s, f := MSF(lba)
	dst[headerOffset+0] = m
	dst[headerOffset+1] = s
	dst[headerOffset+2] = f
	dst[headerOffset+3] = 2 // mode 2

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
