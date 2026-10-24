// A box to move around the screen in front of a turning cube, to music: the
// program to start from, which the tutorial writes.
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

// The files under assets/ are declared, ready to load, in
// zz_generated.disc.go, by discgen: cubeTIM, cubeTMD, sample1SEQ, std0VB and
// std0VH, and files, every one of them.
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// How far a held direction moves the box each frame.
const speed = 2

var (
	// The screen, 320 by 240 at the console's own frame rate, with ordering
	// tables of 1<<4 depths, enough for the cube's faces to be sorted by
	// distance.
	screen = yaroze.Screen{
		Background: yaroze.CVECTOR{R: 40, G: 60, B: 100},
		Depth:      4,
	}
	pad    yaroze.Pad
	loader disc.Loader

	box  = yaroze.GsBOXF{W: 40, H: 40, R: 255, G: 160, B: 32}
	bar  = yaroze.GsBOXF{H: 4, R: 255, G: 255, B: 255}
	cube yaroze.Object

	view  yaroze.GsRVIEW2
	light yaroze.GsF_LIGHT

	bank   yaroze.Bank
	song   yaroze.Sequence
	paused bool
)

func main() {
	screen.Open()
	// The 3D library puts the origin in the middle of the screen, for 2D
	// as well, so the text and the box are placed from the corner Bounds
	// gives.
	yaroze.GsInit3D()
	b := screen.Bounds()
	screen.Font(int32(b.X)+16, int32(b.Y)+16, 256, 64, 512)
	box.X, box.Y = b.X+24, b.Y+b.H-64
	bar.X, bar.Y = b.X+b.W/2-100, b.Y+b.H/2

	loader.Start(300, files...)
	for ot := range screen.Frames() {
		pad.Update()
		dx, dy := pad.Dir()
		box.X, box.Y = b.Clamp(box.X+int16(dx*speed), box.Y+int16(dy*speed), box.W, box.H)
		if pad.Pressed(yaroze.PADstart) {
			pauseMusic()
		}

		// The box is sorted at depth 0, in front of everything, so it
		// passes over the cube.
		box.Sort(ot, 0)
		yaroze.Print("the directions move the box\nstart pauses the music\n")

		switch {
		case !loader.Done():
			if done, failed := loader.Step(); done && !failed {
				setupCube()
				startMusic()
			}
			bar.W = uint16(200 * loader.Loaded() / len(files))
			bar.Sort(ot, 0)
		case loader.Failed() != nil:
			yaroze.Print("could not read ")
			yaroze.Print(loader.Failed().Name)
		default:
			// The cube turns with the frames, so that where it is is the
			// same at the same frame however long the disc took.
			frame := screen.Frame()
			cube.Rotation.Vx = int16(frame * 8)
			cube.Rotation.Vy = int16(frame * 20)
			cube.Draw(ot)
		}
	}
}

// setupCube puts the cube's texture into video memory, links the cube to
// the model, and sets up the camera and the light it is seen by.
func setupCube() {
	yaroze.TIM(cubeTIM.Data()).Load()
	model := yaroze.TMD(cubeTMD.Data())
	model.Map()
	cube.Init(yaroze.WORLD)
	cube.Link(model, 0)

	// The camera looks at the cube from in front, far enough back that it
	// is a small thing in the middle of the screen.
	view = yaroze.GsRVIEW2{Vpz: -1000}
	yaroze.GsSetProjection(320)
	view.Set()

	// One light from over the camera's shoulder, and some ambient light,
	// so that the faces turned away from it are not black.
	light = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	light.Set(0)
	yaroze.GsSetAmbient(yaroze.ONE/4, yaroze.ONE/4, yaroze.ONE/4)
	yaroze.GsSetLightMode(0)
}

// startMusic sends the sound bank to the SPU and plays the sequence on it
// for ever. Sound that does not load leaves the program silent rather than
// stopped.
func startMusic() {
	yaroze.SsSetMVol(127, 127)
	bank, _ = yaroze.OpenBank(std0VH.Data(), std0VB.Data())
	song, _ = bank.Sequence(sample1SEQ.Data())
	song.SetVolume(127, 127)
	song.Play(yaroze.SSPLAY_INFINITY)
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
