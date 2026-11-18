//go:build yaroze

// listcut asks what a console's GPU does when channel 2 is stopped partway
// through a drawing list, as ResetGraph(1) does (spec 43). It sends a
// linked list of small Gouraud quads, eight words a command, stops the
// channel after a delay swept case by case, and then, for each case:
//
//   - D2_MADR at the stop and a while after, which differ if the channel
//     sent words after its busy bit was cleared;
//   - GPUSTAT at the stop, a while after, and after GP1(01h);
//   - whether a fill rectangle sent to GP0 straight after the stop lands
//     (variant A, no GP1(01h) first), read back from video memory: a
//     command left partway would take the fill's words as its own;
//   - whether it lands after GP1(01h) (variant B).
//
// Everything is the probe's own register writes, with interrupts off and
// channel 2's interrupt masked, so the library only sets the GPU up. Every
// wait gives up after a bound, and it prints END and returns, on either
// library. It draws off the display, from x 640 up.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

const (
	gp0     = 0x1f801810
	gp1     = 0x1f801814
	d2MADR  = 0x1f8010a0
	d2BCR   = 0x1f8010a4
	d2CHCR  = 0x1f8010a8
	dicr    = 0x1f8010f4
	nodes   = 64
	perNode = 2 // quads a node
	cases   = 400
	step    = 2 // delay iterations between cases
)

var (
	list  [nodes * (1 + 8*perNode)]uint32
	spin  uint32
	stats [8]int
)

func load(a uintptr) uint32     { return volatile.LoadUint32((*uint32)(yaroze.Pointer(a))) }
func store(a uintptr, v uint32) { volatile.StoreUint32((*uint32)(yaroze.Pointer(a)), v) }

func delay(n int) {
	for range n {
		volatile.StoreUint32(&spin, volatile.LoadUint32(&spin)+1)
	}
}

// waitStat waits, at most a bound, for GPUSTAT bit b, and reports whether
// it came.
func waitStat(b uint) bool {
	for range 20000 {
		if load(gp1)>>b&1 != 0 {
			return true
		}
	}
	return false
}

// build links the list: each node a header and perNode quads of 4 by 4
// pixels, spread over 640 to 1007, 256 to 495.
func build() {
	at := 0
	for n := range nodes {
		head := at
		at++
		for q := range perNode {
			k := n*perNode + q
			x := uint32(640 + k%92*4)
			y := uint32(256 + k/92*4)
			c := uint32(0x38_000000 | (k*37)&0xff<<16 | 0x4080)
			list[at+0] = c
			list[at+1] = y<<16 | x
			list[at+2] = 0x204060
			list[at+3] = y<<16 | (x + 3)
			list[at+4] = 0x604020
			list[at+5] = (y+3)<<16 | x
			list[at+6] = 0x406080
			list[at+7] = (y+3)<<16 | (x + 3)
			at += 8
		}
		next := uint32(0xffffff)
		if n < nodes-1 {
			next = uint32(uintptr(unsafe.Pointer(&list[at]))) & 0xffffff
		}
		list[head] = uint32(8*perNode)<<24 | next
	}
}

// fill fills 16 by 1 at (1008, 500) with red r (0 to 31), and reports
// what the pixel there reads back, or FFFFFFFFh if the GPU didn't let it
// be read.
func fill(r uint32) uint32 {
	store(gp0, 0x02_000000|r<<3)
	store(gp0, 500<<16|1008)
	store(gp0, 1<<16|16)
	if !waitStat(26) {
		return 0xffffffff
	}
	store(gp0, 0xc0_000000)
	store(gp0, 500<<16|1008)
	store(gp0, 1<<16|1)
	if !waitStat(27) {
		return 0xffffffff
	}
	return load(gp0) & 0xffff
}

// clean puts the GPU back: command buffer reset, DMA direction 2, the
// drawing area all of video memory with no offset, drawing to the
// display allowed, then waits for it to be ready.
func clean() {
	store(gp1, 0x01_000000)
	store(gp1, 0x04_000002)
	store(gp0, 0xe1_000400)
	store(gp0, 0xe3_000000)
	store(gp0, 0xe4_000000|511<<10|1023)
	store(gp0, 0xe5_000000)
	waitStat(26)
}

func hex(v uint32) {
	const digits = "0123456789abcdef"
	var b [8]byte
	for i := range b {
		b[7-i] = digits[v>>(4*i)&15]
	}
	print(string(b[:]))
}

func main() {
	println("listcut")
	yaroze.ResetGraph(0)
	build()
	head := uint32(uintptr(unsafe.Pointer(&list[0]))) & 0xffffff
	end := uint32(uintptr(unsafe.Pointer(&list[len(list)-1]))) & 0xffffff
	print("list ")
	hex(head)
	print(" to ")
	hex(end)
	println()
	println("case delay madr-stop madr-after stat-stop stat-after stat-reset fillA fillB")

	yaroze.EnterCriticalSection()
	saved := load(dicr)
	store(dicr, saved&^(1<<18)) // channel 2's interrupt masked
	for c := range cases {
		clean()
		d := c * step
		store(d2MADR, head)
		store(d2BCR, 0)
		store(d2CHCR, 0x01000401)
		delay(d)
		store(d2CHCR, load(d2CHCR)&^0x01000000)
		m1 := load(d2MADR) & 0xffffff
		s1 := load(gp1)
		delay(500)
		m2 := load(d2MADR) & 0xffffff
		s2 := load(gp1)

		r := uint32(c%30 + 1)
		a := fill(r) // variant A: straight after the stop
		store(gp1, 0x01_000000)
		delay(50)
		s3 := load(gp1)
		clean()
		b := fill(r ^ 0x1f) // variant B: after GP1(01h)

		okA := a == r
		okB := b == r^0x1f
		cut := m1 != 0xffffff && m1 != end
		switch {
		case !cut:
			stats[0]++
		case okA:
			stats[1]++
		default:
			stats[2]++
		}
		if m1 != m2 {
			stats[3]++
		}
		if s2>>26&1 == 0 {
			stats[4]++
		}
		if s3>>26&1 == 0 {
			stats[5]++
		}
		if !okB {
			stats[6]++
		}
		print(c, " ", d, " ")
		hex(m1)
		print(" ")
		hex(m2)
		print(" ")
		hex(s1)
		print(" ")
		hex(s2)
		print(" ")
		hex(s3)
		print(" ")
		hex(a)
		print(" ")
		hex(b)
		println()
	}
	store(dicr, saved)
	clean()
	yaroze.ExitCriticalSection()

	println("summary: list ended", stats[0], "; cut with fill A landing", stats[1], "; cut with fill A lost", stats[2])
	println("summary: madr moved after the stop", stats[3], "; bit 26 clear after the stop", stats[4], "; bit 26 clear after GP1(01h)", stats[5], "; fill B lost", stats[6])
	yaroze.ResetGraph(0)
	println("END")
}
