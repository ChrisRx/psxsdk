package emu

import "encoding/binary"

// SPU register offsets.
const (
	spuKONL    = 0x188
	spuKONH    = 0x18a
	spuKOFFL   = 0x18c
	spuKOFFH   = 0x18e
	spuENDX    = 0x19c
	spuMBASE   = 0x1a2
	spuTADDR   = 0x1a6
	spuTFIFO   = 0x1a8
	spuSPUCNT  = 0x1aa
	spuSPUSTAT = 0x1ae
)

// Where psxe keeps its internal state, past the registers in the same block
// of memory, and so where an access to the upper half of the SPU's range
// lands.
const (
	spuIntTAddr     = 0x200 // u32: transfer address, in bytes
	spuIntTFIFO     = 0x204 // 32 x u16: data written and not yet transferred
	spuIntTFIFOLen  = 0x244 // u16
	spuIntRevbAddr  = 0x246 // u32: reverb work area, in bytes
	spuRAMSize      = 0x80000
	spuVoiceCount   = 24
	spuVoiceEnvCVol = 0x0c // a voice's current envelope volume
)

// spu is the sound processor at 1F801C00. Its voices advance as they would
// while playing, since a program can see that through the registers: which
// voices have ended, their envelope volumes, and the interrupt raised when a
// voice reaches a chosen address. It also makes the sound, as psxe does,
// since its reverb writes to SPU RAM, where a program can read it back.
//
// psxe keeps the registers in a packed struct laid out like the hardware's
// register map, and reads and writes it as raw memory, with its internal
// state following the registers. regs is that struct's bytes, so a register
// access anywhere in the range finds what psxe would. The voices' playback
// state comes after that and is kept separately here.
type spu struct {
	regs   [0x400]byte
	ram    [spuRAMSize]byte
	voices [spuVoiceCount]voice
	ic     *ic

	// The sound: each voice's decoding and interpolation state, and,
	// with audio on, the samples made and not yet taken.
	audio  bool
	sounds [spuVoiceCount]voiceSound
	out    []int16

	// CD audio: the drive's sample for this one, and with capture on, the
	// next place in the capture buffers it is written to.
	cdIn       [2]int16
	capture    bool
	capturePos uint32
}

// voiceSound is a voice's sound, as psxe makes it.
type voiceSound struct {
	buf        [28]int16 // the block's samples, decoded
	h          [2]int16  // the last two samples decoded, for the ADPCM filter
	s          [4]int16  // the last four samples played, newest first
	prevIndex  uint32    // the sample played last
	lvol, rvol float32   // the voice's volume when keyed on
}

// spuAudioMax bounds the samples kept for a frontend to take, a second's
// worth: if it does not keep up, the oldest are dropped.
const spuAudioMax = 2 * 44100

func newSPU(ic *ic) *spu {
	s := &spu{ic: ic}
	s.put32(spuENDX, 0x00ffffff) // every voice ended
	s.put16(spuIRQAddr, 0xffff)
	return s
}

const (
	spuIRQAddr      = 0x1a4
	spuIntEvenCycle = 0x252 // int: toggled every sample, for the reverb
	spuMainVolL     = 0x180
	spuCDVolL       = 0x1b0
	spuCDVolR       = 0x1b2
	spuRAMCtrl      = 0x1ac
	spuMainVolR     = 0x182

	// Voice registers, relative to the voice's block of 16 bytes.
	voiceVolumeL = 0x00
	voiceVolumeR = 0x02
	voicePitch   = 0x04
	voiceStart   = 0x06
	voiceADSR    = 0x08
	voiceEnvCVol = 0x0c
	voiceRepeat  = 0x0e
)

// voice is a voice's playback state.
type voice struct {
	playing    bool
	counter    uint32 // position within the block, in 1/4096ths of a sample
	addr       uint32 // the block being played, in bytes
	repeat     uint32 // where a looping voice goes back to
	blockFlags uint8
	adsr       adsr
}

func (s *spu) voiceReg(v, off uint32) uint16 { return s.get16(v*16 + off) }

func (s *spu) get16(off uint32) uint16    { return binary.LittleEndian.Uint16(s.regs[off:]) }
func (s *spu) get32(off uint32) uint32    { return binary.LittleEndian.Uint32(s.regs[off:]) }
func (s *spu) put16(off uint32, v uint16) { binary.LittleEndian.PutUint16(s.regs[off:], v) }
func (s *spu) put32(off, v uint32)        { binary.LittleEndian.PutUint32(s.regs[off:], v) }

func (s *spu) Read32(off uint32) uint32 { return s.get32(off) }
func (s *spu) Read8(uint32) uint8       { return 0 }

func (s *spu) Read16(off uint32) uint16 {
	if off == spuTFIFO {
		// Reading the FIFO reads SPU RAM at the transfer address.
		addr := s.get32(spuIntTAddr)
		v := binary.LittleEndian.Uint16(s.ram[addr&(spuRAMSize-1):])
		s.put32(spuIntTAddr, addr+2)
		return v
	}
	return s.get16(off)
}

func (s *spu) Write32(off, v uint32) {
	if !s.special(off, v) {
		s.put32(off, v)
	}
}

func (s *spu) Write16(off uint32, v uint16) {
	if s.special(off, uint32(v)) {
		return
	}
	// psxe drops 16-bit writes to voice 0's current envelope volume, and
	// only voice 0's.
	if off != spuVoiceEnvCVol {
		s.put16(off, v)
	}
}

func (s *spu) Write8(uint32, uint8) {}

// special handles the registers whose writes do more than store the value,
// and reports whether off was one.
func (s *spu) special(off, v uint32) bool {
	high := func(v uint32) uint32 {
		if off&2 != 0 {
			return v << 16
		}
		return v
	}
	switch off {
	case spuKONL, spuKONH:
		if v != 0 {
			s.keyOn(high(v))
		}
	case spuKOFFL, spuKOFFH:
		if v != 0 {
			s.keyOff(high(v))
		}
	case spuTADDR:
		s.put16(spuTADDR, uint16(v))
		s.put32(spuIntTAddr, v<<3)
	case spuTFIFO:
		s.put16(spuTFIFO, uint16(v))
		// psxe does not bound the index: past 32 entries the FIFO runs
		// into the fields after it, its own length first.
		n := s.get16(spuIntTFIFOLen)
		if at := spuIntTFIFO + 2*uint32(n); at+2 <= uint32(len(s.regs)) {
			s.put16(at, uint16(v))
		}
		n++
		s.put16(spuIntTFIFOLen, n)
		// A full FIFO is written out only in manual write mode.
		if n == 32 && s.get16(spuSPUCNT)>>4&3 == 2 {
			s.flushFIFO()
		}
	case spuSPUCNT:
		s.put16(spuSPUCNT, uint16(v))
		s.put16(spuSPUSTAT, s.get16(spuSPUSTAT)&0xffc0|uint16(v&0x3f))
		if v>>4&3 != 0 {
			s.flushFIFO()
		}
	case spuMBASE:
		s.put16(spuMBASE, uint16(v))
		s.put32(spuIntRevbAddr, v<<3)
	default:
		return false
	}
	return true
}

// flushFIFO writes the transfer FIFO to SPU RAM at the transfer address.
func (s *spu) flushFIFO() {
	addr := s.get32(spuIntTAddr)
	n := uint32(s.get16(spuIntTFIFOLen))
	for i := range min(n, 32) {
		v := s.get16(spuIntTFIFO + 2*i)
		s.ram[addr&(spuRAMSize-1)] = byte(v)
		s.ram[(addr+1)&(spuRAMSize-1)] = byte(v >> 8)
		addr += 2
	}
	s.put32(spuIntTAddr, addr)
	s.put16(spuIntTFIFOLen, 0)
}

// keyOn starts voices from their start address, with their envelope in the
// attack phase. KON itself is not stored.
//
// psxe sets the envelope volume register to full, though the attack starts
// from zero; the first envelope step that is due brings it back down. The
// position within the block is not reset.
func (s *spu) keyOn(mask uint32) {
	for i := range uint32(spuVoiceCount) {
		if mask&(1<<i) == 0 {
			continue
		}
		v := &s.voices[i]
		v.playing = true
		v.addr = uint32(s.voiceReg(i, voiceStart)) << 3
		v.repeat = uint32(s.voiceReg(i, voiceRepeat)) << 3
		lo, hi := s.voiceReg(i, voiceADSR), s.voiceReg(i, voiceADSR+2)
		v.adsr.attack(uint32(hi)<<16 | uint32(lo))
		snd := &s.sounds[i]
		snd.lvol = float32(float32(s.voiceReg(i, voiceVolumeL))/32767) * 2
		snd.rvol = float32(float32(s.voiceReg(i, voiceVolumeR))/32767) * 2
		s.readBlock(i)
		s.put16(i*16+voiceEnvCVol, 0x7fff)
	}
	s.put32(spuENDX, s.get32(spuENDX)&^(mask&0x00ffffff))
}

// keyOff puts voices into their release phase, which psxe reports as ended
// straight away.
func (s *spu) keyOff(mask uint32) {
	for i := range uint32(spuVoiceCount) {
		if mask&(1<<i) != 0 {
			s.release(i)
		}
	}
}

func (s *spu) release(i uint32) {
	s.voices[i].adsr.release()
	s.put32(spuENDX, s.get32(spuENDX)|1<<i)
}

// readBlock reads the ADPCM block a voice is at: its flags, and its 28
// samples, decoded.
func (s *spu) readBlock(i uint32) {
	v := &s.voices[i]
	v.blockFlags = s.ram[(v.addr+1)&(spuRAMSize-1)]
	s.decodeBlock(v.addr, &s.sounds[i])
}

// ADPCM filters' coefficients, in 64ths, for the last sample and the one
// before it.
var (
	adpcmPos = [5]int32{0, 60, 115, 98, 122}
	adpcmNeg = [5]int32{0, 0, -52, -55, -60}
)

// decodeBlock decodes an ADPCM block of 28 4-bit samples, each scaled by
// the header's shift and added to its filter of the samples before it.
//
// As psxe does it, the sum is taken to 16 bits by truncation, so it wraps
// rather than saturating, and filters past 4 read past its tables; here
// they read as filter 0.
func (s *spu) decodeBlock(addr uint32, snd *voiceSound) {
	at := func(off uint32) byte { return s.ram[(addr+off)&(spuRAMSize-1)] }
	hdr := at(0)
	shiftIn := uint32(hdr & 0x0f)
	if shiftIn > 12 {
		shiftIn = 9
	}
	shift := 12 - shiftIn
	var f0, f1 int32
	if filter := hdr >> 4 & 7; filter < 5 {
		f0, f1 = adpcmPos[filter], adpcmNeg[filter]
	}
	for j := range uint32(28) {
		n := at(2+j>>1) >> (j & 1 * 4) & 0xf
		t := int32(int16(uint16(n)<<12) >> 12)
		v := int16(t<<shift + (int32(snd.h[0])*f0+int32(snd.h[1])*f1+32)/64)
		snd.h[1], snd.h[0] = snd.h[0], v
		snd.buf[j] = v
	}
}

// ADPCM block flags.
const (
	blockEnd    = 1 << 0
	blockRepeat = 1 << 1
	blockStart  = 1 << 2
)

// sample makes one sample, as psxe does every 768 cycles while tracing:
// every voice advances, and the voices are mixed, with reverb if it is on.
// The reverb writes to its work area in SPU RAM, which a program can read,
// so the mix is made whether or not audio is on; audio only keeps it.
func (s *spu) sample() {
	even := s.get32(spuIntEvenCycle) ^ 1
	s.put32(spuIntEvenCycle, even)
	s.put32(spuKONL, 0)
	s.put32(spuKOFFL, 0)

	irqAddr := uint32(s.get16(spuIRQAddr)) << 3
	irqEnabled := s.get16(spuSPUCNT)&0x40 != 0
	eon := s.get32(spuEON)
	var left, right, revl, revr int32

	for i := range uint32(spuVoiceCount) {
		v := &s.voices[i]
		if !v.playing {
			continue
		}

		level, ended := v.adsr.step()
		if ended {
			v.playing = false
		}
		if level >= 0 {
			s.put16(i*16+voiceEnvCVol, uint16(level))
		}

		// Each block holds 28 samples.
		if idx := v.counter >> 12; idx > 27 {
			v.counter = v.counter&0xfff | (idx-28)<<12
			if v.blockFlags&blockStart != 0 {
				v.repeat = v.addr
			}
			switch v.blockFlags & (blockEnd | blockRepeat) {
			case 0, blockRepeat:
				if irqEnabled && irqAddr == v.addr {
					s.ic.raise(irqSPU)
				}
				v.addr += 0x10
				if irqEnabled && irqAddr == v.addr {
					s.ic.raise(irqSPU)
				}
			case blockEnd:
				v.addr = v.repeat
				v.playing = false
				s.put16(i*16+voiceEnvCVol, 0)
				s.release(i)
			case blockEnd | blockRepeat:
				s.put32(spuENDX, s.get32(spuENDX)|1<<i)
				v.addr = v.repeat
			}
			s.readBlock(i)
		}

		// psxe adds each voice's float to an int.
		l, r := s.voiceSample(i)
		left = int32(float32(left) + l)
		right = int32(float32(right) + r)
		if eon&(1<<i) != 0 {
			revl = int32(float32(revl) + l)
			revr = int32(float32(revr) + r)
		}

		v.counter += uint32(s.voiceReg(i, voicePitch))
	}

	// CD audio comes in at its own volume, if its input is on, and into the
	// reverb if that is on for it too.
	if s.capture {
		s.captureCD(irqEnabled, irqAddr)
	}
	var cdl, cdr int32
	if cnt := s.get16(spuSPUCNT); cnt&1 != 0 {
		cdl = int32(s.cdIn[0]) * int32(int16(s.get16(spuCDVolL))) >> 15
		cdr = int32(s.cdIn[1]) * int32(int16(s.get16(spuCDVolR))) >> 15
		if cnt&4 != 0 {
			revl += cdl
			revr += cdr
		}
	}

	s.mix(left, right, revl, revr, cdl, cdr, even != 0)
}

// voiceSample is a voice's part of the mix, left and right, as psxe works it
// out: the sample at the voice's position, interpolated from the four
// played last with the Gaussian table, scaled by the voice's envelope and
// its volume. The arithmetic is psxe's, 16-bit sums wrapping included.
func (s *spu) voiceSample(i uint32) (l, r float32) {
	v, snd := &s.voices[i], &s.sounds[i]
	idx := v.counter >> 12
	if snd.prevIndex != idx {
		snd.s[3], snd.s[2], snd.s[1] = snd.s[2], snd.s[1], snd.s[0]
	}
	// Past the block's end, the position has always been taken back by
	// now; the modulo only keeps an index in range regardless.
	snd.s[0] = snd.buf[idx%28]
	snd.prevIndex = idx

	g := (v.counter >> 4) & 0xff
	out := int16(int32(spuGauss[0x0ff-g]) * int32(snd.s[3]) >> 15)
	out += int16(int32(spuGauss[0x1ff-g]) * int32(snd.s[2]) >> 15)
	out += int16(int32(spuGauss[0x100+g]) * int32(snd.s[1]) >> 15)
	out += int16(int32(spuGauss[0x000+g]) * int32(snd.s[0]) >> 15)

	env := float32(s.voiceReg(i, voiceEnvCVol)) / 32767
	l = float32(float32(out)*snd.lvol) * env
	r = float32(float32(out)*snd.rvol) * env
	return l, r
}

// mix makes a sample from the voices' sums, the sums of those sent to the
// reverb, and the CD audio: with the SPU's output muted, the CD audio
// alone, which the mute bit does not affect; otherwise, with reverb on, the
// reverb takes a sample, on even samples, and what it last made is added
// in. Each channel is clamped to 16 bits and scaled by the main volume.
func (s *spu) mix(left, right, revl, revr, cdl, cdr int32, even bool) {
	var l, r int16
	mainL, mainR := s.get16(spuMainVolL), s.get16(spuMainVolR)
	if cnt := s.get16(spuSPUCNT); cnt&0x4000 != 0 {
		if cnt&0x80 != 0 {
			if even {
				s.reverb(clamp16(revl), clamp16(revr))
			}
			left = int32(clamp16(left)) + int32(s.get32(spuIntRevL))
			right = int32(clamp16(right)) + int32(s.get32(spuIntRevR))
		}
		l = mainVolume(left+cdl, mainL)
		r = mainVolume(right+cdr, mainR)
	} else if cdl|cdr != 0 {
		l = mainVolume(cdl, mainL)
		r = mainVolume(cdr, mainR)
	}
	if !s.audio {
		return
	}
	if len(s.out)+2 > spuAudioMax {
		n := copy(s.out, s.out[len(s.out)-spuAudioMax/2:])
		s.out = s.out[:n]
	}
	s.out = append(s.out, l, r)
}

func clamp16(v int32) int16 { return int16(min(max(v, -0x8000), 0x7fff)) }

// mainVolume scales a channel's sum by the main volume register. psxe takes
// the result to 16 bits by truncation, so a volume past 7FFFh wraps it.
func mainVolume(sum int32, vol uint16) int16 {
	return int16(int32(float32(float32(clamp16(sum))*float32(vol)) / 32767))
}

// setAudio turns the sound on or off for a frontend to take.
func (s *spu) setAudio(on bool) {
	s.audio = on
	s.out = s.out[:0]
}

// takeAudio appends the samples made since the last call to dst.
func (s *spu) takeAudio(dst []int16) []int16 {
	dst = append(dst, s.out...)
	s.out = s.out[:0]
	return dst
}

// adsr is a voice's envelope: attack, decay, sustain and release. It keeps
// the voice's ADSR registers as they were when the voice was keyed on, which
// is what each phase takes its settings from.
type adsr struct {
	envctl  uint32
	sustain int // the level the decay phase ends at

	phase       int
	cycles      int // samples until the next step
	reload      int
	exponential bool
	decrease    bool
	shift       int
	stepSize    int
	levelStep   int
	level       int
}

const (
	adsrAttack = iota
	adsrDecay
	adsrSustain
	adsrRelease
	adsrEnd
)

// calculate works out the step size and how often it is taken, from the
// phase's shift and step and the current level.
func (a *adsr) calculate() {
	a.cycles = 1 << max(0, a.shift-11)
	a.levelStep = a.stepSize << max(0, 11-a.shift)
	if a.exponential && a.level > 0x6000 && !a.decrease {
		a.cycles *= 4
	}
	if a.exponential && a.decrease {
		a.levelStep = (a.levelStep * a.level) >> 15
	}
	a.reload = a.cycles
}

// attack starts the envelope for a voice being keyed on.
func (a *adsr) attack(envctl uint32) {
	a.envctl = envctl
	a.sustain = (int(envctl&0xf) + 1) * 0x800
	// psxe takes the attack mode as envctl>>15 without masking it, so any
	// of the sustain and release settings above it make it exponential too.
	a.exponential = envctl>>15 != 0
	a.decrease = false
	a.shift = int(envctl >> 10 & 0x1f)
	a.stepSize = 7 - int(envctl>>8&3)
	a.level = 0
	a.phase = adsrAttack
	a.calculate()
}

func (a *adsr) decay() {
	a.exponential = true
	a.decrease = true
	a.shift = int(a.envctl >> 4 & 0xf)
	a.stepSize = -8
	a.level = 0x7fff
	a.phase = adsrDecay
	a.calculate()
}

func (a *adsr) sustainPhase() {
	a.exponential = a.envctl>>31 != 0
	a.decrease = a.envctl>>30&1 != 0
	a.shift = int(a.envctl >> 24 & 0x1f)
	step := int(a.envctl >> 22 & 3)
	a.level = a.sustain
	if a.decrease {
		a.stepSize = -8 + step
	} else {
		a.stepSize = 7 - step
	}
	a.phase = adsrSustain
	a.calculate()
}

func (a *adsr) release() {
	a.exponential = a.envctl>>21&1 != 0
	a.decrease = true
	a.shift = int(a.envctl >> 16 & 0x1f)
	a.stepSize = -8
	a.phase = adsrRelease
	a.calculate()
}

// step advances the envelope by a sample. It returns the new level, or -1
// if no step was due and the level is unchanged, and whether the voice has
// stopped.
func (a *adsr) step() (level int, ended bool) {
	if a.cycles > 0 {
		a.cycles--
		return -1, false
	}
	a.calculate()
	a.level += a.levelStep

	clamp := func() { a.level = min(max(a.level, 0), 0x7fff) }
	switch a.phase {
	case adsrAttack:
		clamp()
		if a.level == 0x7fff {
			a.decay()
		}
	case adsrDecay:
		clamp()
		if a.level <= a.sustain {
			a.sustainPhase()
		}
	case adsrSustain:
		// It stays here until the voice is keyed off.
		clamp()
	case adsrRelease:
		clamp()
		if a.level == 0 {
			a.phase = adsrEnd
			a.cycles = 0
			a.levelStep = 0
			ended = true
		}
	case adsrEnd:
		ended = true
	}
	a.cycles = a.reload
	return a.level, ended
}

// captureCD writes the CD audio sample, before the SPU's CD volume, to the
// capture buffers at the start of SPU RAM: left at 000h, right at 400h,
// each a ring of 200h samples. STATX bit 11 says which half is being
// written. A write at the IRQ address raises the SPU's interrupt, if it is
// on, and RAM_CTRL bits 2 and 3 are not both clear.
func (s *spu) captureCD(irqEnabled bool, irqAddr uint32) {
	pos := s.capturePos
	ctrl := s.get16(spuRAMCtrl)&0xc != 0
	for ch, base := range [2]uint32{0x000, 0x400} {
		addr := base + pos*2
		binary.LittleEndian.PutUint16(s.ram[addr:], uint16(s.cdIn[ch]))
		if irqEnabled && ctrl && irqAddr == addr {
			s.ic.raise(irqSPU)
		}
	}
	s.put16(spuSPUSTAT, s.get16(spuSPUSTAT)&^0x800|uint16(b2i(pos >= 0x100))<<11)
	s.capturePos = (pos + 1) & 0x1ff
}
