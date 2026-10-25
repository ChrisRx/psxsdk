//go:build goexperiment.simd && (amd64 || wasm)

package emu

import "simd/archsimd"

// idct is idctScalar a row at a time. Each row of a pass is eight sums of
// products, of a column of the block with the rows of the scale table,
// worked out as two vectors of four. The sums of 16-bit values with the
// table's, divided by 8, fit in 32 bits, and between the passes the
// results are cut to 16 bits as idctScalar stores them.
func idct(blk *[64]int16, scale *[64]int16) {
	if !haveSIMD {
		idctScalar(blk, scale)
		return
	}
	var s, a, b [64]int32
	for i := range s {
		s[i] = int32(scale[i]) / 8
		a[i] = int32(blk[i])
	}
	idctPass(&b, &a, &s)
	idctPass(&a, &b, &s)
	for i := range blk {
		blk[i] = int16(a[i])
	}
}

// idctPass is a pass of idct, from src into dst.
func idctPass(dst, src, s *[64]int32) {
	for y := range 8 {
		var lo, hi archsimd.Int32x4
		for z := range 8 {
			v := archsimd.BroadcastInt32x4(src[y+8*z])
			lo = lo.Add(v.Mul(archsimd.LoadInt32x4(s[8*z : 8*z+4])))
			hi = hi.Add(v.Mul(archsimd.LoadInt32x4(s[8*z+4 : 8*z+8])))
		}
		idctRound(lo).Store(dst[8*y : 8*y+4])
		idctRound(hi).Store(dst[8*y+4 : 8*y+8])
	}
}

// idctRound is idctScalar's (sum + 0xfff) / 0x2000, which rounds toward
// zero, cut to 16 bits: a negative sum has 1FFFh added before the shift.
func idctRound(v archsimd.Int32x4) archsimd.Int32x4 {
	v = v.Add(archsimd.BroadcastInt32x4(0xfff))
	v = v.Add(v.ShiftAllRight(31).And(archsimd.BroadcastInt32x4(0x1fff))).ShiftAllRight(13)
	return v.ShiftAllLeft(16).ShiftAllRight(16)
}
