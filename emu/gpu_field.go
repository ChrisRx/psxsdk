package emu

// With an interlaced 480-line display and drawing to the displayed area off
// (GPUSTAT bit 10, from E1h), a console's GPU leaves alone the VRAM lines of
// the field being shown: it draws only into the other field's lines. The
// emulator draws every line, so a program that draws into the wrong field
// looks the same in it as one that doesn't. Under SetFieldSkip the GPU skips
// those lines as a console does, for polygons, lines, rectangles and fills,
// so that a test can see which lines a frame really reaches. Transfers and
// copies are not affected.
//
// The rows drawn are those GPUSTAT bit 31 names (drawnRows): through the
// picture, the lines the display range GP1(07h) sets, the field not being
// shown, counted from the display area's top line, as Mednafen has it, and
// outside the picture, the vertical blank among it, the even rows, as a
// console is seen to (internal/libps/spec/probes/statlines and teal): a clear
// that ends there every frame reaches the same rows every frame. The field
// changes as the picture ends (hblank), so a drawing that runs past it
// moves to the other field's lines partway through, as on a console.

// gpustatDrawDisplayed is GPUSTAT's bit 10: drawing to the displayed area
// allowed.
const gpustatDrawDisplayed = 1 << 10

// SetFieldSkip sets whether the GPU skips the shown field's lines, as
// described above. It is off unless a test turns it on, and with it off
// nothing about the emulation changes.
func (m *Machine) SetFieldSkip(on bool) { m.gpu.fieldSkip = on }

// skipsRow reports whether, under SetFieldSkip, the GPU leaves VRAM row y
// alone.
func (g *gpu) skipsRow(y int) bool {
	if !g.fieldSkip || !g.interlaced480() || g.gpustat&gpustatDrawDisplayed != 0 {
		return false
	}
	return uint32(y)&1 != g.drawnRows()
}

// interlaced480 reports whether the display is 480 lines, interlaced.
func (g *gpu) interlaced480() bool { return g.displayMode&0x24 == 0x24 }

// drawnRows is the parity of the VRAM rows a 480-line interlaced display
// has the GPU drawing into, which GPUSTAT bit 31 reads as on a console:
// through the picture, the rows of the field not being shown, counted from
// the display area's top line, and through the vertical blank 0, the even
// rows. statlines measured a console's bit 31 holding 1 through one field's
// picture and 0 through the other's and outside both.
func (g *gpu) drawnRows() uint32 {
	if line := uint32(g.scanline); line < g.pictureStart() || line >= g.pictureEnd() {
		return 0
	}
	return (g.dispY + g.gpustat>>13&1 + 1) & 1
}

// The picture's lines, as GPUSTAT shows them, from the display range
// GP1(07h) sets. statlines measured a console with the range 16 to 256,
// which GsInitGraph sets, counting from each vertical blank interrupt:
// bit 13 toggles 8 lines after it, and with it, after the odd field, bit 31
// falls as the picture ends; after the even field, bit 31 rises as the odd
// picture starts, 23 lines after it. The emulator's interrupt comes at line
// 240 of 263, so the picture is the range moved up by 16 at its start and by
// 8 at its end, 0 to 248 for GsInitGraph's.

// pictureStart is the picture's first line, kept inside the frame.
func (g *gpu) pictureStart() uint32 {
	return max(g.dispY1, 16) - 16
}

// pictureEnd is the line after the picture's last, kept inside the frame.
func (g *gpu) pictureEnd() uint32 {
	return min(max(max(g.dispY2, 8)-8, g.pictureStart()+1), gpuFrameLines-1)
}
