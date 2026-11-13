//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The matrix, image and CD audio functions the sound examples use besides
// sound.

// RotMatrixY turns m about the Y axis by r, 4096 to a turn: m becomes the
// turn times m. In assembly.
//
//export RotMatrixY
func RotMatrixY(r int32, m *MATRIX) *MATRIX

// RotMatrixZ turns m about the Z axis by r, 4096 to a turn: m becomes the
// turn times m. In assembly.
//
//export RotMatrixZ
func RotMatrixZ(r int32, m *MATRIX) *MATRIX

// ApplyMatrix multiplies v0 by m's rotation on the GTE, which it leaves holding m's,
// into v1, which it returns: each element the sum of three products shifted right by
// 12, as MAC1 to MAC3 hold it. In assembly.
//
//export ApplyMatrix
func ApplyMatrix(m *MATRIX, v0 *SVECTOR, v1 *VECTOR) *VECTOR

// GsScaleScreen makes GsWSMATRIX the matrix the last GsSetRefView2 or GsSetView2
// left, scaled against the world, so that calling it twice does not scale twice.
// The rotation goes as ScaleMatrix scales it, each column by scale's element for
// it, 4096 being 1, and each element of the translation by scale's element for its
// row, rounded towards zero in 32 bits.
//
//export GsScaleScreen
func GsScaleScreen(scale *SVECTOR) {
	s := [3]int32{int32(scale.Vx), int32(scale.Vy), int32(scale.Vz)}
	for i := range 3 {
		for j := range 3 {
			gsWSMATRIX.M[i][j] = int16(int32(wsUnscaled.M[i][j]) * s[j] >> 12)
		}
		gsWSMATRIX.T[i] = wsUnscaled.T[i] * s[i] / 4096
	}
	pace(gsScaleScreenPad)
}

// gsScaleScreenPad is the rest of GsScaleScreen's 289 cycles (spec 22).
const gsScaleScreenPad = 86

// gteMIMefunc moves n vertices at otp by the differences at dfp scaled by
// p, 4096 being 1: each coordinate becomes o plus d·p >> 12 saturated to 16 bits,
// wrapping at 16 bits. The GTE does the multiply, with GPF: IR holds the difference
// and IR0 the scale. In assembly.
//
//export gteMIMefunc
func gteMIMefunc(otp, dfp *SVECTOR, n, p int32)

// PushMatrix saves the GTE's rotation and translation on a stack of
// twenty, and PopMatrix restores them from it. In assembly.
//
//export PushMatrix
func PushMatrix()

//export PopMatrix
func PopMatrix()

// StoreImage copies the rectangle rect of video memory to p. As LoadImage
// does, the CPU reads the words that do not make a whole block of 16, from
// the start of the image, and DMA the blocks (GP1 04000003h, as Sony's
// library sends for an image of whole blocks and not for one of 8 words,
// spec 13's probes and spec 08's); the call does not wait for the DMA:
// the next call that uses the GPU sees the channel busy, and DrawSync waits.
// A size that comes to nothing moves nothing: the GPU would have no pixels to
// wait for. With the queue idle, the size kept within video memory is left in
// rect too (spec 21). With the GPU busy, the call goes into the queue and p
// is written when its turn comes (imageEnter). It spends its time where
// Sony's does, as LoadImage (spec 49).
//
//export StoreImage
//go:section .text.DrawSync
//go:noinline
func StoreImage(rect *RECT, p *uint32) int32 {
	// The transfer is worked out first, as LoadImage's.
	r := *rect
	r.W, r.H = imageSize(r)
	t := imageTransfer(r, unsafe.Pointer(p))
	spaceWith(storeImageEnterSpace, t.count)
	m, state := imageEnter()
	if state == enterLater {
		if queueStore(r, unsafe.Pointer(p)) {
			spins(storeImageQueuedTurns)
			space(storeImageQueuedSpace)
			imageLeave(m)
			imageQueued()
			space(storeImageQueuedLeaveSpace)
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
		spins(storeImageNoSizeTurns)
		loads(storeImageNoSizeLoads)
		space(storeImageNoSizeSpace)
	default:
		spins(storeImageReadyTurns)
		t.leave()
		loads(storeImageReadyLoads)
		space(storeImageReadySpace)
		for gp1.Get()&gpuStatReady == 0 {
		}
		readImage(t, p)
		// Each kind pads with a value of its own: with the same one the
		// compiler merges the pads' ends, and a change to one moves
		// another's time.
		switch {
		case !t.blocks():
			loads(storeImageNoBlockLoads)
			spaceWith(storeImageNoBlockSpace, 1)
			nops(storeImageNoBlockNops)
		case t.head != 0:
			spaceWith(storeImageMixedSpace, 2)
			nops(storeImageMixedNops)
		default:
			loadTurns7()
			nops(storeImageBlockNops)
		}
	}
	imageLeave(m)
	nops(storeImageLeaveNops)
	imageCallLeaves(m)
	return 0
}

// readImage reads video memory into p, for StoreImage, with the GPU ready
// for it: the CPU's words, then the blocks by DMA, which it does not wait
// for. The stores between its accesses bring each to Sony's cycle (spec
// 49): from GP1 04000000h, the four GP0 words at 8, 16, 26 and 36 cycles,
// GPUSTAT read at 44, the CPU's words from 70, 16 apart, and GP1 04000003h
// 20 after the last of them, or 72 with none; D2_MADR, D2_BCR and D2_CHCR
// 8, 18 and 26 after that.
//
//go:inline
func readImage(t transfer, p *uint32) {
	// The DMA direction off first, as the reference does.
	gp1.Set(gp1DMAOff)
	nops(readImageClearNops)
	gp0.Set(0x01000000)
	loads(readImageCommandSpace)
	gp0.Set(0xc0000000)
	loads(readImagePlaceSpace)
	gp0.Set(t.place)
	loads(readImageSizeSpace)
	gp0.Set(t.size)
	loads(readImageSendSpace)
	// GPUSTAT read until the GPU has the pixels to send.
	for gp1.Get()&gpuStatSend == 0 {
	}
	if t.head != 0 {
		space(readImageWordsSpace)
		// The pad comes between two words, not after the last.
		q := unsafe.Pointer(at(p))
		for n := t.head; ; {
			*(*uint32)(q) = gp0.Get() // GPUREAD shares GP0's address
			q = unsafe.Add(q, 4)
			if n--; n == 0 {
				break
			}
			space(storeImageWordSpace)
			nops(storeImageWordNops)
		}
	}
	if t.blocks() {
		if t.head == 0 {
			nops(readImageWholeNops)
		} else {
			space(readImageMixedSpace)
		}
		gp1.Set(gp1DMAFromGPU)
		loads(readImageAddrSpace)
		dmaGPUAddr.Set(t.addr)
		loads(readImageCountSpace)
		nops(readImageCountNops)
		dmaGPUBlock.Set(t.count)
		nops(readImageStartNops)
		dmaGPUControl.Set(dmaFromGPU)
	}
}

// readImage's pads, in 2-cycle stores.
const (
	readImageClearNops    = 1
	readImageCommandSpace = 2
	readImagePlaceSpace   = 4
	readImageSizeSpace    = 4
	readImageSendSpace    = 2
	readImageWordsSpace   = 1
	readImageWholeNops    = 1
	readImageMixedSpace   = 0
	readImageAddrSpace    = 2
	readImageCountSpace   = 1
	readImageCountNops    = 1
	readImageStartNops    = 0
)

// ClearImage fills the rectangle rect of video memory with r, g, b, queued
// with the drawing, its size kept to 0 to 1023 by 0 to 511. As the reference
// does it, a rectangle whose x and kept width are multiples of 64 takes the
// GPU's fill (02h). Any other is drawn as a flat rectangle (60h) with the
// drawing area opened to all of video memory and its offset at 0, and the area
// and offset the GPU reports as the command goes out put back after it. When
// the request starts at once, the kept size is left in rect too; one that has
// to wait works from a copy (spec 21).
//
// With the GPU free its accesses come at Sony's cycles (spec 49, probe
// spec/probes/imgsync): packetEnter's, and then Sony's builds its packet,
// which takes it 156 cycles from the read of GPUSTAT to the channel's start
// for a fill, 176 to the first GP1 10h for a rectangle off a multiple of
// 64 and 186 for one on it whose width is not, 6 more with a width over
// 1,023, 4 more with a height over 511 and 18 less with a negative width.
// The rectangle's three questions of the GPU come 34 and 36 cycles apart,
// each answer read 8 cycles after it is asked, and the channel starts 42
// after the last. I_MASK is put back 48 cycles after the channel starts.
//
//export ClearImage
//go:section .text.MoveImage
//go:noinline
func ClearImage(rect *RECT, r, g, b uint8) int32 {
	color := uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	w, h := int16(clamp(int(rect.W), 1023)), int16(clamp(int(rect.H), 511))
	place, size := pair(rect.X, rect.Y), pair(w, h)
	// How Sony's draws it: the GPU's fill, or a rectangle, whose time
	// goes by which of the two tests it fails.
	kind := clearFill
	if rect.X&63 != 0 {
		kind = clearOff
	} else if w&63 != 0 {
		kind = clearOdd
	}
	packetRoom()
	spaceWith(clearImageEnterSpace, size)
	m, state, q := packetEnter()
	q.list = 0
	// Sony's builds its packet here. What it takes by the size:
	if rect.W > 1023 {
		space(clearImageWideSpace)
	}
	if rect.H > 511 {
		space(clearImageTallSpace)
	}
	if rect.W >= 0 {
		space(clearImageWidthSpace)
	}
	switch kind {
	case clearOff:
		space(clearImageOffSpace)
	case clearOdd:
		space(clearImageOddSpace)
	}
	fill := kind == clearFill
	if fill {
		// The mask bits off (E6h), then the fill.
		q.restore = 0
		q.env[0] = 4<<24 | endOfList
		q.env[1] = 0xe6000000
		q.env[2] = 0x02000000 | color
		q.env[3] = place
		q.env[4] = size
	} else {
		// Words 8 to 10 are E3h, E4h and E5h, filled in as it goes out.
		q.restore = 8
		q.env[0] = 10<<24 | endOfList
		q.env[1] = 0xe3000000
		q.env[2] = 0xe4ffffff
		q.env[3] = 0xe5000000
		q.env[4] = 0xe6000000
		q.env[5] = 0x60000000 | color
		q.env[6] = place
		q.env[7] = size
	}
	if state == enterLater {
		spins(clearImageQueuedTurns)
		space(clearImageQueuedSpace)
		if fill {
			space(clearImageQueuedFillSpace)
		}
		imageLeave(m)
		imageQueued()
		space(clearImageQueuedLeaveSpace)
		imageCallLeaves(m)
		return 0
	}
	if fill {
		space(clearImageFillSpace)
	} else {
		space(clearImageRectSpace)
		gp1.Set(gp1InfoDrawArea1)
		nops(clearImageAnswerNops)
		q.env[8] = 0xe3<<24 | gp0.Get()&0xffffff // GPUREAD
		space(clearImageAskSpace)
		gp1.Set(gp1InfoDrawArea1 + 1)
		nops(clearImageAnswer2Nops)
		q.env[9] = 0xe4<<24 | gp0.Get()&0xffffff
		space(clearImageAsk2Space)
		gp1.Set(gp1InfoDrawArea1 + 2)
		nops(clearImageAnswer2Nops)
		q.env[10] = 0xe5<<24 | gp0.Get()&0xffffff
		space(clearImageLastSpace)
	}
	packetStart(q, 2, 0, 2)
	rect.W, rect.H = w, h
	// Loads for part of the pad, more of them for a fill: Sony's takes
	// some 100 cycles longer than stores alone on a console, a rectangle,
	// and 150 a fill (spec 50).
	if fill {
		loads(clearImageLeaveFillLoads)
		nops(clearImageLeaveFillNops)
		spaceWith(clearImageLeaveFillSpace, 1)
	} else {
		loads(clearImageLeaveRectLoads)
		nops(clearImageLeaveRectNops)
		spaceWith(clearImageLeaveRectSpace, 2)
	}
	imageLeave(m)
	nops(clearImageLeaveNops)
	imageCallLeaves(m)
	return 0
}

// How ClearImage draws a rectangle (spec 49).
const (
	clearFill = iota // x and the width on multiples of 64: the GPU's fill
	clearOff         // x off a multiple of 64: a rectangle
	clearOdd         // x on one and the width not: a rectangle
)

// ClearImage's pads, in 2-cycle stores, turns of spin and nops.
const (
	clearImageEnterSpace       = 6
	clearImageWideSpace        = 2
	clearImageTallSpace        = 1
	clearImageWidthSpace       = 5
	clearImageOffSpace         = 3
	clearImageOddSpace         = 5
	clearImageFillSpace        = 0
	clearImageRectSpace        = 0
	clearImageAnswerNops       = 0
	clearImageAskSpace         = 4
	clearImageAnswer2Nops      = 3
	clearImageAsk2Space        = 9
	clearImageLastSpace        = 7
	clearImageLeaveFillLoads   = 15
	clearImageLeaveFillNops    = 1
	clearImageLeaveFillSpace   = 0
	clearImageLeaveRectLoads   = 0
	clearImageLeaveRectNops    = 12
	clearImageLeaveRectSpace   = 4
	clearImageLeaveNops        = 3
	clearImageQueuedTurns      = 8
	clearImageQueuedSpace      = 0
	clearImageQueuedFillSpace  = 9
	clearImageQueuedLeaveSpace = 9
)

// The drive's commands and Setmode bits for CD audio (Play is in cd.go).
const (
	cdlGetTN         = 0x13
	cdlGetTD         = 0x14
	cdlModeCDDA      = 0x01
	cdlModeAutoPause = 0x02
	cdStatPlay       = 0x80 // the drive's status while it plays
)

// cdAudio is CdPlay's state: the list of tracks, which one is playing,
// whether the list plays round again, where each track of the disc
// starts (minutes and seconds, BCD, from GetTD) and whether the drive has
// reached the end of the track playing (its data end interrupt, from
// autopause).
var cdAudio struct {
	tracks  *int32
	index   int32
	last    int32 // the disc's last track
	repeat  bool
	playing bool
	starts  [100][2]uint8
	ended   uint32
	endTime uint32

	// answer, track, mode and loc are what CdPlay hands the drive, kept here
	// since the compiler would put locals on the heap, which the image's
	// 32 KB never gets back.
	answer      [8]uint8
	track, mode uint8
	loc         [4]uint8
}

// CdPlay plays the tracks at tracks, a list ending in 0, from offset on:
// once with mode 1, round again with mode 2 (any mode other than 0 and 3 starts a
// list, once). Mode 0 stops and mode 3 keeps the list going, as a program calls it
// every frame. It returns the index of the track playing, or -1.
//
// As the reference does it: modes 1 and 2 read the disc's table of
// contents (GetTN, then GetTD for every track from 0), set CD audio with
// autopause, and play the track from its start (Setloc, Play). The drive
// pauses at the end of each track. Mode 3 moves on to the next track once
// the drive has reported that end; while the drive does not say it is playing
// it plays the track again; otherwise it only asks the status (Getstat), and
// does not wait for the answer (974 cycles on Sony's, 54,202 waiting).
//
//export CdPlay
func CdPlay(mode int32, tracks *int32, offset int32) int32 {
	switch mode {
	case 0:
		if cdAudio.playing {
			cd.controlEv(cdlPause, nil, nil)
			cdAudio.playing = false
		}
	case 3:
		if !cdAudio.playing {
			break
		}
		switch {
		case volatile.LoadUint32(&cdAudio.ended) != 0:
			volatile.StoreUint32(&cdAudio.ended, 0)
			cdDataEndLine(cdAudio.index, int32(volatile.LoadUint32(&cdAudio.endTime)))
			cdAudio.index++
			if cdTrack(cdAudio.index) == 0 {
				if !cdAudio.repeat {
					cdAudio.playing = false
					break
				}
				cdAudio.index = 0
			}
			cdAudio.playing = cdPlayTrack()
		case cd.resp[0]&cdStatPlay == 0:
			cdPlayLoc()
		default:
			// Sent without a wait for the answer: a program calls this
			// every frame.
			CdControlF(cdlGetstat, nil)
			pace(cdPlayStatPad)
		}
	default:
		r := &cdAudio.answer
		cd.control(cdlGetTN, nil, &r[0])
		last := unbcd(r[2])
		cdAudio.last = last
		for t := range last + 1 {
			spinCycles(cdPlayTOCPad)
			cdAudio.track = bcd(t)
			cd.control(cdlGetTD, &cdAudio.track, &r[0])
			cdAudio.starts[t] = [2]uint8{r[1], r[2]}
		}
		spinCycles(cdPlayTOCPad)
		cdAudio.mode = cdlModeCDDA | cdlModeAutoPause
		cd.controlEv(cdlSetmode, &cdAudio.mode, nil)
		cdAudio.tracks, cdAudio.repeat = tracks, mode == 2
		volatile.StoreUint32(&cdAudio.ended, 0)
		// An offset at or past the list's end goes back to its start, when
		// it goes round. An empty list then plays what the table holds at
		// 0, the end of the disc.
		cdAudio.index = offset
		if offset >= cdListLen() {
			if !cdAudio.repeat {
				cdAudio.playing = false
				break
			}
			cdAudio.index = 0
		}
		cdAudio.playing = cdPlayTrack()
	}
	if !cdAudio.playing {
		return -1
	}
	return cdAudio.index
}

// cdPlayStatPad is the rest of the time of a CdPlay(3) that only asks the status.
const cdPlayStatPad = 334

// cdTrack returns the track number at index i of the list.
func cdTrack(i int32) int32 {
	return *(*int32)(unsafe.Add(unsafe.Pointer(cdAudio.tracks), 4*i))
}

// cdListLen returns the tracks in the list before its 0. The load is volatile, or the
// compiler makes the loop a call of wcslen.
func cdListLen() int32 {
	n := int32(0)
	for volatile.LoadUint32((*uint32)(unsafe.Add(unsafe.Pointer(cdAudio.tracks), 4*n))) != 0 {
		n++
	}
	return n
}

// cdPlayTrack plays the track at cdAudio.index from its start. It reports whether
// the drive was asked to: always. A track past the disc's last gets its message and
// plays the last, a negative one plays from the disc's very start, and the 0 of an
// empty list the table's entry 0, the end of the disc.
func cdPlayTrack() bool {
	t := cdTrack(cdAudio.index)
	if t > cdAudio.last {
		ttyPrintf("%d: track overflow\n", uint32(t), 0)
		t = cdAudio.last
	}
	cdAudio.loc = [4]uint8{}
	if t >= 0 {
		cdAudio.loc = [4]uint8{cdAudio.starts[t][0], cdAudio.starts[t][1], 0, 0}
	}
	cdPlayLoc()
	return true
}

// cdPlayLoc plays from the place cdAudio.loc holds, the track last asked for.
func cdPlayLoc() {
	cd.control(cdlSetloc, &cdAudio.loc[0], nil)
	cd.controlEv(cdlPlay, nil, nil)
	spinCycles(cdPlayLocPad)
}

// What CdPlay spends past its drive commands, for Sony's times: after Play, and before
// each command of the table's reading.
const (
	cdPlayLocPad = 224
	cdPlayTOCPad = 568
)

// cdAudioEnd is the drive's data end interrupt, from the interrupt
// handler: autopause has stopped the drive at the end of the track.
func cdAudioEnd() {
	volatile.StoreUint32(&cdAudio.endTime, volatile.LoadUint32(&vsyncBlanks))
	volatile.StoreUint32(&cdAudio.ended, 1)
}

// cdDataEndLine prints the line the reference prints at each data end:
// the index in the list of the track that ended and VSync(-1) when it did,
// in decimal.
func cdDataEndLine(index, time int32) {
	ttyPrintf("cbdataready: CdlDataEnd (track=%d,time=%d)\n", uint32(index), uint32(time))
}

func unbcd(b uint8) int32 { return int32(b>>4)*10 + int32(b&15) }
