// Breakout in sprites, ten levels of it, with music and a high score table.
//
// A port of L. Evans's Net Yaroze demo "breakout" (Sony, 1996): Breakout in
// sprites. A bat slides along the bottom of the screen under a wall of
// bricks, five rows deep and staggered by half a brick, and a ball bounces
// between them, off the walls and the ceiling, and off the bat, which
// nudges it sideways by three quarters of the bat's own speed. Each brick
// goes with one hit and scores fifty; clearing them all starts the next of
// ten levels, all alike, and letting the ball past the bat costs one of
// five lives. When they are gone, or the tenth level is cleared, the score
// and the high score table are shown. The bat and the bricks are textures
// off the disc; the ball is the SDK's ball pattern from balltex.h, in one of
// its thirty-two shades, loaded straight from a table in the program, as
// the check sample's are. Behind everything a semi-transparent black box
// softens the sprites' black edges, the library's font prints the frame,
// the lives and the score, and music plays from a sequence off the disc.
//
// Differences from the original:
//
//   - The textures and the sound are read off the disc into the program's
//     own memory, where the original's batch script sent them to fixed
//     addresses. The sound comes from the check sample's files.
//   - The missing data is reported, where the original would hang.
//
// Nothing allocates after startup.
//
// # Controls
//
// Left and right move the bat, and R1 and R2 make it faster and slower. L1
// pauses for as long as it is held, and Start and Select together quit. On
// the score screen Cross starts a new game and L1 and R1 together quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	mvol = 127
	svol = 127

	maxBricks          = 200
	maxObjects         = maxBricks + 2 // the bricks, the bat and the ball
	maxSizeofPrimitive = 24            // GPU packet space allowed per primitive

	screenWidth  = 320
	screenHeight = 240

	// The walls.
	minX = 16
	maxX = 288
	minY = 30
	maxY = 196

	// The screen offset that makes the picture sit right.
	screenXOffset = 0
	screenYOffset = 16

	// Not a 3D game, so one place in the ordering table does.
	otLength = 1

	maxPlayerXSpeed = 4
	maxBallXSpeed   = 5
	maxBallYSpeed   = 5

	none = 32766 // an empty type field

	fullBrick = 100
	halfBrick = 101
)

// Where a rectangle touches another, from the first's point of view.
const (
	top = 1 + iota
	bottom
	left
	right
	topLeft
	topRight
	bottomLeft
	bottomRight
	middle
	overlap
)

// The sound and the textures, read into the program's own memory, in
// buffers of whole sectors.
var (
	vhData    [(60448 + 2047) / 2048 * 512]uint32
	vbData    [(401360 + 2047) / 2048 * 512]uint32
	seqData   [(17341 + 2047) / 2048 * 512]uint32
	batData   [(864 + 2047) / 2048 * 512]uint32
	brickData [(4116 + 2047) / 2048 * 512]uint32

	vh        = disc.Register("DATA/SOUND/STD0.VH", "data/STD0.VH").Into(vhData[:])
	vb        = disc.Register("DATA/SOUND/STD0.VB", "data/STD0.VB").Into(vbData[:])
	seqFile   = disc.Register("DATA/SOUND/GOGO.SEQ", "data/GOGO.SEQ").Into(seqData[:])
	batFile   = disc.Register("DATA/BREAK3.TIM", "data/BREAK3.TIM").Into(batData[:])
	brickFile = disc.Register("DATA/ODEN3.TIM", "data/ODEN3.TIM").Into(brickData[:])
	files     = []*disc.File{&vh, &vb, &seqFile, &batFile, &brickFile}

	batTIM   = yaroze.TIM(batData[:])
	brickTIM = yaroze.TIM(brickData[:])
)

// A movement is where something is, how big it is and how it moves, in
// pixels.
type movement struct {
	x, y   int32
	w, h   int32
	dx, dy int32
}

type brick struct {
	id        int32
	alive     bool
	typ       int32 // fullBrick, halfBrick or none
	toughness int32 // hits before it goes
	sprite    yaroze.GsSPRITE
	m         movement
}

type score struct {
	name   string
	points uint32
}

var (
	playersMovement movement
	playersSprite   yaroze.GsSPRITE
	playerSpeed     int32

	ballsMovement movement
	ballsSprite   yaroze.GsSPRITE

	backgroundRectangle yaroze.GsBOXF

	worldOT       [2]yaroze.GsOT
	otTags        [2][1 << otLength]yaroze.GsOT_TAG
	gpuPacketArea [2][maxObjects * maxSizeofPrimitive / 4]uint32

	padStatus yaroze.Buttons
	quit      bool

	bricks [maxBricks]brick

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
		yaroze.GsSortSprite(&ballsSprite, &worldOT[active], 0)
		for i := range bricks {
			if bricks[i].alive {
				yaroze.GsSortSprite(&bricks[i].sprite, &worldOT[active], 0)
			}
		}

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(60, 120, 120, &worldOT[active])
		yaroze.GsDrawOt(&worldOT[active])

		text.Reset()
		text.WriteString("Breakout!\nFrame number: ").WriteInt(int(frameNumber)).
			WriteString("\nLives: ").WriteInt(int(numberOfLives)).
			WriteString("\nScore: ").WriteInt(int(currentScore.points)).WriteString("\n").Print()
		yaroze.FntFlush(-1)

		if countNumberOfBricks() == 0 { // time for the next level
			crudestPause(30)
			level++
			if level > 10 {
				if dealWithEnd("You have finished ten levels") {
					newGame()
				} else {
					quit = true
				}
			} else {
				setUpLevel1()
			}
		}

		if numberOfLives <= 0 { // the player has lost
			crudestPause(30)
			if dealWithEnd("You have died") {
				newGame()
			} else {
				quit = true
			}
		}

		frameNumber++
	}

	stopSound()
	yaroze.ResetGraph(3)
}

// newGame starts over from the first level with five lives and no score.
func newGame() {
	level = 1
	numberOfLives = 5
	destroyAllBricks()
	currentScore.points = 0
	setUpLevel1()
}

// crudestPause waits n frames.
func crudestPause(n int) {
	for range n {
		yaroze.VSync(0)
	}
}

// setUpLevel1 sets up a level, which are all alike: the bat and the ball
// at their starts, three rows of fifteen bricks, and two rows of fourteen
// between them with a half brick at each end.
func setUpLevel1() {
	restartPlayersBat()
	restartTheBall()

	for i := int32(0); i < 3; i++ {
		for j := int32(0); j < 15; j++ {
			if id := createNewBrick(fullBrick, 1); id >= 0 {
				bricks[id].m.x = minX + 18*j
				bricks[id].m.y = minY + 29 + 18*i
			}
		}
	}
	for i := int32(0); i < 2; i++ {
		for j := int32(0); j < 14; j++ {
			if id := createNewBrick(fullBrick, 1); id >= 0 {
				bricks[id].m.x = minX + 9 + 18*j
				bricks[id].m.y = minY + 38 + 18*i
			}
		}
		if id := createNewBrick(halfBrick, 1); id >= 0 {
			bricks[id].m.x = minX
			bricks[id].m.y = minY + 38 + 18*i
		}
		if id := createNewBrick(halfBrick, 1); id >= 0 {
			bricks[id].m.x = minX + 29*9
			bricks[id].m.y = minY + 38 + 18*i
		}
	}
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
		highScoreTable[i] = score{name: startingNames[i], points: uint32(5-i) * 1000}
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
	playersMovement = movement{w: 40, h: 8}
	initGsSprite(&playersSprite)
	playerSpeed = 3

	ballsMovement = movement{w: 7, h: 7}
	initGsSprite(&ballsSprite)

	for i := range bricks {
		bricks[i] = brick{id: int32(i), typ: none, toughness: none}
		initGsSprite(&bricks[i].sprite)
	}
}

// createNewBrick brings the first dead brick to life, of a type and a
// toughness, and returns its ID, or -1 if they are all alive.
func createNewBrick(typ, toughness int32) int32 {
	for i := range bricks {
		b := &bricks[i]
		if b.alive {
			continue
		}
		b.alive = true
		b.typ = typ
		b.toughness = toughness
		if typ == fullBrick {
			b.m.w, b.m.h = 16, 8
		} else {
			b.m.w, b.m.h = 8, 8
			b.sprite.Scalex = yaroze.ONE / 8
		}
		return int32(i)
	}
	return -1
}

func destroyBrick(id int32) {
	bricks[id].alive = false
	bricks[id].typ = none
	bricks[id].toughness = none
}

func countNumberOfBricks() int32 {
	var count int32
	for i := range bricks {
		if bricks[i].alive {
			count++
		}
	}
	return count
}

func destroyAllBricks() {
	for i := range bricks {
		if bricks[i].alive {
			bricks[i] = brick{id: int32(i), typ: none, toughness: none, sprite: bricks[i].sprite}
		}
	}
}

// restartTheBall puts the ball at the center of the screen, moving down.
func restartTheBall() {
	ballsMovement.x = screenWidth/2 - ballsMovement.w/2
	ballsMovement.y = screenHeight/2 - ballsMovement.h/2
	ballsMovement.dx = 0
	ballsMovement.dy = 1
}

// restartPlayersBat puts the bat at the middle of the bottom, still.
func restartPlayersBat() {
	playersMovement.x = screenWidth/2 - playersMovement.w/2
	playersMovement.y = maxY - playersMovement.h
	playersMovement.dx = 0
	playersMovement.dy = 0
}

// Sprite attribute bits.
const (
	eightBitTexture   = 1 << 24
	sixteenBitTexture = 1 << 25
	semiTransparent1  = 1 << 29
	semiTransparent2  = 1 << 30
)

// initialiseTexturesAndSprites loads the textures into video memory, where
// the demo chose to put them, and sets the sprites up to show them: the bat
// is 8-bit and so twice as wide in the sprite as in the TIM; the brick is
// 16-bit, drawn at a quarter of its size; and the ball is the 4-bit pattern
// from the table, with the fifth of its palettes, drawn at half size.
func initialiseTexturesAndSprites() {
	var image1, image2 yaroze.GsIMAGE

	// The bat.
	image1 = batTIM.Image()
	rect := yaroze.RECT{X: 320, Y: 0, W: int16(image1.Pw), H: int16(image1.Ph)}
	yaroze.LoadImage(&rect, image1.Pixel)
	yaroze.DrawSync(0)
	rect = yaroze.RECT{X: image1.Cx, Y: image1.Cy, W: int16(image1.Cw), H: int16(image1.Ch)}
	yaroze.LoadImage(&rect, image1.Clut)
	yaroze.DrawSync(0)
	texturePageID1 := yaroze.GetTPage(1, 0, 320, 0)
	playersSprite.W = image1.Pw * 2
	playersSprite.H = image1.Ph
	playersSprite.Tpage = texturePageID1
	playersSprite.Cx = image1.Cx
	playersSprite.Cy = image1.Cy
	playersSprite.Attribute |= eightBitTexture | semiTransparent1 | semiTransparent2

	// The bricks.
	image2 = brickTIM.Image()
	rect = yaroze.RECT{X: 640, Y: 0, W: int16(image2.Pw), H: int16(image2.Ph)}
	yaroze.LoadImage(&rect, image2.Pixel)
	yaroze.DrawSync(0)
	texturePageID2 := yaroze.GetTPage(2, 0, 640, 0)
	for i := range bricks {
		s := &bricks[i].sprite
		s.W = image2.Pw
		s.H = image2.Ph
		s.Tpage = texturePageID2
		s.Cx = image2.Cx
		s.Cy = image2.Cy
		s.Attribute |= sixteenBitTexture | semiTransparent1 | semiTransparent2
		s.Scalex = yaroze.ONE / 4
		s.Scaley = yaroze.ONE / 4
	}

	// The ball.
	rect = yaroze.RECT{X: 320, Y: 256, W: 16 / 4, H: 16}
	yaroze.LoadImage(&rect, &demo.Ball16x16[0])
	yaroze.DrawSync(0)
	rect = yaroze.RECT{X: 0, Y: 481, W: 16, H: 1}
	yaroze.LoadImage(&rect, &demo.BallColor[4][0])
	yaroze.DrawSync(0)
	texturePageID3 := yaroze.GetTPage(0, 0, 320, 256)
	ballsSprite.W = 16
	ballsSprite.H = 16
	ballsSprite.Tpage = texturePageID3
	ballsSprite.Cx = 0
	ballsSprite.Cy = 481
	ballsSprite.Scalex = yaroze.ONE / 2
	ballsSprite.Scaley = yaroze.ONE / 2
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
		playersMovement.dx = max(playersMovement.dx-playerSpeed, -maxPlayerXSpeed)
	}
	if padStatus&yaroze.PADLright != 0 {
		playersMovement.dx = min(playersMovement.dx+playerSpeed, maxPlayerXSpeed)
	}

	if padStatus&yaroze.PADR1 != 0 {
		playerSpeed++
	}
	if padStatus&yaroze.PADR2 != 0 {
		playerSpeed--
	}
}

// rectOf is where a movement is on the screen.
func rectOf(m *movement) yaroze.RECT {
	return yaroze.RECT{X: int16(m.x), Y: int16(m.y), W: int16(m.w), H: int16(m.h)}
}

func sameSign(a, b int32) bool { return (a >= 0) == (b >= 0) }

// nudge is the bat's effect on the ball: three quarters of the bat's
// speed, added to the ball's in quarters so that the sum rounds as the
// original's floating point does.
func nudge(ballDX, batDX int32) int32 { return (4*ballDX + 3*batDX) / 4 }

func clamp(v, lo, hi int32) int32 { return max(lo, min(v, hi)) }

// handleAllObjects moves the ball and the bat, bouncing the ball off the
// bat, the bricks, the walls and the ceiling. The original warns that it
// was written quickly to make the game work, not as an example of how to do
// collisions.
func handleAllObjects() {
	player := rectOf(&playersMovement)
	ball := rectOf(&ballsMovement)
	bm := &ballsMovement
	ballMoved := false

	if rectanglesTouch(&player, &ball) || rectanglesOverlap(&player, &ball) {
		// The ball hits the bat, which only nudges it sideways.
		if rectanglesOverlap(&player, &ball) {
			moveBallOutOfBat()
		}
		if bm.y > maxY-playersMovement.h-bm.h {
			// Below the top of the bat: off its side.
			switch {
			case bm.dx == 0:
				bm.dx = playersMovement.dx
			case sameSign(bm.dx, playersMovement.dx): // the bat overtaking the ball
				bm.dx = nudge(bm.dx, playersMovement.dx)
			default: // moving in opposite directions
				bm.dx = nudge(-bm.dx, playersMovement.dx)
			}
		} else {
			// Off the top of the bat, nudged along by the bat's speed.
			bm.dy = -bm.dy
			bm.dx = nudge(bm.dx, playersMovement.dx)
		}
		bm.dx = clamp(bm.dx, -maxBallXSpeed, maxBallXSpeed)
		bm.dy = clamp(bm.dy, -maxBallYSpeed, maxBallYSpeed)
		bm.x += bm.dx
		bm.y += bm.dy
		ballMoved = true
	} else {
		// Does the ball hit a brick where it is about to be? Reflecting
		// and moving are done in one go, from the distance to the brick
		// and what is left of the move after it.
		ball = yaroze.RECT{X: int16(bm.x + bm.dx), Y: int16(bm.y + bm.dy), W: int16(bm.w), H: int16(bm.h)}
		for i := range bricks {
			b := &bricks[i]
			if !b.alive {
				continue
			}
			b.sprite.X = int16(b.m.x)
			b.sprite.Y = int16(b.m.y)
			brick := rectOf(&b.m)
			if !rectanglesOverlap(&ball, &brick) && !rectanglesTouch(&ball, &brick) {
				continue
			}
			switch findWhereRectanglesTouch(&ball, &brick) {
			case left:
				xInitialDistance := bm.x - (b.m.x + b.m.w)
				xLeftOver := -bm.dx - xInitialDistance
				bm.x += bm.dx + 2*xLeftOver
				bm.dx = -bm.dx
				bm.y += bm.dy
			case right:
				xInitialDistance := b.m.x - (bm.x + bm.w)
				xLeftOver := bm.dx - xInitialDistance
				bm.x += bm.dx - 2*xLeftOver
				bm.dx = -bm.dx
				bm.y += bm.dy
			case top, topLeft, topRight:
				yInitialDistance := bm.y - (b.m.y + b.m.h)
				yLeftOver := -bm.dy - yInitialDistance
				bm.y += bm.dy + 2*yLeftOver
				bm.dy = -bm.dy
				bm.x += bm.dx
			case bottom, bottomRight, bottomLeft:
				yInitialDistance := b.m.y - (bm.y + bm.h)
				yLeftOver := bm.dy - yInitialDistance
				bm.y += bm.dy - 2*yLeftOver
				bm.dy = -bm.dy
				bm.x += bm.dx
			}
			ballMoved = true
			b.toughness--
			if b.toughness <= 0 {
				destroyBrick(int32(i))
				currentScore.points += 50
			}
			break // the ball only hits one brick at a time
		}
	}

	bm.dx = clamp(bm.dx, -maxBallXSpeed, maxBallXSpeed)
	bm.dy = clamp(bm.dy, -maxBallYSpeed, maxBallYSpeed)

	if !ballMoved { // the ball hit nothing
		bm.x += bm.dx
		bm.y += bm.dy
	}

	// Perfectly elastic collisions off the walls and the ceiling.
	if bm.x > maxX-bm.w { // right wall
		bm.x -= 2 * (bm.x - (maxX - bm.w))
		bm.dx = -bm.dx
	}
	if bm.x < minX { // left wall
		bm.x += 2 * (minX - bm.x)
		bm.dx = -bm.dx
	}
	if bm.y < minY { // ceiling
		bm.y += 2 * (minY - bm.y)
		bm.dy = -bm.dy
	}
	if bm.y > maxY-bm.h { // the floor: a life lost
		numberOfLives--
		crudestPause(30)
		restartTheBall()
		restartPlayersBat()
	}

	ballsSprite.X = int16(bm.x - 1)
	ballsSprite.Y = int16(bm.y - 1)

	// The bat, which has no momentum, stops at the walls.
	playersMovement.x += playersMovement.dx
	playersMovement.y += playersMovement.dy
	playersMovement.dx = 0
	playersMovement.dy = 0
	if playersMovement.x > maxX-playersMovement.w {
		playersMovement.x = maxX - playersMovement.w
	} else if playersMovement.x < minX {
		playersMovement.x = minX
	}
	playersSprite.X = int16(playersMovement.x)
	playersSprite.Y = int16(playersMovement.y)
}

// moveBallOutOfBat pushes the ball up out of the bat.
func moveBallOutOfBat() {
	if ballsMovement.y+ballsMovement.h > playersMovement.y {
		ballsMovement.y = playersMovement.y - ballsMovement.h
	}
}

// Rectangle intersections, which the original says could be optimized.

func rectanglesOverlap(first, second *yaroze.RECT) bool {
	return xOverlap(first, second) && yOverlap(first, second)
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

func rectanglesTouch(first, second *yaroze.RECT) bool {
	return (xTouch(first, second) && yOverlap(first, second)) ||
		(yTouch(first, second) && xOverlap(first, second))
}

func xTouch(first, second *yaroze.RECT) bool {
	switch {
	case first.X > second.X:
		return first.X == second.X+second.W
	case first.X == second.X:
		return false
	}
	return first.X+first.W == second.X
}

func yTouch(first, second *yaroze.RECT) bool {
	switch {
	case first.Y > second.Y:
		return first.Y == second.Y+second.H
	case first.Y == second.Y:
		return false
	}
	return first.Y+first.H == second.Y
}

// findWhereRectanglesTouch returns the edge or corner of the first
// rectangle that touches the second, overlap if they overlap, or none if
// they are apart.
func findWhereRectanglesTouch(first, second *yaroze.RECT) int32 {
	var xTouch, yTouch int32
	switch {
	case first.X > second.X:
		switch {
		case first.X == second.X+second.W:
			xTouch = left
		case first.X < second.X+second.W:
			xTouch = middle
		default:
			xTouch = none
		}
	case first.X == second.X:
		xTouch = middle
	default:
		switch {
		case first.X+first.W == second.X:
			xTouch = right
		case first.X+first.W > second.X:
			xTouch = middle
		default:
			xTouch = none
		}
	}

	switch {
	case first.Y > second.Y:
		switch {
		case first.Y == second.Y+second.H:
			yTouch = top
		case first.Y < second.Y+second.H:
			yTouch = middle
		default:
			yTouch = none
		}
	case first.Y == second.Y:
		yTouch = middle
	default:
		switch {
		case first.Y+first.H == second.Y:
			yTouch = bottom
		case first.Y+first.H > second.Y:
			yTouch = middle
		default:
			yTouch = none
		}
	}

	switch xTouch {
	case left:
		switch yTouch {
		case top:
			return topLeft
		case bottom:
			return bottomLeft
		case middle:
			return left
		}
	case right:
		switch yTouch {
		case top:
			return topRight
		case bottom:
			return bottomRight
		case middle:
			return right
		}
	case middle:
		switch yTouch {
		case top:
			return top
		case bottom:
			return bottom
		case middle:
			return overlap
		}
	}
	return none
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
