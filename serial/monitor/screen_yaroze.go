//go:build yaroze

package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	gp0     = (*uint32)(yaroze.Pointer(0x1f801810))
	gp1     = (*uint32)(yaroze.Pointer(0x1f801814))
	gpuStat = gp1
)

// gpuReady is GPUSTAT's bit for a GP0 command word being taken.
const gpuReady = 1 << 26

func gpuCommand(words ...uint32) {
	for _, w := range words {
		for volatile.LoadUint32(gpuStat)&gpuReady == 0 {
		}
		volatile.StoreUint32(gp0, w)
	}
}

// Colors of the monitor's screen, as GP0 takes them: blue in the high byte.
const (
	screenColor = 0x502050 // purple
	letterColor = 0xf0f0f0
)

// glyph is a letter the screen spells, 5 by 7, a row a byte with the leftmost
// dot in bit 4. A switch rather than a map, which would take the monitor's
// heap.
func glyph(c byte) [7]byte {
	switch c {
	case 'C':
		return [7]byte{0x0e, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0e}
	case 'L':
		return [7]byte{0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1f}
	case 'E':
		return [7]byte{0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x1f}
	case 'A':
		return [7]byte{0x0e, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11}
	case 'N':
		return [7]byte{0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11}
	case 'M':
		return [7]byte{0x11, 0x1b, 0x15, 0x15, 0x11, 0x11, 0x11}
	case 'O':
		return [7]byte{0x0e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0e}
	case 'I':
		return [7]byte{0x0e, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0e}
	case 'T':
		return [7]byte{0x1f, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04}
	case 'R':
		return [7]byte{0x1e, 0x11, 0x11, 0x1e, 0x14, 0x12, 0x11}
	}
	return [7]byte{}
}

// showScreen puts the monitor's screen up: a purple screen with CLEAN MONITOR
// on it in big letters, so that whoever is at the console can tell the clean
// monitor is there and waiting from a console showing nothing. It talks to
// the GPU directly rather than through the library, whose state a program
// then starts from as the disc left it, and resets the GPU first, since a
// program that returned may have left it in any state.
func showScreen() {
	volatile.StoreUint32(gp1, 0x00000000)                         // reset
	volatile.StoreUint32(gp1, 0x08000001)                         // 320 by 240, NTSC
	volatile.StoreUint32(gp1, 0x06000000|(0x260+320*8)<<12|0x260) // horizontal range
	volatile.StoreUint32(gp1, 0x07000000|(16+240)<<10|16)         // vertical range
	volatile.StoreUint32(gp1, 0x05000000)                         // display from 0,0
	gpuCommand(
		0xe1000400,             // drawing to the displayed area allowed
		0xe3000000,             // drawing area from 0,0
		0xe4000000|239<<10|319, // to 319,239
		0xe5000000,             // no offset
		0x02000000|screenColor, // fill
		0,                      // from 0,0
		240<<16|320,            // 320 by 240
	)
	const dot = 6
	for row, word := range [2]string{"CLEAN", "MONITOR"} {
		w := len(word)*6*dot - dot
		x0 := (320 - w) / 2
		y0 := 120 - 8*dot + row*9*dot
		for i := range len(word) {
			g := glyph(word[i])
			for y := range 7 {
				for x := range 5 {
					if g[y]&(0x10>>x) == 0 {
						continue
					}
					px, py := x0+(i*6+x)*dot, y0+y*dot
					gpuCommand(0x60000000|letterColor, uint32(py)<<16|uint32(px), dot<<16|dot)
				}
			}
		}
	}
	volatile.StoreUint32(gp1, 0x03000000) // display on
}
