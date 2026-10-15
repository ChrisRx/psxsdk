//go:build goexperiment.simd

package emu

import "simd/archsimd"

// Every wasm runtime that runs SIMD code has all of SIMD128.
const haveSIMD = true

// widen zero-extends eight 16-bit lanes to two vectors of 32-bit lanes.
func widen(v archsimd.Uint16x8) (lo, hi archsimd.Uint32x4) {
	return v.ExtendLo4ToUint32(), v.ExtendHi4ToUint32()
}
