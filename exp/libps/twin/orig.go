//go:build yaroze

package main

import "go.chrisrx.dev/psx/yaroze"

// The originals, under the _orig names playstation-yaroze-symusr-orig.S
// gives the resident library's entry points, with the bindings'
// signatures. The plain names are the Go library's in this program.

//export rand_orig
func rand_orig() int32

//export srand_orig
func srand_orig(seed uint32)

//export GetTPage_orig
func GetTPage_orig(tp, abr, x, y int32) uint16

//export GetVideoMode_orig
func GetVideoMode_orig() int32

//export SetVideoMode_orig
func SetVideoMode_orig(mode int32) int32

//export ResetGraph_orig
func ResetGraph_orig(mode int32) int32

//export DrawSync_orig
func DrawSync_orig(mode int32) int32

//export LoadImage_orig
func LoadImage_orig(rect *yaroze.RECT, p *uint32) int32

//export VSync_orig
func VSync_orig(mode int32) int32

//export GetPadBuf_orig
func GetPadBuf_orig(a0, a1 **uint8)

//export VSyncCallback_orig
func VSyncCallback_orig(f uintptr)
