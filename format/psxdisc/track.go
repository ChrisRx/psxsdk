package psxdisc

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/iso9660"
)

const (
	// AudioSectorSize is a sector of CD audio: 2352 bytes of samples and
	// nothing else. An audio track has no sync pattern, header or error
	// correction, which is why the whole sector is payload.
	AudioSectorSize = cdrom.SectorSize

	// AudioSampleRate, AudioChannels and AudioBitsPerSample are the only
	// sample format a CD holds. Anything else has to be converted before it
	// can be written to a track.
	AudioSampleRate     = 44100
	AudioChannels       = 2
	AudioBitsPerSample  = 16
	audioBytesPerSample = AudioChannels * AudioBitsPerSample / 8

	// StandardPregap is the two second pause the Red Book requires between
	// a data track and the audio that follows it, which Write puts before
	// every audio track. A drive uses it to settle
	// after the mode change, and a player that starts reading at the track
	// boundary finds silence rather than the tail of the data track.
	StandardPregap = 2 * 75
)

// An AudioTrack is a CD audio track to be recorded after the data track.
//
// The contents are 44.1 kHz 16-bit stereo samples, little-endian, which is
// what a CD holds and also what a WAV file of the same format holds, so no
// conversion happens here beyond padding the last sector with silence.
type AudioTrack struct {
	// Name identifies the track in errors, usually the file it came from.
	Name string

	// Open returns the samples.
	Open func() (io.ReadCloser, error)

	// Size is the number of bytes Open returns.
	Size int64
}

// sectors is how many sectors the track occupies, with the last one padded
// with silence.
func (a *AudioTrack) sectors() uint32 {
	return uint32((a.Size + AudioSectorSize - 1) / AudioSectorSize)
}

// AudioTrackFile reads an audio track from a file, which may be a WAV file or
// headerless samples in the format a CD takes.
func AudioTrackFile(path string) (*AudioTrack, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// A WAV file is samples behind a header, so the track is the same file
	// read from where its data chunk begins.
	offset, size, err := wavData(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if size%audioBytesPerSample != 0 {
		return nil, fmt.Errorf("%s: %d bytes of samples is not a whole number of %d-byte stereo frames",
			path, size, audioBytesPerSample)
	}

	return &AudioTrack{
		Name: filepath.Base(path),
		Size: size,
		Open: func() (io.ReadCloser, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			if offset > 0 {
				if _, err := f.Seek(offset, io.SeekStart); err != nil {
					f.Close()
					return nil, err
				}
			}
			return f, nil
		},
	}, nil
}

// wavData locates the samples in an audio file. For a WAV file it returns the
// offset and length of the data chunk, having checked that the samples are in
// the one format a CD can hold. For anything else it returns the whole file,
// on the assumption that a file which is not a WAV is already bare samples.
func wavData(r io.ReaderAt, size int64) (offset, length int64, err error) {
	var riff [12]byte
	if _, err := r.ReadAt(riff[:], 0); err != nil {
		if size < int64(len(riff)) {
			// Too short to be a WAV, so it can only be samples.
			return 0, size, nil
		}
		return 0, 0, err
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0, size, nil
	}

	var haveFormat bool
	// Chunks follow the RIFF header, each an eight byte header and a body
	// padded to an even length.
	for pos := int64(12); pos+8 <= size; {
		var hdr [8]byte
		if _, err := r.ReadAt(hdr[:], pos); err != nil {
			return 0, 0, err
		}
		id := string(hdr[0:4])
		n := int64(binary.LittleEndian.Uint32(hdr[4:]))
		body := pos + 8
		if n < 0 || body+n > size {
			return 0, 0, fmt.Errorf("%s chunk runs past the end of the file", id)
		}

		switch id {
		case "fmt ":
			if n < 16 {
				return 0, 0, fmt.Errorf("fmt chunk is %d bytes, too short to describe the samples", n)
			}
			var f [16]byte
			if _, err := r.ReadAt(f[:], body); err != nil {
				return 0, 0, err
			}
			format := binary.LittleEndian.Uint16(f[0:])
			channels := binary.LittleEndian.Uint16(f[2:])
			rate := binary.LittleEndian.Uint32(f[4:])
			bits := binary.LittleEndian.Uint16(f[14:])
			// 1 is uncompressed PCM, the only thing that can be copied to
			// a track as it stands.
			if format != 1 {
				return 0, 0, fmt.Errorf("samples are in format %d, not uncompressed PCM", format)
			}
			if channels != AudioChannels || rate != AudioSampleRate || bits != AudioBitsPerSample {
				return 0, 0, fmt.Errorf("samples are %d channel %d Hz %d bit, but a CD track is %d channel %d Hz %d bit",
					channels, rate, bits, AudioChannels, AudioSampleRate, AudioBitsPerSample)
			}
			haveFormat = true
		case "data":
			if !haveFormat {
				return 0, 0, fmt.Errorf("data chunk comes before the format is described")
			}
			return body, n, nil
		}

		pos = body + n
		if n%2 != 0 {
			pos++
		}
	}
	return 0, 0, fmt.Errorf("no data chunk, so the file holds no samples")
}

// An Image is a disc as a whole: the data track holding the filesystem, and
// any audio tracks recorded after it.
//
// Audio is why a PlayStation disc image is a .bin and a .cue rather than a
// .iso. The filesystem is only the first track, and nothing in it describes
// the rest of the disc; a program plays a music track by its number, and the
// cue sheet is what tells an emulator where the numbered tracks begin.
type Image struct {
	// Data is the first track: the laid out filesystem.
	Data *iso9660.Writer

	// Audio are the CD audio tracks to follow it, in order.
	Audio []*AudioTrack
}

// A Track is where one track ended up in a written image.
type Track struct {
	// Number is the track number a cue sheet and a program both use, from
	// one.
	Number int

	// Audio distinguishes a CD audio track from the data track.
	Audio bool

	// Name is the audio track's name, and empty for the data track.
	Name string

	// Pregap is the first sector of the pause before the track, and equals
	// Start when there is none.
	Pregap uint32

	// Start is the first sector of the track's own contents.
	Start uint32

	// Sectors is the length of the contents, not counting the pregap.
	Sectors uint32
}

// A Layout is where every track of a written image begins. It is what a cue
// sheet is made of, and is only known once the tracks have been laid down.
type Layout struct {
	Tracks []Track
}

// Sectors is the length of the whole image.
func (l Layout) Sectors() uint32 {
	if len(l.Tracks) == 0 {
		return 0
	}
	last := l.Tracks[len(l.Tracks)-1]
	return last.Start + last.Sectors
}

// Write writes the image and returns where its tracks ended up.
func (img *Image) Write(w io.Writer) (Layout, error) {
	var l Layout
	if img.Data == nil {
		return l, fmt.Errorf("image has no data track")
	}

	if err := img.Data.WriteTo(NewRawWriter(w)); err != nil {
		return l, err
	}
	l.Tracks = append(l.Tracks, Track{
		Number:  1,
		Pregap:  0,
		Start:   0,
		Sectors: img.Data.TotalBlocks(),
	})
	pos := img.Data.TotalBlocks()

	for i, a := range img.Audio {
		// A pause goes before every audio track, as on most CDs. The one
		// before the first is the Red Book's; the rest are for the library,
		// whose CdPlay starts a track at the time GetTD gives for it, which
		// is to the second, and so up to a second early. In a gap that is
		// silence the drive counts as the track's own; without one it is
		// the end of the track before, and with AutoPause on, as CdPlay
		// has it, the drive stops at the change of track a moment later
		// and the library moves on to the next.
		pregap := uint32(StandardPregap)
		if err := writeSilence(w, pregap); err != nil {
			return l, fmt.Errorf("writing the pause before %s: %w", a.Name, err)
		}

		start := pos + pregap
		n, err := writeAudio(w, a)
		if err != nil {
			return l, fmt.Errorf("writing %s: %w", a.Name, err)
		}
		l.Tracks = append(l.Tracks, Track{
			Number:  i + 2,
			Audio:   true,
			Name:    a.Name,
			Pregap:  pos,
			Start:   start,
			Sectors: n,
		})
		pos = start + n
	}
	return l, nil
}

// writeSilence writes n sectors of digital silence, which on a CD is simply
// zero samples.
func writeSilence(w io.Writer, n uint32) error {
	sector := make([]byte, AudioSectorSize)
	for range n {
		if _, err := w.Write(sector); err != nil {
			return err
		}
	}
	return nil
}

// writeAudio copies a track's samples into whole sectors, padding the last
// one with silence.
func writeAudio(w io.Writer, a *AudioTrack) (uint32, error) {
	rc, err := a.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	sector := make([]byte, AudioSectorSize)
	total := a.sectors()
	for range total {
		clear(sector)
		if _, err := io.ReadFull(rc, sector); err != nil &&
			err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, err
		}
		if _, err := w.Write(sector); err != nil {
			return 0, err
		}
	}
	return total, nil
}
