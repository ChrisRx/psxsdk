// Two scenes placed by scene files, cubes over a tile map and the same
// cubes stacked in a tower, drawn from one archive of textures and
// models, with a badge in the corner cut from a picture: the program the
// playground starts from.
//
// Each scene is a file under scenes/, cooked by mkscene into a DAT on the
// disc, which package scene reads and places: the camera, the projection,
// the lights and the tile map are the file's, so the program has nothing
// of its own to set up and swaps scenes by loading another file. The
// cube's model, the tiles and the badge's picture are the files under
// pack/level/, which mkarc packs into ASSETS/LEVEL.ARC, one file to read
// off the disc, and the scene draws from the archive through
// scene.NewArchive, unpacking each piece once. The badge's picture is
// assets/badge.png, which png2tim converts into the archive's directory from
// the go:generate line below, as the playground does for any picture
// dropped on it. Each frame the program turns the middle cube by
// name, draws the scene, the sprite into the overlay so that it stays in
// front, and prints the scene's name and the two fields the scene gives
// each object, numbers of the program's own. Editing a scene and
// regenerating moves everything without a change to the code.
//
// # Controls
//
// The directions turn the middle cube; Cross stops it turning on its own;
// Start swaps the scene.
package main

// The picture under assets/ becomes a TIM in the archive's directory, each
// scene under scenes/ becomes a DAT, the directory under pack/ becomes
// the archive, then the files under assets/ are declared in
// zz_generated.disc.go.
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 704,0 --clut 0,482 -o pack/level/BADGE.TIM assets/badge.png
//go:generate go run go.chrisrx.dev/psx/tools/mkscene -o assets/CUBES.DAT scenes/cubes.json
//go:generate go run go.chrisrx.dev/psx/tools/mkscene -o assets/TOWER.DAT scenes/tower.json
//go:generate go run go.chrisrx.dev/psx/tools/mkarc -o assets/LEVEL.ARC pack/level
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/arc"
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/scene"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	screen = yaroze.Screen{
		Background:   yaroze.CVECTOR{R: 40, G: 60, B: 100},
		Depth:        5,
		OverlayDepth: 1,
	}
	pad     yaroze.Pad
	loader  disc.Loader
	world   scene.Scene
	turning = true

	// Where the scenes draw from: the disc's files and the archive.
	sources scene.Sources

	// The scenes, in the order Start goes through them.
	scenes = [...]*disc.File{&cubesDAT, &towerDAT}
	which  = 0

	text = yaroze.NewBuffer(256)
)

func main() {
	screen.Open()
	yaroze.GsInit3D()
	b := screen.Bounds()
	screen.Font(int32(b.X)+16, int32(b.Y)+16, 256, 64, 512)

	loader.Start(300, files...)
	for ot := range screen.Frames() {
		pad.Update()
		if !world.Ready() {
			done, failed := loader.Step()
			switch {
			case failed:
				yaroze.Print("could not read ")
				yaroze.Print(loader.Failed().Name)
			case done:
				a, err := arc.Open(levelARC.Data())
				if err != nil {
					yaroze.Print(err.Error())
					continue
				}
				sources = scene.Sources{scene.Files(files), scene.NewArchive(a)}
				if err := world.Load(scenes[which].Data(), sources); err != nil {
					yaroze.Print(err.Error())
				}
			}
			continue
		}

		if pad.Pressed(yaroze.PADstart) {
			which = (which + 1) % len(scenes)
			if err := world.Load(scenes[which].Data(), sources); err != nil {
				yaroze.Print(err.Error())
				continue
			}
		}
		cube := world.Object("cube")
		dx, dy := pad.Dir()
		if pad.Pressed(yaroze.PADRdown) {
			turning = !turning
		}
		if turning {
			cube.Rotation.Vx += 8
			cube.Rotation.Vy += 20
		}
		cube.Rotation.Vy += int16(dx * 32)
		cube.Rotation.Vx += int16(dy * 32)
		world.Draw(ot, screen.Overlay())

		// The scene's name, then each object's name and its two fields.
		text.Reset()
		text.WriteString("the directions turn the cube\ncross stops it, start swaps the scene\n")
		text.WriteString(world.Name()).
			WriteString("\n")
		for i := range world.NObjects {
			f0, f1 := world.ObjectFields(i)
			text.WriteString(world.ObjectName(i)).
				WriteString(" ").
				WriteInt(f0).
				WriteString(",").
				WriteInt(f1).
				WriteString("\n")
		}
		text.Print()
	}
}
