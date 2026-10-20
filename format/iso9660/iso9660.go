// Package iso9660 writes the ISO 9660 filesystem used on PlayStation discs.
//
// The format is from 1988 and shows it. Multi-byte numbers are stored twice,
// once little-endian and once big-endian, so that a reader of either
// persuasion can pick the half it likes. Directories are described twice over
// as well: once as a tree of directory records, and once as a flat path table
// that lets a drive find any directory without walking the tree. Keeping the
// two consistent is most of the work in writing one.
package iso9660

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	// LogicalBlockSize is the ISO 9660 block, which is also exactly the
	// user data of a Mode 2 Form 1 sector.
	LogicalBlockSize = 2048

	// SystemAreaSectors is the 32KB reserved before the first volume
	// descriptor. On a PlayStation disc this holds the licence data.
	SystemAreaSectors = 16

	// PVDSector is where the primary volume descriptor lives, immediately
	// after the system area.
	PVDSector = SystemAreaSectors

	// DirRecordFixedLen is the size of a directory record before its file
	// identifier and any system use area.
	DirRecordFixedLen = 33
)

// Volume descriptor types.
const (
	volumeDescriptorPrimary    = 1
	volumeDescriptorTerminator = 255
)

// File flags in a directory record.
const (
	fileFlagHidden    = 1 << 0
	fileFlagDirectory = 1 << 1
)

// putBoth16 writes a 16-bit value in both byte orders, as ISO 9660 requires
// for most numeric fields.
func putBoth16(b []byte, v uint16) {
	binary.LittleEndian.PutUint16(b, v)
	binary.BigEndian.PutUint16(b[2:], v)
}

// putBoth32 writes a 32-bit value in both byte orders.
func putBoth32(b []byte, v uint32) {
	binary.LittleEndian.PutUint32(b, v)
	binary.BigEndian.PutUint32(b[4:], v)
}

// padString writes s into a fixed width field, space padded, as ISO 9660
// expects for its text fields.
func padString(b []byte, s string) {
	n := copy(b, s)
	for i := n; i < len(b); i++ {
		b[i] = ' '
	}
}

// putDecDateTime writes the 17-byte "digits" form used for volume dates.
func putDecDateTime(b []byte, t time.Time) {
	if t.IsZero() {
		// All zeros with a zero offset means "not specified".
		copy(b, "0000000000000000")
		b[16] = 0
		return
	}
	copy(b, fmt.Sprintf("%04d%02d%02d%02d%02d%02d%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0))
	b[16] = byte(zoneOffset(t))
}

// zoneOffset is a time's offset from GMT in 15 minute intervals, which is how
// both date forms record a zone. The digits in the fields before it are local
// time, so leaving this zero would move the recorded instant by whatever the
// local offset happened to be when the image was built.
func zoneOffset(t time.Time) int8 {
	_, off := t.Zone()
	return int8(off / (15 * 60))
}

// putDirDateTime writes the compact 7-byte form used in directory records.
func putDirDateTime(b []byte, t time.Time) {
	if t.IsZero() {
		// All zeros means "not specified". Encoding the zero time
		// arithmetically instead would wrap the year byte and claim a date
		// in the next century.
		clear(b[:7])
		return
	}
	b[0] = byte(t.Year() - 1900)
	b[1] = byte(t.Month())
	b[2] = byte(t.Day())
	b[3] = byte(t.Hour())
	b[4] = byte(t.Minute())
	b[5] = byte(t.Second())
	b[6] = byte(zoneOffset(t))
}

// NormalizeName converts a name to the restricted ISO 9660 form: uppercase
// letters, digits and underscore only.
//
// This is deliberately strict rather than clever. A PlayStation looks files up
// by the exact name the program asks for, and those names are baked into the
// executable, so silently rewriting one into something that merely resembles
// it would turn a build error into a runtime failure on hardware.
func NormalizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ShortName is the 8.3 name a file gets on the disc: in capitals, letters,
// digits and underscores, its stem cut to eight characters and its
// extension to three.
func ShortName(name string) string {
	ext := ""
	if i := strings.LastIndex(name, "."); i > 0 {
		name, ext = name[:i], name[i+1:]
	}
	stem := NormalizeName(name)
	if len(stem) > 8 {
		stem = stem[:8]
	}
	if ext == "" {
		return stem
	}
	ext = NormalizeName(ext)
	if len(ext) > 3 {
		ext = ext[:3]
	}
	return stem + "." + ext
}

// MaxVolumeIDLen is the width of the volume identifier field.
const MaxVolumeIDLen = 32

// FileIdentifier returns the on-disc identifier for a file name, which
// carries the ";1" version suffix that PlayStation programs include in the
// paths they open.
func FileIdentifier(name string) string {
	name = strings.ToUpper(name)
	ext := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		ext, name = name[i+1:], name[:i]
	}
	id := NormalizeName(name)
	if ext != "" {
		id += "." + NormalizeName(ext)
	}
	return id + ";1"
}

// blocks returns the number of logical blocks needed to hold n bytes.
func blocks(n uint32) uint32 {
	return (n + LogicalBlockSize - 1) / LogicalBlockSize
}
