package emu

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"

	mdecfmt "go.chrisrx.dev/psx/format/mdec"
	"go.chrisrx.dev/x/assert"
)

// mdecPattern is a picture with smooth gradients and hard edges in every
// color, w by h.
func mdecPattern(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), uint8((x + y) * 4), 0xff}
			if (x/12+y/12)%3 == 0 {
				c.R, c.B = 255-c.R, 40
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// MDEC script words: mdecControl writes the word after it to the control
// register, mdecRead reads as many words as the word after it says from the
// data register, then the status. Every other word is written to the data
// register.
const (
	mdecControl = 0xffffffff
	mdecRead    = 0xfffffffe
)

// runMDECScript runs a script, printing what it reads as psxe's harness in
// testdata/mdec-harness.c does.
func runMDECScript(m *mdec, script []uint32) string {
	var b strings.Builder
	for i := 0; i < len(script); i++ {
		switch script[i] {
		case mdecControl:
			i++
			m.Write32(4, script[i])
		case mdecRead:
			i++
			for range script[i] {
				fmt.Fprintf(&b, "%08x\n", m.Read32(0))
			}
			fmt.Fprintf(&b, "s %08x\n", m.Read32(4))
		default:
			m.Write32(0, script[i])
		}
	}
	return b.String()
}

// mdecDecodeWords is a decode command for codes without the padding at the
// end, which psxe would decode past its input: an odd code out is made up
// with padding at the front instead, which every block skips.
func mdecDecodeWords(cmd uint32, codes []uint16) []uint32 {
	if len(codes)%2 == 1 {
		codes = append([]uint16{mdecfmt.EOB}, codes...)
	}
	words := []uint32{cmd | uint32(len(codes)/2)}
	for i := 0; i < len(codes); i += 2 {
		words = append(words, uint32(codes[i])|uint32(codes[i+1])<<16)
	}
	return words
}

// mdecScript loads the tables and decodes a 32x32 picture into 24-bit and
// signed 15-bit pixels with bit 15 set, and a block into 8-bit monochrome,
// reading a word past the end of each.
func mdecScript() []uint32 {
	s := []uint32{mdecControl, 0x80000000, mdecControl, 0x60000000}
	s = append(s, mdecfmt.TableCommands()...)
	codes := mdecfmt.Encode(mdecPattern(32, 32), 6)
	s = append(s, mdecDecodeWords(0x30000000, codes)...)
	s = append(s, mdecRead, 4*192+1)
	s = append(s, mdecDecodeWords(0x38000000|1<<26|1<<25, codes)...)
	s = append(s, mdecRead, 4*128+1)
	s = append(s, mdecDecodeWords(0x28000000, []uint16{0x0c10, 0x0005, 0x0803, 0x1bf0, mdecfmt.EOB})...)
	s = append(s, mdecRead, 16+1)
	return s
}

// TestMDECMatchesPsxe checks the MDEC against psxe's, word for word, through
// the status register and every pixel of a decode in each depth.
func TestMDECMatchesPsxe(t *testing.T) {
	script := mdecScript()
	if h := os.Getenv("MDEC_HARNESS"); h != "" {
		in := make([]byte, 4*len(script))
		for i, w := range script {
			binary.LittleEndian.PutUint32(in[4*i:], w)
		}
		cmd := exec.Command(h)
		cmd.Stdin = bytes.NewReader(in)
		out, err := cmd.Output()
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		if !assert.NoError(t, os.WriteFile("testdata/mdec-psxe.txt", out, 0o644)) {
			t.FailNow()
		}
	}
	want, err := os.ReadFile("testdata/mdec-psxe.txt")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, string(want), runMDECScript(newMDEC(func(error) {}), script))
}

// mdecPicture lays 24-bit macroblocks out as a picture, w by h, column by
// column as the encoder orders them.
func mdecPicture(out []byte, w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	i := 0
	for mx := 0; mx < w; mx += 16 {
		for my := 0; my < h; my += 16 {
			for p := range 256 {
				px := out[(i*256+p)*3:]
				img.SetNRGBA(mx+p%16, my+p/16, color.NRGBA{px[0], px[1], px[2], 0xff})
			}
			i++
		}
	}
	return img
}

func psnr(a, b *image.NRGBA) float64 {
	var sum float64
	for i := range a.Pix {
		if i%4 == 3 {
			continue
		}
		d := float64(a.Pix[i]) - float64(b.Pix[i])
		sum += d * d
	}
	mse := sum / float64(len(a.Pix)/4*3)
	return 10 * math.Log10(255*255/mse)
}

// TestMDECRoundTrip encodes a smooth picture and decodes it again, padding
// and all, as a program would, and checks it comes back close to how it went
// in, and closer the finer it is quantized. psxe's inverse DCT rounds
// between its passes, which costs a few dB however fine the quantization.
func TestMDECRoundTrip(t *testing.T) {
	const w, h = 64, 48
	in := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			in.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 5), uint8(128 + x - y), 0xff})
		}
	}
	var last float64
	for _, q := range []int{16, 4, 1} {
		m := newMDEC(func(error) {})
		runMDECScript(m, mdecfmt.TableCommands())
		cmd := mdecfmt.DecodeCommand(mdecfmt.Encode(in, q))
		runMDECScript(m, cmd)
		assert.Equal(t, uint32(w*h*3/4), m.outputLeft, "the padding decodes to nothing")

		out := make([]byte, 0, w*h*3)
		for range w * h * 3 / 4 {
			out = binary.LittleEndian.AppendUint32(out, m.Read32(0))
		}
		p := psnr(in, mdecPicture(out, w, h))
		assert.Equal(t, true, p > 30, "qscale %d: PSNR %.1fdB", q, p)
		assert.Equal(t, true, p > last, "qscale %d: PSNR %.1fdB, no better than %.1fdB", q, p, last)
		last = p
	}
}

// TestMDECStatus checks the status register through a command's life.
func TestMDECStatus(t *testing.T) {
	m := newMDEC(func(error) {})
	m.Write32(4, 0x80000000|0x60000000)
	assert.Equal(t, uint32(0x80040000), m.Read32(4), "reset: output empty, current block 4")

	m.Write32(0, 0x30000002) // decode, two words to come
	assert.Equal(t, uint32(0xb4040002), m.Read32(4), "busy, wanting input, 24-bit")
	m.Write32(0, 0x0000fe00|0x0400<<16)
	assert.Equal(t, uint32(0xb4040001), m.Read32(4))
	m.Write32(0, 0xfe00fe00)
	assert.Equal(t, uint32(0x4c040000), m.Read32(4), "done: input full, output requested")
}

// TestMDECDMA decodes through the DMA channels, as a program does: the
// command and codes in on channel 0 in blocks, the pixels out on channel 1.
func TestMDECDMA(t *testing.T) {
	m := newTestMachine(t)
	const (
		ctrl  = 0x1f801824
		data  = 0x1f801820
		dpcr  = 0x1f8010f0
		in    = 0x10000
		out   = 0x40000
		w, h  = 32, 16
		words = w * h * 3 / 4
	)
	m.Bus.Write32(ctrl, 0x80000000)
	m.Bus.Write32(ctrl, 0x60000000)
	for _, v := range mdecfmt.TableCommands() {
		m.Bus.Write32(data, v)
	}
	pic := mdecPattern(w, h)
	cmd := mdecfmt.DecodeCommand(mdecfmt.Encode(pic, 2))
	for i, v := range cmd {
		m.Bus.Write32(in+4*uint32(i), v)
	}

	m.Bus.Write32(dpcr, 0x88)
	m.Bus.Write32(0x1f801080, in)
	m.Bus.Write32(0x1f801084, uint32(len(cmd)/32)<<16|32)
	m.Bus.Write32(0x1f801088, 0x01000201)
	m.Bus.Write32(0x1f801090, out)
	m.Bus.Write32(0x1f801094, words/32<<16|32)
	m.Bus.Write32(0x1f801098, 0x01000200)

	ref := newMDEC(func(error) {})
	runMDECScript(ref, mdecfmt.TableCommands())
	runMDECScript(ref, cmd)
	for i := range uint32(words) {
		if want, got := ref.Read32(0), m.Bus.Read32(out+4*i); want != got {
			t.Fatalf("word %d: %08x, want %08x", i, got, want)
		}
	}
	assert.Equal(t, uint32(0), m.Bus.Read32(0x1f801098)&0x01000000, "channel 1 is done")
}

// TestDisplay24 shows VRAM as 24-bit pixels, three bytes each, from the
// display area's left edge.
func TestDisplay24(t *testing.T) {
	m := drawing(t)
	const gp1 = 0x1f801814
	// Two pixels, red then blue, at x=100 in VRAM's halfwords.
	m.gpu.vram[100] = 0x00ff                  // R=ff G=00
	m.gpu.vram[101] = 0x0000                  // B=00 R=00
	m.gpu.vram[102] = 0xff00                  // G=00 B=ff
	m.Bus.Write32(gp1, 0x03000000)            // display on
	m.Bus.Write32(gp1, 0x05000000|100)        // display area at (100, 0)
	m.Bus.Write32(gp1, 0x07000000|256<<10|16) // 240 lines
	m.Bus.Write32(gp1, 0x08000000|0x10|1)     // 320 wide, 24-bit

	img := m.Display()
	assert.Equal(t, [2]int{320, 240}, [2]int{img.Rect.Dx(), img.Rect.Dy()})
	assert.Equal(t, color.RGBA{R: 0xff, A: 0xff}, img.RGBAAt(0, 0))
	assert.Equal(t, color.RGBA{B: 0xff, A: 0xff}, img.RGBAAt(1, 0))
}
