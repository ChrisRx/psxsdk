//go:build yaroze

// Command gscost is a probe for spec 19: what the coordinate calls cost in CPU
// cycles, from root counter 2 in system clock mode, the minimum of five runs with an
// empty call taken off. A case's prep function runs before every timed call, outside
// the time.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern PSDCNT
var psdCNT uint32

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))

	chain [64]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
	ls    yaroze.MATRIX
	comp  yaroze.MATRIX
	v2    yaroze.GsVIEW2
	rv    yaroze.GsRVIEW2
	base  int32
)

//go:noinline
func empty() {}

func cost(prep, call func()) int32 {
	best := uint32(0xffffffff)
	for range 6 {
		prep()
		volatile.StoreUint32(mode, 0)
		t0 := volatile.LoadUint32(count) & 0xffff
		call()
		t1 := volatile.LoadUint32(count) & 0xffff
		if d := (t1 - t0) & 0xffff; d < best {
			best = d
		}
	}
	return int32(best) - base
}

// build makes a chain of d, flag f for each and a real rotation in each matrix.
func build(d int, f func(int) uint32) *yaroze.GsCOORDINATE2 {
	var up *yaroze.GsCOORDINATE2
	for i := 0; i < d; i++ {
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord = yaroze.MATRIX{
			M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}},
			T: [3]int32{int32(100 * (i + 1)), 200, 300},
		}
		chain[i].Workm = chain[i].Coord
		chain[i].Flg = f(i)
		up = &chain[i]
	}
	return up
}

func zero(int) uint32  { return 0 }
func cur(int) uint32   { return psdCNT }
func other(int) uint32 { return psdCNT + 1 }

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	base = 0
	base = cost(func() {}, empty)
	println("empty", base)
	yaroze.EnterCriticalSection()
	var c *yaroze.GsCOORDINATE2
	for _, d := range [...]int{1, 2, 3, 4, 5, 6, 8, 12, 16, 32, 64} {
		lwOf := func() { yaroze.GsGetLw(c, &lw) }
		lsOf := func() { yaroze.GsGetLs(c, &ls) }
		lwsOf := func() { yaroze.GsGetLws(c, &lw, &ls) }
		print("d ", d)
		c = build(d, zero)
		print(" Lw0 ", cost(func() { c = build(d, zero) }, lwOf))
		print(" Ls0 ", cost(func() { c = build(d, zero) }, lsOf))
		print(" Lws0 ", cost(func() { c = build(d, zero) }, lwsOf))
		print(" LwC ", cost(func() { c = build(d, cur) }, lwOf))
		print(" LsC ", cost(func() { c = build(d, cur) }, lsOf))
		print(" LwsC ", cost(func() { c = build(d, cur) }, lwsOf))
		print(" Lwx ", cost(func() { c = build(d, other) }, lwOf))
		print(" Lsx ", cost(func() { c = build(d, other) }, lsOf))
		println(" Lwsx ", cost(func() { c = build(d, other) }, lwsOf))
	}
	// One stale step under a current parent, deep in a chain: the walk's stop.
	for _, d := range [...]int{2, 3, 4, 8, 16} {
		last := d - 1
		f := func(i int) uint32 {
			if i == last-1 {
				return psdCNT
			}
			return 0
		}
		print("under C d ", d)
		println(" Lw ", cost(func() { c = build(d, f) }, func() { yaroze.GsGetLw(c, &lw) }))
	}
	// One target with a stale step only, its parents all x.
	for _, d := range [...]int{2, 3, 4, 8, 16} {
		last := d - 1
		f := func(i int) uint32 {
			if i == last {
				return 0
			}
			return psdCNT + 1
		}
		print("under x d ", d)
		println(" Lw ", cost(func() { c = build(d, f) }, func() { yaroze.GsGetLw(c, &lw) }))
	}
	a, b := chain[0].Coord, chain[1].Coord
	println("CompMatrix", cost(func() {}, func() { yaroze.CompMatrix(&a, &b, &comp) }))
	println("InitCoordinate2 nil", cost(func() {}, func() { yaroze.GsInitCoordinate2(nil, &chain[0]) }))
	println("InitCoordinate2 up", cost(func() {}, func() { yaroze.GsInitCoordinate2(&chain[0], &chain[1]) }))
	v2.View = a
	v2.Super = nil
	println("View world", cost(func() {}, func() { yaroze.GsSetView2(&v2) }))
	for _, d := range [...]int{1, 2, 4, 8} {
		print("View d ", d)
		println(" ", cost(func() { v2.Super = build(d, zero) }, func() { yaroze.GsSetView2(&v2) }))
	}
	rv = yaroze.GsRVIEW2{Vpx: 100, Vpy: 200, Vpz: -1000, Vrx: 10, Vry: 20, Vrz: 30}
	println("RefView world", cost(func() {}, func() { yaroze.GsSetRefView2(&rv) }))
	for _, d := range [...]int{1, 2, 4, 8} {
		print("RefView d ", d)
		println(" ", cost(func() { rv.Super = build(d, zero) }, func() { yaroze.GsSetRefView2(&rv) }))
	}
	yaroze.ExitCriticalSection()
	println("END")
	for {
	}
}
