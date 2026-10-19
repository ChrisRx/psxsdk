package main

// The game's three data archives, which siocons sent to fixed addresses
// and the program unpacked piece by piece as it needed them. An archive is
// a table of groups, each a table of entries, each an LZSS-packed file:
// textures, models, animations, maps and sounds. The tables are offsets
// from the start of the archive, which the original rewrote into pointers
// in place; here they are read as offsets.

import (
	"encoding/binary"
	"unsafe"
)

// An archive is one of the DATAn.ARC files, as the disc delivered it.
type archive struct {
	data   []byte
	groups [][]arcEntry
}

// An arcEntry is one packed file in an archive: its size unpacked, and the
// packed bytes, which start with a six byte header the unpacker reads.
type arcEntry struct {
	size   int
	packed []byte
}

// parseArchive reads the tables of the archive held in words, which package
// disc read off the disc.
func parseArchive(words []uint32) *archive {
	data := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), 4*len(words))
	u32 := func(off int) int { return int(int32(binary.LittleEndian.Uint32(data[off:]))) }
	a := &archive{data: data}
	ngroups, goff := u32(0), u32(4)
	a.groups = make([][]arcEntry, ngroups)
	for g := range a.groups {
		n, eoff := u32(goff+8*g), u32(goff+8*g+4)
		entries := make([]arcEntry, n)
		for e := range entries {
			rec := eoff + 12*e
			size, packed, off := u32(rec), u32(rec+4), u32(rec+8)
			// The last entry's packed length runs two bytes past the
			// file, which the sectors read from the disc pad.
			entries[e] = arcEntry{size: size, packed: data[off:min(off+6+packed, len(data))]}
		}
		a.groups[g] = entries
	}
	return a
}

// entries returns how many entries group g has.
func (a *archive) entries(g int) int { return len(a.groups[g]) }

// size returns the unpacked size of entry e of group g.
func (a *archive) size(g, e int) int { return a.groups[g][e].size }

// unpack unpacks entry e of group g into dst, which must hold its size.
func (a *archive) unpack(g, e int, dst []byte) {
	lzss(dst, a.groups[g][e].packed)
}

// words unpacks entry e of group g into new, word-aligned memory and returns
// it as words, which is how the library takes a TIM or a sound bank, with
// the bytes of the same memory. The memory is never given back, so it is
// for what the game keeps.
func (a *archive) words(g, e int) ([]uint32, []byte) {
	n := a.size(g, e)
	w := make([]uint32, (n+3)/4)
	b := unsafe.Slice((*byte)(unsafe.Pointer(&w[0])), n)
	a.unpack(g, e, b)
	return w, b
}

// unpackWords unpacks entry e of group g into the start of buf and returns
// the words it filled.
func (a *archive) unpackWords(g, e int, buf []uint32) []uint32 {
	n := a.size(g, e)
	w := buf[:(n+3)/4]
	a.unpack(g, e, unsafe.Slice((*byte)(unsafe.Pointer(&w[0])), n))
	return w
}

// lzss unpacks src into dst. The packed file starts with its unpacked size
// and a mode byte: mode 0 is the bytes themselves, and the others are
// Okumura's LZSS with a window of 256<<mode bytes and matches of up to
// (256>>mode)+2, the window starting out zeroed and written from its end
// minus the longest match. Each flag byte then governs eight items, a set
// bit for a literal byte and a clear one for a two byte match: the low
// byte of the window position, then the rest of the position in the high
// bits of the second byte over the match length less three in its low
// bits.
func lzss(dst, src []byte) {
	size := int(binary.LittleEndian.Uint32(src))
	mode := uint(src[4])
	src = src[6:]
	if mode == 0 {
		copy(dst[:size], src)
		return
	}
	n := 256 << mode
	longest := 256 >> mode
	mask := n - 1
	lenMask := longest - 1
	window := lzssWindow[:n]
	clear(window)
	r := n - (longest + 2)
	out, in := 0, 0
	flags := 0
	for out < size {
		flags >>= 1
		if flags&0x100 == 0 {
			flags = int(src[in]) | 0xff00
			in++
		}
		if flags&1 != 0 {
			c := src[in]
			in++
			dst[out] = c
			out++
			window[r] = c
			r = (r + 1) & mask
			continue
		}
		b0, b1 := int(src[in]), int(src[in+1])
		in += 2
		pos := b0 | (b1&^lenMask)<<mode
		for k := b1&lenMask + 3; k > 0 && out < size; k-- {
			c := window[pos&mask]
			pos++
			dst[out] = c
			out++
			window[r] = c
			r = (r + 1) & mask
		}
	}
}

// lzssWindow is the unpacker's window, as large as the widest mode, 4,
// makes it.
var lzssWindow [256 << 4]byte
