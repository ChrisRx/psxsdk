package main

// The story's scenes, played by the companion, who waits on the ship:
// the quarrel at the start that sends the player off alone, the return
// with the pedestal or without it, the sunset that strands them, and the
// sailing away; and the picture-card scenes over them.

import "go.chrisrx.dev/psx/yaroze"

var (
	pedestalEnt *entity
	scene2On    bool // the picture cards after the quarrel
	scene3On    bool // the credits
)

// pedestalPath is how the pedestal comes down off the player's head onto
// the ship, frame by frame: nothing, down, and forward.
var pedestalPath = [32][2]int16{
	{0, -24}, {0, -24}, {0, -24}, {0, -24}, {0, -24}, {0, -24}, {0, -24}, {0, -24},
	{-16, -24}, {-24, -24}, {-32, -24}, {-48, -24}, {-64, -16}, {-128, -16}, {-128, -16}, {-128, -16},
	{-128, -16}, {-128, -16}, {-128, -8}, {-128, -8}, {-64, -8}, {-32, -8}, {-32, -8}, {-32, -8},
	{-32, 0}, {-32, 0}, {-32, 0}, {-32, 0}, {-32, 0}, {-32, 0}, {-32, 0}, {-32, 0},
}

// companionAnim starts the companion on an animation.
func companionAnim(e *entity, anim, mode int) {
	e.anim, e.animMode = anim, mode
	e.frame |= entFrameNewAnim
}

// playerAnim starts the player on an animation at once.
func playerAnim(anim, mode int) {
	playerEnt.anim, playerEnt.animMode = anim, mode
	entSetAnim(playerEnt)
}

// companionBehavior plays the companion's part in whichever scene is on,
// counting frames in t1 and the scene's steps in t4, each step started
// by the lines of the conversation as they end.
func companionBehavior(e *entity) {
	t := e.t1
	step := e.t4
	next := t + 1
	nextSet := false
	wait := false
	switch {
	case flags[storyLeft] != 0:
		switch flags[storyStep] {
		case 1:
			if step < 1 {
				flags[storyWait] = 0
				step++
				t = 0
				startDemo(0)
				nextCameraKey()
			}
			if t == 0x6e {
				companionAnim(e, 0x1c, 1)
			}
			if t == 200 {
				companionAnim(e, 0x1e, 1)
			}
			if t-0xdc >= 0 && t-0xdc < 0x50 {
				if t == 0xdc {
					companionAnim(e, 0x16, 0)
				}
				if e.rot.Vy > 0x840 {
					e.rot.Vy -= 0x20
				}
			}
			wait = t == 300
		case 2, 6:
			if step < int(flags[storyStep]) {
				flags[storyWait] = 0
				step++
				t = 0
				companionAnim(e, 0x19, 1)
			}
			if t == 0x78 {
				companionAnim(e, 0x16, 0)
				wait = true
			}
		case 3, 7:
			if step < int(flags[storyStep]) {
				flags[storyWait] = 0
				step++
				t = 0
				companionAnim(e, 0x20, 0)
			}
		case 4:
			if step < int(flags[storyStep]) {
				flags[storyWait] = 0
				step++
				t = 0
				companionAnim(e, 0x16, 0)
			}
		case 5:
			if step < int(flags[storyStep]) {
				flags[storyWait] = 0
				step++
				t = 0
				companionAnim(e, 0x1f, 0)
			}
			if t == 0x40 {
				companionAnim(e, 0x16, 0)
				wait = true
			}
		case 8:
			if step < 8 {
				flags[storyWait] = 0
				step++
				t = 0
			}
			if flags[storyShipT] == 0 && flags[storyLeft] == 1 {
				t = 0
				scene2Start()
				flags[storyLeft] = 2
				startDemo(1)
				entSpawn(1, -1, 0, 9, 0x3100, 0x240, 0x1700, 0)
				nextCameraKey()
			}
			if t == 0x78 || t == 0x10e || t == 0x1c2 || t == 0x2b2 {
				nextCameraKey()
			}
			if flags[storyLeft] == 2 {
				e.rot.Vy = int16(atanA(int(e.z)-int(playerEnt.z), int(e.x)-int(playerEnt.x)) + 0x400)
			}
		}
	case flags[storyScene] == 1:
		switch flags[storyStep2] {
		case 0:
			if t == 0xb4 {
				startDialog(1, true)
				companionAnim(e, 0x19, 1)
			}
		case 1:
			if step < 1 {
				flags[storyWait] = 0
				step++
				if c := playerEnt.child; c != nil && c.flagsA&entFlagPedestal != 0 {
					flags[storyWithBox] = 1
				}
				playerEnt.frame |= entFrameAtEdge
				flags[storyStep2] = 0
				next = 1
				nextSet = true
				break
			}
			next = t + 1
		default:
			next = t + 1
		}
	case flags[storyScene] == 2 && flags[storyWithBox] == 1:
		switch flags[storyStep2] {
		case 1:
			if step < 1 {
				flags[storyWait] = 0
				step++
				t = 0
				companionAnim(e, 0x1a, 0)
				nextCameraKey()
			}
			switch {
			case t < 0x14:
				e.z -= 8
			case t < 0x28:
				switch t {
				case 0x14:
					companionAnim(e, 0x21, 1)
				case 0x23:
					companionAnim(e, 0x1b, 0)
				default:
					companionAnim(e, 0x22, 1)
				}
			case t > 0x8b:
				wait = true
			default:
				e.z -= 6
				playerEnt.z -= 6
				playerEnt.child.z -= 6
			}
		case 2:
			v := t - 5
			if step < 2 {
				flags[storyWait] = 0
				step++
				c := playerEnt.child
				pedestalEnt = c
				c.flagsA = 0
				c.behaviors = staticSteps
				c.onLand, c.onHit = nil, nil
				c.dy = 0
				t = 0
				entAttach(c, entRoot)
				playerEnt.t8 = 0
				playerEnt.state &^= entStateCarrying
				v = -5
			}
			if v >= 0 && v < 0x20 {
				if t == 5 {
					companionAnim(e, 0x21, 1)
					playerAnim(2, 0)
				}
				if t == 10 {
					nextCameraKey()
				}
				if t == 0x19 {
					playEffect(0xb)
				}
				if t == 0x14 {
					companionAnim(e, 0x16, 0)
				}
				p := pedestalEnt
				p.y += pedestalPath[t-5][0]
				p.z += pedestalPath[t-5][1]
				if int(p.y) < p.height-0x200 {
					p.y = int16(p.height - 0x200)
				}
				if t-0x14 >= 0 && t-0x14 < 8 {
					playerEnt.rot.Vy += 0x100
				}
			}
			wait = t == 0x25
		case 3:
			started := step < 3
			if started {
				flags[storyWait] = 0
				step++
				t = 0
				nextCameraKey()
			}
			if started || t < 8 {
				playerEnt.rot.Vy -= 0x100
			}
			wait = t == 8
		case 4:
			if step < 4 {
				step++
				t = 0
				companionAnim(e, 0x1f, 0)
			}
			wait = true
		case 5:
			if step < 5 {
				flags[storyWait] = 0
				step++
				next = 1
				nextSet = true
				nextCameraKey()
				scene3Start()
				spawnGull(0x30, 5, 0x2a, -1)
				spawnGull(0x3e, 4, 0x1c, 0x16)
			}
		}
	case flags[storyScene] == 2 && flags[storyWithBox] == 0:
		switch flags[storyStep2] {
		case 1:
			started := step < 1
			if started {
				flags[storyWait] = 0
				step++
				t = 0
			}
			if started || t < 8 {
				playerEnt.rot.Vy += 0x40
			}
			wait = t == 8
		case 2:
			started := step < 2
			if started {
				flags[storyWait] = 0
				step++
				t = 0
				nextCameraKey()
			}
			if started || t < 0x21 {
				if t == 0 {
					companionAnim(e, 0x1a, 0)
				}
				e.z += 8
			}
			if t == 0x21 {
				companionAnim(e, 0x16, 0)
				wait = true
			}
		case 3:
			started := step < 3
			if started {
				flags[storyWait] = 0
				step++
				t = 0
			}
			if started || t < 6 {
				playerEnt.rot.Vy += 0x100
			}
			switch t {
			case 8:
				playerAnim(10, 1)
			case 0xc:
				playerAnim(0xb, 1)
			case 0x10:
				playerAnim(2, 0)
			}
			if t-0xc >= 0 && t-0xc < 0xe {
				e.z -= 0x20
				e.rot.Vx -= 0x10
			}
			wait = t == 0x1a
		case 4:
			if step < 4 {
				step++
				t = 0
				companionAnim(e, 0x1f, 0)
			}
			wait = true
		case 5:
			if step < 5 {
				flags[storyWait] = 0
				step++
				next = 1
				nextSet = true
				nextCameraKey()
				scene3Start()
				spawnGull(0x30, 3, 0x14, 0x15)
				spawnGull(0x3e, 4, 0xe, -1)
			}
		}
	case flags[storyLost] == 1:
		switch {
		case t == 100:
			companionAnim(e, 0x19, 1)
			nextCameraKey()
		case t-0xbe >= 0 && t-0xbe < 8:
			if t == 0xbe {
				companionAnim(e, 0x16, 1)
			}
			e.rot.Vy += 0x80
		case t == 0xd2:
			companionAnim(e, 0x1d, 1)
		case t == 0xfa:
			companionAnim(e, 0x1c, 1)
		case t == 0x172:
			companionAnim(e, 0x18, 0)
			flags[storyEnding] = 1
		}
		if t == 0x14a {
			spawnGull(0x30, 6, 0x1c, -1)
		}
		if t == 0x16b {
			spawnGull(0x36, 2, 0x17, -1)
		}
	}
	if wait {
		flags[storyWait] = 1
	}
	// The counter goes on from where the case left it, which may be where
	// it reset it, unless the case set where it goes on from.
	if !nextSet {
		next = t + 1
	}
	e.t1 = next
	e.t4 = step
}

// startDemo plays recording n of presses through the controller, for the
// scenes where the player walks on their own.
func startDemo(n int) {
	demoPlaying = true
	demo = demos[n]
}

// scene2Start starts the picture cards after the quarrel.
func scene2Start() {
	sceneT = 0
	scene2On = true
	setAmbient(false)
	playTune(tunePort)
}

// scene3Start starts the credits.
func scene3Start() {
	sceneT = -0xd2
	scene3On = true
	setAmbient(false)
	playTune(tunePort)
}

// A card is one of the second scene's pictures: where it goes and when it
// is up.
type card struct {
	y, from, to int16
}

var scene2Cards = [10]card{
	{112, 32, 160}, {128, 32, 160}, {120, 160, 384}, {120, 384, 480}, {112, 496, 608},
	{128, 496, 608}, {112, 592, 704}, {128, 592, 704}, {112, 688, 800}, {128, 688, 800},
}

// scene2Frame draws the cards that are up, fading in and out, and ends
// after 800 frames.
func scene2Frame() {
	t := sceneT
	sceneT++
	over := screen.Overlay()
	for i := range scene2Cards {
		c := &scene2Cards[i]
		if int(c.from) > t || t > int(c.to) {
			continue
		}
		tex := &panels[23+i]
		bright := (t - int(c.from)) * 4
		if t-int(c.from) > 0x1f {
			bright = (int(c.to) - t) * 4
			if int(c.to)-t > 0x1f {
				bright = 0x80
			}
		}
		x := -int(tex.w) / 2
		y := int(c.y) - int(tex.h)/2 - 0x78
		cardSprite(over.Entry(0), int16(x), int16(y), tex, uint8(bright), 1)
	}
	if t == 800 {
		scene2On = false
	}
}

// cardSprite links a semi-transparent sprite of the texture, lit c, with
// its texture page at the rate, at tag.
func cardSprite(tag *uint32, x, y int16, tex *texture, c uint8, abr uint32) {
	p := yaroze.AllocPacket(5)
	p[0] = 4 << 24
	p[1] = 0x66<<24 | uint32(c)<<16 | uint32(c)<<8 | uint32(c)
	p[2] = xy(x, y)
	p[3] = uint32(tex.u) | uint32(tex.v)<<8 | uint32(tex.clut())<<16
	p[4] = uint32(uint16(tex.w)) | uint32(uint16(tex.h))<<16
	yaroze.LinkTag(tag, p)
	q := yaroze.AllocPacket(2)
	q[0] = 1 << 24
	q[1] = 0xe1000200 | abr<<5 | uint32(tex.tpageY>>4&0x10) | uint32((tex.tpageX&0x3ff)>>6) | uint32((tex.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, q)
}

// A credit is one line of the roll: when it starts, its texture, and how
// it is placed: 0 centered, 1 right of the middle, 2 left of it.
type credit struct {
	at    int32
	tex   int32
	align int32
}

var credits = [16]credit{
	{0, 25, 0}, {120, 26, 0}, {200, 27, 0}, {214, 28, 0}, {300, 29, 0}, {314, 30, 0},
	{400, 31, 0}, {414, 32, 0}, {600, 37, 0}, {614, 38, 0}, {630, 39, 0}, {800, 33, 0},
	{900, 34, 1}, {900, 35, 2}, {1050, 40, 0}, {1200, 36, 0},
}

// scene3Frame rolls the credits up the screen, fades the picture to
// black, and starts the game again from the opening.
func scene3Frame() {
	t := sceneT
	sceneT++
	over := screen.Overlay()
	for i := range credits {
		c := &credits[i]
		tex := &panels[c.tex]
		y := int32(t/2) - c.at
		if y < 0 || y > int32(tex.h)+0xf0 {
			continue
		}
		var x int16
		switch c.align {
		case 0:
			x = -tex.w / 2
		case 1:
			x = 0x10 - tex.w
		case 2:
			x = 0x10
		}
		cardSprite(over.Entry(0), x, int16(0x78-y), tex, 0x80, 1)
	}
	switch {
	case t-0x100 >= 0 && t-0x100 < 0x100:
		c := uint8(t - 0x100)
		fillRect(over.Entry(0), -160, -120, 320, 240, c, c, c, true, 2)
	case t >= 0x200:
		fillRect(over.Entry(0), -160, -120, 320, 240, 0, 0, 0, false, 0)
	}
	if t == 0xc08 {
		fadeTune(tunePort)
	}
	if t == 0xd34 {
		scene3On = false
		flags[flagStart] = 1
		nextCameraKey()
		flags[flagStart] = 1
	}
}
