//go:build yaroze

// Command gslong is a probe for spec 19: long coordinate chains. Each depth is built
// with every flag 0 and every step 1 in x, so GsGetLw should give the depth in x and
// set every flag. The stack at two depths, then a vertical blank callback that looks at
// the chain's Super and Sub fields while long calls run, and last the first depth that
// fails.
package main

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern PSDCNT
var psdCNT uint32

const most = 1200

var (
	chain  [most]yaroze.GsCOORDINATE2
	lw     yaroze.MATRIX
	seen   int
	bad    int
	depth  int
	active bool
)

// look runs at each vertical blank and notes whether the chain looks whole.
func look() {
	if !active {
		return
	}
	seen++
	for i := 1; i < depth; i++ {
		if chain[i].Super != &chain[i-1] || chain[i-1].Sub != &chain[i] {
			bad++
			return
		}
	}
}

//go:noinline
func reach(c *yaroze.GsCOORDINATE2) int32 {
	sp := uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	low := sp - 4096
	for a := low; a < sp-8; a += 4 {
		*(*uint32)(unsafe.Pointer(uintptr(a))) = 0x5aa5c33c
	}
	yaroze.GsGetLw(c, &lw)
	a := low
	for a < sp-8 && *(*uint32)(unsafe.Pointer(uintptr(a))) == 0x5aa5c33c {
		a += 4
	}
	return int32(sp - a)
}

func build(d int, flag uint32) {
	var up *yaroze.GsCOORDINATE2
	for i := 0; i < d; i++ {
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord.T = [3]int32{1, 0, 0}
		chain[i].Flg = flag
		chain[i].Workm = yaroze.MATRIX{T: [3]int32{-1, -1, -1}}
		up = &chain[i]
	}
}

func report(what string, d int) {
	fresh := 0
	first, wrong := -1, -1
	for i := 0; i < d; i++ {
		if chain[i].Flg == psdCNT {
			fresh++
		} else if first < 0 {
			first = i
		}
		if chain[i].Workm.T[0] != int32(i+1) && wrong < 0 {
			wrong = i
		}
	}
	println(what, "depth", d, "lw", lw.T[0], "fresh", fresh, "first unfresh", first, "first wrong workm", wrong)
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	for _, d := range [...]int{1, 8, 15, 16, 17, 18, 24, 31, 32, 33, 34, 48, 63, 64, 65, 66, 100, 127, 128, 129, 130, 200, 255, 256, 257, 258, 300, 400, 511, 512, 513, 600} {
		build(d, 0)
		lw = yaroze.MATRIX{}
		yaroze.GsGetLw(&chain[d-1], &lw)
		report("Lw", d)
	}
	// The stack at the depths, every flag 0.
	for _, d := range [...]int{16, 600} {
		build(d, 0)
		println("stack depth", d, reach(&chain[d-1]))
	}
	// The chain as seen from an interrupt during calls that span blanks.
	depth = 600
	build(depth, 0)
	f := look
	yaroze.VSyncCallback((*[2]uintptr)(unsafe.Pointer(&f))[1])
	yaroze.VSync(0)
	active = true
	for range 30 {
		for i := range depth {
			chain[i].Flg = 0
		}
		yaroze.GsGetLw(&chain[depth-1], &lw)
	}
	active = false
	yaroze.VSyncCallback(0)
	println("callback saw", seen, "with a broken chain", bad, "lw", lw.T[0])
	// The first depth that goes wrong, counting up from a depth that works. What is
	// past it writes where it should not, so nothing after it can be trusted.
	for d := 601; d < 1200; d++ {
		build(d, 0)
		lw = yaroze.MATRIX{}
		yaroze.GsGetLw(&chain[d-1], &lw)
		fresh := 0
		for i := 0; i < d; i++ {
			if chain[i].Flg == psdCNT {
				fresh++
			}
		}
		if lw.T[0] != int32(d) || fresh != d {
			report("first wrong", d)
			for i := 0; i < d; i++ {
				if chain[i].Flg != psdCNT {
					println("  unfresh node", i, "from target", d-1-i)
				}
			}
			break
		}
	}
	println("END")
	for {
	}
}
