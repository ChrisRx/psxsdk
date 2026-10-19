package tests

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/format/scene"
)

// bootExample boots the BIOS with an example's disc in the drive and runs
// until the BIOS starts the program.
func bootExample(t *testing.T, name string) *emutest.Machine {
	t.Helper()
	cue := disc(t, name)
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc(cue),
		emutest.Symbols(filepath.Join(filepath.Dir(cue), "prog.elf")),
	)
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	return m
}

// The examples' screenshots: each once the program's own frame counter
// reaches a count, which puts the test at the same point in the program's
// life whichever BIOS started it, and however long that BIOS took reading
// its data off the disc; a count of frames since the BIOS said it was
// starting the program came out a frame or so apart between Sony's and
// OpenBIOS. Run them with -update-golden to accept a change.

// frameBudget is the time a program is given to reach a count, with its
// loading included.
const frameBudget = 30 * emutest.Second

// TestMDECScreenshot checks a frame of the movie the MDEC decodes, shown in
// 24-bit color.
func TestMDECScreenshot(t *testing.T) {
	m := bootExample(t, "mdec")
	m.RunUntilCount("main.shown", 86, frameBudget)
	emutest.Golden(t, m.Display(), "mdec.png")
}

// TestDungeonScreenshot checks the maze after a turn to the left and a walk
// up to a wall, with the compass turned to match. The program prints the
// count of horizontal syncs VSync returns, which is the BIOS's to make, so
// this golden is OpenBIOS's by that one line.
func TestDungeonScreenshot(t *testing.T) {
	m := bootExample(t, "dungeon")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonLeft, 40)
	m.Hold(1, emu.ButtonUp, 150)
	m.RunFrames(10)
	emutest.Golden(t, m.Display(), "dungeon.png")
}

// TestCarScreenshot checks the car fifty frames after its drive in from
// the distance, close to the viewpoint and turning.
func TestCarScreenshot(t *testing.T) {
	m := bootExample(t, "car")
	m.RunUntilCount("main.frameNumber", 520, frameBudget)
	emutest.Golden(t, m.Display(), "car.png")
}

// TestOdenScreenshot checks the three shapes under the three lights after
// the ball is swapped for its textured version and the scene is turned.
func TestOdenScreenshot(t *testing.T) {
	m := bootExample(t, "oden")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonLeft, 3)
	m.Hold(1, emu.ButtonSquare, 20)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "oden.png")
}

// TestMorphScreenshot checks the sheet most of the way to becoming a
// cylinder, with the picture wrapped round it.
func TestMorphScreenshot(t *testing.T) {
	m := bootExample(t, "morph")
	m.RunUntilCount("main.frameNumber", 400, frameBudget)
	emutest.Golden(t, m.Display(), "morph.png")
}

// TestOffscrScreenshot checks the sphere wearing the texture the sprite is
// drawn into every frame, past the point where a block of VRAM is copied
// over the square's texture. The program prints the count of horizontal
// syncs VSync returns, which is the BIOS's to make: Sony's says 51 a frame
// and OpenBIOS 52, so this golden is Sony's by that one line.
func TestOffscrScreenshot(t *testing.T) {
	m := bootExample(t, "offscr")
	m.RunUntilCount("main.frameNumber", 330, frameBudget)
	emutest.Golden(t, m.Display(), "offscr.png")
}

// TestTunnelScreenshot checks the tunnel after the flight in, with the
// score drawn in sprites.
func TestTunnelScreenshot(t *testing.T) {
	m := bootExample(t, "tunnel")
	m.RunUntilCount("main.frameNumber", 166, frameBudget)
	emutest.Golden(t, m.Display(), "tunnel.png")
}

// TestFlameScreenshot checks the sheet of fire a second and a half in, with
// the hot spots along its bottom edge and the heat risen through the
// palette above them.
func TestFlameScreenshot(t *testing.T) {
	m := bootExample(t, "flame")
	m.RunUntilCount("main.frameNumber", 90, frameBudget)
	emutest.Golden(t, m.Display(), "flame.png")
}

// TestInvadersScreenshot checks the invaders on their march after the ship
// has slid to the left and fired a shot.
func TestInvadersScreenshot(t *testing.T) {
	m := bootExample(t, "invaders")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonLeft, 20)
	m.RunFrames(5)
	m.Hold(1, emu.ButtonTriangle, 5)
	m.RunFrames(40)
	emutest.Golden(t, m.Display(), "invaders.png")
}

// TestBreakoutScreenshot checks the wall of bricks with the ball on its
// way down and the bat moved to the right to meet it.
func TestBreakoutScreenshot(t *testing.T) {
	m := bootExample(t, "breakout")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonRight, 20)
	m.RunFrames(100)
	emutest.Golden(t, m.Display(), "breakout.png")
}

// TestBlowScreenshot checks the shuttle's first piece a second and a half
// into its explosion, with the menu on the explosion's page: R1 steps the
// menu back from the first page to the last, Circle moves to its second
// line and Triangle turns the explosion on.
func TestBlowScreenshot(t *testing.T) {
	m := bootExample(t, "blow")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonR1, 3)
	m.RunFrames(15)
	m.Hold(1, emu.ButtonCircle, 3)
	m.RunFrames(15)
	m.Hold(1, emu.ButtonTriangle, 3)
	m.RunFrames(90)
	emutest.Golden(t, m.Display(), "blow.png")
}

// TestDinoScreenshot checks the dinosaur in front of the castle two
// seconds into its walk.
func TestDinoScreenshot(t *testing.T) {
	m := bootExample(t, "dino")
	m.RunUntilCount("main.frameNumber", 120, frameBudget)
	emutest.Golden(t, m.Display(), "dino.png")
}

// TestDogTaleScreenshot checks the level a little way in: the title is
// left by choosing Play with Right and Cross, "get ready" is shown for 75
// frames, and then the man walks right, past the first bones, and jumps
// before he reaches the first cactus.
func TestDogTaleScreenshot(t *testing.T) {
	m := bootExample(t, "dogtale")
	m.RunUntilCount("main.frameNumber", 60, frameBudget)
	m.Hold(1, emu.ButtonRight, 3)
	m.RunFrames(3)
	m.Hold(1, emu.ButtonCross, 3)
	m.RunFrames(90)
	m.Hold(1, emu.ButtonRight, 25)
	m.Hold(1, emu.ButtonCross|emu.ButtonRight, 3)
	m.RunFrames(10)
	emutest.Golden(t, m.Display(), "dogtale.png")
}

// TestSceneScreenshot checks the three cubes a scene file places, under
// its camera and light, with the sprite in the corner, after the middle
// cube has turned by the screen's count and been turned further to the
// right, and that Cross stops it turning: a frame held after the press
// then matches a frame fifteen later.
func TestSceneScreenshot(t *testing.T) {
	m := bootExample(t, "scene")
	m.RunUntilCount("main.screen", 120, frameBudget)
	m.Hold(1, emu.ButtonRight, 10)
	m.Hold(1, emu.ButtonCross, 2)
	m.RunFrames(2)
	still := m.Display()
	m.RunFrames(15)
	assert.Equal(t, true, bytes.Equal(still.Pix, m.Display().Pix), "the cube should have stopped turning")
	emutest.Golden(t, m.Display(), "scene.png")

	// Start loads the other scene file: the tower, under its own camera.
	m.Hold(1, emu.ButtonStart, 2)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "scene-tower.png")
}

// TestSceneLive checks the playground's live view of a scene: a new scene
// file written over the one the program loaded, with the scene package's
// reload flag set, moves the cubes on the next frame without a rebuild.
func TestSceneLive(t *testing.T) {
	m := bootExample(t, "scene")
	m.RunUntilCount("main.screen", 60, frameBudget)
	live := m.Sym("go.chrisrx.dev/psx/scene.live")
	ptr, words := m.Read32(live), m.Read32(live+4)
	assert.Equal(t, true, ptr != 0 && words > 0, "the scene should have recorded its file")
	assert.Equal(t, uint32(0x41544144), m.Read32(live+16), "the file's name, DATA/CUBES.DAT, should follow") // "DATA"

	// The example's scene with the cubes raised and the middle one far
	// to the left, which must fit the sectors the file was loaded into.
	src, err := os.ReadFile("../examples/scene/scenes/cubes.json")
	assert.NoError(t, err)
	var sc scene.Scene
	assert.NoError(t, json.Unmarshal(src, &sc))
	for i := range sc.Objects {
		sc.Objects[i].Position[1] -= 100
	}
	sc.Objects[0].Position[0] = -400
	data, err := sc.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, true, len(data) <= int(words)*4, "the new scene should fit")
	before := m.Display()
	for i := 0; i+4 <= len(data); i += 4 {
		m.Bus.Write32(ptr+uint32(i), binary.LittleEndian.Uint32(data[i:]))
	}
	m.Bus.Write32(live+8, 1)
	m.RunFrames(3)
	assert.Equal(t, uint32(0), m.Read32(live+8), "the program should have taken the flag")
	assert.Equal(t, false, bytes.Equal(before.Pix, m.Display().Pix), "the picture should have changed")
	emutest.Golden(t, m.Display(), "scene-live.png")
}

// TestSquareScreenshot checks the box after it is moved up and to the right
// over the turning cube, with left and right held together for a while to
// show that they cancel, and that the music plays, stops while Start has
// paused it, and plays again after Start resumes it. The square is the
// program the playground starts from. Its frames are counted by the
// screen's own count, the first word of yaroze.Screen, and the cube turns
// by that count, so that the picture is the same however long the BIOS
// took to start the program and the disc to deliver its files.
func TestSquareScreenshot(t *testing.T) {
	m := bootExample(t, "square")
	m.SetAudio(true)
	m.RunUntilCount("main.screen", 150, frameBudget)

	// Playing is a peak in the thousands. Paused, what is left of the
	// notes' release after half a second is a few steps from silence.
	const quiet = 100
	assert.Equal(t, true, peak(m, 60) > quiet, "the music plays")
	m.Hold(1, emu.ButtonStart, 2)
	peak(m, 30) // the notes' release, which also empties what Audio keeps
	assert.Equal(t, true, peak(m, 30) < quiet, "the music is paused")
	m.Hold(1, emu.ButtonStart, 2)
	assert.Equal(t, true, peak(m, 60) > quiet, "the music plays again")

	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 60)
	m.Hold(1, emu.ButtonLeft|emu.ButtonRight, 20)
	m.RunFrames(5)
	emutest.Golden(t, m.Display(), "square.png")
}

// TestTerraScreenshot checks the first conversation at the port: the
// opening's logo flies by until its counter reaches its end, the title
// comes up over the ship and holds at its count, Cross takes it down, and
// the camera's tour ends on the two of them with the first line said. The
// counts are the scene counter's, which the opening and the title each
// run from zero, so the picture is the same however long the BIOS took.
func TestTerraScreenshot(t *testing.T) {
	m := bootExample(t, "terra")
	sceneT := m.Sym("main.sceneT")
	m.RunUntilCount("main.sceneT", 0x11f, frameBudget)
	// The port is built while the counter still reads the opening's end.
	m.RunUntil(func() bool { return m.Read32(sceneT) < 0x11f }, frameBudget, "the title starting")
	m.RunUntilCount("main.sceneT", 0xe0, frameBudget)
	// The program reads the controller every other vertical blank, so a
	// press must last two of its frames to be sure of being seen.
	m.Hold(1, emu.ButtonCross, 4)
	m.RunUntilCount("main.sceneT", 0x2c0, frameBudget)
	m.RunFrames(80)
	emutest.Golden(t, m.Display(), "terra.png")
}

// peak runs the machine for frames frames and returns the loudest sample
// the SPU made meanwhile.
func peak(m *emutest.Machine, frames int) int {
	var p int
	var buf []int16
	for range frames {
		m.RunFrames(1)
		buf = m.Audio(buf[:0])
		for _, s := range buf {
			p = max(p, int(s), -int(s))
		}
	}
	return p
}

// TestPadsScreenshot plugs an analog controller into port 1 and a mouse into
// port 2, holds Cross and pushes the left stick right while the mouse moves
// up, and checks that the cursor went right and up and that the screen
// names both and what they send.
func TestPadsScreenshot(t *testing.T) {
	m := bootExample(t, "pads")
	analog := emu.NewPad(emu.PadAnalog)
	mouse := emu.NewPad(emu.PadMouse)
	m.ConnectPad(1, analog)
	m.ConnectPad(2, mouse)
	m.RunUntilCount("main.screen", 30, frameBudget)
	cx, cy := m.Sym("main.cursorX"), m.Sym("main.cursorY")
	x0, y0 := int32(m.Read32(cx)), int32(m.Read32(cy))

	analog.SetAxes(0x80, 0x80, 0xff, 0x80) // left stick hard right
	analog.Press(emu.ButtonCross)
	mouse.Press(emu.MouseLeft)
	for range 30 {
		mouse.Move(0, -2)
		m.RunFrames(1)
	}
	x1, y1 := int32(m.Read32(cx)), int32(m.Read32(cy))
	assert.Equal(t, true, x1 > x0, "the stick moves the cursor right")
	assert.Equal(t, true, y1 < y0, "the mouse moves the cursor up")
	emutest.Golden(t, m.Display(), "pads.png")

	// A GunCon in port 2, aimed at the middle of the picture, puts the
	// cursor there; a neGcon in port 1 twisted right moves it right.
	gun := emu.NewPad(emu.PadGunCon)
	gun.Aim(269, 145) // the middle: (269-77)*320/384 and (145-25)*240/239
	m.ConnectPad(1, emu.NewPad(emu.PadDigital))
	m.ConnectPad(2, gun)
	m.RunFrames(3)
	assert.Equal(t, [2]int32{160, 120}, [2]int32{int32(m.Read32(cx)), int32(m.Read32(cy))}, "the GunCon aims the cursor")
	negcon := emu.NewPad(emu.PadNeGcon)
	negcon.SetAxes(0xc0, 0, 0, 0)
	m.ConnectPad(1, negcon)
	m.ConnectPad(2, nil)
	m.RunFrames(10)
	assert.Equal(t, true, int32(m.Read32(cx)) > 160, "the neGcon's twist steers the cursor")
}

// TestCardScreenshot formats a blank memory card in slot 1, saves a number
// to it, changes the number and loads it back, and then lists the card's
// files and dumps the save, whose first bytes are Sony's header: "SC", one
// icon frame and one block, and the title in full-width Shift-JIS. It
// passes under Sony's BIOS as well as OpenBIOS: run it with PSX_BIOS set to
// one of Sony's to check the card against the BIOS that games used.
func TestCardScreenshot(t *testing.T) {
	m := bootExample(t, "card")
	card, err := emu.NewMemoryCard(nil)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m.InsertCard(1, card)
	m.RunUntilCount("main.screen", 60, frameBudget)
	number, status := m.Sym("main.number"), m.Sym("main.status")
	t.Logf("card status %d", int32(m.Read32(status)))

	// The program runs at thirty frames a second, so a press lasts long
	// enough for it to be read on one of them. Each action talks to the
	// card, which Sony's BIOS takes a second or so over, while the program
	// waits, so an action is given that before the next press.
	press := func(b uint16) { m.Hold(1, b, 6) }
	act := func() {
		press(emu.ButtonCross)
		m.RunFrames(90)
	}
	for range 5 {
		press(emu.ButtonDown) // to FORMAT
	}
	act()
	assert.Equal(t, int32(1), int32(m.Read32(status)), "the card is formatted")
	press(emu.ButtonDown)
	press(emu.ButtonDown) // round to SAVE
	for range 3 {
		press(emu.ButtonRight)
	}
	act()
	assert.Equal(t, int32(3), int32(m.Read32(number)))
	press(emu.ButtonLeft)
	press(emu.ButtonLeft)
	press(emu.ButtonDown) // LOAD
	act()
	assert.Equal(t, int32(3), int32(m.Read32(number)), "the number is loaded back")

	// The save on the card: its directory entry names it, and the block
	// the BIOS put it in starts with the header.
	data := card.Data()
	assert.Equal(t, true, bytes.Contains(data[:8192], []byte("BA-NETYAROZEPSXSDK")), "the directory names the save")
	header := false
	for b := 8192; b < len(data); b += 8192 {
		header = header || bytes.HasPrefix(data[b:], []byte{'S', 'C', 0x11, 1, 0x82, 0x6f})
	}
	assert.Equal(t, true, header, "a block starts with the header and the title's first P")

	press(emu.ButtonDown)
	press(emu.ButtonDown)
	press(emu.ButtonDown) // DUMP
	act()
	act()
	emutest.Golden(t, m.Display(), "card.png")
}

// TestCDDAScreenshot checks that the program learns its tracks' numbers,
// 2 to 4 after the data track, that cross plays the chosen one, which is
// heard in what the SPU mixes and which the drive then reports playing, and
// that circle stops it.
func TestCDDAScreenshot(t *testing.T) {
	m := bootExample(t, "cdda")
	m.SetAudio(true)
	m.RunUntilCount("main.screen", 30, frameBudget)
	tracks, playing := m.Sym("main.tracks"), m.Sym("main.playing")
	assert.Equal(t, [3]uint32{2, 3, 4}, [3]uint32{m.Read32(tracks), m.Read32(tracks + 4), m.Read32(tracks + 8)}, "the tracks' numbers")
	quiet := peak(m, 30)

	m.Hold(1, emu.ButtonRight, 3) // the second track
	m.Hold(1, emu.ButtonCross, 3)
	m.RunFrames(60)
	assert.Equal(t, int32(1), int32(m.Read32(playing)), "the second track plays")
	loud := peak(m, 30)
	t.Logf("peak %d stopped, %d playing", quiet, loud)
	assert.Equal(t, true, loud > 1000 && quiet < 100, "the track is heard")
	emutest.Golden(t, m.Display(), "cdda.png")

	m.Hold(1, emu.ButtonCircle, 3)
	m.RunFrames(30)
	assert.Equal(t, int32(-1), int32(m.Read32(playing)), "circle stops it")
}

// TestTimersScreenshot checks that the line counter runs and passes its
// mark every four seconds or so, and that more sprites take more of the
// frame, measured in lines.
func TestTimersScreenshot(t *testing.T) {
	m := bootExample(t, "timers")
	m.RunUntilCount("main.screen", 60, frameBudget)
	count, lines, wraps := m.Sym("main.count"), m.Sym("main.lines"), m.Sym("main.wraps")
	few := int32(m.Read32(lines))
	m.Pad(1).Press(emu.ButtonUp)
	m.RunUntil(func() bool { return m.Read32(count) >= 400 }, 30*emutest.Second, "four hundred sprites")
	m.Pad(1).Release(emu.ButtonUp)
	m.RunFrames(300)
	many := int32(m.Read32(lines))
	t.Logf("%d lines with 20 sprites, %d with 400; the counter passed its mark %d times", few, many, m.Read32(wraps))
	assert.Equal(t, true, many > few+20, "more sprites take more of the frame")
	assert.Equal(t, true, m.Read32(wraps) >= 1, "the line counter passes its mark")
	emutest.Golden(t, m.Display(), "timers.png")
}

// TestTilesScreenshot scrolls the map right and down, to where its four
// screens meet, and checks the scroll stops at the map's far corner.
func TestTilesScreenshot(t *testing.T) {
	m := bootExample(t, "tiles")
	m.RunUntilCount("main.screen", 30, frameBudget)
	m.Hold(1, emu.ButtonRight|emu.ButtonDown, 60)
	emutest.Golden(t, m.Display(), "tiles.png")
	m.Hold(1, emu.ButtonRight|emu.ButtonDown, 200)
	bg := m.Sym("main.bg")
	scroll := m.Read32(bg + 12)
	assert.Equal(t, [2]int16{40*16 - 320, 32*16 - 240}, [2]int16{int16(scroll), int16(scroll >> 16)}, "scrolled to the far corner")
}

// TestAIScreenshot checks that the enemy closes on the player while circle
// is held and draws away when it is let go, that in the random states it
// changes state, and shows the random states.
func TestAIScreenshot(t *testing.T) {
	m := bootExample(t, "ai")
	m.RunUntilCount("main.screen", 30, frameBudget)
	dist := m.Sym("main.distance")
	far := int32(m.Read32(dist))
	m.Pad(1).Press(emu.ButtonCircle)
	m.RunFrames(60)
	near := int32(m.Read32(dist))
	m.Pad(1).Release(emu.ButtonCircle)
	m.RunFrames(60)
	away := int32(m.Read32(dist))
	t.Logf("distance %d, %d attacking, %d evading", far, near, away)
	assert.Equal(t, true, near < far-50, "attack closes in")
	assert.Equal(t, true, away > near+20, "evade draws away")

	for range 5 {
		m.Hold(1, emu.ButtonR1, 6)
		m.RunFrames(4)
	}
	assert.Equal(t, uint32(5), m.Read32(m.Sym("main.mode")), "the random states")
	st := m.Sym("main.state")
	first := m.Read32(st)
	m.RunUntil(func() bool { return m.Read32(st) != first }, 60*emutest.Second, "a change of state")
	m.RunFrames(40)
	emutest.Golden(t, m.Display(), "ai.png")
}

// The layout of yaroze.Object on the console: a GsDOBJ2 of 16 bytes and a
// GsCOORDINATE2 of 80, then Position, then Rotation and Scale.
const (
	objectPosition = 16 + 80
	objectSize     = objectPosition + 16 + 8 + 16
)

// TestSplitScreenshot drives each player's cube, checks each moved, and
// shows the halves one above the other and then side by side.
func TestSplitScreenshot(t *testing.T) {
	m := bootExample(t, "split")
	frames := m.Sym("main.frames")
	m.RunUntil(func() bool { return m.Read32(frames) >= 30 }, frameBudget, "thirty frames")
	players := m.Sym("main.players")
	// An Object's Position follows its GsDOBJ2 and coordinate system.
	pos := func(i int) int32 { return int32(m.Read32(players + uint32(i)*objectSize + objectPosition)) }
	x0, x1 := pos(0), pos(1)
	m.Pad(1).Press(emu.ButtonRight | emu.ButtonSquare)
	m.RunFrames(30)
	m.Pad(1).Release(emu.ButtonRight | emu.ButtonSquare)
	m.RunFrames(2)
	t.Logf("player 1 %d to %d, player 2 %d to %d", x0, pos(0), x1, pos(1))
	assert.Equal(t, true, pos(0) > x0, "the first cube moves right")
	assert.Equal(t, true, pos(1) < x1, "the second cube moves left")
	emutest.Golden(t, m.Display(), "split.png")

	m.Hold(1, emu.ButtonSelect, 6)
	assert.Equal(t, uint32(1), m.Read32(m.Sym("main.sideBySide"))&0xff, "side by side")
	emutest.Golden(t, m.Display(), "split-side.png")
}
