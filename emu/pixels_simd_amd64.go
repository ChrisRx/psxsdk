//go:build goexperiment.simd

package emu

import "simd/archsimd"

// The 128-bit operations rgba15 uses are AVX's.
var haveSIMD = archsimd.X86.AVX()

// widen zero-extends eight 16-bit lanes to two vectors of 32-bit lanes, by
// interleaving them with zeros.
func widen(v archsimd.Uint16x8) (lo, hi archsimd.Uint32x4) {
	var zero archsimd.Uint16x8
	return v.InterleaveLo(zero).AsUint32x4(), v.InterleaveHi(zero).AsUint32x4()
}
