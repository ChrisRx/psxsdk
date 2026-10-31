// A stone hall, a meadow and a house seen from above at a slant, walked
// around by Hero, a figure drawn flat over them, and joined by doorways,
// as Dragon Quest VII's towns were: the start of a role playing game.
//
// examples/quest on the exp/ps API, through exp/ps/scene and
// exp/ps/stage: the rooms are the same scene files, the setting draws
// them the same way through the camera, and the hero, the people, the
// sky and the text box are the API's sprites, boxes and packets.
//
// # Controls
//
// The directions walk, relative to the camera: up is away from it. L1 and
// R1 turn the camera an eighth of a turn each way. Cross speaks to the
// person in front of Hero, or opens the chest there, and reads on through
// what is said.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/exp/ps"
	"go.chrisrx.dev/psx/exp/ps/stage"
)

// The art, as examples/quest has it on the disc.
const assets = "../../../examples/quest/assets/"

var (
	bedTIM    = disc.Register("ASSETS/BED.TIM", assets+"BED.TIM")
	chestTIM  = disc.Register("ASSETS/CHEST.TIM", assets+"CHEST.TIM")
	floorTIM  = disc.Register("ASSETS/FLOOR.TIM", assets+"FLOOR.TIM")
	fontTIM   = disc.Register("ASSETS/FONT.TIM", assets+"FONT.TIM")
	grassTIM  = disc.Register("ASSETS/GRASS.TIM", assets+"GRASS.TIM")
	hallDAT   = disc.Register("ASSETS/HALL.DAT", assets+"HALL.DAT")
	heroTIM   = disc.Register("ASSETS/HERO.TIM", assets+"HERO.TIM")
	houseDAT  = disc.Register("ASSETS/HOUSE.DAT", assets+"HOUSE.DAT")
	logTIM    = disc.Register("ASSETS/LOG.TIM", assets+"LOG.TIM")
	meadowDAT = disc.Register("ASSETS/MEADOW.DAT", assets+"MEADOW.DAT")
	npcTIM    = disc.Register("ASSETS/NPC.TIM", assets+"NPC.TIM")
	roofTIM   = disc.Register("ASSETS/ROOF.TIM", assets+"ROOF.TIM")
	skyTIM    = disc.Register("ASSETS/SKY.TIM", assets+"SKY.TIM")
	sunTIM    = disc.Register("ASSETS/SUN.TIM", assets+"SUN.TIM")
	wallTIM   = disc.Register("ASSETS/WALL.TIM", assets+"WALL.TIM")
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
	// The screen sorts as deep as package stage sorts a room, with more
	// packet space than the screen's own for a room's polygons.
	screen = ps.Screen{
		Background: ps.Color{R: 8, G: 8, B: 16},
		Depth:      stage.Depth,
		Packets:    32 << 10,
		Center:     true,
	}
	pad    ps.Controller
	failed *disc.File
	text   ps.Text

	// blank is the frames the screen has still to stay black, and black
	// the box that blacks it.
	blank int
	black ps.Box

	// set is the room the hero is in, as package stage draws it, and
	// view where it is drawn from this frame.
	set  stage.Setting
	view stage.View

	// The room textures the maps pick from by index.
	floors [2]stage.Surface // stone, grass
	walls  [2]stage.Surface // stone, logs
)

func main() {
	screen.Open()
	b := screen.Bounds()
	text = screen.Font(b.X+8, b.Y+8, 200, 32, 64)
	black.SetRect(b)

	for k := range rooms {
		*rooms[k].file = rooms[k].file.Into(roomData[k][:])
	}
	if failed = disc.LoadAll(300); failed == nil {
		setup(b)
		hero.here().load()
		screen.Background = hero.here().background()
		cam.slant(world.Room())
	}
	for frame := range screen.Frames() {
		if failed != nil {
			text.Print("could not read ").Print(failed.Name)
			continue
		}
		pad.Update()
		// The playground's live view: a room painted in the editor is
		// written over the file and placed again, and Play from here
		// puts Hero on a cell, in another room when it says so.
		if world.Reload() {
			hero.here().place()
		}
		if scene, i, j, ok := world.Teleport(); ok {
			hero.visit(scene, i, j)
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
			cross := k == 0 && pad.Pressed(ps.Cross)
			if talk.up {
				talk.update(cross)
			} else {
				hero.walk()
				if cross {
					hero.interact()
				}
			}
			cam.update(k == 0 && !talk.up)
		}
		cam.place(hero.at(), hero.height)
		hero.project()
		view = stage.View{At: hero.at(), Ahead: cam.ahead, Right: cam.right, X: hero.sx, Y: hero.sy, Depth: hero.otz}
		set.Draw(frame, &view)
		hero.draw(frame)
		drawNPCs(frame)
		sky.draw(frame, b)
		if blank > 0 {
			blank--
			black.Draw(frame, 0)
		}
		talk.draw(frame)
	}
}

// setup loads the textures and the sprite sheets into video memory,
// keeps what the polygons name the textures by, cuts the sprites, sets up
// the sky and the text box, and places the camera.
func setup(b ps.Rect) {
	floors[floorStone] = stage.NewSurface(ps.LoadTIM(floorTIM.Data()).Texture())
	floors[floorGrass] = stage.NewSurface(ps.LoadTIM(grassTIM.Data()).Texture())
	walls[wallStone] = stage.NewSurface(ps.LoadTIM(wallTIM.Data()).Texture())
	walls[wallLog] = stage.NewSurface(ps.LoadTIM(logTIM.Data()).Texture())
	set.Art = stage.Art{
		Floors: floors[:],
		Walls:  walls[:],
		Logs:   walls[wallLog],
		Roof:   stage.NewSurface(ps.LoadTIM(roofTIM.Data()).Texture()),
		Bed:    ps.LoadTIM(bedTIM.Data()).Texture(),
		Chest:  ps.LoadTIM(chestTIM.Data()).Texture(),
	}
	people.cut(ps.LoadTIM(npcTIM.Data()))
	hero.cut(ps.LoadTIM(heroTIM.Data()))
	sky.setup(b)
	talk.setup(b)
	cam.place(hero.at(), hero.height)
}
