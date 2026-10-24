// Two rooms seen from above at a slant, a stone hall and a meadow, walked
// around by Nora, a figure drawn flat over them, and joined by a doorway,
// as Dragon Quest VII's towns were: the start of a role playing game.
//
// Each room is a scene file in scenes/, cooked by mkscene onto the disc
// and read with package scene: a map of cells, a tile each, walls, floor,
// a doorway and a house; links from the doorway to the room it leads to;
// and markers, the cells the chest and the villager stand on, which
// rooms.go gives their lines by name. Package stage draws the room from
// the map every frame, a textured quad for each floor cell, for each side
// of a wall cell that borders floor, two tiles high, and for the wall's
// top, each put through the GTE and built as a GPU packet, which is what
// the library's own model drawing does for a model; drawing from the map
// means a room is a few lines of text, and only the cells near enough to
// be on the screen are drawn, so a room can be as large as it likes. A
// face is drawn only when it faces the camera, so the walls nearest the
// camera, whose faces look away from it, are not drawn and the room is
// seen into from whichever side the camera is on. The walls nearer the
// camera than the hero have no tops, and one that stands between the
// camera and the hero is drawn half see-through, so that the hero shows
// through it as Dragon Quest's did. Both rooms are flat, but a cell given
// a height in the playground's room editor lifts the ground through the
// corners of the cells round it, and the hero and the camera follow it.
// The camera (camera.go) looks down at the hero from a fixed slant and
// turns about them an eighth of a turn at a time, taking eight frames
// over each turn.
//
// The house is a block of cells two rows deep whose sides are drawn as
// log walls and whose top is a roof of two slopes meeting at a ridge
// between the rows, with gables at the block's ends.
//
// Nora, the hero (hero.go), is a sprite, placed each frame where their
// feet project to on the screen and sorted at that depth, a little in
// front of the floor under them. The sprite sheet has a column of frames
// for each of the eight ways the hero can face, the figure seen from
// behind, the side or the front, and a row for each frame of walking;
// which column is drawn is the way the hero faces in the world turned by
// the camera. Walking is relative to the camera too: up is away from it,
// whichever way it has turned. A step is tried a map cell at a time: the
// corners of the hero's footprint must all be on floor, and the ground
// must not rise or fall too steeply for the step; each axis is tried on
// its own, so that a step into a wall slides along it. Stepping onto the
// doorway puts the hero in the other room, just inside its doorway, after
// a few black frames.
//
// Cross speaks to whoever Nora faces, or opens the chest in front of
// them, and what is said comes up in a text box along the bottom of the
// screen as Dragon Quest's did, a letter at a time, three lines at a
// time, with an arrow for more (talk.go).
//
// The textures and the sprite sheets are written by gen, which stands in
// for an artist: hand-made pictures of the same names and sizes take
// their place.
//
// The program is written as Go rather than as C would have it: the game's
// numbers are plain ints, since the console's word is 32 bits and int32
// compiles to the same code, with the library's own int16 and int32 kept
// to its structs; the camera, the hero, the room's setting and the text
// box are types with methods, each one a package variable, so that
// nothing allocates in the frame loop; and there are no interfaces, since
// nothing here needs to be one of several kinds at run time.
//
// # Controls
//
// The directions walk, relative to the camera: up is away from it. L1 and
// R1 turn the camera an eighth of a turn each way. Cross speaks to the
// person in front of Nora, or opens the chest there, and reads on through
// what is said.
package main

// The art: gen writes the sources, png2tim converts the textures and the
// sprite sheets, and discgen declares the files on the disc. Video
// memory: the display buffers take x 0-319; the floor, the wall, the
// grass and the chest share the top of the texture page at x 320 (4-bit,
// so 64 pixels is 16 units), the hero's 256x192 sheet fills most of the
// page at x 384, the logs and the roof share the page at x 448, the
// villager's sheet the page at x 512, and the letters start the page at
// x 576. The CLUTs take a row each from y 480.
//go:generate go run ./gen assets
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 320,0 --clut 0,480 -o assets/FLOOR.TIM assets/floor.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 336,0 --clut 0,481 --semi -o assets/WALL.TIM assets/wall.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 352,0 --clut 0,482 -o assets/GRASS.TIM assets/grass.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 368,0 --clut 0,483 -o assets/CHEST.TIM assets/chest.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 384,0 --clut 0,484 -o assets/HERO.TIM assets/hero.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 448,0 --clut 0,485 --semi -o assets/LOG.TIM assets/log.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 464,0 --clut 0,486 --semi -o assets/ROOF.TIM assets/roof.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 512,0 --clut 0,487 -o assets/NPC.TIM assets/npc.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 576,0 --clut 0,488 -o assets/FONT.TIM assets/font.png
//go:generate go run go.chrisrx.dev/psx/tools/mkscene -o assets/HALL.DAT scenes/hall.json
//go:generate go run go.chrisrx.dev/psx/tools/mkscene -o assets/MEADOW.DAT scenes/meadow.json
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// A frame: the controller, then the world moved a step, or two when the
// last frame took two fields so that the hero's speed holds whatever is
// on the screen, then the room and everything in it drawn.
const (
	// blankTime is the frames the screen stays black for after a
	// doorway.
	blankTime = 12
)

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 8, G: 8, B: 16}}
	pad    yaroze.Pad
	failed *disc.File

	// otTags are the two ordering tables, as deep as package stage sorts
	// a room into, and packets both frames' GPU packets, more than the
	// screen's own for a room's polygons.
	otTags  [2 << stage.Depth]yaroze.GsOT_TAG
	packets [32 << 10]uint32

	// blank is the frames the screen has still to stay black, and black
	// the box that blacks it.
	blank int
	black yaroze.GsBOXF

	// set is the room the hero is in, as package stage draws it, and
	// view where it is drawn from this frame.
	set  stage.Setting
	view stage.View

	// The room textures the maps pick from by index.
	floors [2]stage.Surface // stone, grass
	walls  [2]stage.Surface // stone, logs
)

func main() {
	screen.Init(otTags[:], packets[:])
	screen.Open()
	yaroze.GsInit3D()
	b := screen.Bounds()
	screen.Font(int32(b.X)+8, int32(b.Y)+8, 200, 32, 64)
	black = yaroze.GsBOXF{X: b.X, Y: b.Y, W: uint16(b.W), H: uint16(b.H)}

	for k := range rooms {
		*rooms[k].file = rooms[k].file.Into(roomData[k][:])
	}
	if failed = disc.LoadAll(300, files...); failed == nil {
		setup(b)
		nora.here().load()
		screen.Background = nora.here().background()
		cam.slant(world.Room())
	}
	for ot := range screen.Frames() {
		if failed != nil {
			yaroze.Print("could not read ")
			yaroze.Print(failed.Name)
			continue
		}
		pad.Update()
		// The playground's live view: a room painted in the editor is
		// written over the file and placed again, and Play from here
		// puts Nora on a cell, in the other room when it says so.
		if world.Reload() {
			nora.here().place()
		}
		if scene, i, j, ok := world.Teleport(); ok {
			nora.visit(scene, i, j)
		}
		// A frame that took two fields moves everything twice, so that
		// the hero's speed holds whatever is on the screen. Through the
		// black frames after a doorway nothing moves: the frame that
		// loaded the room took longer, and the hero sets off from just
		// inside the doorway when the room appears.
		steps := min(screen.Fields(), 2)
		if blank > 0 {
			steps = 0
		}
		for k := range steps {
			// A press counts once, on the first step. While the text box
			// is up, Cross reads on and nothing else moves.
			cross := k == 0 && pad.Pressed(yaroze.PADRdown)
			if talk.up {
				talk.update(cross)
			} else {
				nora.walk()
				if cross {
					nora.interact()
				}
			}
			cam.update(k == 0 && !talk.up)
		}
		cam.place(nora.at(), nora.height)
		nora.project()
		view = stage.View{At: nora.at(), Ahead: cam.ahead, Right: cam.right, X: nora.sx, Y: nora.sy, Depth: nora.otz}
		set.Draw(ot, &view)
		nora.draw(ot)
		drawNPCs(ot)
		if blank > 0 {
			blank--
			black.Sort(ot, 0)
		}
		talk.draw(ot)
	}
}

// setup loads the textures and the sprite sheets into video memory,
// keeps what the polygons name the textures by, cuts the sprites, sets up
// the text box, and sets the projection.
func setup(b yaroze.RECT) {
	floors[floorStone] = stage.NewSurface(yaroze.TIM(floorTIM.Data()).Load().Texture())
	floors[floorGrass] = stage.NewSurface(yaroze.TIM(grassTIM.Data()).Load().Texture())
	walls[wallStone] = stage.NewSurface(yaroze.TIM(wallTIM.Data()).Load().Texture())
	walls[wallLog] = stage.NewSurface(yaroze.TIM(logTIM.Data()).Load().Texture())
	set.Art = stage.Art{
		Floors: floors[:],
		Walls:  walls[:],
		Logs:   walls[wallLog],
		Roof:   stage.NewSurface(yaroze.TIM(roofTIM.Data()).Load().Texture()),
		Chest:  yaroze.TIM(chestTIM.Data()).Load().Texture(),
	}
	people.cut(yaroze.TIM(npcTIM.Data()).Load())
	nora.cut(yaroze.TIM(heroTIM.Data()).Load())
	talk.setup(b)

	yaroze.GsSetProjection(projection)
	cam.place(nora.at(), nora.height)
}
