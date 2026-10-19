// Package tmd reads and writes TMD files, the model format the extended
// graphics library draws with GsSortObject4: objects of 16-bit vertices,
// normals and primitives, each primitive a packet that GsSortObject4 turns
// into a GPU primitive after transforming its vertices.
//
// A file is little-endian words:
//
//	000h  ID, 0x41
//	004h  flags: bit 0 set when the object table holds addresses, which
//	      GsMapModelingData sets when it rewrites the offsets in memory
//	008h  the number of objects
//	00Ch  the object table, 28 bytes an object: offsets from 00Ch of its
//	      vertices, its normals and its primitives, each with its count,
//	      and a scale, a power of two the library ignores
//
// Each object's primitives, vertices and normals follow, in that order. A
// vertex is X, Y, Z and a pad, 16 bits each, with X right, Y down and Z
// away from the camera; a normal the same, in fixed point with 4096 as 1.
//
// A primitive is a word of mode, flag, ilen and olen, from the high byte,
// then ilen words of packet. The mode is the GPU's command (0x20 polygon,
// 0x40 line, 0x60 sprite) with its bits for gouraud, quad, texture and
// semitransparency, and bit 0 set when the primitive is not lit; the flag
// says not lit (bit 0), double-sided (bit 1) and gradated (bit 2); olen is
// the size in words of the GPU primitive the library makes of it. The
// packets' layouts are tabulated in Sony's File Formats manual and in
// Primitive's methods here.
package tmd

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ID is the first word of a TMD file.
const ID = 0x41

// A Vector is a vertex, or a normal in 4096ths.
type Vector struct{ X, Y, Z int16 }

// A Primitive is one packet as the file holds it.
type Primitive struct {
	Mode, Flag byte
	Olen       byte
	Packet     []uint32
}

// An Object is a model.
type Object struct {
	Vertices   []Vector
	Normals    []Vector
	Primitives []Primitive
	Scale      int32
}

// A TMD is a set of models.
type TMD struct {
	Objects []Object
}

// Bytes returns the TMD as a file, with offsets, as rsdlink wrote it.
func (t *TMD) Bytes() []byte {
	le := binary.LittleEndian
	b := le.AppendUint32(nil, ID)
	b = le.AppendUint32(b, 0)
	b = le.AppendUint32(b, uint32(len(t.Objects)))
	table := len(b)
	b = append(b, make([]byte, 28*len(t.Objects))...)
	for i, o := range t.Objects {
		prims := len(b) - table
		for _, p := range o.Primitives {
			b = le.AppendUint32(b, uint32(p.Mode)<<24|uint32(p.Flag)<<16|uint32(len(p.Packet))<<8|uint32(p.Olen))
			for _, w := range p.Packet {
				b = le.AppendUint32(b, w)
			}
		}
		verts := len(b) - table
		for _, v := range o.Vertices {
			b = appendVector(b, v)
		}
		norms := len(b) - table
		for _, n := range o.Normals {
			b = appendVector(b, n)
		}
		e := b[table+28*i:]
		for k, v := range [7]uint32{
			uint32(verts), uint32(len(o.Vertices)),
			uint32(norms), uint32(len(o.Normals)),
			uint32(prims), uint32(len(o.Primitives)),
			uint32(o.Scale),
		} {
			le.PutUint32(e[4*k:], v)
		}
	}
	return b
}

func appendVector(b []byte, v Vector) []byte {
	for _, c := range [4]int16{v.X, v.Y, v.Z, 0} {
		b = binary.LittleEndian.AppendUint16(b, uint16(c))
	}
	return b
}

// Decode reads a TMD file, whose table holds offsets, not yet mapped.
func Decode(b []byte) (*TMD, error) {
	le := binary.LittleEndian
	if len(b) < 12 || le.Uint32(b) != ID {
		return nil, errors.New("tmd: not a TMD file")
	}
	if le.Uint32(b[4:])&1 != 0 {
		return nil, errors.New("tmd: the object table holds addresses, not offsets")
	}
	n := int(le.Uint32(b[8:]))
	if 12+28*n > len(b) {
		return nil, fmt.Errorf("tmd: %d objects run past the end of the file", n)
	}
	t := &TMD{}
	at := func(off, count, size int) ([]byte, error) {
		start := 12 + off
		if off < 0 || count < 0 || start+count*size > len(b) {
			return nil, fmt.Errorf("tmd: %d items at %#x run past the end of the file", count, off)
		}
		return b[start : start+count*size], nil
	}
	for i := range n {
		e := b[12+28*i:]
		var o Object
		vb, err := at(int(le.Uint32(e)), int(le.Uint32(e[4:])), 8)
		if err != nil {
			return nil, err
		}
		o.Vertices = vectors(vb)
		nb, err := at(int(le.Uint32(e[8:])), int(le.Uint32(e[12:])), 8)
		if err != nil {
			return nil, err
		}
		o.Normals = vectors(nb)
		o.Scale = int32(le.Uint32(e[24:]))
		pos := 12 + int(le.Uint32(e[16:]))
		for range int(le.Uint32(e[20:])) {
			if pos+4 > len(b) {
				return nil, errors.New("tmd: primitives run past the end of the file")
			}
			h := le.Uint32(b[pos:])
			ilen := int(h >> 8 & 0xff)
			if pos+4+4*ilen > len(b) {
				return nil, errors.New("tmd: a primitive runs past the end of the file")
			}
			p := Primitive{Mode: byte(h >> 24), Flag: byte(h >> 16), Olen: byte(h), Packet: make([]uint32, ilen)}
			for k := range p.Packet {
				p.Packet[k] = le.Uint32(b[pos+4+4*k:])
			}
			o.Primitives = append(o.Primitives, p)
			pos += 4 + 4*ilen
		}
		t.Objects = append(t.Objects, o)
	}
	return t, nil
}

func vectors(b []byte) []Vector {
	out := make([]Vector, len(b)/8)
	for i := range out {
		out[i] = Vector{
			int16(binary.LittleEndian.Uint16(b[8*i:])),
			int16(binary.LittleEndian.Uint16(b[8*i+2:])),
			int16(binary.LittleEndian.Uint16(b[8*i+4:])),
		}
	}
	return out
}
