// A cube that runs and jumps along a level of tiles laid out in the
// playground's scene editor, turning a quarter turn through each jump and
// taking coins on the way to a flag, with hills far off scrolling at half
// the speed behind.
//
// The level is scenes/level.json, cooked by mkscene into LEVEL.DAT and
// read with package scene. It has two tile layers: "far", the hills and
// clouds, which moves at half the speed of the scroll, and "ground", the
// level itself, whose cells carry the flags the editor sets on its cell
// palette. Grass, dirt and blocks are solid. The planks are one
// way, stood on from above and jumped up through from below. The spikes
// hurt, the coins are picked up, and the flag has the first of the four
// flags the editor leaves to the program, which this program takes to
// mean the end of the level. The program never looks at which picture a
// cell has, only at its flags: it moves the hero a pixel at a time and
// asks the ground layer with FlagsAt whether the hero's box would go into
// anything solid, takes a coin by emptying its cell with SetCell, and
// scrolls both layers with Scroll, each at its own speed. The hero is the
// scene's sprite named so, the cube, so the hero starts where the editor
// put that sprite; the program draws it itself, turned about its middle
// with GsSortSprite, which the scene's own drawing of its sprites does
// not do, and has the scene leave its copy undrawn. Since the level is a
// scene, a change made in the editor while the program runs is in the
// level at the next frame.
//
// The hero runs at up to two pixels a frame and falls faster by six
// sixteenths of a pixel every frame, everything in sixteenths of a pixel,
// since the machine has no floating point. A jump can start a few frames
// after running off an edge, and a press of Cross a few frames before
// landing still jumps, as players of the genre expect. Off the ground
// the cube turns the way it faces, a quarter turn in the time a full
// jump takes, and on landing it finishes the quarter it was turning to in
// a few frames. Spikes, or a fall out of the level, put the hero back on
// the last solid ground they stood on.
//
// # Controls
//
// Left and right run. Cross jumps, higher the longer it is held. Start
// plays the level again once the flag is reached.
package main

// The pictures, made by gen/ as an artist would in a paint program, become
// TIMs, the level a DAT, then the files under assets/ are declared in
// zz_generated.disc.go.
//go:generate go run ./gen assets
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 320,0 --clut 0,480 -o assets/TILES.TIM assets/tiles.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 320,32 --clut 0,481 -o assets/FAR.TIM assets/far.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 320,64 --clut 0,482 -o assets/HERO.TIM assets/hero.png
//go:generate go run go.chrisrx.dev/psx/tools/mkscene -o assets/LEVEL.DAT scenes/level.json
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/scene"
	"go.chrisrx.dev/psx/yaroze"
)

// The flag the level's goal has: the first of the four the editor leaves
// to the program.
const goal = 1 << 4

// How the cube turns, in the library's angles, 4096 to a degree: a
// quarter turn in about the frames a full jump lasts, and the rest of a
// quarter in a few once it lands.
const (
	quarter  = 90 * 4096
	spinRate = quarter / 32
	settle   = quarter / 6
)

// doff is the attribute bit that leaves a sprite undrawn.
const doff = 1 << 31

// How the hero moves, in sixteenths of a pixel and frames.
const (
	runSpeed  = 32  // the fastest the hero runs, two pixels a frame
	runAccel  = 4   // how much faster each frame a direction is held
	airAccel  = 3   // the same in the air, a little less
	friction  = 4   // how much slower each frame none is
	gravity   = 6   // how much faster each frame the hero falls
	fallSpeed = 96  // the fastest the hero falls, less than a cell a frame
	jumpSpeed = 104 // the speed a jump starts at, about three and a half cells high
	jumpCut   = 32  // letting go of Cross on the way up slows the rise to this
	coyote    = 6   // frames after running off an edge that a jump still starts
	buffered  = 6   // frames a press of Cross waits for the hero to land
)

// The hero's box, about the middle of their feet: ten pixels wide and
// fourteen high, the cube's height and narrower than it, so that a jump
// past a corner is not caught on it.
const (
	halfW = 5
	tall  = 14
)

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 104, G: 172, B: 236}, Depth: 2}
	pad    yaroze.Pad
	world  scene.Scene
	failed *disc.File
	err    error

	// The level is read into memory with room to spare, so that the
	// playground can write a larger level over it while the program runs.
	levelData [8192]uint32

	// The hero, in plain variables rather than a struct, which the
	// compiler would split into a global for each field, so that a test
	// finds them by name.
	heroX, heroY   int  // the middle of their feet, in pixels of the level
	subX, subY     int  // the sixteenths of a pixel past that
	speedX, speedY int  // in sixteenths of a pixel a frame
	standing       bool // on something that holds them up
	airTime        int  // frames since they last stood on something
	jumpWait       int  // frames a press of Cross still waits to start a jump
	facingLeft     bool // which way they face
	spin, target   int  // the cube's angle, and the quarter it turns to

	startX, startY int  // where the editor put the hero
	safeX, safeY   int  // the last solid ground the hero stood on
	camX           int  // the scroll: the level's pixel at the screen's left
	coins, total   int  // the coins taken, and how many the level had
	won            bool // the flag reached

	// cube is the hero's sprite as the program draws it, turned.
	cube yaroze.GsSPRITE

	// The text, and dark panels under it, so that it reads over a cloud:
	// see-through, subtracting from what is under them, as the quest
	// example's talk box does.
	text    = yaroze.NewBuffer(160)
	hudBack = yaroze.GsBOXF{Attribute: 1<<30 | 2<<28, W: 120, H: 16, R: 150, G: 150, B: 150}
	wonBack = yaroze.GsBOXF{Attribute: 1<<30 | 2<<28, W: 176, H: 60, R: 150, G: 150, B: 150}
)

func main() {
	screen.Open()
	yaroze.GsInit3D()
	b := screen.Bounds()
	screen.Font(int32(b.X)+12, int32(b.Y)+10, 296, 120, 256)
	hudBack.X, hudBack.Y = b.X+8, b.Y+6
	wonBack.X, wonBack.Y = b.X+72, b.Y+72

	levelDAT = levelDAT.Into(levelData[:])
	if failed = disc.LoadAll(300, files...); failed == nil {
		start(b)
	}
	for ot := range screen.Frames() {
		if failed != nil {
			yaroze.Print("could not read ")
			yaroze.Print(failed.Name)
			continue
		}
		if err != nil {
			yaroze.Print(err.Error())
			continue
		}
		pad.Update()
		// A level changed in the editor: the coins are all back, and the
		// hero may have been moved.
		if world.Reload() {
			coins = 0
			read(b)
		}
		ground, sp := world.Layer("ground"), world.Sprite("hero")
		if ground == nil || sp == nil {
			yaroze.Print("the level needs a layer named ground\nand a sprite named hero\n")
			continue
		}
		if won {
			if pad.Pressed(yaroze.PADstart) {
				start(b)
				continue
			}
		} else {
			play(ground)
		}
		follow(ground, b)
		world.Scroll(camX, 0)
		show(sp, b)
		world.Draw(ot, ot)
		cube.Sort(ot, 0)

		// The panels go in after the cube, at the same depth, so that they
		// are drawn before it, under the cube. The text goes over everything.
		hudBack.Sort(ot, 0)
		text.Reset()
		text.WriteString("coins ").WriteInt(coins).WriteString(" of ").WriteInt(total)
		if won {
			wonBack.Sort(ot, 0)
			text.WriteString("\n\n\n\n\n\n\n\n\n            you made it!\n\n           ").
				WriteInt(coins).WriteString(" coins of ").WriteInt(total).
				WriteString("\n\n          start plays again")
		}
		text.Print()
	}
}

// start loads the level, which puts back the coins taken, and puts the
// hero where it starts.
func start(b yaroze.RECT) {
	if err = world.Load(levelDAT.Data(), scene.Files(files)); err != nil {
		return
	}
	read(b)
	heroX, heroY = startX, startY
	subX, subY, speedX, speedY = 0, 0, 0, 0
	safeX, safeY = startX, startY
	coins, won, camX = 0, false, 0
	spin, target = 0, 0
}

// read takes what the program needs from the level as the file has it:
// where the hero's sprite is, before the program moves it, and how many
// coins the ground has.
func read(b yaroze.RECT) {
	sp, ground := world.Sprite("hero"), world.Layer("ground")
	if sp == nil || ground == nil {
		return
	}
	startX = int(sp.X) - int(b.X) + int(sp.W)/2
	startY = int(sp.Y) - int(b.Y) + int(sp.H)
	total = 0
	for y := range ground.Height() {
		for x := range ground.Width() {
			if ground.Flags(x, y)&scene.Pickup != 0 {
				total++
			}
		}
	}
}

// play moves the hero a frame: running, jumping and falling, against the
// ground layer's cells, then what they touch.
func play(l *scene.Layer) {
	dx, _ := pad.Dir()
	accel := runAccel
	if !standing {
		accel = airAccel
	}
	switch {
	case dx != 0:
		speedX = max(-runSpeed, min(runSpeed, speedX+dx*accel))
		facingLeft = dx < 0
	case speedX > 0:
		speedX = max(0, speedX-friction)
	case speedX < 0:
		speedX = min(0, speedX+friction)
	}

	// A press of Cross waits a few frames for the ground, and the ground
	// stays under the hero for a few after they run off it.
	if pad.Pressed(yaroze.PADRdown) {
		jumpWait = buffered
	} else if jumpWait > 0 {
		jumpWait--
	}
	if jumpWait > 0 && airTime <= coyote {
		speedY = -jumpSpeed
		jumpWait = 0
		airTime = coyote + 1
	}
	if !pad.Held(yaroze.PADRdown) && speedY < -jumpCut {
		speedY = -jumpCut
	}
	speedY = min(fallSpeed, speedY+gravity)

	moveX(l)
	moveY(l)
	standing = holds(l, heroX, heroY)
	if standing {
		airTime = 0
		speedY = max(0, speedY)
		if speedY == 0 {
			subY = 0
		}
	} else {
		airTime++
	}
	turn()
	switch {
	case touch(l), heroY > l.Height()*scene.CellSize+2*tall:
		// Spikes, or a fall out of the level: back to the last safe place.
		heroX, heroY = safeX, safeY
		subX, subY, speedX, speedY = 0, 0, 0, 0
	case standing && l.FlagsAt(heroX-halfW, heroY)&l.FlagsAt(heroX+halfW-1, heroY)&scene.Solid != 0:
		// Solid ground under both feet, and nothing hurting, is somewhere
		// safe to come back to.
		safeX, safeY = heroX, heroY
	}
}

// moveX moves the hero across by their speed, a pixel at a time, stopping
// at a wall or the level's side.
func moveX(l *scene.Layer) {
	subX += speedX
	n := subX >> 4
	subX &= 15
	step := 1
	if n < 0 {
		step = -1
	}
	edge := halfW // the column the box's leading side moves into, from x
	if step < 0 {
		edge = -halfW - 1
	}
	for ; n != 0; n -= step {
		x := heroX + edge
		if x < 0 || x >= l.Width()*scene.CellSize || solid(l, x, heroY-tall) || solid(l, x, heroY-1) {
			speedX, subX = 0, 0
			return
		}
		heroX += step
	}
}

// moveY moves the hero up or down by their speed, a pixel at a time,
// stopping on what holds them up and under what they bump into.
func moveY(l *scene.Layer) {
	subY += speedY
	n := subY >> 4
	subY &= 15
	for ; n > 0; n-- {
		if holds(l, heroX, heroY) {
			speedY, subY = 0, 0
			return
		}
		heroY++
	}
	for ; n < 0; n++ {
		top := heroY - tall - 1
		if solid(l, heroX-halfW, top) || solid(l, heroX+halfW-1, top) {
			speedY, subY = 0, 0
			return
		}
		heroY--
	}
}

// solid reports whether the pixel at x, y of the level is in a solid cell.
func solid(l *scene.Layer, x, y int) bool { return l.FlagsAt(x, y)&scene.Solid != 0 }

// holds reports whether the row of pixels under the hero's feet at x, y
// holds them up: a solid cell under either side of the box, or the top
// row of a one-way cell, which the hero lands on from above and jumps up
// through from below.
func holds(l *scene.Layer, x, y int) bool {
	f := l.FlagsAt(x-halfW, y) | l.FlagsAt(x+halfW-1, y)
	return f&scene.Solid != 0 || f&scene.OneWay != 0 && y&(scene.CellSize-1) == 0
}

// touch takes the coins the hero's box overlaps, ends the level at the
// flag, and reports whether the hero is in spikes, which hurt only once
// the hero is a few pixels into them, so that grazing one is forgiven.
func touch(l *scene.Layer) bool {
	x0, x1 := (heroX-halfW)>>4, (heroX+halfW-1)>>4
	y0, y1 := (heroY-tall)>>4, (heroY-1)>>4
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			f := l.Flags(x, y)
			if f&scene.Pickup != 0 {
				l.SetCell(x, y, scene.EmptyCell)
				coins++
			}
			if f&goal != 0 {
				won = true
				speedX = 0
			}
		}
	}
	const in = 3
	return (l.FlagsAt(heroX-halfW+in, heroY-1-in)|l.FlagsAt(heroX+halfW-1-in, heroY-1-in)|
		l.FlagsAt(heroX-halfW+in, heroY-tall+in)|l.FlagsAt(heroX+halfW-1-in, heroY-tall+in))&scene.Hurts != 0
}

// follow scrolls the level to keep the hero within the middle fifth of the
// screen, never past the level's ends.
func follow(l *scene.Layer, b yaroze.RECT) {
	w := int(b.W)
	lo, hi := w*2/5, w*3/5
	if heroX-camX > hi {
		camX = heroX - hi
	}
	if heroX-camX < lo {
		camX = heroX - lo
	}
	camX = max(0, min(camX, l.Width()*scene.CellSize-w))
}

// turn spins the cube: off the ground it turns towards the next quarter
// the way it faces, and starts another when it gets there, so that a long
// fall keeps it turning; on the ground it finishes the quarter it was
// turning to, quickly, and rests, its angle kept within one turn.
func turn() {
	if !standing && spin == target {
		if facingLeft {
			target = spin - quarter
		} else {
			target = spin + quarter
		}
	}
	step := spinRate
	if standing {
		step = settle
	}
	switch {
	case spin < target:
		spin = min(spin+step, target)
	case spin > target:
		spin = max(spin-step, target)
	}
	if spin == target {
		spin %= 4 * quarter
		target = spin
	}
}

// show puts the cube where the hero is on the screen, turned about its
// middle, with the box's fourteen rows over their feet, and leaves the
// scene's own copy of the sprite undrawn.
func show(sp *yaroze.GsSPRITE, b yaroze.RECT) {
	cube = *sp
	cube.Attribute &^= doff
	cube.Mx, cube.My = int16(sp.W/2), int16(sp.H/2)
	cube.Scalex, cube.Scaley = 4096, 4096
	cube.Rotate = int32(spin)
	cube.X = int16(heroX - camX + int(b.X))
	cube.Y = int16(heroY - tall + int(sp.H)/2 - 1 + int(b.Y))
	sp.Attribute |= doff
}
