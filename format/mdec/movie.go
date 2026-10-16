package mdec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

/*
A movie is laid out for a program to hand to the MDEC as it is, every field
a little-endian word:

  000h      "MDEC"
  004h      width, height (16 bits each, multiples of 16)
  008h      frames (16 bits), and 16 bits of zero
  00Ch      the set quantization tables command, then the Y and UV tables:
            33 words, which the program writes to the MDEC to load them
  090h      the set scale table command, then the table: 33 words
  114h...   each frame: the number of words that follow, then the decode
            command and its codes, padded with EOB to a multiple of 32
            words, the size of a DMA block

The decode command asks for 24-bit unsigned output. Clearing bit 27 of it
asks for 15-bit instead.
*/

// Magic starts a movie.
const Magic = "MDEC"

// DMABlock is the size of the blocks a decode command's words are sent in.
const DMABlock = 32

// MDEC commands.
const (
	CmdDecode24 = 0x30000000 // decode macroblocks into 24-bit pixels
	CmdSetQuant = 0x40000001 // set the Y and UV quantization tables
	CmdSetScale = 0x60000000 // set the scale table
)

// A Movie is a sequence of frames of the same size, each the codes for one
// picture as Encode returns them.
type Movie struct {
	Width, Height int
	Frames        [][]uint16
}

// DecodeCommand returns the words a program sends the MDEC to decode a
// frame: the command, then the codes two to a word, padded with EOB to a
// multiple of DMABlock words, the command included.
func DecodeCommand(codes []uint16) []uint32 {
	n := (len(codes) + 1) / 2
	total := (1 + n + DMABlock - 1) / DMABlock * DMABlock
	words := make([]uint32, total)
	words[0] = CmdDecode24 | uint32(total-1)
	for i := 1; i < total; i++ {
		lo, hi := uint32(EOB), uint32(EOB)
		if j := 2 * (i - 1); j < len(codes) {
			lo = uint32(codes[j])
		}
		if j := 2*(i-1) + 1; j < len(codes) {
			hi = uint32(codes[j])
		}
		words[i] = lo | hi<<16
	}
	return words
}

// TableCommands returns the words a program writes to the MDEC to load
// Quant, for Y and UV both, and Scale: each command followed by its table.
func TableCommands() []uint32 {
	w := []uint32{CmdSetQuant}
	q := append(Quant[:], Quant[:]...) // the same table for Y and UV
	for i := 0; i < len(q); i += 4 {
		w = append(w, binary.LittleEndian.Uint32(q[i:]))
	}
	w = append(w, CmdSetScale)
	for i := 0; i < len(Scale); i += 2 {
		w = append(w, uint32(uint16(Scale[i]))|uint32(uint16(Scale[i+1]))<<16)
	}
	return w
}

// WriteTo writes the movie.
func (m *Movie) WriteTo(w io.Writer) (int64, error) {
	if m.Width%16 != 0 || m.Height%16 != 0 || m.Width > 0xffff || m.Height > 0xffff {
		return 0, fmt.Errorf("a movie of %dx%d is not a multiple of 16 each way", m.Width, m.Height)
	}
	if len(m.Frames) > 0xffff {
		return 0, fmt.Errorf("%d frames is more than a movie holds", len(m.Frames))
	}
	words := []uint32{
		binary.LittleEndian.Uint32([]byte(Magic)),
		uint32(m.Width) | uint32(m.Height)<<16,
		uint32(len(m.Frames)),
	}
	words = append(words, TableCommands()...)
	for i, f := range m.Frames {
		cmd := DecodeCommand(f)
		if len(cmd)-1 > 0xffff {
			return 0, fmt.Errorf("frame %d is %d words, more than a decode command takes", i, len(cmd))
		}
		words = append(words, uint32(len(cmd)))
		words = append(words, cmd...)
	}
	buf := make([]byte, 4*len(words))
	for i, v := range words {
		binary.LittleEndian.PutUint32(buf[4*i:], v)
	}
	n, err := w.Write(buf)
	return int64(n), err
}

// Read reads a movie, keeping each frame's codes as they were written, EOB
// padding and all.
func Read(r io.Reader) (*Movie, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) < 12+4*(2*33) || string(data[:4]) != Magic {
		return nil, errors.New("not an MDEC movie")
	}
	word := func(i int) uint32 { return binary.LittleEndian.Uint32(data[4*i:]) }
	m := &Movie{Width: int(word(1) & 0xffff), Height: int(word(1) >> 16)}
	frames := int(word(2) & 0xffff)
	at := 3 + 2*33
	for i := range frames {
		if 4*(at+1) > len(data) {
			return nil, fmt.Errorf("frame %d: %w", i, io.ErrUnexpectedEOF)
		}
		n := int(word(at))
		if n == 0 || 4*(at+1+n) > len(data) {
			return nil, fmt.Errorf("frame %d: %w", i, io.ErrUnexpectedEOF)
		}
		codes := make([]uint16, 0, 2*(n-1))
		for j := at + 2; j < at+1+n; j++ {
			codes = append(codes, uint16(word(j)), uint16(word(j)>>16))
		}
		m.Frames = append(m.Frames, codes)
		at += 1 + n
	}
	return m, nil
}
