package main

// The things in the world: the player and everything that moves or can be
// picked up, talked to, pushed or fought. Each is an entity: a record the
// original kept in a pool of 196, in a tree under a root, so that what
// stands on something moves with it, with a list of behavior steps run
// every frame by its type, and hooks for when it lands on something or
// something lands on it. The physics is the original's: a box swept
// against the map's cells and the other entities a step at a time, and a
// drop against what is under it.
//
// Units are the world's, y up: an entity's y is the bottom of its box,
// its height is half its box's height above and below that, and halfW
// half its width.

import "go.chrisrx.dev/psx/yaroze"

const maxEntities = 196

// An entity is one thing in the world. Its fields keep the original's
// names where it had one, and its counters are named by what they count
// for the types that use them.
type entity struct {
	index    int
	actor    *actor
	kind     int // the type, which is also the model it is drawn with
	anim     int
	animMode int
	animDone bool
	param    int    // what the area's table gave it: a key number, a path, a target
	flagsA   uint32 // what it is: the entFlag constants
	state    uint32 // what it is doing: the entState constants
	frame    uint32 // what happened this frame: the entFrame constants

	t1, t2, t3, t4, t5 int // counters at 0x28 to 0x38
	dying              int // 0x3c: the step through the jump or fall arc
	t7, t8, t9         int // counters at 0x40 to 0x48

	hp, attack, defense int
	halfW, height       int
	x, y, z             int16
	vx, vy, vz          int16 // the velocity the behaviors ask for
	dx, dy, dz          int16 // the movement the frame makes, after collisions
	rot                 yaroze.SVECTOR
	scale               yaroze.VECTOR
	ground              int32 // the height of what is under it
	otOffset            int32 // how many depths behind the mesh it draws

	behaviors []func(*entity)
	onHit     func(e, other *entity, how uint32) // something landed on it, or it on something
	onLand    func(e *entity, how uint32)        // it met the map

	parent, child, lastChild, prev, next *entity
	slot                                 *(*entity) // its place in the frame's list, while there is one

	// Per-type extras at 0xc0: a crate's push, a platform's bounce, a key's
	// gate, the boss's phase.
	pushX, pushZ int16
	bounced      bool
	gate         *entity
	phase        int
}

// What an entity is, flagsA.
const (
	entFlagNoTurn    = 1 << 0  // drawn unturned
	entFlagShadowBig = 1 << 1  // casts the big shadow
	entFlagShadow    = 1 << 2  // casts the small shadow
	entFlagSolid     = 1 << 5  // can be stood on
	entFlagLiftable  = 1 << 7  // the player can lift it, and it collides
	entFlagHittable  = 1 << 8  // the sword hurts it
	entFlagBlastable = 1 << 9  // a blast hurts it
	entFlagBomb      = 1 << 10 // a blast sets it off
	entFlagAlways    = 1 << 12 // runs while the root is paused
	entFlagSign      = 1 << 13 // read when touched
	entFlagChest     = 1 << 14 // opens when touched
	entFlagGoal      = 1 << 15 // a door's key
	entFlagPedestal  = 1 << 16 // lifting it starts the end
	entFlagHeavy     = 1 << 17 // does not fly when hit
	entFlagEdgeSafe  = 1 << 21 // the map's edge stops it rather than the sea
	entFlagQuiet     = 1 << 22 // no sound when it lands on the player
)

// What an entity is doing, state.
const (
	entStateHurt     = 1 << 1
	entStateSwing    = 1 << 2
	entStateWalk     = 1 << 3
	entStateStill    = 1 << 4
	entStateAir      = 1 << 5 // jumping or falling
	entStateUseItem  = 1 << 6
	entStateCarrying = 1 << 7
	entStateDead     = 1 << 8
)

// What happened to an entity this frame, frame.
const (
	entFrameActive   = 1 << 0 // updated; on the root, everything runs
	entFrameVisible  = 1 << 1
	entFrameOnGround = 1 << 2
	entFrameOnThing  = 1 << 3
	entFrameHidden   = 1 << 6
	entFrameTalked   = 1 << 7
	entFrameOnStone  = 1 << 8
	entFrameAtEdge   = 1 << 9
	entFrameNewActor = 1 << 10
	entFrameNewAnim  = 1 << 11
)

var (
	entities   [maxEntities]entity
	freeStack  [maxEntities]uint8
	freeTop    int
	entCount   int
	entRoot    *entity
	playerEnt  *entity
	frameList  [maxEntities]*entity
	frameCount int
	updating   bool

	// mapW etc. as the entities see them, and noclip, which lets them
	// through the map's cells.
	noclip bool
)

// entitiesInit sets up the pool, the root and the player.
func entitiesInit() {
	for i := range freeStack {
		freeStack[i] = uint8(i)
	}
	freeTop = 0
	entCount = 0
	entRoot = entAlloc()
	playerEnt = entSpawn(9, 2, 0, 0, 0, 0, 0, 0)
	playerEnt.hp = 0xa0
}

// entAlloc takes an entity from the pool, cleared, standing still, unscaled.
func entAlloc() *entity {
	i := int(freeStack[freeTop])
	freeTop++
	e := &entities[i]
	*e = entity{index: i}
	e.frame = entFrameActive | entFrameVisible
	e.scale = yaroze.VECTOR{Vx: 0x1000, Vy: 0x1000, Vz: 0x1000}
	entCount++
	return e
}

// entFree takes the entity out of the tree, with everything under it,
// and gives it back to the pool.
func entFree(e *entity) {
	entDetach(e)
	if e.child != nil {
		entFreeChain(e.child)
		e.child, e.lastChild = nil, nil
	}
	if updating && e.slot != nil {
		*e.slot = nil
	}
	if e.actor != nil {
		freeActor(e.actor)
		e.actor = nil
	}
	freeTop--
	freeStack[freeTop] = uint8(e.index)
	entCount--
}

// entFreeChain gives back a chain of siblings and everything under them.
func entFreeChain(e *entity) {
	for e != nil {
		if e.child != nil {
			entFreeChain(e.child)
		}
		next := e.next
		e.parent.child, e.parent.lastChild = nil, nil
		if updating && e.slot != nil {
			*e.slot = nil
		}
		if e.actor != nil {
			freeActor(e.actor)
			e.actor = nil
		}
		freeTop--
		freeStack[freeTop] = uint8(e.index)
		entCount--
		e = next
	}
}

// entDetach takes the entity out from under its parent.
func entDetach(e *entity) {
	p := e.parent
	if p == nil {
		return
	}
	if e.prev == nil {
		if e.next == nil {
			p.child, p.lastChild = nil, nil
		} else {
			e.next.prev = nil
			p.child = e.next
		}
	} else if e.next == nil {
		e.prev.next = nil
		p.lastChild = e.prev
	} else {
		e.prev.next = e.next
		e.next.prev = e.prev
	}
	e.parent, e.prev, e.next = nil, nil, nil
}

// entLink puts the entity, and its siblings after it, under parent, at the
// end of its children.
func entLink(e, parent *entity) {
	last := e
	for {
		last.parent = parent
		if last.next == nil {
			break
		}
		last = last.next
	}
	if parent.child == nil {
		parent.child = e
	} else {
		e.prev = parent.lastChild
		parent.lastChild.next = e
	}
	parent.lastChild = last
}

// entAttach moves the entity under parent.
func entAttach(e, parent *entity) {
	if e.parent != nil {
		entDetach(e)
	}
	entLink(e, parent)
}

// entFlatten lists the entity, those under it and those after it, in
// order, from list[i] on, and returns where the list ends.
func entFlatten(e *entity, i int) int {
	frameList[i] = e
	e.slot = &frameList[i]
	i++
	if e.child != nil {
		i = entFlatten(e.child, i)
	}
	if e.next != nil {
		i = entFlatten(e.next, i)
	}
	return i
}

// entUpdateAll runs the frame for every entity under the root, in the
// order the tree gives them.
func entUpdateAll() {
	if entCount <= 1 {
		return
	}
	updating = true
	frameCount = entCount - 1
	entFlatten(entRoot.child, 0)
	for i := 0; i < frameCount; i++ {
		if frameList[i] != nil {
			entUpdate(frameList[i], &frameList[i])
		}
	}
	updating = false
}

// entUpdate runs the entity's frame: its behaviors, while the root and
// its parent are active or it runs always, then its movement, a new
// actor or animation if asked for, its animation, and its drawing. A
// behavior that frees the entity ends its frame there.
func entUpdate(e *entity, slot *(*entity)) {
	var a *actor
	advance := false
	if (entRoot.frame&entFrameActive != 0 && e.parent.frame&entFrameActive != 0) || e.flagsA&entFlagAlways != 0 {
		e.frame &= entFrameAtEdge
		e.ground = -0x200
		for _, step := range e.behaviors {
			step(e)
			if *slot == nil {
				return
			}
		}
		e.x += e.dx
		e.y += e.dy
		e.z += e.dz
		if e.frame&entFrameNewActor != 0 {
			if e.actor != nil {
				freeActor(e.actor)
			}
			e.actor = newActor(e.kind, e.anim, e.animMode, false)
		} else if e.frame&entFrameNewAnim != 0 {
			entSetAnim(e)
		}
		a = e.actor
		advance = true
	} else {
		a = e.actor
	}
	if a != nil {
		e.animDone = a.animate(advance)
	}
	entDraw(e)
}

// entSetAnim starts the entity's actor on the animation it asks for.
func entSetAnim(e *entity) {
	e.actor.setAnim(e.anim, e.animMode, false)
}

// Working storage for entDraw.
var (
	entPos     [3]int16
	shadowXYZ  [4]yaroze.SVECTOR
	shadowSXY  [3]uint32
	shadowSXY3 uint32
)

// entDraw draws the entity, when it is visible this frame, and its shadow
// on what is under it, smaller the higher it is.
func entDraw(e *entity) {
	if e.frame&entFrameHidden != 0 {
		return
	}
	ot := screen.OT()
	entPos = [3]int16{e.x, -e.y, -e.z}
	if e.frame&entFrameVisible != 0 && e.actor != nil {
		var rot *yaroze.SVECTOR
		if e.flagsA&entFlagNoTurn == 0 {
			rot = &e.rot
		}
		e.actor.place(&entPos, rot, &e.scale)
		e.actor.drawActor(ot, e.otOffset)
	}
	if e.flagsA&(entFlagShadowBig|entFlagShadow) == 0 || e.frame&entFrameHidden != 0 || e.frame&0x20 != 0 {
		return
	}
	tex := &things[2]
	if e.flagsA&entFlagShadowBig != 0 {
		tex = &things[1]
	}
	h := int(e.y) - int(e.ground)
	if h < 0 {
		h += 0x1f
	}
	shrink := int16(h >> 5)
	hw := int16(e.halfW)
	x0, x1 := e.x-hw+shrink, e.x+hw-shrink
	y := -4 - int16(e.ground)
	z0, z1 := hw-e.z-shrink, -hw-e.z+shrink
	shadowXYZ[0] = yaroze.SVECTOR{Vx: x0, Vy: y, Vz: z0}
	shadowXYZ[1] = yaroze.SVECTOR{Vx: x1, Vy: y, Vz: z0}
	shadowXYZ[2] = yaroze.SVECTOR{Vx: x0, Vy: y, Vz: z1}
	shadowXYZ[3] = yaroze.SVECTOR{Vx: x1, Vy: y, Vz: z1}
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	if yaroze.RotTransPers3(&shadowXYZ[0], &shadowXYZ[1], &shadowXYZ[2], &shadowSXY) < 0 {
		return
	}
	p := yaroze.AllocPacket(10)
	p[2], p[4], p[6] = shadowSXY[0], shadowSXY[1], shadowSXY[2]
	yaroze.RotTransPers(&shadowXYZ[3], &shadowSXY3)
	p[8] = shadowSXY3
	otz := yaroze.AverageZ4()
	c := uint32(uint8(0x60 - int(h>>5)))
	p[0] = 9 << 24
	p[1] = 0x2e<<24 | c<<16 | c<<8 | c
	u0, v0 := uint32(tex.u), uint32(tex.v)
	u1, v1 := uint32(uint8(int(tex.u)+int(tex.w)-1)), uint32(uint8(int(tex.v)+int(tex.h)-1))
	p[3] = u0 | v0<<8 | uint32(tex.clut())<<16
	p[5] = u1 | v0<<8 | uint32(tex.tpageY>>4&0x10|(tex.tpageX&0x3ff)>>6|0x40|(tex.tpageY&0x200)<<2)<<16
	p[7] = u0 | v1<<8
	p[9] = u1 | v1<<8
	ot.Link(max((otz-2)>>2, 0), p)
}

// entSpawn makes an entity of a type at x, y, z, facing dir quarters from
// the first, with the animation and the parameter the area's table gives
// it, sets it up as its type is, and puts it under the root. A door that
// is already open is not made again.
func entSpawn(kind, anim, mode, param int, x, y, z int16, dir int) *entity {
	if kind == 0xd && param >= 0 && param < len(doorsOpen) && doorsOpen[param] != 0 {
		return nil
	}
	e := entAlloc()
	e.kind, e.anim, e.animMode, e.param = kind, anim, mode, param
	e.x, e.y, e.z = x, y, z
	e.rot.Vy = int16(uint16((dir-1)*0x400) & 0xfff)
	entSetup(e)
	if kind < 0x1a {
		e.actor = newActor(e.kind, e.anim, e.animMode, false)
	}
	e.ground = int32(y) - int32(e.height)
	entAttach(e, entRoot)
	return e
}

var doorsOpen [5]uint8
