//go:build yaroze

package libps

// Voices played directly, without a sequence.

// voiceVolume works out a voice's two register volumes: vol is the
// note's volume (a velocity, or SsUtKeyOn's louder side), pan its own
// pan, and seqL and seqR the sequence's volume, all 0 to 127. The bank,
// program and tone add their volumes and pans; the sum is squared.
func voiceVolume(b *vab, p *vabProg, t *vabTone, vol, pan, seqL, seqR int32) (uint16, uint16) {
	return voiceSide(b, p, t, vol, pan, seqL, panLeft), voiceSide(b, p, t, vol, pan, seqR, panRight)
}

// voiceSide is one side of voiceVolume: seq is the sequence's volume on that
// side, and pf what the side takes of a pan. It is a function with every
// value passed to it, not a closure inside voiceVolume: a closure's captured
// variables go on the heap where the compiler does not inline it (spec 59).
func voiceSide(b *vab, p *vabProg, t *vabTone, vol, pan, seq int32, pf func(int32) int32) uint16 {
	x := int32(0x3fff)
	x = x * (vol * int32(b.mvol)) / (127 * 127)
	x = x * (int32(t.vol) * int32(p.vol)) / (127 * 127)
	x = x * pf(int32(t.pan)) / 63
	x = x * pf(int32(p.pan)) / 63
	x = x * pf(pan) / 63
	x = x * seq / 127
	return volumeReg(x)
}

// startVoice sets voice v up to play tone t of bank b at note and fine,
// with the volumes given and the sample at addr, and keys it on at the next
// tick. A noise voice writes its clock and noise bit at once and leaves the
// pitch as it is.
func startVoice(v int, b *vab, t *vabTone, note, fine int, l, r, addr uint16, noise bool) {
	sv := &snd.voices[v]
	sv.note = int16(note)
	sv.prior = t.prior
	sv.noise = noise
	if noise {
		noiseOn(v, note, t.center)
	} else {
		sv.setReg(regPitch, notePitch(note, fine, t.center, t.shift))
	}
	sv.setReg(regVolL, l)
	sv.setReg(regVolR, r)
	sv.setReg(regAddr, addr)
	sv.setReg(regADSR1, t.adsr1)
	sv.setReg(regADSR2, t.adsr2)
	if t.mode&4 != 0 {
		snd.reverb |= 1 << v
	} else {
		snd.reverb &^= 1 << v
	}
	keyOn(v)
}

// SsUtKeyOn plays tone of program prog in bank vabid at note and fine,
// with volumes l and r, and returns the voice, or -1. The program and
// tone are read from the bank's header as it is now; a tone whose VAG
// index is 0 is refused. A VAG index of exactly 00FFh is a noise voice, which
// starts silent: its volumes are 0 until SsUtSetVVol.
//
//export SsUtKeyOn
func SsUtKeyOn(vabid, prog, tone, note, fine, l, r int16) int32 {
	soundStart()
	if vabid < 0 || vabid >= vabMax || !vabs[vabid].open || prog < 0 || prog >= vabPrograms || tone < 0 || tone >= vabTones {
		return -1
	}
	b := &vabs[vabid]
	p := &b.progs[prog]
	t := b.tone(int(prog), int(tone))
	if t.vag == 0 {
		return -1
	}
	// The louder side is the volume; the quieter sets the pan.
	vol, pan := int32(l), int32(64)
	switch {
	case l > r:
		pan = int32(r) * 64 / int32(l)
	case l < r:
		vol = int32(r)
		pan = 127 - int32(l)*64/int32(r)
	}
	vl, vr := voiceVolume(b, p, t, vol, pan, 127, 127)
	noise := t.vag == vagNoise
	if noise {
		vl, vr = 0, 0
	}
	state := EnterCriticalSection()
	v := allocVoice(t.prior, true)
	if v < 0 {
		if state != 0 {
			ExitCriticalSection()
		}
		return -1
	}
	sv := &snd.voices[v]
	sv.seq = -1
	sv.vab, sv.prog, sv.tone = vabid, prog, tone
	startVoice(v, b, t, int(note), int(fine), vl, vr, b.utAddr(int(t.vag)), noise)
	// Until the next tick reads it, the voice counts as silent, the
	// first a steal takes; a sequence's note on leaves the last reading.
	sv.env = 0
	if state != 0 {
		ExitCriticalSection()
	}
	pace(ssUtKeyOnPad)
	return int32(v)
}

// The rest of SsUtKeyOn's 2,529 cycles and SsUtAllKeyOff's 5,124 (spec 21).
const (
	ssUtKeyOnPad     = 734
	ssUtAllKeyOffPad = 2402
)

// SsUtKeyOff releases voice at the next tick, and returns 0, when the
// bank, program, tone and note given are those the voice was keyed on
// with, by SsUtKeyOn or by a sequence's note on (its tone is the row's
// index in the program); otherwise it returns -1 and keys off nothing,
// whichever voice plays them. The record is the voice's own and outlives
// the key off and the voice's silence until the voice is taken again or
// SsUtAllKeyOff drops it, so a second call gives 0 too, in the same frame
// or a later one. The header is not read. A noise voice is keyed off as a
// note off does (noteKeyOff).
//
//export SsUtKeyOff
func SsUtKeyOff(voice, vabid, prog, tone, note int16) int32 {
	soundStart()
	if voice < 0 || voice >= spuVoices {
		return -1
	}
	sv := &snd.voices[voice]
	if sv.vab != vabid || sv.prog != prog || sv.tone != tone || sv.note != note {
		return -1
	}
	noteKeyOff(int(voice))
	return 0
}

// SsUtAllKeyOff silences every voice at once, parking each on the silent
// block and keying it off at the next tick, one voice after another with
// interrupts on, so that a tick in between keys off those done so far.
// Every voice is free and as old as the others, though one the next tick
// finds still sounding is taken again until its envelope reaches 0.
// Register writes already waiting for the tick (a closed sequence's
// volumes of 0) are still made. The voices' records are dropped: SsUtKeyOff
// matches none of them after.
//
//export SsUtAllKeyOff
func SsUtAllKeyOff(mode int16) {
	soundStart()
	for v := range snd.voices {
		dropVoice(v)
	}
	pace(ssUtAllKeyOffPad)
}

// dropVoice is what SsUtAllKeyOff does to voice v, on its own so that the call
// saves no more than it must.
//
//go:noinline
func dropVoice(v int) {
	parkVoice(uint32(v))
	sv := &snd.voices[v]
	sv.keyed, sv.fresh, sv.env, sv.age = false, false, 0, 0
	sv.seq, sv.vab = -1, -1
	keyOff(v)
}

// utVolumeScale is what SsUtSetVVol multiplies a volume by for the SPU's
// register, and SsUtGetVVol divides by.
const utVolumeScale = 129

// SsUtSetVVol sets voice's volumes, 0 to 127 each, as they are: no
// squaring, no bank or tone volume, from the next tick.
//
//export SsUtSetVVol
func SsUtSetVVol(voice, l, r int16) int32 {
	soundStart()
	if voice < 0 || voice >= spuVoices {
		return -1
	}
	sv := &snd.voices[voice]
	sv.setReg(regVolL, uint16(int32(l)*utVolumeScale))
	sv.setReg(regVolR, uint16(int32(r)*utVolumeScale))
	return 0
}

// SsUtGetVVol returns voice's volumes as the SPU has them now, over 129:
// nothing a call has staged since the last tick, and for SsUtKeyOn's volumes
// what the squaring made of them, 32 for 64 (spec 23).
//
//export SsUtGetVVol
func SsUtGetVVol(voice int16, l, r *int16) int32 {
	soundStart()
	if voice < 0 || voice >= spuVoices {
		return -1
	}
	r0 := uint32(voice) * 16
	*l = int16(spuGet(r0+regVolL*2) / utVolumeScale)
	*r = int16(spuGet(r0+regVolR*2) / utVolumeScale)
	return 0
}

// utVoice returns voice's state if it is a voice number and plays the
// bank and program given, or nil.
func utVoice(voice, vabid, prog int16) *sndVoice {
	if voice < 0 || voice >= spuVoices || vabid < 0 || vabid >= vabMax || !vabs[vabid].open {
		return nil
	}
	sv := &snd.voices[voice]
	if sv.vab != vabid || sv.prog != prog {
		return nil
	}
	return sv
}

// SsUtChangePitch retunes voice, which SsUtKeyOn started with the bank,
// program and note given (the fine pitch given is not checked), to
// newNote and newFine, at the next tick, without any bend. The voice
// keeps the note it was keyed on with, for later calls to match. It
// returns 0, or -1 if the voice does not match.
//
//export SsUtChangePitch
func SsUtChangePitch(voice, vabid, prog, oldNote, oldFine, newNote, newFine int16) int32 {
	soundStart()
	sv := utVoice(voice, vabid, prog)
	if sv == nil || sv.note != oldNote {
		return -1
	}
	t := vabs[vabid].tone(int(prog), int(sv.tone))
	sv.setReg(regPitch, notePitch(int(newNote), int(newFine), t.center, t.shift))
	sv.changed = true
	return 0
}

// SsUtPitchBend bends voice, which SsUtKeyOn started with the bank and
// program given, by b (0 to 127, 64 the center, as a sequence's pitch
// bend) from the note it was keyed on with, its fine pitch left out, at
// the next tick. The pitch is written even when note is not the voice's;
// the result is then -1, unless SsUtChangePitch has moved the voice. It
// returns -1 without writing for a voice of another bank or program.
//
//export SsUtPitchBend
func SsUtPitchBend(voice, vabid, prog, note, b int16) int32 {
	soundStart()
	sv := utVoice(voice, vabid, prog)
	if sv == nil {
		return -1
	}
	t := vabs[vabid].tone(int(prog), int(sv.tone))
	sv.setReg(regPitch, notePitch(int(sv.note), bendFine(t, int(b)), t.center, t.shift))
	if sv.note != note && !sv.changed {
		return -1
	}
	return 0
}
