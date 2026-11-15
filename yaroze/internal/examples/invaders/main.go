// Space Invaders in sprites, on the yaroze package's Go API: examples/invaders
// written against the Go library's own surface rather than the library's C
// names. It draws the same frames, which the tests check against the
// original's picture.
//
// A port of L. Evans's Net Yaroze demo "invaders" (Sony, 1996). Four rows
// of eight invaders march across the screen, dropping a line each time
// they reach a side, and one of them fires back now and then; the
// player's ship slides along the bottom and shoots upwards, no more than
// once every ten frames. Clearing the screen starts the next of ten
// levels, and an invader or a shot reaching the ship costs one of five
// lives. When they are gone, or the tenth level is cleared, the score and
// the high score table are shown. Behind the sprites a semi-transparent
// black box covers the screen, which the original uses to soften the
// black edges its sprites have, and the font prints the frame, the lives
// and the score. Music plays from a sequence off the disc.
//
// Nothing allocates after startup.
//
// # Controls
//
// Left and right push the ship, which has momentum, and Triangle fires.
// L1 pauses for as long as it is held, and Start and Select together
// quit. On the score screen Cross starts a new game and L1 and R1
// together quit. If the invaders reach the bottom of the screen the
// program ends.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	mvol = 127
	svol = 127

	maxInvaders = 100
	maxShots    = 250

	screenWidth  = 320
	screenHeight = 240

	// The walls.
	minX = 16
	maxX = 286
	minY = 30
	maxY = 196

	maxXSpeed = 2

	// fix is how many units make a pixel in a position or a speed.
	fix = 16

	none = 32766 // an empty type field

	// Kinds of invader; only the first is used.
	invader1 = 1

	// Whose shot it is.
	playersShot  = 1
	invadersShot = 2

	// readTimeout is how many frames one disc read may take.
	readTimeout = 300
)

// The screen: 320 by 240, two depths, a frame's packets sized for every
// sprite at once, the picture moved down to sit as the original's did.
var screen = yaroze.Screen{
	Depth:      1,
	Packets:    (maxInvaders + maxShots + 1) * 24,
	Background: yaroze.Color{R: 60, G: 120, B: 120},
	Offset:     yaroze.Point{Y: 16},
}

// The sound and the textures, read into the program's own memory, in
// buffers of whole sectors, from the original example's assets.
var (
	vhData      [(60448 + 2047) / 2048 * 512]uint32
	vbData      [(401360 + 2047) / 2048 * 512]uint32
	seqData     [(17341 + 2047) / 2048 * 512]uint32
	invaderData [(1568 + 2047) / 2048 * 512]uint32
	shipData    [(2592 + 2047) / 2048 * 512]uint32
	shotData    [(576 + 2047) / 2048 * 512]uint32

	vh          = disc.Register("ASSETS/SOUND/STD0.VH", "../../../../examples/invaders/assets/STD0.VH").Into(vhData[:])
	vb          = disc.Register("ASSETS/SOUND/STD0.VB", "../../../../examples/invaders/assets/STD0.VB").Into(vbData[:])
	seqFile     = disc.Register("ASSETS/SOUND/GOGO.SEQ", "../../../../examples/invaders/assets/GOGO.SEQ").Into(seqData[:])
	invaderFile = disc.Register("ASSETS/INVADER1.TIM", "../../../../examples/invaders/assets/INVADER1.TIM").Into(invaderData[:])
	shipFile    = disc.Register("ASSETS/SHIP1.TIM", "../../../../examples/invaders/assets/SHIP1.TIM").Into(shipData[:])
	shotFile    = disc.Register("ASSETS/FIRE04.TIM", "../../../../examples/invaders/assets/FIRE04.TIM").Into(shotData[:])
)

// A movement is where something is and how it moves: its position and
// speed in fix units, and its size in pixels.
type movement struct {
	x, y   int
	dx, dy int
	w, h   int
}

// rect is where the movement is on the screen, in pixels.
func (m *movement) rect() yaroze.Rect {
	return yaroze.Rect{
		X: m.x / fix,
		Y: m.y / fix,
		W: m.w,
		H: m.h,
	}
}

type invader struct {
	alive  bool
	typ    int
	sprite yaroze.Sprite
	m      movement
}

type shot struct {
	typ    int // playersShot, invadersShot or none
	sprite yaroze.Sprite
	m      movement
}

type score struct {
	name   string
	points int
}

var (
	player                   movement
	playersSprite            yaroze.Sprite
	timeSincePlayerLastFired int

	// background is the box behind everything: black, and
	// semi-transparent, which takes the hard black edges off the sprites.
	background = yaroze.Box{
		Mode: yaroze.Translucent,
		W:    screenWidth,
		H:    screenHeight,
	}

	text yaroze.Text
	quit bool

	invaders [maxInvaders]invader
	shots    [maxShots]shot

	level         int
	numberOfLives int

	highScoreTable [5]score
	currentScore   score

	bank yaroze.Bank
	song yaroze.Sequence
)

func main() {
	screen.Open()
	text = screen.Font(16, 16, 256, 200, 512)
	if failed := disc.LoadAll(readTimeout); failed != nil {
		fail("could not read ", failed.Name)
		return
	}
	initializeObjects()
	initializeTexturesAndSprites()
	newGame()
	clearScores()
	initSound()

	for frame := range screen.Frames() {
		// The end of a level or of the game, found as the last frame
		// was drawn, is dealt with before this one.
		if countNumberOfInvaders() == 0 {
			level++
			if level > 10 {
				if dealWithEnd("You have finished ten levels") {
					newGame()
				} else {
					break
				}
			} else {
				renewLevel()
			}
		}
		if numberOfLives <= 0 {
			if dealWithEnd("You have died") {
				newGame()
			} else {
				break
			}
		}

		dealWithControllerPad()
		if quit {
			break
		}
		handleAllObjects()
		if quit {
			break
		}

		background.Draw(frame, 0)
		playersSprite.Draw(frame, 0)
		for i := range invaders {
			if invaders[i].alive {
				invaders[i].sprite.Draw(frame, 0)
			}
		}
		for i := range shots {
			if shots[i].typ != none {
				shots[i].sprite.Draw(frame, 0)
			}
		}
		text.Print("Space Invaders!\nFrame number: ").Int(frame.Number()).
			Print("\nLives: ").Int(numberOfLives).
			Print("\nScore: ").Int(currentScore.points).Print("\n")

		timeSincePlayerLastFired++
	}

	stopSound()
	screen.Close()
}

// newGame starts over from the first level with five lives and no score.
func newGame() {
	level = 1
	numberOfLives = 5
	currentScore.points = 0
	setupLevel()
}

// setupLevel sets up a level, which are all alike: the ship still at the
// middle of the bottom, and four rows of eight invaders moving right.
func setupLevel() {
	destroyAllInvaders()

	player.x = (minX + screenWidth/2) * fix
	_, h := playersSprite.Size()
	player.y = (maxY - h) * fix
	player.dx = 0
	player.dy = 0

	for i := range 4 {
		for j := range 8 {
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
	setupLevel()
}

// dealWithEnd shows the score and the high score table under a heading,
// and reports whether the player wants another game: Cross for one, L1
// and R1 together to quit, either taking effect a second after it is
// pressed.
//
// It is a frame loop inside the game's, which allocates the range's
// iterator and the variables its body shares each time it runs, a few
// words at the end of each game; the pragma tells the build so.
//
//go:allocates
func dealWithEnd(heading string) bool {
	position := dealWithNewScore(&currentScore, highScoreTable[:])

	newGame := false
	frameNow, whenToLeaveLoop := 0, -1
	for frame := range screen.Frames() {
		if frameNow == whenToLeaveLoop {
			break
		}
		pad := yaroze.PadButtons(0)
		if whenToLeaveLoop < 0 {
			if pad.Has(yaroze.L1 | yaroze.R1) {
				newGame = false
				whenToLeaveLoop = frameNow + 60
			} else if pad.Has(yaroze.Cross) {
				newGame = true
				whenToLeaveLoop = frameNow + 60
			}
		}

		background.Draw(frame, 0)
		text.Print(heading).Print("\n\nYour final score was: ").Int(currentScore.points).Print("\n\n")
		if position != -1 {
			text.Print("New high score! position ").Int(position + 1).Print("\n\n")
		}
		text.Print("High Score Table\n")
		for i := range highScoreTable {
			text.Print(highScoreTable[i].name).Print("    ").Int(highScoreTable[i].points).Print("\n")
		}
		text.Print("\npress <R-down> for new game\npress L1 and R1 to quit\n")

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
	currentScore = score{name: "Player One"}
	for i := range highScoreTable {
		highScoreTable[i] = score{name: startingNames[i], points: (5 - i) * 10000}
	}
}

// dealWithNewScore puts a score into the table, which is in descending
// order, if it beats one there, and returns its position, or -1 if it
// does not.
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

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	for frame := range screen.Frames() {
		if yaroze.PadButtons(0).Has(yaroze.Start | yaroze.Select) {
			break
		}
		background.Draw(frame, 0)
		text.Print(what).Print(detail)
	}
	screen.Close()
}

func initializeObjects() {
	player = movement{w: 44, h: 24}
	for i := range invaders {
		invaders[i] = invader{typ: none, m: movement{w: 16, h: 16}}
	}
	for i := range shots {
		shots[i] = shot{typ: none, m: movement{w: 8, h: 8}}
	}
}

// createNewInvader brings the first dead invader to life, of a type, and
// returns its index, or -1 if they are all alive.
func createNewInvader(typ int) int {
	for i := range invaders {
		if !invaders[i].alive {
			invaders[i].alive = true
			invaders[i].typ = typ
			return i
		}
	}
	return -1
}

func destroyInvader(id int) {
	invaders[id].alive = false
	invaders[id].typ = none
}

func destroyAllInvaders() {
	for i := range invaders {
		invaders[i].alive = false
		invaders[i].typ = none
		invaders[i].m = movement{w: 16, h: 16}
	}
}

func countNumberOfInvaders() int {
	count := 0
	for i := range invaders {
		if invaders[i].alive {
			count++
		}
	}
	return count
}

// findIDOfNthInvader returns the index of the nth living invader, counting
// from one, or -1 if there are fewer.
func findIDOfNthInvader(n int) int {
	count := 0
	for i := range invaders {
		if invaders[i].alive {
			count++
			if count == n {
				return i
			}
		}
	}
	return -1
}

// createNewShot takes the first free shot for a side, and returns its
// index, or -1 if they are all in flight.
func createNewShot(typ int) int {
	for i := range shots {
		if shots[i].typ == none {
			shots[i].typ = typ
			return i
		}
	}
	return -1
}

func destroyShot(id int) {
	shots[id].typ = none
}

func destroyAllShots() {
	for i := range shots {
		shots[i].typ = none
	}
}

// initializeTexturesAndSprites loads the three TIMs into video memory,
// where the demo chose to put them, and sets the sprites up to show them,
// drawn about their corners as the original drew them: the ship and the
// invaders are 8-bit, the shot 4-bit, and the invaders and the shots are
// then scaled down to size.
func initializeTexturesAndSprites() {
	ship := yaroze.LoadTIMAt(shipFile.Data(), 320, 0)
	playersSprite = ship.Sprite()
	playersSprite.Pivot = yaroze.Point{}
	playersSprite.Mode |= yaroze.Translucent | yaroze.Subtract

	image := yaroze.LoadTIMAt(invaderFile.Data(), 320, 256)
	for i := range invaders {
		s := &invaders[i].sprite
		*s = image.Sprite()
		s.Pivot = yaroze.Point{}
		s.Mode |= yaroze.Translucent | yaroze.Subtract
		s.ScaleX, s.ScaleY = yaroze.One/2, yaroze.One/2
	}

	image = yaroze.LoadTIMAt(shotFile.Data(), 640, 0)
	for i := range shots {
		s := &shots[i].sprite
		*s = image.Sprite()
		s.Pivot = yaroze.Point{}
		s.ScaleX, s.ScaleY = yaroze.One/4, yaroze.One/4
	}
}

func dealWithControllerPad() {
	pad := yaroze.PadButtons(0)
	if pad.Has(yaroze.Select | yaroze.Start) {
		quit = true
		return
	}

	for pad.Has(yaroze.L1) { // pause
		pad = yaroze.PadButtons(0)
	}

	if pad.Has(yaroze.Left) {
		player.dx = max(player.dx-1*fix, -maxXSpeed*fix)
	}
	if pad.Has(yaroze.Right) {
		player.dx = min(player.dx+1*fix, maxXSpeed*fix)
	}

	if pad.Has(yaroze.Triangle) && timeSincePlayerLastFired > 10 {
		if id := createNewShot(playersShot); id >= 0 {
			// Above the ship, moving up at two and a half pixels a
			// frame.
			shots[id].m.x = player.x + 24*fix
			shots[id].m.y = player.y - 2*fix
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
	leastX := 1500 * fix
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

// loseALife puts the ship back at the start, clears the shots and slides
// the invaders to the left, after a pause.
func loseALife() {
	screen.Wait(30)
	numberOfLives--
	destroyAllShots()
	player.x = (minX + screenWidth/2) * fix
	_, h := playersSprite.Size()
	player.y = (maxY - h) * fix
	player.dx = 0
	player.dy = 0
	repositionAllInvadersOnTheLeft()
}

// handleAllObjects moves everything and deals with what collides.
func handleAllObjects() {
	player.x += player.dx
	player.y += player.dy

	// The ship stops at the walls.
	if player.x > (maxX-player.w)*fix {
		player.x = (maxX - player.w) * fix
		player.dx = 0
	} else if player.x < minX*fix {
		player.x = minX * fix
		player.dx = 0
	}
	playersSprite.MoveTo(player.x/fix, player.y/fix)
	playerRect := player.rect()

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
		inv.sprite.MoveTo(inv.m.x/fix, inv.m.y/fix)

		if playerRect.Overlaps(inv.m.rect()) {
			destroyInvader(i)
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
		s.sprite.MoveTo(s.m.x/fix, s.m.y/fix)

		if s.m.y > (maxY-s.m.h)*fix || s.m.y < minY*fix {
			destroyShot(i)
		}

		shotRect := s.m.rect()
		if s.typ == invadersShot {
			// An invader's shot can only hit the ship.
			if playerRect.Overlaps(shotRect) {
				destroyShot(i)
				loseALife()
			}
		} else {
			// The player's can only hit invaders.
			for j := range invaders {
				inv := &invaders[j]
				if inv.alive && shotRect.Overlaps(inv.m.rect()) {
					destroyShot(i)
					currentScore.points += 50 * inv.typ
					destroyInvader(j)
					break
				}
			}
		}
	}
}

// handleInvadersShooting has one invader, chosen at random, fire one
// frame in sixty or so, straight down.
func handleInvadersShooting() {
	if yaroze.Random()%60 != 0 {
		return
	}
	n := countNumberOfInvaders()
	if n == 0 {
		return
	}
	id := findIDOfNthInvader(yaroze.Random()%n + 1)
	shotID := createNewShot(invadersShot)
	if id < 0 || shotID < 0 {
		return
	}
	shots[shotID].m.x = invaders[id].m.x + 8*fix
	shots[shotID].m.y = invaders[id].m.y + 17*fix
	shots[shotID].m.dy = 1 * fix
}

// initSound sends the sound bank to the sound processor, opens the
// sequence and starts it. A bank or sequence that fails is the zero
// value, which plays nothing.
func initSound() {
	var err error
	if bank, err = yaroze.OpenBank(vh.Data(), vb.Data()); err != nil {
		return
	}
	if song, err = bank.Sequence(seqFile.Data()); err != nil {
		return
	}
	yaroze.SetVolume(mvol, mvol)
	song.SetVolume(svol, svol)
	song.Loop()
}

func stopSound() {
	song.Stop()
	screen.Wait(2)
	song.Close()
	bank.Close()
}
