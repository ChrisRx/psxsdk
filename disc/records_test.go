//go:build !yaroze

package disc

import (
	"encoding/binary"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// An image builds the loadable part of a program, laid out as TinyGo lays
// out the records: the strings wherever they fall, and each record pointing
// back at its two.
type image struct {
	base uint32
	data []byte
}

func (im *image) word(v uint32) {
	im.data = binary.LittleEndian.AppendUint32(im.data, v)
}

// str appends s and returns its address.
func (im *image) str(s string) uint32 {
	addr := im.base + uint32(len(im.data))
	im.data = append(im.data, s...)
	return addr
}

func (im *image) align() {
	for len(im.data)%4 != 0 {
		im.data = append(im.data, 0)
	}
}

// record appends a record as Register lays one out, and returns its offset.
func (im *image) record(name, src string) int {
	nameAddr, srcAddr := im.str(name), im.str(src)
	im.align()
	off := len(im.data)
	im.word(magic)
	im.word(nameAddr)
	im.word(uint32(len(name)))
	im.word(srcAddr)
	im.word(uint32(len(src)))
	im.word(0)
	im.word(0)
	return off
}

func TestFind(t *testing.T) {
	im := &image{base: 0x80140000}
	im.word(0x27bdffe8) // some code first
	wall := im.record("DATA/WALL0.TMD", "assets/wall0.tmd")

	// The magic word followed by pointers that lead nowhere, as an
	// instruction stream or the resident library might hold.
	im.word(magic)
	im.word(0x12345678)
	im.word(14)
	im.word(0)
	im.word(0)

	// A name pointing at bytes that are not text.
	bad := im.base + uint32(len(im.data))
	im.word(magic)
	im.word(bad)
	im.word(8)
	im.word(bad)
	im.word(8)
	im.word(0)
	im.word(0)

	movie := im.record("MOVIE.STR", "../build/movie.str")

	// A record cut off by the end of the image is not one.
	im.word(magic)
	im.word(im.base)
	im.word(4)

	assert.Equal(t, []Record{
		{Name: "DATA/WALL0.TMD", Src: "assets/wall0.tmd", Offset: wall},
		{Name: "MOVIE.STR", Src: "../build/movie.str", Offset: movie},
	}, Find(im.data, im.base))

	// Nothing before the image's base address is reachable.
	assert.Equal(t, 0, len(Find(im.data, im.base+4)))
}

func TestPatch(t *testing.T) {
	im := &image{base: 0x80140000}
	off := im.record("DATA/WALL0.TMD", "assets/wall0.tmd")
	before := append([]byte(nil), im.data...)

	recs := Find(im.data, im.base)
	recs[0].Patch(im.data, 23, 4096)

	assert.Equal(t, uint32(23), binary.LittleEndian.Uint32(im.data[off+offSector:]))
	assert.Equal(t, uint32(4096), binary.LittleEndian.Uint32(im.data[off+offSize:]))
	// Only those two words changed.
	assert.Equal(t, before[:off+offSector], im.data[:off+offSector])

	// A patched record is still found, so an executable taken back off a
	// disc can be placed on another.
	assert.Equal(t, recs, Find(im.data, im.base))
}
