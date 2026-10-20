package archive

import "unsafe"

// Inflate unpacks a raw deflate stream (RFC 1951) from src into dst,
// which must be the unpacked size, and returns the bytes written, or -1
// for a stream it cannot read. It allocates nothing and keeps its tables
// in one package-level state, so it is not for two goroutines at once.
//
// The stream is read a symbol at a time through a table of the codes up
// to eleven bits, the rest by the canonical walk, and copied a byte at a
// time without bounds checks past the ones made once a run: on the
// console, which has no data cache, those checks were half the time.
func Inflate(dst, src []byte) int {
	f := &state
	f.src, f.pos, f.bits, f.nbits = src, 0, 0, 0
	n := f.run(dst)
	f.src = nil
	return n
}

var state inflater

// fastBits is how many bits the lookup table takes at once.
const fastBits = 11

// The decoding tables of a block: a lookup of the first fastBits bits
// for the codes that short, and the canonical counts and symbols for the
// rest.
type huff struct {
	fast  [1 << fastBits]uint16 // symbol<<4 | length, or 0 for a longer code
	count [16]uint16
	sym   [288]uint16
}

type inflater struct {
	src   []byte
	pos   int
	bits  uint32
	nbits uint
	lit   huff
	dist  huff
	lens  [320]uint8
}

// byteAt is the byte at i of a slice's memory, unchecked.
func byteAt(b []byte, i int) byte {
	return *(*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(b)), i))
}

func setByte(b []byte, i int, v byte) {
	*(*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(b)), i)) = v
}

// need makes sure n bits are buffered, reading zeros past the end.
func (f *inflater) need(n uint) {
	for f.nbits < n {
		if f.pos < len(f.src) {
			f.bits |= uint32(byteAt(f.src, f.pos)) << f.nbits
			f.pos++
		}
		f.nbits += 8
	}
}

func (f *inflater) get(n uint) uint32 {
	f.need(n)
	v := f.bits & (1<<n - 1)
	f.bits >>= n
	f.nbits -= n
	return v
}

// build makes the tables for symbols 0..n-1 with the code lengths given.
func (h *huff) build(lens []uint8, n int) {
	for i := range h.count {
		h.count[i] = 0
	}
	for i := range h.fast {
		h.fast[i] = 0
	}
	for i := 0; i < n; i++ {
		h.count[lens[i]]++
	}
	h.count[0] = 0
	var offs [16]uint16
	var next [16]uint32
	code := uint32(0)
	for l := 1; l < 16; l++ {
		offs[l] = offs[l-1] + h.count[l-1]
		code = (code + uint32(h.count[l-1])) << 1
		next[l] = code
	}
	for i := 0; i < n; i++ {
		l := lens[i]
		if l == 0 {
			continue
		}
		h.sym[offs[l]] = uint16(i)
		offs[l]++
		c := next[l]
		next[l]++
		if l <= fastBits {
			// Reversed, as the stream is read least significant bit first,
			// at every entry that starts with it.
			r := uint32(0)
			for k := uint8(0); k < l; k++ {
				r = r<<1 | (c>>k)&1
			}
			for e := r; e < 1<<fastBits; e += 1 << l {
				h.fast[e] = uint16(i)<<4 | uint16(l)
			}
		}
	}
}

// decode reads one symbol with the tables.
func (f *inflater) decode(h *huff) int {
	f.need(fastBits)
	if e := h.fast[f.bits&(1<<fastBits-1)]; e != 0 {
		l := uint(e & 15)
		f.bits >>= l
		f.nbits -= l
		return int(e >> 4)
	}
	// A longer code: the canonical walk, a bit at a time.
	code, first, index := 0, 0, 0
	for l := 1; l < 16; l++ {
		f.need(1)
		code |= int(f.bits & 1)
		f.bits >>= 1
		f.nbits--
		count := int(h.count[l])
		if code-count < first {
			return int(h.sym[index+(code-first)])
		}
		index += count
		first += count
		first <<= 1
		code <<= 1
	}
	return -1
}

var lbase = [29]uint16{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258}
var lext = [29]uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}
var dbase = [30]uint16{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577}
var dext = [30]uint8{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13}
var order = [19]uint8{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

func (f *inflater) run(dst []byte) int {
	out := 0
	for {
		last := f.get(1)
		switch f.get(2) {
		case 0:
			// Stored: to the byte, then the length and its complement.
			drop := f.nbits & 7
			f.bits >>= drop
			f.nbits -= drop
			n := int(f.get(16))
			if int(f.get(16)) != ^n&0xffff {
				return -1
			}
			// Whole bytes may be buffered; give them back.
			f.pos -= int(f.nbits / 8)
			f.bits, f.nbits = 0, 0
			if f.pos+n > len(f.src) || out+n > len(dst) {
				return -1
			}
			copy(dst[out:out+n], f.src[f.pos:f.pos+n])
			f.pos += n
			out += n
		case 1:
			for i := 0; i < 144; i++ {
				f.lens[i] = 8
			}
			for i := 144; i < 256; i++ {
				f.lens[i] = 9
			}
			for i := 256; i < 280; i++ {
				f.lens[i] = 7
			}
			for i := 280; i < 288; i++ {
				f.lens[i] = 8
			}
			f.lit.build(f.lens[:], 288)
			for i := 0; i < 30; i++ {
				f.lens[i] = 5
			}
			f.dist.build(f.lens[:], 30)
			if out = f.codes(dst, out); out < 0 {
				return -1
			}
		case 2:
			nlen := int(f.get(5)) + 257
			ndist := int(f.get(5)) + 1
			ncode := int(f.get(4)) + 4
			for i := range f.lens {
				f.lens[i] = 0
			}
			for i := 0; i < ncode; i++ {
				f.lens[order[i]] = uint8(f.get(3))
			}
			f.lit.build(f.lens[:], 19)
			for i := 0; i < nlen+ndist; {
				s := f.decode(&f.lit)
				switch {
				case s < 0:
					return -1
				case s < 16:
					f.lens[i] = uint8(s)
					i++
				case s == 16:
					if i == 0 {
						return -1
					}
					prev := f.lens[i-1]
					for k := 3 + int(f.get(2)); k > 0 && i < len(f.lens); k-- {
						f.lens[i] = prev
						i++
					}
				case s == 17:
					for k := 3 + int(f.get(3)); k > 0 && i < len(f.lens); k-- {
						f.lens[i] = 0
						i++
					}
				default:
					for k := 11 + int(f.get(7)); k > 0 && i < len(f.lens); k-- {
						f.lens[i] = 0
						i++
					}
				}
			}
			f.lit.build(f.lens[:nlen], nlen)
			f.dist.build(f.lens[nlen:nlen+ndist], ndist)
			if out = f.codes(dst, out); out < 0 {
				return -1
			}
		default:
			return -1
		}
		if last == 1 {
			return out
		}
	}
}

// codes decodes a block's literals and matches into dst from out.
func (f *inflater) codes(dst []byte, out int) int {
	for {
		s := f.decode(&f.lit)
		switch {
		case s < 0:
			return -1
		case s < 256:
			if out >= len(dst) {
				return -1
			}
			setByte(dst, out, byte(s))
			out++
		case s == 256:
			return out
		default:
			s -= 257
			if s >= 29 {
				return -1
			}
			length := int(lbase[s]) + int(f.get(uint(lext[s])))
			d := f.decode(&f.dist)
			if d < 0 || d >= 30 {
				return -1
			}
			dist := int(dbase[d]) + int(f.get(uint(dext[d])))
			if dist > out || out+length > len(dst) {
				return -1
			}
			from := out - dist
			for i := 0; i < length; i++ {
				setByte(dst, out, byteAt(dst, from))
				out++
				from++
			}
		}
	}
}
