package iso9660

import (
	"encoding/binary"

	"go.chrisrx.dev/psx/format/cdrom"
)

// CD-XA extends ISO 9660 in two places: a signature in the primary volume
// descriptor announcing that the extension is in use, and a 14-byte block in
// each directory record's system use area describing the file.
//
// A PlayStation disc is an XA disc, so this is written by default. What the
// console's own filesystem code reads of it is not much, but the attributes
// are where a file says it holds Form 2 sectors, and a reader that trusts the
// extension will believe a disc that claims it and then omits the entries.
// Options.OmitXA turns it off for comparison against a plain ISO 9660 writer.

// RawSectorSize is the unit a raw entry's contents are made of: one sector's
// Mode 2 payload, subheader included. See Entry.Raw.
const RawSectorSize = cdrom.Mode2PayloadSize

// xaSysUseLen is the length of the XA block appended to a directory record,
// after the file identifier and its padding byte.
const xaSysUseLen = 14

// xaSignatureOffset is where "CD-XA001" goes in the primary volume
// descriptor. It falls inside the 512-byte application use area, which starts
// at byte 883 and is otherwise unused.
const xaSignatureOffset = 1024

// XA attribute bits, in the 16-bit big-endian attributes field. The low bits
// are Unix-style permissions and the high bits describe what the file holds.
const (
	xaOwnerRead   = 1 << 0
	xaOwnerExec   = 1 << 2
	xaGroupRead   = 1 << 4
	xaGroupExec   = 1 << 6
	xaWorldRead   = 1 << 8
	xaWorldExec   = 1 << 10
	xaForm1       = 1 << 11
	xaForm2       = 1 << 12
	xaInterleaved = 1 << 13
	xaCDDA        = 1 << 14
	xaDirectory   = 1 << 15
)

// xaReadable is read and execute permission for owner, group and world, the
// only answer a read-only disc has to give.
const xaReadable = xaOwnerRead | xaOwnerExec | xaGroupRead | xaGroupExec | xaWorldRead | xaWorldExec

// XA attributes for the three kinds of object a disc built here holds. The
// first two are the values a PlayStation disc is expected to carry, 0x8D55
// and 0x0D55.
const (
	xaAttrDir   = xaDirectory | xaForm1 | xaReadable
	xaAttrFile  = xaForm1 | xaReadable
	xaAttrForm2 = xaForm2 | xaReadable
)

// writeXA fills in the XA block of a directory record's system use area.
//
// The owner and group fields are left zero: they are a CD-i inheritance with
// nothing to say about a disc that grants everyone read and execute anyway.
// The file number is zero as well, which marks a file that is not one channel
// of an interleaved set; a file holding interleaved XA audio still reaches the
// console as a single stream, and it is the per-sector subheaders that sort
// its channels out, not this field.
func writeXA(b []byte, attr uint16) {
	binary.BigEndian.PutUint16(b[0:], 0) // owner group
	binary.BigEndian.PutUint16(b[2:], 0) // owner user
	binary.BigEndian.PutUint16(b[4:], attr)
	copy(b[6:], "XA")
	b[8] = 0       // file number
	clear(b[9:14]) // reserved
}
