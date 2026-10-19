package main

// Sound: two banks on the SPU, Sony's standard one for the music and the
// game's own for its effects, four tunes, and effects keyed on by number
// from a table of program, volume, note and how much to vary it.

import "go.chrisrx.dev/psx/yaroze"

var (
	musicBank, effectBank yaroze.Bank
	tunes                 [4]yaroze.Sequence

	// tuneState is each tune's state: stopped, playing, or fading out.
	tuneState [4]int

	// ambient, when set, plays the port's sounds, the sea and the gulls,
	// counting the frames in ambientT.
	ambient  bool
	ambientT int
)

const (
	tuneStopped = 0
	tunePlaying = 1
	tuneFading  = 2
)

// Tunes by number: the port's, the ruins', and the game's own two.
const (
	tunePort  = 0
	tuneRuins = 1
	tuneOpen  = 2
	tuneEnd   = 3
)

// An effect is one sound effect: the program in the game's bank, its
// volume, its note, and a range the note and the pan vary by, or 0 for
// none.
type effect struct {
	prog, vol, note, vary int32
}

var effects = [...]effect{
	{0, 96, 60, 0}, {1, 96, 60, 0}, {2, 96, 60, 0}, {3, 96, 65, 0},
	{4, 96, 60, 0}, {5, 96, 60, 0}, {6, 80, 75, 0}, {7, 96, 65, 0},
	{8, 96, 60, 0}, {9, 40, 57, 7}, {10, 40, 50, 10}, {11, 96, 60, 0},
	{12, 96, 60, 0}, {13, 96, 60, 0}, {3, 96, 45, 0}, {7, 96, 70, 0},
	{7, 96, 80, 0}, {13, 96, 30, 0}, {4, 96, 40, 0}, {1, 96, 55, 10},
	{10, 80, 50, 10}, {13, 80, 30, 0},
}

// soundInit sends the banks to the SPU and opens the tunes. The game's
// bank comes from the third archive, unpacked over the second, which has
// given up everything it holds by now, and Sony's from the disc, its body
// read over both archives once the game's bank has gone to the SPU. Sound
// that fails to load leaves the game silent.
func soundInit() {
	seq2, _ := arc3.words(2, 0)
	seq3, _ := arc3.words(2, 1)
	vh := arc3.unpackWords(0, 0, arcBuf[:])
	vb := arc3.unpackWords(1, 0, arcBuf[len(vh):])
	effectBank, _ = yaroze.OpenBank(vh, vb)
	if !soundStd0VB.Load() {
		return
	}
	musicBank, _ = yaroze.OpenBank(soundStd0VH.Data(), soundStd0VB.Data())
	yaroze.SsUtAllKeyOff(0)
	yaroze.SsSetMVol(127, 127)
	yaroze.SsUtSetReverbType(yaroze.SS_REV_TYPE_STUDIO_A)
	yaroze.SsUtSetReverbDepth(0x30, 0x30)
	yaroze.SsUtReverbOn()
	tunes[tunePort], _ = musicBank.Sequence(soundGogoSEQ.Data())
	tunes[tuneRuins], _ = musicBank.Sequence(soundMusiSEQ.Data())
	tunes[tuneOpen], _ = musicBank.Sequence(seq2)
	tunes[tuneEnd], _ = musicBank.Sequence(seq3)
}

// playTune starts tune n from the beginning, at volume 96, stopping it
// first if it is playing.
func playTune(n int) {
	if tuneState[n] != tuneStopped {
		tunes[n].Stop()
	}
	tunes[n].SetVolume(0x60, 0x60)
	tunes[n].Play(1)
	tuneState[n] = tunePlaying
}

// fadeTune starts tune n fading out, if it is playing.
func fadeTune(n int) {
	if tuneState[n] == tunePlaying {
		tuneState[n] = tuneFading
	}
}

// stopTune stops tune n.
func stopTune(n int) {
	if tuneState[n] != tuneStopped {
		tunes[n].Stop()
		tuneState[n] = tuneStopped
	}
}

var tuneVol [2]int16

// soundFrame fades the tunes that are fading, a step a frame until they
// are silent and stopped, and plays the ambient sounds.
func soundFrame() {
	for n := range tunes {
		if tuneState[n] != tuneFading {
			continue
		}
		yaroze.SsSeqGetVol(tunes[n].ID, 0, &tuneVol[0], &tuneVol[1])
		if tuneVol[0] == 0 {
			stopTune(n)
		} else {
			tunes[n].SetVolume(tuneVol[0]-1, tuneVol[1]-1)
		}
	}
	if ambient {
		t := ambientT
		ambientT++
		if t%6 == 0 {
			playEffect(10)
		}
		if t%300 == 0 {
			playEffect(0x14)
		}
		if yaroze.Rand()%60 == 0 {
			playEffect(9)
		}
	}
}

// setAmbient turns the ambient sounds on or off, and silences every voice
// when off.
func setAmbient(on bool) {
	ambientT = 0
	ambient = on
	if !on {
		yaroze.SsUtAllKeyOff(0)
	}
}

// playEffect keys on effect n: at its note and volume, or with the note
// raised by up to its range and the sound panned to one side by as much.
func playEffect(n int) {
	if !effectBank.Valid() {
		return
	}
	e := &effects[n]
	note := e.note
	left, right := e.vol, e.vol
	if e.vary != 0 {
		note += yaroze.Rand() % e.vary
		p := yaroze.Rand() % e.vol
		if yaroze.Rand()&1 != 0 {
			p = -p
		}
		left, right = e.vol+p, e.vol-p
	}
	yaroze.SsUtKeyOn(effectBank.ID, int16(e.prog), 0, int16(note), 0, int16(left), int16(right))
}
