// Package vag encodes sound into the SPU's ADPCM and reads and writes VAG
// files, Sony's format for one sound in it.
//
// The SPU plays blocks of 16 bytes, each 28 samples: a byte of shift (low
// nibble) and filter (high nibble), a byte of flags, and 28 four-bit
// residuals. A sample is its residual shifted left 12 and right by the
// shift, plus a prediction from the two samples before it by the filter:
//
//	filter  0    1       2         3         4
//	old     0    60/64   115/64    98/64     122/64
//	older   0    0       -52/64    -55/64    -60/64
//
// The flags say where a voice goes next: bit 0 ends the block's run, and
// with bit 1 clear also mutes the voice; bit 1 repeats from the loop start;
// bit 2 marks this block as the loop start.
//
// A VAG file is a header of 48 big-endian bytes and then the blocks:
//
//	000h  "VAGp"
//	004h  version, 0x20
//	008h  reserved
//	00Ch  size of the blocks in bytes
//	010h  sample rate in Hz
//	014h  12 bytes reserved
//	020h  name, 16 bytes
//
// Sony's aiff2vag began the blocks with one of silence, so that a voice
// keyed on starts from a clean history. A sound that does not loop ends with
// flag 1 on its last block and then a block of silence with flags 7, which
// loops on itself. A sound that loops has flag 2 on every block, 6 on the
// one the loop starts at, and 3 on its last.
package vag

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Magic starts a VAG file.
const Magic = "VAGp"

// HeaderSize is the size of a VAG file's header.
const HeaderSize = 48

// Version is the version Sony's tools wrote.
const Version = 0x20

// BlockSize is the size of a block, and BlockSamples the samples it holds.
const (
	BlockSize    = 16
	BlockSamples = 28
)

// Block flags.
const (
	FlagEnd    = 1 // the block ends a run: jump to the loop start, or mute
	FlagRepeat = 2 // with FlagEnd, keep playing from the loop start
	FlagStart  = 4 // the loop starts at this block
)

// A VAG is a sound for the SPU: its blocks and the rate to play them at.
type VAG struct {
	Rate int
	Name string
	Data []byte // the blocks, a multiple of BlockSize
}

// Decode reads a VAG file.
func Decode(b []byte) (*VAG, error) {
	if len(b) < HeaderSize || string(b[:4]) != Magic {
		return nil, errors.New("vag: not a VAG file")
	}
	size := int(binary.BigEndian.Uint32(b[12:]))
	if size > len(b)-HeaderSize {
		size = len(b) - HeaderSize
	}
	name := b[32:48]
	if i := indexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	return &VAG{
		Rate: int(binary.BigEndian.Uint32(b[16:])),
		Name: string(name),
		Data: b[HeaderSize : HeaderSize+size],
	}, nil
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

// Bytes returns the VAG as a file.
func (v *VAG) Bytes() []byte {
	b := make([]byte, HeaderSize, HeaderSize+len(v.Data))
	copy(b, Magic)
	binary.BigEndian.PutUint32(b[4:], Version)
	binary.BigEndian.PutUint32(b[12:], uint32(len(v.Data)))
	binary.BigEndian.PutUint32(b[16:], uint32(v.Rate))
	copy(b[32:48], v.Name)
	return append(b, v.Data...)
}

// Options says how Encode lays out the blocks.
type Options struct {
	// Loop is the range of samples to repeat, the end exclusive, or nil
	// for a sound that plays once. The SPU loops whole blocks, so the
	// loop starts at the first block that starts at or after the start,
	// as Sony's aiff2vag rounded it, and the sound is cut at the end of
	// the block the end falls in: a loop sounds right when both are
	// multiples of 28.
	Loop *[2]int
}

// filters are the SPU's prediction coefficients, in 64ths.
var filters = [5][2]int{{0, 0}, {60, 0}, {115, -52}, {98, -55}, {122, -60}}

// Encode turns samples into blocks, choosing for each block the filter and
// shift that decode closest to it.
func Encode(samples []int16, o Options) []byte {
	n := len(samples)
	loopStart := -1
	if o.Loop != nil {
		start, end := o.Loop[0], o.Loop[1]
		if start < 0 || end > n || start >= end {
			start, end = 0, n
		}
		loopStart = (start + BlockSamples - 1) / BlockSamples
		n = (end + BlockSamples - 1) / BlockSamples * BlockSamples
	}
	blocks := (n + BlockSamples - 1) / BlockSamples
	out := make([]byte, BlockSize, BlockSize*(blocks+2)) // the block of silence
	var e encoder
	var buf [BlockSamples]float64
	for b := range blocks {
		for i := range buf {
			buf[i] = 0
			if j := b*BlockSamples + i; j < len(samples) {
				buf[i] = float64(samples[j])
			}
		}
		block := e.block(&buf)
		flags := byte(0)
		switch {
		case loopStart >= 0:
			flags = FlagRepeat
			if b == loopStart {
				flags |= FlagStart
			}
			if b == blocks-1 {
				flags |= FlagEnd
			}
		case b == blocks-1:
			flags = FlagEnd
		}
		block[1] = flags
		out = append(out, block[:]...)
	}
	if loopStart < 0 {
		end := make([]byte, BlockSize)
		end[0] = out[len(out)-BlockSize]
		end[1] = FlagEnd | FlagRepeat | FlagStart
		out = append(out, end...)
	}
	return out
}

// encoder carries what Encode keeps from one block to the next: the last two
// samples the SPU will have decoded, which the next block's predictions
// start from.
type encoder struct {
	old, older int
}

// block encodes 28 samples, trying every filter and every shift against the
// SPU's own arithmetic, from the history the SPU will have, and keeping the
// one that comes out closest to the samples. Encoding against what will be
// decoded, rather than against the input, keeps rounding errors from
// adding up from one sample to the next.
func (e *encoder) block(samples *[BlockSamples]float64) [BlockSize]byte {
	var best [BlockSamples]int8
	bestErr := math.Inf(1)
	bestFilter, bestShift := 0, 0
	var end [2]int
	var nibs [BlockSamples]int8
	for f := range filters {
		for shift := 0; shift <= 12; shift++ {
			old, older := e.old, e.older
			sum := 0.0
			for i, x := range samples {
				pred := (old*filters[f][0] + older*filters[f][1] + 32) >> 6
				n := math.Round((x - float64(pred)) * float64(int(1)<<shift) / 4096)
				n = max(min(n, 7), -8)
				nibs[i] = int8(n)
				y := (int(n)<<12)>>shift + pred
				y = max(min(y, 32767), -32768)
				d := x - float64(y)
				sum += d * d
				if sum >= bestErr {
					break
				}
				older, old = old, y
			}
			if sum < bestErr {
				bestErr, bestFilter, bestShift, best = sum, f, shift, nibs
				end = [2]int{old, older}
			}
		}
	}
	e.old, e.older = end[0], end[1]
	var out [BlockSize]byte
	out[0] = byte(bestFilter<<4 | bestShift)
	for i, n := range best {
		out[2+i/2] |= byte(n&15) << (4 * (i % 2))
	}
	return out
}

// Samples decodes blocks as the SPU plays them, from the first block to the
// first with FlagEnd, without looping.
func Samples(data []byte) []int16 {
	var out []int16
	var old, older int
	for b := 0; b+BlockSize <= len(data); b += BlockSize {
		shift := int(data[b] & 15)
		if shift > 12 {
			shift = 9
		}
		f := min(int(data[b]>>4), 4)
		for i := range BlockSamples {
			nib := int(data[b+2+i/2]>>(4*(i%2))) & 15
			s := int(int16(nib<<12)) >> shift
			s += (old*filters[f][0] + older*filters[f][1] + 32) >> 6
			s = max(min(s, 32767), -32768)
			out = append(out, int16(s))
			older, old = old, s
		}
		if data[b+1]&FlagEnd != 0 {
			break
		}
	}
	return out
}

// String describes the VAG for a listing.
func (v *VAG) String() string {
	return fmt.Sprintf("%q, %d Hz, %d blocks", v.Name, v.Rate, len(v.Data)/BlockSize)
}
