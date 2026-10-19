// Terra Incognita, in Go.
//
// A port of Team Fatal's Net Yaroze game "Terra Incognita" (M. Kamiyama,
// S. Tajima, K. Tachi, 1996-97), the international version from Sony's
// Net Yaroze showcase: an action adventure in a small 3D world. Two
// treasure hunters sail to an island where, rumor has it, a golden
// pedestal lies in ancient ruins guarded by a monster. One of them is too
// scared to leave the ship, so the other sets off alone, with the day's
// light to find it in: through the island's areas, over its hills and
// water by crates, moving platforms and slime balls, past its pots,
// bombs, switches and locked doors, fighting its monsters with a sword
// and keeping hearts up with the potions chests hold, to the ruins, the
// guardian and the pedestal, and back to the port before sunset, where
// how the story ends depends on whether the pedestal came too, and on
// whether the day ran out. The program was reconstructed from the
// executable, as no source survives: the island, its creatures and its
// scenes are the data files the original shipped with, read off the disc
// and decoded here, and everything that moves them is Go.
//
// The directional buttons walk, relative to the camera; Cross jumps;
// Square swings the sword, lifts and throws what can be lifted, and reads
// the signs; Circle drinks a potion; L1 and R1 turn the camera and L2 and
// R2 bring it closer or farther. On the title, left and right choose
// Japanese or English and Cross or Start begins; Cross or Square takes a
// conversation on. Start and Select together quit the program, which the
// original, with nowhere to quit to, could not.
//
// Differences from the original:
//
//   - The three archives and the sound files are read off the disc into
//     the program's own memory and unpacked there, rather than loaded into
//     fixed addresses over the serial cable by the Net Yaroze tools. The
//     unpacking and the texture, model and animation decoding is this
//     program's; the formats are the original's.
//   - An ordering table depth past the end of the table is held to the
//     last entry, where the original let it run on past the table.
//   - An animation track is padded with one zero key, and a spline key
//     past the end of a track reads as zero, where the original read
//     whatever followed the track in memory.
//   - An actor whose entity is freed is kept for the next of its model,
//     since the program's allocator never gives memory back; the original
//     freed and allocated them.
//   - The GTE is driven through the yaroze package's inline functions
//     rather than the SDK's macros, and the polygons are built the same way
//     the original built them, into packets of its own.
//
// The pictures match the original's frame for frame in the emulator up to
// a vertical blank, and the story's flags, the areas entered and the
// player's position came out the same through tens of thousands of frames
// of scripted play: the port and the quarrel, random walks and fights
// through the island's areas, the ruins, the pedestal's lifting, the
// guardian, the returns with and without the pedestal, the day running
// out, and the endings. The port runs at thirty frames a second as the
// original did, with the disc build optimizing for speed.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// The files on the disc, and the memory they are read into. The console
// has two megabytes and the program's allocator never gives anything
// back, so the big transient files take fixed buffers that are used
// again, as the original used its data area: the sound bank's body is
// read over the second and third archives once everything in them has
// been unpacked.
const (
	arc2Words = (234204 + 2047) / 2048 * 512
	arc3Words = (108108 + 2047) / 2048 * 512
	vbWords   = (401360 + 2047) / 2048 * 512
)

var (
	arc1Buf [(2620 + 2047) / 2048 * 512]uint32
	arcBuf  [vbWords]uint32
	vhBuf   [(60448 + 2047) / 2048 * 512]uint32
	gogoBuf [(17341 + 2047) / 2048 * 512]uint32
	musiBuf [(39616 + 2047) / 2048 * 512]uint32

	data1ARC     = disc.Register("DATA/DATA1.ARC", "data/DATA1.ARC").Into(arc1Buf[:])
	data2ARC     = disc.Register("DATA/DATA2.ARC", "data/DATA2.ARC").Into(arcBuf[:arc2Words])
	data3ARC     = disc.Register("DATA/DATA3.ARC", "data/DATA3.ARC").Into(arcBuf[arc2Words : arc2Words+arc3Words])
	soundStd0VH  = disc.Register("DATA/SOUND/STD0.VH", "data/SOUND/STD0.VH").Into(vhBuf[:])
	soundStd0VB  = disc.Register("DATA/SOUND/STD0.VB", "data/SOUND/STD0.VB").Into(arcBuf[:])
	soundGogoSEQ = disc.Register("DATA/SOUND/GOGO.SEQ", "data/SOUND/GOGO.SEQ").Into(gogoBuf[:])
	soundMusiSEQ = disc.Register("DATA/SOUND/MUSI.SEQ", "data/SOUND/MUSI.SEQ").Into(musiBuf[:])

	// files is what is read at the start; the bank's body comes after.
	files = []*disc.File{&data1ARC, &data2ARC, &data3ARC, &soundStd0VH, &soundGogoSEQ, &soundMusiSEQ}
)

var (
	// The screen as the original set it up: 320 by 240 at thirty frames a
	// second, the two buffers side by side, an ordering table of 4096
	// depths for the world and one of four for what goes over it, and
	// 104000 bytes of packets a frame.
	screen = yaroze.Screen{
		Width:        320,
		Height:       240,
		Depth:        12,
		OverlayDepth: 2,
		Packets:      0x19640,
		Interval:     2,
		SideBySide:   true,
	}
	pad yaroze.Pad

	arc1, arc2, arc3 *archive

	// The texture tables: the world's tiles, the things in it, the sun
	// and its flares, and the panels and lettering.
	tiles, things, panels []texture

	light yaroze.GsF_LIGHT

	// Game flags, as the original kept them by number.
	flags [14]uint8
)

// The story's flags, by the numbers the original gave them.
const (
	storyBossMet  = 0  // the guardian has spoken
	storyBossDead = 1  // the guardian is dead
	storyPedestal = 2  // the pedestal has been lifted from its room
	storyLeft     = 3  // the player is at the port: 1 setting out, 2 come back
	storyScene    = 4  // the scene at the port: 1 the quarrel, 2 the return
	storyLost     = 5  // the day ran out
	storyWithBox  = 7  // the player came back carrying the pedestal
	storyEnding   = 8  // the ship is sailing away
	storyStep     = 9  // the quarrel's progress, advanced by its lines
	storyWait     = 10 // the companion is done with a step and the next line may go
	storyShipT    = 11
	storyStep2    = 12 // the return's progress, advanced by its lines
	flagStart     = 13 // the game is to start from the opening
)

func main() {
	screen.Open()
	env := yaroze.GsDISPENV()
	env.Screen.Y, env.Screen.H = 0, 240
	clearVRAM()
	yaroze.GsInit3D()
	probeInit()

	if failed := disc.LoadAll(300, files...); failed != nil {
		for range screen.Frames() {
		}
	}
	arc1 = parseArchive(data1ARC.Data())
	arc2 = parseArchive(data2ARC.Data())
	arc3 = parseArchive(data3ARC.Data())
	// The textures are unpacked one at a time into what the buffer has
	// left after the archives.
	scratch := arcBuf[arc2Words+arc3Words:]
	tiles = loadTextures(arc2, 0, scratch)
	things = loadTextures(arc2, 1, scratch)
	sunTextures = loadTextures(arc2, 2, scratch)
	panels = loadTextures(arc2, 3, scratch)
	loadModels(arc2)
	entitiesInit()

	yaroze.GsSetProjection(0x180)
	camZoom = 1
	yaroze.GsSetAmbient(0x555, 0x555, 0x555)
	flags[flagStart] = 1
	soundInit()

	for ot := range screen.Frames() {
		padUpdate()
		pad.Update()
		gameFrame(ot)
		soundFrame()
		probeFrame(ot)
		if padHeldRaw()&0x900 == 0x900 {
			break
		}
	}
}

// gameFrame is a frame of the game: the opening while it runs, and
// otherwise the world and everything in it, and over either the scenes
// and conversations.
func gameFrame(ot *yaroze.GsOT) {
	if flags[flagStart] == 1 {
		flags[flagStart] = 0
		introStart()
	}
	if introOn {
		introFrame(ot)
	} else {
		drawSky(ot)
		cameraFrame()
		if probeOn && areaStyle != areaIndoor {
			drawSun(ot, screen.Overlay())
		}
		drawWorld(ot)
		animateTextures()
		entFrame()
	}
	dialogFrame()
}

// entFrame runs everything in the area a frame: the entities, the
// animated sprite, the panel, and the fade between areas, crossing at its
// whitest.
func entFrame() {
	entUpdateAll()
	thingsAnimate()
	if hudOn {
		hudFrame()
	}
	if fadeFrame() {
		crossArea()
	}
}

// thingsAnimate cycles the animated sprite's texture through its four
// frames, one every four frames.
var thingsT int

func thingsAnimate() {
	copyTexture(&things[64+(thingsT>>2&3)], &things[63], true)
	thingsT = (thingsT + 1) & 0xf
}

// Sky colors by area style: top and bottom of the gradient behind the
// world.
var skyColors = [3][2][3]uint8{
	{{0x20, 0x40, 0x80}, {0x80, 0xa0, 0xc0}},
	{{0x50, 0x40, 0x20}, {0, 0, 0}},
	{{0xc0, 0x40, 0x30}, {0xff, 0x80, 0x50}},
}

// drawSky sorts the gradient behind everything.
func drawSky(ot *yaroze.GsOT) {
	top, bottom := &skyColors[areaStyle][0], &skyColors[areaStyle][1]
	p := yaroze.AllocPacket(9)
	p[0] = 8 << 24
	p[1] = 0x38<<24 | uint32(top[2])<<16 | uint32(top[1])<<8 | uint32(top[0])
	p[2] = xy(-160, -120)
	p[3] = uint32(top[2])<<16 | uint32(top[1])<<8 | uint32(top[0])
	p[4] = xy(160, -120)
	p[5] = uint32(bottom[2])<<16 | uint32(bottom[1])<<8 | uint32(bottom[0])
	p[6] = xy(-160, 120)
	p[7] = uint32(bottom[2])<<16 | uint32(bottom[1])<<8 | uint32(bottom[0])
	p[8] = xy(160, 120)
	ot.Link(4095, p)
}

// clearVRAM blacks out all of video memory, as the original did at the
// start.
func clearVRAM() {
	var r yaroze.RECT
	r.W, r.H = 64, 64
	for r.X = 0; r.X < 1024; r.X += 64 {
		for r.Y = 0; r.Y < 512; r.Y += 64 {
			r.Clear(0, 0, 0)
		}
	}
	yaroze.DrawSync(0)
}

// areaLight sets the light the area is lit by: from the side in the ruins,
// warm and low at night, and white by day.
func areaLight() {
	light = yaroze.GsF_LIGHT{Vx: 4, Vz: 5}
	switch {
	case areaStyle == areaIndoor:
		light.Vy = 4
		light.R, light.G, light.B = 0xe0, 0xe0, 0x78
	case areaStyle != areaDark:
		light.Vy = 5
		light.R, light.G, light.B = 0xe0, 0xe0, 0xe0
	default:
		light.Vy = 2
		light.R, light.G, light.B = 0xf0, 0xa0, 0xa0
	}
	light.Set(0)
	yaroze.GsSetAmbient(0x555, 0x555, 0x555)
}

// The opening: the logo under sail, until the picture whites out and the
// game begins at the port.
var (
	introOn   bool
	introShip *actor
)

// introStart sets the logo flying under the opening's light and camera.
func introStart() {
	sceneT = 0
	introOn = true
	introShip = newActor(len(models)-1, 37, animOnce, true)
	light = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 255, G: 255, B: 214}
	light.Set(0)
	yaroze.GsSetAmbient(0x200, 0x200, 0x200)
	view = yaroze.GsRVIEW2{Vpy: 0x100, Vpz: -0x800, Vry: 0x100}
	view.Set()
	setAmbient(false)
	playTune(tuneOpen)
}

// introFrame draws a frame of the opening.
func introFrame(ot *yaroze.GsOT) {
	t := sceneT
	sceneT++
	introShip.animate(true)
	introShip.drawActor(ot, 0)
	over := screen.Overlay()
	if t-0x100 >= 0 && t-0x100 < 0x20 {
		c := uint8((t - 0x100) * 8)
		fillRect(over.Entry(0), -160, -120, 320, 240, c, c, c, true, 1)
	}
	if t == 0x11f {
		fillRect(over.Entry(0), -160, -120, 320, 240, 255, 255, 255, true, 1)
		freeActor(introShip)
		introShip = nil
		introOn = false
		gameStart()
	}
	if t < 0x100 && padPressed()&0x8e0 != 0 {
		sceneT = 0x100
	}
}

// xy packs a screen point as the GPU takes it.
func xy(x, y int16) uint32 { return uint32(uint16(x)) | uint32(uint16(y))<<16 }

// fillRect links a filled rectangle at tag: opaque, or semi-transparent at
// the rate abr, 0 to 3, with a texture page packet setting the rate.
func fillRect(tag *uint32, x, y, w, h int16, r, g, b uint8, semi bool, abr uint32) {
	p := yaroze.AllocPacket(4)
	p[0] = 3 << 24
	code := uint32(0x60)
	if semi {
		code = 0x62
	}
	p[1] = code<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	p[2] = xy(x, y)
	p[3] = uint32(uint16(w)) | uint32(uint16(h))<<16
	yaroze.LinkTag(tag, p)
	if semi {
		q := yaroze.AllocPacket(2)
		q[0] = 1 << 24
		q[1] = 0xe1000200 | abr&3<<5
		yaroze.LinkTag(tag, q)
	}
}
