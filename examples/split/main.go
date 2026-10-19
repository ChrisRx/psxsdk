// A split screen: two players, each with a cube to drive among stone
// pillars and a camera of their own following it, one above the other or
// side by side.
//
// A port of the SCEE tutorial's chapter 10, examples 20 and 21, the
// horizontal and vertical split screens, in one program. The trick is two
// ordering tables a frame, one for each half. Each half's models are sorted
// into its own table, with the GTE set to that half's camera, and when the
// frame is drawn each table is drawn with its own drawing offset, the
// screen position the camera's centre lands on, and its own clip, the half
// of the screen it may draw in. GsSetOffset and GsSetClip change both, for
// the buffer being drawn, and the next GsSwapDispBuff puts them back.
//
// They take effect at once rather than in turn with what the GPU is
// drawing, and the library asks that the GPU not be drawing when they are
// called; so the frame waits with DrawSync for one half to be drawn before
// it sets up the next. The tutorial did not wait, which leaves the second
// half's settings to land in the middle of the first half's drawing.
//
// The scene is drawn twice, once for each camera, so a split screen costs
// twice the sorting of one view; a game with two players kept its scenes
// smaller, or drew less far, to make up for it. Here it is ten cubes.
//
// The models and the texture are the tutorial's own: CUBE.TMD, a cube of
// coloured, lit triangles, for the players, and CUBETEX.TMD with
// STONE02.TIM for the pillars.
//
// Differences from the tutorial: there each half showed one of two cubes,
// moved with the one controller, through one camera; here each half has a
// camera that follows its player's cube, and the scene is the same in both;
// the two examples are one, switched with select; and the frame waits for
// each half to be drawn.
//
// # Controls
//
// The directions on the first controller move the first cube; the second
// cube moves with the second controller's directions, or with the first
// controller's triangle, cross, square and circle. Select switches between
// the halves one above the other and side by side.
package main

//go:generate go run go.chrisrx.dev/psx/tools/discgen data

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	depth   = 6  // the ordering tables hold 1<<depth depths
	speed   = 60 // world units a frame
	pillars = 8
	arena   = 6000 // how far the pillars stand from the middle
)

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 20, G: 30, B: 60}, Depth: depth, Packets: 24 << 10}
	pad1   yaroze.Pad
	pad2   yaroze.Controller

	// second is the second half's ordering tables, one for each buffer;
	// the screen keeps the first half's.
	second     [2]yaroze.GsOT
	secondTags [2 << depth]yaroze.GsOT_TAG

	players [2]yaroze.Object
	stones  [pillars]yaroze.Object
	views   [2]yaroze.GsRVIEW2
	light   yaroze.GsF_LIGHT

	// sideBySide is whether the halves are side by side, and frames the
	// frames drawn: both for a test to read.
	sideBySide bool
	frames     int32

	text    yaroze.Buffer
	textBuf [256]byte
)

// The pillars' places on a ring, in eighths of a turn: cos and sin times
// arena, rounded.
var ring = [pillars][2]int32{
	{6000, 0}, {4243, 4243}, {0, 6000}, {-4243, 4243},
	{-6000, 0}, {-4243, -4243}, {0, -6000}, {4243, -4243},
}

func main() {
	screen.Open()
	yaroze.GsInit3D()
	for i := range second {
		second[i] = yaroze.GsOT{Length: depth, Org: &secondTags[i<<depth]}
	}
	b := screen.Bounds()
	font := screen.Font(int32(b.X)+8, int32(b.Y)+8, int32(b.W)-16, int32(b.H)-16, 200)
	text.Init(textBuf[:])

	if f := disc.LoadAll(300, files...); f != nil {
		for range screen.Frames() {
			yaroze.Print("could not read ")
			yaroze.Print(f.Name)
		}
	}
	setUp()

	for {
		ot := screen.Begin()
		buf := yaroze.GsGetActiveBuff()
		yaroze.GsClearOt(0, 0, &second[buf])

		pad1.Update()
		pad2 = yaroze.ReadController(1)
		if pad1.Pressed(yaroze.PADselect) {
			sideBySide = !sideBySide
		}
		dx, dz := pad1.Dir()
		drive(&players[0], dx, -dz)
		dx, dz = faceDir()
		if pad2.Connected {
			if x, z := padDir(pad2.Buttons); x != 0 || z != 0 {
				dx, dz = x, z
			}
		}
		drive(&players[1], dx, -dz)

		// Each half: its camera, then the scene sorted into its table.
		for i, t := range [2]*yaroze.GsOT{ot, &second[buf]} {
			follow(&views[i], &players[i])
			views[i].Set()
			for j := range players {
				players[j].Draw(t)
			}
			for j := range stones {
				stones[j].Draw(t)
			}
		}

		text.Reset()
		text.WriteString("~c990PLAYER 1~c999")
		if sideBySide {
			text.WriteString("                ~c099PLAYER 2~c999\n")
		} else {
			text.WriteString("\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n~c099PLAYER 2~c999\n")
		}
		text.Print()

		draw(ot, &second[buf], font)
	}
}

// setUp loads the models and texture, places the cubes, and sets the
// light.
func setUp() {
	yaroze.TIM(stone02TIM.Data()).Load()
	cube := yaroze.TMD(cubeTMD.Data())
	cube.Map()
	stone := yaroze.TMD(cubetexTMD.Data())
	stone.Map()

	for i := range players {
		p := &players[i]
		p.Init(yaroze.WORLD)
		p.Link(cube, 0)
		p.Position = yaroze.VECTOR{Vx: int32(i)*3000 - 1500}
	}
	for i := range stones {
		s := &stones[i]
		s.Init(yaroze.WORLD)
		s.Link(stone, 0)
		// Tall and thin, standing on the floor the cubes slide on.
		s.Scale = yaroze.VECTOR{Vx: yaroze.ONE, Vy: 3 * yaroze.ONE, Vz: yaroze.ONE}
		s.Position = yaroze.VECTOR{Vx: ring[i][0], Vy: -1000, Vz: ring[i][1]}
	}

	yaroze.GsSetProjection(256)
	light = yaroze.GsF_LIGHT{Vx: 1, Vy: 2, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	light.Set(0)
	yaroze.GsSetAmbient(yaroze.ONE/3, yaroze.ONE/3, yaroze.ONE/3)
	yaroze.GsSetLightMode(0)
}

// drive moves a cube across the floor, keeps it within the ring, and turns
// it the way it is going.
func drive(p *yaroze.Object, dx, dz int) {
	if dx == 0 && dz == 0 {
		return
	}
	p.Position.Vx = max(-arena, min(arena, p.Position.Vx+int32(dx*speed)))
	p.Position.Vz = max(-arena, min(arena, p.Position.Vz+int32(dz*speed)))
	p.Rotation.Vy += int16(dx * 32)
	p.Rotation.Vx += int16(dz * 32)
}

// follow puts a camera behind and above a cube, looking at it.
func follow(v *yaroze.GsRVIEW2, p *yaroze.Object) {
	*v = yaroze.GsRVIEW2{
		Vpx: p.Position.Vx, Vpy: p.Position.Vy - 3000, Vpz: p.Position.Vz - 6000,
		Vrx: p.Position.Vx, Vry: p.Position.Vy, Vrz: p.Position.Vz,
		Super: yaroze.WORLD,
	}
}

// faceDir is the first controller's face buttons as directions, as the
// tutorial moved its second cube.
func faceDir() (dx, dz int) {
	if pad1.Held(yaroze.PADRleft) {
		dx--
	}
	if pad1.Held(yaroze.PADRright) {
		dx++
	}
	if pad1.Held(yaroze.PADRup) {
		dz--
	}
	if pad1.Held(yaroze.PADRdown) {
		dz++
	}
	return dx, dz
}

// padDir is a controller's directions, as Pad.Dir gives them.
func padDir(b yaroze.Buttons) (dx, dz int) {
	if b&yaroze.PADLleft != 0 {
		dx--
	}
	if b&yaroze.PADLright != 0 {
		dx++
	}
	if b&yaroze.PADLup != 0 {
		dz--
	}
	if b&yaroze.PADLdown != 0 {
		dz++
	}
	return dx, dz
}

// draw ends the frame as Screen.End does, but draws each half's table with
// that half's offset and clip, waiting for the GPU between them.
func draw(first, second *yaroze.GsOT, font int32) {
	w, h := int32(screen.Width), int32(screen.Height)
	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	// The clear is a fill, which no clip holds in: it clears the whole
	// screen though it is drawn with the first half.
	yaroze.GsSortClear(screen.Background.R, screen.Background.G, screen.Background.B, first)

	halves := [2]yaroze.RECT{
		{W: int16(w), H: int16(h / 2)},
		{Y: int16(h / 2), W: int16(w), H: int16(h / 2)},
	}
	if sideBySide {
		halves = [2]yaroze.RECT{
			{W: int16(w / 2), H: int16(h)},
			{X: int16(w / 2), W: int16(w / 2), H: int16(h)},
		}
	}
	for i, t := range [2]*yaroze.GsOT{first, second} {
		r := &halves[i]
		// The camera's centre goes to the middle of its half.
		yaroze.GsSetOffset(int32(r.X)+int32(r.W)/2, int32(r.Y)+int32(r.H)/2)
		yaroze.GsSetClip(r)
		yaroze.GsDrawOt(t)
		yaroze.DrawSync(0)
	}

	// The text across the whole screen, from its middle as GsInit3D set.
	whole := yaroze.RECT{W: int16(w), H: int16(h)}
	yaroze.GsSetOffset(w/2, h/2)
	yaroze.GsSetClip(&whole)
	yaroze.FntFlush(font)
	frames++
}
