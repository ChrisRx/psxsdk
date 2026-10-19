package main

// The entity physics: a box moved through the map and the other entities,
// and the steps every type's behavior list shares.

import "go.chrisrx.dev/psx/yaroze"

// Bits an onLand hook is told, from entMove.
const (
	landHitCeiling = 1
	landHitFloor   = 2
	landHitWall    = 4
)

// Bits an onHit hook is told, from entBoxCollide: the other entity came
// down on this one, this one came down on the other, or they met sideways.
const (
	hitFromAbove = 8
	hitOnTop     = 0x10
	hitSideways  = 0x20
)

// Working storage for entProject.
var projectV yaroze.SVECTOR
var projectSXY uint32

// entProject marks the entity active and finds out whether any corner of
// its box is on the screen, which makes it visible this frame.
func entProject(e *entity) {
	e.frame |= entFrameActive
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	for i := 0; i < 8; i++ {
		hw, h := int16(e.halfW), int16(e.height)
		if i&1 != 0 {
			hw = -hw
		}
		if i&2 != 0 {
			h = -h
		}
		dz := int16(e.halfW)
		if i&4 != 0 {
			dz = -dz
		}
		projectV = yaroze.SVECTOR{Vx: e.x + hw, Vy: h - e.y, Vz: dz - e.z}
		if yaroze.RotTransPers(&projectV, &projectSXY) < 0 {
			continue
		}
		sx, sy := int16(projectSXY), int16(projectSXY>>16)
		if uint16(sx+0xa0) < 0x141 && uint16(sy+0x78) < 0xf1 {
			e.frame |= entFrameVisible
			return
		}
	}
}

// entProject2 marks the entity active and visible without looking: for
// what is drawn by its behavior rather than its actor.
func entProject2(e *entity) {
	e.frame |= entFrameActive | entFrameVisible
}

// entStep1 takes the velocity as the frame's movement.
func entStep1(e *entity) {
	e.dx, e.dy, e.dz = e.vx, e.vy, e.vz
}

// entStep2 adds the parent's movement, for what rides on something.
func entStep2(e *entity) {
	p := e.parent
	e.dx += p.dx
	e.dy += p.dy
	e.dz += p.dz
}

// entStep4 gives a jumping or falling entity its vertical speed from the
// arc, and a standing one none.
func entStep4(e *entity) {
	if e.state&entStateAir == 0 {
		e.vy = 0
		return
	}
	e.vy = jumpArc[e.dying]
	e.dying++
	if e.dying == 0x12 {
		e.dying = 0x11
	}
}

// jumpArc is the vertical speed through a jump, frame by frame, and then
// the speed of a fall.
var jumpArc = [18]int16{128, 128, 128, 128, 64, 64, 64, 32, 32, -32, -32, -64, -64, -64, -128, -128, -128, -128}

// entStep3 moves a platform along its path: a run of steps, each held for
// a number of frames, round and round, reversing when it is bumped.
func entStep3(e *entity) {
	path := &platformPaths[e.param]
	t := e.t4
	if e.bounced {
		t--
		if t < 0 {
			t = path.hold*len(path.steps) - 1
		}
		e.bounced = false
	}
	step := path.steps[t/path.hold]
	e.vx, e.vy, e.vz = step[0], step[1], step[2]
	t++
	if path.hold*len(path.steps) == t {
		t = 0
	}
	e.t4 = t
}

// A platformPath is the steps a platform cycles through and how many
// frames it takes each.
type platformPath struct {
	hold  int
	steps [][3]int16
}

var platformPaths = [12]platformPath{
	{1, [][3]int16{{0, 0, 0}}},
	{0x20, [][3]int16{{16, 0, 0}, {-16, 0, 0}}},
	{0x30, [][3]int16{{-16, 32, 0}, {16, -32, 0}}},
	{0x20, [][3]int16{{0, 0, 32}, {0, 0, 32}, {0, 0, 32}, {0, 0, 32}, {32, 0, 0}, {0, 0, -32}, {0, 0, -32}, {0, 0, -32}, {0, 0, -32}, {-32, 0, 0}}},
	{0x20, [][3]int16{{0, 0, -32}, {0, 0, -32}, {0, 0, -32}, {0, 0, -32}, {0, 0, -32}, {-32, 0, 0}, {0, 0, 32}, {0, 0, 32}, {0, 0, 32}, {0, 0, 32}, {0, 0, 32}, {32, 0, 0}}},
	{0x48, [][3]int16{{0, 0, 32}, {0, 0, -32}}},
	{0x48, [][3]int16{{0, 0, -32}, {0, 0, 32}}},
	{0x20, [][3]int16{{-32, 0, 0}, {-32, 0, 0}, {-32, 0, 0}, {0, -32, 0}, {0, 32, 0}, {32, 0, 0}, {32, 0, 0}, {32, 0, 0}}},
	{0x40, [][3]int16{{0, 32, 0}, {0, -32, 0}}},
	{0x20, [][3]int16{{0, 32, 0}, {0, -32, 0}}},
	{0x20, [][3]int16{{0, -32, 0}, {0, 32, 0}}},
	{0x20, [][3]int16{{0, 32, 0}, {0, -32, 0}}},
}

// entStepMove moves the entity against the map and then against the other
// entities: the shared step at the end of most behavior lists.
func entStepMove(e *entity) {
	entMove(e)
	entCollideAll(e)
}

// entStepCollide is entStepMove without the map, for what the map cannot
// stop.
func entStepCollide(e *entity) {
	entCollideAll(e)
}

// entStepMapOnly is entStepMove without the other entities.
func entStepMapOnly(e *entity) {
	entMove(e)
}

// entKill starts the entity dying, if it is not standing on the ground.
func entKill(e *entity) {
	if e.frame&entFrameOnGround != 0 || e.state&entStateAir != 0 {
		return
	}
	e.state |= entStateAir
	e.dying = 9
	if e.parent != entRoot {
		entAttach(e, entRoot)
	}
}

// entTurn turns the entity to face the way it moves, when it is moving on
// its own feet.
func entTurn(e *entity) {
	s := e.state
	if s&entStateStill == 0 && s&(entStateSwing|entStateAir) != entStateSwing && s&0x140 == 0 && (s&entStateHurt == 0 || e.t2 > 10) {
		e.rot.Vy = int16(atanA(-int(e.vx), int(e.vz)))
	}
}

// entMove sweeps the entity's box along its movement, an axis at a time,
// stopping it short at the map's cells, and tells its onLand hook what
// it met. Then it finds what ground is under it.
func entMove(e *entity) {
	dx, dy, dz := e.dx, e.dy, e.dz
	hw, h := e.halfW, e.height
	var how uint32
	top := h
	if e.state&entStateCarrying != 0 {
		top = e.child.height*2 + h
	}
	x, y, z := int(e.x), int(e.y), int(e.z)
	if dx != 0 {
		edge := hw
		if dx < 1 {
			edge = -hw
		}
		at := x + int(dx) + edge
		if sweep(e, at, at, y-h, y+top, z-hw, z+hw) {
			if dx < 1 {
				dx = dx - int16(at&0x1ff) + 0x200
			} else {
				dx = dx - int16(at&0x1ff) - 1
			}
			how = landHitWall
		}
	}
	if dz != 0 {
		edge := hw
		if dz < 1 {
			edge = -hw
		}
		at := z + int(dz) + edge
		if sweep(e, x-hw+int(dx), x+hw+int(dx), y-h, y+top, at, at) {
			if dz < 1 {
				dz = dz - int16(at&0x1ff) + 0x200
			} else {
				dz = dz - int16(at&0x1ff) - 1
			}
			how = landHitWall
		}
	}
	if dy == 0 {
		at := y + int(dy) - h - 1
		if sweep(e, x-hw+int(dx), x+hw+int(dx), at, at, z-hw+int(dz), z+hw+int(dz)) {
			how |= landHitFloor
			e.frame |= entFrameOnGround
		}
	} else {
		edge := top
		if dy < 1 {
			edge = -h
		}
		at := y + int(dy) + edge
		if sweep(e, x-hw+int(dx), x+hw+int(dx), at, at, z-hw+int(dz), z+hw+int(dz)) {
			if dy > 0 {
				dy = dy - int16(at&0x1ff) - 1
				how |= landHitCeiling
			} else {
				dy = dy - int16(at&0x1ff) + 0x200
				how |= landHitFloor
				e.frame |= entFrameOnGround
			}
		}
	}
	e.dx, e.dy, e.dz = dx, dy, dz
	if how != 0 && e.onLand != nil {
		e.onLand(e, how)
	}
	if e.frame&entFrameOnThing == 0 {
		findGround(e)
	}
}

// sweep reports whether any cell the box from x0 to x1, y0 to y1 and z0
// to z1 covers blocks the entity: every cell's corner within it, and its
// far edges.
func sweep(e *entity, x0, x1, y0, y1, z0, z1 int) bool {
	hit := false
	for x := x0; x <= x1; x += 0x200 {
		for z := z0; z <= z1; z += 0x200 {
			for y := y0; y <= y1; y += 0x200 {
				if cellBlocked(e, x, y, z) {
					hit = true
				}
				if y1 < y+0x200 && cellBlocked(e, x, y1, z) {
					hit = true
				}
			}
			if z1 < z+0x200 {
				for y := y0; y <= y1; y += 0x200 {
					if cellBlocked(e, x, y, z1) {
						hit = true
					}
					if y1 < y+0x200 && cellBlocked(e, x, y1, z1) {
						hit = true
					}
				}
			}
		}
		if x1 < x+0x200 {
			for z := z0; z <= z1; z += 0x200 {
				for y := y0; y <= y1; y += 0x200 {
					if cellBlocked(e, x1, y, z) {
						hit = true
					}
					if y1 < y+0x200 && cellBlocked(e, x1, y1, z) {
						hit = true
					}
				}
				if z1 < z+0x200 {
					for y := y0; y <= y1; y += 0x200 {
						if cellBlocked(e, x1, y, z1) {
							hit = true
						}
						if y1 < y+0x200 && cellBlocked(e, x1, y1, z1) {
							hit = true
						}
					}
				}
			}
		}
	}
	return hit
}

// floor512 is a coordinate's cell, rounding toward minus infinity as the
// original did with its adjustment before the shift.
func floor512(v int) int {
	if v < 0 {
		v += 0x1ff
	}
	return v >> 9
}

// cellBlocked reports whether the map blocks the entity at x, y, z: off the
// map it does, and marks the entity at the edge, with the sea's sound;
// below the map it does; under the top it does where a cell above the
// point holds anything, with a note of stone for what stone sets off, and
// where the cell above is not an open top; and at the top where the top
// cell is neither marked open nor empty. With noclip, nothing but the
// edges and cells block.
func cellBlocked(e *entity, x, y, z int) bool {
	if x < 0 || z < 0 || x >= mapW<<9 || z >= mapD<<9 {
		if e == nil || e.flagsA&entFlagEdgeSafe == 0 || e.frame&entFrameAtEdge != 0 {
			return true
		}
		e.frame |= entFrameAtEdge
		playEffect(8)
		return true
	}
	if y < 0 {
		t := cell(floor512(x), 0, floor512(z))
		if t == 4 && e != nil && e.flagsA&entFlagEdgeSafe != 0 {
			e.frame |= entFrameOnStone
		}
		return true
	}
	if y < (mapH-1)*0x200 {
		cx, cy, cz := floor512(x), floor512(y)+1, floor512(z)
		if t := cell(cx, cy, cz); t != 0 {
			if t == 4 && e != nil && e.flagsA&entFlagEdgeSafe != 0 {
				e.frame |= entFrameOnStone
			}
			return true
		}
		if noclip {
			return false
		}
		return !cellTop(cx, cy, cz)
	}
	if noclip {
		return false
	}
	cx, cz := floor512(x), floor512(z)
	if cellTop(cx, mapH-1, cz) {
		return false
	}
	return cell(cx, mapH-1, cz) == 0
}

// findGround finds the highest block under the entity's feet, for its
// shadow, and marks it on the ground when its feet are on it.
func findGround(e *entity) {
	if e.flagsA&(entFlagShadowBig|entFlagShadow) == 0 {
		return
	}
	x := floor512(int(e.x) + int(e.dx))
	feet := int(e.y) + int(e.dy) - e.height
	z := floor512(int(e.z) + int(e.dz))
	if feet < 0 {
		return
	}
	for feet >= mapH<<9 {
		feet -= 0x200
	}
	top := floor512(feet)
	y := top
	for ; y >= 0; y-- {
		if t := cell(x, y, z); t > 1 && int32(y<<9) > e.ground {
			e.ground = int32(y << 9)
		}
	}
	if y == top && e.frame&entFrameOnGround != 0 {
		e.frame |= entFrameOnThing
		if e.flagsA&0x40 == 0 {
			e.frame |= entFrameOnThing | 0x20
		}
	}
}

// entCollideAll collides the entity with every solid entity in the frame.
func entCollideAll(e *entity) {
	for i := 0; i < frameCount; i++ {
		o := frameList[i]
		if o != nil && o.flagsA&entFlagLiftable != 0 {
			entCollide(e, o)
		}
	}
}

// entCollide collides the entity with another: against itself, it drops
// what it carries as far as it can; against anything that is not under
// it in the tree, its box against the other's.
func entCollide(e, o *entity) {
	if e == o {
		if o.child != nil && o.dy > 0 {
			dy := dropSweep(o.child, o.dy)
			was := o.dy
			o.dy = dy
			if was != dy && o.onLand != nil {
				o.onLand(o, landHitCeiling)
			}
		}
		return
	}
	for p := o.parent; p != entRoot; p = p.parent {
		if p == e {
			return
		}
	}
	entBoxCollide(e, o)
}

// entBoxCollide collides the entity's box, where its movement takes it,
// with the other's: stopped sideways, landed on top, or hit from below,
// with the onHit hook told which; and, standing on a solid one, takes it
// as its ground.
func entBoxCollide(e, o *entity) {
	hw := e.halfW + o.halfW
	var how uint32
	var hh, ey int
	if e.state&entStateCarrying == 0 {
		hh = e.height + o.height
		ey = int(e.y)
	} else {
		ch := e.child.height
		hh = e.height + ch + o.height
		ey = int(e.y) + ch
	}
	dx := abs(int(e.x) + int(e.dx) - int(o.x))
	dy := abs(ey+int(e.dy)-int(o.y)) - hh
	dz := abs(int(e.z) + int(e.dz) - int(o.z))
	if dx-hw < 0 && dz-hw < 0 {
		inside := dy < 0
		if e.state&entStateAir != 0 {
			inside = dy < 1
		}
		if inside {
			ox := abs(int(e.x)-int(o.x)) - hw
			oz := abs(int(e.z)-int(o.z)) - hw
			if ox >= 0 {
				m := int(e.dx)
				if ox < abs(m) {
					s := int16(ox)
					if m < 1 {
						s = -s
					}
					e.dx = s
					how = hitSideways
				}
			}
			if oz >= 0 {
				m := int(e.dz)
				if oz < abs(m) {
					s := int16(oz)
					if m < 1 {
						s = -s
					}
					e.dz = s
					how = hitSideways
				}
			}
		}
		if dy <= 0 && how == 0 {
			gap := int16(abs(ey-int(o.y))) - int16(hh)
			if int(o.y) < ey {
				e.dy = -gap
				e.frame |= entFrameOnGround
				how = hitOnTop
			} else {
				e.dy = gap
				how = hitFromAbove
			}
		}
	}
	if how != 0 && e.onHit != nil {
		e.onHit(e, o, how)
	}
	if e.flagsA&(entFlagShadowBig|entFlagShadow) != 0 && o.flagsA&entFlagSolid != 0 && e.frame&entFrameOnThing == 0 {
		feet := int(e.y) + int(e.dy)
		if int(e.ground) < feet-e.height && feet-int(o.y) >= 0 {
			ddx := abs(int(e.x) + int(e.dx) - int(o.x))
			ddz := abs(int(e.z) + int(e.dz) - int(o.z))
			if ddx-o.halfW < 0 && ddz-o.halfW < 0 {
				e.ground = int32(o.y) + int32(o.height)
				if e.frame&entFrameOnGround != 0 {
					e.frame |= entFrameOnThing
					if e.flagsA&0x40 == 0 {
						e.frame |= entFrameOnThing | 0x20
					}
				}
			}
		}
	}
}

// dropSweep finds how far down, of dy, the entity carried on another can
// go before the map or another entity, not under it in the tree, stops
// it, and the same for what it carries and what comes after it.
func dropSweep(e *entity, dy int16) int16 {
	dy = dropAgainstMap(e, dy)
	if dy == 0 {
		return 0
	}
	for i := 0; i < frameCount; i++ {
		o := frameList[i]
		if o == nil {
			continue
		}
		if o != entRoot {
			p := o
			for p != e && p != entRoot {
				p = p.parent
			}
			if p != entRoot {
				continue
			}
		}
		dy = dropAgainstEntity(e, o, dy)
		if dy == 0 {
			return 0
		}
	}
	if e.child != nil {
		dy = dropSweep(e.child, dy)
	}
	if e.next != nil {
		dy = dropSweep(e.next, dy)
	}
	return dy
}

// dropAgainstEntity shortens a drop of dy to what the other's box allows.
func dropAgainstEntity(e, o *entity, dy int16) int16 {
	dx := abs(int(e.x) - int(o.x))
	hw := e.halfW + o.halfW
	ddy := abs(int(e.y) + int(dy) - int(o.y))
	hh := e.height + o.height
	dz := abs(int(e.z) - int(o.z))
	if dx-hw < 0 && ddy-hh < 0 && dz-hw < 0 {
		gap := abs(int(e.y)-int(o.y)) - hh
		if gap >= 0 && gap < int(dy) {
			dy = int16(gap)
		}
	}
	return dy
}

// dropAgainstMap shortens a drop of dy to what the map allows under the
// entity's box.
func dropAgainstMap(e *entity, dy int16) int16 {
	hw := e.halfW
	at := int(e.y) + int(dy) + e.height
	if !sweep(nil, int(e.x)-hw, int(e.x)+hw, at, at, int(e.z)-hw, int(e.z)+hw) {
		return dy
	}
	short := int16(int(dy) - at&0x1ff - 1)
	if short < dy {
		return short
	}
	return dy
}

// spaceFree reports whether a box of halfW and halfH at x, y, z is clear
// of the map and of every solid entity but except.
func spaceFree(x, y, z int16, halfW, halfH int, except *entity) bool {
	if sweep(nil, int(x)-halfW, int(x)+halfW, int(y)-halfH, int(y)+halfH, int(z)-halfW, int(z)+halfW) {
		return false
	}
	for i := 0; i < frameCount; i++ {
		o := frameList[i]
		if o != nil && o.flagsA&entFlagLiftable != 0 && o != except && !entOverlapClear(x, y, z, halfW, halfH, o) {
			return false
		}
	}
	return true
}

// entOverlapClear reports whether a box at x, y, z is clear of the
// entity's, when the entity is active.
func entOverlapClear(x, y, z int16, halfW, halfH int, o *entity) bool {
	if o.frame&entFrameActive == 0 {
		return true
	}
	dx := abs(int(o.x) - int(x))
	hw := halfW + o.halfW
	dy := abs(int(o.y) - int(y))
	dz := abs(int(o.z) - int(z))
	if dx-hw < 0 && dy-(halfH+o.height) < 0 {
		return dz-hw >= 0
	}
	return true
}

// Working storage for playerTouch.
var touchPos [3]int16

// playerTouch acts on what is in front of the player when they press
// Square: reads a sign, opens a chest, or lifts what can be lifted onto
// their head, and starts the end when it is the pedestal.
func playerTouch(e *entity) {
	hw := e.halfW
	reach := hw / 3
	tx := e.x - int16(div4096(reach*sinA(int(e.rot.Vy))))
	tz := e.z + int16(div4096(reach*cosA(int(e.rot.Vy))))
	for i := 0; i < frameCount; i++ {
		o := frameList[i]
		if o == nil || o.flagsA&entFlagLiftable == 0 || o == e || entOverlapClear(tx, e.y, tz, hw, e.height*2/3, o) {
			continue
		}
		if o.flagsA&entFlagNoTurn == 0 {
			if o.flagsA&entFlagSign != 0 {
				e.frame |= entFrameTalked
				startDialog(o.param, true)
			}
			if o.flagsA&entFlagChest != 0 && o.state&entStateDead == 0 {
				o.state |= entStateDead
				o.anim, o.animMode = 1, 1
				entSetAnim(o)
				e.frame |= entFrameTalked
				switch {
				case o.param < 3:
					startDialog(4, true)
				case o.param-3 < 2:
					startDialog(5, true)
				case o.param-5 < 10:
					startDialog(6, true)
				}
				playEffect(0xe)
				chestsOpened[o.param] = 1
			}
			continue
		}
		touchPos = [3]int16{e.x, e.y + int16(e.height) + int16(o.height), e.z}
		if !spaceFree(touchPos[0], touchPos[1], touchPos[2], o.halfW, o.height, o) {
			continue
		}
		entAttach(o, e)
		e.state |= entStateCarrying
		o.state |= entStateStill
		o.t4 = 0
		o.state &^= entStateWalk
		o.dying = 0
		o.state &^= entStateAir
		o.x, o.y, o.z = touchPos[0], touchPos[1], touchPos[2]
		e.anim, e.animMode = 10, 1
		e.frame |= entFrameNewAnim
		e.t8 = 1
		if o.flagsA&entFlagPedestal != 0 && flags[storyPedestal] == 0 {
			startDialog(7, true)
			flags[storyPedestal] = 1
			e.anim, e.animMode = 3, 0
			entSetAnim(e)
		}
		playEffect(7)
	}
}

var chestsOpened [16]uint8

// div4096 divides by 4096 rounding toward zero, as the original's shifts
// with their adjustment did.
func div4096(v int) int {
	if v < 0 {
		v += 0xfff
	}
	return v >> 12
}

// div128 divides by 128 rounding toward zero.
func div128(v int) int {
	if v < 0 {
		v += 0x7f
	}
	return v >> 7
}

// hurt hurts the victim by the attacker's attack less a third of the
// victim's defense, or by six less that when the attacker is the world,
// kills it at no hit points left, and knocks it away from the attacker,
// or along its own facing from the world, unless it is heavy.
func hurt(attacker, victim *entity) {
	var d int
	if attacker == nil {
		d = 6 - victim.defense/3
	} else {
		d = attacker.attack/2 - victim.defense/3
	}
	if d > 0 {
		victim.hp -= d
	}
	if victim.hp < 1 {
		victim.hp = 0
		victim.state |= entStateDead
	}
	victim.state |= entStateHurt
	if victim.flagsA&entFlagHeavy != 0 {
		return
	}
	if attacker == nil {
		a := int(int16(int(uint16(victim.rot.Vy)) + 0x800))
		victim.vx = int16(div4096(sinA(a) * -0x20))
		victim.vz = int16(div128(cosA(a)))
		return
	}
	a := atanA(int(victim.z)-int(attacker.z), int(victim.x)-int(attacker.x))
	victim.vx = int16(div128(cosA(a)))
	victim.vz = int16(div128(sinA(a)))
}

// abs is the absolute value.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
