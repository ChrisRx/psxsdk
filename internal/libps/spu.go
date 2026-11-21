//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The SPU, as the sound functions drive it: its registers, its start-up,
// its sound memory, and the voices, whose registers are written once a
// vertical blank by the tick, which then runs the sequences.

// SPU registers, as offsets from 1F801C00h. Each voice has 16 bytes of
// its own from 0.
const (
	spuVolL    = 0x00 // a voice's volume, left and right
	spuVolR    = 0x02
	spuPitch   = 0x04
	spuAddr    = 0x06 // where its sample starts, in 8-byte units
	spuADSR1   = 0x08
	spuADSR2   = 0x0a
	spuEnv     = 0x0c // its envelope's current level
	spuRepeat  = 0x0e
	spuMainL   = 0x180
	spuMainR   = 0x182
	spuRevL    = 0x184
	spuRevR    = 0x186
	spuKeyOn   = 0x188 // and 18Ah for voices 16 to 23
	spuKeyOff  = 0x18c // and 18Eh
	spuFM      = 0x190
	spuNoise   = 0x194
	spuReverb  = 0x198 // the voices sent through the reverb, and 19Ah
	spuEndX    = 0x19c
	spuRevBase = 0x1a2
	spuXferAt  = 0x1a6
	spuFIFO    = 0x1a8
	spuCtrl    = 0x1aa
	spuStat    = 0x1ae
	spuXferCtl = 0x1ac
	spuCDL     = 0x1b0
	spuCDR     = 0x1b2
	spuExtL    = 0x1b4
	spuExtR    = 0x1b6
	spuRevRegs = 0x1c0 // the reverb's 32 settings

	spuVoices = 24
	allVoices = 1<<spuVoices - 1

	// Sound memory below this holds the SPU's capture buffers and the
	// silent block voices are parked on; banks go above it.
	spuUserBase = 0x1010
	spuRAMSize  = 0x80000
)

func spuReg(off uint32) *volatile.Register16 {
	return (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801c00 + off)))
}

func spuSet(off uint32, v uint16) { spuReg(off).Set(v) }
func spuGet(off uint32) uint16    { return spuReg(off).Get() }

// spuSetMask writes a 24-bit voice mask to the register pair at off.
func spuSetMask(off, mask uint32) {
	spuSet(off, uint16(mask))
	spuSet(off+2, uint16(mask>>16))
}

// The registers a voice's shadow holds, in the order the tick writes
// them, as indexes into sndVoice.regs and bits of sndVoice.dirty.
const (
	regVolL = iota
	regVolR
	regPitch
	regAddr
	regADSR1
	regADSR2
	voiceRegs
)

type sndVoice struct {
	// For allocation: whether the voice is keyed on (not since keyed
	// off), the envelope the last tick read (0 once SsUtKeyOn takes the
	// voice, until the next tick), how many ticks in a row have read 0 while it
	// was keyed on, when it was last taken (a count of key ons) and its
	// tone's priority.
	keyed bool

	// quiet is how many ticks in a row have read 0, keyed or not, up to silentReads. Sony's
	// tick takes longer over a voice at the top (fidelity.go). From the start-up its
	// count starts at 2: all but the first 14 ticks of a program take the longer way.
	quiet uint8

	// fresh is set from a key on to the end of the next tick: such a
	// voice is not free even when keyed off again before that.
	fresh bool
	env   uint16
	zeros uint8
	age   uint32
	prior uint8

	// What the voice plays, for a note off or SsUtKeyOff to find it:
	// the sequence that owns it (-1 for SsUtKeyOn), its channel, note,
	// bank (-1 while the voice has no record: never taken, or since
	// SsUtAllKeyOff), program and tone.
	seq       int8
	channel   uint8
	note      int16 // as it was given, whole: SsUtKeyOn's may be any
	vab, prog int16
	tone      int16
	// vel is a sequence's note's velocity, for its controllers to work
	// the volume out again; changed is set once SsUtChangePitch has moved
	// the voice (SsUtPitchBend then accepts any note), and stays.
	vel     uint8
	changed bool
	// noise is the voice's kind as its key on read it from the header: a
	// voice of the noise generator, whose note off and SsUtKeyOff write
	// noiseOff rather than a key off bit.
	noise bool

	// The registers the next tick writes, those whose dirty bit is set.
	regs  [voiceRegs]uint16
	dirty uint8
}

var snd struct {
	started bool

	voices [spuVoices]sndVoice

	// The key on and off masks the next tick writes, and the voices
	// that go through the reverb.
	keyOn, keyOff, reverb uint32

	// ons counts key ons, for the voices' ages.
	ons uint32

	// seqsOpen has a bit for each open sequence, for the tick's sequence
	// stage: a walk over the closed slots would take longer than Sony's
	// tick has left after the mask words.
	seqsOpen uint32

	// stopAsked is set when a sequence has asked to stop since the last
	// tick (see stopSequences).
	stopAsked bool

	// lastNoise is the voice of the last noise key on, -1 for none: a steal
	// of it or its freeing stops the noise (spec 05, "Noise voices").
	lastNoise int8
}

// soundStart brings the SPU up, once: from the library's start-up
// (startup, after the controller driver), or from the first sound or CD
// call in a program that makes one before any ResetGraph. The drive's
// reset comes first, as CdRead would run it, then the SPU reset to
// silence, every voice parked on a silent block at 1000h. The reads,
// writes and delay loops are the reference's, in its order (spec 08): the
// timing of what follows (the first tick, a CD read) depends on them.
func soundStart() {
	if !snd.started {
		soundBegin()
	}
}

// soundBegin is soundStart's work, apart so that a call after the start costs
// no stack.
//
//go:noinline
func soundBegin() {
	cd.start()
	// 1.
	dmaPriority.Set(dmaPriority.Get() | dmaSPUEnable)
	spuSet(spuMainL, 0)
	spuSet(spuMainR, 0)
	spuSet(spuCtrl, 0)
	spuDelay(1)
	// 2.
	spuSet(spuMainL, 0)
	spuSet(spuMainR, 0)
	spuGet(spuStat)
	spuSet(spuXferCtl, 4)
	spuSet(spuRevL, 0)
	spuSet(spuRevR, 0)
	spuSetMask(spuKeyOff, 0xffffffff)
	spuSetMask(spuReverb, 0)
	spuSetMask(spuFM, 0)
	spuSetMask(spuNoise, 0)
	spuSet(spuCDL, 0)
	spuSet(spuCDR, 0)
	spuSet(spuExtL, 0)
	spuSet(spuExtR, 0)
	// 3. The silent block, 16 bytes of 07h: an end and repeat flag on
	// every block, so a voice on it loops on silence.
	spuSet(spuXferAt, 0x1000>>3)
	spuGet(spuStat)
	spuDelay(1)
	for range 8 {
		spuSet(spuFIFO, 0x0707)
	}
	spuGet(spuCtrl)
	spuSet(spuCtrl, 0x10)
	spuDelay(1)
	spuGet(spuStat)
	spuDelay(2)
	spuGet(spuCtrl)
	spuSet(spuCtrl, 0)
	spuGet(spuStat)
	// 4.
	for v := range uint32(spuVoices) {
		r := v * 16
		spuSet(r+spuVolL, 0)
		spuSet(r+spuVolR, 0)
		spuSet(r+spuPitch, 0x3fff)
		spuSet(r+spuAddr, 0x1000>>3)
		spuSet(r+spuADSR1, 0)
		spuSet(r+spuADSR2, 0)
	}
	spuSet(spuKeyOn, 0xffff)
	spuGet(spuKeyOn + 2)
	spuSet(spuKeyOn+2, 0xff)
	spuDelay(4)
	// 5.
	spuSet(spuKeyOff, 0xffff)
	spuGet(spuKeyOff + 2)
	spuSet(spuKeyOff+2, 0xff)
	spuDelay(4)
	// 6. The reference turns the SPU's DMA interrupt on and opens an
	// event for it, which nothing in the clean library waits on; the
	// interrupt handler acknowledges the channel.
	spuSet(spuCtrl, 0xc000)
	spinCycles(startCtrl)
	imagePad(residentSoundPad)
	EnterCriticalSection()
	dmaInterrupt.Get()
	dmaInterrupt.Set(dicrMaster | dicrSPUEnable)
	ev := libps_biosB(biosOpenEvent, spuEventClass, 0x20, evModeNoIntr, 0)
	libps_biosB(biosEnableEvent, uintptr(ev), 0, 0, 0)
	ExitCriticalSection()
	spinCycles(startEvent)
	spuSet(spuRevBase, 0xfffe)
	// 7.
	for v := range uint32(spuVoices) {
		r := v * 16
		spuSet(r+spuVolL, 0)
		spuSet(r+spuVolR, 0)
		spuSet(r+spuPitch, 0x1000)
		spuSet(r+spuAddr, 0x3000)
		spuSet(r+spuADSR1, 0xbf)
		spuSet(r+spuADSR2, 0)
		spuSet(r+spuEnv, 0)
		spuSet(r+spuRepeat, 0)
	}
	spuSet(spuMainL, 0x3fff)
	spuSet(spuMainR, 0x3fff)
	for off := uint32(spuRevL); off <= spuEndX+2; off += 2 {
		spuSet(off, 0)
	}
	// The reference clears 288 bytes of its voice state here, a byte at
	// a time, and sets up more; the clean library's is set below.
	spinCycles(startClear)
	// Each voice parked, with the reference's bookkeeping for it.
	for v := range uint32(spuVoices) {
		parkVoice(v)
		spinCycles(startPark)
	}
	// 8. A tick with nothing to play: the envelopes read, then the key
	// off, key on and reverb masks, with the reference's time around
	// each.
	for v := range uint32(spuVoices) {
		spuGet(v*16 + spuEnv)
		spinCycles(startEnv)
	}
	spinCycles(startTick)
	spuSet(spuKeyOff, 0)
	spuSet(spuKeyOff+2, 0xff)
	spuSetMask(spuKeyOn, 0)
	spuSetMask(spuReverb, 0)
	for i := range snd.voices {
		snd.voices[i] = sndVoice{
			seq:   -1,
			vab:   -1,
			quiet: 2,
		}
	}
	snd.lastNoise = -1
	memInit()
	spinCycles(startSeqs)
	spuCount(1000)
	imagePad(residentSoundEndPad)
	// The controllers' and the vertical blank's acknowledgement handed to
	// the kernel and back, each with the interrupts masked, in a critical
	// section as Sony's does.
	EnterCriticalSection()
	m := iMask.Get()
	iMask.Set(0)
	libps_biosB(biosClearPad, 1, 0, 0, 0)
	libps_biosC(biosClearRCnt, 3, 1, 0, 0)
	iMask.Set(m &^ irqVBlank)
	m = iMask.Get()
	iMask.Set(0)
	libps_biosB(biosClearPad, 0, 0, 0, 0)
	libps_biosC(biosClearRCnt, 3, 0, 0, 0)
	iMask.Set(m | irqVBlank)
	ExitCriticalSection()
	// The tick runs at every vertical blank from here (spec 07): a blank
	// that comes earlier in the start-up is counted but not ticked, as in
	// the reference.
	snd.started = true
}

// The time the reference spends in the sound start-up on work that has no
// counterpart in the clean library, in cycles, measured with iotrace on
// invaders-ref (spec 08): after 1AAh=C000h; after enabling its event;
// clearing 288 bytes of voice state and the rest of its voice set-up;
// parking each voice; its tick between and after the envelope reads;
// setting up 16 slots for sequences.
const (
	startCtrl  = 420
	startEvent = 370
	startClear = 2482
	startPark  = 130
	startEnv   = 12
	startTick  = 2018
	startSeqs  = 442
)

// The SPU's kernel event, which the reference opens: its class, and the
// mode that has the kernel only mark it.
const (
	spuEventClass   = 0xf0000009
	evModeNoIntr    = 0x2000
	biosOpenEvent   = 0x08 // B0
	biosEnableEvent = 0x0c // B0
	dicrSPUEnable   = 1 << 20
	dicrSPUFlag     = 1 << 28
)

// spuCounter is what the start-up's delay loops count on, in memory as
// the reference's does.
var spuCounter uint32

// spuDelay runs n of the reference's delay loops, each 240 steps of about
// 10 cycles, as 300 of spuCount's steps of 8.
func spuDelay(n int) {
	for range n {
		spuCount(300)
	}
}

// spuCount counts from 1 to n, storing the count in spuCounter at each
// step, 8 cycles a step.
//
//go:noinline
func spuCount(n uint32) {
	for i := uint32(1); i <= n; i++ {
		volatile.StoreUint32(&spuCounter, i)
	}
}

// spuDMADone acknowledges the SPU's DMA interrupt, which the start-up
// turns on.
func spuDMADone() {
	if d := dmaInterrupt.Get(); d&dicrSPUFlag != 0 {
		dmaInterrupt.Set(d&0x00ffffff | dicrSPUFlag)
	}
}

// parkVoice points voice v at the silent block with its volume off.
func parkVoice(v uint32) {
	r := v * 16
	spuSet(r+spuAddr, 0x1000>>3)
	spuSet(r+spuPitch, 0x1000)
	spuSet(r+spuADSR1, 0x80ff)
	spuSet(r+spuVolL, 0)
	spuSet(r+spuVolR, 0)
	spuSet(r+spuADSR2, 0x4000)
}

// The tick takes as long as Sony's vertical blank handler, measured from
// the handler's I_STAT read on terra (spec 08, "Pacing to Sony's times"):
// the first envelope read 342 cycles after it, the reads 32 apart, the
// six key and reverb words at 4,634 and ReturnFromException at 5,204, in
// a frame with no sequence playing. The pads are what those leave once
// the clean handler's own work is taken off, measured the same way, a store
// or two (space) taking up what a pace's whole turns leave. A sequence
// playing lengthens the time after the words, where its stage runs.
const (
	tickReadsPad = 194 // before the reads, less libps_interrupt's spaces (spec 45)
	tickReadGap  = 32  // from one envelope read to the next; the loop's own work is 12
	tickExitPad  = 362 // after them: tickExitPace's turns come to it
)

// What fills the rest of tickReadGap: nops and then loads of spacer, 2
// cycles each in the emulator. They were ten stores to spacer, which on a
// console cost more than the emulator's model had them, each read's store
// being to another DRAM row than the pad's (spec 46). Nops and loads cost a
// console what the model says, and seven and three put the reads as far
// apart under a console's costs as Sony's, 52 or 53 cycles.
const (
	tickReadNops  = 7
	tickReadLoads = (tickReadGap-12)/2 - tickReadNops
)

// tickReads holds the envelopes a tick reads until it keeps them. It is
// not a local: the compiler would zero one before each tick, 48 stores.
var tickReads [spuVoices]uint16

// soundTick runs every vertical blank once sound has started, from the
// library's interrupt handler, after the kernel's handlers: it reads the
// voices' envelopes, takes in stops, writes the voices' changed registers,
// keeps the envelopes it read for the allocation of voices until the next
// tick, writes the key masks, and then runs the sequences, whose writes wait
// for the next tick (spec 05, "Where the sequences run").
func soundTick() {
	pace(tickReadsPad)
	space(2)
	nops(10)
	irqStamp(5)
	reads := &tickReads
	for v := range snd.voices {
		reads[v] = spuGet(uint32(v*16 + spuEnv))
		nops(tickReadNops)
		loadSpace(tickReadLoads)
	}
	irqStamp(6)
	if snd.stopAsked {
		stopSequences()
	}
	written := uint32(0)
	for v := range snd.voices {
		sv := &snd.voices[v]
		if sv.dirty == 0 {
			continue
		}
		written++
		for r := range voiceRegs {
			if sv.dirty&(1<<r) != 0 {
				spuSet(uint32(v*16+r*2), sv.regs[r])
			}
		}
		sv.dirty = 0
	}
	irqStamp(7)
	settled, live := uint32(0), uint32(0)
	for v := range snd.voices {
		sv := &snd.voices[v]
		e := reads[v]
		sv.env = e
		sv.fresh = false
		tickVoiceTime(sv, e, &settled, &live)
		if sv.keyed {
			if e != 0 {
				sv.zeros = 0
			} else if sv.zeros < silentReads {
				sv.zeros++
				if sv.zeros == silentReads && v == int(snd.lastNoise) {
					noiseOff()
					snd.lastNoise = -1
				}
			}
		}
	}
	tickWordsTime(settled, written, live)
	irqStamp(8)
	spuSetMask(spuKeyOff, snd.keyOff)
	spuSetMask(spuKeyOn, snd.keyOn)
	spuSetMask(spuReverb, snd.reverb)
	snd.keyOn, snd.keyOff = 0, 0
	for i, open := 0, snd.seqsOpen; open != 0; i, open = i+1, open>>1 {
		if open&1 != 0 {
			seqs[i].tick()
		}
	}
	irqStamp(9)
	tickExitPace()
	irqStamp(10)
}

// silentReads is how many ticks in a row must read a keyed on voice's
// envelope as 0 for it to be free: a sound that has ended by itself.
const silentReads = 16

// free reports whether voice v is free to take: keyed off with its
// envelope read as 0, or keyed on and read as 0 silentReads times in a
// row.
func (sv *sndVoice) free() bool {
	if sv.fresh {
		return false
	}
	if sv.keyed {
		return sv.zeros >= silentReads
	}
	return sv.env == 0
}

// allocVoice chooses a voice for a tone of priority prior: the lowest
// free one, or else one stolen (see stealVoice). It returns -1 if there
// is none. It stays out of line: inlined into the vertical blank
// handler, as LLVM 23 does, it makes that too big for its place in the
// resident image.
//
//go:noinline
func allocVoice(prior uint8, pending bool) int {
	v := -1
	for i := range snd.voices {
		if snd.voices[i].free() {
			v = i
			break
		}
	}
	if v < 0 {
		v = stealVoice(prior, pending)
	}
	if v >= 0 && v == int(snd.lastNoise) {
		// The last noise voice is taken: a steal of it stops the noise
		// first. Keyed off already, it has stopped its noise or been told
		// to be quiet, and writes nothing.
		if snd.voices[v].keyed {
			noiseOff()
		}
		snd.lastNoise = -1
	}
	return v
}

// stealVoice chooses a voice for a tone of priority prior from those
// whose tone's priority is at most prior: the lowest priority, then the
// lowest envelope, then the oldest, then the lowest number. Voices keyed
// on since the last tick are candidates only if pending is set (SsUtKeyOn's
// steals take them, a sequence's do not, even when keyed off again). It
// returns -1 if there is none.
func stealVoice(prior uint8, pending bool) int {
	best := -1
	for v := range snd.voices {
		sv := &snd.voices[v]
		if sv.prior > prior || !pending && sv.fresh {
			continue
		}
		if best >= 0 {
			b := &snd.voices[best]
			if sv.prior != b.prior {
				if sv.prior > b.prior {
					continue
				}
			} else if sv.env != b.env {
				if sv.env > b.env {
					continue
				}
			} else if sv.age >= b.age {
				continue
			}
		}
		best = v
	}
	return best
}

// keyOn starts voice v at the next tick: it is taken, and the newest
// voice.
func keyOn(v int) {
	sv := &snd.voices[v]
	sv.keyed, sv.fresh = true, true
	sv.zeros = 0
	snd.ons++
	sv.age = snd.ons
	snd.keyOn |= 1 << v
	snd.keyOff &^= 1 << v
}

// keyOff releases voice v at the next tick.
func keyOff(v int) {
	snd.voices[v].keyed = false
	snd.keyOff |= 1 << v
	snd.keyOn &^= 1 << v
}

// noteKeyOff releases voice v for a sequence's note off or SsUtKeyOff: a
// sample voice's key off bit at the next tick, or for a noise voice the noise
// off at once and no key off bit, so that a key on still staged is written.
func noteKeyOff(v int) {
	sv := &snd.voices[v]
	if !sv.noise {
		keyOff(v)
		return
	}
	noiseOff()
	sv.keyed = false
	if v == int(snd.lastNoise) {
		snd.lastNoise = -1
	}
}

// noiseOn makes voice v the SPU's noise voice at once: SPUCNT's clock is
// the note's distance from the tone's center, the other bits kept, and the
// noise mask is written whole, with the voice's bit alone, so that only the
// newest noise voice is noise. Nothing else sets the clock or takes it back.
func noiseOn(v, note int, center uint8) {
	clock := uint16(note-int(center)) & 0x3f
	spuSet(spuCtrl, spuGet(spuCtrl)&0xc0ff|clock<<8)
	spuSetMask(spuNoise, 1<<v)
	snd.lastNoise = int8(v)
}

// noiseOff clears the noise mask at once, whichever voice's bit is there.
func noiseOff() {
	spuSetMask(spuNoise, 0)
}

// setReg sets a voice's register for the next tick to write.
func (sv *sndVoice) setReg(r int, v uint16) {
	sv.regs[r] = v
	sv.dirty |= 1 << r
}

// The pitch table: 4096·2^(i/192) rounded down, an octave in steps of a
// sixteenth of a semitone.
var pitchTable = [192]uint16{
	4096, 4110, 4125, 4140, 4155, 4170, 4185, 4200, 4216, 4231, 4246, 4261, 4277, 4292, 4308, 4323,
	4339, 4355, 4371, 4386, 4402, 4418, 4434, 4450, 4466, 4482, 4499, 4515, 4531, 4548, 4564, 4581,
	4597, 4614, 4630, 4647, 4664, 4681, 4698, 4715, 4732, 4749, 4766, 4783, 4801, 4818, 4835, 4853,
	4870, 4888, 4906, 4924, 4941, 4959, 4977, 4995, 5013, 5031, 5050, 5068, 5086, 5105, 5123, 5142,
	5160, 5179, 5198, 5216, 5235, 5254, 5273, 5292, 5311, 5331, 5350, 5369, 5389, 5408, 5428, 5447,
	5467, 5487, 5507, 5527, 5547, 5567, 5587, 5607, 5627, 5648, 5668, 5688, 5709, 5730, 5750, 5771,
	5792, 5813, 5834, 5855, 5876, 5898, 5919, 5940, 5962, 5983, 6005, 6027, 6049, 6070, 6092, 6114,
	6137, 6159, 6181, 6203, 6226, 6248, 6271, 6294, 6316, 6339, 6362, 6385, 6408, 6431, 6455, 6478,
	6501, 6525, 6549, 6572, 6596, 6620, 6644, 6668, 6692, 6716, 6741, 6765, 6789, 6814, 6839, 6863,
	6888, 6913, 6938, 6963, 6988, 7014, 7039, 7064, 7090, 7116, 7141, 7167, 7193, 7219, 7245, 7271,
	7298, 7324, 7351, 7377, 7404, 7431, 7458, 7485, 7512, 7539, 7566, 7593, 7621, 7648, 7676, 7704,
	7732, 7760, 7788, 7816, 7844, 7873, 7901, 7930, 7958, 7987, 8016, 8045, 8074, 8103, 8133, 8162,
}

// notePitch returns the SPU pitch for note, with fine in 128ths of a
// semitone, on a tone whose sample sounds at center plus shift 128ths.
// The steps are sixteenths of a semitone; the octave shift wraps at 16
// bits, as the register does.
func notePitch(note, fine int, center, shift uint8) uint16 {
	return stepPitch((note-int(center)+60)*16 + (fine+int(shift))>>3)
}

// stepPitch returns the SPU pitch k sixteenths of a semitone above five
// octaves below the sample's own pitch. An octave shift above 31 wraps
// round, as the CPU's shift does (only its low five bits count). A
// negative k (a note far below the tone's center: terra's effects keyed
// on from an overwritten header are 14 octaves down) reads off Sony's
// table, which the spec leaves open: the step is wrapped back into the
// table and the shift is the formula's, 0 at those depths.
func stepPitch(k int) uint16 {
	i, oct := k%192, k/192-5
	if i < 0 {
		i += 192
	}
	p := uint32(pitchTable[i])
	if oct >= 0 {
		return uint16(p << (oct & 31))
	}
	return uint16(p >> -oct)
}

// panLeft and panRight are how much of a voice goes to each side at pan,
// 0 to 127 with 64 the center, in 63rds. A pan past either end (a tone
// read from an overwritten header) gives the far side nothing, where the
// formula alone would go negative.
func panLeft(pan int32) int32  { return max(min(127-pan, 63), 0) }
func panRight(pan int32) int32 { return max(min(pan, 63), 0) }

// volumeReg squares x, a volume from 0 to 3FFFh, into the register's
// scale.
func volumeReg(x int32) uint16 { return uint16(x * x / 0x3fff) }

// SsSetMVol sets the main volume, left and right, from 0 to 127.
//
//export SsSetMVol
func SsSetMVol(l, r int16) {
	soundStart()
	spuSet(spuMainL, uint16(int32(l)*129)&0x7fff)
	spuSet(spuMainR, uint16(int32(r)*129)&0x7fff)
	pace(ssSetMVolPad)
}

// ssSetMVolPad is the rest of SsSetMVol's 178 cycles (spec 21).
const ssSetMVolPad = 110

// SsSetSerialAttr turns the CD's audio into the mix (SS_MIX) or the
// reverb (SS_REV) on or off.
//
//export SsSetSerialAttr
func SsSetSerialAttr(s, attr, mode uint8) {
	soundStart()
	bit := uint16(1)
	if attr == 1 {
		bit = 4
	}
	c := spuGet(spuCtrl)
	if mode != 0 {
		c |= bit
	} else {
		c &^= bit
	}
	spuSet(spuCtrl, c)
}

// SsSetSerialVol sets the CD audio's volume, left and right, from 0 to
// 127.
//
//export SsSetSerialVol
func SsSetSerialVol(s uint8, l, r int16) {
	soundStart()
	spuSet(spuCDL, serialVolume(l))
	spuSet(spuCDR, serialVolume(r))
}

// serialVolume is a CD volume's register value: 258 a step, louder than
// 127 taken as 127.
func serialVolume(v int16) uint16 {
	return uint16(int32(min(v, 127)) * 258)
}

// SndVolume is a stereo volume, as SsGetMVol and SsGetSerialVol fill it.
type SndVolume struct {
	Left, Right uint16
}

// spuUnmute is SPUCNT's unmute bit: clear, the SPU's output is silent.
const spuUnmute = 1 << 14

// SsGetMVol reads the main volume back from the SPU's registers: each
// register's 15-bit volume, signed, over 129, rounded toward zero, so that
// it gives what SsSetMVol was given for 0 to 127, and what the register
// wrapped to for others.
//
//export SsGetMVol
func SsGetMVol(v *SndVolume) {
	soundStart()
	v.Left = uint16(mainVolume(spuGet(spuMainL)))
	v.Right = uint16(mainVolume(spuGet(spuMainR)))
}

func mainVolume(r uint16) int32 {
	x := int32(r & 0x7fff)
	if x&0x4000 != 0 {
		x -= 0x8000
	}
	return x / 129
}

// SsSetMute mutes the SPU's output (SS_MUTE_ON, 1) or unmutes it
// (SS_MUTE_OFF, 0) at once, with SPUCNT's bit 14; any other mode does
// nothing. Voices go on being written while muted.
//
//export SsSetMute
func SsSetMute(mode uint8) {
	soundStart()
	switch mode {
	case 1:
		spuSet(spuCtrl, spuGet(spuCtrl)&^spuUnmute)
	case 0:
		spuSet(spuCtrl, spuGet(spuCtrl)|spuUnmute)
	}
}

// SsGetMute returns 1 while the SPU's output is muted, 0 if not.
//
//export SsGetMute
func SsGetMute() uint8 {
	soundStart()
	if spuGet(spuCtrl)&spuUnmute == 0 {
		return 1
	}
	return 0
}

// SsGetSerialAttr returns 1 if the CD audio's attribute attr, SS_MIX (0)
// or SS_REV (1), is on in SPUCNT, and 0 if not. With any other attribute, or
// a part s above 1, it returns 1 (spec 23).
//
//export SsGetSerialAttr
func SsGetSerialAttr(s, attr uint8) uint8 {
	soundStart()
	if s > 1 || attr > 1 {
		return 1
	}
	bit := uint16(1)
	if attr == 1 {
		bit = 4
	}
	if spuGet(spuCtrl)&bit != 0 {
		return 1
	}
	return 0
}

// SsGetSerialVol reads the CD audio's volume back from the SPU's
// registers: each, signed, over 258.
//
//export SsGetSerialVol
func SsGetSerialVol(s uint8, v *SndVolume) {
	soundStart()
	v.Left = uint16(int32(int16(spuGet(spuCDL))) / 258)
	v.Right = uint16(int32(int16(spuGet(spuCDR))) / 258)
}
