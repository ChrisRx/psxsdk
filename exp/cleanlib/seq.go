//go:build yaroze

package cleanlib

import "unsafe"

// The sequencer: SEQ files, Sony's MIDI, played on a bank by the tick
// every vertical blank.
//
// A SEQ is "pQES", a version, the resolution (ticks a quarter note, big
// endian), the tempo (microseconds a quarter note, three bytes), the time
// signature, then MIDI events each after its delta time, with running
// status, up to the end-of-track meta event.
//
// Time is kept in tenths of a tick. A sequence advances rate tenths a
// vertical blank, rate being the tempo in whole beats a minute times the
// resolution over 360 (300 on PAL), rounded; an event plays in the first
// blank whose running total has passed it, or, for the first events it
// plays, reached it. The first blank after SsSeqPlay only starts the
// sequence.
//
// The sequences run at the end of the tick, after it has written the
// registers staged since the last, so what their events stage is written at
// the next one (spec 05, "Where the sequences run"). A program's call made
// after a blank comes after that blank's events. What it asks (a play, a
// pause, a replay, a tempo) is taken in at the next tick's sequence stage,
// before the events, and SsIsEos reports what the last stage left.

const (
	seqMax    = 32
	seqMagic  = 0x53455170 // "pQES" read little-endian
	seqHeader = 15

	// NRPN numbers the SEQ marks loops with.
	nrpnLoopStart = 20
	nrpnLoopEnd   = 30
)

// Sequence states.
const (
	seqIdle = iota
	seqPlaying
	seqPaused
	seqStopped // SsSeqStop: its voices are keyed off every tick
	seqEnded   // played to the end
)

type sequence struct {
	open  bool
	state uint8
	id    int8
	vab   int16

	start, pos unsafe.Pointer
	status     uint8

	// res is the resolution, ticks a quarter note, and bpm the tempo in
	// beats a minute, which rate (tenths of a tick a blank) is worked out
	// from.
	res, bpm       int32
	rate, acc, due int32
	stopTicks      uint8
	// stopReq is SsSeqStop since the last tick.
	stopReq bool

	// eos is what SsIsEos reports: set by SsSeqPlay, by a replay and by a
	// sequence that this one follows, cleared by a pause, a stop and the
	// end.
	eos bool
	// pauseReq and replayReq are SsSeqPause and SsSeqReplay since the
	// last tick, which the next sequence stage takes in. pauseReq outlives
	// SsSeqOpen: a pause on a sequence not open waits for it.
	pauseReq, replayReq bool
	// tempoReq, if not 0, is SsSetTempo's tempo, taken in likewise.
	tempoReq int32
	// pauseOff is a pause made before SsSeqPlay: the first stage that plays
	// keys the sequence's voices off after its events, and the sequence
	// plays on (with SsIsEos 0).
	pauseOff bool
	// offNext keys the sequence's voices off in this stage: a pause taken
	// in.
	offNext bool
	// next is the sequence SsSeqSetNext set to play after this one, or -1.
	next int8

	// An accelerando or ritardando: the tempo moves from tempoFrom to
	// tempoTo over tempoTime ticks (see tempoTick); tempoTicks counts the
	// ticks since it was taken in.
	tempoChange        bool
	tempoFrom, tempoTo int32
	tempoTime          int32
	tempoTicks         int32
	tempoNew           tempoRequest

	// loops is how many more times the sequence plays, 0 for ever.
	loops int16

	volL, volR int16
	program    [16]uint8
	// bend is each channel's pitch bend, 0 to 127 with 64 the center:
	// the high byte of the event's 14-bit value. It retunes the voices
	// playing on the channel; a note started later starts unbent.
	bend [16]uint8
	// chVol and chPan are each channel's volume (controller 7) and pan
	// (controller 10). The expression (controller 11) is not kept here: it
	// goes into the bank's header (see control).
	chVol, chPan [16]uint8

	// The SEQ's own loop: where it starts, how many more times, and the
	// NRPN being set.
	loopPos   unsafe.Pointer
	loopCount uint8
	nrpn      uint8
}

// tempoRequest is an accelerando or ritardando asked for since the last
// tick.
type tempoRequest struct {
	set         bool
	tempo, time int32
}

var seqs [seqMax]sequence

// readDelta reads a delta time at s.pos.
func (s *sequence) readDelta() int32 {
	var d int32
	for {
		c := *(*uint8)(s.pos)
		s.pos = unsafe.Add(s.pos, 1)
		d = d<<7 | int32(c&0x7f)
		if c&0x80 == 0 {
			return d
		}
	}
}

func (s *sequence) byte() uint8 {
	c := *(*uint8)(s.pos)
	s.pos = unsafe.Add(s.pos, 1)
	return c
}

// atEnd reports whether the event at s.pos is the end of the track.
func (s *sequence) atEnd() bool {
	p := (*[2]uint8)(s.pos)
	return p[0] == 0xff && p[1] == 0x2f
}

// rewind goes back to the first event.
func (s *sequence) rewind() {
	s.pos = s.start
	s.status = 0
	s.due += s.readDelta() * 10
}

// SsSeqOpen opens the SEQ at addr to play on bank vabid, and returns its
// id, or -1.
//
//export SsSeqOpen
func SsSeqOpen(addr *uint32, vabid int16) int16 {
	soundStart()
	p := unsafe.Pointer(addr)
	if *(*uint32)(p) != seqMagic {
		return -1
	}
	id := -1
	for i := range seqs {
		if !seqs[i].open {
			id = i
			break
		}
	}
	if id < 0 {
		return -1
	}
	b := (*[seqHeader]uint8)(p)
	res := int32(b[8])<<8 | int32(b[9])
	tempo := int32(b[10])<<16 | int32(b[11])<<8 | int32(b[12])
	// A tempo of 0 stops Sony's library at a break (a division by it);
	// here, as in the reference, the sequence holds still until a tempo is
	// set (spec 10, "SsSetTempo").
	var bpm int32
	if tempo > 0 {
		bpm = (60000000 + tempo/2) / tempo
	}
	s := &seqs[id]
	pauseReq := s.pauseReq
	*s = sequence{
		pauseReq: pauseReq,
		open:     true,
		id:       int8(id),
		vab:      vabid,
		start:    unsafe.Add(p, seqHeader),
		res:      res,
		bpm:      bpm,
		rate:     seqRate(res, bpm),
		volL:     127,
		volR:     127,
		next:     -1,
	}
	for i := range s.bend {
		s.bend[i] = 64
		s.chVol[i] = 127
		s.chPan[i] = 64
	}
	s.rewind()
	snd.seqsOpen |= 1 << id
	return int16(id)
}

// seqRate is how far a sequence advances a vertical blank, in tenths of
// a tick, rounded: there are 60 blanks a second on NTSC and 50 on PAL.
// Below a tenth of a tick a blank the library takes the inverse, blanks a
// tick, for the rate: a tempo that slow plays fast.
func seqRate(res, bpm int32) int32 {
	perMinute := int32(60 * 60)
	if video.mode == modePAL {
		perMinute = 50 * 60
	}
	n := res * bpm * 10
	if n <= 0 {
		return 0
	}
	if n < perMinute {
		return perMinute * 100 / n
	}
	return (n + perMinute/2) / perMinute
}

// SsSeqClose closes sequence id, silencing its voices: their volumes go
// to 0 and they are keyed off at the next tick. The voices stay the
// sequence's, so that closing it again silences them again.
//
//export SsSeqClose
func SsSeqClose(id int16) {
	soundStart()
	if id < 0 || id >= seqMax {
		return
	}
	state := EnterCriticalSection()
	for v := range snd.voices {
		sv := &snd.voices[v]
		if sv.seq == int8(id) {
			sv.setReg(regVolL, 0)
			sv.setReg(regVolR, 0)
			keyOff(v)
		}
	}
	seqs[id].open = false
	seqs[id].eos = false
	snd.seqsOpen &^= 1 << id
	if state != 0 {
		ExitCriticalSection()
	}
}

func seqByID(id int16) *sequence {
	if id < 0 || id >= seqMax || !seqs[id].open {
		return nil
	}
	return &seqs[id]
}

// play starts s from where it is, or from the beginning after a stop or
// its end; its first events play in the next sequence stage. A pause asked
// for before it keys the first notes off after them instead.
func (s *sequence) play(loops int16) {
	if s.state == seqStopped || s.state == seqEnded || s.stopReq {
		s.stopReq = false
		// Played again after a stop, it starts from the beginning.
		s.restart()
	}
	if s.pauseReq {
		s.pauseReq = false
		s.pauseOff = true
	}
	s.loops = loops
	s.state = seqPlaying
	s.offNext = false
}

// restart goes back to the beginning with nothing played.
func (s *sequence) restart() {
	s.pos, s.status, s.acc, s.due = s.start, 0, 0, 0
	s.loopPos = nil
	s.due = s.readDelta() * 10
}

// SsSeqPlay plays sequence id, at once (mode 1) or paused, loops times or
// for ever (0).
//
//export SsSeqPlay
func SsSeqPlay(id int16, mode uint8, loops int16) {
	soundStart()
	s := seqByID(id)
	if s == nil {
		return
	}
	s.play(loops)
	if mode == 1 {
		s.eos = true
	} else {
		// Paused at once: as a play and a pause.
		s.eos = false
		s.pauseReq = true
	}
}

// SsPlayBack plays sequence id as SsSeqPlay does, l times or for ever,
// but without telling SsIsEos: what it reports stays as it was. The
// sequences already playing go on.
//
//export SsPlayBack
func SsPlayBack(id, n, loops int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.play(loops)
	}
}

// SsSeqSetNext makes sequence next play, once, when sequence id ends.
//
//export SsSeqSetNext
func SsSeqSetNext(id, next int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.next = int8(next)
	}
}

// SsIsEos reports 1 while sequence id plays (as SsSeqPlay or a sequence
// before it started it), 0 otherwise.
//
//export SsIsEos
func SsIsEos(id, n int16) int16 {
	soundStart()
	if s := seqByID(id); s != nil && s.eos {
		return 1
	}
	return 0
}

// SsSeqPause pauses sequence id, from the tick after next. Even a
// sequence not open yet remembers it.
//
//export SsSeqPause
func SsSeqPause(id int16) {
	soundStart()
	if id < 0 || id >= seqMax {
		return
	}
	seqs[id].pauseReq = true
}

// SsSeqReplay carries on with sequence id where it was paused, from the
// tick after next.
//
//export SsSeqReplay
func SsSeqReplay(id int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.replayReq = true
	}
}

// SsSeqStop stops sequence id and keys its voices off, at the next tick
// (see stopSequences) and at the one after.
//
//export SsSeqStop
func SsSeqStop(id int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.stopReq = true
		snd.stopAsked = true
		s.eos = false
	}
}

// SsSeqSetVol sets sequence id's volume, left and right, 0 to 127: a value
// outside that acts as 127 (see seqVolume).
//
//export SsSeqSetVol
func SsSeqSetVol(id, l, r int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.volL, s.volR = l, r
	}
}

// SsSeqGetVol reads sequence id's volume.
//
//export SsSeqGetVol
func SsSeqGetVol(id, mode int16, l, r *int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		*l, *r = s.volL, s.volR
	}
}

// SsSetTempo sets sequence id's tempo, in beats a minute, from the next
// sequence stage. A tempo of 0 stops Sony's library at a break; here, as in
// the reference, the sequence holds still (its ticks go on, nothing
// advances) until another tempo is set, which plays it on (spec 10,
// "SsSetTempo").
//
//export SsSetTempo
func SsSetTempo(id, n, tempo int16) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.tempoReq = int32(tempo)
		if s.tempoReq == 0 {
			s.tempoReq = -1
		}
	}
}

// SsSeqSetAccelerando moves sequence id's tempo towards tempo over time
// ticks; see tempoTick.
//
//export SsSeqSetAccelerando
func SsSeqSetAccelerando(id int16, tempo, time int32) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.tempoNew = tempoRequest{true, tempo, time}
	}
}

// SsSeqSetRitardando moves sequence id's tempo towards tempo over time
// ticks, as SsSeqSetAccelerando does; see tempoTick.
//
//export SsSeqSetRitardando
func SsSeqSetRitardando(id int16, tempo, time int32) {
	soundStart()
	if s := seqByID(id); s != nil {
		s.tempoNew = tempoRequest{true, tempo, time}
	}
}

// stopSequences takes in SsSeqStop for each sequence asked to stop since
// the last tick: its voices are keyed off before the tick writes its masks,
// which drops a key on the last stage staged, and the sequence's stage in
// this tick keys them off again.
func stopSequences() {
	snd.stopAsked = false
	for i := range seqs {
		s := &seqs[i]
		if s.open && s.stopReq {
			s.stopReq = false
			s.keyOffAll()
			s.state = seqStopped
			s.stopTicks = 1
			s.pauseReq, s.replayReq = false, false
		}
	}
}

// tick is the sequence's stage in a vertical blank: it takes in what was
// asked since the last tick and plays the blank's events.
func (s *sequence) tick() {
	s.takeRequests()
	switch s.state {
	case seqStopped:
		// A stop keys the sequence's voices off again in the stage after
		// the tick that took it in.
		if s.stopTicks > 0 {
			s.stopTicks--
			s.keyOffAll()
		}
	case seqPaused:
		if s.offNext {
			s.offNext = false
			s.keyOffAll()
		}
	case seqPlaying:
		if s.pauseOff {
			s.eos = false
		}
		s.advance()
		if s.pauseOff && s.state == seqPlaying {
			s.pauseOff = false
			s.keyOffAll()
		}
	}
}

// takeRequests takes in a pause, a replay and tempo changes asked for
// since the last tick.
func (s *sequence) takeRequests() {
	if s.pauseReq {
		s.pauseReq = false
		switch s.state {
		case seqPlaying:
			s.state = seqPaused
			s.eos = false
			s.offNext = true
		case seqPaused:
			s.offNext = true
		}
	}
	if s.replayReq {
		s.replayReq = false
		if s.state == seqPaused {
			s.state = seqPlaying
			s.eos = true
			s.offNext = false
		}
	}
	if s.tempoReq != 0 {
		t := s.tempoReq
		if t < 0 {
			t = 0
		}
		s.tempoReq = 0
		s.tempoChange = false
		s.bpm = t
		s.rate = seqRate(s.res, t)
	}
	if r := s.tempoNew; r.set {
		s.tempoNew = tempoRequest{}
		s.tempoChange = true
		s.tempoFrom, s.tempoTo = s.bpm, r.tempo
		s.tempoTime = r.time
		s.tempoTicks = 0
	}
}

// tempoTick moves an accelerando or ritardando on by a tick. The stage
// that takes it in plays at the old tempo; from the next,
// with d the distance in beats a minute from the tempo then to the one
// asked for, and t the time asked for: if d is at least t, the tempo
// moves d/t+1 beats a minute (whole, rounded down) towards it every tick,
// otherwise one beat every t/d ticks. It stops on reaching it. An
// accelerando that asks for a slower tempo slows down the same way, and a
// ritardando to a faster one speeds up. The rate is the tempo's rounded
// down.
func (s *sequence) tempoTick() {
	if !s.tempoChange {
		return
	}
	s.tempoTicks++
	if s.tempoTicks == 1 {
		return
	}
	d := s.tempoTo - s.tempoFrom
	dir := int32(1)
	if d < 0 {
		d, dir = -d, -1
	}
	b := s.bpm
	switch {
	case d == 0:
		s.tempoChange = false
		return
	case s.tempoTime <= 0 || d >= s.tempoTime:
		step := d + 1
		if s.tempoTime > 0 {
			step = d/s.tempoTime + 1
		}
		b += dir * step
	default:
		if (s.tempoTicks-1)%(s.tempoTime/d) != 0 {
			return
		}
		b += dir
	}
	if dir > 0 && b >= s.tempoTo || dir < 0 && b <= s.tempoTo {
		b = s.tempoTo
		s.tempoChange = false
	}
	s.bpm = b
	s.rate = seqRateDown(s.res, b)
}

// seqRateDown is seqRate rounded down.
func seqRateDown(res, bpm int32) int32 {
	perMinute := int32(60 * 60)
	if video.mode == modePAL {
		perMinute = 50 * 60
	}
	n := res * bpm * 10
	if n <= 0 {
		return 0
	}
	if n < perMinute {
		return perMinute * 100 / n
	}
	return n / perMinute
}

// advance plays the events of a vertical blank.
func (s *sequence) advance() {
	s.tempoTick()
	s.acc += s.rate
	// The first events a blank plays may fall exactly on its total; those
	// after them must fall before it. Events at the same tick as one
	// played play with it, even when a tempo change has moved the total.
	if !(s.due <= s.acc) {
		return
	}
	for s.state == seqPlaying {
		s.event()
		if s.state != seqPlaying {
			break
		}
		d := s.readDelta()
		if s.atEnd() {
			// The end of the track comes at once, whatever its delta.
			s.end()
			if s.state != seqPlaying {
				break
			}
			d = s.readDelta()
		}
		if d > 0 {
			s.due += d * 10
			if !(s.due < s.acc) {
				break
			}
		}
	}
}

// keyOffAll keys off every voice the sequence has played on.
func (s *sequence) keyOffAll() {
	for v := range snd.voices {
		if snd.voices[v].seq == s.id {
			keyOff(v)
		}
	}
}

// event plays the event at s.pos.
func (s *sequence) event() {
	c := *(*uint8)(s.pos)
	if c&0x80 != 0 {
		s.status = c
		s.pos = unsafe.Add(s.pos, 1)
	}
	ch := s.status & 0x0f
	switch s.status >> 4 {
	case 0x8:
		note := s.byte()
		s.byte()
		s.noteOff(ch, note)
	case 0x9:
		note, vel := s.byte(), s.byte()
		if vel == 0 {
			s.noteOff(ch, note)
		} else {
			s.noteOn(ch, note, vel)
		}
	case 0xa:
		s.byte()
		s.byte()
	case 0xb:
		s.control(ch, s.byte(), s.byte())
	case 0xc:
		s.program[ch] = s.byte()
	case 0xd:
		s.byte()
	case 0xe:
		s.byte() // the low 7 bits, which the library does not use
		s.pitchBend(ch, s.byte())
	case 0xf:
		switch s.byte() {
		case 0x51:
			t := int32(s.byte())<<16 | int32(s.byte())<<8 | int32(s.byte())
			s.tempo(t)
		case 0x2f:
			s.end()
		}
	}
}

// tempo is a tempo change, in microseconds a quarter note: the new rate
// counts from the start of the blank it comes in, so the blank's total
// moves by the difference at once. An event of 0 microseconds stops
// Sony's library at a break; here, as in the reference, the sequence
// holds still from it (spec 10, "SsSetTempo").
func (s *sequence) tempo(us int32) {
	s.bpm = 0
	if us > 0 {
		s.bpm = (60000000 + us/2) / us
	}
	r := seqRate(s.res, s.bpm)
	s.acc += r - s.rate
	s.rate = r
}

// end is the end of the track: the sequence plays again or ends. At its
// end a sequence set by SsSeqSetNext starts, at once.
func (s *sequence) end() {
	if s.loops != 1 {
		if s.loops > 1 {
			s.loops--
		}
		s.pos = s.start
		s.status = 0
		return
	}
	s.state = seqEnded
	s.eos = false
	s.keyOffAll()
	if n := seqByID(int16(s.next)); n != nil {
		n.restart()
		n.loops = 1
		n.state = seqPlaying
		n.offNext = false
		n.eos = true
	}
}

// control handles a control change. Expression, controller 11, replaces the
// volume byte of the channel's program in the bank's header, which every key
// on reads: it stays for any sequence on the program and for SsUtKeyOn,
// until something writes the byte again (spec 05, "Expression and the VAG
// index").
func (s *sequence) control(ch, c, v uint8) {
	switch c {
	case 99:
		s.nrpn = v
		if v == nrpnLoopEnd && s.loopPos != nil {
			if s.loopCount != 127 {
				if s.loopCount == 0 {
					return
				}
				s.loopCount--
			}
			s.pos = s.loopPos
		}
	case 6:
		if s.nrpn == nrpnLoopStart {
			s.loopCount = v
			s.loopPos = s.pos
		}
	case 7:
		s.chVol[ch] = v
		s.revolume(ch)
	case 10:
		s.chPan[ch] = v
		s.revolume(ch)
	case 11:
		if b := s.bank(); b != nil {
			b.progs[s.program[ch]].vol = v
		}
		s.revolume(ch)
	}
}

// bank returns the sequence's bank, or nil if it is not open.
func (s *sequence) bank() *vab {
	if s.vab < 0 || s.vab >= vabMax || !vabs[s.vab].open {
		return nil
	}
	return &vabs[s.vab]
}

// revolume works out again the volumes of the sequence's voices that
// play channel ch's program, whichever channel started them, with ch's
// volume and pan and each voice's own velocity.
func (s *sequence) revolume(ch uint8) {
	b := s.bank()
	if b == nil {
		return
	}
	prog := int16(s.program[ch])
	p := &b.progs[prog]
	for v := range snd.voices {
		sv := &snd.voices[v]
		if sv.seq != s.id || sv.prog != prog {
			continue
		}
		t := b.tone(int(prog), int(sv.tone))
		l, r := s.noteVolume(b, p, t, ch, sv.vel)
		sv.setReg(regVolL, l)
		sv.setReg(regVolR, r)
	}
}

// noteVolume is the volume of a note at velocity vel on channel ch: the
// velocity scaled by the channel's volume, and the channel's pan as the
// note's.
func (s *sequence) noteVolume(b *vab, p *vabProg, t *vabTone, ch, vel uint8) (uint16, uint16) {
	vol := int32(vel) * int32(s.chVol[ch]) / 127
	return voiceVolume(b, p, t, vol, int32(s.chPan[ch]), seqVolume(s.volL), seqVolume(s.volR))
}

// noiseVolume is the volumes of a noise tone's note on: no velocity, no
// channel volume, no bank volume and no squaring (spec 05, "Noise voices").
func (s *sequence) noiseVolume(p *vabProg, t *vabTone, ch uint8) (uint16, uint16) {
	side := func(pf func(int32) int32, seq int32) uint16 {
		x := int32(0x3fff) * (int32(t.vol) * int32(p.vol)) / (127 * 127)
		x = x * seq / 127
		x = x * pf(int32(t.pan)) / 63
		x = x * pf(int32(p.pan)) / 63
		x = x * pf(int32(s.chPan[ch])) / 63
		return uint16(x)
	}
	return side(panLeft, seqVolume(s.volL)), side(panRight, seqVolume(s.volR))
}

// seqVolume is a sequence's volume as the volumes take it: outside 0 to 127
// it acts as 127.
func seqVolume(v int16) int32 {
	if v < 0 || v > 127 {
		return 127
	}
	return int32(v)
}

// noteOn plays note on channel ch: a voice for each of the channel's
// program's tones whose range has it (min <= note <= max), in row order,
// read from the bank's header as it is now, the program's tone count and
// the header's program count included: a program whose slot (the entry's
// byte 8, its index among the programs with tones, which picks its rows)
// is not below the count plays nothing. The probe (spec 05, "The
// sequencer and the header") refused program 1, slot 1, with counts 0 and
// 1 and played it with 2 and 3; Sony's STD0 bank, whose count is 113 for
// programs 0 to 111 and 127, plays program 127 (slot 112) in breakout,
// dogtale and invaders, so the count is not compared with the program's
// number. Unlike SsUtKeyOn it does not refuse a tone whose VAG index is
// 0: that plays with the second VAG's address (seqAddr). A left volume of
// 0 keys nothing (spec 10).
func (s *sequence) noteOn(ch, note, vel uint8) {
	b := s.bank()
	if b == nil || s.volL == 0 {
		return
	}
	prog := int(s.program[ch])
	p := &b.progs[prog]
	if int(uint8(p.slot)) >= b.programs() {
		return
	}
	for t := 0; t < int(p.tones) && t < vabTones; t++ {
		tn := b.tone(prog, t)
		if note < tn.min || note > tn.max {
			continue
		}
		noise := uint8(tn.vag) == vagNoise
		var vl, vr uint16
		if noise {
			vl, vr = s.noiseVolume(p, tn, ch)
		} else {
			vl, vr = s.noteVolume(b, p, tn, ch, vel)
		}
		v := allocVoice(tn.prior, false)
		if v < 0 {
			continue
		}
		sv := &snd.voices[v]
		sv.seq = s.id
		sv.channel = ch
		sv.vel = vel
		sv.vab, sv.prog, sv.tone = s.vab, int16(prog), int16(t)
		startVoice(v, b, tn, int(note), 0, vl, vr, b.seqAddr(tn.vag), noise)
	}
}

// bendFine is a pitch bend in 128ths of a semitone, for a tone that
// bends up to pbmax semitones (pbmin down), b being 0 to 127 with 64 the
// center. Up, b - 64 of 63 steps reach pbmax: whole semitones of 63 steps
// each, and two 128ths for each step left over. Down, 64 steps reach
// pbmin, two 128ths a step, less one. It adds to the tone's shift before
// that becomes sixteenths of a semitone.
func bendFine(t *vabTone, b int) int {
	d := b - 64
	switch {
	case d > 0:
		v := d * int(t.pbMax)
		return v/63*128 + v%63*2
	case d < 0:
		return d*int(t.pbMin)*2 - 1
	}
	return 0
}

// pitchBend sets channel ch's pitch bend and retunes the voices playing
// on it.
func (s *sequence) pitchBend(ch, bend uint8) {
	s.bend[ch] = bend
	b := s.bank()
	if b == nil {
		return
	}
	for v := range snd.voices {
		sv := &snd.voices[v]
		if sv.seq != s.id || sv.channel != ch {
			continue
		}
		t := b.tone(int(sv.prog), int(sv.tone))
		sv.setReg(regPitch, notePitch(int(sv.note), bendFine(t, int(bend)), t.center, t.shift))
	}
}

// noteOff releases the voices playing note in the program of channel ch,
// whichever channel started them.
func (s *sequence) noteOff(ch, note uint8) {
	prog := int16(s.program[ch])
	for v := range snd.voices {
		sv := &snd.voices[v]
		// A voice keeps its note, and is found, however long ago it
		// was released, until it is taken again.
		if sv.seq == s.id && sv.prog == prog && sv.note == int16(note) {
			noteKeyOff(v)
		}
	}
}
