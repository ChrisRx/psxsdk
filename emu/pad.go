package emu

// Controller buttons, as bits of the two button bytes a digital pad sends.
// The pad reports them active low; Pad.Press and Pad.Release take them as
// set bits.
const (
	ButtonSelect   = 1 << 0
	ButtonL3       = 1 << 1
	ButtonR3       = 1 << 2
	ButtonStart    = 1 << 3
	ButtonUp       = 1 << 4
	ButtonRight    = 1 << 5
	ButtonDown     = 1 << 6
	ButtonLeft     = 1 << 7
	ButtonL2       = 1 << 8
	ButtonR2       = 1 << 9
	ButtonL1       = 1 << 10
	ButtonR1       = 1 << 11
	ButtonTriangle = 1 << 12
	ButtonCircle   = 1 << 13
	ButtonCross    = 1 << 14
	ButtonSquare   = 1 << 15
)

// Pad is a digital controller.
type Pad struct {
	buttons uint16 // active low, as sent
	state   int
	tx      uint8
	ready   bool
}

// Press holds buttons down until they are released.
func (p *Pad) Press(buttons uint16) { p.buttons &^= buttons }

// Release lets buttons go.
func (p *Pad) Release(buttons uint16) { p.buttons |= buttons }

func newDigitalPad() *Pad {
	return &Pad{buttons: 0xffff, tx: 0xff, ready: true}
}

// Transfer states: high impedance before the ID, the two ID bytes, the two
// button bytes.
const (
	padHiZ = iota
	padIDLow
	padIDHigh
	padButtonsLow
	padButtonsHigh
)

// read clocks out the next byte of the pad's reply.
func (p *Pad) read() uint8 {
	switch p.state {
	case padHiZ:
		p.tx = 0xff
	case padIDLow:
		p.tx = 0x41 // digital pad
	case padIDHigh:
		p.tx = 0x5a
	case padButtonsLow:
		p.tx = uint8(p.buttons)
	case padButtonsHigh:
		// The last byte: the pad has nothing more to send.
		p.ready = false
		p.state = padHiZ
		return uint8(p.buttons >> 8)
	}
	p.ready = true
	p.state++
	return p.tx
}

// write receives a command byte. Only 43h, which enters configuration mode
// on an analog pad, does anything: a digital pad answers it by going quiet.
func (p *Pad) write(v uint16) {
	if v == 0x43 {
		p.tx = 0xff
		p.ready = false
		p.state = padHiZ
	}
}

// JOY_CTRL bits.
const (
	joyTXEnable = 0x0001
	joyOutput   = 0x0002
	joyRXEnable = 0x0004
	joyAck      = 0x0010
	joyAckIRQ   = 0x1000
	joySlot     = 0x2000
	joyStatIRQ  = 0x0200
	joyIRQDelay = 512
	joyDestPad  = 0x01
	joyDestCard = 0x81
)

// sio0 is the serial port at 1F801040 that the controllers and memory cards
// share, as psxe models it: the first byte of a transfer picks a pad or a
// memory card, and each byte after it is exchanged with that device. psxe
// connects a digital pad to port 1 and nothing else; its memory card
// support is disabled, so a card is only there once InsertCard puts one in.
type sio0 struct {
	pads  [2]*Pad
	cards [2]*MemoryCard
	dest  [2]int

	mode, ctrl, baud, stat uint16
	cyclesUntilIRQ         int
	irqBit                 bool

	// psxe counts the cycles until the interrupt down every step; it is
	// only counted down when it is due, from cycles, the machine's cycle
	// count: synced is the count it was last counted down to, and due the
	// count it runs out at.
	cycles *uint64
	synced uint64
	due    uint64
	wake   *bool // set when due changes

	ic *ic
}

// newSIO0 returns the port with the given pads connected, counting cycles
// from the given clock and setting wake when it changes what it has due.
func newSIO0(ic *ic, cycles *uint64, wake *bool, pads [2]*Pad) *sio0 {
	return &sio0{ic: ic, cycles: cycles, due: ^uint64(0), wake: wake, pads: pads}
}

func (s *sio0) slot() int { return int(s.ctrl>>13) & 1 }

func (s *sio0) rx() uint32 {
	slot := s.slot()
	if s.dest[slot] == 0 {
		return 0xffffffff
	}
	if s.ctrl&joyOutput == 0 && s.ctrl&joyRXEnable == 0 {
		return 0xffffffff
	}
	if s.dest[slot] == joyDestPad {
		p := s.pads[slot]
		if p == nil {
			s.dest[slot] = 0
			return 0xffffffff
		}
		v := p.read()
		if !p.ready {
			s.dest[slot] = 0
		}
		return uint32(v)
	}
	c := s.cards[slot]
	if c == nil {
		s.dest[slot] = 0
		return 0xffffffff
	}
	v := c.read()
	if !c.ready {
		s.dest[slot] = 0
	}
	return uint32(v)
}

func (s *sio0) tx(v uint16) {
	if s.ctrl&joyTXEnable == 0 {
		return
	}
	slot := s.slot()
	if s.dest[slot] == 0 {
		if v != joyDestPad && v != joyDestCard {
			return
		}
		s.dest[slot] = int(v)
		if v == joyDestPad && s.pads[slot] == nil || v == joyDestCard && s.cards[slot] == nil {
			return
		}
		if s.ctrl&joyAckIRQ != 0 {
			s.cyclesUntilIRQ = joyIRQDelay
		}
		return
	}

	if s.dest[slot] == joyDestPad {
		p := s.pads[slot]
		if p == nil {
			s.dest[slot] = 0
			return
		}
		p.write(v)
		if !p.ready {
			s.dest[slot] = 0
		}
	} else {
		c := s.cards[slot]
		if c == nil {
			s.dest[slot] = 0
			return
		}
		c.write(uint8(v))
		// psxe acknowledges a card's byte later than a pad's, and leaves
		// ending the transfer to the read.
		if s.ctrl&joyAckIRQ != 0 {
			s.irqBit = true
			s.cyclesUntilIRQ = 2 * joyIRQDelay
			return
		}
		if !c.ready {
			s.dest[slot] = 0
		}
		return
	}
	if s.ctrl&joyAckIRQ != 0 {
		s.irqBit = true
		s.cyclesUntilIRQ = joyIRQDelay
	}
}

func (s *sio0) writeCtrl(v uint16) {
	slot := s.slot()
	s.ctrl = v
	if s.ctrl&joyOutput == 0 {
		s.ctrl &^= joySlot
		// Deselecting ends a card's transfer: the card in the slot that
		// was selected, as psxe has it.
		if c := s.cards[slot]; c != nil {
			c.reset()
		}
	}
	// Acknowledging clears the error and interrupt bits of JOY_STAT.
	if s.ctrl&joyAck != 0 {
		s.stat &= 0xfdc7
		s.ctrl &^= joyAck
	}
}

func (s *sio0) read(off uint32) uint32 {
	switch off {
	case 0:
		return s.rx()
	case 4:
		// Always ready to send, and with a byte to receive.
		return uint32(s.stat | 7)
	case 8:
		return uint32(s.mode)
	case 10:
		return uint32(s.ctrl)
	case 14:
		return uint32(s.baud)
	}
	return 0
}

func (s *sio0) Read32(off uint32) uint32 { return s.read(off) }
func (s *sio0) Read16(off uint32) uint16 { return uint16(s.read(off)) }
func (s *sio0) Read8(off uint32) uint8   { return uint8(s.read(off)) }

func (s *sio0) write(off uint32, v uint16) {
	// A transfer restarts the count, so what has run off it is counted
	// first, and the count is scheduled anew after.
	s.sync()
	defer s.schedule()
	switch off {
	case 0:
		s.tx(v)
	case 8:
		s.mode = v
	case 10:
		s.writeCtrl(v)
	case 14:
		s.baud = v
	}
}

// psxe's registers are 16 bits wide, and it truncates a 32-bit write to them.
func (s *sio0) Write32(off, v uint32)        { s.write(off, uint16(v)) }
func (s *sio0) Write16(off uint32, v uint16) { s.write(off, v) }
func (s *sio0) Write8(off uint32, v uint8)   { s.write(off, uint16(v)) }

// sync counts the cycles since the count was last counted down off it.
// They cannot have run it out, or update would have been called at the
// step they did.
func (s *sio0) sync() {
	n := *s.cycles - s.synced
	s.synced = *s.cycles
	if s.cyclesUntilIRQ > 0 {
		s.cyclesUntilIRQ -= int(n)
	}
}

// schedule works out when the count runs out, if one is running.
func (s *sio0) schedule() {
	*s.wake = true
	s.due = ^uint64(0)
	if s.cyclesUntilIRQ > 0 {
		s.due = s.synced + uint64(s.cyclesUntilIRQ)
	}
}

// update raises the acknowledge interrupt, which the machine calls for when
// the cycle count reaches due. Counting the cycles off all at once comes to
// the same as psxe counting a step's at a time, so it is raised on the
// step psxe raises it.
func (s *sio0) update() {
	n := *s.cycles - s.synced
	s.synced = *s.cycles
	if s.cyclesUntilIRQ != 0 {
		s.cyclesUntilIRQ -= int(n)
		if s.cyclesUntilIRQ <= 0 {
			s.ack()
		}
	}
	s.schedule()
}

func (s *sio0) ack() {
	s.ic.raise(irqPad)
	if s.irqBit {
		s.stat |= joyStatIRQ
		s.irqBit = false
	}
	s.cyclesUntilIRQ = 0
}
