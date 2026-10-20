package archive

import (
	"bytes"
	"math/rand"
	"testing"

	"go.chrisrx.dev/psx/archive"
	"go.chrisrx.dev/x/assert"
)

func sample() []Entry {
	r := rand.New(rand.NewSource(1))
	noise := make([]byte, 5000)
	r.Read(noise)
	text := bytes.Repeat([]byte("the cube turns and the badge stays in the corner. "), 400)
	return []Entry{
		{Name: Name("data/cube.tmd"), Data: text},
		{Name: Name("art/noise.vb"), Data: noise},
		{Name: "FORCED.TIM", Data: text[:1000], Store: true},
		{Name: "EMPTY.DAT"},
	}
}

func TestRoundTrip(t *testing.T) {
	entries := sample()
	b, err := Pack(entries)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(b)%4, "the archive is whole words")

	words := make([]uint32, len(b)/4)
	for i := range words {
		words[i] = uint32(b[4*i]) | uint32(b[4*i+1])<<8 | uint32(b[4*i+2])<<16 | uint32(b[4*i+3])<<24
	}
	a, err := archive.Open(words)
	assert.NoError(t, err)
	assert.Equal(t, 4, a.Len())
	assert.Equal(t, len(b), a.Size())

	cube, ok := a.Lookup("CUBE.TMD")
	assert.Equal(t, true, ok)
	assert.Equal(t, archive.Deflate, cube.Method, "text packs")
	assert.Equal(t, len(entries[0].Data), cube.Size)
	assert.Equal(t, true, len(cube.Packed()) < cube.Size/4, "and packs well")
	assert.Equal(t, 0, len(cube.Data()), "a packed entry has no data in place")
	got, err := cube.Load()
	assert.NoError(t, err)
	assert.Equal(t, true, bytes.Equal(asBytes(got)[:cube.Size], entries[0].Data), "the text comes back")

	noise, ok := a.Lookup("NOISE.VB")
	assert.Equal(t, true, ok)
	assert.Equal(t, archive.Stored, noise.Method, "noise is stored")
	assert.Equal(t, true, bytes.Equal(asBytes(noise.Data())[:noise.Size], entries[1].Data), "and read in place")
	buf := make([]uint32, noise.Words())
	w, err := noise.Unpack(buf)
	assert.NoError(t, err)
	assert.Equal(t, true, bytes.Equal(asBytes(w)[:noise.Size], entries[1].Data), "or copied out")
	_, err = noise.Unpack(buf[:1])
	assert.Equal(t, archive.ErrSpace, err)

	forced, _ := a.Lookup("FORCED.TIM")
	assert.Equal(t, archive.Stored, forced.Method, "Store is honoured")
	empty, _ := a.Lookup("EMPTY.DAT")
	assert.Equal(t, 0, empty.Size)
	w, err = empty.Unpack(nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(w))
	_, ok = a.Lookup("NOWHERE")
	assert.Equal(t, false, ok)
}

func TestPackRefuses(t *testing.T) {
	_, err := Pack([]Entry{{Name: "A"}, {Name: "A"}})
	assert.Equal(t, true, err != nil, "two of a name")
	_, err = Pack([]Entry{{Name: "DATA/A"}})
	assert.Equal(t, true, err != nil, "a path")
	_, err = Pack([]Entry{{Name: "ABCDEFGHIJKLMNOP"}})
	assert.Equal(t, true, err != nil, "too long")
	_, err = Pack([]Entry{{}})
	assert.Equal(t, true, err != nil, "no name")
}

func TestOpenRefuses(t *testing.T) {
	b, err := Pack(sample())
	assert.NoError(t, err)
	_, err = archive.OpenBytes(b[:8])
	assert.Equal(t, archive.ErrFormat, err, "short")
	bad := bytes.Clone(b)
	bad[0] = 'X'
	_, err = archive.OpenBytes(bad)
	assert.Equal(t, archive.ErrFormat, err, "wrong id")
	bad = bytes.Clone(b)
	bad[8] = 0xff // size past the bytes
	_, err = archive.OpenBytes(bad)
	assert.Equal(t, archive.ErrFormat, err, "size past the end")
	bad = bytes.Clone(b)
	a, _ := archive.OpenBytes(bad)
	e := a.At(0)
	for i := range e.Packed() {
		e.Packed()[i] ^= 0x5a
	}
	_, err = e.Load()
	assert.Equal(t, archive.ErrCorrupt, err, "a damaged stream")
}

func TestName(t *testing.T) {
	assert.Equal(t, "CUBE.TMD", Name("data/cube.tmd"))
	assert.Equal(t, "STONE_WA.TIM", Name("art\\stone_wall.tim"))
}

func asBytes(w []uint32) []byte {
	b := make([]byte, 4*len(w))
	for i, v := range w {
		b[4*i], b[4*i+1], b[4*i+2], b[4*i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	return b
}
