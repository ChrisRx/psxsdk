//go:build yaroze

package cleanlib

import "unsafe"

// Fixed backgrounds: a map of 16 by 16 cells drawn as sprites.

// GsBG is a background: a map, scrolled. Its window and brightness fields are
// the program's own: the library goes by the screen (see below).
type GsBG struct {
	Attribute        uint32
	X, Y             int16
	W, H             int16
	Scrollx, Scrolly int16
	R, G, B          uint8
	_                uint8
	Map              *GsMAP
	Mx, My           int16
	Scalex, Scaley   int16
	Rotate           int32
}

// GsMAP is a map: its cells' size, its size in cells, the cells, and the
// index of the cell at each place, FFFFh for none.
type GsMAP struct {
	Cellw, Cellh   uint8
	Ncellw, Ncellh uint16
	Base           *GsCELL
	Index          *uint16
}

// GsCELL is a cell: its place in a texture page, its palette, its flip
// flags and its texture page.
type GsCELL struct {
	U, V  uint8
	Cba   uint16
	Flag  uint16
	Tpage uint16
}

// GetClut returns the palette ID of a palette at x, y in video memory, as a
// packet takes it: y in bits 6 to 15, x/16 below.
//
//export GetClut
func GetClut(x, y int32) uint32 {
	return uint32(uint16(y<<6 | x>>4&0x3f))
}

// The work area holds a packet list for each buffer, built once by
// GsInitFixBg16: the addresses of the two lists, and then for each a head, a
// packet for each of the screen's cells, and a tail. GsSortFixBg16 fills in a
// list from the map and links it. The GsBG's window and brightness fields have
// no effect: everything is sized and placed by the screen, GsInitGraph's W and H
// (spec 20, "Fixed backgrounds").
const (
	bgEndBytes  = 8  // the head and the tail: a link and an E5h
	bgCellBytes = 20 // a cell's packet: link, E1h, command, point, texture
)

// bgScreen returns the cells of a list, across and down.
func bgScreen() (cols, rows int) {
	return int(gs.width)/16 + 1, int(gs.height)/16 + 1
}

// GsInitFixBg16 builds the two lists in work. A list's sprites (7Dh, or 7Fh with
// the attribute's bit 30 at this call) are placed on the 16-pixel grid of its
// buffer, that of list 1 a screen's height down whatever GsDefDispBuff said. The
// cells link on through the list, and its tail to the end until a sort links it.
//
//export GsInitFixBg16
func GsInitFixBg16(bg *GsBG, work *uint32) {
	cols, rows := bgScreen()
	code := uint32(0x7d << 24)
	if bg.Attribute&attrSemi != 0 {
		code = 0x7f << 24
	}
	base := uintptr(unsafe.Pointer(at(work)))
	p := base + 8
	for i := range uintptr(2) {
		set(base, i, uint32(p))
		set(p, 0, 1<<24|uint32(p+bgEndBytes)&0xffffff)
		set(p, 1, 0xe5000000)
		p += bgEndBytes
		for r := range rows {
			y := 16*r + int(i)*int(gs.height)
			for c := range cols {
				set(p, 0, 4<<24|uint32(p+bgCellBytes)&0xffffff)
				set(p, 1, 0xe1000000)
				set(p, 2, code)
				set(p, 3, pair(int16(16*c), int16(y)))
				set(p, 4, 0)
				p += bgCellBytes
			}
		}
		set(p, 0, 1<<24|endOfList)
		set(p, 1, 0xe5000000)
		p += bgEndBytes
	}
	spinCycles(gsInitFixBg16CellPad * uint32(cols*rows))
}

// gsInitFixBg16CellPad is the rest of GsInitFixBg16's time, for each cell of a
// screen: 31,400 cycles at 320 by 240 (spec 21), 108,600 at 640 by 480.
const gsInitFixBg16CellPad = 36

// bgFirst returns the first row or column of a map n cells long for the scroll
// s: the scroll's whole cells, wrapped to be positive. A map of no cells has only
// its first.
func bgFirst(s, n int) int {
	if n == 0 {
		return 0
	}
	return ((s>>4)%n + n) % n
}

// GsSortFixBg16 fills in the list of the buffer after the coming swap, list 1
// when buffer 0 is drawn into, and links it into ot at pri plus the table's
// Offset (the sorts of primitives do not add it), unless the attribute's bit 31 is
// set. The head moves the drawing offset by the scroll within a cell. Each packet
// takes its cell from the map at the scroll's whole cells, stepping past the map's
// end back to 0. An empty cell, FFFFh, has its tag's size set to 0 and its other
// words left, and the GPU skips it. Any other has a size of 4, and an E1h from the
// cell's texture page with the attribute's color mode and rate. The tail puts the
// offset back: in the GsOFSGTE mode to 0, and in the GsOFSGPU mode to that buffer's,
// with the screen origin.
//
//export GsSortFixBg16
func GsSortFixBg16(bg *GsBG, work *uint32, ot *GsOT, pri uint16) {
	attr := bg.Attribute
	if attr&attrHidden != 0 {
		return
	}
	cols, rows := bgScreen()
	j := 1 - activeBuf()
	head := uintptr(word(uintptr(unsafe.Pointer(at(work))), uintptr(j)))

	sx, sy := int(bg.Scrollx), int(bg.Scrolly)
	set(head, 1, 0xe5000000|uint32(-(sx&15))&0x7ff|(uint32(-(sy&15))&0x7ff)<<11)

	m := at(bg.Map)
	nw, nh := int(m.Ncellw), int(m.Ncellh)
	index := uintptr(unsafe.Pointer(at(m.Index)))
	cells := uintptr(unsafe.Pointer(at(m.Base)))
	page := semiRate(attr)<<5 | (attr>>24&3)<<7
	col0, row := bgFirst(sx, nw), bgFirst(sy, nh)
	p := head + bgEndBytes
	// Stepping past a map's end goes back to 0, and one with no columns has only that.
	wrap := max(nw, 1)
	for range rows {
		line := index + 2*uintptr(row*nw)
		end := line + 2*uintptr(wrap)
		next := line + 2*uintptr(col0)
		for range cols {
			i := half(next, 0)
			if i == 0xffff {
				*(*uint8)(unsafe.Pointer(p + 3)) = 0
			} else {
				// A cell's first word is U, V and Cba, as the packet takes them.
				cell := cells + 8*uintptr(i)
				*(*uint8)(unsafe.Pointer(p + 3)) = 4
				*(*uint16)(unsafe.Pointer(p + 4)) = uint16(uint32(half(cell, 3))&0x1f | page)
				set(p, 4, word(cell, 0))
			}
			p += bgCellBytes
			next += 2
			if next == end {
				next = line
			}
		}
		if row++; row >= nh {
			row = 0
		}
	}

	back := uint32(0xe5000000)
	if gs.ofsGPU {
		bx, by := bufCorner(j)
		back |= (uint32(bx)+uint32(position.Offx))&0x7ff | ((uint32(by)+uint32(position.Offy))&0x7ff)<<11
	}
	set(p, 1, back)
	ot = at(ot)
	e := at((*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(ot.Org)) + 4*(uintptr(pri)+uintptr(ot.Offset)))))
	set(p, 0, 1<<24|*e&0xffffff)
	*e = uint32(head) & 0xffffff
}
