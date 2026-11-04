package libps

import "go.chrisrx.dev/psx/internal/volatile"

// The resident library's calls take longer than this one's, instruction
// for instruction, and a program's frames keep their place against the
// original's only if the time is spent somewhere (exp/cleanlib/spec/
// 08-timing.md, "Calls through the drawing queue" and "terra's unpacking
// against Sony's library"). pace spends it: n cycles, as near as whole
// turns of its loop allow, measured with the iotrace harness on the
// store interval of the loop (paceTurn) and on spec/probes/drawq
// (paceCall, the call and the return around the loop).

// paceWord is what pace's loop stores to.
var paceWord uint32

const (
	paceTurn = 8  // one turn of pace's loop, in cycles
	paceCall = 22 // the call, the division and the return, in cycles
)

// pace spends about n cycles, the call included.
//
//go:noinline
func pace(n int32) {
	for i := int32(0); i < (n-paceCall+paceTurn/2)/paceTurn; i++ {
		volatile.StoreUint32(&paceWord, uint32(i))
	}
}

// The cycles each call spends beyond its own work, so that it takes as
// long as the resident library's did on spec/probes/drawq (the table in
// spec 08): Sony's time less this library's before pacing. LoadImage's
// words sent through GP0 are paced in gpuDws itself.
const (
	paceLoadImage     = 244 // a 16 by 16 image, all by DMA
	paceStoreImage    = 184 // 8 words
	paceMoveImage     = 158
	paceClearImage    = 268
	paceDrawSync      = 104
	paceGsClearOt     = 162
	paceGsSortClear   = 80
	paceGsDrawOt      = 110
	paceFntPrint      = 58  // an empty string
	paceFntFlush      = 104 // an empty window
	pacePutDrawEnv    = 1304
	paceGsSwapDisp    = 420 // beyond its two PutDrawEnv
	paceGsSetWorkBase = 22
)

// The tick's pads (sndVoiceTick), so that its six words and its return
// fall where Sony's do in the vertical blank handler (spec 08, "terra's
// unpacking against Sony's library").
const (
	paceTickStart = 56
	paceTickWords = 1212
	paceTickEnd   = 32
)
