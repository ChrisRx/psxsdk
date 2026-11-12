//go:build yaroze

// Command qstate is a probe for the spec: the calls that go through the
// drawing queue, each made in two states of the queue, between two GP1
// marker words (10000200h plus a number), so that a trace shows whether a
// call's time depends on the state. In state 0 a DrawSync(0) came last, so
// nothing is queued. In state 1 a PutDrawEnv came last and nothing has
// looked at the queue since: its request has gone out but no call has seen
// that yet, as when a frame loop's FntFlush follows GsSwapDispBuff (spec 47).
// The marker before case c in state s of round r is r*128 + (c*2+s)*2, the
// one after it one more.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000200|n)
}

var (
	pixels [16 * 16]uint16
	back   [16 * 16]uint16
	work   [256]uint32
	tags   [16]yaroze.GsOT_TAG
	ot     = yaroze.GsOT{Length: 4, Org: &tags[0]}
	img    = yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
	wide   = yaroze.RECT{X: 640, Y: 0, W: 64, H: 16}
	pal    = yaroze.RECT{X: 640, Y: 32, W: 16, H: 1}
	mixed  = yaroze.RECT{X: 640, Y: 64, W: 20, H: 2}
	none   = yaroze.RECT{X: 640, Y: 96, W: 0, H: 4}
)

// The cases.
const (
	caseDrawOt = iota
	caseDrawSync
	caseFlushDefault
	caseFlushZero
	casePutDrawEnv
	caseLoadImage
	caseSwap
	caseFlushText
	caseStoreImage
	caseMoveImage
	caseClearImage
	caseClearFill
	caseFlushZeroText
	caseLoadWords
	caseLoadMixed
	caseStoreWords
	caseStoreMixed
	caseLoadNone
	caseStoreNone
	cases
)

//go:noinline
func run(c, n uint32) {
	switch c {
	case caseDrawOt:
		mark(n)
		yaroze.GsDrawOt(&ot)
		mark(n + 1)
	case caseDrawSync:
		mark(n)
		yaroze.DrawSync(0)
		mark(n + 1)
	case caseFlushDefault, caseFlushText:
		mark(n)
		yaroze.FntFlush(-1)
		mark(n + 1)
	case caseClearFill:
		mark(n)
		yaroze.ClearImage(&wide, 1, 2, 3)
		mark(n + 1)
	case caseFlushZero, caseFlushZeroText:
		mark(n)
		yaroze.FntFlush(0)
		mark(n + 1)
	case casePutDrawEnv:
		mark(n)
		yaroze.PutDrawEnv(yaroze.GsDRAWENV())
		mark(n + 1)
	case caseLoadImage:
		mark(n)
		yaroze.LoadImage(&img, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
	case caseLoadWords:
		mark(n)
		yaroze.LoadImage(&pal, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
	case caseLoadMixed:
		mark(n)
		yaroze.LoadImage(&mixed, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
	case caseStoreWords:
		mark(n)
		yaroze.StoreImage(&pal, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 1)
	case caseStoreMixed:
		mark(n)
		yaroze.StoreImage(&mixed, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 1)
	case caseLoadNone:
		mark(n)
		yaroze.LoadImage(&none, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
	case caseStoreNone:
		mark(n)
		yaroze.StoreImage(&none, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 1)
	case caseSwap:
		mark(n)
		yaroze.GsSwapDispBuff()
		mark(n + 1)
	case caseStoreImage:
		mark(n)
		yaroze.StoreImage(&img, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 1)
	case caseMoveImage:
		mark(n)
		yaroze.MoveImage(&img, 704, 0)
		mark(n + 1)
	case caseClearImage:
		mark(n)
		yaroze.ClearImage(&img, 1, 2, 3)
		mark(n + 1)
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 64, 0, 512)
	yaroze.DrawSync(0)
	println("start")
	for round := range uint32(2) {
		yaroze.VSync(0)
		for c := range uint32(cases) {
			for s := range uint32(2) {
				yaroze.DrawSync(0)
				yaroze.GsSetWorkBase(yaroze.Packets(&work[0]))
				yaroze.GsClearOt(0, 0, &ot)
				if c == caseFlushText || c == caseFlushZeroText {
					yaroze.Print("hi")
				}
				yaroze.DrawSync(0)
				if s == 1 {
					yaroze.PutDrawEnv(yaroze.GsDRAWENV())
				}
				run(c, round*128+(c*2+s)*2)
			}
		}
	}
	yaroze.DrawSync(0)
	println("END")
	for {
		yaroze.VSync(0)
	}
}
