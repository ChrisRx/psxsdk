//go:build yaroze

package libps

// The voices: the 24 the processor has, given out to notes as they are
// keyed on, and written to the processor once a tick from a shadow of
// their registers.

// A sndVoice is what the library knows of a voice: the note it plays and
// where it came from, its age for choosing one to take, and the fades a
// program can set on its volume and pan.
//
// libps 0x8004F79C, 52 bytes each
type sndVoice struct {
	vag      int16 // the sample's number in the bank
	age      int16 // ticks since it was keyed on
	pitch    int16
	envelope int16 // the processor's envelope level, read each tick
	velocity int16
	pan      uint8
	note     int16
	seq      int16 // the sequence and track it plays for, the track in the high byte
	progIdx  int16
	prog     int16
	tone     int16
	vab      int16
	prio     int16
	status   uint8 // voiceFree, voiceOn or voiceReleased
	volFade  sndFade
	panFade  sndFade
}

const (
	voiceFree     = 0
	voiceOn       = 1
	voiceReleased = 2
)

// A sndFade moves a voice's volume or pan a step at a time: every reload
// ticks when reload is set, until it reaches its target.
type sndFade struct {
	on      int16
	step    int16
	reload  int16
	counter int16
	cur     int16
	target  int16
}

// A voice's registers as the library keeps them until the tick writes
// them: the volumes, the pitch, the sample's address and the envelope.
//
// libps 0x8004F604, 16 bytes each
type sndShadow struct {
	volL  uint16
	volR  uint16
	pitch uint16
	addr  uint16
	adsr1 uint16
	adsr2 uint16
}

const (
	dirtyVol   = 0x3
	dirtyPitch = 0x4
	dirtyAddr  = 0x8
	dirtyADSR  = 0x30
)

var (
	sndVoices  [24]sndVoice
	sndRegs    [24]sndShadow
	sndDirty   [24]uint8 // libps 0x8004F784
	sndNVoices uint8     // libps 0x80063F6C: how many of the 24 are used

	// The key on, key off and reverb bits to write at the tick, as the
	// pairs of halfwords the registers take.
	sndKeyOnLo, sndKeyOnHi   uint16 // libps 0x80062CF0, 0x80062CF4
	sndKeyOffLo, sndKeyOffHi uint16 // libps 0x800662B8, 0x800662BC
	sndRevLo, sndRevHi       uint16 // libps 0x80062CF8, 0x80062CFC

	// Which voices' envelopes read zero, for each of the last 16 ticks:
	// a voice that was released and has read zero on 15 ticks in a row
	// is free again.
	sndEnded    [16]uint32 // libps 0x8004FC80
	sndEndedIdx int32      // libps 0x8004FC7C

	sndMono        int16        // libps 0x80063E50: both channels at the louder volume
	sndSustain     uint16       // libps 0x80063DEC: added to a tone's second envelope word while the pedal is down
	sndNoRelease   uint8        // libps 0x80063FE4: released voices are not freed by their envelopes
	sndMallocTable [32]spuBlock // libps 0x8004FCC4: the processor's memory table
)

// sndCur is the note being keyed on: what the tone, the program, the
// bank and the sequence say about it, gathered before the voice is set.
//
// libps 0x80063FA4 on
var sndCur struct {
	ntones  uint8 // 0x80063FA4
	vab     uint8 // 0x80063FA5
	note    uint8 // 0x80063FA6
	vel     uint8 // 0x80063FA8
	chPan   uint8 // 0x80063FA9
	prog    uint8 // 0x80063FAA
	progIdx uint8 // 0x80063FAB
	progVol uint8 // 0x80063FAE
	progPan uint8 // 0x80063FAF
	tone    uint8 // 0x80063FB0
	toneVol uint8 // 0x80063FB1
	tonePan uint8 // 0x80063FB2
	prio    uint8 // 0x80063FB3
	center  uint8 // 0x80063FB4
	shift   uint8 // 0x80063FB5
	min     uint8 // 0x80063FB6
	max     uint8 // 0x80063FB7
	mode    uint8 // 0x80063FB8
	seq     int16 // 0x80063FBA
	vag     int16 // 0x80063FBC
	voice   int16 // 0x80063FBE
}

// The sequence id SsUtKeyOn plays under, which skips the sequence's
// volume.
const sndUtSeq = 0x21

// voiceBits is voice v's bit in a pair of halfwords.
func voiceBits(v int32) (lo, hi uint16) {
	if v < 16 {
		return 1 << uint(v), 0
	}
	return 0, 1 << uint(v-16)
}

// sndVoiceInit resets the voices, n of them, and the processor's memory
// table.
//
// libps 0x80018368, 976 bytes
func sndVoiceInit(n int32) {
	spuSetInTransfer(0)
	sndSustain = 0
	SpuInitMalloc(32, &sndMallocTable)
	sndRegs = [24]sndShadow{}
	sndDirty = [24]uint8{}
	vabCount = 0
	vabState = [16]uint8{}
	if n >= 24 {
		n = 24
	}
	sndNVoices = uint8(n)
	for v := int32(0); v < int32(sndNVoices); v++ {
		vc := &sndVoices[v]
		*vc = sndVoice{age: 24, seq: -1, vag: 0xff, tone: 0xff, pan: 0x40}
		r := spuReg(uintptr(v) * 16)
		spuReg(uintptr(v)*16 + 6).Set(0x200)
		spuReg(uintptr(v)*16 + 4).Set(0x1000)
		spuReg(uintptr(v)*16 + 8).Set(0x80ff)
		r.Set(0)
		spuReg(uintptr(v)*16 + 2).Set(0)
		spuReg(uintptr(v)*16 + 10).Set(0x4000)
		sndCur.voice = int16(v)
		vc.status = voiceFree
		vc.pitch = 0
		vc.vag = 0
		sndKeyOff(v)
	}
	sndReverb.DepthL, sndReverb.DepthR = 0x3fff, 0x3fff
	sndKeyOnLo, sndKeyOnHi = 0, 0
	sndKeyOffLo = 0
	sndRevLo, sndRevHi = 0, 0
	sndReverb.Mask, sndReverb.Mode = 0, 0
	sndNoRelease = 0
	sndMono = 0
	vabMaxProg = 0x80
	sndVoiceTick()
}

// sndKeyOff marks voice v to be keyed off at the tick, and not on.
func sndKeyOff(v int32) {
	lo, hi := voiceBits(v)
	sndKeyOffLo |= lo
	sndKeyOnLo &^= sndKeyOffLo
	sndKeyOffHi |= hi
	sndKeyOnHi &^= sndKeyOffHi
}

// sndAllocVoice picks a voice for a note: a free one whose envelope has
// ended, or else the one with the lowest priority below the note's, the
// lowest envelope among those, and the oldest among those. It returns
// the voice count when there is none. Every voice ages a tick.
//
// libps 0x80016640, 616 bytes
func sndAllocVoice() int32 {
	n := int32(sndNVoices)
	found := int32(0x63)
	best := int32(0x63)
	bestEnv := int32(0xffff)
	bestAge := int32(0)
	prio := int32(sndCur.prio)
	taken := int32(0)
	for v := int32(0); v < n; v++ {
		vc := &sndVoices[v]
		if vc.status == voiceFree && vc.envelope == 0 {
			found = v
			break
		}
		p := int32(vc.prio)
		if p < prio {
			prio = p
			best = v
			bestEnv = int32(uint16(vc.envelope))
			bestAge = int32(uint16(vc.age))
			taken = 1
		} else if p == prio {
			env := int32(uint16(vc.envelope))
			if env < bestEnv {
				taken++
				bestAge = int32(uint16(vc.age))
				bestEnv = env
				best = v
			} else if env == bestEnv && bestAge < int32(vc.age) {
				bestAge = int32(vc.age)
				best = v
			}
		}
	}
	if found == 0x63 {
		found = best
		if taken == 0 {
			found = n
		}
	}
	if found < n {
		for v := int32(0); v < n; v++ {
			sndVoices[v].age++
		}
		vc := &sndVoices[found]
		vc.age = 0
		vc.prio = int16(sndCur.prio)
		if vc.status == voiceReleased {
			SpuSetNoiseVoice(0, 0xffffff)
		}
	}
	return found
}

// sndVoiceRegs sets the voice sndCur names up for the tone: the sample's
// address from the program table and the envelope from the tone, and
// its envelope as unread.
//
// libps 0x80016D8C, 532 bytes
func sndVoiceRegs() {
	v := int32(sndCur.voice)
	sndVoices[v].envelope = 0x7fff
	for i := range sndEnded {
		sndEnded[i] &^= 1 << uint(v)
	}
	vag := int32(sndCur.vag)
	p := vabProg((vag - 1) / 2)
	if vag&1 != 0 {
		sndRegs[v].addr = peek16(p, progAddrs)
	} else {
		sndRegs[v].addr = peek16(p, progAddrs+2)
	}
	sndDirty[v] |= dirtyAddr
	t := vabTone(int32(sndCur.progIdx), int32(sndCur.tone))
	sndRegs[v].adsr1 = peek16(t, toneADSR1)
	sndRegs[v].adsr2 = peek16(t, toneADSR2) + sndSustain
	sndDirty[v] |= dirtyADSR
}

// sndPitch is the pitch register value for the note sndCur names on its
// tone: the tone's center note and fine shift against the note, looked
// up in the table by semitone and sixteenth, and shifted by the octave
// from the table's.
//
// libps 0x8001760C, 184 bytes
func sndPitch() uint16 {
	n := int32(int16(int32(sndCur.note) + 60 - int32(sndCur.center)))
	fine := int32(sndCur.shift) >> 3
	if fine >= 16 {
		fine = 15
	}
	octave := n / 12
	semi := n - octave*12
	if semi < 0 {
		// The original read before its table here; a note below the
		// tone's center by more than an octave is taken into the one
		// below.
		semi += 12
		octave--
	}
	return sndPitchShift(sndPitchTable[semi*16+fine], octave-5)
}

// sndPitchFrom is the pitch register value for note with fine sixteenths
// of a semitone added, on the tone sndCur names.
//
// libps 0x800176C4, 252 bytes
func sndPitchFrom(note int32, fine int32) uint16 {
	t := vabTone(int32(sndCur.progIdx), int32(sndCur.tone))
	f := fine + int32(peek8(t, toneShift))
	if f < 0 {
		f += 7
	}
	f >>= 3
	carry := int32(0)
	if f >= 16 {
		carry = 1
		f -= 16
	}
	n := int32(int16(note + 60 - int32(peek8(t, toneCenter)) + carry))
	octave := n / 12
	semi := n - octave*12
	if semi < 0 {
		semi += 12
		octave--
	}
	return sndPitchShift(sndPitchTable[semi*16+f], octave-5)
}

// sndPitchShift shifts a table value by octaves either way.
func sndPitchShift(v uint16, octave int32) uint16 {
	if octave > 0 {
		return uint16(uint32(v) << uint(octave))
	}
	if octave < 0 {
		return v >> uint(-octave)
	}
	return v
}

// sndKeyOnVolume sets the voice sndCur names going at pitch: its volumes
// from the bank's, the velocity, the program's and tone's volumes and
// pans, the channel's pan and the sequence's volume, squared into the
// register's range; its key on, and its reverb by the tone's mode.
//
// libps 0x800168A8, 1252 bytes
func sndKeyOnVolume(pitch uint16) {
	mvol := int32(peek8(sndVH, vabMVol))
	vel := int32(sndCur.vel) * (mvol<<14 - mvol) / 16129
	vol := uint32(vel*int32(sndCur.progVol)*int32(sndCur.toneVol)) / 16129
	v := int32(sndCur.voice)
	l, r := vol, vol
	if sndCur.seq != sndUtSeq {
		sc := sndScore(sndCur.seq)
		l = vol * uint32(sc.volL) / 127
		r = vol * uint32(sc.volR) / 127
	}
	l, r = sndPanned(l, r, sndCur.tonePan)
	l, r = sndPanned(l, r, sndCur.progPan)
	l, r = sndPanned(l, r, sndCur.chPan)
	if sndMono == 1 {
		if l < r {
			l = r
		} else {
			r = l
		}
	}
	l = l * l / 16383
	r = r * r / 16383
	sndRegs[v].pitch = pitch
	sndRegs[v].volL = uint16(l)
	sndRegs[v].volR = uint16(r)
	sndDirty[v] |= dirtyVol | dirtyPitch
	sndVoices[v].pitch = int16(pitch)
	sndVoices[v].status = voiceOn
	lo, hi := voiceBits(v)
	if sndCur.mode&4 != 0 {
		sndRevLo |= lo
		sndRevHi |= hi
	} else {
		sndRevLo &^= lo
		sndRevHi &^= hi
	}
	sndKeyOnLo |= lo
	sndKeyOffLo &^= sndKeyOnLo
	sndKeyOnHi |= hi
	sndKeyOffHi &^= sndKeyOnHi
}

// sndPanned scales one side down for a pan off center, 64 being center:
// the right for a pan to the left and the left for one to the right.
func sndPanned(l, r uint32, pan uint8) (uint32, uint32) {
	if pan < 64 {
		return l, r * uint32(pan) / 63
	}
	return l * uint32(127-pan) / 63, r
}

// sndKeyOn plays note at velocity vel with pan on program prog of bank
// vab for sequence st: a voice for each tone of the program whose range
// covers the note. It returns the voices as bits, -1 when one could not
// be had, or 0 when the program has no tone for the note.
//
// libps 0x80018FA8, 1520 bytes
func sndKeyOn(st int16, vab int16, prog int16, note uint8, vel uint8, pan uint8) int32 {
	sc := sndScore(st)
	if sndSetVab(vab, prog) != 0 {
		return -1
	}
	sndCur.seq = st
	sndCur.note = note
	if st == sndUtSeq {
		sndCur.vel = vel
	} else {
		sndCur.vel = uint8(int32(vel) * int32(sc.chVol[sc.channel]) / 127)
	}
	sndCur.chPan = pan
	p := vabProg(int32(prog))
	sndCur.progVol = peek8(p, progVol)
	sndCur.progPan = peek8(p, progPan)
	sndCur.ntones = peek8(p, progTones)
	if uint32(sndCur.progIdx) >= uint32(peek16(sndVH, vabPrograms)) {
		return -1
	}
	if vel == 0 {
		return sndKeyOffNote(st, vab, prog, note)
	}
	var vags [128]uint8
	var tones [128]uint8
	cnt := int32(0)
	for t := int32(0); t < int32(sndCur.ntones); t++ {
		tp := vabTone(int32(sndCur.progIdx), t)
		if note < peek8(tp, toneMin) || peek8(tp, toneMax) < note {
			continue
		}
		vags[cnt] = peek8(tp, toneVAG)
		tones[cnt] = uint8(t)
		cnt++
	}
	if cnt == 0 {
		return 0
	}
	mask := int32(0)
	for i := int32(0); i < cnt; i++ {
		sndCur.vag = int16(vags[i])
		sndCur.tone = tones[i]
		tp := vabTone(int32(sndCur.progIdx), int32(sndCur.tone))
		sndCur.prio = peek8(tp, tonePrio)
		sndCur.toneVol = peek8(tp, toneVol)
		sndCur.tonePan = peek8(tp, tonePan)
		sndCur.center = peek8(tp, toneCenter)
		sndCur.shift = peek8(tp, toneShift)
		sndCur.mode = peek8(tp, toneMode)
		sndCur.min = peek8(tp, toneMin)
		sndCur.max = peek8(tp, toneMax)
		v := sndAllocVoice()
		sndCur.voice = int16(v)
		if v >= int32(sndNVoices) {
			mask = -1
			continue
		}
		vc := &sndVoices[v]
		vc.status = voiceOn
		vc.age = 0
		vc.seq = st
		vc.vab = int16(sndCur.vab)
		vc.progIdx = int16(sndCur.progIdx)
		vc.prog = prog
		if st != sndUtSeq {
			vc.velocity = int16(vel)
		}
		vc.pan = pan
		vc.tone = int16(sndCur.tone)
		vc.note = int16(note)
		vc.prio = int16(sndCur.prio)
		vc.vag = sndCur.vag
		sndVoiceRegs()
		// A sample numbered 255 took the original down SsUtKeyOn's
		// volume path (libps 0x80016FA0), which is not translated; no
		// bank the converters write has one.
		sndKeyOnVolume(sndPitch())
		mask |= 1 << uint(sndCur.voice)
	}
	return mask
}

// sndKeyOffNote releases the voices playing note on program prog of
// bank vab for sequence st, and returns how many.
//
// libps 0x80019598, 496 bytes
func sndKeyOffNote(st int16, vab int16, prog int16, note uint8) int32 {
	n := int32(0)
	for v := int32(0); v < int32(sndNVoices); v++ {
		vc := &sndVoices[v]
		if vc.note != int16(note) || vc.prog != prog || vc.seq != st || vc.vab != vab {
			continue
		}
		if vc.vag == 0xff {
			vc.status = voiceFree
			vc.pitch = 0
			spuReg(spuNoiseLo).Set(0)
			spuReg(spuNoiseHi).Set(0)
		} else {
			sndCur.voice = int16(v)
			vc.status = voiceFree
			vc.pitch = 0
			vc.vag = 0
			sndKeyOff(v)
		}
		n++
	}
	return n
}

// sndAllKeysOff releases every voice playing for sequence st.
//
// libps 0x80019B30, 324 bytes
func sndAllKeysOff(st int16) {
	for v := int32(0); v < int32(sndNVoices); v++ {
		vc := &sndVoices[v]
		if vc.seq != st {
			continue
		}
		sndCur.voice = int16(v)
		vc.status = voiceFree
		vc.pitch = 0
		vc.vag = 0
		sndKeyOff(v)
	}
}

// sndSeqSetVol sets sequence st's volume, each side up to 127, and when
// apply is 1 sets its voices' registers to it directly, 129 times each.
//
// libps 0x800198B0, 372 bytes
func sndSeqSetVol(st int16, l uint16, r uint16, apply int32) int16 {
	sc := sndScore(st)
	sndCur.seq = st
	sc.volL = l
	if sc.volL >= 128 {
		sc.volL = 0x7f
	}
	sc.volR = r
	if sc.volR >= 128 {
		sc.volR = 0x7f
	}
	if apply == 1 {
		for v := int32(0); v < int32(sndNVoices); v++ {
			if sndVoices[v].seq != st {
				continue
			}
			sndRegs[v].volL = l * 129
			sndRegs[v].volR = r * 129
			sndDirty[v] |= dirtyVol
		}
	}
	return sndCur.seq
}

// sndSeqGetVol reads sequence st's volume.
//
// libps 0x80019A24, 108 bytes
func sndSeqGetVol(st int16, l *int16, r *int16) int16 {
	sc := sndScore(st)
	sndCur.seq = st
	*l = int16(sc.volL)
	*r = int16(sc.volR)
	return sndCur.seq
}

// sndChanVolume sets the volume of the voices playing program prog of
// bank vab for sequence st again, for a channel volume of vol and pan,
// and returns how many. The channel's volume in the score is set to 1
// when it was 0 and differs.
//
// libps 0x80019E04, 1336 bytes
func sndChanVolume(st int16, vab int16, prog int16, vol uint16, pan uint8) int32 {
	sc := sndScore(st)
	sndSetVab(vab, prog)
	sndCur.seq = st
	n := int32(0)
	for v := int32(0); v < int32(sndNVoices); v++ {
		vc := &sndVoices[v]
		if vc.seq != st || vc.prog != prog || vc.vab != vab {
			continue
		}
		cv := &sc.chVol[sc.channel]
		if *cv != int16(vol) && *cv == 0 {
			*cv = 1
		}
		vel := int32(vc.velocity) * int32(vol) / 127
		mvol := int32(peek8(sndVH, vabMVol))
		a := vel * (mvol<<14 - mvol) / 16129
		tp := vabTone(int32(prog), int32(vc.tone))
		b := uint32(a*int32(peek8(vabProg(int32(prog)), progVol))*int32(peek8(tp, toneVol))) / 16129
		l := b * uint32(sc.volL) / 127
		r := b * uint32(sc.volR) / 127
		l, r = sndPanned(l, r, peek8(tp, tonePan))
		l, r = sndPanned(l, r, peek8(vabProg(int32(vc.progIdx)), progPan))
		l, r = sndPanned(l, r, pan)
		if sndMono == 1 {
			if l < r {
				l = r
			} else {
				r = l
			}
		}
		sndRegs[v].volL = uint16(l * l / 16383)
		sndRegs[v].volR = uint16(r * r / 16383)
		sndDirty[v] |= dirtyVol
		n++
	}
	return n
}

// sndPitchBend bends the voices playing program prog of bank vab for
// sequence st to bend, 64 being none, and returns how many.
//
// libps 0x80018B0C, 240 bytes
func sndPitchBend(st int16, vab int16, prog int16, bend uint8) int32 {
	sndSetVab(vab, prog)
	sndCur.seq = st
	n := int32(0)
	for v := int32(0); v < int32(sndNVoices); v++ {
		n += sndBendVoice(v, st, vab, prog, bend)
	}
	return n
}

// sndBendVoice bends voice v when it plays for st on prog of vab: up by
// the tone's range above center in semitones, down by the range below.
//
// libps 0x80018908, 516 bytes
func sndBendVoice(v int32, st int16, vab int16, prog int16, bend uint8) int32 {
	vc := &sndVoices[v]
	if vc.seq != st || vc.vab != vab || vc.prog != prog {
		return 0
	}
	tp := vabTone(int32(sndCur.progIdx), int32(vc.tone))
	note := int32(uint16(vc.note))
	b := int32(int16(int32(bend) - 64))
	fine := int32(0)
	switch {
	case b > 0:
		a := b * int32(peek8(tp, tonePBMax))
		semis := a / 63
		note += semis
		fine = (a - semis*63) * 2
	case b < 0:
		a := b * int32(peek8(tp, tonePBMin))
		q := a / 64
		note += q - 1
		fine = (a-q*64)*2 + 127
	}
	sndCur.voice = int16(v)
	sndCur.tone = uint8(vc.tone)
	sndRegs[v].pitch = sndPitchFrom(note&0xffff, fine&0xffff)
	sndDirty[v] |= dirtyPitch
	return 1
}

// sndFadeVolume sets voice v's volume registers from its fade's level
// as the velocity, with what sndCur holds of the last note keyed on and
// pan as the channel's.
//
// libps 0x80017C4C (shared by 0x80017B24 and 0x80018024)
func sndFadeVolume(v int32, pan uint8) {
	mvol := int32(peek8(sndVH, vabMVol))
	a := int32(int16(sndCur.vel)) * (mvol<<14 - mvol) / 16129
	b := int32(uint32(a*int32(sndCur.progVol)*int32(sndCur.toneVol)) / 16129)
	l, r := b, b
	if sndCur.tonePan < 64 {
		r = b * int32(sndCur.tonePan) >> 6
	} else {
		l = b * int32(127-sndCur.tonePan) >> 6
	}
	if sndCur.progPan < 64 {
		r = int32(uint16(r)) * int32(sndCur.progPan) / 64
	} else {
		l = int32(uint16(l)) * int32(127-sndCur.progPan) / 64
	}
	if pan < 64 {
		r = int32(uint16(r)) * int32(pan) / 64
	} else {
		l = int32(uint16(l)) * int32(127-pan) / 64
	}
	if sndMono == 1 {
		if uint16(l) < uint16(r) {
			l = r
		} else {
			r = l
		}
	}
	sndRegs[v].volR = uint16(r)
	sndRegs[v].volL = uint16(l)
	sndDirty[v] |= dirtyVol
}

// sndFadeStep moves a fade a step, every reload ticks when it has one,
// stopping at the target. It reports whether the step was taken.
func sndFadeStep(f *sndFade) bool {
	if f.reload != 0 {
		c := f.counter
		f.counter = c - 1
		if c > 0 {
			return false
		}
		f.counter = f.reload
	}
	f.cur += f.step
	if f.step > 0 {
		if f.cur >= f.target {
			f.cur = f.target
			f.on = 0
		}
	} else if f.step < 0 {
		if f.target >= f.cur {
			f.cur = f.target
			f.on = 0
		}
	}
	return true
}

// sndVolFadeTick moves voice v's volume fade.
//
// libps 0x80017B24, 848 bytes
func sndVolFadeTick(v int32) {
	vc := &sndVoices[v]
	if !sndFadeStep(&vc.volFade) {
		return
	}
	sndCur.vel = uint8(vc.volFade.cur)
	sndFadeVolume(v, sndCur.chPan)
}

// sndPanFadeTick moves voice v's pan fade.
//
// libps 0x80018024, 836 bytes
func sndPanFadeTick(v int32) {
	vc := &sndVoices[v]
	if !sndFadeStep(&vc.panFade) {
		return
	}
	sndCur.chPan = uint8(vc.panFade.cur)
	sndFadeVolume(v, sndCur.chPan)
}

// sndVoiceTick is the voices' part of the tick: each voice's envelope is
// read, released voices whose envelopes have read zero for 15 ticks are
// freed, the fades move, the registers that changed are written, and the
// key on, key off and reverb bits are sent.
//
// libps 0x80018BFC, 940 bytes
func sndVoiceTick() {
	sndEndedIdx = (sndEndedIdx + 1) & 0xf
	ended := &sndEnded[sndEndedIdx]
	*ended = 0
	n := int32(sndNVoices)
	for v := int32(0); v < n; v++ {
		e := spuReg(uintptr(v)*16 + 12).Get()
		sndVoices[v].envelope = int16(e)
		if e == 0 {
			*ended |= 1 << uint(v)
		}
	}
	if sndNoRelease == 0 {
		all := uint32(0xffffffff)
		for i := 0; i < 15; i++ {
			all &= sndEnded[i]
		}
		for v := int32(0); v < n; v++ {
			if all&(1<<uint(v)) != 0 {
				if sndVoices[v].status == voiceReleased {
					SpuSetNoiseVoice(0, 0xffffff)
				}
				sndVoices[v].status = voiceFree
			}
		}
	}
	sndKeyOnLo &^= sndKeyOffLo
	sndKeyOnHi &^= sndKeyOffHi
	for v := int32(0); v < 24; v++ {
		if sndVoices[v].volFade.on != 0 {
			sndVolFadeTick(v)
		}
		if sndVoices[v].panFade.on != 0 {
			sndPanFadeTick(v)
		}
	}
	for v := uintptr(0); v < 24; v++ {
		d := sndDirty[v]
		r := &sndRegs[v]
		if d&1 != 0 {
			spuReg(v * 16).Set(r.volL)
			spuReg(v*16 + 2).Set(r.volR)
		}
		if d&4 != 0 {
			spuReg(v*16 + 4).Set(r.pitch)
		}
		if d&8 != 0 {
			spuReg(v*16 + 6).Set(r.addr)
		}
		if d&0x10 != 0 {
			spuReg(v*16 + 8).Set(r.adsr1)
			spuReg(v*16 + 10).Set(r.adsr2)
		}
		sndDirty[v] = 0
	}
	offLo, offHi := sndKeyOffLo, sndKeyOffHi
	onLo, onHi := sndKeyOnLo, sndKeyOnHi
	revLo, revHi := sndRevLo, sndRevHi
	sndKeyOffLo, sndKeyOffHi = 0, 0
	sndKeyOnLo, sndKeyOnHi = 0, 0
	spuReg(spuKeyOffLo).Set(offLo)
	spuReg(spuKeyOffHi).Set(offHi)
	spuReg(spuKeyOnLo).Set(onLo)
	spuReg(spuKeyOnHi).Set(onHi)
	spuReg(spuRevOnLo).Set(revLo)
	spuReg(spuRevOnHi).Set(revHi)
}
