// The todview sample, in Go.
//
// A port of Sony's todview (1994-1997), which plays an animation from a TOD
// file: a hand of fifteen jointed parts from one TMD playing rock, paper,
// scissors. A TOD is a script of frames, each a list of packets that create
// objects, link them to models in the TMD, hang them off one another, and
// move, turn and scale them, a coordinate system each, so that turning a
// knuckle carries the finger past it along. The library draws each part
// through the chain of coordinate systems above it, which neither the check
// nor the chrome example has.
//
// The pad moves the viewpoint: the directional buttons along X and Y, L1
// and L2 along Z, and Start puts it back. A right-hand button starts the
// animation again. Select quits.
//
// Differences from the original:
//
//   - The animation plays over and over by itself, a second apart; the
//     original plays it once each time a right-hand button is pressed, and
//     otherwise shows nothing new. The object table is cleared before each
//     play, where the original goes on creating objects into it.
//   - Each frame waits for the GPU before swapping buffers. The original
//     resets the GPU instead, after the vertical blank, which cuts short
//     whatever it has not drawn yet.
//   - A packet for an object that is not in the table goes to a dummy
//     object with a coordinate system of its own; the original's dummy has
//     an uninitialised pointer for one. A parent that is not in the table
//     is ignored rather than followed through a nil pointer, and a light
//     beyond the three there are is taken modulo three rather than written
//     past the end of the array.
//   - The model and the animation are read off the disc to the addresses
//     the original's batch script sent them to.
package main

import (
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	otLength  = 12   // ordering table size, as a power of two
	objMax    = 50   // objects the table holds
	packetMax = 2400 // GPU packets a frame

	modelAddr = 0x800c0000
	todAddr   = 0x800e0000

	// The viewpoint and the point it looks at.
	vpx, vpy, vpz = -8000, -2000, -5000
	vrx, vry, vrz = -900, -1500, 0

	// Frames to hold the last pose for before playing again.
	pause = 60
)

var files = []demo.File{
	{Name: "\\DATA\\TE.TMD;1", Addr: modelAddr},
	{Name: "\\DATA\\JAANKEN.TOD;1", Addr: todAddr},
}

// teList is te.h: the TOD's ID for each object in te.tmd, in order.
var teList = []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

var (
	ot        [2]yaroze.GsOT
	otTags    [2][1 << otLength]yaroze.GsOT_TAG
	gpuPacket [2][packetMax * 24]yaroze.PACKET

	objArea   [objMax]yaroze.GsDOBJ2
	objCoord  [objMax]yaroze.GsCOORDINATE2
	objParams [objMax]yaroze.GsCOORD2PARAM
	objs      objTable

	view   yaroze.GsRVIEW2
	lights [3]yaroze.GsF_LIGHT

	tmd *uint32 // the TMD past its ID word, as GsMapModelingData takes it
)

// The animation: its first frame, how many frames it has, and its first
// frame number.
var (
	todTop     *uint32
	numFrames  int
	startFrame int
)

func initSystem() {
	demo.PadInit()
	yaroze.GsInitGraph(640, 240, 0, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	for i := range ot {
		ot[i].Length = otLength
		ot[i].Org = &otTags[i][0]
		ot[i].Offset = 0
	}
	yaroze.GsInit3D()
}

func initView() {
	yaroze.GsSetProjection(1000)
	view = yaroze.GsRVIEW2{Vpx: vpx, Vpy: vpy, Vpz: vpz, Vrx: vrx, Vry: vry, Vrz: vrz, Super: nil}
	yaroze.GsSetRefView2(&view)
}

func initLight() {
	// Light 0 shines along (100, 100, 100); the other two are dark.
	lights[0] = yaroze.GsF_LIGHT{Vx: 100, Vy: 100, Vz: 100, R: 0xd0, G: 0xd0, B: 0xd0}
	lights[1] = yaroze.GsF_LIGHT{Vx: 100, Vy: 100, Vz: 100}
	lights[2] = yaroze.GsF_LIGHT{Vx: 100, Vy: 100, Vz: 100}
	for i := range lights {
		yaroze.GsSetFlatLight(int32(i), &lights[i])
	}
	yaroze.GsSetAmbient(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
	yaroze.GsSetLightMode(0)
}

func initModelingData() {
	tmd = demo.At(modelAddr + 4) // past the file's ID word
	yaroze.GsMapModelingData(tmd)
}

func initTod() {
	p := demo.At(todAddr + 4)
	numFrames = int(*p)
	todTop = word(p, 1)
	startFrame = int(*word(todTop, 1))
}

func drawObjects() {
	buff := yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(&gpuPacket[buff][0])
	yaroze.GsClearOt(0, 0, &ot[buff])

	for i := range objs.nobj {
		o := objs.obj(i)
		o.Coord2.Flg = 0
		if o.Id == todUndef || o.Tmd == nil {
			continue
		}
		var ls yaroze.MATRIX
		yaroze.GsGetLs(o.Coord2, &ls)
		yaroze.GsSetLsMatrix(&ls)
		yaroze.GsSetLightMatrix(&ls)
		yaroze.GsSortObject4(o, &ot[buff], 14-otLength, demo.Scratch())
	}

	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(0, 0, 0, &ot[buff])
	yaroze.GsDrawOt(&ot[buff])
}

// A player plays the animation a frame at a time: the frame number now,
// the next of its frames to play, and how many it has played.
type player struct {
	now    int
	next   *uint32
	played int
	held   int
}

func (p *player) start() {
	objs.init(objArea[:], objCoord[:], objParams[:])
	p.now = startFrame
	p.next = setFrame(p.now, todTop, &objs, teList, tmd)
	p.played = 1
	p.held = 0
}

// step moves the animation on a frame, playing the TOD's frame if one is
// due, and starting again once it has rested on the last for a while.
func (p *player) step() {
	if p.played == numFrames {
		if p.held++; p.held == pause {
			p.start()
		}
		return
	}
	p.now++
	if int(*word(p.next, 1)) <= p.now {
		p.next = setFrame(p.now, p.next, &objs, teList, tmd)
		p.played++
	}
}

// moves are the buttons that move the viewpoint, and how.
var moves = [...]struct {
	button uint32
	coord  *int32
	by     int32
}{
	{demo.PADL1, &view.Vpz, 100},
	{demo.PADL2, &view.Vpz, -100},
	{demo.PADLleft, &view.Vpx, -100},
	{demo.PADLright, &view.Vpx, 100},
	{demo.PADLdown, &view.Vpy, 100},
	{demo.PADLup, &view.Vpy, -100},
}

var (
	lineBuf [64]byte
	line    yaroze.Buffer

	// frameNumber counts the frames drawn, which a test reads.
	frameNumber int32
)

func main() {
	line.Init(lineBuf[:])
	initSystem()
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(32, 32, 576, 200, 0, 256)
	if missing := demo.Load(files, func(string) {
		line.Reset()
		line.WriteString("loading\n").Print()
		drawObjects()
		yaroze.FntFlush(-1)
	}); missing != "" {
		for demo.PadRead()&demo.PADselect == 0 {
			line.Reset()
			line.WriteString("could not read\n").WriteString(missing).Print()
			drawObjects()
			yaroze.FntFlush(-1)
		}
		yaroze.ResetGraph(3)
		return
	}

	initView()
	initLight()
	initModelingData()
	initTod()

	var anim player
	anim.start()
	drawObjects()

	for {
		pad := demo.PadRead()
		if pad&demo.PADselect != 0 {
			break
		}
		if pad&(demo.PADRright|demo.PADRleft|demo.PADRup|demo.PADRdown) != 0 {
			anim.start()
		}

		for _, m := range moves {
			if pad&m.button != 0 {
				*m.coord += m.by
				yaroze.GsSetRefView2(&view)
			}
		}
		if pad&demo.PADstart != 0 {
			initView()
		}

		anim.step()
		drawObjects()
		frameNumber++
	}
	yaroze.ResetGraph(3)
}
