package mdec

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestEncodeFlat(t *testing.T) {
	// A flat grey 16x16 picture is one macroblock: two chroma blocks of
	// nothing, and four Y blocks of only their DC term, each as a header
	// code and the end of the block.
	img := image.NewUniform(color.Gray{Y: 128 + 64})
	codes := Encode(subImage{img, image.Rect(0, 0, 16, 16)}, 4)
	dc := uint16(64 * 8 / 2) // the mean times 8, over Quant[0]
	assert.Equal(t, []uint16{
		4 << 10, EOB, // Cr
		4 << 10, EOB, // Cb
		4<<10 | dc, EOB,
		4<<10 | dc, EOB,
		4<<10 | dc, EOB,
		4<<10 | dc, EOB,
	}, codes)
}

type subImage struct {
	image.Image
	r image.Rectangle
}

func (s subImage) Bounds() image.Rectangle { return s.r }

func TestSize(t *testing.T) {
	w, h := Size(320, 240)
	assert.Equal(t, [2]int{320, 240}, [2]int{w, h})
	w, h = Size(17, 1)
	assert.Equal(t, [2]int{32, 16}, [2]int{w, h})
}

func TestDecodeCommand(t *testing.T) {
	words := DecodeCommand([]uint16{1, 2, 3})
	assert.Equal(t, DMABlock, len(words), "padded to a DMA block")
	assert.Equal(t, uint32(CmdDecode24|DMABlock-1), words[0])
	assert.Equal(t, uint32(1|2<<16), words[1])
	assert.Equal(t, uint32(3|EOB<<16), words[2])
	assert.Equal(t, uint32(EOB|EOB<<16), words[DMABlock-1])
}

func TestMovieRoundTrip(t *testing.T) {
	m := &Movie{Width: 32, Height: 16, Frames: [][]uint16{{1, 2, 3}, {4}}}
	var buf bytes.Buffer
	_, err := m.WriteTo(&buf)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	data := buf.Bytes()
	assert.Equal(t, Magic, string(data[:4]))
	assert.Equal(t, uint32(CmdSetQuant), binary.LittleEndian.Uint32(data[0x0c:]))
	assert.Equal(t, uint32(CmdSetScale), binary.LittleEndian.Uint32(data[0x90:]))
	assert.Equal(t, uint32(DMABlock), binary.LittleEndian.Uint32(data[0x114:]), "the first frame's length")
	assert.Equal(t, 0x114+2*4*(1+DMABlock), len(data))

	got, err := Read(bytes.NewReader(data))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, [2]int{32, 16}, [2]int{got.Width, got.Height})
	assert.Equal(t, 2, len(got.Frames))
	assert.Equal(t, []uint16{1, 2, 3, EOB}, got.Frames[0][:4])
	assert.Equal(t, []uint16{4, EOB}, got.Frames[1][:2])
}

func TestWriteRejectsOddSize(t *testing.T) {
	_, err := (&Movie{Width: 30, Height: 16}).WriteTo(&bytes.Buffer{})
	assert.Equal(t, true, err != nil)
}
