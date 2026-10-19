package yaroze

import "unsafe"

// Memory cards. A card is 15 blocks of 8 KB, and the BIOS keeps a file
// system on it: a file is a name of up to 20 characters and a whole number
// of blocks, and is read and written through the kernel's file functions,
// Open, Read, Write, Lseek and Close, with "bu00:" naming the card in slot
// 1 and "bu10:" the card in slot 2. A save file that other games and the
// console's own card screen can show begins with a header Sony set out,
// SaveHeader, and has a name in Sony's form (docs/manuals/savefile.md and
// docs/manuals/peripherals.md). The card sample in Sony's Net Yaroze
// samples, ported as examples/card, uses all of it.

// Flags for Open, the kernel's.
const (
	O_RDONLY = 0x0001
	O_WRONLY = 0x0002
	O_RDWR   = 0x0003
	O_CREAT  = 0x0200 // with the file's blocks in the top 16 bits: O_CREAT | n<<16
	O_NOBUF  = 0x4000
	O_NOWAIT = 0x8000
)

// Where Lseek counts from.
const (
	SEEK_SET = 0
	SEEK_CUR = 1
)

// What TestCard reports of a slot.
const (
	CardNone        = 0 // no card
	CardPresent     = 1 // a card, as before
	CardNew         = 2 // a card put in since the last test
	CardError       = 3 // a card that did not answer properly
	CardUnformatted = 4
)

// CardBlock is the size of a card's block, the unit files are made in.
const CardBlock = 8192

// CardSector is the unit a card is read and written in: a read or write of
// a file is a whole number of them.
const CardSector = 128

// A SaveHeader is the 128 bytes Sony's convention puts at the start of a
// save file, which the console's card screen and other games read to show
// it: "SC", the number of icon frames, the file's blocks, its title in
// Shift-JIS, and the icon's palette. The icon's frames, 16 by 16 pixels of
// 4 bits each, 128 bytes a frame, follow it, and then the game's data.
type SaveHeader struct {
	// Frames is how many frames the icon has, 1 to 3, which the card
	// screen shows in turn. Sony asked European games for 1.
	Frames uint8

	// Blocks is the file's size in blocks.
	Blocks uint8

	// Title is the name the card screen shows, in Shift-JIS, up to 32
	// full-width characters. SetTitle makes one from ASCII.
	Title [64]byte

	// CLUT is the icon's palette, sixteen colours as a TIM's.
	CLUT [16]uint16
}

// SetTitle sets the title from ASCII, each character turned into its
// full-width Shift-JIS form, as Sony's convention asks, so that the card
// screen shows the whole of it. It keeps the first 32 characters.
func (h *SaveHeader) SetTitle(s string) {
	h.Title = [64]byte{}
	for i := 0; i < len(s) && i < 32; i++ {
		c := ShiftJIS(s[i])
		h.Title[2*i], h.Title[2*i+1] = byte(c>>8), byte(c)
	}
}

// Put writes the header's 128 bytes into b.
func (h *SaveHeader) Put(b []byte) {
	_ = b[CardSector-1]
	clear(b[:CardSector])
	b[0], b[1] = 'S', 'C'
	b[2] = 0x10 + max(h.Frames, 1)
	b[3] = h.Blocks
	copy(b[4:68], h.Title[:])
	for i, c := range h.CLUT {
		b[96+2*i], b[97+2*i] = byte(c), byte(c>>8)
	}
}

// ShiftJIS returns the full-width Shift-JIS character for an ASCII one,
// from the table in Sony's FAQ ("How can I convert an ASCII string into a
// Kanji string?"), or the full-width space for anything it has none for.
func ShiftJIS(c byte) uint16 {
	switch {
	case c >= '0' && c <= '9':
		return 0x824f + uint16(c-'0')
	case c >= 'A' && c <= 'Z':
		return 0x8260 + uint16(c-'A')
	case c >= 'a' && c <= 'z':
		return 0x8281 + uint16(c-'a')
	case c >= 0x20 && c <= 0x2f:
		return sjisPunct[c-0x20]
	case c >= 0x3a && c <= 0x40:
		return sjisPunct[c-0x3a+16]
	case c >= 0x5b && c <= 0x60:
		return sjisPunct[c-0x5b+23]
	case c >= 0x7b && c <= 0x7e:
		return sjisPunct[c-0x7b+29]
	}
	return 0x8140
}

// sjisPunct is the full-width form of ASCII's punctuation, from space to
// '/', then ':' to '@', '[' to '`' and '{' to '~'.
var sjisPunct = [33]uint16{
	0x8140, 0x8149, 0x8168, 0x8194, 0x8190, 0x8193, 0x8195, 0x8166,
	0x8169, 0x816a, 0x8196, 0x817b, 0x8143, 0x817c, 0x8144, 0x815e,
	0x8146, 0x8147, 0x8171, 0x8181, 0x8172, 0x8148, 0x8197,
	0x816d, 0x818f, 0x816e, 0x814f, 0x8151, 0x8165,
	0x816f, 0x8162, 0x8170, 0x8150,
}

// CardPath writes the C string naming a file on the card in slot 0 or 1,
// "bu00:name" or "bu10:name", into buf and returns it for the file
// functions. buf must hold the name and six bytes more.
func CardPath(buf []byte, slot int, name string) *byte {
	dev := "bu00:"
	if slot == 1 {
		dev = "bu10:"
	}
	n := copy(buf, dev)
	n += copy(buf[n:len(buf)-1], name)
	buf[n] = 0
	return &buf[0]
}

// CardWrite creates the file name on the card in slot 0 or 1, as big as
// data needs in whole blocks, replacing one that is there, writes data,
// which must be a whole number of sectors, to it, and reads it back to see
// that it is there. It returns whether it is. A save file's data starts
// with its SaveHeader and icon.
func CardWrite(slot int, name string, data []byte) bool {
	if len(data) == 0 || len(data)%CardSector != 0 {
		return false
	}
	blocks := (len(data) + CardBlock - 1) / CardBlock
	var path [32]byte
	p := CardPath(path[:], slot, name)
	Delete(p)
	fd := Open(p, O_CREAT|int32(blocks)<<16)
	if fd < 0 {
		return false
	}
	Close(fd)
	fd = Open(p, O_WRONLY)
	if fd < 0 {
		return false
	}
	n := Write(fd, &data[0], int32(len(data)))
	Close(fd)
	if n != int32(len(data)) {
		return false
	}
	// Read it back a sector at a time, so that a card pulled out half way
	// is caught here rather than when the save is loaded.
	fd = Open(p, O_RDONLY)
	if fd < 0 {
		return false
	}
	ok := true
	for i := 0; ok && i < len(data); i += CardSector {
		ok = Read(fd, &cardCheck[0], CardSector) == CardSector && [CardSector]byte(data[i:]) == cardCheck
	}
	Close(fd)
	return ok
}

// cardCheck is the sector CardWrite reads back.
var cardCheck [CardSector]byte

// CardRead reads the start of the file name on the card in slot 0 or 1
// into buf, a whole number of sectors, and returns how many bytes it read,
// or -1 if there is no such file. A program checks what it reads, with a
// mark of its own at the start of its data say, since a card can be pulled
// out while it is written.
func CardRead(slot int, name string, buf []byte) int {
	if len(buf) == 0 || len(buf)%CardSector != 0 {
		return -1
	}
	var path [32]byte
	fd := Open(CardPath(path[:], slot, name), O_RDONLY)
	if fd < 0 {
		return -1
	}
	n := Read(fd, &buf[0], int32(len(buf)))
	Close(fd)
	if n <= 0 {
		return -1
	}
	return int(n)
}

// CardFiles fills files with the files on the card in slot 0 or 1 and
// returns how many there are, up to len(files).
func CardFiles(slot int, files []DIRENTRY) int {
	if len(files) == 0 {
		return 0
	}
	var path [32]byte
	n := 0
	if Firstfile(CardPath(path[:], slot, "*"), &files[0]) == nil {
		return 0
	}
	for n = 1; n < len(files); n++ {
		if Nextfile(&files[n]) == nil {
			break
		}
	}
	return n
}

// FileName returns a DIRENTRY's name as a string, without its terminator.
// The string shares the entry's memory.
func (d *DIRENTRY) FileName() string {
	n := 0
	for n < len(d.Name) && d.Name[n] != 0 {
		n++
	}
	return unsafe.String(&d.Name[0], n)
}
