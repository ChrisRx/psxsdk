// Seven simple ways for an enemy to move: chasing and fleeing, bouncing,
// wandering, following a set pattern, buzzing like a fly, switching between
// them at random, and swooping in a wave.
//
// A port of the SCEE tutorial's chapter 8, its seven examples in one
// program, each a few lines of the kind most games' enemies were made of.
// Attack and evade step the enemy one pixel towards the player, or away,
// in each direction it is off by. Bounce reverses a speed at each edge of
// the screen. The random pattern moves one way until it meets an edge and
// then picks another at random. The set pattern reads its moves from a
// table, one a step, as a game would script an enemy's dive. The flies each
// move a random amount every frame, which reads as buzzing. The random
// states put the others behind a state the enemy changes now and then, the
// start of every game's state machine. And the wave swings an enemy from
// side to side by a sine of the frame count while it falls.
//
// The sprites are the tutorial's own, from its SPRITES.TIM: the blue box is
// the player, the red box the enemy and the yellow one a fly.
//
// Differences from the tutorial: the seven programs are one, chosen with
// L1 and R1; the set pattern moves every other frame where the tutorial
// waited an extra vertical blank inside its step; in the random states the
// enemy alone changes state, against a player the directions move, where
// the tutorial moved both boxes by turns; and the screen is 240 lines
// rather than 256.
//
// # Controls
//
// L1 and R1 choose the pattern. The directions move the player, in the
// patterns that have one. In attack and evade, holding circle attacks.
package main

//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// The patterns, in the tutorial's order.
const (
	attackEvade = iota
	bounce
	randomPattern
	setPattern
	flies
	randomStates
	wave
	patterns
)

var patternNames = [patterns]string{
	"ATTACK AND EVADE", "BOUNCE", "RANDOM PATTERN", "SET PATTERN",
	"BUZZING FLIES", "RANDOM STATES", "WAVE ATTACK",
}

// The enemy's states in the random states pattern.
const (
	attack = iota
	evade
	bouncing
	patterned
	wandering
	normal
	states
)

var stateNames = [states]string{"ATTACK", "EVADE", "BOUNCE", "PATTERN", "WANDER", "NORMAL"}

// Directions, as the tutorial numbered them.
const (
	left = iota
	right
	up
	down
)

const (
	screenW, screenH = 320, 240
	numFlies         = 10
	// stateTime is how long the enemy keeps a state, in frames.
	stateTime = 200
)

// steps is the tutorial's set pattern: right for fifty steps, down for ten
// and left for forty.
var steps = [100]uint8{
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{B: 128}, Depth: 1}
	pad    yaroze.Pad

	player, enemy yaroze.GsSPRITE
	fly           [numFlies]yaroze.GsSPRITE

	// The enemy's speed, its direction in the random pattern, and how far
	// it is through the set pattern.
	speedX, speedY int16
	direction      int32
	step           int32

	// mode is the pattern shown, state the enemy's state in the random
	// states, and distance how far apart the player and enemy are, in
	// pixels across and down: all for a test to read.
	mode     int32
	state    int32 = normal
	distance int32
	// stateClock counts the frames the enemy has been in its state, and
	// frames those the pattern has been running.
	stateClock int32
	frames     int32

	text    yaroze.Buffer
	textBuf [256]byte
)

func main() {
	screen.Open()
	screen.Font(16, 16, 288, 40, 200)
	text.Init(textBuf[:])

	if f := disc.LoadAll(300, files...); f != nil {
		for range screen.Frames() {
			yaroze.Print("could not read ")
			yaroze.Print(f.Name)
		}
	}
	// The sheet holds the three sprites down its left edge.
	sheet := yaroze.TIM(spritesTIM.Data()).Load().Sprite()
	enemy = cut(sheet, 0, 44, 22)
	player = cut(sheet, 24, 44, 38)
	yaroze.Srand(1)
	for i := range fly {
		fly[i] = cut(sheet, 63, 24, 20)
	}
	start()

	for ot := range screen.Frames() {
		pad.Update()
		switch {
		case pad.Pressed(yaroze.PADR1):
			mode = (mode + 1) % patterns
			start()
		case pad.Pressed(yaroze.PADL1):
			mode = (mode + patterns - 1) % patterns
			start()
		}
		frames++

		text.Reset()
		text.WriteString("~c990").WriteInt(mode + 1).WriteString(" ").WriteString(patternNames[mode]).WriteString("~c999\n")
		switch mode {
		case attackEvade:
			movePlayer()
			if pad.Held(yaroze.PADRright) {
				chase()
				text.WriteString("ATTACK")
			} else {
				flee()
				keepOn(&enemy)
				text.WriteString("EVADE: HOLD CIRCLE TO ATTACK")
			}
		case bounce:
			bounceOff()
			move()
		case randomPattern:
			wander()
			move()
		case setPattern:
			followPattern()
		case flies:
			for i := range fly {
				f := &fly[i]
				f.X += int16(yaroze.Rand()%5 - 2)
				f.Y += int16(yaroze.Rand()%5 - 2)
				keepOn(f)
			}
		case randomStates:
			movePlayer()
			changeStates()
			text.WriteString(stateNames[state])
		case wave:
			// Across by the sine of the frame count, a twentieth of a
			// circle a frame, and down a pixel every other frame.
			enemy.X = int16(rsin(50*frames)/137) + 140
			enemy.Y += int16(frames % 2)
			if enemy.Y > screenH {
				enemy.Y = -int16(enemy.H)
			}
		}
		measure()
		if mode == attackEvade || mode == randomStates {
			text.WriteString("\nAPART ").WriteInt(distance)
		}
		text.Print()

		switch mode {
		case flies:
			for i := range fly {
				fly[i].Sort(ot, 0)
			}
		case attackEvade, randomStates:
			player.Sort(ot, 0)
			enemy.Sort(ot, 0)
		default:
			enemy.Sort(ot, 0)
		}
	}
}

// measure sets distance. It is a function of its own so that distance
// stays a variable a test can find: one only main uses, the compiler makes
// a local.
//
//go:noinline
func measure() {
	dx, dy := int32(player.X-enemy.X), int32(player.Y-enemy.Y)
	distance = max(dx, -dx) + max(dy, -dy)
}

// cut returns the part of the sheet v lines down its left edge, w by h.
// Its X and Y are its top left corner: GsSortSprite puts the point Mx, My
// of a sprite there, which Sprite set to the middle of the whole sheet.
func cut(sheet yaroze.GsSPRITE, v uint8, w, h uint16) yaroze.GsSPRITE {
	s := sheet
	s.V += v
	s.W, s.H = w, h
	s.Mx, s.My = 0, 0
	return s
}

// start puts everything where a pattern starts.
func start() {
	frames, step, stateClock = 0, 0, 0
	player.X, player.Y = 220, 150
	enemy.X, enemy.Y = 50, 60
	speedX, speedY = 2, 1
	direction = right
	state = normal
	for i := range fly {
		fly[i].X = int16(yaroze.Rand() % (screenW - 44))
		fly[i].Y = int16(40 + yaroze.Rand()%(screenH-62))
	}
	if mode == setPattern {
		enemy.X, enemy.Y = 100, 100
	}
	if mode == wave {
		enemy.Y = 0
	}
}

// movePlayer moves the player with the directions.
func movePlayer() {
	dx, dy := pad.Dir()
	player.X += int16(dx * 2)
	player.Y += int16(dy * 2)
	keepOn(&player)
}

// move moves the enemy by its speed.
func move() {
	enemy.X += speedX
	enemy.Y += speedY
}

// chase steps the enemy a pixel towards the player each way it is off.
func chase() {
	enemy.X += sign(player.X - enemy.X)
	enemy.Y += sign(player.Y - enemy.Y)
}

// flee steps the enemy a pixel away from the player each way.
func flee() {
	enemy.X -= sign(player.X - enemy.X)
	enemy.Y -= sign(player.Y - enemy.Y)
}

func sign(v int16) int16 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// keepOn keeps a sprite on the screen.
func keepOn(s *yaroze.GsSPRITE) {
	s.X = max(0, min(s.X, screenW-int16(s.W)))
	s.Y = max(0, min(s.Y, screenH-int16(s.H)))
}

// bounceOff turns the enemy back from the edges of the screen.
func bounceOff() {
	switch {
	case enemy.X < 0:
		speedX = 2
	case enemy.X > screenW-int16(enemy.W):
		speedX = -2
	}
	switch {
	case enemy.Y < 0:
		speedY = 2
	case enemy.Y > screenH-int16(enemy.H):
		speedY = -2
	}
}

// wander moves the enemy one way until it meets an edge, and then picks a
// way at random.
func wander() {
	speedX, speedY = 0, 0
	switch direction {
	case left:
		speedX = -2
		if enemy.X < 0 {
			direction = yaroze.Rand() % 4
		}
	case right:
		speedX = 2
		if enemy.X > screenW-int16(enemy.W) {
			direction = yaroze.Rand() % 4
		}
	case up:
		speedY = -2
		if enemy.Y < 0 {
			direction = yaroze.Rand() % 4
		}
	case down:
		speedY = 2
		if enemy.Y > screenH-int16(enemy.H) {
			direction = yaroze.Rand() % 4
		}
	}
}

// followPattern takes the set pattern's next step every other frame, and
// starts it again from the same place at its end.
func followPattern() {
	if frames%2 != 0 {
		return
	}
	speedX, speedY = 0, 0
	switch steps[step] {
	case left:
		speedX = -2
	case right:
		speedX = 2
	case up:
		speedY = -2
	case down:
		speedY = 2
	}
	move()
	step++
	if step == int32(len(steps)) {
		step = 0
		enemy.X, enemy.Y = 100, 100
	}
}

// changeStates moves the enemy as its state says, and picks another state
// at random every stateTime frames.
func changeStates() {
	stateClock++
	if stateClock > stateTime {
		state = yaroze.Rand() % states
		stateClock = 0
		step = 0
	}
	switch state {
	case attack:
		chase()
	case evade:
		flee()
	case bouncing:
		bounceOff()
		move()
	case patterned:
		followPattern()
	case wandering:
		wander()
		move()
	case normal:
		// Side to side, turning gently at the edges.
		if enemy.X > screenW-int16(enemy.W) {
			speedX--
		}
		if enemy.X < 0 {
			speedX++
		}
		enemy.X += speedX
	}
	keepOn(&enemy)
}

// rsin is the sine of a, where 4096 is a whole circle, times 4096, as the
// library's rsin returns it, by Bhaskara's approximation, which is within
// a fraction of a percent: plenty for moving a sprite.
func rsin(a int32) int32 {
	a &= 4095
	neg := a >= 2048
	a &= 2047
	p := int64(a) * int64(2048-a)
	s := int32(4096 * 16 * p / (5*2048*2048 - 4*p))
	if neg {
		return -s
	}
	return s
}
