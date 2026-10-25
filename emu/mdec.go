package emu

import (
	"encoding/binary"
	"fmt"
)

// MDEC commands, from the top three bits of the command word.
const (
	mdecNop = iota
	mdecDecode
	mdecSetQuant
	mdecSetScale
)

// mdecEOB is the halfword that ends a block's run-length codes, and pads
// the input to a whole number of DMA blocks.
const mdecEOB = 0xfe00

// zagzig maps a coefficient's place in the zigzag order to its place in the
// 8x8 block, row by row.
var zagzig = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// mdec is the motion decoder at 1F801820, which turns run-length coded,
// quantized DCT blocks into pixels. psxe decodes the whole of a command's
// input the moment its last word arrives, and hands the pixels out a word
// at a time from then on.
type mdec struct {
	cmd uint32

	// input collects the words a command takes, of which wordsLeft are
	// still to come.
	input     []uint32
	wordsLeft uint32

	// output is what the last command produced, of which outputLeft words
	// are still to be read from outputAt.
	output     []byte
	outputAt   int
	outputLeft uint32

	currentBlock  uint32
	outputBit15   uint32
	outputSigned  uint32
	outputDepth   uint32
	inputRequest  uint32
	outputRequest uint32
	busy          uint32
	inputFull     uint32
	outputEmpty   uint32
	enableDMA0    bool
	enableDMA1    bool

	recvColor bool
	uvQuant   [64]uint8
	yQuant    [64]uint8
	scale     [64]int16

	yblk, crblk, cbblk [64]int16

	stop func(error)
}

func newMDEC(stop func(error)) *mdec { return &mdec{stop: stop} }

func (m *mdec) Read32(off uint32) uint32 {
	switch off {
	case 0:
		if m.outputLeft == 0 {
			// psxe clears the empty flag here, though there is nothing
			// left to read.
			m.outputEmpty, m.outputAt, m.outputRequest = 0, 0, 0
			return 0xaaaaaaaa
		}
		m.outputLeft--
		v := binary.LittleEndian.Uint32(m.output[m.outputAt*4:])
		m.outputAt++
		return v
	case 4:
		return m.wordsLeft |
			m.currentBlock<<16 |
			m.outputBit15<<23 |
			m.outputSigned<<24 |
			m.outputDepth<<25 |
			m.outputRequest<<27 |
			m.inputRequest<<28 |
			m.busy<<29 |
			m.inputFull<<30 |
			m.outputEmpty<<31
	}
	return 0
}

func (m *mdec) Read16(off uint32) uint16 {
	m.stop(fmt.Errorf("16-bit MDEC read at offset %d, which psxe does not implement", off))
	return 0
}

func (m *mdec) Read8(off uint32) uint8 {
	m.stop(fmt.Errorf("8-bit MDEC read at offset %d, which psxe does not implement", off))
	return 0
}

// psxe ignores the narrower writes.
func (m *mdec) Write16(uint32, uint16) {}
func (m *mdec) Write8(uint32, uint8)   {}

func (m *mdec) Write32(off, v uint32) {
	switch off {
	case 0:
		m.writeData(v)
	case 4:
		m.enableDMA0 = v&(1<<30) != 0
		m.enableDMA1 = v&(1<<29) != 0
		if v&(1<<31) != 0 {
			m.busy, m.wordsLeft = 0, 0
			m.outputBit15, m.outputSigned, m.outputDepth = 0, 0, 0
			m.inputRequest, m.outputRequest, m.inputFull = 0, 0, 0
			m.outputEmpty = 1
			m.currentBlock = 4
		}
	}
}

// writeData takes a word written to the command/parameter register: the
// next word a command is waiting for, or else a new command.
func (m *mdec) writeData(v uint32) {
	if m.wordsLeft != 0 {
		m.input = append(m.input, v)
		m.wordsLeft--
		if m.wordsLeft == 0 {
			m.outputEmpty = 0
			m.inputFull = 1
			m.inputRequest = 0
			m.busy = 0
			m.outputRequest = b2u(m.enableDMA1)
			m.execute()
		}
		return
	}

	m.cmd = v
	m.outputRequest = 0
	m.outputEmpty = 1
	m.outputBit15 = v >> 25 & 1
	m.outputSigned = v >> 26 & 1
	m.outputDepth = v >> 27 & 3
	m.inputFull = 0
	m.busy = 1

	switch v >> 29 {
	case mdecNop:
		m.busy = 0
		m.wordsLeft = 0
	case mdecDecode:
		m.wordsLeft = v & 0xffff
	case mdecSetQuant:
		m.recvColor = v&1 != 0
		m.wordsLeft = 16
		if m.recvColor {
			m.wordsLeft = 32
		}
	case mdecSetScale:
		m.wordsLeft = 32
	}
	m.input = m.input[:0]
	if m.wordsLeft != 0 {
		m.inputRequest = 1
	}
}

func (m *mdec) execute() {
	switch m.cmd >> 29 {
	case mdecDecode:
		m.decode()
	case mdecSetQuant:
		in := m.inputBytes()
		copy(m.yQuant[:], in)
		if m.recvColor {
			copy(m.uvQuant[:], in[64:])
		}
	case mdecSetScale:
		in := m.inputBytes()
		for i := range m.scale {
			m.scale[i] = int16(binary.LittleEndian.Uint16(in[i*2:]))
		}
	}
}

func (m *mdec) inputBytes() []byte {
	b := make([]byte, len(m.input)*4)
	for i, w := range m.input {
		binary.LittleEndian.PutUint32(b[i*4:], w)
	}
	return b
}

// rlInput reads the input a halfword at a time, as the decoder takes it.
// psxe reads on past the end of its input; here what is past the end reads
// as the end of a block.
type rlInput struct {
	words []uint32
	at    int
}

func (r *rlInput) len() int { return len(r.words) * 2 }

func (r *rlInput) peek() uint16 {
	if r.at >= r.len() {
		return mdecEOB
	}
	return uint16(r.words[r.at/2] >> (16 * (r.at & 1)))
}

// padded reports whether all that is left of the input is padding.
func (r *rlInput) padded() bool {
	for i := r.at; i < r.len(); i++ {
		if uint16(r.words[i/2]>>(16*(i&1))) != mdecEOB {
			return false
		}
	}
	return true
}

// decode decodes the input into pixels: in 4- and 8-bit depth, a single
// monochrome block; in 15- and 24-bit depth, as many 16x16 macroblocks as
// the input holds, each a Cr, a Cb and four Y blocks.
func (m *mdec) decode() {
	in := &rlInput{words: m.input}

	if m.outputDepth < 2 {
		m.output = make([]byte, 768)
		m.decodeBlock(&m.yblk, in, &m.yQuant)
		for i, y := range m.yblk {
			if m.outputDepth == 1 {
				m.output[i] = byte(y)
			}
			// psxe leaves 4-bit output to do, as zeros.
		}
		m.outputLeft = 64 >> 2
		if m.outputDepth == 0 {
			m.outputLeft = 32 >> 2
		}
		m.outputEmpty, m.outputAt = 0, 0
		return
	}

	size := 768
	if m.outputDepth == 3 {
		size = 512
	}
	m.output = m.output[:0]
	// psxe decodes macroblocks while any input is left, padding and all,
	// reading past the end of its input for the last. Here padding at the
	// end is left alone.
	for in.at < in.len() && !in.padded() {
		m.output = append(m.output, make([]byte, size)...)
		buf := m.output[len(m.output)-size:]
		m.decodeBlock(&m.crblk, in, &m.uvQuant)
		m.decodeBlock(&m.cbblk, in, &m.uvQuant)
		m.decodeBlock(&m.yblk, in, &m.yQuant)
		m.yuvToRGB(buf, 0, 0)
		m.decodeBlock(&m.yblk, in, &m.yQuant)
		m.yuvToRGB(buf, 8, 0)
		m.decodeBlock(&m.yblk, in, &m.yQuant)
		m.yuvToRGB(buf, 0, 8)
		m.decodeBlock(&m.yblk, in, &m.yQuant)
		m.yuvToRGB(buf, 8, 8)
	}
	m.outputLeft = uint32(len(m.output) >> 2)
	m.outputEmpty, m.outputAt = 0, 0
}

// exts10 sign-extends a 10-bit coefficient.
func exts10(v uint16) int { return int(int16(v<<6) >> 6) }

func clampMDEC(v, lo, hi int) int16 { return int16(min(max(v, lo), hi)) }

// decodeBlock decodes one block's run-length codes into blk and transforms
// it back from frequencies to samples, as psxe's rl_decode_block does.
func (m *mdec) decodeBlock(blk *[64]int16, in *rlInput, quant *[64]uint8) {
	*blk = [64]int16{}

	n := in.peek()
	in.at++
	for n == mdecEOB && in.at < in.len() {
		n = in.peek()
		in.at++
	}

	qScale := int(n>>10) & 0x3f
	val := int16(exts10(n&0x3ff) * int(quant[0]))
	k := 0
	for {
		if qScale == 0 {
			val = int16(exts10(n&0x3ff) * 2)
		}
		val = clampMDEC(int(val), -0x400, 0x3ff)
		if qScale > 0 {
			blk[zagzig[k]] = val
		} else {
			blk[k] = val
		}

		n = in.peek()
		if k == 63 {
			break
		}
		in.at++
		k += int(n>>10&0x3f) + 1
		// psxe works the next value out before it checks k, reading the
		// quantization table past its end at the end of a block.
		if k >= 64 {
			break
		}
		val = int16((exts10(n&0x3ff)*int(quant[k])*qScale + 4) / 8)
	}
	idct(blk, &m.scale)
}

// idctScalar is psxe's real_idct: the inverse DCT as two passes of a
// matrix product with the scale table, in integers.
func idctScalar(blk *[64]int16, scale *[64]int16) {
	var buf [64]int16
	src, dst := blk, &buf
	for range 2 {
		for x := range 8 {
			for y := range 8 {
				sum := 0
				for z := range 8 {
					sum += int(src[y+z*8]) * (int(scale[x+z*8]) / 8)
				}
				dst[x+y*8] = int16((sum + 0xfff) / 0x2000)
			}
		}
		src, dst = dst, src
	}
}

// yuvToRGB converts the Y block just decoded, with the chroma blocks, into
// the 8x8 quarter of the macroblock at (xx, yy), in the output depth. The
// arithmetic is psxe's, in doubles.
func (m *mdec) yuvToRGB(buf []byte, xx, yy int) {
	for y := range 8 {
		for x := range 8 {
			c := (x+xx)>>1 + (y+yy)>>1*8
			r := m.crblk[c]
			b := m.cbblk[c]
			// The products are converted explicitly so that they are
			// rounded before the sum, never fused into one operation.
			g := int16(float64(-0.3437*float64(b)) + float64(-0.7143*float64(r)))
			r = int16(1.402 * float64(r))
			b = int16(1.772 * float64(b))

			l := int(m.yblk[x+y*8])
			r = clampMDEC(l+int(r), -128, 127)
			g = clampMDEC(l+int(g), -128, 127)
			b = clampMDEC(l+int(b), -128, 127)
			if m.outputSigned == 0 {
				r ^= 0x80
				g ^= 0x80
				b ^= 0x80
			}

			p := (x + xx) + (y+yy)*16
			if m.outputDepth == 3 {
				rgb := uint16(uint8(b)>>3)<<10 | uint16(uint8(g)>>3)<<5 | uint16(uint8(r)>>3)
				if m.outputBit15 != 0 {
					rgb |= 0x8000
				}
				binary.LittleEndian.PutUint16(buf[p*2:], rgb)
			} else {
				buf[p*3+0] = byte(r)
				buf[p*3+1] = byte(g)
				buf[p*3+2] = byte(b)
			}
		}
	}
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
