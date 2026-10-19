package main

// The areas: entering one, what stands in it, and the ways between them.

// An areaEntity is one thing the area's table puts in it.
type areaEntity struct {
	kind, anim, mode, param int
	x, y, z, dir            int16
}

// areaTables are what each area starts with, by area number.
var areaTables = map[int][]areaEntity{
	0:  {{7, 1, 2, 9, 13568, 672, 3840, 2}},
	1:  {{1, -1, 0, 1, 1792, 2688, 1536, 0}, {8, -1, 0, 9, 2816, 240, 2304, 1}, {7, 1, 2, 2, 2304, 160, 1280, 1}, {14, 15, 0, 0, 1792, 80, 1792, 0}},
	2:  {{8, -1, 0, 10, 1536, 240, 2816, 0}, {6, 0, 2, 0, 5376, 100, 5376, 0}, {6, 0, 2, 0, 5376, 100, 768, 0}, {3, -1, 0, 0, 2816, 1024, 2816, 0}, {5, -1, 0, 0, 2816, 1408, 2816, 0}, {5, -1, 0, 0, 2816, 1664, 2816, 0}, {7, 1, 2, 5, 2304, 1184, 768, 0}},
	3:  {{8, -1, 0, 11, 4352, 240, 3328, 0}, {2, -1, 0, 0, 2304, 704, 1792, 0}, {1, -1, 0, 0, 1792, 2624, 3328, 0}, {12, -1, 0, 0, 2816, 64, 2816, 0}, {7, 1, 2, 0, 768, 2720, 2816, 1}, {14, 15, 0, 0, 2816, 592, 3840, 0}},
	4:  {{1, -1, 0, 3, 1792, 576, 2816, 0}, {1, -1, 0, 4, 5376, 576, 7936, 0}, {1, -1, 0, 5, 2816, 2112, 1792, 0}, {1, -1, 0, 6, 3584, 2112, 4864, 0}, {8, -1, 0, 12, 5376, 752, 9472, 3}, {17, 15, 0, 0, 3328, 2128, 5888, 0}, {13, -1, 0, 0, 256, 2816, 9984, 0}, {7, 1, 2, 6, 768, 672, 9472, 3}},
	5:  {{6, 0, 2, 0, 1792, 612, 1792, 0}, {6, 0, 2, 0, 1536, 612, 768, 0}, {6, 0, 2, 0, 2048, 612, 768, 0}, {6, 0, 2, 0, 2560, 1124, 768, 0}, {6, 0, 2, 0, 3072, 1124, 768, 0}, {6, 0, 2, 0, 3584, 1636, 768, 0}, {6, 0, 2, 0, 3840, 1636, 1024, 0}, {6, 0, 2, 0, 3840, 1636, 1536, 0}, {6, 0, 2, 0, 3840, 1636, 2048, 0}, {3, -1, 0, 0, 3840, 2304, 2304, 0}, {5, -1, 0, 0, 3776, 2688, 2240, 0}, {12, -1, 0, 2, 3840, 64, 2304, 0}},
	6:  {{5, -1, 0, 0, 13056, 128, 2816, 0}, {1, -1, 0, 7, 6400, 1608, 2816, 0}, {1, -1, 0, 8, 1280, 64, 2816, 0}, {15, 15, 0, 0, 13568, 80, 3328, 0}, {16, 15, 0, 0, 5376, 80, 2816, 0}, {16, 15, 0, 0, 2304, 80, 1280, 0}, {7, 1, 2, 7, 4352, 2720, 1792, 1}},
	7:  {{5, -1, 0, 0, 2304, 128, 3328, 0}, {5, -1, 0, 0, 2816, 128, 3840, 0}, {6, 0, 2, 0, 2304, 100, 2816, 0}, {6, 0, 2, 0, 3328, 100, 2816, 0}, {7, 1, 2, 8, 3328, 160, 768, 1}},
	8:  {{12, -1, 0, 1, 768, 1088, 7424, 0}, {2, -1, 0, 0, 768, 2240, 8448, 0}, {7, 1, 2, 10, 768, 3744, 5376, 1}, {7, 1, 2, 11, 1792, 2720, 1280, 3}, {15, 15, 0, 0, 8960, 1104, 1280, 0}, {16, 15, 0, 0, 6400, 1104, 768, 0}, {5, -1, 0, 0, 4864, 1152, 768, 0}},
	9:  {{7, 1, 2, 12, 768, 672, 1280, 0}, {16, 15, 0, 0, 8960, 80, 1792, 0}, {5, -1, 0, 0, 8448, 640, 2304, 0}},
	10: {{5, -1, 0, 0, 768, 128, 6400, 0}, {5, -1, 0, 0, 4352, 128, 5888, 0}, {8, -1, 0, 13, 3328, 240, 6400, 1}, {8, -1, 0, 14, 1280, 240, 5632, 1}, {7, 1, 2, 13, 1280, 1696, 4352, 1}, {7, 1, 2, 4, 768, 160, 1792, 0}, {13, -1, 0, 1, 2816, 256, 256, 1}},
	22: {{7, 1, 2, 1, 4352, 160, 2304, 3}, {32, 0, 0, 0, 1792, 256, 1536, 3}, {32, 0, 0, 0, 3840, 256, 1536, 3}},
	30: {{12, -1, 0, 3, 1280, 1088, 2816, 0}, {12, -1, 0, 4, 1792, 1088, 2816, 0}, {12, -1, 0, 5, 2304, 1088, 2816, 0}, {6, 0, 2, 0, 1280, 1124, 3328, 0}, {6, 0, 2, 0, 1792, 1124, 3328, 0}, {6, 0, 2, 0, 2304, 1124, 3328, 0}, {13, -1, 0, 4, 256, 1280, 2816, 0}},
}

// An areaLink joins a cell of one area to a cell of another, both ways.
type areaLink struct {
	a, x, y, z    uint8
	b, x2, y2, z2 uint8
}

var areaLinks = [17]areaLink{
	{0, 26, 1, 0, 10, 5, 0, 15}, {10, 5, 0, 0, 1, 2, 0, 11}, {1, 5, 3, 0, 2, 5, 0, 11}, {2, 5, 2, 0, 8, 2, 2, 19},
	{2, 0, 3, 5, 3, 11, 0, 6}, {2, 11, 3, 5, 30, 0, 2, 5}, {3, 0, 7, 9, 4, 11, 5, 1}, {4, 11, 1, 20, 5, 0, 1, 3},
	{4, 0, 1, 1, 9, 19, 1, 3}, {4, 0, 5, 19, 6, 31, 3, 2}, {6, 0, 7, 5, 7, 9, 4, 11}, {7, 9, 3, 2, 21, 0, 6, 1},
	{21, 7, 0, 1, 22, 0, 0, 3}, {22, 13, 2, 3, 20, 0, 3, 8}, {20, 17, 0, 8, 23, 0, 0, 5}, {23, 10, 0, 5, 24, 0, 0, 1},
	{24, 6, 7, 9, 30, 6, 6, 0},
}

var fadeT int

// gameNew starts a game over: no story, no items, ten hearts, an hour on
// the clock, and a new player.
func gameNew() {
	clear(flags[:])
	clear(switchDown[:])
	clear(doorsOpen[:])
	clear(chestsOpened[:])
	hearts = 10
	potions, keys = 0, 0
	timeLeft = 0x1a5e0
	entFree(playerEnt)
	playerEnt = entSpawn(9, 2, 0, 0, 0, 0, 0, 0)
	playerEnt.hp = 0xa0
}

// gameStart begins the game at the port, under the title.
func gameStart() {
	gameNew()
	flags[storyLeft] = 1
	enterArea(0, 0, 0, 0)
}

// enterArea puts the player in area n at x, y, z: the music for it, the
// story's scenes when they fall there, its light, its map built into a
// mesh, its things, and the camera behind the player.
func enterArea(n int, x, y, z int16) {
	area = n
	indoors := n >= 0x14 && n < 0x19
	if indoors {
		if flags[storyLost] == 0 && areaStyle != areaIndoor {
			fadeTune(tunePort)
			playTune(tuneRuins)
		}
		areaStyle = areaIndoor
		probeOn = false
	} else {
		if flags[storyLost] == 0 && areaStyle == areaIndoor {
			fadeTune(tuneRuins)
			playTune(tunePort)
		}
		areaStyle = areaOutdoor
		if flags[storyPedestal] == 1 {
			areaStyle = areaDark
		}
		probeOn = true
	}
	if flags[storyLost] == 1 {
		x, y, z = 0x200, -0x200, 0x200
		hudOn = true
		areaStyle = areaDark
		probeOn = true
		demoOnly = true
		startCameraScript(5)
	} else if n != 0 {
		switch {
		case flags[storyLeft] == 2:
			flags[storyLeft] = 0
			noclip = false
			demoOnly = false
			nextCameraKey()
			hudOn = true
		case n == 0x14 && flags[storyBossDead] == 0:
			demoOnly = true
			startCameraScript(1)
			startDemo(2)
		case n == 0x1e && x == 256:
			x = 0x300
		}
	} else if flags[storyLeft] != 1 {
		if flags[storyPedestal] == 1 {
			hudOn = false
			fadeTune(tunePort)
			setAmbient(true)
			if flags[storyScene] == 0 {
				flags[storyScene] = 1
				demoOnly = true
				startCameraScript(2)
				startDemo(3)
			} else {
				flags[storyScene] = 2
				noclip = true
				x, y, z = 0x2f00, 0x3c0, 0x3100
				if flags[storyWithBox] == 1 {
					startDialog(2, false)
					startCameraScript(3)
				} else {
					z = 0x3114
					startDialog(3, false)
					startCameraScript(4)
				}
			}
		}
	} else {
		noclip = true
		demoOnly = true
		x, y, z = 0x1b00, 0x3c0, 0x3100
		startCameraScript(0)
		titleStart()
		hudOn = false
	}
	areaLight()
	loadMap(n)
	buildWorld()
	areaEntities(n, x, y, z)
	resetView()
}

// areaEntities fills the area: the player is made again at x, y, z with
// what they were, keeping the pedestal on their head if they carried it,
// and then the area's table, the ship and the companion as the story has
// them at the port, the pedestal in its room, and the guardian in its.
func areaEntities(n int, x, y, z int16) {
	old := *playerEnt
	withPedestal := old.child != nil && old.child.flagsA&entFlagPedestal != 0
	entFreeChain(entRoot.child)
	entRoot.child, entRoot.lastChild = nil, nil
	anim := 2
	if withPedestal {
		anim = 3
	}
	p := entSpawn(9, anim, 0, 0, x, y, z, 0)
	playerEnt = p
	p.state, p.frame, p.hp = old.state, old.frame, old.hp
	p.rot = old.rot
	p.t1, p.t2, p.t3, p.t4, p.t5, p.dying, p.t7, p.t8, p.t9 = old.t1, old.t2, old.t3, old.t4, old.t5, old.dying, old.t7, old.t8, old.t9
	p.state |= entStateStill
	p.t4 = 0
	p.state &^= entStateWalk
	p.t3 = 0
	p.state &^= entStateSwing
	p.dying = 0
	p.state &^= entStateAir
	monstersLeft = 0
	for _, a := range areaTables[n] {
		entSpawn(a.kind, a.anim, a.mode, a.param, a.x, a.y, a.z, int(a.dir))
	}
	if withPedestal {
		c := entSpawn(0x14, -1, 0, 0, p.x, p.y+int16(p.height)+0x80, p.z, 0)
		entAttach(c, p)
	} else if n == 0x17 && flags[storyPedestal] == 0 {
		entSpawn(0x14, -1, 0, 0, 0xb00, 0x280, 0xb00, 0)
	}
	if n == 0 {
		switch {
		case flags[storyLeft] == 1:
			entSpawn(0x16, -1, 0, 0, 0x1900, 0x300, 0x3100, 0)
			entSpawn(0x17, 0x18, 0, 0, 0x164c, 0x3b1, 0x30d8, 0)
			entSpawn(6, 0, 2, 0, 0x1a00, 0x334, 0x3240, 0)
			entSpawn(5, -1, 0, 0, 0x1b00, 0x350, 0x3280, 0)
			entSpawn(5, -1, 0, 0, 0x1700, 0x350, 0x2e80, 0)
			entSpawn(3, -1, 0, 0, 0x2d00, 0x300, 0x2900, 0)
		case flags[storyScene] == 1:
			entSpawn(5, -1, 0, 0, 0x3100, 0x280, 0xf00, 0)
			entSpawn(5, -1, 0, 0, 0x3100, 0x380, 0xf40, 0)
			entSpawn(0x17, 0x16, 0, 0, 0x3100, 0x6e1, 0x1140, 3)
		case flags[storyScene] == 2:
			entSpawn(0x16, -1, 0, 0, 0x2d00, 0x300, 0x3100, 0)
			if flags[storyWithBox] == 1 {
				entSpawn(0x17, 0x16, 0, 0, 0x2f00, 0x3b1, 0x3270, 3)
			} else {
				entSpawn(0x17, 0x16, 0, 0, 0x2f00, 0x3b1, 0x2f32, 1)
			}
		case flags[storyLost] == 1:
			entSpawn(0x16, -1, 0, 0, 0x2d00, 0x300, 0x3100, 0)
			entSpawn(0x17, 0x16, 0, 0, 0x2a42, 0x3b1, 0x30b0, 3)
		default:
			entSpawn(1, -1, 0, 9, 0x3100, 0x240, 0x1700, 0)
		}
	}
	if n == 0x14 && flags[storyBossDead] == 0 {
		entSpawn(0x15, 0x13, 0, 0, 0x1300, 0x186, 0x1100, 2)
	}
}

// crossArea takes the player through whichever link they stand at, or
// back to the port when the day is lost or a scene calls them there.
func crossArea() {
	if flags[storyLost] == 1 || flags[storyScene] != 0 {
		enterArea(0, 1, 1, 1)
		return
	}
	cx := floor512(int(playerEnt.x))
	cz := floor512(int(playerEnt.z))
	for i := range areaLinks {
		l := &areaLinks[i]
		if area == int(l.a) && cx == int(l.x) && cz == int(l.z) {
			enterArea(int(l.b), int16(l.x2)*0x200+0x100, int16(l.y2)*0x200+0xf0, int16(l.z2)*0x200+0x100)
			return
		}
		if area == int(l.b) && cx == int(l.x2) && cz == int(l.z2) {
			enterArea(int(l.a), int16(l.x)*0x200+0x100, int16(l.y)*0x200+0xf0, int16(l.z)*0x200+0x100)
			return
		}
	}
}

// fadeFrame fades the picture to white and back while the player crosses
// from one area to another, the world standing still, and reports when
// the crossing is to happen, at the whitest.
func fadeFrame() bool {
	if playerEnt.frame&entFrameAtEdge == 0 {
		return false
	}
	over := screen.Overlay()
	cross := false
	var c int
	switch {
	case fadeT < 0x10:
		entRoot.frame &^= entFrameActive
		c = min(fadeT*16+16, 0xff)
	case fadeT == 0x10:
		entRoot.frame &^= entFrameActive
		c = 0xff
		cross = true
	default:
		if fadeT > 0x1f {
			// The fade is over: the player is free again, and nothing
			// is drawn this frame.
			playerEnt.frame &^= entFrameAtEdge
			fadeT = 0
			return false
		}
		if fadeT == 0x11 {
			entRoot.frame |= entFrameActive
		}
		c = max((fadeT-0x10)*-0x10+0xff, 0)
	}
	fillRect(over.Entry(0), -160, -120, 320, 240, uint8(c), uint8(c), uint8(c), true, 1)
	fadeT++
	return cross
}
