package main

// The controller as the original read it: what is held, what was just
// pressed, and a repeat for held buttons that fires on the eighth frame
// and every other frame after, so that menus scroll; and the playback of
// recorded presses for the title's demonstration.

import "go.chrisrx.dev/psx/yaroze"

// A padState is one controller's reading.
type padState struct {
	held     yaroze.Buttons
	pressed  yaroze.Buttons
	repeat   yaroze.Buttons
	heldRaw  yaroze.Buttons
	pressRaw yaroze.Buttons
	rptRaw   yaroze.Buttons
	last     yaroze.Buttons
	repeated yaroze.Buttons
	count    int
	carry    yaroze.Buttons
}

var (
	pads [2]padState

	// demoPlaying plays recorded presses from demo in place of the first
	// controller's buttons other than Start and Select; demoOnly keeps
	// only Start and Select from the controller.
	demoPlaying bool
	demoOnly    bool
	demo        []uint16
)

// padUpdate reads both controllers for the frame.
func padUpdate() {
	for i := range pads {
		p := &pads[i]
		held := yaroze.PadRead() >> (16 * i) & 0xffff
		if i == 0 && demoPlaying {
			if len(demo) == 0 {
				demoPlaying = false
			} else {
				held = yaroze.Buttons(demo[0]) | held&0x900
				demo = demo[1:]
			}
		}
		pressed := held & (held ^ p.last)
		p.carry |= pressed
		fresh := p.carry
		if fresh == 0 {
			fresh = held & p.repeated
		} else {
			p.carry = 0
		}
		p.rptRaw = 0
		if p.repeated == fresh {
			p.count++
			if p.count > 10 {
				p.count = 8
			}
			if p.count == 8 {
				p.rptRaw = fresh
			}
		} else {
			p.repeated = fresh
			p.count = 0
			p.rptRaw = fresh
		}
		p.last = held
		p.heldRaw, p.pressRaw = held, pressed
		if !demoPlaying && demoOnly {
			p.held, p.pressed, p.repeat = held&0x900, pressed&0x900, p.rptRaw&0x900
		} else {
			p.held, p.pressed, p.repeat = held, pressed, p.rptRaw
		}
	}
}

// padHeld and padPressed are the first controller's buttons held and just
// pressed, as the demonstration allows them; the Raw forms are the
// controller's own, and the repeat was only ever read from it.
func padHeld() yaroze.Buttons       { return pads[0].held }
func padPressed() yaroze.Buttons    { return pads[0].pressed }
func padRepeat() yaroze.Buttons     { return pads[0].rptRaw }
func padHeldRaw() yaroze.Buttons    { return pads[0].heldRaw }
func padPressedRaw() yaroze.Buttons { return pads[0].pressRaw }
