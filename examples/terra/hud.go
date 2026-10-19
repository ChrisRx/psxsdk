package main

// The panel over the game: a row of hearts for the player's health, the
// counts of healing items and keys, and the clock counting the day down,
// which kills the player when it runs out.

import "go.chrisrx.dev/psx/yaroze"

// hudFrame draws the panel and counts the clock down.
func hudFrame() {
	over := screen.Overlay()
	tag := over.Entry(1)
	heart := &things[42]
	hp := playerEnt.hp
	for i := range 20 {
		p := yaroze.AllocPacket(5)
		p[0] = 4 << 24
		x := int16((i%10)*8 - 0x90)
		y := int16(-0x65)
		if i > 9 {
			y = -0x5d
		}
		p[2] = xy(x, y)
		p[4] = 8 | 8<<16
		u := uint32(heart.u)
		code := uint32(0x64)
		var c uint32
		if i < hearts {
			switch {
			case hp >= 8:
			case hp == 0:
				u += 0x10
			default:
				u += 8
			}
			code = 0x65
		} else {
			u += 0x10
			code = 0x66
			c = 0x404040
		}
		p[1] = code<<24 | c
		p[3] = u | uint32(heart.v)<<8 | uint32(heart.clut())<<16
		yaroze.LinkTag(tag, p)
		hp -= 0x10
		if hp < 0 {
			hp = 0
		}
	}
	for i := range 2 {
		icon := &things[44+i]
		p := yaroze.AllocPacket(5)
		p[0] = 4 << 24
		p[1] = 0x65 << 24
		p[2] = xy(-0x3c, int16(i*0xc-0x68))
		p[3] = uint32(icon.u) | uint32(icon.v)<<8 | uint32(icon.clut())<<16
		p[4] = 0x20 | 0x10<<16
		yaroze.LinkTag(tag, p)
	}
	digits := &things[43]
	for i := range 2 {
		n := potions
		if i == 0 {
			n = keys
		}
		p := yaroze.AllocPacket(5)
		p[0] = 4 << 24
		p[1] = 0x65 << 24
		p[2] = xy(-0x20, int16(i*0xc-0x68))
		p[3] = uint32(uint8(int(digits.u)+n*6)) | uint32(digits.v)<<8 | uint32(digits.clut())<<16
		p[4] = 6 | 9<<16
		yaroze.LinkTag(tag, p)
	}
	clock := &things[46]
	p := yaroze.AllocPacket(5)
	p[0] = 4 << 24
	p[1] = 0x65 << 24
	p[2] = xy(-0x90, -0x53)
	p[3] = uint32(clock.u) | uint32(clock.v)<<8 | uint32(clock.clut())<<16
	p[4] = 8 | 8<<16
	yaroze.LinkTag(tag, p)
	secs := timeLeft / 30
	var face [5]int
	face[0] = secs % 6000 / 600
	face[1] = secs % 600 / 60
	face[2] = 10
	face[3] = secs % 60 / 10
	face[4] = secs % 10
	for i := range 5 {
		p := yaroze.AllocPacket(5)
		p[0] = 4 << 24
		p[1] = 0x65 << 24
		p[2] = xy(int16(i*6-0x87), -0x53)
		p[3] = uint32(uint8(int(digits.u)+face[i]*6)) | uint32(digits.v)<<8 | uint32(digits.clut())<<16
		p[4] = 6 | 9<<16
		yaroze.LinkTag(tag, p)
	}
	timeLeft--
	if timeLeft < 0 {
		timeLeft = 0
		if playerEnt.state&entStateDead == 0 {
			playerEnt.state |= entStateDead
			playerEnt.t9 = 1
		}
	}
	q := yaroze.AllocPacket(2)
	q[0] = 1 << 24
	q[1] = 0xe1000200 | 1<<5 | uint32(heart.tpageY>>4&0x10) | uint32((heart.tpageX&0x3ff)>>6) | uint32((heart.tpageY&0x200)<<2)
	yaroze.LinkTag(tag, q)
}
