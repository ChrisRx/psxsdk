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
//
// Talking to a card takes the kernel a moment, during which the program
// waits, so a program asks when it starts and when told to, not every
// frame.

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

const (
	// CardBlock is the size of a card's block, the unit files are made
	// in.
	CardBlock = 8192

	// CardSector is the unit a card is read and written in: a read or a
	// write is a whole number of them.
	CardSector = 128
)

// A CardState is what CardTest says of a slot.
type CardState int

const (
	CardNone        CardState = 0 // no card
	CardPresent     CardState = 1 // a card, as before
	CardNew         CardState = 2 // a card put in since the last test
	CardError       CardState = 3 // a card that did not answer properly
	CardUnformatted CardState = 4
)

var (
	// ErrCard is returned when the kernel refuses a card, or the card
	// read back does not hold what was written.
	ErrCard error = apiError("yaroze: the memory card refused")

	// ErrNoFile is returned when a card has no file of the name.
	ErrNoFile error = apiError("yaroze: no such file on the card")

	// ErrSectors is returned for data that is not a whole number of
	// sectors.
	ErrSectors error = apiError("yaroze: card data must be whole sectors")
)

// CardTest asks whether a card is in the slot and what state it is in.
func CardTest(slot int) CardState { return testCard(slot & 1) }

// A SaveHeader is the 128 bytes Sony's convention puts at the start of
// a save file, which the console's card screen and other games read to
// show it: "SC", the number of icon frames, the file's blocks, its title
// in Shift-JIS, and the icon's palette. The icon's frames, 16 by 16
// pixels of 4 bits each, 128 bytes a frame, follow it, and then the
// game's data.
type SaveHeader struct {
	// Frames is how many frames the icon has, 1 to 3, which the card
	// screen shows in turn. Sony asked European games for 1.
	Frames int

	// Blocks is the file's size in blocks.
	Blocks int

	// Title is the name the card screen shows, in Shift-JIS, up to 32
	// full-width characters. SetTitle makes one from ASCII.
	Title [64]byte

	// CLUT is the icon's palette, sixteen colors as a TIM's.
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
	b[2] = byte(0x10 + max(h.Frames, 1))
	b[3] = byte(h.Blocks)
	copy(b[4:68], h.Title[:])
	for i, c := range h.CLUT {
		b[96+2*i], b[97+2*i] = byte(c), byte(c>>8)
	}
}

// ShiftJIS returns the full-width Shift-JIS character for an ASCII one,
// from the table in Sony's FAQ, or the full-width space for anything it
// has none for.
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

// A CardFile is a file on a card as the kernel lists it.
type CardFile struct {
	name   [20]byte
	attr   int32
	size   int32
	next   *CardFile
	head   int32
	system [4]byte
}

// Name is the file's name.
func (f *CardFile) Name() string {
	n := 0
	for n < len(f.name) && f.name[n] != 0 {
		n++
	}
	return unsafe.String(&f.name[0], n)
}

// Size is the file's size in bytes, a whole number of blocks.
func (f *CardFile) Size() int { return int(f.size) }

// CardPath writes the C string naming a file on the card in slot 0 or 1,
// "bu00:name" or "bu10:name", into buf and returns it for the file
// functions. buf must hold the name and six bytes more.
func CardPath(buf []byte, slot int, name string) *byte {
	dev := "bu00:"
	if slot&1 == 1 {
		dev = "bu10:"
	}
	n := copy(buf, dev)
	n += copy(buf[n:len(buf)-1], name)
	buf[n] = 0
	return &buf[0]
}

// CardWrite makes the file on the card in the slot, as big as data
// needs in whole blocks, in place of one of the name already there,
// writes data to it, a whole number of sectors, and reads it back to see
// that it is there. A save file's data starts with its SaveHeader and
// icon.
func CardWrite(slot int, name string, data []byte) error {
	if len(data) == 0 || len(data)%CardSector != 0 {
		return ErrSectors
	}
	blocks := (len(data) + CardBlock - 1) / CardBlock
	var path [32]byte
	p := CardPath(path[:], slot, name)
	fileDelete(p)
	fd := fileOpen(p, O_CREAT|int32(blocks)<<16)
	if fd < 0 {
		return ErrCard
	}
	fileClose(fd)
	fd = fileOpen(p, O_WRONLY)
	if fd < 0 {
		return ErrCard
	}
	n := fileWrite(fd, &data[0], int32(len(data)))
	fileClose(fd)
	if n != int32(len(data)) {
		return ErrCard
	}
	// Read it back a sector at a time, so that a card pulled out half
	// way is caught here rather than when the save is loaded.
	fd = fileOpen(p, O_RDONLY)
	if fd < 0 {
		return ErrCard
	}
	ok := true
	for i := 0; ok && i < len(data); i += CardSector {
		ok = fileRead(fd, &cardCheck[0], CardSector) == CardSector && [CardSector]byte(data[i:]) == cardCheck
	}
	fileClose(fd)
	if !ok {
		return ErrCard
	}
	return nil
}

// cardCheck is the sector CardWrite reads back.
var cardCheck [CardSector]byte

// CardRead reads the start of the file on the card in the slot into
// buf, a whole number of sectors, and returns how many bytes it read. A
// program checks what it reads, with a mark of its own at the start of
// its data say, since a card can be pulled out while it is written.
func CardRead(slot int, name string, buf []byte) (int, error) {
	if len(buf) == 0 || len(buf)%CardSector != 0 {
		return 0, ErrSectors
	}
	var path [32]byte
	fd := fileOpen(CardPath(path[:], slot, name), O_RDONLY)
	if fd < 0 {
		return 0, ErrNoFile
	}
	n := fileRead(fd, &buf[0], int32(len(buf)))
	fileClose(fd)
	if n <= 0 {
		return 0, ErrCard
	}
	return int(n), nil
}

// CardFiles fills files with the files on the card in the slot and
// returns how many there are, up to len(files).
func CardFiles(slot int, files []CardFile) int {
	if len(files) == 0 {
		return 0
	}
	var path [32]byte
	if !fileFirst(CardPath(path[:], slot, "*"), &files[0]) {
		return 0
	}
	n := 1
	for ; n < len(files); n++ {
		if !fileNext(&files[n]) {
			break
		}
	}
	return n
}

// CardDelete deletes the file on the card in the slot.
func CardDelete(slot int, name string) error {
	var path [32]byte
	if fileDelete(CardPath(path[:], slot, name)) != 1 {
		return ErrNoFile
	}
	return nil
}

// CardFormat formats the card in the slot, which writes its directory
// and loses whatever was on it.
func CardFormat(slot int) error {
	var path [8]byte
	if fileFormat(CardPath(path[:], slot, "")) != 1 {
		return ErrCard
	}
	return nil
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
