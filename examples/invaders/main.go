// Space Invaders in sprites, ten levels of it, with music and a high score
// table.
//
// A port of L. Evans's Net Yaroze demo "invaders" (Sony, 1996): Space
// Invaders in sprites. Four rows of eight invaders march across the screen,
// dropping a line each time they reach a side, and one of them fires back
// now and then; the player's ship slides along the bottom and shoots
// upwards, no more than once every ten frames. Clearing the screen starts
// the next of ten levels, all alike, and an invader or a shot reaching the
// ship costs one of five lives. When they are gone, or the tenth level is
// cleared, the score and the high score table are shown. Behind the sprites
// a semi-transparent black box covers the screen, which the original uses to
// soften the black edges its sprites have, and the library's font prints the
// frame, the lives and the score. Music plays from a sequence off the disc.
//
// Differences from the original:
//
//   - Positions and speeds are fixed point, with four bits of fraction,
//     where the original has doubles. The only fraction in the game is the
//     player's shot, which moves two and a half pixels a frame.
//   - The textures and the sound are read off the disc into the program's
//     own memory, where the original's batch script sent them to fixed
//     addresses. The sound comes from the check sample's files.
//   - An invader only fires when there is one to fire. The original picks
//     one at random by dividing by how many are left, which is zero in the
//     frame after the last is destroyed, and then indexes its arrays with
//     what that gives.
//   - A shot that cannot be created, with all of them in flight, is not
//     fired. The original writes the shot into the element before the first.
//   - The missing data is reported, where the original would hang.
//
// Nothing allocates after startup.
//
// # Controls
//
// Left and right push the ship, which has momentum, and Triangle fires. L1
// pauses for as long as it is held, and Start and Select together quit. On
// the score screen Cross starts a new game and L1 and R1 together quit. If
// the invaders reach the bottom of the screen the program ends.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	mvol = 127
	svol = 127

	maxInvaders        = 100
	maxShots           = 250
	maxObjects         = maxInvaders + maxShots + 1
	maxSizeofPrimitive = 24 // GPU packet space allowed per primitive

	screenWidth  = 320
	screenHeight = 240

	// The walls.
	minX = 16
	maxX = 286
	minY = 30
	maxY = 196

	// The screen offset that makes the picture sit right.
	screenXOffset = 0
	screenYOffset = 16

	// Not a 3D game, so one place in the ordering table does.
	otLength = 1

	maxXSpeed = 2

	// fix is how many units make a pixel in a position or a speed.
	fix = 16

	none = 32766 // an empty type field

	// Kinds of invader; only the first is used.
	invader1 = 1

	// Whose shot it is.
	playersShot  = 1
	invadersShot = 2
)

// The sound and the textures, read into the program's own memory, in
// buffers of whole sectors.
var (
	vhData      [(60448 + 2047) / 2048 * 512]uint32
	vbData      [(401360 + 2047) / 2048 * 512]uint32
	seqData     [(17341 + 2047) / 2048 * 512]uint32
	invaderData [(1568 + 2047) / 2048 * 512]uint32
	shipData    [(2592 + 2047) / 2048 * 512]uint32
	shotData    [(576 + 2047) / 2048 * 512]uint32

	vh          = disc.Register("ASSETS/SOUND/STD0.VH", "assets/STD0.VH").Into(vhData[:])
	vb          = disc.Register("ASSETS/SOUND/STD0.VB", "assets/STD0.VB").Into(vbData[:])
	seqFile     = disc.Register("ASSETS/SOUND/GOGO.SEQ", "assets/GOGO.SEQ").Into(seqData[:])
	invaderFile = disc.Register("ASSETS/INVADER1.TIM", "assets/INVADER1.TIM").Into(invaderData[:])
	shipFile    = disc.Register("ASSETS/SHIP1.TIM", "assets/SHIP1.TIM").Into(shipData[:])
	shotFile    = disc.Register("ASSETS/FIRE04.TIM", "assets/FIRE04.TIM").Into(shotData[:])
	files       = []*disc.File{&vh, &vb, &seqFile, &invaderFile, &shipFile, &shotFile}

	invaderTIM = yaroze.TIM(invaderData[:])
	shipTIM    = yaroze.TIM(shipData[:])
	shotTIM    = yaroze.TIM(shotData[:])
)

// A movement is where something is and how it moves: its position and
// speed in fix units, and its size in pixels.
type movement struct {
	x, y   int32
	dx, dy int32
	w, h   int32
}

type invader struct {
	id     int32
	alive  bool
	typ    int32
	sprite yaroze.GsSPRITE
	m      movement
}

type shot struct {
	id     int32
	typ    int32 // playersShot, invadersShot or none
	sprite yaroze.GsSPRITE
	m      movement
}

type score struct {
	name   string
	points uint32
}

var (
	playersMovement          movement
	playersSprite            yaroze.GsSPRITE
	timeSincePlayerLastFired int32

	backgroundRectangle yaroze.GsBOXF

	worldOT       [2]yaroze.GsOT
	otTags        [2][1 << otLength]yaroze.GsOT_TAG
	gpuPacketArea [2][maxObjects * maxSizeofPrimitive / 4]uint32

	padStatus yaroze.Buttons
	quit      bool

	invaders [maxInvaders]invader
	shots    [maxShots]shot

	level         int32
	numberOfLives int32

	highScoreTable [5]score
	currentScore   score

	bank         yaroze.Bank
	song         yaroze.Sequence
	soundSuccess bool

	frameNumber uint32

	textBuf [512]byte
	text    yaroze.Buffer
)

func main() {
	text.Init(textBuf[:])
	if !initialiseAll() {
		return
	}

	for !quit {
		dealWithControllerPad()
		handleAllObjects()

		active := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[active][0]))
		yaroze.GsClearOt(0, 0, &worldOT[active])

		yaroze.GsSortBoxFill(&backgroundRectangle, &worldOT[active], 0)
		yaroze.GsSortSprite(&playersSprite, &worldOT[active], 0)
		for i := range invaders {
			if invaders[i].alive {
				yaroze.GsSortSprite(&invaders[i].sprite, &worldOT[active], 0)
			}
		}
		for i := range shots {
			if shots[i].typ != none {
				yaroze.GsSortSprite(&shots[i].sprite, &worldOT[active], 0)
			}
		}

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(60, 120, 120, &worldOT[active])
		yaroze.GsDrawOt(&worldOT[active])

		text.Reset()
		text.WriteString("Space Invaders!\nFrame number: ").WriteInt(int(frameNumber)).
			WriteString("\nLives: ").WriteInt(numberOfLives).
			WriteString("\nScore: ").WriteInt(int(currentScore.points)).WriteString("\n").Print()
		yaroze.FntFlush(-1)

		if countNumberOfInvaders() == 0 { // end of level
			level++
			if level > 10 {
				if dealWithEnd("You have finished ten levels") {
					newGame()
				} else {
					quit = true
				}
			} else {
				renewLevel()
			}
		}

		if numberOfLives <= 0 {
			if dealWithEnd("You have died") {
				newGame()
			} else {
				quit = true
			}
		}

		frameNumber++
		timeSincePlayerLastFired++
	}

	stopSound()
	yaroze.ResetGraph(3)
}

// newGame starts over from the first level with five lives and no score.
func newGame() {
	level = 1
	numberOfLives = 5
	currentScore.points = 0
	setUpLevel1()
}

// crudestPause waits n frames.
func crudestPause(n int) {
	for range n {
		yaroze.VSync(0)
	}
}

// setUpLevel1 sets up a level, which are all alike: the ship still at the
// middle of the bottom, and four rows of eight invaders moving right.
func setUpLevel1() {
	destroyAllInvaders()

	playersMovement.x = (minX + screenWidth/2) * fix
	playersMovement.y = (maxY - int32(playersSprite.H)) * fix
	playersMovement.dx = 0
	playersMovement.dy = 0

	for i := int32(0); i < 4; i++ {
		for j := int32(0); j < 8; j++ {
			id := createNewInvader(invader1 + i)
			if id < 0 {
				return
			}
			invaders[id].m.x = (minX + 20 + 20*j) * fix
			invaders[id].m.y = (minY + 20 + 20*i) * fix
			invaders[id].m.dx = 1 * fix
		}
	}
}

func renewLevel() {
	destroyAllShots()
	setUpLevel1()
}

// dealWithEnd shows the score and the high score table under a heading, and
// reports whether the player wants another game: Cross for one, L1 and R1
// together to quit, either taking effect a second after it is pressed.
func dealWithEnd(heading string) bool {
	position := dealWithNewScore(&currentScore, highScoreTable[:])

	newGame := false
	frameNow, whenToLeaveLoop := 0, -1
	exitSet := false
	for {
		padStatus = yaroze.PadRead()
		if padStatus&yaroze.PADL1 != 0 && padStatus&yaroze.PADR1 != 0 && !exitSet {
			newGame = false
			whenToLeaveLoop = frameNow + 60
			exitSet = true
		}
		if padStatus&yaroze.PADRdown != 0 && !exitSet {
			newGame = true
			whenToLeaveLoop = frameNow + 60
			exitSet = true
		}

		active := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[active][0]))
		yaroze.GsClearOt(0, 0, &worldOT[active])
		yaroze.GsSortBoxFill(&backgroundRectangle, &worldOT[active], 0)
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(60, 120, 120, &worldOT[active])
		yaroze.GsDrawOt(&worldOT[active])

		text.Reset()
		text.WriteString(heading).WriteString("\n\nYour final score was: ").WriteInt(int(currentScore.points)).WriteString("\n\n")
		if position != -1 {
			text.WriteString("New high score! position ").WriteInt(position + 1).WriteString("\n\n")
		}
		text.WriteString("High Score Table\n")
		for i := range highScoreTable {
			text.WriteString(highScoreTable[i].name).WriteString("    ").WriteInt(int(highScoreTable[i].points)).WriteString("\n")
		}
		text.WriteString("\npress <R-down> for new game\npress L1 and R1 to quit\n").Print()
		yaroze.FntFlush(-1)

		frameNow++
		if whenToLeaveLoop == frameNow {
			break
		}
	}
	return newGame
}

// The names the high score table starts with.
var startingNames = [5]string{"AAAAAAAA", "BBBBBBBB", "CCCCCCCC", "DDDDDDDD", "EEEEEEEE"}

func clearScores() {
	currentScore = score{name: "Player One", points: 0}
	for i := range highScoreTable {
		highScoreTable[i] = score{name: startingNames[i], points: uint32(5-i) * 10000}
	}
}

// dealWithNewScore puts a score into the table, which is in descending
// order, if it beats one there, and returns its position, or -1 if it does
// not.
func dealWithNewScore(last *score, table []score) int {
	highest := len(table)
	for i := len(table) - 1; i >= 0; i-- {
		if last.points > table[i].points {
			highest = i
		}
	}
	if highest == len(table) {
		return -1
	}
	for i := len(table) - 2; i >= highest; i-- {
		table[i+1] = table[i]
	}
	table[highest] = *last
	return highest
}

func initialiseAll() bool {
	yaroze.GsInitGraph(320, 240, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)

	// The screen offset, which would change for PAL.
	yaroze.GsDISPENV().Screen.X = screenXOffset
	yaroze.GsDISPENV().Screen.Y = screenYOffset

	for i := range 2 {
		worldOT[i].Length = otLength
		worldOT[i].Org = &otTags[i][0]
	}

	yaroze.PadInit()

	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 200, 0, 512)

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return false
	}

	initialiseObjects()
	initialiseTexturesAndSprites()
	clearBackground()

	level = 1
	numberOfLives = 5
	setUpLevel1()
	clearScores()

	initSound()
	playSound()
	return true
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		active := yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[active][0]))
		yaroze.GsClearOt(0, 0, &worldOT[active])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(60, 120, 120, &worldOT[active])
		yaroze.GsDrawOt(&worldOT[active])
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

func initGsSprite(s *yaroze.GsSPRITE) {
	*s = yaroze.GsSPRITE{R: 0x80, G: 0x80, B: 0x80, Scalex: yaroze.ONE, Scaley: yaroze.ONE}
}

func initialiseObjects() {
	playersMovement = movement{w: 44, h: 24}
	initGsSprite(&playersSprite)

	for i := range invaders {
		invaders[i] = invader{id: int32(i), typ: none, m: movement{w: 16, h: 16}}
		initGsSprite(&invaders[i].sprite)
	}
	for i := range shots {
		shots[i] = shot{id: int32(i), typ: none, m: movement{w: 8, h: 8}}
		initGsSprite(&shots[i].sprite)
	}
}

// createNewInvader brings the first dead invader to life, of a type, and
// returns its ID, or -1 if they are all alive.
func createNewInvader(typ int32) int32 {
	for i := range invaders {
		if !invaders[i].alive {
			invaders[i].alive = true
			invaders[i].typ = typ
			return int32(i)
		}
	}
	return -1
}

func destroyInvader(id int32) {
	invaders[id].alive = false
	invaders[id].typ = none
}

func destroyAllInvaders() {
	for i := range invaders {
		invaders[i].id = int32(i)
		invaders[i].alive = false
		invaders[i].typ = none
		invaders[i].m = movement{w: 16, h: 16}
	}
}

func countNumberOfInvaders() int32 {
	var count int32
	for i := range invaders {
		if invaders[i].alive {
			count++
		}
	}
	return count
}

// findIDOfNthInvader returns the ID of the nth living invader, counting
// from one, or -1 if there are fewer.
func findIDOfNthInvader(n int32) int32 {
	var count int32
	for i := range invaders {
		if invaders[i].alive {
			count++
			if count == n {
				return int32(i)
			}
		}
	}
	return -1
}

// createNewShot takes the first free shot for a side, and returns its ID,
// or -1 if they are all in flight.
func createNewShot(typ int32) int32 {
	for i := range shots {
		if shots[i].typ == none {
			shots[i].typ = typ
			return int32(i)
		}
	}
	return -1
}

func destroyShot(id int32) {
	shots[id].typ = none
}

func destroyAllShots() {
	for i := range shots {
		shots[i].typ = none
	}
}

// Sprite attribute bits.
const (
	eightBitTexture  = 1 << 24
	semiTransparent1 = 1 << 29
	semiTransparent2 = 1 << 30
)

// initialiseTexturesAndSprites loads the three TIMs into video memory,
// where the demo chose to put them, and sets the sprites up to show them:
// the ship and the invaders are 8-bit and so twice as wide in the sprite
// as in the TIM, the shot 4-bit and four times, and the invaders and the
// shots are then scaled down to size.
func initialiseTexturesAndSprites() {
	var image1, image2, image3 yaroze.GsIMAGE

	// The player's ship.
	image1 = shipTIM.Image()
	loadTexture(&image1, 320, 0)
	texturePageID1 := yaroze.GetTPage(1, 0, 320, 0)
	playersSprite.W = image1.Pw * 2
	playersSprite.H = image1.Ph
	playersSprite.Tpage = texturePageID1
	playersSprite.Cx = image1.Cx
	playersSprite.Cy = image1.Cy
	playersSprite.Attribute |= eightBitTexture | semiTransparent1 | semiTransparent2

	// The invaders.
	image2 = invaderTIM.Image()
	loadTexture(&image2, 320, 256)
	texturePageID2 := yaroze.GetTPage(1, 0, 320, 256)
	for i := range invaders {
		s := &invaders[i].sprite
		s.W = image2.Pw * 2
		s.H = image2.Ph
		s.Tpage = texturePageID2
		s.Cx = image2.Cx
		s.Cy = image2.Cy
		s.Attribute |= eightBitTexture | semiTransparent1 | semiTransparent2
		s.Scalex = yaroze.ONE / 2
		s.Scaley = yaroze.ONE / 2
	}

	// The shots.
	image3 = shotTIM.Image()
	loadTexture(&image3, 640, 0)
	texturePageID3 := yaroze.GetTPage(0, 0, 640, 0)
	for i := range shots {
		s := &shots[i].sprite
		s.W = image3.Pw * 4
		s.H = image3.Ph
		s.Tpage = texturePageID3
		s.Cx = image3.Cx
		s.Cy = image3.Cy
		s.Scalex = yaroze.ONE / 4
		s.Scaley = yaroze.ONE / 4
	}
}

// loadTexture loads a TIM's pixels into video memory at x, y, and its
// palette where the TIM says.
func loadTexture(image *yaroze.GsIMAGE, x, y int16) {
	rect := yaroze.RECT{X: x, Y: y, W: int16(image.Pw), H: int16(image.Ph)}
	yaroze.LoadImage(&rect, image.Pixel)
	yaroze.DrawSync(0)
	rect = yaroze.RECT{X: image.Cx, Y: image.Cy, W: int16(image.Cw), H: int16(image.Ch)}
	yaroze.LoadImage(&rect, image.Clut)
	yaroze.DrawSync(0)
}

// clearBackground sets up the box behind everything: black, and
// semi-transparent, which takes the hard black edges off the sprites.
func clearBackground() {
	backgroundRectangle = yaroze.GsBOXF{
		Attribute: semiTransparent2,
		X:         0, Y: 0, W: screenWidth, H: screenHeight,
		R: 0, G: 0, B: 0,
	}
}

func dealWithControllerPad() {
	padStatus = yaroze.PadRead()

	if padStatus&yaroze.PADselect != 0 && padStatus&yaroze.PADstart != 0 {
		quit = true
		return
	}

	for padStatus&yaroze.PADL1 != 0 { // pause
		padStatus = yaroze.PadRead()
	}

	if padStatus&yaroze.PADLleft != 0 {
		playersMovement.dx = max(playersMovement.dx-1*fix, -maxXSpeed*fix)
	}
	if padStatus&yaroze.PADLright != 0 {
		playersMovement.dx = min(playersMovement.dx+1*fix, maxXSpeed*fix)
	}

	if padStatus&yaroze.PADRup != 0 && timeSincePlayerLastFired > 10 {
		if id := createNewShot(playersShot); id >= 0 {
			// Above the ship, moving up at two and a half pixels a frame.
			shots[id].m.x = playersMovement.x + 24*fix
			shots[id].m.y = playersMovement.y - 2*fix
			shots[id].m.dx = 0
			shots[id].m.dy = -5 * fix / 2
			timeSincePlayerLastFired = 0
		}
	}
}

// sendAllInvadersTheOtherWay turns the invaders round and drops them a
// line.
func sendAllInvadersTheOtherWay() {
	for i := range invaders {
		if invaders[i].alive {
			invaders[i].m.dx = -invaders[i].m.dx
			invaders[i].m.y += 1 * fix
		}
	}
}

// repositionAllInvadersOnTheLeft slides the invaders over so that the
// leftmost is at the left wall.
func repositionAllInvadersOnTheLeft() {
	leastX := int32(1500 * fix)
	for i := range invaders {
		if invaders[i].alive && invaders[i].m.x < leastX {
			leastX = invaders[i].m.x
		}
	}
	if leastX != 1500*fix {
		amount := leastX - minX*fix
		for i := range invaders {
			if invaders[i].alive {
				invaders[i].m.x -= amount
			}
		}
	}
}

// rect is where a movement is on the screen, in pixels.
func rect(m *movement, w, h int32) yaroze.RECT {
	return yaroze.RECT{X: int16(m.x / fix), Y: int16(m.y / fix), W: int16(w), H: int16(h)}
}

// loseALife puts the ship back at the start, clears the shots and slides
// the invaders to the left, after a pause.
func loseALife() {
	crudestPause(30)
	numberOfLives--
	destroyAllShots()
	playersMovement.x = (minX + screenWidth/2) * fix
	playersMovement.y = (maxY - int32(playersSprite.H)) * fix
	playersMovement.dx = 0
	playersMovement.dy = 0
	repositionAllInvadersOnTheLeft()
}

// handleAllObjects moves everything and deals with what collides.
func handleAllObjects() {
	playersMovement.x += playersMovement.dx
	playersMovement.y += playersMovement.dy

	// The ship stops at the walls.
	if playersMovement.x > (maxX-playersMovement.w)*fix {
		playersMovement.x = (maxX - playersMovement.w) * fix
		playersMovement.dx = 0
	} else if playersMovement.x < minX*fix {
		playersMovement.x = minX * fix
		playersMovement.dx = 0
	}
	playersSprite.X = int16(playersMovement.x / fix)
	playersSprite.Y = int16(playersMovement.y / fix)
	player := rect(&playersMovement, playersMovement.w, playersMovement.h)

	// An invader about to leave the screen sends the whole lot the other
	// way.
	for i := range invaders {
		inv := &invaders[i]
		if !inv.alive {
			continue
		}
		if inv.m.x+inv.m.dx > (maxX-16)*fix || inv.m.x+inv.m.dx < minX*fix {
			sendAllInvadersTheOtherWay()
			break
		}
	}

	for i := range invaders {
		inv := &invaders[i]
		if !inv.alive {
			continue
		}
		inv.m.x += inv.m.dx
		inv.m.y += inv.m.dy
		if inv.m.y > (maxY-16)*fix {
			quit = true
			return
		}
		inv.sprite.X = int16(inv.m.x / fix)
		inv.sprite.Y = int16(inv.m.y / fix)

		if rectanglesOverlap(&player, rect(&inv.m, 16, 16)) {
			destroyInvader(int32(i))
			loseALife()
		}
	}

	handleInvadersShooting()

	for i := range shots {
		s := &shots[i]
		if s.typ == none {
			continue
		}
		s.m.x += s.m.dx
		s.m.y += s.m.dy
		s.sprite.X = int16(s.m.x / fix)
		s.sprite.Y = int16(s.m.y / fix)

		if s.m.y > (maxY-s.m.h)*fix || s.m.y < minY*fix {
			destroyShot(int32(i))
		}

		shotRect := rect(&s.m, 8, 8)
		if s.typ == invadersShot {
			// An invader's shot can only hit the ship.
			if rectanglesOverlap(&player, shotRect) {
				destroyShot(int32(i))
				loseALife()
			}
		} else {
			// The player's can only hit invaders.
			for j := range invaders {
				inv := &invaders[j]
				if inv.alive && rectanglesOverlap(&shotRect, rect(&inv.m, 16, 16)) {
					destroyShot(int32(i))
					currentScore.points += uint32(50 * inv.typ)
					destroyInvader(int32(j))
					break
				}
			}
		}
	}
}

// handleInvadersShooting has one invader, chosen at random, fire one frame
// in sixty or so, straight down.
func handleInvadersShooting() {
	if yaroze.Rand()%60 != 0 {
		return
	}
	n := countNumberOfInvaders()
	if n == 0 {
		return
	}
	id := findIDOfNthInvader(yaroze.Rand()%n + 1)
	shotID := createNewShot(invadersShot)
	if id < 0 || shotID < 0 {
		return
	}
	shots[shotID].m.x = invaders[id].m.x + 8*fix
	shots[shotID].m.y = invaders[id].m.y + 17*fix
	shots[shotID].m.dy = 1 * fix
}

func rectanglesOverlap(first *yaroze.RECT, second yaroze.RECT) bool {
	return xOverlap(first, &second) && yOverlap(first, &second)
}

func xOverlap(first, second *yaroze.RECT) bool {
	if first.X >= second.X {
		return first.X < second.X+second.W
	}
	return first.X+first.W > second.X
}

func yOverlap(first, second *yaroze.RECT) bool {
	if first.Y >= second.Y {
		return first.Y < second.Y+second.H
	}
	return first.Y+first.H > second.Y
}

// initSound transfers the sound bank to the SPU and opens the sequence,
// remembering whether both worked so that nothing is played or closed
// otherwise.
func initSound() {
	var ok bool
	if bank, ok = yaroze.OpenBank(vh.Data(), vb.Data()); !ok {
		soundSuccess = false
		return
	}
	song, soundSuccess = bank.Sequence(seqData[:])
}

func playSound() {
	if soundSuccess {
		yaroze.SsSetMVol(mvol, mvol)
		song.SetVolume(svol, svol)
		song.Play(yaroze.SSPLAY_INFINITY)
	}
}

func stopSound() {
	if soundSuccess {
		song.Stop()
		yaroze.VSync(0)
		yaroze.VSync(0)
		song.Close()
		bank.Close()
	}
}
