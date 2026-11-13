//go:build yaroze

package cleanlib

import "unsafe"

// Images in video memory: loading them, and the TIM files they come in.

// GetTPage returns the texture page value for a texture at x, y in video
// memory, of color depth tp (0 for 4-bit, 1 for 8-bit, 2 for 16-bit), with
// the semi-transparency rate abr.
//
//export GetTPage
//go:noinline
func GetTPage(tp, abr, x, y int32) uint16 {
	v := uint16((tp&3)<<7 | (abr&3)<<5 | (y&0x100)>>4 | (x&0x3ff)>>6)
	space(4)
	pace(getTPagePad)
	return v
}

// getTPagePad is the rest of GetTPage's 96 cycles (spec 21), with the four stores
// for what its turns leave.
const getTPagePad = 26

// LoadImage copies the pixels at p, 16 bits each, into the rectangle rect
// of video memory. As Sony's library does, the CPU writes the words that
// do not make a whole block of 16, from the start of the image, and DMA
// the rest, which goes on after it returns: the next call that uses the
// GPU waits for it, and DrawSync does. A size that comes to nothing sends
// nothing. With the queue idle, the size it kept within video memory is
// left in rect too (spec 21). With the GPU busy, the image goes into the
// queue instead (imageEnter).
//
// The call spends its time where Sony's does (spec 49): after the
// transfer's watch (imageEnter) it waits 140 cycles before it reads
// GPUSTAT for the GPU to be ready and sends the image 16 after that, puts
// I_MASK back 52 cycles after the blocks start, or 60 after the last word
// with no blocks, and returns 54 after that. The blocks are then still
// going when it returns, and a DrawSync(1) straight after gives 1.
//
// On a console the blocks hold the bus while they go, and every load and
// store the call makes after starting them waits a block's time for it
// (spec 50). Sony's call takes some 20 cycles more for each block of the
// image up to 24. So the pad after the blocks start is six loads in a
// loop, which with the call's own loads on its way out come to Sony's 24,
// and the pads before it are loads in part, the longest of them a loop too,
// for the call's time with no blocks and for the lines of code a cold call
// has to fetch.
//
//export LoadImage
//go:section .text.DrawSync
//go:noinline
func LoadImage(rect *RECT, p *uint32) int32 {
	// The transfer is worked out first, while there is time to spare:
	// Sony's has 140 cycles between the transfer's watch and its read of
	// GPUSTAT, less than working it out there would leave.
	r := *rect
	r.W, r.H = imageSize(r)
	t := imageTransfer(r, unsafe.Pointer(p))
	spaceWith(loadImageEnterSpace, t.count)
	m, state := imageEnter()
	if state == enterLater {
		if queueLoad(r, unsafe.Pointer(p)) {
			spins(loadImageQueuedTurns)
			space(loadImageQueuedSpace)
			imageLeave(m)
			imageQueued()
			space(loadImageQueuedLeaveSpace)
			imageCallLeaves(m)
			return 0
		}
		m = imageRefused(m)
	}
	if state == enterIdle {
		rect.W, rect.H = r.W, r.H
	}
	switch {
	case t.none():
		spins(loadImageNoSizeTurns)
		space(loadImageNoSizeSpace)
	default:
		spins(loadImageReadyTurns)
		t.leave()
		loadTurns6()
		for gp1.Get()&gpuStatReady == 0 {
		}
		sendImage(t, unsafe.Pointer(p))
		// Each kind pads with a value of its own: with the same one the
		// compiler merges the pads' ends, and a change to one moves
		// another's time.
		switch {
		case !t.blocks():
			nops(loadImageNoBlockNops)
			loadTurns5()
		case t.head != 0:
			nops(loadImageMixedNops)
			loadTurns5()
		default:
			// The blocks are going, and each load waits for the
			// bus for a block's time on a console (spec 50).
			// Sony's call takes 20 cycles more for each block of
			// the image up to 24: six loads here and those of
			// the call's way out come to that.
			nops(loadImageBlockNops)
			loadTurns6()
		}
	}
	imageLeave(m)
	nops(loadImageLeaveNops)
	imageCallLeaves(m)
	return 0
}

// GsIMAGE is a TIM image as GsGetTimInfo describes it: its pixel mode,
// where its pixels and palette go in video memory, and where they are.
type GsIMAGE struct {
	Pmode  uint32
	Px, Py int16
	Pw, Ph uint16
	Pixel  *uint32
	Cx, Cy int16
	Cw, Ch uint16
	Clut   *uint32
}

// timHasClut is the bit of a TIM's flags that says a palette comes first.
const timHasClut = 1 << 3

// GsGetTimInfo fills info from the TIM image at im, which points at the
// word after the file's ID: the flags, then the palette's block if there
// is one, then the pixels' block. Each block is its length in bytes, its
// place and size in video memory, and the data. With no palette the palette's
// fields stay as they were: programs load whatever they last held.
//
//export GsGetTimInfo
func GsGetTimInfo(im *uint32, info *GsIMAGE) {
	// Either may be null, which is memory here (probe spec/probes/timnull).
	flags := *at(im)
	info = at(info)
	p := unsafe.Add(unsafe.Pointer(im), 4)
	// Pmode is the flags as they are, with the palette bit, which a
	// program tests to know whether there is a palette to load.
	info.Pmode = flags
	if flags&timHasClut != 0 {
		info.Cx, info.Cy, info.Cw, info.Ch, info.Clut = timBlock(p)
		p = unsafe.Add(p, *(*uint32)(p))
	}
	info.Px, info.Py, info.Pw, info.Ph, info.Pixel = timBlock(p)
}

// timBlock reads a TIM block's place and size in video memory and returns
// them with the address of its data.
func timBlock(p unsafe.Pointer) (x, y int16, w, h uint16, data *uint32) {
	h16 := unsafe.Slice((*uint16)(unsafe.Add(p, 4)), 4)
	return int16(h16[0]), int16(h16[1]), h16[2], h16[3], (*uint32)(unsafe.Add(p, 12))
}
