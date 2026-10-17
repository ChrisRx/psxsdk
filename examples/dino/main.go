// The dino demo, in Go.
//
// A port of Sony's Net Yaroze demo "dino" (1996): a dinosaur walking in
// front of a castle, at 640 by 240. The dinosaur is one model of 2,724
// polygons, animated by the GTE's vertex interpolation, MIME: the model
// file comes with a file of vertex differences, nine sets of them, each
// moving part of the body, a leg, the neck, the eyelids, and every frame
// the model's vertices are put back as they were and each set is added in
// some proportion. The proportions come from the controls through a bank
// of transfer functions, sixteen filters that each smooth their input over
// the last few frames with a window of weights, so that a button pressed
// becomes a step taken. The walk itself is four of them cycling through a
// table. The castle is a full-screen 16-bit picture copied into the
// drawing buffer every frame before the model is drawn over it, in fog,
// under three lights. Music plays from a sequence, with reverb.
//
// The dinosaur walks by itself. Square and Circle turn it, Triangle and
// Cross tip it, the d-pad moves it across and up and down, and Start and
// Select bring it nearer and further. R2 opens its mouth, L2 blinks, R1
// turns its head and L1 lifts its tail. Holding L1 and R1 with Select
// stops the walk and puts the legs on the buttons instead: R1, R2, L1 and
// L2 lift each in turn, with Down the feet and with Up the knees. L1 and R1
// with Start starts the walk again, and then Up and Down choose between
// the buttons above and R1 and R2 changing the walk's speed, with L1
// putting it back. Start and Select together quit.
//
// Differences from the original:
//
//   - The data is read off the disc rather than sent over the serial
//     cable, and where it lands is rearranged: the sound comes first, to
//     the addresses the check sample loads it at, and once the sound bank
//     has gone to the SPU the model and its textures are loaded over it.
//     The castle picture is loaded into the program's own memory, since
//     with everything else there is no room for it below the program.
//   - The GPU packet space is the program's own, and larger. The original
//     puts it where the sound bank was, with its two buffers closer
//     together than a frame of the model can need.
//   - The walk's speed, when it is turned down past zero, wraps round the
//     step tables rather than reading before them.
//
// Nothing allocates after startup.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	// Where the data is loaded. The sound bank's body is only needed
	// until it has been sent to the SPU, and the textures are loaded over
	// it afterwards.
	seqAddr    = 0x80090000
	vhAddr     = 0x8009a000
	vbAddr     = 0x800a9000
	dinonewTIM = vbAddr
	md0TMD     = 0x8010a000
	dinoVDF    = 0x80127000

	// castleSectors is how many sectors the castle picture takes, which is
	// how much of the program's memory the read of it fills.
	castleSectors = (307220 + 2047) / 2048

	objectMax = 100
	otLength  = 10
	packetMax = 5000 * 24

	mvol = 127
	svol = 127
)

var soundFiles = []demo.File{
	{File: disc.Register("DATA/SOUND/SAMPLE1.SEQ", "data/SAMPLE1.SEQ"), Addr: seqAddr},
	{File: disc.Register("DATA/SOUND/STD0.VH", "data/STD0.VH"), Addr: vhAddr},
	{File: disc.Register("DATA/SOUND/STD0.VB", "data/STD0.VB"), Addr: vbAddr},
}

// The model and its textures, loaded once the sound has been sent to the
// SPU. The castle's address is the program's own memory, filled in at run
// time.
var dataFiles = []demo.File{
	{File: disc.Register("DATA/DINONEW.TIM", "data/DINONEW.TIM"), Addr: dinonewTIM},
	{File: disc.Register("DATA/MD0.TMD", "data/MD0.TMD"), Addr: md0TMD},
	{File: disc.Register("DATA/DINO.VDF", "data/DINO.VDF"), Addr: dinoVDF},
	{File: disc.Register("DATA/CASTLE.TIM", "data/CASTLE.TIM")},
}

const (
	dinonewFile = iota
	md0File
	vdfFile
	castleFile
)

var castle [castleSectors * 512]uint32

var (
	texInfo yaroze.GsIMAGE // the castle, in memory
	bgPos   yaroze.RECT    // where it is copied each frame

	wot        [2]yaroze.GsOT
	zsorttable [2][1 << otLength]yaroze.GsOT_TAG
	packetArea [2][packetMax / 4]uint32

	object [objectMax]yaroze.GsDOBJ2
	objnum uint32

	fog1   yaroze.GsFOGPARAM
	dWorld yaroze.GsCOORDINATE2 // the dinosaur's coordinate system
	pWorld yaroze.SVECTOR       // and how it is turned

	view yaroze.GsRVIEW2
	pslt [3]yaroze.GsF_LIGHT

	// alpha is the speed of the walk; modeflag whether the dinosaur
	// walks by itself, and modeflagex which set of movements the buttons
	// control.
	alpha      int32
	modeflag   int32
	modeflagex int32

	frameNumber uint32
	padd        uint32
)

// The walk: which of the four leg movements is on at each of the sixteen
// steps of the cycle.
var (
	lf = [16]int32{1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rf = [16]int32{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0}
	ln = [16]int32{0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rn = [16]int32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0}
)

// blinktable is how far the eyelids are down at each frame of a two
// second cycle.
var blinktable = [120]int32{
	0, 1024, 2048, 3072, 3072, 3072, 3072, 2048, 2048, 2048,
	1024, 1024, 1024, 768, 768, 512, 256, 128, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 2048, 4096, 4096, 4096, 4096, 4096,
	3072, 2048, 1024, 0, 0, 0, 2048, 4096, 2048, 0,
	0, 0, 0, 0, 512, 4096, 3072, 2048, 1024, 512,
	0, 0, 0, 512, 4096, 3072, 2048, 1024, 512, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 512, 1024, 2048, 2048, 2048, 2048, 2048,
	1024, 768, 512, 256, 0, 0, 0, 0, 0, 0,
}

func main() {
	demo.PadInit()
	initAll()

	if missing := demo.Load(soundFiles, nil); missing != "" {
		fail("could not read ", missing)
		return
	}
	sndSet(demo.At(seqAddr), demo.BytesAt(vhAddr), demo.BytesAt(vbAddr))

	dataFiles[castleFile].Addr = uintptr(unsafe.Pointer(&castle[0]))
	if missing := demo.Load(dataFiles, nil); missing != "" {
		fail("could not read ", missing)
		return
	}

	modeflag = 1
	modeflagex = 0
	alpha = 32

	initTexture1()
	initTexture2()
	initAll2()

	// The vertex interpolation.
	setTMDData()
	setVertexMimeData()
	for i := range mimepr {
		mimepr[i] = 0
	}

	// The transfer functions: the first four, the legs, with a sixteen
	// frame window; most of the rest with the longer one; and the mouth
	// with the one that overshoots.
	initCntrlarry(cnv0[:])
	for i := 4; i <= 12; i++ {
		ctlfc[i].cnv = cnv3[:]
	}
	ctlfc[9].cnv = cnv2[:]

	sndPlay()

	yaroze.SetVideoMode(yaroze.MODE_PAL)
	yaroze.GsDISPENV().Screen.Y += 27

	for frameNumber = 0; ; frameNumber++ {
		if objInteractive() < 0 {
			break
		}

		setCntrl(frameNumber)
		pWorld.Vy += int16(ctlfc[4].out)
		pWorld.Vx += int16(ctlfc[5].out)
		dWorld.Coord.T[2] += ctlfc[8].out
		dWorld.Coord.T[0] += ctlfc[6].out
		dWorld.Coord.T[1] += ctlfc[7].out

		if modeflag != 0 {
			// The walk: a step of the cycle every so many frames, by
			// alpha.
			ph2 := int32(frameNumber & 0xffff)
			ph1 := ((ph2 * alpha) >> 6) % 16
			if ph1 < 0 {
				ph1 += 16
			}
			ctlfc[0].in = lf[ph1] * 4096
			ctlfc[1].in = rf[ph1] * 4096
			ctlfc[2].in = ln[ph1] * 6000
			ctlfc[3].in = rn[ph1] * 6000
		}
		mimepr[0] = ctlfc[0].out
		mimepr[1] = ctlfc[1].out
		mimepr[2] = ctlfc[2].out
		mimepr[3] = ctlfc[3].out
		mimepr[4] = ctlfc[9].out
		mimepr[5] = ctlfc[10].out
		mimepr[6] = ctlfc[11].out
		mimepr[7] = pick(frameNumber)
		mimepr[8] = ctlfc[12].out
		vertexMime()

		yaroze.GsSetRefView2(&view)

		outbufIdx := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[outbufIdx][0]))
		yaroze.GsClearOt(0, 0, &wot[outbufIdx])

		for i := range objnum {
			op := &object[i]
			var tmpls yaroze.MATRIX
			yaroze.GsGetLs(op.Coord2, &tmpls)
			yaroze.GsSetLightMatrix(&tmpls)
			yaroze.GsSetLsMatrix(&tmpls)
			yaroze.GsSortObject4(op, &wot[outbufIdx], 14-otLength, demo.Scratch())
		}

		padd = demo.PadRead()
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()

		// The castle, straight into the buffer about to be drawn.
		bgPos.Y = int16((1 - outbufIdx) * 240)
		yaroze.LoadImage(&bgPos, texInfo.Pixel)
		yaroze.DrawSync(0)

		yaroze.GsDrawOt(&wot[outbufIdx])
	}

	sndStop()
}

// pick returns how far the eyelids are down this frame.
func pick(x uint32) int32 {
	return blinktable[x%120]
}

// pressed reports whether any of the buttons in bits are down.
func pressed(bits uint32) bool { return padd&bits != 0 }

// choose returns by if up is pressed, -by if down is, and 0 otherwise.
func choose(up, down uint32, by int32) int32 {
	switch {
	case pressed(up):
		return by
	case pressed(down):
		return -by
	}
	return 0
}

// onOff returns on while bits are pressed, and 0 otherwise.
func onOff(bits uint32, on int32) int32 {
	if pressed(bits) {
		return on
	}
	return 0
}

// objInteractive turns the pad into the transfer functions' inputs, and
// returns -1 when the program is to quit.
func objInteractive() int32 {
	ctlfc[4].in = choose(demo.PADRleft, demo.PADRright, 6*yaroze.ONE/360)
	ctlfc[5].in = choose(demo.PADRup, demo.PADRdown, 6*yaroze.ONE/360)
	ctlfc[8].in = choose(demo.PADselect, demo.PADstart, 300)
	ctlfc[6].in = choose(demo.PADLleft, demo.PADLright, 200)
	ctlfc[7].in = choose(demo.PADLdown, demo.PADLup, 200)

	// L1 and R1 with another button switch modes.
	if pressed(demo.PADR1) && pressed(demo.PADL1) {
		switch {
		case pressed(demo.PADselect):
			modeflag = 0
			ctlfc[0].in, ctlfc[1].in, ctlfc[2].in, ctlfc[3].in = 0, 0, 0, 0
		case pressed(demo.PADstart):
			modeflag = 1
			ctlfc[9].in, ctlfc[10].in, ctlfc[11].in, ctlfc[12].in = 0, 0, 0, 0
		case pressed(demo.PADLup):
			modeflagex = 0
			ctlfc[0].in, ctlfc[1].in, ctlfc[2].in, ctlfc[3].in = 0, 0, 0, 0
		case pressed(demo.PADLdown):
			modeflagex = 1
			ctlfc[9].in, ctlfc[10].in, ctlfc[11].in, ctlfc[12].in = 0, 0, 0, 0
		}
	}

	switch {
	case modeflag == 0 && modeflagex == 1: // the feet on the buttons
		ctlfc[0].in = onOff(demo.PADR1, yaroze.ONE)
		ctlfc[1].in = onOff(demo.PADR2, yaroze.ONE)
		ctlfc[2].in = onOff(demo.PADL1, yaroze.ONE)
		ctlfc[3].in = onOff(demo.PADL2, yaroze.ONE)
	case modeflag == 0: // the knees, and the mouth, the eyes and the head
		ctlfc[9].in = onOff(demo.PADR2, yaroze.ONE)
		ctlfc[10].in = onOff(demo.PADL2, yaroze.ONE)
		ctlfc[11].in = onOff(demo.PADR1, 6000)
		if pressed(demo.PADL1) {
			ctlfc[0].in = yaroze.ONE
		} else {
			ctlfc[0].in = 0
			ctlfc[12].in = 0
		}
	case modeflagex == 1: // walking: the speed of the walk
		if pressed(demo.PADR1) {
			alpha++
		}
		if pressed(demo.PADR2) {
			alpha--
		}
		if pressed(demo.PADL1) {
			alpha = 32
		}
	default: // walking: the mouth, the eyes, the head and the tail
		ctlfc[9].in = onOff(demo.PADR2, 6000)
		ctlfc[10].in = onOff(demo.PADL2, yaroze.ONE)
		ctlfc[11].in = onOff(demo.PADR1, 6000)
		ctlfc[12].in = onOff(demo.PADL1, 6000)
	}

	if pressed(demo.PADstart) && pressed(demo.PADselect) {
		return -1
	}

	setCoordinate(&pWorld, &dWorld)
	return 0
}

func initAll() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(640, 240, 0, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	padd = 0

	for i := range 2 {
		wot[i].Length = otLength
		wot[i].Org = &zsorttable[i][0]
	}
}

func initAll2() {
	coordInit()
	modelInit()
	viewInit()
	lightInit()
}

func viewInit() {
	yaroze.GsSetProjection(1000)
	view = yaroze.GsRVIEW2{
		Vpx: 0, Vpy: 0, Vpz: 2000,
		Vrx: 0, Vry: 0, Vrz: 0,
		Rz: 0, Super: demo.WORLD,
	}
	yaroze.GsSetRefView2(&view)
}

func lightInit() {
	pslt[0] = yaroze.GsF_LIGHT{Vx: 20, Vy: -100, Vz: 100, R: 0xf0, G: 0xf0, B: 0xf0}
	yaroze.GsSetFlatLight(0, &pslt[0])
	pslt[1] = yaroze.GsF_LIGHT{Vx: 20, Vy: -50, Vz: -100, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(1, &pslt[1])
	pslt[2] = yaroze.GsF_LIGHT{Vx: -20, Vy: 20, Vz: 100, R: 0x80, G: 0x80, B: 0x80}
	yaroze.GsSetFlatLight(2, &pslt[2])

	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	yaroze.GsSetLightMode(1)

	fog1 = yaroze.GsFOGPARAM{Dqa: -5 * 12000 * 64 / 600, Dqb: 5 / 4 * yaroze.ONE * yaroze.ONE}
	yaroze.GsSetFogParam(&fog1)
}

func coordInit() {
	yaroze.GsInitCoordinate2(demo.WORLD, &dWorld)
	pWorld = yaroze.SVECTOR{}
	dWorld.Coord.T[2] = -30000
}

// setCoordinate turns a coordinate system by pos, keeping where it is.
func setCoordinate(pos *yaroze.SVECTOR, coor *yaroze.GsCOORDINATE2) {
	tmp1 := demo.IDMATRIX
	tmp1.T = coor.Coord.T
	v1 := *pos
	yaroze.RotMatrix(&v1, &tmp1)
	coor.Coord = tmp1
	coor.Flg = 0
}

// timSize is how many bytes a TIM takes in memory, from its description.
func timSize(im *yaroze.GsIMAGE) uintptr {
	n := uintptr(8) + 12 + uintptr(im.Pw)*uintptr(im.Ph)*2
	if im.Pmode&8 != 0 {
		n += 12 + uintptr(im.Cw)*uintptr(im.Ch)*2
	}
	return n
}

// initTexture1 loads the dinosaur's textures into video memory: a file of
// TIMs one after another, each with its palette.
func initTexture1() {
	addr := uintptr(dinonewTIM)
	end := addr + uintptr(dataFiles[dinonewFile].Size())
	for addr < end && *demo.At(addr) == 0x10 {
		im := demo.LoadTIM(addr)
		addr += timSize(&im)
	}
}

// initTexture2 puts the castle up once, so that the first frame has it,
// and keeps its description for copying it every frame after.
func initTexture2() {
	yaroze.GsGetTimInfo(demo.At(uintptr(unsafe.Pointer(&castle[0]))+4), &texInfo)
	rect := yaroze.RECT{X: texInfo.Px, Y: texInfo.Py, W: int16(texInfo.Pw), H: int16(texInfo.Ph)}
	yaroze.LoadImage(&rect, texInfo.Pixel)
	if texInfo.Pmode&8 != 0 {
		rect = yaroze.RECT{X: texInfo.Cx, Y: texInfo.Cy, W: int16(texInfo.Cw), H: int16(texInfo.Ch)}
		yaroze.LoadImage(&rect, texInfo.Clut)
	}
	bgPos = yaroze.RECT{X: 0, Y: 0, W: int16(texInfo.Pw), H: 240}
}

// modelInit maps the model's offsets to addresses and links a handle to
// each of its objects, all in the dinosaur's coordinate system.
func modelInit() {
	yaroze.GsMapModelingData(demo.At(md0TMD + 4))
	objnum = min(*demo.At(md0TMD + 8), objectMax)
	for i := range objnum {
		yaroze.GsLinkObject4(uint32(md0TMD+12), &object[i], int32(i))
		object[i].Coord2 = &dWorld
		object[i].Attribute = 0
	}
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	var buf [64]byte
	var text yaroze.Buffer
	text.Init(buf[:])
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 320, 32, 0, 128)
	for demo.PadRead()&(demo.PADstart|demo.PADselect) != demo.PADstart|demo.PADselect {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[buff][0]))
		yaroze.GsClearOt(0, 0, &wot[buff])
		yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &wot[buff])
		yaroze.GsDrawOt(&wot[buff])
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

// The music: the demo's snd.c.

var vab, seq int16

// sndSet sends the sound bank to the SPU and opens the sequence, with
// the volume down until it plays.
func sndSet(seqAddr *uint32, vh, vb *uint8) {
	yaroze.SsSetMVol(0, 0)
	yaroze.SsUtSetReverbType(yaroze.SS_REV_TYPE_STUDIO_C)
	vab = yaroze.SsVabTransfer(vh, vb, -1, 1)
	if vab < 0 {
		return
	}
	seq = yaroze.SsSeqOpen(seqAddr, vab)
}

func sndPlay() {
	if vab < 0 || seq < 0 {
		return
	}
	yaroze.SsSetMVol(mvol, mvol)
	yaroze.SsSeqSetVol(seq, svol, svol)
	yaroze.SsUtReverbOn()
	yaroze.SsUtSetReverbDepth(80, 80)
	yaroze.SsSeqPlay(seq, yaroze.SSPLAY_PLAY, yaroze.SSPLAY_INFINITY)
}

func sndStop() {
	if vab < 0 {
		return
	}
	if seq >= 0 {
		yaroze.SsSeqStop(seq)
		yaroze.VSync(0)
		yaroze.VSync(0)
		yaroze.SsSeqClose(seq)
	}
	yaroze.SsVabClose(vab)
}
