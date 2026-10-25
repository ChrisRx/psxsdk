//go:build goexperiment.simd

package emu

import "simd/archsimd"

// The 128-bit operations the pixel and drawing code use are AVX's.
var haveSIMD = archsimd.X86.AVX()

// widen zero-extends eight 16-bit lanes to two vectors of 32-bit lanes, by
// interleaving them with zeros.
func widen(v archsimd.Uint16x8) (lo, hi archsimd.Uint32x4) {
	var zero archsimd.Uint16x8
	return v.InterleaveLo(zero).AsUint32x4(), v.InterleaveHi(zero).AsUint32x4()
}

// permute picks the bytes of v by index, where an index of -1 picks zero.
func permute(v archsimd.Uint8x16, index archsimd.Int8x16) archsimd.Uint8x16 {
	return v.PermuteOrZero(index)
}
