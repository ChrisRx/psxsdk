//go:build goexperiment.simd && (amd64 || wasm)

package emu

import (
	"simd/archsimd"
	"unsafe"
)

// rgba15 converts 15-bit pixels to RGBA as rgba15Scalar does, eight at a
// time: each half of eight pixels is widened to 32-bit lanes, where the
// components are moved apart as rgba15Pixel moves them. The lanes are
// stored as words, which both architectures store little-endian; dst must
// be aligned for them, as each row of an image.RGBA is.
func rgba15(dst []byte, src []uint16) {
	if !haveSIMD || len(src) < 8 {
		rgba15Scalar(dst, src)
		return
	}
	dst = dst[:4*len(src)]
	words := unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(dst))), len(src))
	var (
		r = archsimd.BroadcastUint32x4(0x1f)
		g = archsimd.BroadcastUint32x4(0x3e0)
		b = archsimd.BroadcastUint32x4(0x7c00)
		a = archsimd.BroadcastUint32x4(0xff000000)
	)
	conv := func(p archsimd.Uint32x4, out []uint32) {
		p.And(r).ShiftAllLeft(3).
			Or(p.And(g).ShiftAllLeft(6)).
			Or(p.And(b).ShiftAllLeft(9)).
			Or(a).Store(out)
	}
	i := 0
	for ; i+8 <= len(src); i += 8 {
		lo, hi := widen(archsimd.LoadUint16x8(src[i : i+8]))
		conv(lo, words[i:i+4])
		conv(hi, words[i+4:i+8])
	}
	rgba15Scalar(dst[4*i:], src[i:])
}

// fill16 sets every pixel of dst to v, as fill16Scalar does, eight at a
// time.
func fill16(dst []uint16, v uint16) {
	if !haveSIMD || len(dst) < 8 {
		fill16Scalar(dst, v)
		return
	}
	vv := archsimd.BroadcastUint16x8(v)
	i := 0
	for ; i+8 <= len(dst); i += 8 {
		vv.Store(dst[i : i+8])
	}
	// The tail overlaps the last full vector rather than going a pixel at
	// a time.
	if i < len(dst) {
		vv.Store(dst[len(dst)-8:])
	}
}
