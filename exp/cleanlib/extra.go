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

// StoreImage copies the rectangle rect of video memory to p. As loadImage
// does, the CPU reads the words that do not make a whole block of 16, from
// the start of the image, and DMA the blocks (GP1 04000003h, as Sony's
// library sends for an image of whole blocks and not for one of 8 words,
// spec 13's probes and spec 08's); the call does not wait for the DMA:
// the next call that uses the GPU sees the channel busy, and DrawSync waits.
// A size that comes to nothing moves nothing: the GPU would have no pixels to
// wait for. With the queue idle, the size kept within video memory is left in
// rect too (spec 21).
//
//export StoreImage
//go:noinline
func StoreImage(rect *RECT, p *uint32) int32 {
	w, h := imageSize(*rect)
	m, idle := imageEnter()
	if idle {
		rect.W, rect.H = w, h
	}
	if w == 0 || h == 0 {
		queueLeave(m)
		pace(storeImagePad)
		imageCallLeaves()
		return 0
	}
	// The DMA direction off first, as the reference does.
	gp1.Set(gp1DMAOff)
	gp0.Set(0x01000000)
	gp0.Set(0xc0000000)
	gp0.Set(pair(rect.X, rect.Y))
	gp0.Set(pair(w, h))
	// GPUSTAT read until the GPU has the pixels to send.
	for gp1.Get()&gpuStatSend == 0 {
	}
	words := (int(w)*int(h) + 1) / 2
	const block = 16
	head := words % block
	dst := unsafe.Slice(at(p), head)
	for i := range dst {
		dst[i] = gp0.Get() // GPUREAD shares GP0's address
		space(storeImageWordSpace)
	}
	if n := words / block; n > 0 {
		gp1.Set(gp1DMAFromGPU)
		// The pointer as given, as Sony's sends it: the channel uses its
		// low 24 bits.
		dmaGPUAddr.Set(uint32(uintptr(unsafe.Add(unsafe.Pointer(p), 4*head))))
		dmaGPUBlock.Set(uint32(n)<<16 | block)
		dmaGPUControl.Set(dmaFromGPU)
		space(storeImageBlockSpace)
		if head == 0 {
			space(storeImageWholeSpace)
		}
	} else {
		space(storeImageNoBlockSpace)
	}
	queueLeave(m)
	pace(storeImagePad)
	imageCallLeaves()
	return 0
}

// ClearImage fills the rectangle rect of video memory with r, g, b, queued
// with the drawing, its size kept to 0 to 1023 by 0 to 511. As the reference
// does it, a rectangle whose x and kept width are multiples of 64 takes the
// GPU's fill (02h). Any other is drawn as a flat rectangle (60h) with the
// drawing area opened to all of video memory and its offset at 0, and the area
// and offset the GPU reports as the command goes out put back after it. When
// the request starts at once, the kept size is left in rect too; one that has
// to wait works from a copy (spec 21).
//
//export ClearImage
//go:noinline
func ClearImage(rect *RECT, r, g, b uint8) int32 {
	color := uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	w, h := int16(clamp(int(rect.W), 1023)), int16(clamp(int(rect.H), 511))
	if rect.X%64 == 0 && w%64 == 0 {
		// The mask bits off (E6h), then the fill.
		p := [5]uint32{
			4<<24 | endOfList,
			0xe6000000,
			0x02000000 | color,
			pair(rect.X, rect.Y),
			pair(w, h),
		}
		if enqueue(0, p[:], 0) {
			rect.W, rect.H = w, h
		}
		pace(clearFillPad)
		imageCallLeaves()
		return 0
	}
	p := [8]uint32{
		10<<24 | endOfList,
		0xe3000000,
		0xe4ffffff,
		0xe5000000,
		0xe6000000,
		0x60000000 | color,
		pair(rect.X, rect.Y),
		pair(w, h),
	}
	// Words 8 to 10 are E3h, E4h and E5h, filled in as it goes out.
	if enqueue(0, p[:], 8) {
		rect.W, rect.H = w, h
	}
	pace(clearImagePad)
	imageCallLeaves()
	return 0
}

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
	volatile.StoreUint32(&cdAudio.endTime, volatile.LoadUint32(&vsync.blanks))
	volatile.StoreUint32(&cdAudio.ended, 1)
}

// cdDataEndLine prints the line the reference prints at each data end:
// the index in the list of the track that ended and VSync(-1) when it did,
// in decimal.
func cdDataEndLine(index, time int32) {
	ttyPrintf("cbdataready: CdlDataEnd (track=%d,time=%d)\n", uint32(index), uint32(time))
}

func unbcd(b uint8) int32 { return int32(b>>4)*10 + int32(b&15) }
