//go:build !goexperiment.simd || !(amd64 || wasm)

package emu

// Without SIMD, the pixel operations are their scalar versions.

func rgba15(dst []byte, src []uint16) { rgba15Scalar(dst, src) }

func rgb24(dst []byte, src []uint16) int { return rgb24Scalar(dst, src) }

func fill16(dst []uint16, v uint16) { fill16Scalar(dst, v) }

// triangleVec and rectVec leave every triangle and rectangle to be drawn a
// pixel at a time.
func (g *gpu) triangleVec(*raster) bool                  { return false }
func (g *gpu) rectVec(rectData, int, int, int, int) bool { return false }
