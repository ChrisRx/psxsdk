//go:build yaroze

package libps

import "unsafe"

// Fixed backgrounds: a window onto a map of 16 by 16 cells, drawn as a
// list of sprite packets built once by GsInitFixBg16, one for each cell
// position on the screen, and filled in by GsSortFixBg16 each frame from
// the cells the scroll puts there. Two lists are built, one for each
// display buffer.

// setTag links a packet of n words to next.
func setTag(p, next unsafe.Pointer, n uint8) {
	*(*uint32)(p) = uint32(uintptr(next))&0xffffff | uint32(n)<<24
}

// GsInitFixBg16 builds the two packet lists for bg in work and records
// where each begins in work's first two words. A list is a draw offset
// packet, a cell packet for every 16 pixels of the screen and one more
// each way, and a draw offset packet to end on.
//
// libps 0x8001F7E0, 444 bytes
//
//export GsInitFixBg16
func GsInitFixBg16(bg *GsBG, work *uint32) {
	w, h := int32(screenW), int32(screenH)
	cols, rows := w>>4+1, h>>4+1
	cmd := bg.Attribute>>5&0x200 + 0x7d000000
	base := unsafe.Pointer(work)
	list := unsafe.Add(base, 8)
	*work = uint32(uintptr(list))
	*(*uint32)(unsafe.Add(base, 4)) = uint32(uintptr(base)) + 24 + uint32(cols*rows)*20
	for i := int32(0); i < 2; i++ {
		setTag(list, unsafe.Add(list, 8), 1)
		*(*uint32)(unsafe.Add(list, 4)) = 0xe5000000
		list = unsafe.Add(list, 8)
		for y := int32(0); y < h+16; y += 16 {
			for x := int32(0); x < w+16; x += 16 {
				setTag(list, unsafe.Add(list, 20), 4)
				*(*uint32)(unsafe.Add(list, 4)) = 0xe1000000
				*(*uint32)(unsafe.Add(list, 8)) = cmd
				*(*int16)(unsafe.Add(list, 12)) = int16(x)
				*(*int16)(unsafe.Add(list, 14)) = int16(y + i*h)
				list = unsafe.Add(list, 20)
			}
		}
		*(*uint32)(unsafe.Add(list, 4)) = bgOffset(i)
		list = unsafe.Add(list, 8)
	}
}

// bgOffset is the draw offset packet word that ends a list: the buffer's
// offset when the GPU applies it, none otherwise.
func bgOffset(buffer int32) uint32 {
	if gsOfsGPU == 0 {
		return 0xe5000000
	}
	x := int32(POSITION.Offx) + int32(PSDOFSX[buffer])
	y := int32(POSITION.Offy) + int32(PSDOFSY[buffer])
	return 0xe5000000 + uint32(x) + uint32(y)<<11
}

// GsSortFixBg16 fills the list for the buffer being drawn with the cells
// the scroll puts on the screen, the fraction of a cell the scroll is
// off going into the list's first draw offset, and links the list into
// the ordering table at pri.
//
// libps 0x8001F4B8, 808 bytes
//
//export GsSortFixBg16
func GsSortFixBg16(bg *GsBG, work *uint32, ot *GsOT, pri uint16) {
	attr := bg.Attribute
	if int32(attr) < 0 {
		return
	}
	cols, rows := int32(screenW)>>4+1, int32(screenH)>>4+1
	tpage := uint16(attr>>17&0x180 | attr>>23&0x60)
	var head uint32
	if PSDIDX != 0 {
		head = *work
	} else {
		head = *(*uint32)(unsafe.Add(unsafe.Pointer(work), 4))
	}
	p := unsafe.Pointer(uintptr(head))
	sx := int32(bg.Scrollx) &^ 15
	sy := int32(bg.Scrolly) &^ 15
	dx := uint32(sx-int32(bg.Scrollx)) & 0x7ff
	dy := uint32(sy-int32(bg.Scrolly)) & 0x7ff
	*(*uint32)(unsafe.Add(p, 4)) = 0xe5000000 + dx + dy<<11
	p = unsafe.Add(p, 8)
	m := bg.Map
	ncw, nch := int32(m.Ncellw), int32(m.Ncellh)
	col := cellAt(sx, ncw)
	row := cellAt(sy, nch)
	for range rows {
		rowBase := unsafe.Add(unsafe.Pointer(m.Index), row*ncw*2)
		c := col
		cell := unsafe.Add(rowBase, c*2)
		for range cols {
			n := *(*uint16)(cell)
			if n == 0xffff {
				*(*uint8)(unsafe.Add(p, 3)) = 0
			} else {
				*(*uint8)(unsafe.Add(p, 3)) = 4
				cl := (*GsCELL)(unsafe.Add(unsafe.Pointer(m.Base), int(n)*8))
				*(*uint16)(unsafe.Add(p, 4)) = cl.Tpage&0x1f | tpage
				*(*uint32)(unsafe.Add(p, 16)) = *(*uint32)(unsafe.Pointer(cl))
			}
			p = unsafe.Add(p, 20)
			c++
			cell = unsafe.Add(cell, 2)
			if c >= ncw {
				cell = rowBase
				c = 0
			}
		}
		row++
		if row >= nch {
			row = 0
		}
	}
	*(*uint32)(unsafe.Add(p, 4)) = bgOffset(int32(PSDIDX+1) & 1)
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), (int32(pri)+int32(ot.Offset))*4))
	*(*uint32)(p) = *entry
	*(*uint8)(unsafe.Add(p, 3)) = 1
	*entry = head
	*(*uint8)(unsafe.Add(unsafe.Pointer(entry), 3)) = 0
}

// cellAt is which cell of n a scroll of s pixels, a multiple of 16,
// begins on, wrapping around the map either way.
func cellAt(s, n int32) int32 {
	if s < 0 {
		s += 15
	}
	c := (s >> 4) % n
	if c < 0 {
		c += n
	}
	return c
}
