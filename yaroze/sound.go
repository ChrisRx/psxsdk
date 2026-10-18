package yaroze

// The sequencer, as a program uses it: a sound bank sent to the SPU, and
// sequences played on it. The library's functions take small integer
// handles and report failure as a negative one, which these carry along
// with whether they are valid, so that a program whose sound did not load
// can carry on without it.

import "unsafe"

// A Bank is a sound bank, a VAB, open on the SPU: the instruments a
// sequence plays with. OpenBank sends one.
type Bank struct {
	ID int16
}

// OpenBank is SsVabTransfer: it reads the bank's header, the VH file in vh,
// sends its waveforms, the VB file in vb, to the SPU, and returns the bank
// and whether it was accepted. Each is a file's words, as package disc reads
// them. SsSetMVol has to be called before anything plays. A bank that was
// not accepted has a negative ID, and its methods do nothing; nor is one
// with either file empty sent.
func OpenBank(vh, vb []uint32) (Bank, bool) {
	if len(vh) == 0 || len(vb) == 0 {
		return Bank{-1}, false
	}
	id := SsVabTransfer(firstByte(vh), firstByte(vb), -1, 1)
	return Bank{id}, id >= 0
}

// firstByte is the first byte of words, as the library takes a sound bank.
//
//go:inline
func firstByte(words []uint32) *uint8 { return (*uint8)(unsafe.Pointer(&words[0])) }

// Valid reports whether the bank was accepted by the SPU.
func (b Bank) Valid() bool { return b.ID >= 0 }

// Close is SsVabClose on the bank.
func (b Bank) Close() {
	if b.Valid() {
		SsVabClose(b.ID)
	}
}

// Sequence is SsSeqOpen: it opens the sequence, the SEQ file in seq, to
// play on the bank, and returns it and whether it opened. An invalid bank,
// or an empty file, opens nothing.
func (b Bank) Sequence(seq []uint32) (Sequence, bool) {
	if !b.Valid() || len(seq) == 0 {
		return Sequence{-1}, false
	}
	id := SsSeqOpen(&seq[0], b.ID)
	return Sequence{id}, id >= 0
}

// A Sequence is a piece of music open for playing, a SEQ file on a bank.
// One that did not open has a negative ID, and its methods do nothing.
type Sequence struct {
	ID int16
}

// Valid reports whether the sequence opened.
func (s Sequence) Valid() bool { return s.ID >= 0 }

// Play is SsSeqPlay: it plays the sequence times times, or for ever with
// SSPLAY_INFINITY.
func (s Sequence) Play(times int16) {
	if s.Valid() {
		SsSeqPlay(s.ID, SSPLAY_PLAY, times)
	}
}

// SetVolume is SsSeqSetVol: the sequence's volume, left and right, from 0
// to 127.
func (s Sequence) SetVolume(left, right int16) {
	if s.Valid() {
		SsSeqSetVol(s.ID, left, right)
	}
}

// Pause is SsSeqPause.
func (s Sequence) Pause() {
	if s.Valid() {
		SsSeqPause(s.ID)
	}
}

// Resume is SsSeqReplay: it resumes the sequence from where Pause left it.
func (s Sequence) Resume() {
	if s.Valid() {
		SsSeqReplay(s.ID)
	}
}

// Stop is SsSeqStop.
func (s Sequence) Stop() {
	if s.Valid() {
		SsSeqStop(s.ID)
	}
}

// Playing is SsIsEos: whether the sequence is still playing.
func (s Sequence) Playing() bool {
	return s.Valid() && SsIsEos(s.ID, 0) != 0
}

// Close is SsSeqClose. A sequence is stopped before it is closed, and the
// library wants a frame or two between, so a program does that itself.
func (s Sequence) Close() {
	if s.Valid() {
		SsSeqClose(s.ID)
	}
}
