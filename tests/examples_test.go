package tests

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/format/scene"
)

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
	m.Golden("mdec.png")
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
	m.Golden("dungeon.png")
}

// TestCarScreenshot checks the car fifty frames after its drive in from
// the distance, close to the viewpoint and turning.
func TestCarScreenshot(t *testing.T) {
	m := bootExample(t, "car")
	m.RunUntilCount("main.frameNumber", 520, frameBudget)
	m.Golden("car.png")
}

// TestOdenScreenshot checks the three shapes under the three lights after
// the ball is swapped for its textured version and the scene is turned.
func TestOdenScreenshot(t *testing.T) {
	m := bootExample(t, "oden")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonLeft, 3)
	m.Hold(1, emu.ButtonSquare, 20)
	m.RunFrames(5)
	m.Golden("oden.png")
}

// TestMorphScreenshot checks the sheet most of the way to becoming a
// cylinder, with the picture wrapped round it.
func TestMorphScreenshot(t *testing.T) {
	m := bootExample(t, "morph")
	m.RunUntilCount("main.frameNumber", 400, frameBudget)
	m.Golden("morph.png")
}

// TestOffscrScreenshot checks the sphere wearing the texture the sprite is
// drawn into every frame, past the point where a block of VRAM is copied
// over the square's texture. The program prints the count of horizontal
// syncs VSync returns, which is the BIOS's to make: Sony's says 51 a frame
// and OpenBIOS 52, so this golden is Sony's by that one line.
func TestOffscrScreenshot(t *testing.T) {
	m := bootExample(t, "offscr")
	m.RunUntilCount("main.frameNumber", 330, frameBudget)
	m.Golden("offscr.png")
}

// TestTunnelScreenshot checks the tunnel after the flight in, with the
// score drawn in sprites.
func TestTunnelScreenshot(t *testing.T) {
	m := bootExample(t, "tunnel")
	m.RunUntilCount("main.frameNumber", 166, frameBudget)
	m.Golden("tunnel.png")
}

// TestFlameScreenshot checks the sheet of fire a second and a half in, with
// the hot spots along its bottom edge and the heat risen through the
// palette above them.
func TestFlameScreenshot(t *testing.T) {
	m := bootExample(t, "flame")
	m.RunUntilCount("main.frameNumber", 90, frameBudget)
	m.Golden("flame.png")
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
	m.Golden("invaders.png")
}

// TestBreakoutScreenshot checks the wall of bricks with the ball on its
// way down and the bat moved to the right to meet it.
func TestBreakoutScreenshot(t *testing.T) {
	m := bootExample(t, "breakout")
	m.RunUntilCount("main.frameNumber", 30, frameBudget)
	m.Hold(1, emu.ButtonRight, 20)
	m.RunFrames(100)
	m.Golden("breakout.png")
}

// TestDinoScreenshot checks the dinosaur in front of the castle two
// seconds into its walk.
func TestDinoScreenshot(t *testing.T) {
	m := bootExample(t, "dino")
	m.RunUntilCount("main.frameNumber", 120, frameBudget)
	m.Golden("dino.png")
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
	m.Golden("dogtale.png")
}

// The camera's quarters look along the rooms' diagonals, so a pair of
// directions walks the hero straight along a room's axis: before the
// camera has turned, up and left is +Z, the far wall, up and right +X,
// down and right -Z and down and left -X, and each quarter turn of the
// camera moves them round a pair.

// questState is where the quest example's state the tests read lies: the
// hero's room, position, height and facing are the first words of
// main.hero, the camera's quarter the first of main.cam, and the text
// box's up flag and line the first of main.talk.
type questSyms struct {
	cur, px, pz, py, facing, camDir, talking, talkLine uint32
}

func questState(m *emutest.Machine) questSyms {
	n, talk := m.Sym("main.hero"), m.Sym("main.talk")
	return questSyms{cur: n, px: n + 4, pz: n + 8, py: n + 12, facing: n + 16, camDir: m.Sym("main.cam"), talking: talk, talkLine: talk + 4}
}

// TestQuestScreenshot starts Hero in the middle of the hall, walks them
// up into its far wall, which stops them at the wall's margin, and right
// along it until the chest stops them, for the first picture; then L1
// eight times turns the camera all the way round.
func TestQuestScreenshot(t *testing.T) {
	m := bootExample(t, "quest")
	m.RunUntilCount("main.screen", 60, frameBudget)
	// The count is reached in the middle of a frame, after its controller
	// read; a frame on, a hold is seen from its first frame.
	m.RunFrames(1)
	j := questState(m)
	assert.Equal(t, uint32(0), m.Read32(j.cur), "Hero should start in the hall")
	assert.Equal(t, uint32(0), m.Read32(j.px), "in its middle")
	assert.Equal(t, uint32(0), m.Read32(j.pz), "in its middle")

	// The hall is 17 by 13 cells of 64, so its floor reaches z 352, and
	// the hero's footprint of 14 either way stops them at 336, in steps
	// of 3.
	m.Hold(1, emu.ButtonUp|emu.ButtonLeft, 150)
	assert.Equal(t, uint32(0), m.Read32(j.px), "walking to +Z before the camera has turned should not move the hero across")
	assert.Equal(t, uint32(336<<4), m.Read32(j.pz), "the hero should have stopped at the far wall")
	assert.Equal(t, uint32(0), m.Read32(j.facing), "the hero should face away from the camera")
	assert.Equal(t, uint32(0), m.Read32(j.py), "the hall is flat")

	// The chest is at x 384, z 320, its box 20 either side across and 14
	// deep, so along the wall the footprint's 14 stops Hero at x 348.
	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 130)
	assert.Equal(t, int32(348<<4), int32(m.Read32(j.px)), "the chest should stop Hero")
	assert.Equal(t, uint32(2), m.Read32(j.facing), "Hero should face along the wall")
	m.Golden("quest.png")

	// The camera starts at its second step, along the room's diagonals;
	// eight presses of L1 turn it all the way round.
	for k := range 8 {
		m.Hold(1, emu.ButtonL1, 2)
		m.RunFrames(10)
		assert.Equal(t, uint32(2+k)%8, m.Read32(j.camDir), "each press of L1 should turn the camera an eighth")
	}
}

// TestQuestChest walks Hero to the chest in the hall's far right corner,
// opens it with Cross, and reads what the text box says: the first three
// lines come a letter a frame, then the box waits with its arrow, Cross
// scrolls the last line in, and Cross closes the box, leaving the chest
// open; opened again, it is empty.
func TestQuestChest(t *testing.T) {
	m := bootExample(t, "quest")
	m.RunUntilCount("main.screen", 60, frameBudget)
	m.RunFrames(1)
	j := questState(m)

	// The chest is in the fifteenth column, x 384, and the second row, z
	// 320: 128 frames right reach its column and up stops Hero against
	// the chest's box, 14 deep, the footprint's 14 short of it, at z 291.
	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 128)
	m.Hold(1, emu.ButtonUp|emu.ButtonLeft, 120)
	assert.Equal(t, int32(384<<4), int32(m.Read32(j.px)), "Hero should stand in the chest's column")
	assert.Equal(t, int32(291<<4), int32(m.Read32(j.pz)), "the chest should stop Hero")
	m.Hold(1, emu.ButtonCross, 2)
	assert.Equal(t, uint32(1), m.Read32(j.talking)&0xff, "Cross at the chest should open it and put the text box up")
	// The first page is 21, 25 and 23 letters, a frame each, then the box
	// waits, the arrow showing on frames 96 to 111 of its count.
	m.RunFrames(100)
	assert.Equal(t, uint32(3), m.Read32(j.talkLine), "the first three lines should have been read out")
	m.Golden("quest-chest.png")
	m.Hold(1, emu.ButtonCross, 2)
	m.RunFrames(30)
	assert.Equal(t, uint32(4), m.Read32(j.talkLine), "Cross should scroll the last line in")
	assert.Equal(t, uint32(1), m.Read32(j.talking)&0xff, "the box should wait at the end of the text")
	m.Hold(1, emu.ButtonCross, 2)
	assert.Equal(t, uint32(0), m.Read32(j.talking)&0xff, "Cross at the end should close the box")
	m.RunFrames(10)
	m.Golden("quest-chest-open.png")
	m.Hold(1, emu.ButtonCross, 2)
	m.RunFrames(25)
	assert.Equal(t, uint32(1), m.Read32(j.talking)&0xff, "Cross at the opened chest should say it is empty")
	assert.Equal(t, uint32(1), m.Read32(j.talkLine), "the empty chest's line is one line")
}

// TestQuestMeadow walks Hero straight down out of the hall's doorway,
// below where they start, into the meadow, just inside the doorway in its
// far side, still walking down, for a picture of the grass and the
// house; then down to the villager's row and right until the villager
// stops them, and has the villager say their line.
func TestQuestMeadow(t *testing.T) {
	m := bootExample(t, "quest")
	m.RunUntilCount("main.screen", 60, frameBudget)
	m.RunFrames(1)
	j := questState(m)

	// The hall's floor reaches z -352, so the doorway is entered on the
	// 118th frame down, and Hero lands in the meadow's sixteenth column,
	// second row, z 704, where they stand through the black frames.
	m.Hold(1, emu.ButtonDown|emu.ButtonRight, 125)
	assert.Equal(t, uint32(1), m.Read32(j.cur), "the hall's doorway should lead to the meadow")
	assert.Equal(t, uint32(0), m.Read32(j.px), "Hero should stand in the doorway's column")
	assert.Equal(t, int32(704<<4), int32(m.Read32(j.pz)), "Hero should stand in the cell inside the meadow's doorway")
	assert.Equal(t, uint32(4), m.Read32(j.facing), "Hero should face into the meadow")
	m.RunFrames(20)
	m.Golden("quest-meadow.png")

	// The villager stands at x 256, z 0: 234 frames down is z 2, their
	// row, and right stops Hero a footprint either side short of them,
	// at x 228, facing them, close enough to speak to.
	m.Hold(1, emu.ButtonDown|emu.ButtonRight, 234)
	assert.Equal(t, int32(2<<4), int32(m.Read32(j.pz)), "Hero should stand in the villager's row")
	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 100)
	assert.Equal(t, int32(228<<4), int32(m.Read32(j.px)), "the villager should stop Hero")
	assert.Equal(t, uint32(2), m.Read32(j.facing), "Hero should face the villager")
	m.Hold(1, emu.ButtonCross, 2)
	assert.Equal(t, uint32(1), m.Read32(j.talking)&0xff, "Cross facing the villager should put their line up")
	m.RunFrames(80)
	m.Golden("quest-villager.png")
}

// TestQuestLive plays the playground's part: it asks the program to put
// Hero on a cell through the scene package's live hook, as Play from
// here does, and writes a hall with a wall painted beside her over the
// file the program loaded, as the Room panel does, which the program
// places again, so that the wall stops her.
func TestQuestLive(t *testing.T) {
	m := bootExample(t, "quest")
	m.RunUntilCount("main.screen", 60, frameBudget)
	m.RunFrames(1)
	j := questState(m)
	live := m.Sym("go.chrisrx.dev/psx/scene.live")
	ptr, words := m.Read32(live), m.Read32(live+4)
	assert.Equal(t, true, ptr != 0 && words > 0, "the hall's file should be the live scene")
	name := make([]byte, 0, 32)
	for k := uint32(0); k < 32; k += 4 {
		name = binary.LittleEndian.AppendUint32(name, m.Read32(live+16+k))
	}
	assert.Equal(t, "ASSETS/HALL.DAT", string(bytes.TrimRight(name, "\x00")), "the scene should know its file")

	// Play from here: cell 2, 2 of the hall.
	m.Bus.Write32(live+48, 2|2<<16)
	m.Bus.Write32(live+52, 1)
	m.RunFrames(2)
	assert.Equal(t, int32(-384<<4), int32(m.Read32(j.px)), "Hero should stand on cell 2, 2")
	assert.Equal(t, uint32(256<<4), m.Read32(j.pz))
	assert.Equal(t, uint32(0), m.Read32(live+52), "the request should be taken once")

	// A wall painted on the cell to her right, written over the file.
	b, err := os.ReadFile("../examples/quest/assets/HALL.DAT")
	assert.NoError(t, err)
	sc, err := scene.Decode(b)
	assert.NoError(t, err)
	row := []byte(sc.Room.Cells[2])
	row[3] = '#'
	sc.Room.Cells[2] = string(row)
	nb, err := sc.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, true, len(nb) <= int(words)*4, "the room should fit the memory it was loaded into")
	for k := 0; k+4 <= len(nb); k += 4 {
		m.Bus.Write32(ptr+uint32(k), binary.LittleEndian.Uint32(nb[k:]))
	}
	m.Bus.Write32(live+8, 1)
	m.RunFrames(2)
	assert.Equal(t, uint32(0), m.Read32(live+8), "the file should have been read again")
	m.Hold(1, emu.ButtonUp|emu.ButtonRight, 30)
	assert.Equal(t, int32(-369<<4), int32(m.Read32(j.px)), "the new wall should stop Hero at its edge")
	m.Golden("quest-live.png")

	// Play from here in the other room: the meadow's cell 4, 5, named
	// after the cell and the flag.
	want := []byte("ASSETS/MEADOW.DAT")
	for k := 0; k < 32; k += 4 {
		var w [4]byte
		copy(w[:], want[min(k, len(want)):])
		m.Bus.Write32(live+56+uint32(k), binary.LittleEndian.Uint32(w[:]))
	}
	m.Bus.Write32(live+48, 4|5<<16)
	m.Bus.Write32(live+52, 1)
	m.RunFrames(20)
	assert.Equal(t, uint32(1), m.Read32(j.cur), "Hero should be in the meadow")
	assert.Equal(t, int32(-704<<4), int32(m.Read32(j.px)), "on cell 4, 5")
	assert.Equal(t, int32(448<<4), int32(m.Read32(j.pz)))
	name = name[:0]
	for k := uint32(0); k < 32; k += 4 {
		name = binary.LittleEndian.AppendUint32(name, m.Read32(live+16+k))
	}
	assert.Equal(t, "ASSETS/MEADOW.DAT", string(bytes.TrimRight(name, "\x00")), "the live scene should be the meadow")
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
	m.Golden("scene.png")

	// Start loads the other scene file: the tower, under its own camera.
	m.Hold(1, emu.ButtonStart, 2)
	m.RunFrames(5)
	m.Golden("scene-tower.png")
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
	assert.Equal(t, uint32(0x45535341), m.Read32(live+16), "the file's name, ASSETS/CUBES.DAT, should follow") // "ASSE"

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
	m.Golden("scene-live.png")
}

// TestJumpcube plays jumpcube: a run and a jump through the first coins,
// the cube turning through the jump and finishing its quarter turn on
// landing, a jump from under the one-way planks up through them onto
// their top, a fall into spikes, which puts the cube back on the ground
// before them, and the flag, which ends the level until Start plays it
// again with its coins back. The cube is put where each part of the
// level is by writing its position between frames.
func TestJumpcube(t *testing.T) {
	m := bootExample(t, "jumpcube")
	m.RunUntilCount("main.screen", 60, frameBudget)
	get := func(name string) int { return int(int32(m.Read32(m.Sym(name)))) }
	set := func(name string, v int) { m.Bus.Write32(m.Sym(name), uint32(int32(v))) }
	is := func(name string) bool { return m.Bus.Read8(m.Sym(name)) != 0 }
	put := func(x, y int) {
		set("main.heroX", x)
		set("main.heroY", y)
		set("main.speedX", 0)
		set("main.speedY", 0)
	}
	assert.Equal(t, 40, get("main.heroX"), "the hero should start where the scene's sprite is")
	assert.Equal(t, 192, get("main.heroY"))
	assert.Equal(t, 35, get("main.total"), "the level's coins")

	// Running into the row of three coins and jumping through it, and on
	// into the crates.
	pad := m.Pad(1)
	pad.Press(emu.ButtonRight)
	m.RunFrames(60)
	pad.Press(emu.ButtonCross)
	m.RunFrames(8)
	assert.Equal(t, true, get("main.spin") > 0, "the cube should be turning, clockwise, as it jumps to the right")
	m.Golden("jumpcube.png")
	pad.Release(emu.ButtonCross)
	m.RunFrames(40)
	pad.Release(emu.ButtonRight)
	m.RunFrames(20)
	assert.Equal(t, 3, get("main.coins"), "the jump should have taken the three coins")
	assert.Equal(t, 235, get("main.heroX"), "the blocks should have stopped the cube")
	assert.Equal(t, true, is("main.standing"))
	assert.Equal(t, 0, get("main.spin")%(90*4096), "the cube should have landed on a side")

	// From under the planks a jump goes up through them, taking the coin
	// over them, and comes down on their top.
	put(33*16+8, 192)
	m.RunFrames(2)
	m.Hold(1, emu.ButtonCross, 30)
	assert.Equal(t, 144, get("main.heroY"), "the hero should stand on the planks")
	assert.Equal(t, true, is("main.standing"))
	assert.Equal(t, 4, get("main.coins"))

	// Down into the spikes at the foot of the pit, and back to the ground
	// before it.
	set("main.safeX", 700)
	set("main.safeY", 192)
	put(47*16+8, 160)
	m.RunFrames(30)
	assert.Equal(t, 700, get("main.heroX"), "the spikes should have sent the hero back")
	assert.Equal(t, 192, get("main.heroY"))

	// To the flag, which ends the level, then Start, which plays it again
	// with every coin back.
	put(150*16, 192)
	pad.Press(emu.ButtonRight)
	m.RunFrames(60)
	pad.Release(emu.ButtonRight)
	m.RunFrames(10)
	assert.Equal(t, true, is("main.won"), "the flag should end the level")
	assert.Equal(t, 2240, get("main.camX"), "the scroll should stop at the level's end")
	m.Golden("jumpcube-won.png")
	m.Hold(1, emu.ButtonStart, 2)
	assert.Equal(t, false, is("main.won"))
	assert.Equal(t, 40, get("main.heroX"))
	assert.Equal(t, 0, get("main.coins"))
	assert.Equal(t, 35, get("main.total"), "the coins taken should be back")
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
	m.Golden("square.png")
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
	m.Golden("terra.png")
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
	m.Golden("pads.png")

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
	m.Golden("card.png")
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
	// Sony's BIOS plays a chime with its logo, which is still ringing down
	// when the program starts, so wait for the SPU to fall silent before
	// taking that as what silence sounds like.
	quiet := peak(m, 30)
	for i := 0; i < 10 && quiet >= 100; i++ {
		quiet = peak(m, 30)
	}

	m.Hold(1, emu.ButtonRight, 3) // the second track
	m.Hold(1, emu.ButtonCross, 3)
	m.RunFrames(60)
	assert.Equal(t, int32(1), int32(m.Read32(playing)), "the second track plays")
	loud := peak(m, 30)
	t.Logf("peak %d stopped, %d playing", quiet, loud)
	assert.Equal(t, true, loud > 1000 && quiet < 100, "the track is heard")
	m.Golden("cdda.png")

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
	m.Golden("timers.png")
}

// TestTilesScreenshot scrolls the map right and down, to where its four
// screens meet, and checks the scroll stops at the map's far corner.
func TestTilesScreenshot(t *testing.T) {
	m := bootExample(t, "tiles")
	m.RunUntilCount("main.screen", 30, frameBudget)
	m.Hold(1, emu.ButtonRight|emu.ButtonDown, 60)
	m.Golden("tiles.png")
	m.Hold(1, emu.ButtonRight|emu.ButtonDown, 200)
	bg := m.Sym("main.bg")
	scroll := m.Read32(bg + 12)
	assert.Equal(t, [2]int16{40*16 - 320, 32*16 - 240}, [2]int16{int16(scroll), int16(scroll >> 16)}, "scrolled to the far corner")
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
	m.Golden("split.png")

	m.Hold(1, emu.ButtonSelect, 6)
	assert.Equal(t, uint32(1), m.Read32(m.Sym("main.sideBySide"))&0xff, "side by side")
	m.Golden("split-side.png")
}
