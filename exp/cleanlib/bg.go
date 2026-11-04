//go:build yaroze

package cleanlib

import "unsafe"

// Fixed backgrounds: a map of 16 by 16 cells drawn as sprites.

// GsBG is a background: a window on the screen showing a map, scrolled,
// with a brightness.
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
func GetClut(x, y int32) uint16 {
	return uint16(y<<6 | x>>4&0x3f)
}

// The work area's layout: for each of the two buffers, a packet that
// moves the drawing offset by the scroll, one for each cell the window can
// show, and one that puts the offset back, in the manual's
// ((W/16+1)*(H/16+2)*6+4) words, after two words of the library's.
const (
	bgCellWords = 6 // a cell's packet: link, E1h, 7Ch, point, texture
	bgHead      = 2
)

// GsInitFixBg16 prepares the work area for bg. GsSortFixBg16 builds every
// packet each time, so there is nothing to prepare.
//
//export GsInitFixBg16
func GsInitFixBg16(bg *GsBG, work *uint32) {
}

// GsSortFixBg16 sorts bg into ot at pri: one 16 by 16 sprite (7Ch) for
// each cell of a window W/16+1 cells across and H/16+1 down, enough for
// any scroll, each after the E1h that selects its cell's texture page. The
// sprites sit on the 16-pixel grid from the window's corner, and the
// scroll within a cell moves them all by a drawing offset (E5h) set before
// them and put back after. The cells come from the map at the scroll
// divided by 16, wrapping around its edges, row by row; an empty cell,
// FFFFh, draws nothing. The sprites are placed for the buffer the table
// is drawn into after the coming GsSwapDispBuff. At the normal brightness, 128 in each of R, G
// and B, the sprites are drawn without it (7Dh). The packets go in the
// half of work for the buffer drawn into.
//
//export GsSortFixBg16
func GsSortFixBg16(bg *GsBG, work *uint32, ot *GsOT, pri uint16) {
	m := bg.Map
	if bg.Attribute&attrHidden != 0 || m == nil || m.Ncellw == 0 || m.Ncellh == 0 {
		return
	}
	cols, rows := int(bg.W)/16+1, int(bg.H)/16+1
	half := (cols*(rows+1)*bgCellWords + 4)
	base := unsafe.Add(unsafe.Pointer(work), 4*(bgHead+gs.active*half))
	words := unsafe.Slice((*uint32)(base), half)
	head, tail := words[0:2], words[2:4]
	cells := words[4:]

	sx, sy := int(bg.Scrollx), int(bg.Scrolly)
	head[1] = 0xe5000000 | uint32(-(sx&15))&0x7ff | (uint32(-(sy&15))&0x7ff)<<11

	code := uint32(0x7c)
	color := uint32(bg.B)<<16 | uint32(bg.G)<<8 | uint32(bg.R)
	if bg.R == 128 && bg.G == 128 && bg.B == 128 {
		code, color = 0x7d, 0
	}
	// The sprites are placed in the buffer the table will be drawn into,
	// the one after the coming swap, as the reference places them.
	draw := gs.buf[1-gs.active]
	ox, oy := int(draw[0])+int(position.Offx), int(draw[1])+int(position.Offy)
	x0, y0 := ox+int(bg.X), oy+int(bg.Y)
	// The offset put back is that buffer's, with the screen origin.
	tail[1] = 0xe5000000 | uint32(ox)&0x7ff | (uint32(oy)&0x7ff)<<11
	nw, nh := int(m.Ncellw), int(m.Ncellh)
	index := unsafe.Slice(m.Index, nw*nh)
	base0 := unsafe.Slice(m.Base, 1<<16)

	// The packets are linked from the head through the cells to the tail
	// and on to what the table's entry held.
	n := 1 << ot.Length
	e := &unsafe.Slice(ot.Org, n)[min(int(pri), n-1)]
	tail[0] = 1<<24 | *e&0xffffff
	next := physical(unsafe.Pointer(&tail[0]))
	k := 0
	// Built from the last cell back, so that each links to the one after.
	for r := rows - 1; r >= 0; r-- {
		my := ((sy>>4+r)%nh + nh) % nh
		for c := cols - 1; c >= 0; c-- {
			mx := ((sx>>4+c)%nw + nw) % nw
			i := index[my*nw+mx]
			if i == 0xffff {
				continue
			}
			cell := &base0[i]
			p := cells[k*bgCellWords : k*bgCellWords+5]
			k++
			p[0] = 4<<24 | next
			p[1] = 0xe1000000 | uint32(cell.Tpage)
			p[2] = code<<24 | color
			p[3] = pair(int16(x0+16*c), int16(y0+16*r))
			p[4] = uint32(cell.Cba)<<16 | uint32(cell.V)<<8 | uint32(cell.U)
			next = physical(unsafe.Pointer(&p[0]))
		}
	}
	head[0] = 1<<24 | next
	*e = *e&0xff000000 | physical(unsafe.Pointer(&head[0]))
}
