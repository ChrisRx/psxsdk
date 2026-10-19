// Package lz4 is the LZ4 block format, decoder and a greedy encoder, kept
// for the compression benchmark as the point of comparison the archive's
// deflate was chosen against. It is not what the toolchain ships.
package lz4

import "unsafe"

func byteAt(b []byte, i int) byte {
	return *(*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(b)), i))
}

func setByte(b []byte, i int, v byte) {
	*(*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(b)), i)) = v
}

// Decode unpacks an LZ4 block from src into dst, which must be the
// unpacked size, and returns the bytes written, or -1.
func Decode(dst, src []byte) int {
	in, out := 0, 0
	for in < len(src) {
		token := byteAt(src, in)
		in++
		lit := int(token >> 4)
		if lit == 15 {
			for {
				b := byteAt(src, in)
				in++
				lit += int(b)
				if b != 255 {
					break
				}
			}
		}
		if in+lit > len(src) || out+lit > len(dst) {
			return -1
		}
		copy(dst[out:out+lit], src[in:in+lit])
		in += lit
		out += lit
		if in >= len(src) {
			break
		}
		off := int(byteAt(src, in)) | int(byteAt(src, in+1))<<8
		in += 2
		ml := int(token&15) + 4
		if ml == 19 {
			for {
				b := byteAt(src, in)
				in++
				ml += int(b)
				if b != 255 {
					break
				}
			}
		}
		if off == 0 || off > out || out+ml > len(dst) {
			return -1
		}
		ref := out - off
		for i := 0; i < ml; i++ {
			setByte(dst, out, byteAt(dst, ref))
			out++
			ref++
		}
	}
	return out
}

// Encode packs src as an LZ4 block, greedily: the first match of four or
// more bytes found through a hash of four, within 64 KB. The real lz4hc
// finds a few percent more.
func Encode(src []byte) []byte {
	const minMatch = 4
	var table [1 << 16]int32
	for i := range table {
		table[i] = -1
	}
	hash := func(i int) uint32 {
		v := uint32(src[i]) | uint32(src[i+1])<<8 | uint32(src[i+2])<<16 | uint32(src[i+3])<<24
		return (v * 2654435761) >> 16
	}
	out := make([]byte, 0, len(src)+len(src)/255+16)
	ext := func(n int) {
		for ; ; n -= 255 {
			if n >= 255 {
				out = append(out, 255)
			} else {
				out = append(out, byte(n))
				return
			}
		}
	}
	emit := func(lits []byte, ml int, off int) {
		ll := len(lits)
		token := byte(min(ll, 15)) << 4
		if ml > 0 {
			token |= byte(min(ml-minMatch, 15))
		}
		out = append(out, token)
		if ll >= 15 {
			ext(ll - 15)
		}
		out = append(out, lits...)
		if ml > 0 {
			out = append(out, byte(off), byte(off>>8))
			if ml-minMatch >= 15 {
				ext(ml - minMatch - 15)
			}
		}
	}
	n := len(src)
	anchor := 0
	i := 0
	limit := n - 12 // a match starts at least 12 bytes before the end
	for i < limit {
		h := hash(i)
		cand := int(table[h])
		table[h] = int32(i)
		if cand < 0 || i-cand > 65535 || src[cand] != src[i] || src[cand+1] != src[i+1] || src[cand+2] != src[i+2] || src[cand+3] != src[i+3] {
			i++
			continue
		}
		ml := minMatch
		for i+ml < n-5 && src[cand+ml] == src[i+ml] {
			ml++
		}
		emit(src[anchor:i], ml, i-cand)
		i += ml
		anchor = i
	}
	emit(src[anchor:], 0, 0)
	return out
}
