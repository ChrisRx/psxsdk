package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/scene"
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// A room is a scene file in scenes/, cooked by mkscene onto the disc: a
// map of cells for package stage to draw, in its legend ('#' a wall, '.'
// floor, 'c' a house's wall, 't' a treasure chest and a digit a
// doorway), the look round it, its links, which say which room each
// doorway's digit leads to, and its markers, cells named for the people
// and things that stand on them. What they say is here, by the markers'
// names. The hall's doorway in its near side leads to the meadow's in
// its far side, so that the hero comes out of it walking the way they
// went in and a held direction carries them on into the meadow.
type room struct {
	// Map is the room's map, once it is loaded, which world holds.
	*stage.Map
	name string
	file *disc.File

	// npcs are the people standing in the room and chests its treasure
	// chests, one for each 't' cell, each on the cell of the marker with
	// its name.
	npcs   []npc
	chests []chest
}

// An npc is a person standing on a cell, facing a way in eighths of a
// turn from +Z clockwise, the marker's first field, who turns to face
// Hero when they come near, and says their line when spoken to.
type npc struct {
	name   string
	i, j   int
	facing int
	say    string
}

// A chest is a treasure chest on a cell, with what is said when Hero opens
// it, and whether it has been opened.
type chest struct {
	name   string
	i, j   int
	text   string
	opened bool
}

// The rooms' floors and walls, as the room files index the setting's art.
const (
	floorStone = iota
	floorGrass
)

const (
	wallStone = iota
	wallLog
)

// world is the room's scene file as loaded, and roomData the memory each
// room's file is read into, with room to grow, so that the playground
// can write a room painted larger over the one the program loaded.
var (
	world    scene.Scene
	roomData [2][4096]uint32
)

var rooms = [...]room{
	{
		// The hall: a big room of stone, with the chest in its far right
		// corner and the way out in its near wall.
		name:   "hall",
		file:   &hallDAT,
		chests: []chest{{name: "chest", text: "{Player Name} opens the chest.\nInside she found a Copper Sword. She puts it away in her bag."}},
	},
	{
		// The meadow: open grass, flat all over, with a house of logs
		// and the villager who lives in it standing at its corner.
		name: "meadow",
		file: &meadowDAT,
		npcs: []npc{{name: "villager", say: "Welcome to the meadow, traveler! That house is mine. The hall behind you is older than anyone can remember."}},
	},
}

// load reads the room's file into world and places it; the files are
// given so that the scene knows its name, which the playground's live
// view goes by.
func (r *room) load() {
	if err := world.Load(r.file.Data(), scene.Files(files)); err != nil {
		failed = r.file
		return
	}
	r.place()
}

// place makes the room ready to draw, with its chests as they were left,
// and puts its people on their markers' cells; again, when the
// playground has written a new map over the file.
func (r *room) place() {
	r.Map = &world.Room().Map
	set.Load(r.Map)
	for k := range r.npcs {
		n := &r.npcs[k]
		if m := world.Marker(n.name); m != nil {
			n.i, n.j, n.facing = m.I, m.J, int(m.Fields[0])
		}
	}
	for k := range r.chests {
		c := &r.chests[k]
		if m := world.Marker(c.name); m != nil {
			c.i, c.j = m.I, m.J
		}
		if c.opened {
			set.Open(c.i, c.j)
		}
	}
}

// background is the color the screen clears to in the room.
func (r *room) background() yaroze.CVECTOR {
	bg := world.Room().Background
	if bg == (stage.Color{}) {
		return yaroze.CVECTOR{R: 8, G: 8, B: 16}
	}
	return yaroze.CVECTOR{R: bg[0], G: bg[1], B: bg[2]}
}
