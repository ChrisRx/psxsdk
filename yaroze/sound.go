package yaroze

// The sequencer: a sound bank sent to the sound processor, and
// sequences, pieces of music, played on it. A Bank or a Sequence that
// is the zero value, or that failed to open, does nothing, so a program
// whose music did not load can carry on without it.

var (
	// ErrBank is returned when the sound processor does not accept a
	// bank: its header is not a VAB's, or its waveforms do not fit.
	ErrBank error = apiError("yaroze: the sound bank was not accepted")

	// ErrSequence is returned when a sequence does not open: it is not
	// a SEQ, or the library's sixteen are open.
	ErrSequence error = apiError("yaroze: the sequence did not open")

	// ErrEmpty is returned for a file with nothing in it, a disc file
	// that was not read.
	ErrEmpty error = apiError("yaroze: the file is empty")
)

// SetVolume sets the main volume, left and right, 0 to 127. It starts
// at full, 127.
func SetVolume(left, right int) { mainVolume(left, right) }

// A Reverb is one of the sound processor's reverbs.
type Reverb int

const (
	ReverbRoom    Reverb = 1
	ReverbStudioA Reverb = 2
	ReverbStudioB Reverb = 3
	ReverbStudioC Reverb = 4
	ReverbHall    Reverb = 5
	ReverbSpace   Reverb = 6
	ReverbEcho    Reverb = 7
	ReverbDelay   Reverb = 8
	ReverbPipe    Reverb = 9
)

// SetReverb turns the reverb on, of the kind given, at a depth left and
// right, 0 to 127.
func SetReverb(kind Reverb, left, right int) { setReverb(kind, left, right) }

// ReverbOff turns the reverb off.
func ReverbOff() { reverbOff() }

// A Bank is a sound bank open on the sound processor: the instruments
// sequences play with. It holds the library's id plus one, so that the
// zero value is no bank.
type Bank struct {
	h int16
}

// OpenBank sends a bank to the sound processor: its header, the VH file
// in vh, and its waveforms, the VB file in vb, each a file's words as
// package disc reads them.
func OpenBank(vh, vb []uint32) (Bank, error) {
	if len(vh) == 0 || len(vb) == 0 {
		return Bank{}, ErrEmpty
	}
	id := vabTransfer(vh, vb)
	if id < 0 {
		return Bank{}, ErrBank
	}
	return Bank{id + 1}, nil
}

// ID is the library's id for the bank, for the Ss calls that take one,
// or -1 for no bank.
func (b Bank) ID() int16 { return b.h - 1 }

// Close takes the bank off the sound processor.
func (b Bank) Close() {
	if b.h > 0 {
		vabClose(b.h - 1)
	}
}

// Sequence opens the SEQ file in seq to play on the bank.
func (b Bank) Sequence(seq []uint32) (Sequence, error) {
	if b.h <= 0 {
		return Sequence{}, ErrBank
	}
	if len(seq) == 0 {
		return Sequence{}, ErrEmpty
	}
	id := seqOpen(seq, b.h-1)
	if id < 0 {
		return Sequence{}, ErrSequence
	}
	return Sequence{id + 1}, nil
}

// A Sequence is a piece of music open for playing, the library's id
// plus one.
type Sequence struct {
	h int16
}

// ID is the library's id for the sequence, for the Ss calls that take
// one, or -1 for no sequence.
func (q Sequence) ID() int16 { return q.h - 1 }

// Play plays the sequence times times, or over and over for
// SSPLAY_INFINITY, which is 0, as Loop does.
func (q Sequence) Play(times int) {
	if q.h > 0 {
		seqPlay(q.h-1, int16(times))
	}
}

// Loop plays the sequence over and over.
func (q Sequence) Loop() {
	if q.h > 0 {
		seqPlay(q.h-1, 0)
	}
}

// SetVolume sets the sequence's volume, left and right, 0 to 127.
func (q Sequence) SetVolume(left, right int) {
	if q.h > 0 {
		seqVolume(q.h-1, left, right)
	}
}

// Pause stops the sequence where it is, for Resume.
func (q Sequence) Pause() {
	if q.h > 0 {
		seqPause(q.h - 1)
	}
}

// Resume goes on from where Pause stopped.
func (q Sequence) Resume() {
	if q.h > 0 {
		seqResume(q.h - 1)
	}
}

// Stop stops the sequence.
func (q Sequence) Stop() {
	if q.h > 0 {
		seqStop(q.h - 1)
	}
}

// Playing says whether the sequence is still playing.
func (q Sequence) Playing() bool {
	return q.h > 0 && seqPlaying(q.h-1)
}

// Close closes the sequence. One that is playing is stopped first, and
// the library wants a frame or two between, so a program does that
// itself.
func (q Sequence) Close() {
	if q.h > 0 {
		seqClose(q.h - 1)
	}
}

// An apiError is one of the Go API's errors, a message as errors.New
// makes one, kept from the errors package, whose initialization would
// otherwise come with every program that imports this one.
type apiError string

func (e apiError) Error() string { return string(e) }
