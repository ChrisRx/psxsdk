package emu

// SIO_STAT bits.
const (
	sioTXReady = 1 << 0 // the transmit buffer is free
	sioRXReady = 1 << 1 // the receive FIFO holds a byte
	sioTXIdle  = 1 << 2 // nothing is being sent
	sioOverrun = 1 << 4 // a byte arrived to a full FIFO, and was lost
	sioDSR     = 1 << 7 // the host's DTR
	sioCTS     = 1 << 8 // the host's RTS, after ctsAt
	sioIRQ     = 1 << 9
)

// SIO_CTRL bits.
const (
	sioTXEnable = 1 << 0
	sioRXEnable = 1 << 2
	sioAck      = 1 << 4 // clears SIO_STAT's error and interrupt bits
	sioRTS      = 1 << 5 // the console's RTS, the host's CTS
	sioReset    = 1 << 6
	sioTXIRQ    = 1 << 10
	sioRXIRQ    = 1 << 11
)

// sioFIFO is how many received bytes the port holds.
const sioFIFO = 8

// rtsPulse is how long the host's RTS stays off, in cycles, when a
// SerialLine turns it off and straight back on: about 5us, which is what
// the few port writes SIOCONS's receive interrupt drops it for took on a
// PC of the time. The line runs the machine only while it waits, so a
// pulse would otherwise last no time at all.
const rtsPulse = 170

// sio1 is the serial port at 1F801050, the one on the back of the console
// that the Net Yaroze's cable plugs into. psxe does not emulate it, and
// answers only its status, as ready to send and idle, and its control
// register, as zero; this follows the hardware as psx-spx documents it.
//
// A byte takes as long to send as the port's settings say: a start bit,
// the data bits, the parity bit if any and the stop bits, each SIO_BAUD
// times the mode's factor cycles. A byte written waits in the transmit
// buffer, with SIO_STAT's ready bit off, until the next bit starts it, or
// until the byte before it has gone. The Net Yaroze's monitor counts on
// that: after writing a byte, it waits for the ready bit to go off. Bytes
// from the host arrive at the pace the port sends at. The port is brought
// up to date when the program reads or writes it and when a byte starts or
// finishes, which is when it is due.
type sio1 struct {
	mode, ctrl, baud uint16
	stat             uint16 // the sticky bits, sioOverrun and sioIRQ

	rx  [sioFIFO]byte
	rxN int

	// The byte being sent, which finishes at cycle txAt, or none if txAt
	// is 0, and the byte in the transmit buffer, if txQueued, which can
	// start at txStart, once the line is free. txFree is when the line
	// last became free.
	tx, txNext    byte
	txAt, txStart uint64
	txFree        uint64
	txQueued      bool

	// connected is whether a cable is plugged in, with its host end out
	// through a SerialLine: in holds the bytes the host has written, each
	// with the cycle it arrives, and out the bytes sent to the host and
	// not yet read.
	connected bool
	hostRTS   bool
	ctsAt     uint64 // when the host's RTS last came back on
	in        []sioByte
	inEnd     uint64 // when the last of in arrives
	out       []byte

	cycles *uint64
	due    uint64
	wake   *bool // set when due changes
	ic     *ic
}

// sioByte is a byte on its way from the host, arriving at cycle at.
type sioByte struct {
	b  byte
	at uint64
}

func newSIO1(ic *ic, cycles *uint64, wake *bool) *sio1 {
	return &sio1{ic: ic, cycles: cycles, due: ^uint64(0), wake: wake}
}

// bitCycles is how many cycles a bit takes on the line.
func (s *sio1) bitCycles() uint64 {
	factor := [4]uint64{1, 1, 16, 64}[s.mode&3]
	return max(uint64(s.baud), 1) * factor
}

// charCycles is how many cycles a byte takes on the line: a start bit, the
// data bits, the parity bit if any and the stop bits.
func (s *sio1) charCycles() uint64 {
	data := 5 + uint64(s.mode>>2&3)
	parity := uint64(s.mode >> 4 & 1)
	halfStops := [4]uint64{2, 2, 3, 4}[s.mode>>6&3] // reserved, 1, 1.5, 2
	halfBits := 2*(1+data+parity) + halfStops
	return halfBits * s.bitCycles() / 2
}

// rxIRQAt is how many bytes in the FIFO raise the receive interrupt.
func (s *sio1) rxIRQAt() int {
	return 1 << (s.ctrl >> 8 & 3)
}

// advance brings the port up to the current cycle: the byte in the
// transmit buffer starts once it can, the bytes sent that have finished go
// to the host, and the bytes from the host that have arrived go into the
// FIFO.
func (s *sio1) advance() {
	now := *s.cycles
	for {
		if s.txAt != 0 && now >= s.txAt {
			if s.connected {
				s.out = append(s.out, s.tx)
			}
			s.txFree, s.txAt = s.txAt, 0
			if s.ctrl&sioTXIRQ != 0 {
				s.interrupt()
			}
		} else if s.txAt == 0 && s.txQueued && now >= s.txStart {
			start := max(s.txStart, s.txFree)
			s.tx, s.txQueued, s.txAt = s.txNext, false, start+s.charCycles()
		} else {
			break
		}
	}
	n := 0
	for n < len(s.in) && now >= s.in[n].at {
		s.receive(s.in[n].b)
		n++
	}
	s.in = s.in[n:]
}

// receive takes a byte that has arrived from the host.
func (s *sio1) receive(b byte) {
	if s.ctrl&sioRXEnable == 0 {
		return
	}
	if s.rxN == sioFIFO {
		s.stat |= sioOverrun
		return
	}
	s.rx[s.rxN] = b
	s.rxN++
	if s.ctrl&sioRXIRQ != 0 && s.rxN >= s.rxIRQAt() {
		s.interrupt()
	}
}

func (s *sio1) interrupt() {
	s.stat |= sioIRQ
	s.ic.raise(irqSIO)
}

// schedule works out when a byte next starts or finishes, sent or
// received.
func (s *sio1) schedule() {
	*s.wake = true
	s.due = ^uint64(0)
	if s.txAt != 0 {
		s.due = s.txAt
	} else if s.txQueued {
		s.due = s.txStart
	}
	if len(s.in) > 0 {
		s.due = min(s.due, s.in[0].at)
	}
}

// update is called by the machine when the cycle count reaches due.
func (s *sio1) update() {
	s.advance()
	s.schedule()
}

func (s *sio1) send(b byte) {
	if s.ctrl&sioTXEnable == 0 {
		return
	}
	data := 5 + s.mode>>2&3
	s.txNext, s.txQueued = b&byte(1<<data-1), true
	s.txStart = *s.cycles + s.bitCycles()
}

func (s *sio1) writeCtrl(v uint16) {
	if v&sioReset != 0 {
		*s = sio1{connected: s.connected, hostRTS: s.hostRTS, ctsAt: s.ctsAt, in: s.in, inEnd: s.inEnd, out: s.out,
			cycles: s.cycles, due: s.due, wake: s.wake, ic: s.ic}
		return
	}
	if v&sioAck != 0 {
		s.stat = 0
	}
	s.ctrl = v &^ sioAck
	if s.ctrl&sioRXEnable == 0 {
		s.rxN = 0
	}
}

func (s *sio1) status() uint16 {
	st := s.stat
	if !s.txQueued {
		st |= sioTXReady
	}
	if s.txAt == 0 && !s.txQueued {
		st |= sioTXIdle
	}
	if s.rxN > 0 {
		st |= sioRXReady
	}
	if s.connected {
		st |= sioDSR
		if s.hostRTS && *s.cycles >= s.ctsAt {
			st |= sioCTS
		}
	}
	return st
}

func (s *sio1) read(off uint32) uint32 {
	s.advance()
	switch off {
	case 0:
		if s.rxN == 0 {
			return 0
		}
		b := s.rx[0]
		copy(s.rx[:], s.rx[1:s.rxN])
		s.rxN--
		return uint32(b)
	case 4:
		return uint32(s.status())
	case 8:
		return uint32(s.mode)
	case 10:
		return uint32(s.ctrl)
	case 14:
		return uint32(s.baud)
	}
	return 0
}

func (s *sio1) Read32(off uint32) uint32 { return s.read(off) }
func (s *sio1) Read16(off uint32) uint16 { return uint16(s.read(off)) }
func (s *sio1) Read8(off uint32) uint8   { return uint8(s.read(off)) }

func (s *sio1) write(off uint32, v uint16) {
	s.advance()
	defer s.schedule()
	switch off {
	case 0:
		s.send(byte(v))
	case 8:
		s.mode = v
	case 10:
		s.writeCtrl(v)
	case 14:
		s.baud = v
	}
}

func (s *sio1) Write32(off, v uint32)        { s.write(off, uint16(v)) }
func (s *sio1) Write16(off uint32, v uint16) { s.write(off, v) }
func (s *sio1) Write8(off uint32, v uint8)   { s.write(off, uint16(v)) }

// lineSteps is how many steps a SerialLine runs the machine for at a time
// while it waits for the console: about a millisecond.
const lineSteps = 20_000

// SerialLine is the host's end of a cable plugged into the console's serial
// port, which ConnectSerial returns. It has what package serial's Line
// needs, so a serial.Port can talk to the machine as it would to a
// console. The host's DTR is on, and so is its RTS until SetRTS turns it
// off, which the console sees as CTS.
//
// The machine runs only while the line waits for it: Read runs it until
// the console has sent something, or for about a millisecond if it has not,
// and CTS runs it until the bytes the host wrote have arrived. So a line
// and its machine must be used from one goroutine.
type SerialLine struct {
	m *Machine
}

// ConnectSerial plugs a cable into the serial port and returns its host
// end.
func (m *Machine) ConnectSerial() *SerialLine {
	m.sio1.connected, m.sio1.hostRTS = true, true
	return &SerialLine{m: m}
}

// SetRTS turns the host's RTS on or off. Turned back on, it stays off a
// few microseconds more (rtsPulse), so that the console sees a pulse.
func (l *SerialLine) SetRTS(on bool) error {
	s := l.m.sio1
	if on && !s.hostRTS {
		s.ctsAt = *s.cycles + rtsPulse
	}
	s.hostRTS = on
	return nil
}

// Write sends bytes to the console, which arrive one after another, each
// as long after the last as the port's settings make a byte take.
func (l *SerialLine) Write(p []byte) (int, error) {
	s := l.m.sio1
	s.advance()
	for _, b := range p {
		s.inEnd = max(s.inEnd, *s.cycles) + s.charCycles()
		s.in = append(s.in, sioByte{b: b, at: s.inEnd})
	}
	s.schedule()
	return len(p), nil
}

// Read returns what the console has sent, running the machine for a
// while first if it has sent nothing. It returns 0 and no error if it
// still has not, as a serial device with a read timeout does.
func (l *SerialLine) Read(p []byte) (int, error) {
	s := l.m.sio1
	if len(s.out) == 0 {
		if _, err := l.m.Run(lineSteps); err != nil {
			return 0, err
		}
		s.advance()
	}
	n := copy(p, s.out)
	s.out = s.out[n:]
	return n, nil
}

// CTS reports whether the console's RTS is on, once the bytes the host
// has written have arrived, as the host's own port would only look once
// it had sent them. If it is off, CTS runs the machine for a while first,
// so that a host waiting for it gives the console the time to turn it on.
func (l *SerialLine) CTS() (bool, error) {
	s := l.m.sio1
	for s.advance(); len(s.in) > 0; s.advance() {
		if _, err := l.m.Run(lineSteps); err != nil {
			return false, err
		}
	}
	if s.ctrl&sioRTS == 0 {
		if _, err := l.m.Run(lineSteps); err != nil {
			return false, err
		}
		s.advance()
	}
	return s.ctrl&sioRTS != 0, nil
}
