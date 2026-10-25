//go:build goexperiment.simd

package emu

import "simd/archsimd"

// Every wasm runtime that runs SIMD code has all of SIMD128.
const haveSIMD = true

// widen zero-extends eight 16-bit lanes to two vectors of 32-bit lanes.
func widen(v archsimd.Uint16x8) (lo, hi archsimd.Uint32x4) {
	return v.ExtendLo4ToUint32(), v.ExtendHi4ToUint32()
}

// permute picks the bytes of v by index, where an index of -1 picks zero.
func permute(v archsimd.Uint8x16, index archsimd.Int8x16) archsimd.Uint8x16 {
	return v.BitsToInt8().LookupOrZero(index).ToBits()
}
