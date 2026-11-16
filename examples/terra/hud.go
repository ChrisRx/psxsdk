package main

// The panel over the game: a row of hearts for the player's health, the
// counts of healing items and keys, and the clock counting the day down,
// which kills the player when it runs out.

import "go.chrisrx.dev/psx/gpu"

// hudFrame draws the panel and counts the clock down.
func hudFrame() {
	heart := &things[42]
	hp := playerEnt.hp
	for i := range 20 {
		p := over.Packet[gpu.Sprite](1)
		x := int16((i%10)*8 - 0x90)
		y := int16(-0x65)
		if i > 9 {
			y = -0x5d
		}
		p.XY = gpu.XY{X: x, Y: y}
		p.W, p.H = 8, 8
		u := uint32(heart.u)
		if i < hearts {
			switch {
			case hp >= 8:
			case hp == 0:
				u += 0x10
			default:
				u += 8
			}
			p.Color = gpu.Color{}
			p.Code |= gpu.CodeRaw
		} else {
			u += 0x10
			p.Color = gray(0x40)
			p.Code |= gpu.CodeTranslucent
		}
		// The original added to the corner as a word, so a carry out of
		// U lands in V.
		p.UV = gpu.UV{U: uint8(u), V: heart.v | uint8(u>>8)}
		p.Clut = heart.clut()
		hp -= 0x10
		if hp < 0 {
			hp = 0
		}
	}
	for i := range 2 {
		icon := &things[44+i]
		p := over.Packet[gpu.Sprite](1)
		p.Color = gpu.Color{}
		p.Code |= gpu.CodeRaw
		p.XY = gpu.At(-0x3c, i*0xc-0x68)
		p.UV = gpu.UV{U: icon.u, V: icon.v}
		p.Clut = icon.clut()
		p.W, p.H = 0x20, 0x10
	}
	digits := &things[43]
	for i := range 2 {
		n := potions
		if i == 0 {
			n = keys
		}
		digit(-0x20, i*0xc-0x68, digits, n)
	}
	clock := &things[46]
	p := over.Packet[gpu.Sprite](1)
	p.Color = gpu.Color{}
	p.Code |= gpu.CodeRaw
	p.XY = gpu.At(-0x90, -0x53)
	p.UV = gpu.UV{U: clock.u, V: clock.v}
	p.Clut = clock.clut()
	p.W, p.H = 8, 8
	secs := timeLeft / 30
	var face [5]int
	face[0] = secs % 6000 / 600
	face[1] = secs % 600 / 60
	face[2] = 10
	face[3] = secs % 60 / 10
	face[4] = secs % 10
	for i := range 5 {
		digit(i*6-0x87, -0x53, digits, face[i])
	}
	timeLeft--
	if timeLeft < 0 {
		timeLeft = 0
		if playerEnt.state&entStateDead == 0 {
			playerEnt.state |= entStateDead
			playerEnt.t9 = 1
		}
	}
	over.Packet[gpu.DrawMode](1).Page = heart.mode() | 1<<5
}

// digit links glyph n of the panel's digits, six by nine each in a row,
// at x, y.
func digit(x, y int, digits *texture, n int) {
	p := over.Packet[gpu.Sprite](1)
	p.Color = gpu.Color{}
	p.Code |= gpu.CodeRaw
	p.XY = gpu.At(x, y)
	p.UV = gpu.UV{U: uint8(int(digits.u) + n*6), V: digits.v}
	p.Clut = digits.clut()
	p.W, p.H = 6, 9
}
