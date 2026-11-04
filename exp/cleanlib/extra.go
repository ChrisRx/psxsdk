//go:build yaroze

package cleanlib

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The matrix, image and CD audio functions the sound examples use besides
// sound.

// RotMatrixY turns m about the Y axis by r, 4096 to a turn: m becomes the
// turn times m.
//
//export RotMatrixY
func RotMatrixY(r int32, m *MATRIX) *MATRIX {
	s, c := sin(r), cos(r)
	for j := range 3 {
		a, b := int32(m.M[0][j]), int32(m.M[2][j])
		m.M[0][j] = int16((c*a + s*b) >> 12)
		m.M[2][j] = int16((-s*a + c*b) >> 12)
	}
	return m
}

// RotMatrixZ turns m about the Z axis by r, 4096 to a turn: m becomes the
// turn times m.
//
//export RotMatrixZ
func RotMatrixZ(r int32, m *MATRIX) *MATRIX {
	s, c := sin(r), cos(r)
	for j := range 3 {
		a, b := int32(m.M[0][j]), int32(m.M[1][j])
		m.M[0][j] = int16((c*a - s*b) >> 12)
		m.M[1][j] = int16((s*a + c*b) >> 12)
	}
	return m
}

// ApplyMatrix multiplies v0 by m's rotation into v1, which it returns.
// The GTE's rotation matrix becomes m's.
//
//export ApplyMatrix
func ApplyMatrix(m *MATRIX, v0 *SVECTOR, v1 *VECTOR) *VECTOR {
	SetRotMatrix(m)
	x, y, z := int32(v0.Vx), int32(v0.Vy), int32(v0.Vz)
	v1.Vx = (int32(m.M[0][0])*x + int32(m.M[0][1])*y + int32(m.M[0][2])*z) >> 12
	v1.Vy = (int32(m.M[1][0])*x + int32(m.M[1][1])*y + int32(m.M[1][2])*z) >> 12
	v1.Vz = (int32(m.M[2][0])*x + int32(m.M[2][1])*y + int32(m.M[2][2])*z) >> 12
	return v1
}

// GsScaleScreen scales the screen against the world: each row of
// GsWSMATRIX, and the translation's element, by scale's element for that
// axis, 4096 being 1.
//
//export GsScaleScreen
func GsScaleScreen(scale *SVECTOR) {
	s := [3]int32{int32(scale.Vx), int32(scale.Vy), int32(scale.Vz)}
	for i := range 3 {
		for j := range 3 {
			gsWSMATRIX.M[i][j] = int16(int32(gsWSMATRIX.M[i][j]) * s[i] >> 12)
		}
		gsWSMATRIX.T[i] = gsWSMATRIX.T[i] * s[i] >> 12
	}
}

// gteMIMefunc moves n vertices at otp by the differences at dfp scaled by
// p, 4096 being 1: each coordinate becomes o + (d·p >> 12), saturated to
// 16 bits. The GTE does the arithmetic, with GPL: MAC holds the vertex,
// IR the difference and IR0 the scale.
//
//export gteMIMefunc
func gteMIMefunc(otp, dfp *SVECTOR, n, p int32) {
	ov := unsafe.Slice(otp, n)
	dv := unsafe.Slice(dfp, n)
	device.AsmFull(`.set push
	.set noat
	mtc2 {p}, $$8
	.set pop`, map[string]interface{}{"p": p})
	for i := range ov {
		o, d := &ov[i], &dv[i]
		device.AsmFull(`.set push
	.set noat
	mtc2 {a}, $$25
	mtc2 {b}, $$26
	mtc2 {c}, $$27
	mtc2 {x}, $$9
	mtc2 {y}, $$10
	mtc2 {z}, $$11
	nop
	nop
	.word 0x4a08003e
	.set pop`, map[string]interface{}{
			"a": int32(o.Vx), "b": int32(o.Vy), "c": int32(o.Vz),
			"x": int32(d.Vx), "y": int32(d.Vy), "z": int32(d.Vz),
		})
		o.Vx = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$9
	nop
	.set pop`, nil))
		o.Vy = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$10
	nop
	.set pop`, nil))
		o.Vz = int16(device.AsmFull(`.set push
	.set noat
	mfc2 {}, $$11
	nop
	.set pop`, nil))
	}
}

// The GTE's rotation and translation, control registers 0 to 7, saved by
// PushMatrix.
var matrixStack struct {
	saved [20][8]uint32
	n     int
}

// PushMatrix saves the GTE's rotation and translation on a stack of
// twenty.
//
//export PushMatrix
func PushMatrix() {
	if matrixStack.n >= len(matrixStack.saved) {
		return
	}
	s := &matrixStack.saved[matrixStack.n]
	matrixStack.n++
	s[0], s[1], s[2], s[3] = gteCtrl0(), gteCtrl1(), gteCtrl2(), gteCtrl3()
	s[4], s[5], s[6], s[7] = gteCtrl4(), gteCtrl5(), gteCtrl6(), gteCtrl7()
}

// PopMatrix restores the GTE's rotation and translation from the stack.
//
//export PopMatrix
func PopMatrix() {
	if matrixStack.n == 0 {
		return
	}
	matrixStack.n--
	s := &matrixStack.saved[matrixStack.n]
	var m MATRIX
	w := (*[5]uint32)(unsafe.Pointer(&m.M))
	copy(w[:], s[:5])
	m.T = [3]int32{int32(s[5]), int32(s[6]), int32(s[7])}
	SetRotMatrix(&m)
	SetTransMatrix(&m)
}

// The control register reads, each a cfc2 into $gp and a move out of it.

func gteCtrl0() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c0000\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl1() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c0800\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl2() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c1000\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl3() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c1800\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl4() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c2000\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl5() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c2800\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl6() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c3000\nnop\nmove {}, $$28\n.set pop", nil))
}
func gteCtrl7() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c3800\nnop\nmove {}, $$28\n.set pop", nil))
}

// StoreImage copies the rectangle rect of video memory to p. As loadImage
// does, the CPU reads the words that do not make a whole block of 16, from
// the start of the image, and DMA the blocks (GP1 04000003h, as Sony's
// library sends for an image of whole blocks and not for one of 8 words,
// spec 13's probes and spec 08's); the call does not wait for the DMA:
// the next call that uses the GPU sees the channel busy, and DrawSync waits.
//
//export StoreImage
//go:noinline
func StoreImage(rect *RECT, p *uint32) int32 {
	m := imageEnter()
	// The DMA direction off first, as the reference does.
	gp1.Set(gp1DMAOff)
	gp0.Set(0x01000000)
	gp0.Set(0xc0000000)
	gp0.Set(pair(rect.X, rect.Y))
	gp0.Set(pair(rect.W, rect.H))
	// GPUSTAT read until the GPU has the pixels to send.
	for gp1.Get()&gpuStatSend == 0 {
	}
	words := (int(rect.W)*int(rect.H) + 1) / 2
	const block = 16
	head := words % block
	dst := unsafe.Slice(p, head)
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
	return 0
}

// ClearImage fills the rectangle rect of video memory with r, g, b, queued
// with the drawing. As the reference does it, a rectangle whose x and
// width are multiples of 64, the width below 1024, takes the GPU's fill
// (02h). Any other is drawn as a flat rectangle (60h) with the drawing
// area opened to all of video memory and its offset at 0, and the area
// and offset the GPU reports as the command goes out put back after it.
//
//export ClearImage
//go:noinline
func ClearImage(rect *RECT, r, g, b uint8) int32 {
	color := uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	if rect.X%64 == 0 && rect.W%64 == 0 && rect.W < 1024 {
		// The mask bits off (E6h), then the fill.
		p := [5]uint32{
			4<<24 | endOfList,
			0xe6000000,
			0x02000000 | color,
			pair(rect.X, rect.Y),
			pair(rect.W, min(rect.H, 511)),
		}
		submit(0, p[:])
		pace(clearFillPad)
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
		pair(min(rect.W, 1023), min(rect.H, 511)),
	}
	// Words 8 to 10 are E3h, E4h and E5h, filled in as it goes out.
	enqueue(0, p[:], 8)
	pace(clearImagePad)
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
	repeat  bool
	playing bool
	starts  [100][2]uint8
	ended   uint32
	endTime uint32
}

// CdPlay plays the tracks at tracks, a list ending in 0, from offset on:
// once with mode 1, round again with mode 2. Mode 0 stops and mode 3 keeps
// the list going, as a program calls it every frame. It returns the index
// of the track playing, or -1.
//
// As the reference does it: modes 1 and 2 read the disc's table of
// contents (GetTN, then GetTD for every track from 0), set CD audio with
// autopause, and play the track from its start (Setloc, Play). The drive
// pauses at the end of each track. Mode 3 moves on to the next track once
// the drive has reported that end, playing it and then asking the drive's
// status; while the drive does not say it is playing it plays the track
// again; otherwise it only asks the status (Getstat).
//
//export CdPlay
func CdPlay(mode int32, tracks *int32, offset int32) int32 {
	switch mode {
	case 0:
		if cdAudio.playing {
			cd.controlEv(cdlPause, nil, nil)
			cdAudio.playing = false
		}
	case 1, 2:
		var r [8]uint8
		cd.control(cdlGetTN, nil, &r[0])
		last := unbcd(r[2])
		for t := range last + 1 {
			n := bcd(t)
			cd.control(cdlGetTD, &n, &r[0])
			cdAudio.starts[t] = [2]uint8{r[1], r[2]}
		}
		m := uint8(cdlModeCDDA | cdlModeAutoPause)
		cd.controlEv(cdlSetmode, &m, nil)
		cdAudio.tracks, cdAudio.index, cdAudio.repeat = tracks, offset, mode == 2
		volatile.StoreUint32(&cdAudio.ended, 0)
		cdAudio.playing = cdPlayTrack()
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
			cd.controlEv(cdlGetstat, nil, nil)
		case cd.resp[0]&cdStatPlay == 0:
			cdAudio.playing = cdPlayTrack()
		default:
			cd.controlEv(cdlGetstat, nil, nil)
		}
	}
	if !cdAudio.playing {
		return -1
	}
	return cdAudio.index
}

// cdTrack returns the track number at index i of the list.
func cdTrack(i int32) int32 {
	return *(*int32)(unsafe.Add(unsafe.Pointer(cdAudio.tracks), 4*i))
}

// cdPlayTrack plays the track at cdAudio.index from its start, and
// reports whether there is one.
func cdPlayTrack() bool {
	t := cdTrack(cdAudio.index)
	if t <= 0 || t >= int32(len(cdAudio.starts)) {
		return false
	}
	loc := [4]uint8{cdAudio.starts[t][0], cdAudio.starts[t][1], 0, 0}
	cd.control(cdlSetloc, &loc[0], nil)
	cd.controlEv(cdlPlay, nil, nil)
	return true
}

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
