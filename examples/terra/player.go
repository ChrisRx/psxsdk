package main

// The player: walking under the direction buttons as the camera turns
// them, jumping with Cross, swinging the sword, lifting, carrying and
// throwing with Square, using a healing item with Circle, being hurt,
// falling off the map, and dying.

import "go.chrisrx.dev/psx/yaroze"

var playerSteps []func(*entity)

func init() {
	playerSteps = []func(*entity){entProject2, playerControl, playerAct, playerJump, playerSwing, playerCarry, playerFall, playerDie, entStep1, entStep2, entStepMove, entKill, entTurn}
}

// Items held, hearts, and the clock.
var (
	potions  int
	keys     int
	hearts   = 10
	timeLeft = 0x1a5e0
	hudOn    bool
)

// The directions the direction buttons give, by the eight combinations,
// and the walk each gives before the camera turns it.
var (
	padDirs  = [8]yaroze.Buttons{0x4000, 0xc000, 0x8000, 0x9000, 0x1000, 0x3000, 0x2000, 0x6000}
	walkStep = [8][2]int16{{0, 32}, {-24, 24}, {-32, 0}, {-24, -24}, {0, -32}, {24, -24}, {32, 0}, {24, 24}}
)

// playerControl walks the player under the direction buttons, turned by
// the camera's quarter unless a demonstration drives them, starting the
// walking animation, or stops them with the standing one.
func playerControl(e *entity) {
	if e.state&entStateHurt != 0 && e.t2 <= 9 {
		return
	}
	e.vx, e.vz = 0, 0
	s := e.state
	if s&0x140 != 0 || (s&entStateSwing != 0 && s&(entStateSwing|entStateAir) != entStateSwing|entStateAir) {
		return
	}
	held := padHeld()
	for d := range 8 {
		if held&0xf000 != padDirs[d] {
			continue
		}
		turn := 0
		if !demoPlaying {
			turn = camQuarter()
		}
		dir := (d + turn) & 7
		e.vx, e.vz = walkStep[dir][0], walkStep[dir][1]
		s := e.state
		e.state = s | entStateWalk
		if s&(entStateStill|entStateSwing) == entStateStill {
			e.anim = 4
			if s&entStateCarrying != 0 {
				e.anim = 5
			}
			e.animMode = 0
			e.frame |= entFrameNewAnim
		}
		e.t5 = 0
		e.state &^= entStateStill
		return
	}
	s = e.state
	e.state = s | entStateStill
	if s&(entStateWalk|entStateSwing) == entStateWalk {
		e.anim = 2
		if s&entStateCarrying != 0 {
			e.anim = 3
		}
		e.animMode = 0
		e.frame |= entFrameNewAnim
	}
	e.t4 = 0
	e.state &^= entStateWalk
}

// camQuarter is which quarter the camera is turned to, for turning the
// walk with it.
func camQuarter() int {
	a := camAngle
	if a < 0 {
		a += 0x1ff
	}
	return a >> 9
}

// Working storage for playerAct.
var liftPos [3]int16

// playerAct does Square: lifts what is in front or on top, or, carrying
// something, puts it down in front, or throws it when in the air; and
// ends the lifting animation five frames in.
func playerAct(e *entity) {
	if e.t8 > 0 {
		e.t8++
		if e.t8 == 5 {
			e.t8 = 0
			if e.state&0x166 == 0 {
				switch {
				case e.state&entStateStill == 0 && e.state&entStateCarrying == 0:
					e.anim = 4
				case e.state&entStateStill == 0:
					e.anim = 5
				case e.state&entStateCarrying == 0:
					e.anim = 2
				default:
					e.anim = 3
				}
				e.animMode = 0
				e.frame |= entFrameNewAnim
			}
		}
	}
	if e.state&entStateCarrying != 0 && e.child == nil {
		e.t8 = 0
		e.state &^= entStateCarrying
		e.frame |= 0x10
		e.anim, e.animMode = 2, 0
		e.frame |= entFrameNewAnim
		e.t8 = 1
	}
	if padPressed()&yaroze.PADRleft == 0 || e.state&0x144 != 0 {
		return
	}
	if e.state&entStateCarrying == 0 {
		c := e.child
		if c == nil {
			playerTouch(e)
			return
		}
		if c.flagsA&entFlagNoTurn == 0 {
			return
		}
		liftPos = [3]int16{e.x, e.y + int16(e.height) + int16(c.height), e.z}
		if !spaceFree(liftPos[0], liftPos[1], liftPos[2], c.halfW, c.height, c) {
			return
		}
		e.state |= entStateCarrying
		c.t4 = 0
		c.state &^= entStateWalk
		c.dying = 0
		c.state &^= entStateAir
		c.state |= entStateStill
		c.x, c.y, c.z = liftPos[0], liftPos[1], liftPos[2]
		playEffect(0xf)
		e.anim, e.animMode = 10, 1
		e.frame |= entFrameNewAnim
		e.t8 = 1
		return
	}
	c := e.child
	cw, ch := int16(c.halfW), int16(c.height)
	hw := int16(e.halfW)
	y := e.y + int16(e.height) + ch - 1
	a := int(uint16(e.rot.Vy) & 0xfff)
	var x, z int16
	reach := int(cw + hw)
	switch {
	case a-0x200 >= 0 && a-0x200 < 0x400:
		x = e.x - hw - cw
		z = e.z + hw + cw - int16(div1024(reach*(a-0x200)*2))
	case a-0x600 >= 0 && a-0x600 < 0x400:
		z = e.z - hw - cw
		x = e.x - hw - cw + int16(div1024(reach*(a-0x600)*2))
	case a-0xa00 >= 0 && a-0xa00 < 0x400:
		x = e.x + hw + cw
		z = e.z - hw - cw + int16(div1024(reach*(a-0xa00)*2))
	default:
		z = e.z + hw + cw
		x = e.x + hw + cw - int16(div1024(reach*((a-0xe00)&0xfff)*2))
	}
	if !spaceFree(x, y, z, int(cw), int(ch), c) {
		return
	}
	e.t8 = 0
	e.state &^= entStateCarrying
	e.frame |= 0x10
	playEffect(0xc)
	e.anim, e.animMode = 0xb, 1
	e.frame |= entFrameNewAnim
	e.t8 = 1
	if e.state&entStateAir == 0 {
		c.pushX, c.pushZ = e.dx, e.dz
	} else {
		e.pushX, e.pushZ = 0, 0
		c.state |= entStateWalk
		c.t5 = 0
		c.state &^= entStateStill
		if e.state&entStateStill != 0 {
			c.t4 = 3
		}
	}
	c.x, c.y, c.z = x, y, z
	c.rot.Vy = int16(a)
}

// div1024 divides by 1024 rounding toward zero.
func div1024(v int) int {
	if v < 0 {
		v += 0x3ff
	}
	return v >> 10
}

// playerJump starts a jump with Cross, from the ground or whatever the
// player stands on, and plays out the arc.
func playerJump(e *entity) {
	if e.state&entStateAir == 0 {
		if (e.state&entStateHurt == 0 || e.t2 > 9) && e.state&0x144 == 0 && padPressed()&yaroze.PADRdown != 0 {
			playEffect(7)
			e.state |= entStateAir
			if e.parent != entRoot {
				entAttach(e, entRoot)
			}
			e.anim = 6
			if e.state&entStateCarrying != 0 {
				e.anim = 7
			}
			e.animMode = 1
			e.frame |= entFrameNewAnim
		}
		if e.state&entStateAir == 0 {
			e.vy = 0
			return
		}
	}
	e.vy = jumpArc[e.dying]
	e.dying++
	if e.dying == 0x12 {
		e.dying = 0x11
	}
}

// Working storage for playerSwing.
var swingPos [3]int16

// playerSwing swings the sword with Square when the player's hands are
// free, twelve frames with the sword model, hurting anything hittable in
// front on the tenth.
func playerSwing(e *entity) {
	if e.state&(entStateSwing|entStateCarrying) == 0 && e.frame&0x90 == 0 && e.state&0x142 == 0 && padPressed()&yaroze.PADRleft != 0 {
		s := e.state
		e.state = s | entStateSwing
		if s&entStateAir == 0 {
			e.state = s | entStateSwing | entStateStill
			e.t4 = 0
			e.state &^= entStateWalk
		}
		e.kind = 10
		e.anim, e.animMode = 9, 1
		e.frame |= entFrameNewActor
	}
	if e.state&entStateSwing == 0 {
		return
	}
	t := e.t3
	if t == 6 {
		playEffect(0)
	}
	if t == 10 {
		reach := e.halfW * 5 / 3
		swingPos = [3]int16{e.x - int16(div4096(reach*sinA(int(e.rot.Vy)))), e.y, e.z + int16(div4096(reach*cosA(int(e.rot.Vy))))}
		for i := 0; i < frameCount; i++ {
			o := frameList[i]
			if o == nil || o.flagsA&entFlagLiftable == 0 || o == e {
				continue
			}
			if !entOverlapClear(swingPos[0], swingPos[1], swingPos[2], reach, e.height*2/3, o) && o.flagsA&entFlagHittable != 0 && o.state&0x302 == 0 {
				hurt(e, o)
				playEffect(6)
			}
		}
	}
	e.t3 = t + 1
	if t+1 == 0xc {
		e.t3 = 0
		s := e.state
		e.state = s &^ entStateSwing
		e.anim = 2
		if s&entStateWalk != 0 {
			e.anim = 4
		}
		e.kind = 9
		e.animMode = 0
		e.frame |= entFrameNewActor
	}
}

// playerCarry does Circle: drinks a healing item, with sparkles rising
// for 64 frames and hearts refilling over 96, when there is one.
func playerCarry(e *entity) {
	if e.state&entStateUseItem == 0 {
		if padPressed()&yaroze.PADRright != 0 && e.state&0x1a6 == 0 && potions != 0 {
			potions--
			e.state |= entStateUseItem
			playTune(tuneEnd)
			e.anim, e.animMode = 0xe, 1
			e.frame |= entFrameNewAnim
		}
		return
	}
	t := e.t7
	if t < 0x40 && t&1 == 0 {
		entSpawn(0x1f, 0, 0, t>>1, e.x, e.y, e.z, 0)
	}
	e.t7 = t + 1
	if t+1 == 0x60 {
		e.t7 = 0
		e.state &^= entStateUseItem
		e.anim, e.animMode = 2, 0
		e.frame |= entFrameNewAnim
		e.state |= entStateStill
		e.t4 = 0
		e.state &^= entStateWalk
	}
	playerEnt.hp += 4
	if hearts<<4 < playerEnt.hp {
		playerEnt.hp = hearts << 4
	}
}

// playerFall plays out being hurt: the player flickers for thirty frames,
// stopped for the first ten.
func playerFall(e *entity) {
	if e.state&entStateHurt == 0 {
		return
	}
	t := e.t2
	if t == 0 {
		playEffect(4)
		if e.state&entStateSwing != 0 {
			e.t3 = 0
			e.state &^= entStateSwing
			e.frame |= entFrameNewActor
			e.kind = 9
		}
		e.t4 = 0
		s := e.state
		e.state = s&^entStateWalk | entStateStill
		e.anim = 0xc
		if s&entStateCarrying != 0 {
			e.anim = 0xd
		}
		e.animMode = 1
		e.frame |= entFrameNewAnim
	}
	if t&1 == 0 {
		e.frame |= entFrameHidden
	}
	t++
	e.t2 = t
	if t == 10 {
		e.anim = 2
		if e.state&entStateCarrying != 0 {
			e.anim = 3
		}
		e.animMode = 0
		e.frame |= entFrameNewAnim
		e.vx, e.vz = 0, 0
	}
	if t == 0x1e {
		e.t2 = 0
		e.state &^= entStateHurt
	}
}

// playerDie plays out death: the player stands still, flickering, while
// the screen fades to white over 128 frames, and then the day is lost.
func playerDie(e *entity) {
	if e.state&entStateDead == 0 {
		return
	}
	over := screen.Overlay()
	t := e.t9
	e.vx, e.vz = 0, 0
	if t == 0 {
		playEffect(4)
		e.anim = 2
		if e.state&entStateCarrying != 0 {
			e.anim = 3
		}
		e.animMode = 0
		e.frame |= entFrameNewAnim
	}
	if t < 0x80 {
		if t&1 == 0 {
			e.frame |= entFrameHidden
		}
		c := uint8(t * 2)
		fillRect(over.Entry(0), -160, -120, 320, 240, c, c, c, true, 1)
		if t == 0x7f {
			e.frame |= entFrameAtEdge
			flags[storyLost] = 1
			fadeT = 0x10
			fadeTune(tunePort)
			fadeTune(tuneRuins)
			setAmbient(true)
		}
		t++
	} else {
		e.frame |= entFrameHidden
	}
	e.t9 = t
}

// playerHit is the player's onHit: landing on something ends a jump with
// the landing animation, and the player rides it; hitting it from below
// bounces; what they land on hurts them if it can, and a door's key
// opens the door.
func playerHit(e, o *entity, how uint32) {
	if how&hitOnTop == 0 {
		if how&hitFromAbove != 0 && e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
	} else {
		if e.state&entStateAir != 0 {
			e.dying = 0
			e.state &^= entStateAir
			e.t4 = 0
			e.state = e.state&^entStateWalk | entStateStill
			if o.flagsA&entFlagQuiet == 0 || (flags[storyLeft] != 1 && flags[storyScene] == 0) {
				playEffect(3)
			}
			if e.state&entStateSwing == 0 {
				e.anim = 2
				if e.state&entStateCarrying != 0 {
					e.anim = 3
				}
				e.animMode = 0
				e.frame |= entFrameNewAnim
			}
		}
		if e.parent != o {
			entAttach(e, o)
		}
	}
	if o.attack != 0 && e.state&(entStateDead|entStateHurt) == 0 {
		hurt(o, e)
	}
	if o.flagsA&entFlagGoal != 0 && keys != 0 {
		keys--
		doorsOpen[o.param] = 1
	}
}

// playerLand is the player's onLand: landing on the map ends a jump, and
// stone underfoot hurts.
func playerLand(e *entity, how uint32) {
	if how&landHitFloor == 0 {
		if how&landHitCeiling != 0 && e.state&entStateAir != 0 {
			e.dying = 0x12 - e.dying
		}
	} else {
		if e.state&entStateAir != 0 {
			e.dying = 0
			e.state &^= entStateAir
			e.t4 = 0
			e.state = e.state&^entStateWalk | entStateStill
			playEffect(3)
			if e.state&entStateSwing == 0 {
				e.anim = 2
				if e.state&entStateCarrying != 0 {
					e.anim = 3
				}
				e.animMode = 0
				e.frame |= entFrameNewAnim
			}
		}
		if e.parent != entRoot {
			entAttach(e, entRoot)
		}
	}
	if e.frame&entFrameOnStone != 0 && e.state&(entStateDead|entStateHurt) == 0 {
		hurt(nil, e)
	}
}

// playerPosition returns where the player stands.
func playerPosition() (x, y, z int16) {
	return playerEnt.x, playerEnt.y, playerEnt.z
}
