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
// The line skipped is the one whose parity matches the field being shown,
// counted from the display area's top line, as Mednafen has it. The field
// changes at the end of the vertical blank (hblank), so a drawing that runs
// past it moves to the other field's lines partway through, as on a console.

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
	if !g.fieldSkip || g.displayMode&0x24 != 0x24 || g.gpustat&gpustatDrawDisplayed != 0 {
		return false
	}
	field := g.gpustat >> 13 & 1
	return (uint32(y)^(g.dispY+field))&1 == 0
}
