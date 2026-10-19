// A sprite painted onto a sphere's texture while it runs: drawing into video
// memory off the screen.
//
// A port of L. Evans's Net Yaroze tutorial on retexturing at run time
// (Sony, 1997). A sphere, textured from a 16-bit image, sits over a square
// of floor. Every frame, after drawing the scene, the program points the
// drawing environment at the part of VRAM outside the two display buffers
// where the sphere's texture lives, draws a sprite there, and points it back:
// the sprite is painted onto the texture, and the sphere wears it from the
// next frame on. Four seconds in, it also copies a block of VRAM over
// another, the simplest way to change a texture. The sprite is also drawn on
// screen, in the top left, as it is.
//
// Differences from the original:
//
//   - The data is read off the disc to the addresses the original's batch
//     script sent it to.
//   - What the original prints to the serial console, a line every frame
//     and the drawing environment at the start, is left out. What it shows
//     on screen is kept.
//   - The second ordering tables, through which the sprite is drawn onto
//     the texture, have the two entries their length of 1 gives them. The
//     original declares them an entry long, so clearing one writes past
//     its end into whatever follows; here that was the view.
//   - Code the original never runs is left out: the sprite placement for
//     objects drawn as sprites, of which it has none, and the palette copy
//     it has commented out.
//
// # Controls
//
// The right-hand buttons move the sprite over the texture, the directional
// buttons and L1 and L2 move the sphere, Select makes either faster, Start
// pauses and resumes, and Start and Select together quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	screenW, screenH = 320, 240
	packetMax        = 2048 * 24
	otLength         = 9

	// Object types.
	ship   = 0
	square = 1

	maxSquares = 256
	squareSize = 128
	floorX     = 1
	floorZ     = 1
	floorY     = 128

	// Where the ship may go.
	shipMinX = -floorY / 2
	shipMinY = -floorY * 2
	shipMinZ = -floorY / 2
	shipMaxX = 31 * squareSize / 2
	shipMaxY = floorY / 2
	shipMaxZ = 31 * squareSize / 2
)

// The models and the textures, read into the program's own memory, in
// buffers of whole sectors.
var (
	squareData [(112 + 2047) / 2048 * 512]uint32
	shipData   [(2044 + 2047) / 2048 * 512]uint32
	waveData   [(2112 + 2047) / 2048 * 512]uint32
	fireData   [(576 + 2047) / 2048 * 512]uint32
	tileData   [(2112 + 2047) / 2048 * 512]uint32
	tile2Data  [(2112 + 2047) / 2048 * 512]uint32
	sphereData [(8248 + 2047) / 2048 * 512]uint32
	dongleData [(131092 + 2047) / 2048 * 512]uint32

	squareFile = disc.Register("DATA/TEN.TMD", "data/TEN.TMD").Into(squareData[:])
	shipFile   = disc.Register("DATA/THESHIP.TMD", "data/THESHIP.TMD").Into(shipData[:])
	waveFile   = disc.Register("DATA/WAVE09.TIM", "data/WAVE09.TIM").Into(waveData[:])
	fireFile   = disc.Register("DATA/FIRE7.TIM", "data/FIRE7.TIM").Into(fireData[:])
	tileFile   = disc.Register("DATA/TILE2.TIM", "data/TILE2.TIM").Into(tileData[:])
	tile2File  = disc.Register("DATA/TILE3.TIM", "data/TILE3.TIM").Into(tile2Data[:])
	sphereFile = disc.Register("DATA/16BIT.TMD", "data/16BIT.TMD").Into(sphereData[:])
	dongleFile = disc.Register("DATA/DONGLE.TIM", "data/DONGLE.TIM").Into(dongleData[:])
	files      = []*disc.File{&squareFile, &shipFile, &waveFile, &fireFile, &tileFile, &tile2File, &sphereFile, &dongleFile}

	squareModel = yaroze.TMD(squareData[:])
	sphereModel = yaroze.TMD(sphereData[:])
	waveTIM     = yaroze.TIM(waveData[:])
	fireTIM     = yaroze.TIM(fireData[:])
	tileTIM     = yaroze.TIM(tileData[:])
	tile2TIM    = yaroze.TIM(tile2Data[:])
	dongleTIM   = yaroze.TIM(dongleData[:])
)

var (
	packetArea      [2][packetMax / 4]uint32
	otherPacketArea [2][packetMax / 4]uint32

	wot       [2]yaroze.GsOT
	wtags     [2][1 << otLength]yaroze.GsOT_TAG
	otherOT   [2]yaroze.GsOT
	otherTags [2][2]yaroze.GsOT_TAG // a length of 1 is two entries

	view       yaroze.GsRVIEW2
	projection int32 = 192
	fogging    yaroze.GsFOGPARAM

	waveTexture, fireTexture, tileTexture, tile2Texture, dongleTexture yaroze.GsIMAGE

	theShip    lpge.Handler
	theSquares [maxSquares]lpge.Handler
	displayed  [lpge.MaxObjects]*lpge.Handler

	theSprite yaroze.GsSPRITE

	frameNumber int32
)

var (
	lineBuf [256]byte
	line    yaroze.Buffer
)

func main() {
	line.Init(lineBuf[:])
	if !initialiseAll() {
		return
	}

	side := yaroze.GsGetActiveBuff()
	var hsync int32
	for {
		if frameNumber == 240 {
			// Copy a block of VRAM over the square's texture, and a
			// palette over another.
			rect := yaroze.RECT{X: 320, Y: 256, W: 64, H: 64}
			yaroze.MoveImage(&rect, 640, 0)
			rect = yaroze.RECT{X: 0, Y: 488, W: 16, H: 1}
			yaroze.MoveImage(&rect, 0, 480)
			yaroze.DrawSync(0)
		}

		line.Reset()
		line.WriteString("frame: ").WriteInt(int(frameNumber)).WriteString("\n")
		line.WriteString("hsync: ").WriteInt(int(hsync)).WriteString("\n")
		line.WriteString("ship pos:\n")
		line.WriteString("sprite: ").WriteInt(int(theSprite.X)).WriteString(" ").WriteInt(int(theSprite.Y)).WriteString("\n")

		frameNumber++
		if !dealWithControllerPad() {
			break
		}

		yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
		yaroze.GsClearOt(0, 0, &wot[side])

		// Paused, the picture is drawn as it is, with nothing moving.
		if !paused {
			handleAllObjects()
		}
		yaroze.GsSetRefView2(&view)

		n := performWorldClipping()
		line.WriteString("num to display: ").WriteInt(n).WriteString("\n")
		line.Print()

		for _, o := range displayed[:n] {
			var ls yaroze.MATRIX
			yaroze.GsGetLs(&o.Coord, &ls)
			yaroze.GsSetLightMatrix(&ls)
			yaroze.GsSetLsMatrix(&ls)
			yaroze.GsSortObject4(&o.Handler, &wot[side], 3, yaroze.Scratch())
		}

		// The sprite as it is, on top of everything.
		yaroze.GsSortFastSprite(&theSprite, &wot[side], 0)

		hsync = yaroze.VSync(0)
		yaroze.ResetGraph(1)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(64, 64, 128, &wot[side])
		yaroze.GsDrawOt(&wot[side])
		side = yaroze.GsGetActiveBuff()
		yaroze.FntFlush(-1)

		// Then the sprite again, drawn outside both display buffers, onto
		// the sphere's texture: the clip and offset of the drawing
		// environment moved there, and put back after.
		yaroze.GsClearOt(0, 0, &otherOT[side])
		yaroze.GsSetWorkBase(yaroze.Packets(&otherPacketArea[side][0]))
		env := yaroze.GsDRAWENV()
		clip, ofs := env.Clip, env.Ofs
		env.Clip = yaroze.RECT{X: 320, Y: 0, W: 256, H: 256}
		env.Ofs = [2]int16{320, 0}
		yaroze.PutDrawEnv(env)

		yaroze.GsSortSprite(&theSprite, &otherOT[side], 0)
		yaroze.GsDrawOt(&otherOT[side])

		env.Clip, env.Ofs = clip, ofs
		yaroze.PutDrawEnv(env)
	}
	yaroze.ResetGraph(3)
}

// performWorldClipping puts the living objects near the viewpoint into
// displayed, and returns how many there are.
func performWorldClipping() int {
	const clipDistance = squareSize * 6
	vp := findViewPoint()
	n := 0
	for _, o := range &lpge.Objects {
		if o == nil || !o.Alive {
			continue
		}
		p := &o.Position
		if abs(p.Vx-vp.Vx) < clipDistance && abs(p.Vy-vp.Vy) < clipDistance && abs(p.Vz-vp.Vz) < clipDistance {
			displayed[n] = o
			n++
		}
	}
	return n
}

// findViewPoint is where the viewpoint is, taking the view as behind the
// ship along its z axis, as the original does.
func findViewPoint() yaroze.VECTOR {
	m := &theShip.Coord.Coord.M
	return yaroze.VECTOR{
		Vx: theShip.Position.Vx + int32(m[2][0])*view.Vpz>>12,
		Vy: theShip.Position.Vy + int32(m[2][1])*view.Vpz>>12,
		Vz: theShip.Position.Vz + int32(m[2][2])*view.Vpz>>12,
	}
}

// controller is the pad as the frame sees it, which tells the press of
// Start that pauses from the frames it stays held; paused is whether it
// has, until the next press.
var (
	controller yaroze.Pad
	paused     bool
)

// dealWithControllerPad acts on the pad, and reports false when the program
// is to quit. Only the first of the moves held is made, and none while
// paused.
func dealWithControllerPad() bool {
	pad := controller.Update()
	if pad&yaroze.PADstart != 0 && pad&yaroze.PADselect != 0 {
		return false
	}
	if controller.Pressed(yaroze.PADstart) {
		paused = !paused
	}
	if paused {
		return true
	}
	speed := int32(1)
	if pad&yaroze.PADselect != 0 {
		speed = 5
	}
	switch {
	case pad&yaroze.PADRup != 0:
		theSprite.Y -= int16(speed)
	case pad&yaroze.PADRdown != 0:
		theSprite.Y += int16(speed)
	case pad&yaroze.PADRleft != 0:
		theSprite.X -= int16(speed)
	case pad&yaroze.PADRright != 0:
		theSprite.X += int16(speed)
	case pad&yaroze.PADL1 != 0:
		theShip.Position.Vz += speed
	case pad&yaroze.PADL2 != 0:
		theShip.Position.Vz -= speed
	case pad&yaroze.PADLup != 0:
		theShip.Position.Vy -= speed
	case pad&yaroze.PADLdown != 0:
		theShip.Position.Vy += speed
	case pad&yaroze.PADLleft != 0:
		theShip.Position.Vx -= speed
	case pad&yaroze.PADLright != 0:
		theShip.Position.Vx += speed
	}
	return true
}

func initialiseAll() bool {
	yaroze.PadInit()
	yaroze.GsInitGraph(screenW, screenH, yaroze.GsINTER|yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenH)
	yaroze.GsInit3D()

	for i := range wot {
		wot[i].Length = otLength
		wot[i].Org = &wtags[i][0]
		yaroze.GsClearOt(0, 0, &wot[i])
		otherOT[i].Length = 1
		otherOT[i].Org = &otherTags[i][0]
		yaroze.GsClearOt(0, 0, &otherOT[i])
	}

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(-120, 40, 256, 200, 0, 512)
	if missing := demo.Load(files, nil); missing != "" {
		for {
			pad := yaroze.PadRead()
			if pad&yaroze.PADstart != 0 && pad&yaroze.PADselect != 0 {
				break
			}
			line.Reset()
			line.WriteString("could not read\n").WriteString(missing).Print()
			yaroze.VSync(0)
			yaroze.GsSwapDispBuff()
			side := yaroze.GsGetActiveBuff()
			yaroze.GsSetWorkBase(yaroze.Packets(&packetArea[side][0]))
			yaroze.GsClearOt(0, 0, &wot[side])
			yaroze.GsSortClear(64, 64, 128, &wot[side])
			yaroze.GsDrawOt(&wot[side])
			yaroze.FntFlush(-1)
		}
		yaroze.ResetGraph(3)
		return false
	}

	lpge.ProperInitialiseTexture(waveTIM, &waveTexture)
	lpge.ProperInitialiseTexture(fireTIM, &fireTexture)
	lpge.ProperInitialiseTexture(tileTIM, &tileTexture)
	lpge.ProperInitialiseTexture(tile2TIM, &tile2Texture)
	lpge.ProperInitialiseTexture(dongleTIM, &dongleTexture)

	lpge.LinkSpriteToImageInfo(&theSprite, &fireTexture)
	theSprite.Mx = int16(theSprite.W / 2)
	theSprite.My = int16(theSprite.H / 2)
	theSprite.X, theSprite.Y = 80, 70

	initialiseView()
	initialiseLighting()
	initialiseObjects()
	createTheFloor()
	return true
}

func initialiseObjects() {
	lpge.InitialiseObjectClass()

	theShip.Init()
	theShip.BringToLife(ship, sphereModel, 0, lpge.None)
	theShip.RotationSpeed = yaroze.ONE / 128
	theShip.MovementSpeed = 1
	theShip.MovementMomentumFlag = true
	// Turned upside down to face the right way, though the first frame's
	// update sets its coordinates from its matrix, which undoes it.
	theShip.Rotate.Vx = yaroze.ONE / 2
	setCoordinates(&theShip.Rotate, &theShip.Position, &theShip.Coord)
	theShip.Rotate = yaroze.SVECTOR{}
	theShip.Position = yaroze.VECTOR{Vx: squareSize * 2, Vz: squareSize * 2}
	lpge.Register(&theShip)

	for i := range theSquares {
		theSquares[i].Init()
		theSquares[i].BringToLife(square, squareModel, 0, lpge.None)
		lpge.Register(&theSquares[i])
	}

	lpge.LinkAllObjectsToModelsOrSprites()
	lpge.LinkAllObjectsToTheirCoordinateSystems()

	for i := range theSquares {
		theSquares[i].Alive = false
	}
}

func createTheFloor() {
	id := 0
	for x := range floorX {
		for z := range floorZ {
			s := &theSquares[id]
			s.Alive = true
			s.Position = yaroze.VECTOR{Vx: int32(x * squareSize), Vy: floorY, Vz: int32(z * squareSize)}
			s.Rotate = yaroze.SVECTOR{Vx: yaroze.ONE / 2}
			setCoordinates(&s.Rotate, &s.Position, &s.Coord)
			s.Rotate = yaroze.SVECTOR{}
			id++
		}
	}
}

func handleAllObjects() {
	handleAllSquares()
	for _, o := range &lpge.Objects {
		if o == nil || !o.Alive {
			continue
		}
		o.LifeTime++
		if o.Type == ship {
			handleTheShip(o)
		}
	}
}

func handleTheShip(o *lpge.Handler) {
	previous := o.Position
	moveObjectRelative(&o.Twist, &o.Position, &o.Velocity, &o.Coord, &o.Matrix)
	if !o.MovementMomentumFlag {
		o.Velocity = yaroze.VECTOR{}
	}
	if !o.RotationMomentumFlag {
		o.Twist = yaroze.VECTOR{}
	}
	p := &o.Position
	if p.Vx < shipMinX || p.Vx > shipMaxX || p.Vy < shipMinY || p.Vy > shipMaxY || p.Vz < shipMinZ || p.Vz > shipMaxZ {
		o.Position = previous
		o.Velocity = yaroze.VECTOR{}
	}
}

// handleAllSquares subdivides the squares near the viewpoint. What it does
// to the rest, as the original has it, is clear the bit that turns an object
// off, which is never set, rather than the one that subdivides it.
func handleAllSquares() {
	const veryClose = squareSize * 2
	vp := findViewPoint()
	for i := range theSquares {
		s := &theSquares[i]
		if !s.Alive {
			continue
		}
		p := &s.Position
		if abs(p.Vx-vp.Vx) < veryClose && abs(p.Vy-vp.Vy) < veryClose && abs(p.Vz-vp.Vz) < veryClose {
			s.Handler.Attribute |= yaroze.GsDIV1
		} else {
			s.Handler.Attribute &^= yaroze.GsDOFF
		}
	}
}

// setCoordinates is the tutorial's UpdateObjectCoordinates: rotation and
// position, both relative to the world.
func setCoordinates(rotation *yaroze.SVECTOR, position *yaroze.VECTOR, c *yaroze.GsCOORDINATE2) {
	yaroze.RotMatrix(rotation, &c.Coord)
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

// moveObjectRelative is the tutorial's UpdateObjectCoordinates2: movement
// and rotation relative to the object itself.
func moveObjectRelative(twist *yaroze.VECTOR, position, velocity *yaroze.VECTOR, c *yaroze.GsCOORDINATE2, m *yaroze.MATRIX) {
	var real yaroze.VECTOR
	yaroze.ApplyMatrixLV(m, velocity, &real)
	position.Vx += real.Vx
	position.Vy += real.Vy
	position.Vz += real.Vz

	var xm, ym, zm yaroze.MATRIX
	xv := yaroze.SVECTOR{Vx: int16(twist.Vx)}
	yv := yaroze.SVECTOR{Vy: int16(twist.Vy)}
	zv := yaroze.SVECTOR{Vz: int16(twist.Vz)}
	yaroze.RotMatrix(&xv, &xm)
	yaroze.RotMatrix(&yv, &ym)
	yaroze.RotMatrix(&zv, &zm)
	yaroze.MulMatrix0(m, &xm, m)
	yaroze.MulMatrix0(m, &ym, m)
	yaroze.MulMatrix0(m, &zm, m)

	c.Coord = *m
	c.Coord.T = [3]int32{position.Vx, position.Vy, position.Vz}
	c.Flg = 0
}

func initialiseView() {
	yaroze.GsSetProjection(projection)
	view = yaroze.GsRVIEW2{
		Vrx: squareSize * 2, Vry: 0, Vrz: squareSize * 2,
		Vpx: squareSize * 2, Vpy: 0, Vpz: squareSize*2 + 300,
		Super: nil,
	}
	yaroze.GsSetRefView2(&view)
}

// initialiseLighting sets the ambient light and the fog, which is off. The
// original sets no flat lights.
func initialiseLighting() {
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	fogging = yaroze.GsFOGPARAM{}
	yaroze.GsSetFogParam(&fogging)
	yaroze.GsSetLightMode(0)
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
