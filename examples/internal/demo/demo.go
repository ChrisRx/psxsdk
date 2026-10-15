//go:build yaroze

// Package demo holds what the examples ported from Sony's Net Yaroze demos
// share: the demos' own pad routines, loading data files off the disc,
// loading TIM textures, and the TMD layouts from their tmd.h. None of it is
// part of the resident library, which is why it is not in package yaroze.
//
// Like the examples, nothing here allocates after startup.
package demo

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// WORLD is the library's world coordinate system, which it represents as a
// nil pointer.
var WORLD *yaroze.GsCOORDINATE2

// IDMATRIX is the identity matrix, the library's GsIDMATRIX.
var IDMATRIX = yaroze.MATRIX{M: [3][3]int16{
	{yaroze.ONE, 0, 0},
	{0, yaroze.ONE, 0},
	{0, 0, yaroze.ONE},
}}

// Scratch is the CPU's scratchpad, 1KB of fast RAM that GsSortObject4 uses
// as work space: the demos' getScratchAddr(0).
func Scratch() *uint32 { return At(0x1f800000) }

// Data is loaded at fixed addresses between the resident library and the
// program, as the demos load it, so these conversions from integer to
// pointer are deliberate rather than the mistake go vet normally warns
// about.

// At turns a fixed load address into a word pointer.
func At(addr uintptr) *uint32 {
	return (*uint32)(unsafe.Pointer(addr))
}

// BytesAt is At for a byte pointer.
func BytesAt(addr uintptr) *uint8 {
	return (*uint8)(unsafe.Pointer(addr))
}

// Button bits, from the demos' pad.h. PadRead assembles them from the
// library's receive buffers.
const (
	PADLup    = 1 << 12
	PADLdown  = 1 << 14
	PADLleft  = 1 << 15
	PADLright = 1 << 13
	PADRup    = 1 << 4
	PADRdown  = 1 << 6
	PADRleft  = 1 << 7
	PADRright = 1 << 5
	PADL1     = 1 << 2
	PADL2     = 1 << 0
	PADR1     = 1 << 3
	PADR2     = 1 << 1
	PADstart  = 1 << 11
	PADselect = 1 << 8
)

var padBuf0, padBuf1 *uint8

// PadInit finds the library's controller receive buffers, which it refreshes
// every frame.
func PadInit() {
	yaroze.GetPadBuf(&padBuf0, &padBuf1)
}

// PadRead is the demos' PadRead: two bytes from each receive buffer, as one
// word, inverted, since the hardware reports buttons active low.
func PadRead() uint32 {
	if padBuf0 == nil || padBuf1 == nil {
		return 0
	}
	at := func(p *uint8, n uintptr) uint32 { return uint32(*(*uint8)(unsafe.Add(unsafe.Pointer(p), n))) }
	return ^(at(padBuf0, 3) | at(padBuf0, 2)<<8 | at(padBuf1, 3)<<16 | at(padBuf1, 2)<<24)
}

// A File is a data file a demo loads at a fixed address. On a Net Yaroze it
// was sent from the PC by the loader's batch script; here it is on the disc.
type File struct {
	Name string // its path on the disc, as CdSearchFile takes it
	Addr uintptr
}

// readTimeout is how many frames one CD read may take before it is given up
// on, so a missing or unreadable disc shows up as a message rather than a
// hang.
const readTimeout = 300

var (
	fileInfo yaroze.CdlFILE
	pathBuf  [64]byte
)

// Load reads files off the disc to their addresses, calling progress, if it
// is not nil, before each. It returns the name of the first that could not
// be found or read, or "" if they all arrived.
func Load(files []File, progress func(name string)) string {
	for _, f := range files {
		if progress != nil {
			progress(f.Name)
		}
		name := yaroze.CString(pathBuf[:], f.Name)
		if yaroze.CdSearchFile(&fileInfo, name) == nil {
			return f.Name
		}
		yaroze.CdReadFile(name, At(f.Addr), int32(fileInfo.Size))
		done := false
		for range readTimeout {
			if yaroze.CdReadSync(1, nil) <= 0 {
				done = true
				break
			}
			yaroze.VSync(0)
		}
		if !done {
			return f.Name
		}
	}
	return ""
}

// LoadTIM loads a TIM image at addr into video memory, with its palette if
// it has one: the demos' InitialiseTexture.
func LoadTIM(addr uintptr) yaroze.GsIMAGE {
	var image yaroze.GsIMAGE
	// The image description follows the TIM's ID word.
	yaroze.GsGetTimInfo(At(addr+4), &image)

	rect := yaroze.RECT{X: image.Px, Y: image.Py, W: int16(image.Pw), H: int16(image.Ph)}
	yaroze.LoadImage(&rect, image.Pixel)
	yaroze.DrawSync(0)

	if image.Pmode&8 != 0 {
		clut := yaroze.RECT{X: image.Cx, Y: image.Cy, W: int16(image.Cw), H: int16(image.Ch)}
		yaroze.LoadImage(&clut, image.Clut)
		yaroze.DrawSync(0)
	}
	return image
}
