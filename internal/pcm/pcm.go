// Package pcm reads uncompressed sound files, WAV and AIFF, into 16-bit
// mono samples, with the loop the file marks, for the tools that turn them
// into the SPU's ADPCM.
package pcm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// A Sound is a file's samples, mixed down to one channel.
type Sound struct {
	Rate    int
	Samples []int16

	// Loop is the range of samples the file says to repeat, the end
	// exclusive, or nil. WAV files mark it with a smpl chunk, AIFF files
	// with the sustain loop of an INST chunk.
	Loop *[2]int
}

// Read reads a WAV or AIFF file, telling them apart by their first bytes.
func Read(b []byte) (*Sound, error) {
	switch {
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE":
		return readWAV(b)
	case len(b) >= 12 && string(b[:4]) == "FORM" && (string(b[8:12]) == "AIFF" || string(b[8:12]) == "AIFC"):
		return readAIFF(b)
	}
	return nil, errors.New("pcm: not a WAV or AIFF file")
}

// Raw reads headerless 16-bit samples, little-endian unless big is set, as
// Sony's aiff2vag took them with -R.
func Raw(b []byte, rate int, big bool) *Sound {
	s := &Sound{Rate: rate, Samples: make([]int16, len(b)/2)}
	order := binary.ByteOrder(binary.LittleEndian)
	if big {
		order = binary.BigEndian
	}
	for i := range s.Samples {
		s.Samples[i] = int16(order.Uint16(b[2*i:]))
	}
	return s
}

type chunk struct {
	id   string
	data []byte
}

// chunks splits the body of a RIFF or IFF file into its chunks, each padded
// to an even length.
func chunks(b []byte, order binary.ByteOrder) []chunk {
	var out []chunk
	for len(b) >= 8 {
		n := int(order.Uint32(b[4:]))
		if n > len(b)-8 {
			n = len(b) - 8
		}
		out = append(out, chunk{string(b[:4]), b[8 : 8+n]})
		b = b[8+n:]
		if n%2 == 1 && len(b) > 0 {
			b = b[1:]
		}
	}
	return out
}

func readWAV(b []byte) (*Sound, error) {
	var format, channels, bits int
	s := &Sound{}
	var data []byte
	for _, c := range chunks(b[12:], binary.LittleEndian) {
		switch c.id {
		case "fmt ":
			if len(c.data) < 16 {
				return nil, errors.New("pcm: short fmt chunk")
			}
			format = int(binary.LittleEndian.Uint16(c.data))
			channels = int(binary.LittleEndian.Uint16(c.data[2:]))
			s.Rate = int(binary.LittleEndian.Uint32(c.data[4:]))
			bits = int(binary.LittleEndian.Uint16(c.data[14:]))
			if format == 0xfffe && len(c.data) >= 26 {
				format = int(binary.LittleEndian.Uint16(c.data[24:]))
			}
		case "data":
			data = c.data
		case "smpl":
			if len(c.data) >= 36+24 && binary.LittleEndian.Uint32(c.data[28:]) > 0 {
				start := int(binary.LittleEndian.Uint32(c.data[36+8:]))
				end := int(binary.LittleEndian.Uint32(c.data[36+12:]))
				s.Loop = &[2]int{start, end + 1}
			}
		}
	}
	if format != 1 {
		return nil, fmt.Errorf("pcm: WAV format %d is not PCM", format)
	}
	if data == nil {
		return nil, errors.New("pcm: no data chunk")
	}
	var err error
	s.Samples, err = mix(data, channels, bits, binary.LittleEndian, bits == 8)
	return s, err
}

func readAIFF(b []byte) (*Sound, error) {
	var channels, bits int
	s := &Sound{}
	var data []byte
	markers := map[uint16]int{}
	var loop *[2]uint16
	compressed := string(b[8:12]) == "AIFC"
	for _, c := range chunks(b[12:], binary.BigEndian) {
		switch c.id {
		case "COMM":
			if len(c.data) < 18 {
				return nil, errors.New("pcm: short COMM chunk")
			}
			channels = int(binary.BigEndian.Uint16(c.data))
			bits = int(binary.BigEndian.Uint16(c.data[6:]))
			s.Rate = int(math.Round(extended(c.data[8:18])))
			if compressed && len(c.data) >= 22 && string(c.data[18:22]) != "NONE" {
				return nil, fmt.Errorf("pcm: AIFC compression %q is not supported", c.data[18:22])
			}
		case "SSND":
			if len(c.data) < 8 {
				return nil, errors.New("pcm: short SSND chunk")
			}
			offset := int(binary.BigEndian.Uint32(c.data))
			data = c.data[min(8+offset, len(c.data)):]
		case "MARK":
			if len(c.data) < 2 {
				break
			}
			n := int(binary.BigEndian.Uint16(c.data))
			r := c.data[2:]
			for range n {
				if len(r) < 7 {
					break
				}
				id := binary.BigEndian.Uint16(r)
				pos := int(binary.BigEndian.Uint32(r[2:]))
				markers[id] = pos
				name := int(r[6])
				skip := 7 + name
				if skip%2 == 1 {
					skip++
				}
				r = r[min(skip, len(r)):]
			}
		case "INST":
			// The sustain loop: play mode, begin marker, end marker.
			if len(c.data) >= 14 && binary.BigEndian.Uint16(c.data[8:]) != 0 {
				loop = &[2]uint16{binary.BigEndian.Uint16(c.data[10:]), binary.BigEndian.Uint16(c.data[12:])}
			}
		}
	}
	if data == nil {
		return nil, errors.New("pcm: no SSND chunk")
	}
	if loop != nil {
		start, ok1 := markers[loop[0]]
		end, ok2 := markers[loop[1]]
		if ok1 && ok2 && end > start {
			s.Loop = &[2]int{start, end}
		}
	}
	var err error
	s.Samples, err = mix(data, channels, bits, binary.BigEndian, false)
	return s, err
}

// mix reads interleaved samples of 8 or 16 bits and averages the channels.
func mix(data []byte, channels, bits int, order binary.ByteOrder, unsigned8 bool) ([]int16, error) {
	if channels < 1 {
		return nil, errors.New("pcm: no channels")
	}
	if bits != 8 && bits != 16 {
		return nil, fmt.Errorf("pcm: %d-bit samples are not supported, only 8 and 16", bits)
	}
	size := bits / 8
	frames := len(data) / (size * channels)
	out := make([]int16, frames)
	for i := range out {
		sum := 0
		for c := range channels {
			p := data[(i*channels+c)*size:]
			var v int
			switch {
			case bits == 16:
				v = int(int16(order.Uint16(p)))
			case unsigned8:
				v = (int(p[0]) - 128) << 8
			default:
				v = int(int8(p[0])) << 8
			}
			sum += v
		}
		out[i] = int16(sum / channels)
	}
	return out, nil
}

// extended decodes the 80-bit IEEE 754 extended float an AIFF file gives
// its sample rate in.
func extended(b []byte) float64 {
	exp := int(binary.BigEndian.Uint16(b) & 0x7fff)
	mant := binary.BigEndian.Uint64(b[2:])
	if exp == 0 && mant == 0 {
		return 0
	}
	v := math.Ldexp(float64(mant), exp-16383-63)
	if b[0]&0x80 != 0 {
		v = -v
	}
	return v
}

// WAV returns s as a 16-bit mono WAV file.
func (s *Sound) WAV() []byte {
	var b bytes.Buffer
	n := 2 * len(s.Samples)
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + n))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(s.Rate))
	w(uint32(2 * s.Rate))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(uint32(n))
	w(s.Samples)
	return b.Bytes()
}
