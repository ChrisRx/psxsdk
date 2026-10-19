package main

// What each type of entity is and does. The types by number, which are
// also the models they are drawn with: 1 a moving platform, 2 a pushable
// block, 3 a pot, 4 a shard of one, 5 a crate, 6 a bomb, 7 a chest, 8 a
// sign, 9 the player, 0xb and 0xc a floor switch down and up, 0xd a gate,
// 0xe to 0x12 the monsters, 0x13 a monster's shot, 0x14 a crate of the
// other kind, 0x15 the guardian of the ruins, 0x16 the ship, 0x17 the
// companion, 0x18 a gull, and then what is drawn without a model: 0x1a a
// spark, 0x1c a blast, 0x1d a fireball, 0x1e a feather, 0x1f a healing
// sparkle and 0x20 a torch that shoots.

import "go.chrisrx.dev/psx/yaroze"

// entSetup gives a new entity what its type has: its flags, size, depth,
// behavior steps and hooks.
func entSetup(e *entity) {
	switch e.kind {
	case 1:
		e.flagsA, e.state, e.halfW, e.height, e.otOffset = 0xa4, entStateStill, 0xa0, 0x40, 6
		e.behaviors = platformSteps
		e.onHit, e.onLand = hookBounceHit, hookBounceLand
	case 2:
		e.flagsA, e.state, e.halfW, e.height, e.otOffset = 0xa0, entStateStill, 0xc0, 0xc0, 8
		e.behaviors = blockSteps
		e.onHit, e.onLand = hookLandOnHit, hookLandOnLand
	case 3:
		e.flagsA, e.state, e.hp, e.defense, e.halfW, e.height, e.otOffset = 0x2a4, entStateStill, 1, 0, 0x100, 0x100, 8
		e.behaviors = potSteps
		e.onHit, e.onLand = hookLandOnHit, hookLandOnLand
	case 4:
		e.state = entStateAir
		e.behaviors = shardSteps
	case 5:
		e.flagsA = 0xa5
		entSetupCrate(e)
	case 6:
		e.flagsA, e.state, e.halfW, e.height, e.otOffset = 0x4a3, entStateStill, 100, 100, 4
		e.behaviors = bombSteps
		e.onHit, e.onLand = hookCrateHit, hookCrateLand
	case 7:
		e.flagsA, e.halfW, e.height, e.otOffset = 0x50a0, 0x6e, 0xa0, 4
		e.behaviors = staticSteps
		if chestsOpened[e.param] != 0 {
			e.animMode = animLast
			e.state |= entStateDead
		}
	case 8:
		e.flagsA, e.halfW, e.height, e.otOffset = 0x20a0, 0x80, 0xf0, 4
		e.behaviors = staticSteps
	case 9:
		e.flagsA, e.state, e.attack, e.defense, e.halfW, e.height = 0x200ae2, entStateStill, 5, 3, 100, 0xf0
		e.behaviors = playerSteps
		e.onHit, e.onLand = playerHit, playerLand
	case 0xb, 0xc:
		e.flagsA, e.state, e.halfW, e.height, e.otOffset = 0xa4, entStateStill, 100, 0x40, 4
		e.behaviors = switchSteps
		e.onHit, e.onLand = hookLandOnHit, hookLandOnLand
		switchDown[e.param] = 0
	case 0xd:
		e.flagsA, e.halfW, e.height = 0x80, 0x100, 0x1900
		e.behaviors = gateSteps
		e.onHit, e.onLand = hookBounceHit, hookBounceLand
		if e.param < 2 {
			e.flagsA |= entFlagGoal
		}
		doorsOpen[e.param] = 0
	case 0xe, 0xf, 0x10, 0x11, 0x12:
		e.flagsA, e.state, e.halfW, e.height = 0xba0, entStateStill, 200, 0x50
		e.behaviors = monsterSteps
		e.onHit, e.onLand = hookMonster, hookLandOnLand
		switch e.kind {
		case 0xe, 0x10:
			e.hp, e.attack, e.defense = 5, 0x10, 2
		case 0xf:
			e.hp, e.attack, e.defense = 5, 0x18, 2
		case 0x11:
			e.hp, e.attack, e.defense = 5, 0x18, 2
			e.behaviors = monsterSteps2
		}
		monstersLeft++
	case 0x13:
		e.flagsA, e.state, e.attack, e.halfW, e.height, e.otOffset = 0xa2, entStateWalk, 0, 10, 10, 4
		e.onHit, e.onLand = hookShotHit, hookShotLand
		e.behaviors = shotSteps
	case 0x14:
		e.flagsA = 0x100a3
		entSetupCrate(e)
	case 0x15:
		e.flagsA, e.state, e.hp, e.attack, e.defense, e.halfW, e.height = 0x21880, entStateStill, 0xf, 0x20, 1, 0x1c2, 0x186
		e.behaviors = bossSteps
		e.onHit, e.onLand = hookMonster, hookLandOnLand
	case 0x16:
		e.flagsA, e.halfW, e.height, e.otOffset = 0x4000a0, 0x300, -0x30, 4
		e.behaviors = shipSteps
	case 0x17:
		e.flagsA, e.state, e.halfW, e.height = 0x18c2, entStateStill, 100, 0xe1
		e.behaviors = companionSteps
		e.onHit, e.onLand = hookLandOnHit, hookLandOnLand
	case 0x18:
		e.halfW, e.height = 0xc0, 0xc0
		e.behaviors = gullSteps
	case 0x1a:
		e.behaviors = sparkSteps
	case 0x1c:
		e.attack, e.halfW, e.height = 0x40, 0x300, 0x300
		e.behaviors = blastSteps
	case 0x1d:
		e.flagsA, e.state, e.attack, e.halfW, e.height = 2, entStateWalk, 0x10, 0x80, 0xc0
		e.behaviors = fireballSteps
	case 0x1e:
		e.behaviors = featherSteps
	case 0x1f:
		e.behaviors = sparkleSteps
	case 0x20:
		e.behaviors = torchSteps
	}
}

// entSetupCrate is the setup the two crates share.
func entSetupCrate(e *entity) {
	e.state, e.halfW, e.height, e.otOffset = entStateStill, 100, 0x80, 4
	e.behaviors = crateSteps
	e.onHit, e.onLand = hookCrateHit, hookCrateLand
}

var (
	switchDown   [6]uint8 // which floor switches have something on them
	monstersLeft int      // monsters alive in the area, or -1 once all are gone
)

// The behavior lists, filled in by init to keep the compiler from seeing
// a cycle through entSpawn.
var (
	platformSteps  []func(*entity)
	blockSteps     []func(*entity)
	potSteps       []func(*entity)
	shardSteps     []func(*entity)
	crateSteps     []func(*entity)
	bombSteps      []func(*entity)
	staticSteps    []func(*entity)
	switchSteps    []func(*entity)
	gateSteps      []func(*entity)
	monsterSteps   []func(*entity)
	monsterSteps2  []func(*entity)
	shotSteps      []func(*entity)
	bossSteps      []func(*entity)
	shipSteps      []func(*entity)
	companionSteps []func(*entity)
	gullSteps      []func(*entity)
	sparkSteps     []func(*entity)
	blastSteps     []func(*entity)
	fireballSteps  []func(*entity)
	featherSteps   []func(*entity)
	sparkleSteps   []func(*entity)
	torchSteps     []func(*entity)
)

func init() {
	platformSteps = []func(*entity){entProject, entStep3, entStep1, entStepMove}
	blockSteps = []func(*entity){entProject, blockBehavior, entStep4, entStep1, entStepMove, entKill}
	potSteps = []func(*entity){entProject, potBehavior, entStepMapOnly}
	shardSteps = []func(*entity){entProject2, shardBehavior, entStep4, entStep1}
	crateSteps = []func(*entity){entProject, crateBehavior, entStep4, entStep1, entStep2, entStepMove, entKill}
	bombSteps = []func(*entity){entProject, crateBehavior, entStep4, bombBehavior, entStep1, entStep2, entStepMove, entKill}
	staticSteps = []func(*entity){entProject}
	switchSteps = []func(*entity){entProject, entStep4, entStep1, entStepMove, entKill, switchBehavior}
	gateSteps = []func(*entity){entProject2, gateBehavior, entStep1, entStepCollide}
	monsterSteps = []func(*entity){entProject, monsterBehavior, entStep4, monsterHurt, monsterDie, entStep1, entStepMove, entKill, entTurn}
	monsterSteps2 = []func(*entity){entProject, monsterBehavior, entStep4, monsterHurt, monsterDie, entStep1, entStepMove, entKill}
	shotSteps = []func(*entity){entProject, shotBehavior, shotEnd, entStep1, entStepMove}
	bossSteps = []func(*entity){entProject2, bossBehavior, entStep4, bossStomp, bossHurt, bossDie, entStep1, entStepMove, entKill, entTurn}
	shipSteps = []func(*entity){entProject2, shipBehavior}
	companionSteps = []func(*entity){entProject, companionBehavior, entStep4, entStep1, entStep2, entStepMove, entKill}
	gullSteps = []func(*entity){entProject, gullBehavior}
	sparkSteps = []func(*entity){entProject2, sparkBehavior}
	blastSteps = []func(*entity){entProject2, blastBehavior}
	fireballSteps = []func(*entity){entProject2, fireballBehavior, entStep1, entTurn}
	featherSteps = []func(*entity){entProject2, featherBehavior}
	sparkleSteps = []func(*entity){entProject2, sparkleBehavior}
	torchSteps = []func(*entity){entProject2, torchBehavior}
}

// The hooks.

// hookBounceHit and hookBounceLand stop a platform or gate dead when
// anything meets it, and mark it to reverse.
func hookBounceHit(e, o *entity, how uint32) {
	e.dx, e.dy, e.dz = 0, 0, 0
	e.bounced = true
}

func hookBounceLand(e *entity, how uint32) {
	e.dx, e.dy, e.dz = 0, 0, 0
	e.bounced = true
}

// hookLandOnHit lands the entity on another: ends its fall and puts it
// under the other in the tree; from below, it bounces off the other.
func hookLandOnHit(e, o *entity, how uint32) {
	if how&hitOnTop == 0 {
		if how&hitFromAbove != 0 && e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
		return
	}
	e.dying = 0
	e.state &^= entStateAir
	if e.parent != o {
		entAttach(e, o)
	}
}

// hookLandOnLand lands the entity on the map, back under the root.
func hookLandOnLand(e *entity, how uint32) {
	if how&landHitFloor == 0 {
		if how&landHitCeiling != 0 && e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
		return
	}
	e.dying = 0
	e.state &^= entStateAir
	if e.parent != entRoot {
		entAttach(e, entRoot)
	}
}

// hookCrateHit lands a crate on another entity, with a spark where it
// lands when it was falling, and turns it round with a knock when it
// meets one sideways.
func hookCrateHit(e, o *entity, how uint32) {
	if how&hitOnTop == 0 {
		if how&hitFromAbove == 0 {
			e.rot.Vy = int16(uint16(int(e.rot.Vy)+0x800) & 0xfff)
			playEffect(5)
		} else if e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
		return
	}
	if e.state&entStateAir != 0 {
		e.dying = 0
		e.state &^= entStateAir
		entSpawn(0x1a, 0, 0, 0, e.x, e.y-int16(e.height)+e.dy, e.z, 0)
	}
	if e.parent != o {
		entAttach(e, o)
	}
	e.pushX, e.pushZ = 0, 0
}

// hookCrateLand is hookCrateHit against the map.
func hookCrateLand(e *entity, how uint32) {
	if how&landHitFloor == 0 {
		if how&landHitCeiling == 0 {
			e.rot.Vy = int16(uint16(int(e.rot.Vy)+0x800) & 0xfff)
			playEffect(5)
		} else if e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
		return
	}
	if e.state&entStateAir != 0 {
		e.dying = 0
		e.state &^= entStateAir
		entSpawn(0x1a, 0, 0, 0, e.x, e.y-int16(e.height)+e.dy, e.z, 0)
	}
	if e.parent != entRoot {
		entAttach(e, entRoot)
	}
	e.pushX, e.pushZ = 0, 0
}

// hookMonster lands a monster as hookLandOnHit does, and hurts the player
// when the player is what it met.
func hookMonster(e, o *entity, how uint32) {
	hookLandOnHit(e, o, how)
	if o == playerEnt && playerEnt.state&(entStateDead|entStateHurt) == 0 {
		hurt(e, o)
	}
}

// hookShotHit ends a shot on whatever it hits, hurting the player.
func hookShotHit(e, o *entity, how uint32) {
	e.state |= entStateDead
	if o == playerEnt && playerEnt.state&(entStateDead|entStateHurt) == 0 {
		hurt(e, o)
	}
}

// hookShotLand ends a shot on the map.
func hookShotLand(e *entity, how uint32) {
	e.state |= entStateDead
}

// blockBehavior lets the player push a block: when their box meets its,
// it slides at half their speed, with a scrape every eight frames.
func blockBehavior(e *entity) {
	e.vx, e.vz = 0, 0
	if e.frame&entFrameActive == 0 {
		return
	}
	p := playerEnt
	hw := e.halfW + p.halfW
	dx := abs(int(e.x) - (int(p.x) + int(p.vx)))
	dz := abs(int(e.z) - (int(p.z) + int(p.vz)))
	if dx-hw < 0 && dz-hw < 0 {
		if abs(int(e.y)-int(p.y)) < p.height*3/2 {
			if p.vx == 0 {
				e.vz = p.vz / 2
			} else {
				e.vx = p.vx / 2
			}
			if e.t1&7 == 0 {
				playEffect(0x15)
			}
			e.t1++
		}
	}
	e.t1 = 0
}

// potBehavior breaks a pot that has been hit into four shards flying
// outward.
func potBehavior(e *entity) {
	if e.state&entStateDead == 0 {
		return
	}
	for i := range 4 {
		dx, dz := potShards[i][0], potShards[i][1]
		s := entSpawn(4, -1, 0, 0, e.x+dx, e.y, e.z+dz, 0)
		a := atanA(int(dz), int(dx))
		s.vx = int16(div128(cosA(a)))
		s.vz = int16(div128(sinA(a)))
	}
	entFree(e)
}

var potShards = [4][2]int16{{-256, -256}, {256, -256}, {-256, 256}, {256, 256}}

// shardBehavior flickers a shard and takes it away after 24 frames.
func shardBehavior(e *entity) {
	if e.t4&1 != 0 {
		e.frame |= entFrameHidden
	}
	e.t4++
	if e.t4 == 0x18 {
		entFree(e)
	}
}

// crateBehavior flies a thrown crate along its arc, slowing, and carries
// a push it was given.
func crateBehavior(e *entity) {
	if e.state&entStateWalk != 0 {
		speed := throwSpeed[e.t4]
		e.t4++
		e.vx = int16(div4096(-speed * sinA(int(e.rot.Vy))))
		e.vz = int16(div4096(speed * cosA(int(e.rot.Vy))))
		if e.t4 == 0x16 {
			e.t4 = 0
			e.state = e.state&^entStateWalk | entStateStill
		}
	}
	if e.state&entStateStill != 0 {
		e.vx, e.vz = 0, 0
	}
	e.vx += e.pushX
	e.vz += e.pushZ
}

var throwSpeed = [22]int{256, 192, 160, 128, 96, 64, 48, 32, 32, 32, 32, 32, 32, 32, 32, 32, 24, 16, 12, 8, 6, 4}

// bombBehavior counts a bomb down from when the player lifts it, lighting
// the fuse animation, and blows it up into three blasts, one of them
// where it was and two near it.
func bombBehavior(e *entity) {
	t := e.t3
	if t == 0 {
		if e.frame&entFrameActive != 0 && e.parent.state&entStateCarrying != 0 {
			e.t3 = 1
		}
		return
	}
	if t == 1 {
		e.anim, e.animMode = 0, 1
		e.frame |= entFrameNewAnim
	}
	e.t3 = t + 1
	if t != 0x78 {
		return
	}
	entSpawn(0x1c, 0, 0x400, 0, e.x, e.y, e.z, 0)
	entSpawn(0x1c, 0, 0x3c0, 1, e.x+int16(yaroze.Rand()%0x100), e.y+int16(yaroze.Rand()%0x100), e.z+int16(yaroze.Rand()%0x100), 0)
	entSpawn(0x1c, 0, 0x300, 2, e.x+int16(yaroze.Rand()%0x100), e.y+int16(yaroze.Rand()%0x100), e.z+int16(yaroze.Rand()%0x100), 0)
	entFree(e)
	playEffect(1)
}

// switchBehavior presses a floor switch under anything standing on it and
// lets it up after: a platform appears for the first two, a chest for the
// third.
func switchBehavior(e *entity) {
	if e.state&entStateDead != 0 {
		return
	}
	if e.child != nil {
		switchDown[e.param] = 1
		if e.kind != 0xc {
			return
		}
		playEffect(0x10)
		e.frame |= entFrameNewActor
		e.kind = 0xb
		switch e.param {
		case 1:
			e.gate = entSpawn(1, -1, 0, 11, 0x300, 0x600, 0x1b00, 0)
		case 0:
			e.gate = entSpawn(1, -1, 0, 2, 0xb00, 0x400, 0xf00, 0)
		case 2:
			if chestsOpened[3] == 0 {
				e.gate = entSpawn(7, 1, 2, 3, 0x500, 0xa0, 0xb00, 0)
			} else {
				e.gate = entSpawn(7, 1, 2, 14, 0x500, 0xa0, 0xb00, 0)
			}
			e.state |= entStateDead
		}
		return
	}
	switchDown[e.param] = 0
	if e.kind != 0xb {
		return
	}
	e.frame |= entFrameNewActor
	e.kind = 0xc
	if e.param < 2 && e.param >= 0 && e.gate != nil {
		entFree(e.gate)
		e.gate = nil
	}
}

// gateBehavior takes a gate away once its door is open, and slides the
// fourth gate as the three switches of its room are pressed.
func gateBehavior(e *entity) {
	if doorsOpen[e.param] == 1 {
		playEffect(0xd)
		entFree(e)
		return
	}
	if e.param != 4 {
		return
	}
	t := e.t4
	target := (int(switchDown[3]) + int(switchDown[4]) + int(switchDown[5])) * 0x78
	if e.bounced {
		if t < target {
			t -= 8
		} else if target < t {
			t += 8
		}
		e.bounced = false
	}
	switch {
	case t < target:
		t += 8
		e.vz = 8
	case target < t:
		t -= 8
		e.vz = -8
	default:
		e.vz = 0
	}
	e.t4 = t
}

// monsterBehavior is the monsters' minds: standing still, each kind picks
// its next move at random, the first wandering, the second walking toward
// the player, the third charging when the player is near, the fourth
// shooting; walking, each walks for its own number of frames and stops.
func monsterBehavior(e *entity) {
	if e.frame&entFrameActive == 0 || e.state&entStateDead != 0 {
		return
	}
	if e.state&entStateHurt != 0 && e.t2 < 12 {
		return
	}
	if e.state&entStateStill == 0 {
		e.t4++
		limit := 8
		switch e.kind {
		case 0xe:
			limit = 0x40
		case 0xf:
			limit = 0x20
		case 0x12:
			return
		}
		if e.t4 != limit {
			return
		}
		e.t4 = 0
		e.state = e.state&^entStateWalk | entStateStill
		e.vx, e.vz = 0, 0
		if e.kind == 0x10 || e.kind == 0x11 {
			e.anim, e.animMode = 0xf, 0
			e.frame |= entFrameNewAnim
		}
		return
	}
	dx := int(int16(uint16(playerEnt.x) - uint16(e.x)))
	dz := int(int16(uint16(playerEnt.z) - uint16(e.z)))
	switch e.kind {
	case 0xe:
		if yaroze.Rand()&0xff != 0 {
			return
		}
		e.t5 = 0
		e.state = e.state&^entStateStill | entStateWalk
		if yaroze.Rand()&1 == 0 {
			e.vz = -8
			if yaroze.Rand()&1 != 0 {
				e.vz = 8
			}
		} else {
			e.vx = -8
			if yaroze.Rand()&1 != 0 {
				e.vx = 8
			}
		}
	case 0xf:
		if yaroze.Rand()&0x7f != 0 {
			return
		}
		e.t5 = 0
		e.state = e.state&^entStateStill | entStateWalk
		if abs(dz) < abs(dx) {
			e.vx = -16
			if dx > 0 {
				e.vx = 16
			}
		} else {
			e.vz = -16
			if dz > 0 {
				e.vz = 16
			}
		}
	case 0x10:
		near := abs(dx) < e.halfW*3 && abs(dz) < e.halfW*3
		if !near && e.t2 != 12 {
			return
		}
		if yaroze.Rand()&7 != 0 && e.t2 != 12 {
			return
		}
		e.t5 = 0
		e.state = e.state&^entStateStill | entStateWalk
		if abs(dz) < abs(dx) {
			e.vx = -0x30
			if dx > 0 {
				e.vx = 0x30
			}
		} else {
			e.vz = -0x30
			if dz > 0 {
				e.vz = 0x30
			}
		}
		e.anim, e.animMode = 0x10, 1
		e.frame |= entFrameNewAnim
	case 0x11:
		e.t1++
		if abs(dx) >= 0x800 || abs(dz) >= 0x800 || e.t1 < 97 || yaroze.Rand()&0x1f != 0 {
			return
		}
		e.t1, e.t5 = 0, 0
		e.state = e.state&^entStateStill | entStateWalk
		a := atanA(int(playerEnt.z)-int(e.z), int(playerEnt.x)-int(e.x))
		a += int(yaroze.Rand()%128) - 64
		e.rot.Vy = int16(a - 0x400)
		cx := cosA(int(int16(a)))
		if cx < 0 {
			cx += 7
		}
		sz := sinA(int(int16(a)))
		if sz < 0 {
			sz += 7
		}
		entSpawn(0x13, 0x12, 0, 0, e.x+int16(cx>>3), e.y+int16(e.height*4), e.z+int16((sz<<13)>>16), 0)
		playEffect(0xc)
		e.anim, e.animMode = 0x10, 1
		e.frame |= entFrameNewAnim
	}
}

// monsterHurt plays out a monster's hurt: it flickers for twenty frames,
// and halfway through the second kind runs at the player while the others
// stop.
func monsterHurt(e *entity) {
	if e.state&entStateHurt == 0 {
		return
	}
	t := e.t2
	if t&1 == 0 {
		e.frame |= entFrameHidden
	}
	if t == 0 {
		e.anim, e.animMode = 0xf, 0
		e.frame |= entFrameNewAnim
	}
	t++
	e.t2 = t
	if t == 20 {
		e.t2 = 0
		e.state &^= entStateHurt
	}
	if t != 10 {
		return
	}
	switch {
	case e.kind == 0xf:
		e.state = e.state&^entStateStill | entStateWalk
		e.t5 = 0
		dx := int(int16(uint16(playerEnt.x) - uint16(e.x)))
		dz := int(int16(uint16(playerEnt.z) - uint16(e.z)))
		if abs(dz) < abs(dx) {
			e.vx = -16
			if dx > 0 {
				e.vx = 16
			}
			e.vz = 0
		} else {
			e.vx = 0
			e.vz = -16
			if dz > 0 {
				e.vz = 16
			}
		}
	case e.kind == 0xe, e.kind >= 0x10 && e.kind < 0x13:
		e.t4 = 0
		e.state = e.state&^entStateWalk | entStateStill
		e.vx, e.vz = 0, 0
	}
}

// monsterDie plays out a monster's death and takes it away, counting it
// off the area; when the last of the eighth area's goes, a platform
// appears.
func monsterDie(e *entity) {
	if e.state&entStateDead == 0 {
		return
	}
	if e.t9 == 0 {
		e.state |= entStateHurt
	}
	if e.t9 == 9 {
		e.t2 = 0
		e.state &^= entStateHurt
		e.anim, e.animMode = 0x11, 1
		e.frame |= entFrameNewAnim
	}
	e.t9++
	if e.t9 == 0x28 {
		entFree(e)
		monstersLeft--
		if monstersLeft == 0 {
			monstersLeft = -1
			if area == 8 {
				entSpawn(1, -1, 0, 10, 0x1f00, 0xa00, 0x700, 0)
			}
		}
	}
}

// shotBehavior aims a monster's shot at the player as it is fired.
func shotBehavior(e *entity) {
	if e.t4 == 0 {
		a := atanA(int(playerEnt.z)-int(e.z), int(playerEnt.x)-int(e.x))
		e.rot.Vy = int16(a)
		e.vx = int16(div128(cosA(a)))
		e.vz = int16(div128(sinA(a)))
	}
	e.t4++
}

// shotEnd takes a shot away once it has hit.
func shotEnd(e *entity) {
	if e.state&entStateDead != 0 {
		entFree(e)
	}
}

var (
	bossLastPhase int
	bossStomped   bool
)

// bossBehavior is the guardian's mind. Until the player reaches it, it
// only speaks; then it keeps turning to face them and cycles through its
// moves: resting, spinning, two kinds of fireball spray, and a leap that
// shakes the ground and hurts a player standing on it.
func bossBehavior(e *entity) {
	t := e.t4
	if e.state&entStateDead != 0 || flags[storyBossMet] == 0 {
		if t == 0xb4 {
			startDialog(8, true)
		}
		t++
		if t == 0xb6 {
			t = 0xb5
		}
		e.t4 = t
		return
	}
	if e.t1 == 0 {
		t = 0
		nextCameraKey()
		demoOnly = false
		e.t1 = 1
	}
	e.flagsA &^= entFlagAlways
	phase := e.phase
	switch phase {
	case 0:
		if t >= 31 && yaroze.Rand()&7 == 0 {
			bossLastPhase = phase
			for {
				phase = int(yaroze.Rand()%4) + 1
				if bossLastPhase != phase {
					break
				}
			}
			t = 0
		}
	case 1:
		if t >= 65 {
			a := atanA(int(playerEnt.x)-int(e.x), int(playerEnt.z)-int(e.z))
			if (int(int16(-a))-int(e.rot.Vy))&0xfff < 256 && yaroze.Rand()&1 == 0 {
				t, phase = 0, 0
			}
		}
	case 2, 4:
		if t&0x1f == 0 && yaroze.Rand()&1 == 0 {
			t, phase = 0, 0
		}
	case 3:
		if t >= 41 {
			camShake = false
			t, phase = 0, 0
		}
	}
	if phase == 3 || phase == 4 {
		a := atanA(int(playerEnt.x)-int(e.x), int(playerEnt.z)-int(e.z))
		d := (int(e.rot.Vy) - int(int16(-a))) & 0xfff
		var step int
		if d < 0x800 {
			if d != 0 {
				v := -d
				if v < 0 {
					v += 0x1f
				}
				step = v >> 5
				if step == 0 {
					step = -1
				}
			}
		} else {
			v := 0x1000 - d
			if v < 0 {
				v += 0x1f
			}
			step = v >> 5
			if step == 0 {
				step = 1
			}
		}
		e.rot.Vy += int16(step)
	}
	switch phase {
	case 0:
		if t == 0 {
			e.anim, e.animMode = 0x13, 0
			e.frame |= entFrameNewAnim
		}
	case 1:
		if t == 0 {
			e.anim, e.animMode = 0x15, 1
			e.frame |= entFrameNewAnim
		}
		if t >= 24 {
			if t == 24 {
				e.anim, e.animMode = 0x13, 0
				e.frame |= entFrameNewAnim
			}
			e.rot.Vy -= 0x80
		}
	case 2, 4:
		if t == 0 {
			e.anim, e.animMode = 0x14, 0
			e.frame |= entFrameNewAnim
		}
		if t&1 == 0 {
			i := (t >> 2) & 0xf
			spread := int(bossSpray[i])
			a := int(e.rot.Vy) - spread*0x30
			f := entSpawn(0x1d, 0, 0, 0, int16(int(e.x)-sinA(a)/7), 0x1cc, int16(int(e.z)+cosA(a)/7), 0)
			a = int(e.rot.Vy) - spread*0x28
			f.vx = int16(div128(-sinA(a)))
			f.vy = -32
			f.vz = int16(div128(cosA(a)))
			playEffect(0xd)
		}
	case 3:
		if t == 0 {
			bossStomped = false
			e.state |= entStateAir
		} else if e.state&entStateAir == 0 {
			camShake = true
			if !bossStomped {
				if playerEnt.state&entStateAir == 0 {
					hurt(e, playerEnt)
				}
				bossStomped = true
				playEffect(0x11)
			}
		}
	}
	e.t4 = t + 1
	e.phase = phase
	if flags[storyBossMet] == 1 {
		flags[storyBossMet] = 2
		entSpawn(0xd, -1, 0, 2, 0xd00, 0x100, 0x1100, 0)
		entSpawn(0xd, -1, 0, 3, 0x1900, 0x100, 0x1100, 0)
		playEffect(0xd)
	}
}

var bossSpray = [16]int8{0, -1, -2, -3, -4, -3, -2, -1, 0, 1, 2, 3, 4, 3, 2, 1}

// bossStomp hurts the player when the guardian's spin catches them from
// behind, and when its leap lands near them.
func bossStomp(e *entity) {
	if playerEnt.state&(entStateDead|entStateHurt) != 0 {
		return
	}
	a := atanA(int(playerEnt.x)-int(e.x), int(playerEnt.z)-int(e.z))
	d := int(int16(-a)) - int(int16(uint16(e.rot.Vy)+0x800))
	if d < 0 {
		d = -d
	}
	if d&0xfff < 0x100 && playerEnt.y < 0x300 {
		hurt(e, playerEnt)
	}
}

// bossHurt lets the player's sword hurt the guardian when they strike
// from in front, up on its back, and plays out its hurt.
func bossHurt(e *entity) {
	p := playerEnt
	if e.state&(entStateDead|entStateHurt) == 0 && p.state&entStateSwing != 0 && e.phase != 2 && e.phase != 4 && p.t3 == 2 {
		dx := int(p.x) - int(e.x)
		if abs(dx) < 768 {
			dz := int(p.z) - int(e.z)
			if abs(dz) < 768 {
				a := atanA(dx, dz)
				d := int(int16(-a)) - int(e.rot.Vy)
				if d < 0 {
					d = -d
				}
				if d&0xfff < 0x200 && p.y > 0x300 {
					hurt(p, e)
				}
			}
		}
	}
	if e.state&entStateHurt == 0 {
		return
	}
	t := e.t2
	if t == 0 {
		playEffect(6)
	}
	if t&1 == 0 {
		e.frame |= entFrameHidden
	}
	e.t2 = t + 1
	if t+1 == 10 {
		e.t2 = 0
		e.state &^= entStateHurt
	}
}

// bossDie plays out the guardian's death: it sinks, bursting into blasts
// for 120 frames, and then is gone, opening the way.
func bossDie(e *entity) {
	if e.state&entStateDead == 0 {
		return
	}
	t := e.t9
	if t == 0 {
		flags[storyBossDead] = 1
		e.anim, e.animMode = 0x13, 0
		e.frame |= entFrameNewAnim
	}
	e.y -= 4
	hw, h := e.halfW, e.height
	// The random numbers come in the original's order: the place first,
	// then the blast's size, since the sequence is shared with everything.
	bx := e.x - int16(hw) + int16(yaroze.Rand()%int32(hw*2))
	by := e.y - int16(h) + int16(yaroze.Rand()%int32(h*2))
	bz := e.z - int16(hw) + int16(yaroze.Rand()%int32(hw*2))
	entSpawn(0x1c, 0, int(yaroze.Rand()%0x100)+0x100, 1, bx, by, bz, 0)
	if t&1 == 0 {
		playEffect(0x13)
		e.frame |= entFrameHidden
	}
	e.t9 = t + 1
	if t+1 == 0x78 {
		entFree(e)
		flags[storyBossDead] = 2
		doorsOpen[2], doorsOpen[3] = 1, 1
	}
	camShake = false
}

// shipBehavior rocks the ship on the swell, and in the end sails it away
// into the sunset, fading the picture to white.
func shipBehavior(e *entity) {
	t := e.t1
	e.dy = swell[t]
	flags[storyShipT] = uint8(t)
	if flags[storyLeft] != 0 {
		switch {
		case e.x < 0x2a00:
			e.dx = 0xc
		case e.x < 0x2b00:
			e.dx = 6
		case e.x < 0x2c00:
			e.dx = 3
		default:
			e.dx = 0
		}
	}
	if flags[storyEnding] != 0 {
		over := screen.Overlay()
		s := e.t4
		e.t4 = s + 1
		switch {
		case s > 0x64:
			e.dx = 0xc
		case s > 0x3c:
			e.dx = 8
		case s > 0x14:
			e.dx = 4
		}
		if s-0x100 >= 0 && s-0x100 < 0x80 {
			c := uint8((s - 0x100) * 2)
			fillRect(over.Entry(0), -160, -120, 320, 240, c, c, c, true, 2)
		} else if s > 0x17f {
			fillRect(over.Entry(0), -160, -120, 320, 240, 0, 0, 0, false, 0)
		}
		if s == 0x1c0 {
			flags[flagStart] = 1
		}
	}
	t++
	if t == 0x30 {
		t = 0
	}
	e.t1 = t
}

var swell = [48]int16{-1, -1, -2, -2, -3, -3, -4, -4, -5, -5, -6, -6, -5, -5, -4, -4, -3, -3, -2, -2, -1, -1, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 5, 5, 4, 4, 3, 3, 2, 2, 1, 1, 0, 0}

// gullBehavior flies a gull across the sky, bobbing, dropping a feather
// as it passes the cell it was told to, and takes it away off the map.
func gullBehavior(e *entity) {
	e.x -= 0x40
	e.y += swell[e.t4]
	e.t4++
	if e.t4 == 0x30 {
		e.t4 = 0
	}
	if e.param != -1 && e.t1 == 0 && int(e.x) < e.param<<9 {
		e.t1 = 1
		entSpawn(0x1e, 0, 0, 0, e.x, e.y, e.z, 0)
	}
	if e.x < -0x1000 {
		entFree(e)
	}
}

// spawnGull puts a gull in the air at the cell x, y, z, to drop a feather
// at cell c, or none with -1.
func spawnGull(x, y, z, c int) {
	entSpawn(0x18, 0x24, 0, c, int16(x<<9), int16(y<<9), int16(z<<9), 0)
}

// Working storage for the sprite effects.
var (
	fxCorner  [4]yaroze.SVECTOR
	fxSXY     [3]uint32
	fxSXY3    uint32
	fxCenter  yaroze.SVECTOR
	fxMatrix  yaroze.MATRIX
	fxCenterV yaroze.VECTOR
)

// sparkBehavior draws a ring spreading from where something landed, for
// eight frames.
func sparkBehavior(e *entity) {
	t := e.t4
	e.t4++
	if t == 8 {
		entFree(e)
		return
	}
	r := int16(t*8 + 0x80)
	y := -e.y
	fxCorner[0] = yaroze.SVECTOR{Vx: e.x - r, Vy: y, Vz: r - e.z}
	fxCorner[1] = yaroze.SVECTOR{Vx: e.x + r, Vy: y, Vz: r - e.z}
	fxCorner[2] = yaroze.SVECTOR{Vx: e.x - r, Vy: y, Vz: -r - e.z}
	fxCorner[3] = yaroze.SVECTOR{Vx: e.x + r, Vy: y, Vz: -r - e.z}
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	c := uint32(uint8(-0x10*t - 0x80))
	spriteQuad(&things[0], c, 1)
}

// spriteQuad draws a textured quad on fxCorner, lit c, semi-transparent
// at the rate, into the main table at the quad's average depth.
func spriteQuad(tex *texture, c uint32, abr uint32) {
	if yaroze.RotTransPers3(&fxCorner[0], &fxCorner[1], &fxCorner[2], &fxSXY) < 0 {
		return
	}
	p := yaroze.AllocPacket(10)
	p[2], p[4], p[6] = fxSXY[0], fxSXY[1], fxSXY[2]
	yaroze.RotTransPers(&fxCorner[3], &fxSXY3)
	p[8] = fxSXY3
	otz := yaroze.AverageZ4()
	p[0] = 9 << 24
	p[1] = 0x2e<<24 | c<<16 | c<<8 | c
	u0, v0 := uint32(tex.u), uint32(tex.v)
	u1, v1 := uint32(uint8(int(tex.u)+int(tex.w)-1)), uint32(uint8(int(tex.v)+int(tex.h)-1))
	p[3] = u0 | v0<<8 | uint32(tex.clut())<<16
	p[5] = u1 | v0<<8 | uint32(tex.tpageY>>4&0x10|(tex.tpageX&0x3ff)>>6|uint16(abr)<<5|(tex.tpageY&0x200)<<2)<<16
	p[7] = u0 | v1<<8
	p[9] = u1 | v1<<8
	screen.OT().Link(max(otz>>2, 0), p)
}

// billboard draws frame f of a run of textures facing the camera at the
// world point pos, size on a side, lit c, additive: the blasts and the
// fireballs.
func billboard(tex *texture, pos *[3]int16, size int16, c uint32) {
	fxCenter = yaroze.SVECTOR{Vx: pos[0], Vy: -pos[1], Vz: -pos[2]}
	yaroze.ApplyMatrix(yaroze.GsWSMATRIX(), &fxCenter, &fxCenterV)
	ws := yaroze.GsWSMATRIX()
	fxMatrix = yaroze.IDMATRIX
	fxMatrix.T = [3]int32{fxCenterV.Vx + ws.T[0], fxCenterV.Vy + ws.T[1], fxCenterV.Vz + ws.T[2]}
	yaroze.GsSetLsMatrix(&fxMatrix)
	fxCorner[0] = yaroze.SVECTOR{Vx: -size, Vy: -size, Vz: -256}
	fxCorner[1] = yaroze.SVECTOR{Vx: size, Vy: -size, Vz: -256}
	fxCorner[2] = yaroze.SVECTOR{Vx: -size, Vy: size, Vz: -256}
	fxCorner[3] = yaroze.SVECTOR{Vx: size, Vy: size, Vz: -256}
	spriteQuad(tex, c, 1)
}

// blastBehavior draws a blast growing through its frames, hurting what it
// reaches on its sixth and setting off any bomb there, and takes it away
// after.
func blastBehavior(e *entity) {
	t := e.t3 - e.param*4
	e.t3++
	if t >= 0 {
		pos := [3]int16{e.x, e.y, e.z}
		billboard(&things[3+t], &pos, int16(e.animMode), 0x80)
	}
	if e.param == 0 && t == 6 {
		for i := 0; i < frameCount; i++ {
			o := frameList[i]
			if o == nil || o.flagsA&entFlagLiftable == 0 || o == e || entOverlapClear(e.x, e.y, e.z, e.halfW, e.height, o) {
				continue
			}
			if o.flagsA&entFlagBlastable != 0 {
				hurt(e, o)
			} else if o.flagsA&entFlagBomb != 0 {
				o.t3 = 0x77
			}
		}
	}
	if t == 0xf {
		entFree(e)
	}
}

// fireballBehavior flies a fireball, drawn as a growing flame, hurting the
// player it reaches, for sixteen frames.
func fireballBehavior(e *entity) {
	p := playerEnt
	if p.state&(entStateDead|entStateHurt) == 0 {
		if abs(int(p.x)-int(e.x)) < e.halfW && abs(int(p.y)-int(e.y)) < e.height && abs(int(p.z)-int(e.z)) < e.halfW {
			hurt(e, p)
			entFree(e)
			return
		}
	}
	t := e.t4
	e.t4++
	if t == 0x10 {
		entFree(e)
		return
	}
	pos := [3]int16{e.x, e.y, e.z}
	billboard(&things[3+t], &pos, 0xc0, 0x80)
}

// featherBehavior floats a feather down, swaying, as a turning sprite,
// until it reaches the ground.
func featherBehavior(e *entity) {
	t := e.t4
	e.t4++
	if t == 0x30 {
		t = 0
		e.t4 = 0
	}
	if e.y < 0 {
		entFree(e)
		return
	}
	e.x += featherSway[t%12] * 4
	e.y -= 6
	tilt := featherTilt[t%12]
	x0, x1 := e.x-0x40, e.x+0x40
	fxCorner[0] = yaroze.SVECTOR{Vx: x0, Vy: tilt*-2 - e.y, Vz: 0x40 - e.z}
	fxCorner[1] = yaroze.SVECTOR{Vx: x1, Vy: tilt*2 - e.y, Vz: 0x40 - e.z}
	fxCorner[2] = yaroze.SVECTOR{Vx: x0, Vy: tilt*-2 - e.y, Vz: -0x40 - e.z}
	fxCorner[3] = yaroze.SVECTOR{Vx: x1, Vy: tilt*2 - e.y, Vz: -0x40 - e.z}
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	spriteQuad(&things[58], 0x60, 1)
}

var (
	featherSway = [12]int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	featherTilt = [12]int16{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
)

// sparkleBehavior draws a sparkle rising and turning over the player as
// they use a healing item, for 32 frames.
func sparkleBehavior(e *entity) {
	t := e.t4
	a := e.param * 0x155
	size := int(yaroze.Rand() % 0x40)
	c := size + 0x40
	if t > 0x18 {
		size = size * (0x20 - t)
		if size < 0 {
			size += 7
		}
		size >>= 3
		c = c * (0x20 - t)
		if c < 0 {
			c += 7
		}
		c >>= 3
	}
	s := int16(size)
	fxCorner[0] = yaroze.SVECTOR{Vx: -s, Vy: -s}
	fxCorner[1] = yaroze.SVECTOR{Vx: s, Vy: -s}
	fxCorner[2] = yaroze.SVECTOR{Vx: -s, Vy: s}
	fxCorner[3] = yaroze.SVECTOR{Vx: s, Vy: s}
	fxCenter = yaroze.SVECTOR{Vx: e.x + int16(sinA(a)/0x18), Vy: 0xf0 - e.y, Vz: -(e.z + int16(cosA(a)/0x18))}
	yaroze.ApplyMatrix(yaroze.GsWSMATRIX(), &fxCenter, &fxCenterV)
	ws := yaroze.GsWSMATRIX()
	fxMatrix = yaroze.IDMATRIX
	yaroze.RotMatrixZ(int32(t<<7), &fxMatrix)
	fxMatrix.T = [3]int32{fxCenterV.Vx + ws.T[0], fxCenterV.Vy + ws.T[1], fxCenterV.Vz + ws.T[2]}
	yaroze.GsSetLsMatrix(&fxMatrix)
	spriteQuad(&things[70], uint32(uint8(c)), 1)
	e.y += 0x10
	if t+1 == 0x20 {
		entFree(e)
	} else {
		e.t4 = t + 1
	}
}

// torchBehavior wakes a torch when the player comes within reach and has
// it shoot fireballs up every other frame for 90, then sleep.
func torchBehavior(e *entity) {
	dx := int(e.x) - int(playerEnt.x)
	dz := int(e.z) - int(playerEnt.z)
	d2 := dx*dx + dz*dz
	if e.state&entStateWalk == 0 {
		if d2 < 377487 {
			e.state |= entStateWalk
		}
		return
	}
	if e.t4 < 90 && e.t4&1 != 0 && d2 < 377487 {
		f := entSpawn(0x1d, 0, 0, 0, e.x, e.y, e.z, 0)
		f.vx, f.vy, f.vz = 0, 0, 0x1a
		playEffect(0xd)
	}
	e.t4++
	if e.t4 != 150 {
		return
	}
	e.t4 = 0
	e.state &^= entStateWalk
}
