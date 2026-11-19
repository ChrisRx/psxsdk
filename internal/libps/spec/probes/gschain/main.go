//go:build yaroze

// Command gschain is a probe for spec 15: how much stack GsGetLw, GsGetLs and
// GsGetLws take on a coordinate chain of each depth, a few other large 3D and sound
// calls beside them, and what the flag of a GsCOORDINATE2 does: which systems are
// worked out again and what the call leaves in each flag and Workm. An original
// program keeps its stack in the 1 KB scratchpad, so the stack figures matter. The
// stack below sp is filled with a pattern before the call and scanned after it, so a
// line's figure holds the probe's own small frame on both images alike.
package main

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const span = 4096

var (
	chain [17]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
	ls    yaroze.MATRIX
	call  func()
	sink  int32
	rv    yaroze.GsRVIEW2
	v2    yaroze.GsVIEW2
)

//go:noinline
func reach() int32 {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	low := sp - span
	most := int32(0)
	for _, pat := range [...]uint32{0x5aa5c33c, 0xa55a3cc3} {
		for a := low; a < sp-8; a += 4 {
			*(*uint32)(unsafe.Pointer(uintptr(a))) = pat
		}
		call()
		a := low
		for a < sp-8 && *(*uint32)(unsafe.Pointer(uintptr(a))) == pat {
			a += 4
		}
		if d := int32(sp - a); d > most {
			most = d
		}
	}
	return most
}

// build makes a chain of depth d, with a step of translation in each matrix, and
// each flag as given.
func build(d int, flg uint32) *yaroze.GsCOORDINATE2 {
	for i := 0; i < d; i++ {
		var up *yaroze.GsCOORDINATE2
		if i > 0 {
			up = &chain[i-1]
		}
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord.T = [3]int32{int32(i + 1), int32(10 * (i + 1)), int32(100 * (i + 1))}
		chain[i].Flg = flg
		chain[i].Workm.T = [3]int32{-1, -1, -1}
	}
	return &chain[d-1]
}

func state(d int) {
	for i := 0; i < d; i++ {
		println("  node", i, "flg", chain[i].Flg, "work", chain[i].Workm.T[0], chain[i].Workm.T[1], chain[i].Workm.T[2])
	}
	println("  lw", lw.T[0], lw.T[1], lw.T[2], "ls", ls.T[0], ls.T[1], ls.T[2])
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInit3D()
	for _, d := range [...]int{1, 2, 3, 4, 6, 8, 12, 16} {
		for _, flg := range [...]uint32{0, 1} {
			c := build(d, flg)
			call = func() { yaroze.GsGetLw(c, &lw) }
			println("stack GsGetLw depth", d, "flg", flg, reach())
			build(d, flg)
			call = func() { yaroze.GsGetLs(c, &ls) }
			println("stack GsGetLs depth", d, "flg", flg, reach())
			build(d, flg)
			call = func() { yaroze.GsGetLws(c, &lw, &ls) }
			println("stack GsGetLws depth", d, "flg", flg, reach())
		}
	}

	// The flags: every pattern over a chain of three, by the call.
	for pat := uint32(0); pat < 8; pat++ {
		c := build(3, 0)
		for i := range 3 {
			chain[i].Flg = pat >> i & 1
		}
		lw, ls = yaroze.MATRIX{}, yaroze.MATRIX{}
		yaroze.GsGetLw(c, &lw)
		println("GsGetLw flags", pat)
		state(3)
		build(3, 0)
		for i := range 3 {
			chain[i].Flg = pat >> i & 1
		}
		lw, ls = yaroze.MATRIX{}, yaroze.MATRIX{}
		yaroze.GsGetLws(c, &lw, &ls)
		println("GsGetLws flags", pat)
		state(3)
	}

	// A second call after a change to a matrix, with nothing cleared and with the
	// flag cleared.
	c := build(3, 0)
	yaroze.GsGetLw(c, &lw)
	chain[1].Coord.T[0] = 1000
	yaroze.GsGetLw(c, &lw)
	println("changed, flag left")
	state(3)
	chain[1].Flg = 0
	yaroze.GsGetLw(c, &lw)
	println("changed, flag cleared")
	state(3)

	// Other large calls.
	v2 = yaroze.GsVIEW2{}
	call = func() { sink = yaroze.GsSetView2(&v2) }
	println("stack GsSetView2 world", reach())
	v2.Super = build(4, 0)
	call = func() { sink = yaroze.GsSetView2(&v2) }
	println("stack GsSetView2 depth 4", reach())
	rv = yaroze.GsRVIEW2{Vpz: -1000}
	call = func() { sink = yaroze.GsSetRefView2(&rv) }
	println("stack GsSetRefView2 world", reach())
	rv.Super = build(4, 0)
	call = func() { sink = yaroze.GsSetRefView2(&rv) }
	println("stack GsSetRefView2 depth 4", reach())
	call = func() { yaroze.CompMatrix(&lw, &ls, &lw) }
	println("stack CompMatrix", reach())
	call = func() { yaroze.GsSetLsMatrix(&lw) }
	println("stack GsSetLsMatrix", reach())
	call = func() { yaroze.GsSetLightMatrix(&lw) }
	println("stack GsSetLightMatrix", reach())
	println("END")
	for {
	}
}
