//go:build yaroze

// Command imgsync is a probe for spec 49: the image calls made with the GPU
// free, each followed at once by DrawSync(1). Each case comes after a
// VSync(0) and a DrawSync(0), between GP1 marker words (10000200h plus a
// number) for a bus trace (PROBE_BUS=start): the marker before case c of
// round r is r*128 + c*4, the one after the call one more, and the one after
// the DrawSync(1) two more. It prints, for each case, what that DrawSync(1)
// gave and how many more polls of it came before a 0. With the drawing time
// on, an image's blocks take the GPU a while, and whether they are still
// going when the call returns is what the first number shows. A second part
// makes the calls behind a drawing that is still going out (queued), and a
// third MoveImage and ClearImage on sizes and places of each kind (others).
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
	pixels [256 * 256]uint16
	back   [256 * 256]uint16
)

var rects = [...]yaroze.RECT{
	{X: 640, Y: 0, W: 16, H: 16},
	{X: 640, Y: 32, W: 16, H: 1},
	{X: 640, Y: 64, W: 20, H: 2},
	{X: 640, Y: 96, W: 0, H: 4},
	{X: 640, Y: 128, W: 64, H: 64},
	{X: 704, Y: 0, W: 256, H: 256},
}

var names = [...]string{"16x16", "16x1", "20x2", "0x4", "64x64", "256x256"}

const (
	sizes     = uint32(len(rects))
	caseLoad  = 0
	caseStore = sizes
	caseMove  = 2 * sizes
	caseClear = 3 * sizes
	caseFill  = 4 * sizes
	cases     = 5 * sizes
)

const rounds = 2

var (
	first [rounds][cases]int32
	polls [rounds][cases]int32
)

//go:noinline
func run(c, n uint32) int32 {
	r := rects[c%sizes]
	switch c / sizes {
	case 0:
		mark(n)
		yaroze.LoadImage(&r, (*uint32)(unsafe.Pointer(&pixels[0])))
		mark(n + 1)
	case 1:
		mark(n)
		yaroze.StoreImage(&r, (*uint32)(unsafe.Pointer(&back[0])))
		mark(n + 1)
	case 2:
		mark(n)
		yaroze.MoveImage(&r, 384, 256)
		mark(n + 1)
	case 3:
		// Not on a multiple of 64: a rectangle.
		r.X++
		mark(n)
		yaroze.ClearImage(&r, 1, 2, 3)
		mark(n + 1)
	case 4:
		// A width of 64 at least, on a multiple of 64: the GPU's fill.
		r.W = max(r.W, 64) &^ 63
		mark(n)
		yaroze.ClearImage(&r, 1, 2, 3)
		mark(n + 1)
	}
	s := yaroze.DrawSync(1)
	mark(n + 2)
	return s
}

// qmark is mark for the queued part: 10000400h plus a number.
func qmark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000400|n)
}

var (
	otTags  [16]yaroze.GsOT_TAG
	ot      = yaroze.GsOT{Length: 4, Org: &otTags[0]}
	packets [1024]uint32
	box     yaroze.GsBOXF
	qsync   [rounds][4]int32
	qpolls  [rounds]int32
)

// queued makes the image calls behind a drawing that is still going out: a
// clear and a semitransparent box over the screen, the box waiting in the
// channel while the GPU clears, then three LoadImage, two StoreImage,
// a MoveImage and two ClearImage, a rectangle and a fill, each between two
// markers (the first before the first LoadImage is round*32, and one more
// after each call), then four DrawSync(1), each between two markers again.
//
//go:noinline
func queued(round uint32) {
	yaroze.VSync(0)
	yaroze.DrawSync(0)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSortClear(0, 0, 0, &ot)
	box = yaroze.GsBOXF{Attribute: 1 << 30, W: 320, H: 240, R: 20, G: 40, B: 90}
	yaroze.GsSortBoxFill(&box, &ot, 1)
	yaroze.GsDrawOt(&ot)
	n := round * 32
	r := rects[0]
	qmark(n)
	yaroze.LoadImage(&r, (*uint32)(unsafe.Pointer(&pixels[0])))
	qmark(n + 1)
	yaroze.LoadImage(&r, (*uint32)(unsafe.Pointer(&pixels[0])))
	qmark(n + 2)
	yaroze.LoadImage(&r, (*uint32)(unsafe.Pointer(&pixels[0])))
	qmark(n + 3)
	yaroze.StoreImage(&r, (*uint32)(unsafe.Pointer(&back[0])))
	qmark(n + 4)
	yaroze.StoreImage(&r, (*uint32)(unsafe.Pointer(&back[0])))
	qmark(n + 5)
	yaroze.MoveImage(&r, 384, 256)
	qmark(n + 6)
	r.X++
	yaroze.ClearImage(&r, 1, 2, 3)
	qmark(n + 7)
	r.X--
	r.W = 64
	yaroze.ClearImage(&r, 1, 2, 3)
	qmark(n + 8)
	for i := range uint32(4) {
		qsync[round][i] = yaroze.DrawSync(1)
		qmark(n + 9 + i)
	}
	p := int32(0)
	for yaroze.DrawSync(1) != 0 {
		p++
	}
	qpolls[round] = p
}

// omark is mark for the other parts: 10000500h plus a number.
func omark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000500|n)
}

// moves are MoveImage's sizes: one of no size does nothing at all on Sony's.
var moves = [...]yaroze.RECT{
	{X: 640, Y: 0, W: 16, H: 16},
	{X: 640, Y: 0, W: 0, H: 4},
	{X: 640, Y: 0, W: 4, H: 0},
	{X: 640, Y: 0, W: 0, H: 0},
	{X: 640, Y: 0, W: -1, H: 4},
	{X: 640, Y: 0, W: 4, H: -1},
	{X: 640, Y: 0, W: 1, H: 1},
}

// clears are ClearImage's cases: x and the width on a multiple of 64 or
// not, and sizes past video memory.
var clears = [...]yaroze.RECT{
	{X: 640, Y: 0, W: 16, H: 16},
	{X: 641, Y: 0, W: 16, H: 16},
	{X: 641, Y: 0, W: 64, H: 16},
	{X: 640, Y: 0, W: 64, H: 16},
	{X: 640, Y: 0, W: 128, H: 16},
	{X: 704, Y: 0, W: 65, H: 16},
	{X: 640, Y: 0, W: 0, H: 16},
	{X: 640, Y: 0, W: 64, H: 0},
	{X: 640, Y: 0, W: -64, H: 16},
	{X: 640, Y: 0, W: 2000, H: 16},
	{X: 640, Y: 0, W: 64, H: 600},
	{X: 0, Y: 0, W: 64, H: 16},
	{X: -64, Y: 0, W: 64, H: 16},
}

var (
	moveRet  [len(moves)]int32
	clearRet [len(clears)]int32
	clearOut [len(clears)]yaroze.RECT
)

// others makes each MoveImage of moves between markers 2k and 2k+1, and
// each ClearImage of clears between 64+2k and 64+2k+1, the GPU free, and
// keeps what each returns and leaves in its rectangle.
//
//go:noinline
func others() {
	for k := range moves {
		yaroze.VSync(0)
		yaroze.DrawSync(0)
		r := moves[k]
		omark(uint32(2 * k))
		moveRet[k] = yaroze.MoveImage(&r, 384, 256)
		omark(uint32(2*k + 1))
	}
	for k := range clears {
		yaroze.VSync(0)
		yaroze.DrawSync(0)
		r := clears[k]
		omark(uint32(64 + 2*k))
		clearRet[k] = yaroze.ClearImage(&r, 1, 2, 3)
		omark(uint32(64 + 2*k + 1))
		clearOut[k] = r
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	println("start")
	for round := range uint32(rounds) {
		for c := range cases {
			yaroze.VSync(0)
			yaroze.DrawSync(0)
			first[round][c] = run(c, round*128+c*4)
			n := int32(0)
			for yaroze.DrawSync(1) != 0 {
				n++
			}
			polls[round][c] = n
		}
		queued(round)
	}
	others()
	yaroze.DrawSync(0)
	kinds := [...]string{"LOAD", "STORE", "MOVE", "CLEAR", "FILL"}
	for round := range rounds {
		for c := range cases {
			println(kinds[c/sizes], names[c%sizes], "round", round, "sync", first[round][c], "polls", polls[round][c])
		}
	}
	for round := range rounds {
		println("QUEUED round", round, "sync", qsync[round][0], qsync[round][1], qsync[round][2], qsync[round][3], "polls", qpolls[round])
	}
	for k := range moves {
		println("MOVEIMAGE", moves[k].W, moves[k].H, "returns", moveRet[k])
	}
	for k := range clears {
		println("CLEARIMAGE", clears[k].X, clears[k].W, clears[k].H, "returns", clearRet[k], "leaves", clearOut[k].W, clearOut[k].H)
	}
	println("END")
}
