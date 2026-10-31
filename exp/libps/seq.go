//go:build yaroze

package libps

import "unsafe"

// Sequences: a SEQ file is a MIDI track with a 15-byte header, played
// by the tick at the resolution and tempo it names, its events keying
// the voices on and off through the bank it was opened on.

// A seqScore is an open sequence: where it is in its data, its channels'
// programs, volumes and pans, its timing, and the flags the tick acts
// on.
//
// libps 0x8004EB34, 172 bytes each, in the table the startup gave the
// library
type seqScore struct {
	nextTrack  uint8          // +00: the track of the sequence to play next
	pos        unsafe.Pointer // +04: the next byte of the data
	start      unsafe.Pointer // +08: the first event
	loopStart  unsafe.Pointer // +0C: where a loop goes back to
	looping    uint8          // +10: inside a counted loop
	status     uint8          // +11: the running status
	channel    uint8          // +12: the channel of the event being handled
	rpnLSB     uint8          // +13
	rpnMSB     uint8          // +14
	nrpnLSB    uint8          // +15
	nrpnMSB    uint8          // +16
	pan        [16]uint8      // +17: each channel's pan, 64 being center
	loopSet    uint8          // +27: a loop start has been seen
	loopLeft   uint8          // +28: times round the loop still to go, 127 for ever
	rpnCount   uint8          // +29: RPN bytes seen since the last data entry
	nrpnCount  uint8          // +2A: NRPN bytes seen since the last data entry
	playing    uint8          // +2B: what SsIsEos reports
	program    [16]uint8      // +2C: each channel's program
	nextSeq    uint8          // +3C: the sequence to play when this ends, 0xff for none
	volTarget  int16          // +3E: of a crescendo or decrescendo
	volCount   int16          // +40
	volStep    int16          // +42
	accelStep  int16          // +44: of an accelerando or ritardando
	loopCount  int16          // +46: times to play, 0 for ever
	loopDone   int16          // +48: times played
	res        int16          // +4A: ticks a quarter note
	vab        int16          // +4C
	chVol      [16]int16      // +4E: each channel's volume
	tickCount  int16          // +6E: ticks left in the delta being waited out, or -1 when a tick covers several
	tickDelta  int16          // +70: deltas a tick covers, times 10
	tickDelta0 int16          // +72: the same, as the tempo set it
	volL       uint16         // +74
	volR       uint16         // +76
	volL0      int16          // +78: the volume before a pause or fade
	volR0      int16          // +7A
	firstDelta int32          // +7C: the delta before the first event, times 10
	position   int32          // +80: deltas since the start, times 10
	tempo      int32          // +84: beats a minute, from the header
	deltaLeft  int32          // +88: deltas to the next event, times 10
	curTempo   int32          // +8C: the tempo now
	flags      uint32         // +90: flagPlay and the rest
	fadeTotal  int32          // +94: ticks a fade takes
	fadeLeft   int32          // +98
	accelTicks int32          // +9C: ticks an accelerando takes
	accelLeft  int32          // +A0
	accelTempo int32          // +A4: the tempo it ends at
	lastVel    int16          // +A8
	muteMask   int16          // +AA: channels not played, a bit each
}

// The score's flags.
const (
	flagPlay    = 1 << 0 // the tick advances it
	flagStop    = 1 << 1 // the tick keys its voices off and marks it stopped
	flagClose   = 1 << 2 // the tick resets it
	flagResume  = 1 << 3 // the tick marks it playing
	flagCresc   = 1 << 4
	flagDecresc = 1 << 5
	flagAccel   = 1 << 6
	flagRitard  = 1 << 7
	flagQueued  = 1 << 8 // another sequence plays this one when it ends
	flagEnded   = 1 << 9
)

// The library plays up to 16 sequences of one track each. The startup
// gave the original a table for them; SsSetTableSize, which could give
// another, is not translated.
const (
	seqMax   = 16
	trackMax = 1
)

var (
	seqScores   [seqMax * trackMax]seqScore
	seqOpenMask uint32 // libps 0x80063F98: the sequences open, a bit each

	// What a sequence calls at a mark event (NRPN 40) when it has one,
	// which SsSetMarkCallback sets.
	//
	// libps 0x800659DC
	seqMarkCB [seqMax][16]func(seq, track int16, v uint8)
)

// seqAt is sequence seq's track track.
func seqAt(seq, track int32) *seqScore { return &seqScores[seq*trackMax+track] }

// sndScore is the score of the sequence and track st names, the track in
// its high byte.
func sndScore(st int16) *seqScore { return seqAt(int32(st)&0xff, int32(uint16(st))>>8) }

// seqST packs a sequence and a track as the voices know them.
func seqST(seq, track int32) int16 { return int16(track<<8 | seq) }

// next reads the next byte of the sequence.
func (sc *seqScore) next() uint8 {
	b := *(*uint8)(sc.pos)
	sc.pos = unsafe.Add(sc.pos, 1)
	return b
}

// readDelta reads a delta time, a variable length number, and returns it
// times 10, adding it to the position; a first byte of 0 is a delta of 0
// and adds nothing.
//
// libps 0x80013154, 184 bytes
func (sc *seqScore) readDelta() int32 {
	b := sc.next()
	if b == 0 {
		return 0
	}
	v := int32(b & 0x7f)
	for b&0x80 != 0 {
		b = sc.next()
		v = v<<7 + int32(b&0x7f)
	}
	v *= 10
	sc.position += v
	return v
}

// setTicks works out how many deltas a tick covers from the resolution
// and the tempo now: when a tick is less than a delta, how many ticks a
// delta takes instead.
//
// libps 0x800112B4 (shared by the open, SsSetTempo and the tempo event)
func (sc *seqScore) setTicks() {
	rt := int32(sc.res) * sc.curTempo
	rate := sndTickRate
	if uint32(rt*10) < uint32(rate*60) {
		t := int16(udiv(uint32(600*rate), uint32(rt)))
		sc.tickCount = t
		sc.tickDelta = t
	} else {
		q := udiv(uint32(rt*10), uint32(rate*60))
		rem := urem(uint32(rt*10), uint32(rate*60))
		sc.tickCount = -1
		sc.tickDelta = int16(q)
		if uint32(30*rate) < rem {
			sc.tickDelta = int16(q + 1)
		}
	}
	sc.tickDelta0 = sc.tickDelta
}

// udiv and urem divide as the original's divu did, where a zero divisor
// trapped; here it gives 0.
func udiv(a, b uint32) uint32 {
	if b == 0 {
		return 0
	}
	return a / b
}

func urem(a, b uint32) uint32 {
	if b == 0 {
		return 0
	}
	return a % b
}

// seqOpen sets sequence seq up on bank vab from the SEQ data: the
// resolution and tempo from the header, and the first delta. It returns
// -1 for data that is not a SEQ, and 0 for an old format it cannot play
// but opens.
//
// libps 0x800110C0, 808 bytes
func seqOpen(seq int32, vab int16, data unsafe.Pointer) int32 {
	sc := seqAt(seq, 0)
	sc.vab = vab
	sc.res = 0
	sc.rpnLSB, sc.rpnMSB, sc.rpnCount = 0, 0, 0
	sc.nrpnLSB, sc.nrpnMSB, sc.nrpnCount = 0, 0, 0
	sc.channel = 0
	sc.firstDelta, sc.position, sc.tempo = 0, 0, 0
	sc.tickDelta0 = 0
	sc.loopDone = 0
	sc.playing = 0
	sc.deltaLeft = 0
	sc.loopSet, sc.loopLeft = 0, 0
	sc.looping, sc.status = 0, 0
	sc.lastVel = 0x7f
	sc.muteMask = 0
	for i := 0; i < 16; i++ {
		sc.program[i] = uint8(i)
		sc.pan[i] = 0x40
		sc.chVol[i] = 0x7f
	}
	sc.tickCount = 1
	sc.pos = data
	switch peek8(data, 0) {
	case 'S', 'p':
		sc.pos = unsafe.Add(data, 8)
		if peek8(data, 7) != 1 {
			kprintf("This is not SEQ Data.\n")
			return -1
		}
	default:
		kprintf("This is an old SEQ Data Format.\n")
		return 0
	}
	sc.res = int16(uint16(peek8(data, 8))<<8 | uint16(peek8(data, 9)))
	t := int32(peek8(data, 10))<<16 | int32(peek8(data, 11))<<8 | int32(peek8(data, 12))
	sc.tempo = seqTempo(t)
	sc.pos = unsafe.Add(data, 15)
	sc.curTempo = sc.tempo
	d := sc.readDelta()
	sc.loopStart = sc.pos
	sc.firstDelta = d
	sc.deltaLeft = d
	sc.start = sc.pos
	sc.setTicks()
	return 0
}

// seqTempo is the beats a minute of a MIDI tempo, microseconds a beat,
// rounded.
func seqTempo(t int32) int32 {
	if t == 0 {
		return 0
	}
	q := 60000000 / t
	if uint32(t>>1) < uint32(60000000%t) {
		q++
	}
	return q
}

// SsSeqOpen opens the SEQ data at p on bank vab and returns the
// sequence's id, or -1 when none is free or the data is not a SEQ.
//
// libps 0x800113E8, 204 bytes
//
//export SsSeqOpen
func SsSeqOpen(p *uint32, vab int16) int16 {
	sndStart()
	slot := int32(-1)
	for i := int32(0); i < seqMax; i++ {
		if seqOpenMask&(1<<uint(i)) == 0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		kprintf("Can't Open Sequence data any more\n\n")
		return -1
	}
	seqOpenMask |= 1 << uint(slot)
	if seqOpen(slot, vab, unsafe.Pointer(p)) == -1 {
		return -1
	}
	return int16(slot)
}

// seqTick advances sequence seq's track track a tick: it waits the delta
// to the next event out, then handles events until the next delta is
// past this tick.
//
// libps 0x800114B4, 272 bytes
func seqTick(seq, track int32) {
	sc := seqAt(seq, track)
	dpt := int32(sc.tickDelta)
	left := sc.deltaLeft
	if left-dpt > 0 {
		switch tc := sc.tickCount; {
		case tc > 0:
			sc.tickCount = tc - 1
		case tc == 0:
			sc.tickCount = int16(dpt)
			sc.deltaLeft = left - 1
		default:
			sc.deltaLeft = left - dpt
		}
		return
	}
	if dpt < left {
		return
	}
	used := left
	for {
		seqEvent(seq, track)
		if sc.deltaLeft == 0 {
			continue
		}
		used += sc.deltaLeft
		dpt = int32(sc.tickDelta)
		if used >= dpt {
			break
		}
	}
	sc.deltaLeft = used - dpt
}

// seqEvent handles the next event of sequence seq's track track: a
// status byte names it, or the running status does.
//
// libps 0x800115C4, 684 bytes
func seqEvent(seq, track int32) {
	sc := seqAt(seq, track)
	b := sc.next()
	if b&0x80 != 0 {
		ch := b & 0xf
		sc.channel = ch
		switch b & 0xf0 {
		case 0x90:
			sc.status = 0x90
			note := sc.next()
			vel := sc.next()
			sc.deltaLeft = sc.readDelta()
			seqNoteOn(seq, track, note, vel)
		case 0xb0:
			sc.status = 0xb0
			seqControl(seq, track, sc.next())
		case 0xc0:
			sc.status = 0xc0
			seqProgram(seq, track, sc.next())
		case 0xe0:
			sc.status = 0xe0
			sc.next()
			seqPitchBend(seq, track)
		case 0xf0:
			sc.status = 0xff
			sc.channel = ch
			seqMeta(seq, track, sc.next())
		}
		return
	}
	switch sc.status {
	case 0x90:
		vel := sc.next()
		sc.deltaLeft = sc.readDelta()
		seqNoteOn(seq, track, b, vel)
	case 0xb0:
		seqControl(seq, track, b)
	case 0xc0:
		seqProgram(seq, track, b)
	case 0xe0:
		seqPitchBend(seq, track)
	case 0xff:
		seqMeta(seq, track, b)
	}
}

// seqNoteOn keys note on at velocity vel on the event's channel, or off
// for a velocity of 0; nothing while the channel is muted or the
// sequence's volume is 0.
//
// libps 0x80011870, 240 bytes
func seqNoteOn(seq, track int32, note, vel uint8) {
	sc := seqAt(seq, track)
	ch := sc.channel
	if sc.muteMask>>ch&1 != 0 || sc.volL == 0 {
		return
	}
	st := seqST(seq, track)
	if vel != 0 {
		sndKeyOn(st, sc.vab, int16(sc.program[ch]), note, vel, sc.pan[ch])
		sc.lastVel = int16(vel)
	} else {
		sndKeyOffNote(st, sc.vab, int16(sc.program[ch]), note)
	}
}

// seqProgram sets the event's channel's program.
//
// libps 0x80011960, 120 bytes
func seqProgram(seq, track int32, prog uint8) {
	sc := seqAt(seq, track)
	sc.program[sc.channel] = prog
	sc.deltaLeft = sc.readDelta()
}

// seqControl handles a control change: the bank (0), data entry (6), the
// volume (7), pan (10) and expression (11), the sustain pedal (64),
// portamento (65), the reverb depth (91), the NRPN (98, 99) and RPN
// (100, 101) numbers, and reset (121); the rest are skipped.
//
// libps 0x800119D8, 792 bytes
func seqControl(seq, track int32, cc uint8) {
	sc := seqAt(seq, track)
	v := sc.next()
	ch := sc.channel
	st := seqST(seq, track)
	switch cc {
	case 0:
		sc.vab = int16(v)
	case 6:
		seqDataEntry(seq, track, v)
		return
	case 7:
		sndChanVolume(st, sc.vab, int16(sc.program[ch]), uint16(v), sc.pan[ch])
		sc.chVol[ch] = int16(v)
	case 10:
		sndChanVolume(st, sc.vab, int16(sc.program[ch]), uint16(sc.chVol[ch]), v)
		sc.pan[ch] = v
	case 11:
		sndSetProgVol(sc.vab, int16(sc.program[ch]), v)
		sndChanVolume(st, sc.vab, int16(sc.program[ch]), uint16(sc.chVol[ch]), sc.pan[ch])
	case 64:
		if v < 64 {
			sndSustain = 0
		} else {
			sndSustain = 2
		}
	case 65:
		seqPortamento(seq, track, v)
		return
	case 91:
		SsUtSetReverbDepth(int16(v), int16(v))
	case 98:
		seqNRPNLSB(seq, track, v)
		return
	case 99:
		seqNRPNMSB(seq, track, v)
		return
	case 100:
		sc.rpnLSB = v
		sc.rpnCount++
	case 101:
		sc.rpnMSB = v
		sc.rpnCount++
	case 121:
		seqReset(seq, track)
		return
	}
	sc.deltaLeft = sc.readDelta()
}

// seqReset puts the event's channel back to its program, full volume,
// center pan, with the reverb and the pedal off.
//
// libps 0x800120C4, 200 bytes
func seqReset(seq, track int32) {
	sc := seqAt(seq, track)
	SsUtReverbOff()
	sndSustain = 0
	ch := sc.channel
	sc.program[ch] = ch
	sc.rpnLSB, sc.rpnMSB = 0, 0
	sc.chVol[ch] = 0x7f
	sc.pan[ch] = 0x40
	sc.deltaLeft = sc.readDelta()
}

// seqNRPNLSB handles NRPN 98: inside a loop whose count is not set yet
// it is the count; under NRPN 40 it calls the sequence's mark callback
// with it; under the loop numbers it does nothing.
//
// libps 0x8001218C, 304 bytes
func seqNRPNLSB(seq, track int32, v uint8) {
	sc := seqAt(seq, track)
	msb := sc.nrpnMSB
	if sc.loopSet == 1 && sc.looping == 0 {
		sc.loopLeft = v
		sc.looping = 1
	} else if msb != 0x1e && msb != 0x14 {
		sc.nrpnCount++
		sc.nrpnLSB = v
	} else {
		msb = 0
	}
	if msb == 0x28 {
		if cb := seqMarkCB[seq][track]; cb != nil {
			cb(int16(seq), int16(track), v)
		}
	}
	sc.deltaLeft = sc.readDelta()
}

// seqNRPNMSB handles NRPN 99: 20 marks a loop's start, 30 its end, where
// the sequence goes back while the count lasts, or for ever for 127.
//
// libps 0x800122BC, 324 bytes
func seqNRPNMSB(seq, track int32, v uint8) {
	sc := seqAt(seq, track)
	sc.nrpnMSB = v
	switch v {
	case 0x14:
		sc.loopSet = 1
		sc.deltaLeft = sc.readDelta()
		sc.loopStart = sc.pos
	case 0x1e:
		if sc.loopLeft == 0 {
			sc.looping = 0
			sc.deltaLeft = sc.readDelta()
			return
		}
		if sc.loopLeft < 127 {
			sc.loopLeft--
			sc.deltaLeft = sc.readDelta()
			if sc.loopLeft != 0 {
				sc.pos = sc.loopStart
			} else {
				sc.looping = 0
			}
			return
		}
		sc.readDelta()
		sc.deltaLeft = 0
		sc.pos = sc.loopStart
	default:
		sc.nrpnCount++
		sc.deltaLeft = sc.readDelta()
	}
}

// seqPortamento handles control 65 on the event's channel's program:
// its tones' modes are set to 2 for a value under 64 and to 0 for 64
// and over.
//
// libps 0x80011F60, 356 bytes
func seqPortamento(seq, track int32, v uint8) {
	sc := seqAt(seq, track)
	prog := int16(sc.program[sc.channel])
	var pa sndProgAttr
	sndGetProgAttr(sc.vab, prog, &pa)
	for t := int16(0); t < int16(pa.tones); t++ {
		var ta sndToneAttr
		sndGetToneAttr(sc.vab, prog, t, &ta)
		if v < 64 {
			ta.b[toneMode] = 2
		} else if (v+192)&0xff < 64 {
			ta.b[toneMode] = 0
		}
		sndSetToneAttr(sc.vab, prog, t, &ta)
	}
	sc.deltaLeft = sc.readDelta()
}

// seqDataEntry handles control 6: inside a loop whose count is not set
// yet it is the count; after both RPN numbers, RPN 0 sets the pitch bend
// range of the program's tones (RPN 1 and 2, the tunings, changed
// nothing in the original); after both NRPN numbers it sets a tone
// attribute, of every tone for NRPN 16 and of the tone the NRPN number
// names otherwise.
//
// libps 0x800124F0, 1440 bytes
func seqDataEntry(seq, track int32, v uint8) {
	sc := seqAt(seq, track)
	prog := int16(sc.program[sc.channel])
	var pa sndProgAttr
	sndGetProgAttr(sc.vab, prog, &pa)
	if sc.loopSet == 1 && sc.looping == 0 {
		sc.loopLeft = v
		sc.looping = 1
		sc.deltaLeft = sc.readDelta()
		return
	}
	if sc.rpnCount == 2 {
		if sc.rpnLSB == 0 && sc.rpnMSB == 0 {
			for t := int16(0); t < int16(pa.tones); t++ {
				var ta sndToneAttr
				sndGetToneAttr(sc.vab, prog, t, &ta)
				ta.b[tonePBMax] = v & 0x7f
				ta.b[tonePBMin] = v & 0x7f
				sndSetToneAttr(sc.vab, prog, t, &ta)
			}
		}
		if (sc.rpnLSB == 1 || sc.rpnLSB == 2) && sc.rpnMSB == 0 {
			for t := int16(0); t < int16(pa.tones); t++ {
				var ta sndToneAttr
				sndGetToneAttr(sc.vab, prog, t, &ta)
				sndSetToneAttr(sc.vab, prog, t, &ta)
			}
		}
		sc.deltaLeft = sc.readDelta()
		sc.rpnCount = 0
		return
	}
	if sc.nrpnCount == 2 {
		if sc.nrpnMSB == 0x10 {
			for t := int16(0); t < int16(pa.tones); t++ {
				seqSetToneParam(sc.vab, prog, t, sc.nrpnLSB, v)
			}
		} else {
			seqSetToneParam(sc.vab, prog, int16(sc.nrpnMSB), sc.nrpnLSB, v)
		}
		sc.deltaLeft = sc.readDelta()
		sc.nrpnCount = 0
		return
	}
	sc.deltaLeft = sc.readDelta()
}

// A sndEnvelope is a tone's envelope unpacked from its two words.
type sndEnvelope struct {
	ar, dr, sl, sr, rr uint16
	am, sm, rm, sd     uint16
}

// sndUnpackEnvelope unpacks the envelope words.
//
// libps 0x8001320C, 92 bytes
func sndUnpackEnvelope(adsr1, adsr2 uint16) sndEnvelope {
	return sndEnvelope{
		am: adsr1 & 0x8000,
		sm: adsr2 & 0x8000,
		sd: adsr2 & 0x4000,
		rm: adsr2 & 0x20,
		ar: adsr1 >> 8 & 0x7f,
		dr: adsr1 >> 4 & 0xf,
		sl: adsr1 & 0xf,
		sr: adsr2 >> 6 & 0x7f,
		rr: adsr2 & 0x1f,
	}
}

// sndPackEnvelope packs the envelope words again. The release mode is
// not packed, as the original did not.
//
// libps 0x80013268, 140 bytes
func sndPackEnvelope(e *sndEnvelope) (adsr1, adsr2 uint16) {
	if e.sm != 0 {
		adsr2 = 0x8000
	}
	if e.am != 0 {
		adsr1 = 0x8000
	}
	if e.sd != 0 {
		adsr2 |= 0x4000
	}
	adsr1 |= e.ar<<8&0x7f00 | e.dr<<4&0xf0 | e.sl&0xf
	adsr2 |= e.sr<<6&0x1fc0 | e.rr&0x1f
	return adsr1, adsr2
}

// seqSetToneParam sets parameter sub of tone t of program prog of bank
// vab to v: the priority, mode, note range, the envelope's parts, or the
// reverb's settings.
//
// libps 0x80012A90, 708 bytes
func seqSetToneParam(vab int16, prog int16, t int16, sub uint8, v uint8) {
	var ta sndToneAttr
	sndGetToneAttr(vab, prog, t, &ta)
	switch sub {
	case 0:
		ta.b[tonePrio] = v
		sndSetToneAttr(vab, prog, t, &ta)
	case 1:
		ta.b[toneMode] = v
		sndSetToneAttr(vab, prog, t, &ta)
		switch v {
		case 0:
			SsUtReverbOff()
		case 4:
			SsUtReverbOn()
		}
	case 2:
		ta.b[toneMin] = v
		sndSetToneAttr(vab, prog, t, &ta)
	case 3:
		ta.b[toneMax] = v
		sndSetToneAttr(vab, prog, t, &ta)
	case 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14:
		e := sndUnpackEnvelope(ta.adsr1, ta.adsr2)
		switch sub - 4 {
		case 0:
			e.am, e.ar = 0, uint16(v)
		case 1:
			e.am, e.ar = 1, uint16(v)
		case 2:
			e.dr = uint16(v)
		case 3:
			e.sl = uint16(v)
		case 4:
			e.sm, e.sr = 0, uint16(v)
		case 5:
			e.sm, e.sr = 1, uint16(v)
		case 6:
			e.rm, e.rr = 0, uint16(v)
		case 7:
			e.rm, e.rr = 1, uint16(v)
		case 8:
			if v != 0 && v < 64 {
				e.sd = 0
			} else if v-64 < 64 {
				e.sd = 1
			}
		case 9:
			ta.b[toneVibT] = v
			ta.b[tonePorW] = v
		case 10:
			ta.b[tonePorW] = v
		}
		ta.adsr1, ta.adsr2 = sndPackEnvelope(&e)
		sndSetToneAttr(vab, prog, t, &ta)
	case 15:
		SsUtSetReverbType(int16(v))
	case 16:
		SsUtSetReverbDepth(int16(v), int16(v))
	case 17:
		SsUtSetReverbFeedback(int16(v))
	case 18, 19:
		SsUtSetReverbDelay(int16(v))
	}
}

// seqPitchBend handles a pitch bend: its high byte bends the channel's
// program's voices.
//
// libps 0x80012D54, 172 bytes
func seqPitchBend(seq, track int32) {
	sc := seqAt(seq, track)
	b := sc.next()
	sndPitchBend(seqST(seq, track), sc.vab, int16(sc.program[sc.channel]), b)
	sc.deltaLeft = sc.readDelta()
}

// seqMeta handles a meta event: a tempo (0x51) sets the tempo; the end
// of the track (0x2F) plays the sequence again while its count lasts,
// or ends it, starting the sequence set to follow it.
//
// libps 0x80012E00, 852 bytes
func seqMeta(seq, track int32, meta uint8) {
	sc := seqAt(seq, track)
	switch meta {
	case 0x51:
		t := int32(sc.next())<<16 | int32(sc.next())<<8 | int32(sc.next())
		sc.curTempo = seqTempo(t)
		sc.setTicks()
		sc.deltaLeft = sc.readDelta()
	case 0x2f:
		sc.loopDone++
		if sc.loopCount == 0 {
			sc.pos = sc.start
			sc.position = 0
			sc.loopSet = 0
			sc.deltaLeft = 0
			return
		}
		if sc.loopDone < sc.loopCount {
			sc.pos = sc.start
			sc.position = 0
			sc.loopSet = 0
			sc.deltaLeft = 0
			sc.loopStart = sc.start
			return
		}
		sc.flags &^= flagPlay | flagResume | flagStop
		sc.flags |= flagEnded | flagClose
		sc.playing = 0
		sc.loopStart = sc.start
		st := seqST(seq, track)
		if sc.nextSeq != 0xff {
			seqStartNext(int32(sc.nextSeq), int32(sc.nextTrack))
			sc.playing = 0
			sndAllKeysOff(st)
		}
		sndAllKeysOff(st)
		sc.deltaLeft = int32(sc.tickDelta)
	}
}

// seqStartNext starts the sequence another set to follow it, once.
//
// libps 0x800132F4, 264 bytes
func seqStartNext(seq, track int32) {
	sc := seqAt(seq, track)
	sc.loopCount = 1
	sc.loopDone = 0
	sc.flags &^= flagQueued | flagResume | flagStop | flagClose | flagEnded
	sc.playing = 1
	sc.pos = sc.start
	sc.flags |= flagPlay
}

// SsSeqSetNext makes sequence next play when seq ends.
//
// libps 0x800133FC, 92 bytes
//
//export SsSeqSetNext
func SsSeqSetNext(seq int16, next int16) {
	sndStart()
	sc := seqAt(int32(seq), 0)
	sc.nextSeq = uint8(next)
	sc.nextTrack = 0
	nx := seqAt(int32(next), 0)
	nx.flags |= flagQueued
	nx.playing = 0
}

// seqSetAccel has sequence seq's track track reach tempo over ticks
// ticks, with the accelerando or ritardando flag, when it plays and
// the tempo differs.
//
// libps 0x80013458 and 0x80014F90, 412 and 404 bytes
func seqSetAccel(seq, track int32, tempo, ticks int32, flag uint32) {
	sc := seqAt(seq, track)
	if sc.flags&flagPlay == 0 || sc.flags&flagStop != 0 || tempo == sc.curTempo {
		return
	}
	sc.accelTicks = ticks
	sc.accelLeft = ticks
	sc.accelTempo = tempo
	d := tempo - sc.curTempo
	ad := uint32(d)
	if d < 0 {
		ad = uint32(-d)
	}
	if ad < uint32(ticks) {
		sc.accelStep = int16(udiv(uint32(ticks), ad))
	} else {
		sc.accelStep = int16(^udiv(ad, uint32(ticks)))
	}
	sc.flags |= flag
	sc.flags &^= (flagAccel | flagRitard) &^ flag
}

// SsSeqSetAccelerando has sequence seq reach tempo over ticks ticks.
//
// libps 0x800135F4, 52 bytes
//
//export SsSeqSetAccelerando
func SsSeqSetAccelerando(seq int16, tempo int32, ticks int32) {
	sndStart()
	seqSetAccel(int32(seq), 0, tempo, ticks, flagAccel)
}

// SsSeqSetRitardando has sequence seq reach tempo over ticks ticks.
//
// libps 0x80015124, 52 bytes
//
//export SsSeqSetRitardando
func SsSeqSetRitardando(seq int16, tempo int32, ticks int32) {
	sndStart()
	seqSetAccel(int32(seq), 0, tempo, ticks, flagRitard)
}

// seqAccelTick moves the tempo a step toward an accelerando's or
// ritardando's target, and recomputes the deltas a tick covers.
//
// libps 0x80015418, 476 bytes
func seqAccelTick(seq, track int32) {
	sc := seqAt(seq, track)
	sc.accelLeft--
	step := int32(sc.accelStep)
	cur, target := sc.curTempo, sc.accelTempo
	if step > 0 {
		if urem(uint32(sc.accelLeft), uint32(step)) != 0 {
			return
		}
		if uint32(target) < uint32(cur) {
			sc.curTempo = cur - 1
		} else if uint32(cur) < uint32(target) {
			sc.curTempo = cur + 1
		}
	} else if uint32(target) < uint32(cur) {
		sc.curTempo = step + cur
		if uint32(sc.curTempo) < uint32(target) {
			sc.curTempo = target
		}
	} else if uint32(cur) < uint32(target) {
		sc.curTempo = cur - step
		if uint32(target) < uint32(sc.curTempo) {
			sc.curTempo = target
		}
	}
	sc.tickDelta = int16(udiv(uint32(int32(sc.res)*sc.curTempo*10), uint32(sndTickRate*60)))
	if sc.tickDelta <= 0 {
		sc.tickDelta = 1
	}
	if sc.accelLeft != 0 && sc.curTempo != sc.accelTempo {
		return
	}
	sc.flags &^= flagAccel | flagRitard
}

// SsSetTempo sets sequence seq's track track's tempo at once.
//
// libps 0x80013654, 404 bytes
//
//export SsSetTempo
func SsSetTempo(seq int16, track int16, tempo int16) {
	sndStart()
	sc := seqAt(int32(seq), int32(track))
	sc.curTempo = int32(tempo)
	sc.setTicks()
}

// SsSetLoop sets how many times sequence seq's track track plays, 0 for
// ever.
//
// libps 0x800137A0, 72 bytes
//
//export SsSetLoop
func SsSetLoop(seq int16, track int16, count int16) {
	sndStart()
	sc := seqAt(int32(seq), int32(track))
	sc.loopCount = count
	sc.loopDone = 0
}

// SsIsEos reports whether sequence seq's track track is playing: 1
// while it is, 0 once it has ended or stopped.
//
// libps 0x800137E8, 112 bytes
//
//export SsIsEos
func SsIsEos(seq int16, track int16) int16 {
	sndStart()
	return int16(seqAt(int32(seq), int32(track)).playing)
}

// SsSetMarkCallback sets what sequence seq's track track calls at a
// mark event, NRPN 40, with the event's value.
//
// libps 0x80013830, 40 bytes
func SsSetMarkCallback(seq int16, track int16, fn func(seq, track int16, v uint8)) {
	seqMarkCB[seq][track] = fn
}

// flagClose closes sequence seq: its voices are keyed off and its scores
// reset.
//
// libps 0x80013858, 380 bytes
func seqClose(seq int32) {
	sndSeqSetVol(int16(seq), 0, 0, 1)
	sndAllKeysOff(int16(seq))
	seqOpenMask &^= 1 << uint(seq)
	for t := int32(0); t < trackMax; t++ {
		sc := seqAt(seq, t)
		sc.flags = 0
		sc.nextSeq = 0xff
		sc.nextTrack = 0
		sc.volTarget, sc.volCount = 0, 0
		sc.fadeTotal, sc.fadeLeft = 0, 0
		sc.volStep = 0
		sc.accelTempo, sc.accelLeft, sc.accelTicks = 0, 0, 0
		sc.accelStep = 0
		sc.volL, sc.volR = 0x7f, 0x7f
	}
}

// SsSeqClose closes sequence seq.
//
// libps 0x800139D4, 36 bytes
//
//export SsSeqClose
func SsSeqClose(seq int16) {
	sndStart()
	seqClose(int32(seq))
}

// seqPlayMode starts sequence seq's track track: playing count times
// for a mode of 1, or stopped where it is for 0.
//
// libps 0x80014C64, 256 bytes
func seqPlayMode(seq, track int32, mode uint8, count int16) {
	sc := seqAt(seq, track)
	sc.flags &^= flagEnded | flagClose
	sc.loopCount = count
	switch mode {
	case 1:
		sc.flags |= flagPlay
		sc.loopDone = 0
		sc.playing = 1
		sndSeqSetVol(seqST(seq, track), sc.volL, sc.volR, 0)
	case 0:
		sc.flags |= flagStop
	}
}

// SsSeqPlay plays sequence seq count times, 0 for ever: at once for a
// mode of SSPLAY_PLAY, or paused until SsSeqReplay for SSPLAY_PAUSE.
//
// libps 0x80014D64, 56 bytes
//
//export SsSeqPlay
func SsSeqPlay(seq int16, mode uint8, count int16) {
	sndStart()
	seqPlayMode(int32(seq), 0, mode, count)
}

// SsPlayBack plays sequence seq's track track from its start count
// times.
//
// libps 0x80014DD4, 196 bytes
//
//export SsPlayBack
func SsPlayBack(seq int16, track int16, count int16) {
	sndStart()
	sc := seqAt(int32(seq), int32(track))
	sc.flags &^= flagEnded | flagClose
	sc.flags |= flagPlay
	sc.loopCount = count
	sc.loopDone = 0
	sc.pos = sc.start
	sndSeqSetVol(seqST(int32(seq), int32(track)), sc.volL, sc.volR, 0)
}

// seqPause pauses sequence seq's track track, keeping its volume.
//
// libps 0x80014B38, 216 bytes
func seqPause(seq, track int32) {
	sc := seqAt(seq, track)
	sndSeqGetVol(seqST(seq, track), &sc.volL0, &sc.volR0)
	sc.flags &^= flagPlay | flagResume
	sc.flags |= flagStop
}

// SsSeqPause pauses sequence seq.
//
// libps 0x80014C10, 40 bytes
//
//export SsSeqPause
func SsSeqPause(seq int16) {
	sndStart()
	seqPause(int32(seq), 0)
}

// seqReplay resumes sequence seq's track track, unless it has ended, is
// closing, or waits to follow another.
//
// libps 0x80014E98, 164 bytes
func seqReplay(seq, track int32) {
	sc := seqAt(seq, track)
	if sc.flags&(flagEnded|flagClose) != 0 || sc.flags&flagQueued != 0 {
		return
	}
	sc.flags &^= flagStop
	sc.flags |= flagResume
	sc.flags |= flagPlay
}

// SsSeqReplay resumes sequence seq.
//
// libps 0x80014F3C, 40 bytes
//
//export SsSeqReplay
func SsSeqReplay(seq int16) {
	sndStart()
	seqReplay(int32(seq), 0)
}

// SsSeqSetVol sets sequence seq's volume, each side from 0 to 127.
//
// libps 0x800151BC, 48 bytes
//
//export SsSeqSetVol
func SsSeqSetVol(seq int16, l int16, r int16) {
	sndStart()
	sndSeqSetVol(seq, uint16(l), uint16(r), 0)
}

// SsSeqGetVol reads sequence seq's track track's volume.
//
// libps 0x80015224, 52 bytes
//
//export SsSeqGetVol
func SsSeqGetVol(seq int16, track int16, l *int16, r *int16) {
	sndStart()
	sndSeqGetVol(seqST(int32(seq), int32(track)), l, r)
}

// flagStop stops sequence seq's track track and puts it back to its
// start: its voices off, its channels reset, its position at the first
// event.
//
// libps 0x80015258, 364 bytes
func seqStop(seq, track int32) {
	sc := seqAt(seq, track)
	sc.flags &^= flagPlay | flagStop | flagResume
	sc.flags |= flagClose
	sndAllKeysOff(seqST(seq, track))
	sc.playing = 0
	sc.position = 0
	sc.loopSet = 0
	sc.rpnLSB, sc.rpnMSB, sc.rpnCount = 0, 0, 0
	sc.nrpnLSB, sc.nrpnMSB, sc.nrpnCount = 0, 0, 0
	sc.channel = 0
	sc.loopDone = 0
	sc.loopLeft = 0
	sc.looping, sc.status = 0, 0
	sc.deltaLeft = sc.firstDelta
	sc.curTempo = sc.tempo
	sc.tickDelta = sc.tickDelta0
	sc.pos = sc.start
	sc.loopStart = sc.start
	for i := 0; i < 16; i++ {
		sc.program[i] = uint8(i)
		sc.pan[i] = 0x40
		sc.chVol[i] = 0x7f
	}
	sc.volL0, sc.volR0 = 0x7f, 0x7f
}

// SsSeqStop stops sequence seq.
//
// libps 0x800153C4, 40 bytes
//
//export SsSeqStop
func SsSeqStop(seq int16) {
	sndStart()
	seqStop(int32(seq), 0)
}

// seqCrescTick moves a crescendo's volume up a step: every step ticks
// for a positive step, or by the step's size for a negative one, up to
// the target.
//
// libps 0x800140E0, 812 bytes
func seqCrescTick(seq, track int32) {
	sc := seqAt(seq, track)
	st := seqST(seq, track)
	sc.fadeLeft--
	step := int32(sc.volStep)
	var l, r int16
	if step > 0 {
		if urem(uint32(sc.fadeLeft), uint32(step)) != 0 {
			sndSeqGetVol(st, &sc.volL0, &sc.volR0)
			return
		}
		sc.volCount--
		if sc.volCount < 0 {
			sndSeqSetVol(st, 0x7f, 0x7f, 0)
			sc.flags &^= flagCresc
		} else {
			sndSeqGetVol(st, &l, &r)
			c := int32(sc.volCount)
			if int32(uint16(l))+c >= 127 && int32(uint16(r))+c >= 127 {
				sndSeqSetVol(st, 0x7f, 0x7f, 0)
			}
			if int32(uint16(l))+c < int32(sc.volTarget) {
				sndSeqSetVol(st, uint16(l)+1, uint16(r)+1, 0)
			}
		}
	} else if step < 0 {
		sc.volCount += int16(step)
		if sc.volCount < 0 {
			sndSeqSetVol(st, 0x7f, 0x7f, 0)
			sc.flags &^= flagCresc
		} else {
			sndSeqGetVol(st, &l, &r)
			if int32(uint16(l))-step >= 127 && int32(uint16(r))-step >= 127 {
				sndSeqSetVol(st, 0x7f, 0x7f, 0)
			}
			if uint32((sc.fadeTotal-sc.fadeLeft)*(-step)) < uint32(sc.volTarget) {
				sndSeqSetVol(st, uint16(int32(uint16(l))-step), uint16(int32(uint16(r))-step), 0)
			}
		}
	}
	if step != 0 && (sc.fadeLeft == 0 || sc.volCount == 0) {
		sc.flags &^= flagCresc
	}
	sndSeqGetVol(st, &sc.volL0, &sc.volR0)
}

// seqDecrescTick moves a decrescendo's volume down a step, to 1 at the
// least.
//
// libps 0x8001440C, 684 bytes
func seqDecrescTick(seq, track int32) {
	sc := seqAt(seq, track)
	st := seqST(seq, track)
	sc.fadeLeft--
	step := int32(sc.volStep)
	var l, r int16
	if step > 0 {
		if urem(uint32(sc.fadeLeft), uint32(step)) != 0 {
			sndSeqGetVol(st, &sc.volL0, &sc.volR0)
			return
		}
		sc.volCount--
		if sc.volCount <= 0 {
			sc.flags &^= flagDecresc
		} else {
			sndSeqGetVol(st, &l, &r)
			c := int32(sc.volCount)
			if int32(uint16(l))-c <= 0 || int32(uint16(r))-c <= 0 || l == 1 {
				sndSeqSetVol(st, 1, 1, 0)
			} else {
				sndSeqSetVol(st, uint16(l)-1, uint16(r)-1, 0)
			}
		}
	} else {
		sc.volCount += int16(step)
		if sc.volCount <= 0 {
			sc.flags &^= flagDecresc
		} else {
			sndSeqGetVol(st, &l, &r)
			if uint32(sc.volTarget) < uint32((sc.fadeTotal-sc.fadeLeft)*(-step)) || -step >= int32(uint16(l)) {
				sndSeqSetVol(st, 1, 1, 0)
			} else {
				sndSeqSetVol(st, uint16(int32(uint16(l))+step), uint16(int32(uint16(r))+step), 0)
			}
		}
	}
	if sc.fadeLeft == 0 || sc.volCount == 0 {
		sc.flags &^= flagDecresc
	}
	sndSeqGetVol(st, &sc.volL0, &sc.volR0)
}
