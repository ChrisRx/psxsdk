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

// PadKind is what a controller is, which its reply's first byte says: its
// type in the high four bits and the half-words of data that follow in the
// low four.
type PadKind uint8

const (
	PadDigital        PadKind = 0x41 // the standard controller: two button bytes
	PadMouse          PadKind = 0x12 // buttons, then X and Y movement
	PadNeGcon         PadKind = 0x23 // buttons, then twist, I, II and L
	PadAnalogJoystick PadKind = 0x53 // buttons, then the sticks: right X, Y, left X, Y
	PadGunCon         PadKind = 0x63 // buttons, then the X and Y the gun points at, 16 bits each
	PadAnalog         PadKind = 0x73 // the analog controller in analog mode, laid out as the joystick
)

// Mouse buttons, as bits of the buttons Press takes.
const (
	MouseRight = 1 << 10
	MouseLeft  = 1 << 11
)

// GunCon buttons.
const (
	GunConA       = 1 << 3
	GunConTrigger = 1 << 13
	GunConB       = 1 << 14
)

// Pad is a controller: by default the standard digital one, or with
// NewPad another kind, whose sticks, movement or aim the test sets.
type Pad struct {
	Kind    PadKind
	buttons uint16 // active low, as sent

	// axes are the four bytes after the buttons of an analog controller,
	// a joystick or a neGcon, in the order they are sent.
	axes [4]uint8
	// dx, dy are a mouse's movement since it was last read.
	dx, dy int
	// gunX, gunY are where a GunCon points, in its own units.
	gunX, gunY uint16

	reply [8]uint8 // what the pad sends after its ID, made as a transfer starts
	n     int      // bytes of reply in all, the ID's two included
	state int      // bytes sent so far
	tx    uint8
	ready bool
}

// NewPad returns a controller of the given kind, its buttons up, its sticks
// centered, unmoved and aimed at nothing.
func NewPad(kind PadKind) *Pad {
	return &Pad{Kind: kind, buttons: 0xffff, axes: [4]uint8{0x80, 0x80, 0x80, 0x80}, tx: 0xff, ready: true}
}

// Press holds buttons down until they are released.
func (p *Pad) Press(buttons uint16) { p.buttons &^= buttons }

// Release lets buttons go.
func (p *Pad) Release(buttons uint16) { p.buttons |= buttons }

// SetAxes sets the four bytes after the buttons: an analog controller's or a
// joystick's right X, right Y, left X and left Y, 0x80 centered, or a
// neGcon's twist, I, II and L.
func (p *Pad) SetAxes(a, b, c, d uint8) { p.axes = [4]uint8{a, b, c, d} }

// Move moves a mouse, by dx and dy counts, which it reports at its next read
// and then forgets, as a mouse does.
func (p *Pad) Move(dx, dy int) { p.dx += dx; p.dy += dy }

// Aim points a GunCon at x, y in its own units: x counts the GPU's dot
// clock from the horizontal sync, about 77 to 461 across a 320-wide
// picture, and y the line, about 25 to 264.
func (p *Pad) Aim(x, y uint16) { p.gunX, p.gunY = x, y }

func newDigitalPad() *Pad { return NewPad(PadDigital) }

// prepare makes the reply a transfer sends: the ID, 5Ah, the buttons, and
// what the kind sends after them.
func (p *Pad) prepare() {
	kind := p.Kind
	if kind == 0 {
		kind = PadDigital
	}
	r := p.reply[:0]
	r = append(r, uint8(kind), 0x5a, uint8(p.buttons), uint8(p.buttons>>8))
	switch kind {
	case PadMouse:
		clamp := func(v int) uint8 { return uint8(int8(max(min(v, 127), -128))) }
		r = append(r, clamp(p.dx), clamp(p.dy))
		p.dx, p.dy = 0, 0
	case PadNeGcon, PadAnalogJoystick, PadAnalog:
		r = append(r, p.axes[:]...)
	case PadGunCon:
		r = append(r, uint8(p.gunX), uint8(p.gunX>>8), uint8(p.gunY), uint8(p.gunY>>8))
	}
	p.n = len(r)
}

// read clocks out the next byte of the pad's reply: high impedance while
// the address byte goes out, then the reply.
func (p *Pad) read() uint8 {
	if p.state == 0 {
		p.prepare()
		p.tx = 0xff
		p.ready = true
		p.state = 1
		return p.tx
	}
	v := p.reply[p.state-1]
	if p.state == p.n {
		// The last byte: the pad has nothing more to send.
		p.ready = false
		p.state = 0
		return v
	}
	p.ready = true
	p.state++
	p.tx = v
	return v
}

// write receives a command byte. Only 43h, which enters configuration mode
// on an analog pad, does anything: these pads answer it by going quiet.
func (p *Pad) write(v uint16) {
	if v == 0x43 {
		p.tx = 0xff
		p.ready = false
		p.state = 0
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
	rxData                 uint32 // the byte the last exchange received
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
	if s.ctrl&joyOutput == 0 && s.ctrl&joyRXEnable == 0 {
		return 0xffffffff
	}
	return s.rxData
}

// tx sends a byte, and takes the byte the device sends back at the same
// time, which rx then returns: the exchange happens as the byte goes out,
// as on the console, so a program that reads the reply twice, as the BIOS
// does after a card write's checksum, gets the same byte twice rather than
// clocking the device on. psxe clocked the device on each read.
func (s *sio0) tx(v uint16) {
	if s.ctrl&joyTXEnable == 0 {
		return
	}
	s.rxData = 0xffffffff
	slot := s.slot()
	if s.dest[slot] == 0 {
		if v != joyDestPad && v != joyDestCard {
			return
		}
		s.dest[slot] = int(v)
		if v == joyDestPad && s.pads[slot] == nil || v == joyDestCard && s.cards[slot] == nil {
			s.dest[slot] = 0
			return
		}
		// The address byte: the device answers it with high impedance.
		if v == joyDestPad {
			s.rxData = uint32(s.pads[slot].read())
		} else {
			s.rxData = uint32(s.cards[slot].read())
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
		if p.ready {
			s.rxData = uint32(p.read())
		}
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
		s.rxData = uint32(c.read())
		if !c.ready {
			s.dest[slot] = 0
		}
		// psxe acknowledges a card's byte later than a pad's.
		if s.ctrl&joyAckIRQ != 0 {
			s.irqBit = true
			s.cyclesUntilIRQ = 2 * joyIRQDelay
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
