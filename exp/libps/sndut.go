//go:build yaroze

package libps

// The sequencer's utilities: a program's note keyed on and off by hand,
// on a voice the utilities mark as their own, its volume and pitch
// changed while it plays, and the main volume and mute read and set.

// SsUtKeyOn keys note of tone of program prog of bank vab on, with fine
// sixteenths of a semitone added, at the volumes voll and volr, each up
// to 127, and returns the voice it took, or -1 when the bank or program
// is not ready, the tone has no sample, no voice is free, or a tick is
// in progress.
//
// libps 0x8001A33C, 932 bytes
//
//export SsUtKeyOn
func SsUtKeyOn(vab, prog, tone, note, fine, voll, volr int16) int16 {
	sndStart()
	if sndInTick {
		return -1
	}
	sndInTick = true
	if sndSetVab(vab, prog) != 0 {
		sndInTick = false
		return -1
	}
	sndCur.seq = sndUtSeq
	sndCur.note = uint8(note)
	sndCur.tone = uint8(tone)
	switch {
	case voll == volr:
		sndCur.chPan = 0x40
		sndCur.vel = uint8(voll)
	case volr < voll:
		sndCur.vel = uint8(voll)
		sndCur.chPan = uint8((int32(volr) << 6) / int32(voll))
	default:
		sndCur.vel = uint8(volr)
		sndCur.chPan = 0x7f - uint8((int32(voll)<<6)/int32(volr))
	}
	p := vabProg(int32(prog))
	sndCur.progVol = peek8(p, progVol)
	sndCur.progPan = peek8(p, progPan)
	sndCur.ntones = peek8(p, progTones)
	tp := vabTone(int32(sndCur.progIdx), int32(tone))
	sndCur.prio = peek8(tp, tonePrio)
	sndCur.vag = int16(peek16(tp, toneVAG))
	sndCur.toneVol = peek8(tp, toneVol)
	sndCur.tonePan = peek8(tp, tonePan)
	sndCur.center = peek8(tp, toneCenter)
	sndCur.shift = peek8(tp, toneShift)
	sndCur.mode = peek8(tp, toneMode)
	sndCur.min = peek8(tp, toneMin)
	sndCur.max = peek8(tp, toneMax)
	if sndCur.vag == 0 {
		sndInTick = false
		return -1
	}
	v := sndAllocVoice()
	if v == int32(sndNVoices) {
		sndInTick = false
		return -1
	}
	sndCur.voice = int16(v)
	vc := &sndVoices[v]
	vc.seq = sndUtSeq
	vc.vab = vab
	vc.prog = prog
	vc.progIdx = int16(sndCur.progIdx)
	vc.vag = sndCur.vag
	vc.note = note
	vc.status = voiceOn
	vc.age = 0
	vc.tone = tone
	sndVoiceRegs()
	// A sample numbered 255 took the original down its own volume path
	// (libps 0x80016FA0), which is not translated, as in sndKeyOn.
	sndKeyOnVolume(sndPitchFrom(int32(note), int32(fine)))
	sndInTick = false
	return int16(v)
}

// SsUtKeyOff keys the voice off, when it plays note of tone of program
// prog of bank vab, and returns 0, or -1 when it does not or a tick is
// in progress.
//
// libps 0x8001A6E0, 500 bytes
//
//export SsUtKeyOff
func SsUtKeyOff(voice, vab, prog, tone, note int16) int16 {
	if sndInTick {
		return -1
	}
	sndInTick = true
	if uint16(voice) >= 24 {
		sndInTick = false
		return -1
	}
	vc := &sndVoices[voice]
	if vc.vab != vab || vc.prog != prog || vc.tone != tone || vc.note != note {
		sndInTick = false
		return -1
	}
	if vc.vag == 0xff {
		vc.status = voiceFree
		vc.pitch = 0
		spuReg(spuNoiseLo).Set(0)
		spuReg(spuNoiseHi).Set(0)
	} else {
		sndCur.voice = voice
		vc.status = voiceFree
		vc.pitch = 0
		vc.vag = 0
		sndKeyOff(int32(voice))
	}
	sndInTick = false
	return 0
}

// SsUtAllKeyOff keys every voice off and clears its registers and its
// entry, whatever it was playing for. The mode is not used.
//
// libps 0x8001B2B4, 476 bytes
//
//export SsUtAllKeyOff
func SsUtAllKeyOff(mode int16) {
	for v := int32(0); v < int32(sndNVoices); v++ {
		vc := &sndVoices[v]
		vc.age = 24
		vc.vag = 0xff
		vc.status = voiceFree
		vc.pitch = 0
		vc.envelope = 0
		vc.seq = 0xff
		vc.progIdx = 0
		vc.prog = 0
		vc.tone = 0xff
		base := uintptr(v) * 16
		spuReg(base + 6).Set(0x200)
		spuReg(base + 4).Set(0x1000)
		spuReg(base + 8).Set(0x80ff)
		spuReg(base + 0).Set(0)
		spuReg(base + 2).Set(0)
		spuReg(base + 10).Set(0x4000)
		sndCur.voice = int16(v)
		sndKeyOff(v)
	}
}

// SsUtSetVVol sets the voice's volume, each side up to 127, which the
// next tick writes.
//
// libps 0x8001B180, 140 bytes
//
//export SsUtSetVVol
func SsUtSetVVol(voice, voll, volr int16) int16 {
	if uint16(voice) >= 24 {
		return -1
	}
	sndRegs[voice].volL = uint16(int32(voll) * 129)
	sndRegs[voice].volR = uint16(int32(volr) * 129)
	sndDirty[voice] |= dirtyVol
	return 0
}

// SsUtGetVVol reads the voice's volume from the processor, each side up
// to 127.
//
// libps 0x8001B0F4, 140 bytes
//
//export SsUtGetVVol
func SsUtGetVVol(voice int16, voll, volr *int16) int16 {
	if uint16(voice) >= 24 {
		return -1
	}
	base := uintptr(voice) * 16
	*voll = int16(int32(int16(spuReg(base).Get())) / 129)
	*volr = int16(int32(int16(spuReg(base+2).Get())) / 129)
	return 0
}

// SsUtChangePitch moves the voice, playing oldNote of program prog of
// bank vab, to newNote with newFine sixteenths of a semitone, which the
// next tick writes, and returns 0, or -1 when the voice plays something
// else. The old fine pitch is not looked at.
//
// libps 0x8001AE18, 320 bytes
//
//export SsUtChangePitch
func SsUtChangePitch(voice, vab, prog, oldNote, oldFine, newNote, newFine int16) int16 {
	if uint16(voice) >= 24 {
		return -1
	}
	vc := &sndVoices[voice]
	if vc.vab != vab || vc.prog != prog || vc.note != oldNote {
		return -1
	}
	sndSetVab(vab, prog)
	sndCur.seq = sndUtSeq
	sndCur.voice = voice
	sndCur.tone = uint8(vc.tone)
	sndRegs[voice].pitch = sndPitchFrom(int32(newNote), int32(newFine))
	sndDirty[voice] |= dirtyPitch
	return 0
}

// SsUtPitchBend bends the voice, playing program prog of bank vab, to
// pbend, 64 being none, and returns 0, or -1 when the voice plays
// something else. The note is not looked at.
//
// libps 0x8001AD88, 144 bytes
//
//export SsUtPitchBend
func SsUtPitchBend(voice, vab, prog, note, pbend int16) int16 {
	sndSetVab(vab, prog)
	sndCur.seq = sndUtSeq
	if sndBendVoice(int32(voice), sndUtSeq, vab, prog, uint8(pbend)) == 0 {
		return -1
	}
	return 0
}

// SsGetMVol reads the main volume, each side up to 127.
//
// libps 0x80010CB4, 128 bytes
//
//export SsGetMVol
func SsGetMVol(vol *SndVolume) {
	sndStart()
	var a SpuCommonAttr
	SpuGetCommonAttr(&a)
	vol.Left = uint16(int32(a.MVolL) / 129)
	vol.Right = uint16(int32(a.MVolR) / 129)
}

// SsSetMute mutes the processor with 1 and unmutes it with 0; any other
// mode does nothing.
//
// libps 0x80010EA0, 64 bytes
//
//export SsSetMute
func SsSetMute(mode uint8) {
	sndStart()
	if mode <= 1 {
		spuSetMute(int32(mode))
	}
}

// SsGetMute reports whether the processor is muted.
//
// libps 0x80010C90, 36 bytes
//
//export SsGetMute
func SsGetMute() uint8 {
	sndStart()
	return b2i8(spuMuted())
}

func b2i8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
