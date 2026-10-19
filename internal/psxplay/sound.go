package psxplay

import (
	"encoding/binary"
	"sync"
)

// The sound buffer sits between the emulator, which makes a frame's worth
// of samples a tick on its own clock, and the audio player, which reads
// them on its own. Short of samples, it plays silence until a few frames'
// worth have arrived again; with too many, it drops the oldest, which keeps
// the delay from growing. The site's player does the same in its worklet.
const (
	soundCapacity = sampleRate * 4      // a second, in bytes of stereo 16-bit
	soundStart    = sampleRate * 4 / 10 // buffered before playing (100ms)
	soundMax      = sampleRate * 4 / 4  // buffered at most (250ms)
	soundFrame    = sampleRate * 4 / 60 // about a frame's worth
)

// soundBuffer is a ring of interleaved 16-bit little-endian stereo samples,
// as the audio player reads them and Machine.Audio makes them.
type soundBuffer struct {
	mu      sync.Mutex
	ring    []byte
	read    int // where the next byte is read
	count   int // bytes buffered
	playing bool
}

func newSoundBuffer() *soundBuffer {
	return &soundBuffer{ring: make([]byte, soundCapacity)}
}

// write appends samples, dropping the oldest past soundMax.
func (s *soundBuffer) write(samples []int16) {
	if len(samples) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := (s.read + s.count) % len(s.ring)
	for _, v := range samples {
		binary.LittleEndian.PutUint16(s.ring[w:], uint16(v))
		w = (w + 2) % len(s.ring)
	}
	s.count = min(s.count+2*len(samples), len(s.ring))
	// Too far ahead, skip to the newest soundStart bytes.
	if s.count > soundMax {
		s.count = soundStart
	}
	s.read = (w - s.count + len(s.ring)) % len(s.ring)
	if !s.playing && s.count >= soundStart {
		s.playing = true
	}
}

// buffered is how many bytes are waiting to be played.
func (s *soundBuffer) buffered() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// Read fills p with what is buffered, and silence for the rest.
func (s *soundBuffer) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	if s.playing {
		n = min(len(p), s.count)
		// The samples are whole: a read never splits one.
		n &^= 3
		for i := 0; i < n; i++ {
			p[i] = s.ring[(s.read+i)%len(s.ring)]
		}
		s.read = (s.read + n) % len(s.ring)
		s.count -= n
		if n < len(p) {
			s.playing = false
		}
	}
	clear(p[n:])
	return len(p), nil
}
