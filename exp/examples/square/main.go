// A box to move around the screen in front of a turning cube, to music,
// on the exp/ps API: examples/square, the program the tutorial writes,
// against the Go library's own surface. It draws the same frames, which
// the tests check.
//
// It is the smallest program that has the parts most programs need: the
// frame loop, the controller, files read off the disc, a model drawn in
// 3D, and music. The music and the cube are read off the disc while the
// frames go on, with a bar for how far the reading has got; if the disc
// cannot be read, the program shows the box alone and says which file it
// could not read.
//
// # Controls
//
// The directions move the box. Start pauses the music and starts it again.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/exp/ps"
)

// How far a held direction moves the box each frame.
const speed = 2

// The files under the original example's assets, placed on the disc
// under ASSETS.
var (
	cubeTIM    = disc.Register("ASSETS/CUBE.TIM", "../../../examples/square/assets/CUBE.TIM")
	cubeTMD    = disc.Register("ASSETS/CUBE.TMD", "../../../examples/square/assets/CUBE.TMD")
	sample1SEQ = disc.Register("ASSETS/SAMPLE1.SEQ", "../../../examples/square/assets/SAMPLE1.SEQ")
	std0VB     = disc.Register("ASSETS/STD0.VB", "../../../examples/square/assets/STD0.VB")
	std0VH     = disc.Register("ASSETS/STD0.VH", "../../../examples/square/assets/STD0.VH")
)

var (
	// The screen, 320 by 240 at the console's own frame rate, with its
	// origin in the middle for the cube, and frames of 1<<4 depths,
	// enough for the cube's faces to be sorted by distance.
	screen = ps.Screen{
		Background: ps.Color{R: 40, G: 60, B: 100},
		Depth:      4,
		Center:     true,
	}
	text   ps.Text
	pad    ps.Controller
	loader disc.Loader

	box  = ps.Box{W: 40, H: 40, Color: ps.Color{R: 255, G: 160, B: 32}}
	bar  = ps.Box{H: 4, Color: ps.Color{R: 255, G: 255, B: 255}}
	cube ps.Object

	bank   ps.Bank
	song   ps.Sequence
	paused bool
)

func main() {
	screen.Open()
	// The origin is in the middle of the screen, so the text and the box
	// are placed from the corner Bounds gives.
	b := screen.Bounds()
	text = screen.Font(b.X+16, b.Y+16, 256, 64, 512)
	box.MoveTo(b.X+24, b.Y+b.H-64)
	bar.MoveTo(b.X+b.W/2-100, b.Y+b.H/2)

	loader.Start(300)
	for frame := range screen.Frames() {
		pad.Update()
		dx, dy := pad.Dir()
		r := box.Rect()
		box.MoveTo(b.Clamp(r.X+dx*speed, r.Y+dy*speed, r.W, r.H))
		if pad.Pressed(ps.Start) {
			pauseMusic()
		}

		// The box is drawn at depth 0, in front of everything, so it
		// passes over the cube.
		box.Draw(frame, 0)
		text.Print("the directions move the box\nstart pauses the music\n")

		switch {
		case !loader.Done():
			if done, failed := loader.Step(); done && !failed {
				setupCube()
				startMusic()
			}
			bar.W = uint16(200 * loader.Loaded() / loader.Total())
			bar.Draw(frame, 0)
		case loader.Failed() != nil:
			text.Print("could not read ").Print(loader.Failed().Name)
		default:
			// The cube turns with the frames, so that where it is is the
			// same at the same frame however long the disc took.
			n := frame.Number()
			cube.Rotation.X = n * 8
			cube.Rotation.Y = n * 20
			cube.Draw(frame)
		}
	}
}

// setupCube puts the cube's texture into video memory, links the cube to
// the model, and sets up the camera and the light it is seen by.
func setupCube() {
	ps.LoadTIM(cubeTIM.Data())
	model := ps.LoadTMD(cubeTMD.Data())
	cube.Init(nil)
	cube.Link(model, 0)

	// The camera looks at the cube from in front, far enough back that it
	// is a small thing in the middle of the screen.
	camera := ps.Camera{Eye: ps.Vector{Z: -1000}, Distance: 320}
	camera.Set()

	// One light from over the camera's shoulder, and some ambient light,
	// so that the faces turned away from it are not black.
	ps.SetLight(0, ps.Light{Direction: ps.Vector{X: 1, Y: 1, Z: 1}, Color: ps.Color{R: 0xc0, G: 0xc0, B: 0xc0}})
	ps.SetAmbient(ps.One/4, ps.One/4, ps.One/4)
}

// startMusic sends the sound bank to the sound processor and plays the
// sequence on it for ever. Sound that does not load leaves the program
// silent rather than stopped.
func startMusic() {
	ps.SetVolume(127, 127)
	bank, _ = ps.OpenBank(std0VH.Data(), std0VB.Data())
	song, _ = bank.Sequence(sample1SEQ.Data())
	song.SetVolume(127, 127)
	song.Loop()
}

// pauseMusic pauses the music, or starts it again where it was paused.
func pauseMusic() {
	if paused {
		song.Resume()
	} else {
		song.Pause()
	}
	paused = !paused
}
