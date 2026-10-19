// A textured car driving in from the distance, at sixty frames a second or
// thirty.
//
// A port of Sony's Net Yaroze demo "car" (Sony, 1995, "60frame" in its
// source): an Alfa Romeo Giulietta, a textured model wearing seven
// textures, drives in from the distance and then turns slowly at the
// center of the screen under one light, at 640 by 240 interlaced. The
// demo is about frame rate: Start switches it between sixty frames a
// second and thirty, which the library's font reports, and the controls
// double their steps at thirty so that the car moves at the same speed
// either way.
//
// Differences from the original:
//
//   - The textures and model are read off the disc into the program's own
//     memory. The original's batch script sent them to fixed addresses.
//   - The drive in moves the car by whole units each frame, as the
//     original's integer coordinate ends up doing: it adds 1 + i/20 in
//     floating point and C truncates the sum towards zero, which rounds
//     the step up while the car is behind the viewpoint. The port
//     reproduces that arithmetic.
//   - The speed line is printed in a font window the port opens. The
//     original prints without opening one.
//
// Nothing allocates after startup.
//
// # Controls
//
// The d-pad moves the car across and up and down, the symbol buttons turn
// it, and L1 and L2 bring it nearer and push it further. Start switches
// between sixty frames a second and thirty. Select quits.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	otLength  = 12
	otSize    = 1 << otLength
	scrZ      = 1000 // screen to viewpoint distance
	objectMax = 100
	packetMax = 10000 * 24 // GPU packet space per buffer

	frame30 = 0
	frame60 = 1
)

// The data, read into the program's own memory, in buffers of whole
// sectors: the seven textures and the model.
var (
	tex1Data  [(16416 + 2047) / 2048 * 512]uint32
	tex2Data  [(14584 + 2047) / 2048 * 512]uint32
	tex3Data  [(30784 + 2047) / 2048 * 512]uint32
	tex4Data  [(7584 + 2047) / 2048 * 512]uint32
	tex5Data  [(16928 + 2047) / 2048 * 512]uint32
	tex6Data  [(11128 + 2047) / 2048 * 512]uint32
	tex7Data  [(19450 + 2047) / 2048 * 512]uint32
	modelData [(52000 + 2047) / 2048 * 512]uint32

	tex1File  = disc.Register("DATA/GIULI_BD.TIM", "data/GIULI_BD.TIM").Into(tex1Data[:])
	tex2File  = disc.Register("DATA/GIULI_FR.TIM", "data/GIULI_FR.TIM").Into(tex2Data[:])
	tex3File  = disc.Register("DATA/GIULI_PL.TIM", "data/GIULI_PL.TIM").Into(tex3Data[:])
	tex4File  = disc.Register("DATA/GIULI_TL.TIM", "data/GIULI_TL.TIM").Into(tex4Data[:])
	tex5File  = disc.Register("DATA/GIULI_TR.TIM", "data/GIULI_TR.TIM").Into(tex5Data[:])
	tex6File  = disc.Register("DATA/GIULI_WN.TIM", "data/GIULI_WN.TIM").Into(tex6Data[:])
	tex7File  = disc.Register("DATA/WHEEL256.TIM", "data/WHEEL256.TIM").Into(tex7Data[:])
	modelFile = disc.Register("DATA/GIULIETA.TMD", "data/GIULIETA.TMD").Into(modelData[:])
	files     = []*disc.File{&tex1File, &tex2File, &tex3File, &tex4File, &tex5File, &tex6File, &tex7File, &modelFile}

	textures = [...]yaroze.TIM{tex1Data[:], tex2Data[:], tex3Data[:], tex4Data[:], tex5Data[:], tex6Data[:], tex7Data[:]}
	model    = yaroze.TMD(modelData[:])
)

// A DB is one of the two drawing buffers: an ordering table and the GPU
// packets its primitives become.
type DB struct {
	gsot   yaroze.GsOT
	ot     [otSize]yaroze.GsOT_TAG
	packet [packetMax / 4]uint32
}

var (
	db       [2]DB
	objWorld yaroze.GsCOORDINATE2
	rotate   yaroze.SVECTOR
	object   [objectMax]yaroze.GsDOBJ2
	objcnt   int32
	frame    int32 = frame60
	paddOld  yaroze.Buttons

	// The controls' steps at thirty frames a second and at sixty.
	rotSpeed = [2]int16{4 * yaroze.ONE / 360, 2 * yaroze.ONE / 360}
	traSpeed = [2]int32{20, 10}

	frameNumber int32

	textBuf [64]byte
	text    yaroze.Buffer
)

func main() {
	text.Init(textBuf[:])
	initSystem()

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return
	}

	initCoordinate()
	initView()
	initLight()
	initModel()
	initTexture()

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-304, -112, 320, 16, 0, 64)

	opening()

	for padRead() == 0 {
		moveObject()
		idx := yaroze.GsGetActiveBuff()
		drawObject(&db[idx], frame)
	}
	yaroze.ResetGraph(3)
}

func initSystem() {
	yaroze.PadInit()

	yaroze.GsInitGraph(640, 240, yaroze.GsINTER|yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)

	for i := range db {
		db[i].gsot.Length = otLength
		db[i].gsot.Point = 0
		db[i].gsot.Offset = 0
		db[i].gsot.Org = &db[i].ot[0]
	}

	yaroze.GsInit3D()
}

func initCoordinate() {
	objWorld = yaroze.GsCOORDINATE2{}
	rotate = yaroze.SVECTOR{}
	yaroze.GsInitCoordinate2(yaroze.WORLD, &objWorld)
}

func initView() {
	yaroze.GsSetProjection(scrZ)
	view := yaroze.GsRVIEW2{
		Vpx: 0, Vpy: 0, Vpz: 1000,
		Vrx: 0, Vry: 0, Vrz: 0,
		Rz:    0,
		Super: yaroze.WORLD,
	}
	yaroze.GsSetRefView2(&view)
}

func initLight() {
	light := yaroze.GsF_LIGHT{Vx: 20, Vy: -100, Vz: -100, R: 0xd0, G: 0xd0, B: 0xd0}
	yaroze.GsSetFlatLight(0, &light)
	yaroze.GsSetAmbient(yaroze.ONE/2, yaroze.ONE/2, yaroze.ONE/2)
	yaroze.GsSetLightMode(0)
}

// initModel maps the model's addresses and links a handle to each of its
// objects, all in the car's coordinate system.
func initModel() {
	model.Map()
	objcnt = int32(model.Objects())
	if objcnt > objectMax {
		objcnt = objectMax
	}
	for i := range objcnt {
		model.Link(&object[i], i)
		object[i].Coord2 = &objWorld
		object[i].Attribute = 0
	}
}

// initTexture puts the seven textures into video memory.
func initTexture() {
	for _, t := range &textures {
		t.Load()
	}
}

// padRead applies the controls to the car and returns -1 when Select asks
// to quit. Start, on the frame it goes down, switches the frame rate.
func padRead() int32 {
	padd := yaroze.PadRead()

	if padd&yaroze.PADselect != 0 {
		return -1
	}

	if padd&yaroze.PADstart != 0 && padd != paddOld {
		frame ^= 1
	}

	if padd&yaroze.PADRleft != 0 {
		rotate.Vy -= rotSpeed[frame]
	}
	if padd&yaroze.PADRright != 0 {
		rotate.Vy += rotSpeed[frame]
	}
	if padd&yaroze.PADRup != 0 {
		rotate.Vx += rotSpeed[frame]
	}
	if padd&yaroze.PADRdown != 0 {
		rotate.Vx -= rotSpeed[frame]
	}

	if padd&yaroze.PADL1 != 0 && objWorld.Coord.T[2] < 500 {
		objWorld.Coord.T[2] += traSpeed[frame]
	}
	if padd&yaroze.PADL2 != 0 {
		objWorld.Coord.T[2] -= traSpeed[frame]
	}
	if padd&yaroze.PADLdown != 0 {
		objWorld.Coord.T[1] += traSpeed[frame]
	}
	if padd&yaroze.PADLup != 0 {
		objWorld.Coord.T[1] -= traSpeed[frame]
	}
	if padd&yaroze.PADLleft != 0 {
		objWorld.Coord.T[0] += traSpeed[frame]
	}
	if padd&yaroze.PADLright != 0 {
		objWorld.Coord.T[0] -= traSpeed[frame]
	}

	// The car turns on its own too.
	rotate.Vy += rotSpeed[frame] / 2

	paddOld = padd
	return 0
}

// opening drives the car in from the distance to the foreground, over 470
// frames at sixty a second.
func opening() {
	objWorld.Coord.T[0] = -930
	objWorld.Coord.T[1] = 60
	objWorld.Coord.T[2] = -6050
	rotate.Vy = 110

	for i := int32(0); i < 470; i++ {
		objWorld.Coord.T[0] += 2

		// The original adds 1 + i/20 in floating point to the integer z
		// and C truncates the sum towards zero: while z is negative a
		// fraction rounds the step up, and once it is positive the
		// fraction is dropped.
		sum := objWorld.Coord.T[2] + 1 + i/20
		if sum < 0 && i%20 != 0 {
			sum++
		}
		objWorld.Coord.T[2] = sum

		moveObject()
		idx := yaroze.GsGetActiveBuff()
		drawObject(&db[idx], frame60)
	}
}

// moveObject rebuilds the car's matrix from its position and angles.
func moveObject() {
	tmpmat := yaroze.IDMATRIX
	tmpmat.T = objWorld.Coord.T
	tmpvec := rotate
	yaroze.RotMatrix(&tmpvec, &tmpmat)
	objWorld.Coord = tmpmat
	objWorld.Flg = 0
}

// drawObject draws a frame into db: the car's objects into the ordering
// table, a wait for one or two vertical syncs by the frame rate, the speed
// line, and the swap.
func drawObject(db *DB, frame int32) {
	yaroze.GsSetWorkBase(yaroze.Packets(&db.packet[0]))
	yaroze.GsClearOt(0, 0, &db.gsot)

	for i := range objcnt {
		op := &object[i]
		var tmpmat yaroze.MATRIX
		yaroze.GsGetLw(op.Coord2, &tmpmat)
		yaroze.GsSetLightMatrix(&tmpmat)
		yaroze.GsGetLs(op.Coord2, &tmpmat)
		yaroze.GsSetLsMatrix(&tmpmat)
		yaroze.GsSortObject4(op, &db.gsot, 14-otLength, yaroze.Scratch())
	}

	yaroze.DrawSync(0)

	switch frame {
	case frame30:
		yaroze.VSync(2)
	case frame60:
		yaroze.VSync(0)
	}

	text.Reset()
	text.WriteString("speed = ").WriteInt(int(30 * (frame + 1))).WriteString(" frame/sec\n").Print()
	yaroze.FntFlush(-1)

	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(0, 0, 0, &db.gsot)
	yaroze.GsDrawOt(&db.gsot)
	frameNumber++
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-304, -112, 320, 16, 0, 64)
	for yaroze.PadRead()&yaroze.PADselect == 0 {
		buff := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&db[buff].packet[0]))
		yaroze.GsClearOt(0, 0, &db[buff].gsot)
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &db[buff].gsot)
		yaroze.GsDrawOt(&db[buff].gsot)
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}
