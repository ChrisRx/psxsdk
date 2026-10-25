package emu

import "encoding/binary"

// rgba15Scalar converts 15-bit pixels to RGBA, 4 bytes to a pixel in dst,
// widening each 5-bit component to 8 bits by shifting it up, with full
// alpha. As a little-endian word, each pixel is the components moved apart.
func rgba15Scalar(dst []byte, src []uint16) {
	dst = dst[:4*len(src)]
	for i, p := range src {
		binary.LittleEndian.PutUint32(dst[4*i:], rgba15Pixel(uint32(p)))
	}
}

func rgba15Pixel(p uint32) uint32 {
	return p&0x1f<<3 | p&0x3e0<<6 | p&0x7c00<<9 | 0xff000000
}

// rgb24Scalar converts pixels of three bytes, red, green and blue, packed
// two bytes to a halfword from the start of src, to RGBA, 4 bytes to a
// pixel in dst, with full alpha. It converts as many as both have room for,
// and returns how many that is.
func rgb24Scalar(dst []byte, src []uint16) int {
	n := min(len(dst)/4, 2*len(src)/3)
	for x := range n {
		for i := range 3 {
			s := 3*x + i
			dst[4*x+i] = byte(src[s/2] >> (8 * (s & 1)))
		}
		dst[4*x+3] = 0xff
	}
	return n
}

// fill16Scalar sets every pixel of dst to v.
func fill16Scalar(dst []uint16, v uint16) {
	for i := range dst {
		dst[i] = v
	}
}
