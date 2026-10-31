//go:build yaroze

package libps

import "unsafe"

// GsGetTimInfo reads a TIM's header, from the word after its id, into
// im: the pixel mode, where the pixels go in video memory and where they
// are in the file, and the same for the palette when the file has one.
//
// libps 0x8002297C, 228 bytes
//
//export GsGetTimInfo
func GsGetTimInfo(tim *uint32, im *GsIMAGE) {
	w := func(off uintptr) uint32 { return *(*uint32)(unsafe.Add(unsafe.Pointer(tim), off)) }
	h := func(off uintptr) uint16 { return *(*uint16)(unsafe.Add(unsafe.Pointer(tim), off)) }
	im.Pmode = w(0)
	if im.Pmode&8 == 0 {
		im.Px, im.Py = int16(h(8)), int16(h(10))
		im.Pw, im.Ph = h(12), h(14)
		im.Pixel = (*uint32)(unsafe.Add(unsafe.Pointer(tim), 16))
		return
	}
	// The palette block comes first: its length in bytes, its place and
	// size, and its colors; the pixel block follows at the length.
	pix := 4 + uintptr(w(4)&^3)
	im.Cx, im.Cy = int16(h(8)), int16(h(10))
	im.Cw, im.Ch = h(12), h(14)
	im.Clut = (*uint32)(unsafe.Add(unsafe.Pointer(tim), 16))
	im.Px, im.Py = int16(h(pix+4)), int16(h(pix+6))
	im.Pw, im.Ph = h(pix+8), h(pix+10)
	im.Pixel = (*uint32)(unsafe.Add(unsafe.Pointer(tim), pix+12))
}

// ClearImage fills rect with a color, through the queue.
//
// libps 0x800282E0, 148 bytes
//
//export ClearImage
func ClearImage(rect *RECT, r, g, b uint8) int32 {
	checkRect("ClearImage", rect)
	return addque(gpuClr, unsafe.Pointer(rect), 8, uint32(b)<<16|uint32(g)<<8|uint32(r))
}

// StoreImage reads rect out of video memory into p, through the queue.
//
// libps 0x800283D8, 100 bytes
//
//export StoreImage
func StoreImage(rect *RECT, p *uint32) int32 {
	checkRect("StoreImage", rect)
	return addque(gpuDrs, unsafe.Pointer(rect), 8, uint32(uintptr(unsafe.Pointer(p))))
}

// moveBlock is the packet MoveImage sends: a copy within video memory,
// from the rectangle to x, y.
//
// libps 0x8004838C, 20 bytes
var moveBlock = [5]uint32{0x04ffffff, 0x80000000}

// MoveImage copies rect to x, y in video memory, through the queue. A
// rectangle of no width or height is refused with -1.
//
// libps 0x8002843C, 196 bytes
//
//export MoveImage
func MoveImage(rect *RECT, x, y int32) int32 {
	checkRect("MoveImage", rect)
	if rect.W == 0 || rect.H == 0 {
		return -1
	}
	moveBlock[2] = *(*uint32)(unsafe.Pointer(rect))
	moveBlock[3] = uint32(y)<<16 | uint32(x)&0xffff
	moveBlock[4] = *(*uint32)(unsafe.Add(unsafe.Pointer(rect), 4))
	return addque(gpuCwc, unsafe.Pointer(&moveBlock[0]), 20, 0)
}
