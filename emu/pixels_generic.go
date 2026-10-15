//go:build !goexperiment.simd || !(amd64 || wasm)

package emu

// Without SIMD, the pixel operations are their scalar versions.

func rgba15(dst []byte, src []uint16) { rgba15Scalar(dst, src) }

func fill16(dst []uint16, v uint16) { fill16Scalar(dst, v) }
