//go:build yaroze

// Command cdcalls is a probe for spec 18: CdControlF, CdInit, CdStatus, CdSync and
// CdReady, against CdControl and CdControlB. The target has no names for these
// five, so they are called at their entries. Each call prints what it returned,
// the bytes it wrote and how long it took in clocks of the second counter, and the
// drive's state after is read with CdControlB: GetlocP gives the position, Getstat
// the status. The kernel calls are in .build/cdcalls-cimg/out.kernel.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	cdControlF func(com int32, param *uint8) int32
	cdInit     func() int32
	cdStatus   func() int32
	cdSync     func(mode int32, result *uint8) int32
	cdReady    func(mode int32, result *uint8) int32
)

//export CdControl
func cdControl(com int32, param, result *uint8) int32

//export CdControlB
func cdControlB(com int32, param, result *uint8) int32

const (
	cdlNop     = 1
	cdlSetloc  = 2
	cdlReadN   = 6
	cdlPause   = 9
	cdlGetlocP = 0x11
	cdlSeekL   = 0x15
)

var (
	res  [8]uint8
	pos  = [4]uint8{0, 0x02, 0x16, 0}
	far  = [4]uint8{0x20, 0x10, 0x00, 0}
	near = [4]uint8{0, 0x03, 0x00, 0}
	bad  = [4]uint8{0xaa, 0, 0, 0}
)

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	cmode = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))
)

// clocks returns the time of call, up to a counter's wrap: a call that blocks
// longer than that shows as wrapped.
func clocks(call func()) uint32 {
	volatile.StoreUint32(cmode, 0)
	t0 := volatile.LoadUint32(count)
	call()
	return (volatile.LoadUint32(count) - t0) & 0xffff
}

func showRes(what string, v int32) {
	print(what, " returns ", v, " res")
	for i := range res {
		print(" ", res[i])
	}
	println()
}

func clear() {
	for i := range res {
		res[i] = 0xa5
	}
}

// state prints the drive's status and position, through calls that wait.
func state(what string) {
	var r [8]uint8
	v := cdControlB(cdlNop, nil, &r[0])
	print(what, " Nop ", v, " status ", r[0])
	v = cdControlB(cdlGetlocP, nil, &r[0])
	println(" GetlocP", v, "at", r[2], r[3], r[4], r[5], r[6], r[7])
}

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

func f(what string, com int32, param *uint8) {
	var v int32
	t := clocks(func() { v = cdControlF(com, param) })
	println(what, "CdControlF returns", v, "clocks", t)
}

func c(what string, com int32, param *uint8) {
	var v int32
	clear()
	t := clocks(func() { v = cdControl(com, param, &res[0]) })
	print(what, " CdControl clocks ", t, " ")
	showRes("", v)
}

func b(what string, com int32, param *uint8) {
	var v int32
	clear()
	t := clocks(func() { v = cdControlB(com, param, &res[0]) })
	print(what, " CdControlB clocks ", t, " ")
	showRes("", v)
}

func status(what string) {
	println(what, "CdStatus", cdStatus())
}

func sync(what string, mode int32) {
	var v int32
	clear()
	t := clocks(func() { v = cdSync(mode, &res[0]) })
	print(what, " CdSync(", mode, ") clocks ", t, " ")
	showRes("", v)
}

func ready(what string, mode int32) {
	var v int32
	clear()
	t := clocks(func() { v = cdReady(mode, &res[0]) })
	print(what, " CdReady(", mode, ") clocks ", t, " ")
	showRes("", v)
}

// newCalls goes through CdStatus, CdSync, CdReady and CdInit.
func newCalls() {
	println("CdStatus, CdSync and CdReady at rest")
	status("rest")
	sync("rest", 1)
	sync("rest", 0)
	ready("rest", 1)
	println(" nil results", cdSync(1, nil), cdReady(1, nil))
	sync("mode 2", 2)
	sync("mode -1", -1)
	ready("mode 2", 2)
	ready("mode -1", -1)
	var t uint32
	t = clocks(func() { cdStatus() })
	println("CdStatus clocks", t)

	println("after CdControlF Nop")
	f("Nop", cdlNop, nil)
	status("at once")
	sync("at once", 1)
	sync("wait", 0)
	status("after")
	sync("after", 1)

	println("after CdControl Nop")
	c("Nop", cdlNop, nil)
	sync("after", 1)
	status("after")

	println("after CdControlF SeekL")
	f("SeekL", cdlSeekL, &near[0])
	sync("at once", 1)
	status("at once")
	sync("wait", 0)
	status("after")
	state("after")

	println("after CdControlB SeekL")
	b("SeekL", cdlSeekL, &pos[0])
	sync("after", 1)
	status("after")

	println("a refused command")
	f("1f", 0x1f, nil)
	sync("wait", 0)
	status("after")
	sync("after", 1)
	state("after")

	println("a seek past the end")
	f("SeekL", cdlSeekL, &far[0])
	sync("wait", 0)
	status("after")
	state("after")

	println("a read")
	f("Setloc", cdlSetloc, &pos[0])
	sync("setloc", 0)
	f("ReadN", cdlReadN, nil)
	sync("read", 1)
	ready("read", 1)
	ready("read", 0)
	status("read")
	ready("read", 1)
	ready("read", 0)
	status("read")
	b("Pause", cdlPause, nil)
	ready("paused", 1)
	sync("paused", 1)
	status("paused")

	pairs()

	println("CdInit")
	var v int32
	mark(1)
	t = clocks(func() { v = cdInit() })
	mark(2)
	println(" returns", v, "clocks", t)
	status("after")
	sync("after", 1)
	ready("after", 1)
	state("after")
	c("Nop", cdlNop, nil)
	b("SeekL", cdlSeekL, &pos[0])
	state("after")
	mark(3)
	t = clocks(func() { v = cdInit() })
	mark(4)
	println(" again returns", v, "clocks", t)
	state("after")
	println("CdInit in a read")
	f("Setloc", cdlSetloc, &pos[0])
	sync("setloc", 0)
	f("ReadN", cdlReadN, nil)
	wait(4)
	ready("read", 1)
	mark(5)
	v = cdInit()
	mark(6)
	println(" returns", v)
	ready("after", 1)
	state("after")

	// Last, as it may not return: nothing is coming.
	println("CdReady(0) at rest")
	f0 := yaroze.VSync(-1)
	ready("rest", 0)
	println(" frames", yaroze.VSync(-1)-f0)
}

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var r1, r2, r3 int32

// pairs makes the calls with nothing printed between, since each character
// printed takes a good part of a drive's first response.
func pairs() {
	println("CdSync straight after CdControlF, no printing between")
	var t uint32
	for _, m := range [...]int32{1, 2, -1} {
		clear()
		cdControlF(cdlNop, nil)
		r1 = cdSync(m, &res[0])
		r2 = cdStatus()
		println(" Nop mode", m, "returns", r1, "status", r2)
		showRes(" res", 0)
		cdSync(0, nil)
	}
	cdControlF(cdlNop, nil)
	t = clocks(func() { r1 = cdSync(0, &res[0]) })
	println(" Nop then mode 0 returns", r1, "clocks", t)
	showRes(" res", 0)

	// The seek: its first response, then its second.
	cdControlF(cdlSetloc, &near[0])
	cdSync(0, nil)
	cdControlF(cdlSeekL, nil)
	r1 = cdSync(1, &res[0])
	println(" SeekL at once returns", r1, "status", cdStatus())
	t = clocks(func() { r2 = cdSync(0, &res[0]) })
	println(" SeekL then mode 0 returns", r2, "clocks", t)
	showRes(" res", 0)
	r3 = cdSync(1, &res[0])
	println(" SeekL then mode 1 returns", r3, "status", cdStatus())
	wait(200)
	r3 = cdSync(1, &res[0])
	println(" 200 frames on, mode 1 returns", r3, "status", cdStatus())
	showRes(" res", 0)

	// CdControl and CdControlB leave what?
	cdControl(cdlNop, nil, nil)
	r1 = cdSync(1, nil)
	println(" CdControl Nop then mode 1 returns", r1)
	cdControl(cdlSetloc, &pos[0], nil)
	cdControl(cdlSeekL, nil, nil)
	r1 = cdSync(1, nil)
	t = clocks(func() { r2 = cdSync(0, nil) })
	println(" CdControl SeekL then modes 1, 0 return", r1, r2, "clocks", t)
	wait(100)
	cdControlB(cdlSetloc, &near[0], nil)
	cdControlB(cdlSeekL, nil, nil)
	r1 = cdSync(1, nil)
	println(" CdControlB SeekL then mode 1 returns", r1)
	wait(100)

	// An error, read several times.
	cdControlF(0x1f, nil)
	wait(4)
	r1 = cdSync(1, &res[0])
	r2 = cdSync(1, nil)
	r3 = cdSync(0, nil)
	println(" refused, then modes 1, 1, 0 return", r1, r2, r3, "status", cdStatus())
	showRes(" res", 0)
	cdControlF(0x1f, nil)
	r1 = cdSync(0, nil)
	r2 = cdSync(0, nil)
	println(" refused, then modes 0, 0 return", r1, r2)

	// What makes CdReady 0 again, and whether it waits.
	cdControl(cdlSetloc, &pos[0], nil)
	cdControlF(cdlReadN, nil)
	r1 = cdReady(1, &res[0])
	t = clocks(func() { r2 = cdReady(0, &res[0]) })
	println(" ReadN at once, modes 1, 0 return", r1, r2, "clocks", t)
	showRes(" res", 0)
	r1 = cdReady(1, nil)
	r2 = cdReady(1, nil)
	println(" then mode 1 twice return", r1, r2)
	cdControlB(cdlPause, nil, nil)
	r1 = cdReady(1, nil)
	cdControl(cdlNop, nil, nil)
	r2 = cdReady(1, nil)
	cdControlB(cdlSeekL, &near[0], nil)
	r3 = cdReady(1, nil)
	println(" paused 1, after Nop, after SeekL return", r1, r2, r3)
	t = clocks(func() { r1 = cdReady(0, &res[0]) })
	println(" paused, mode 0 returns", r1, "clocks", t)

	// A read past the end.
	cdControl(cdlSetloc, &far[0], nil)
	cdControlF(cdlReadN, nil)
	wait(30)
	r1 = cdReady(1, &res[0])
	println(" ReadN past the end, mode 1 returns", r1, "status", cdStatus())
	showRes(" res", 0)
	r2 = cdSync(1, &res[0])
	r3 = cdSync(1, nil)
	println(" then CdSync(1) twice returns", r2, r3)
	showRes(" res", 0)
	r1 = cdReady(1, nil)
	println(" then mode 1 returns", r1)
	cdControlB(cdlPause, nil, nil)
}

func main() {
	(*[2]uintptr)(unsafe.Pointer(&cdControlF))[1] = 0x80039618
	(*[2]uintptr)(unsafe.Pointer(&cdInit))[1] = 0x8003921c
	(*[2]uintptr)(unsafe.Pointer(&cdStatus))[1] = 0x80039324
	(*[2]uintptr)(unsafe.Pointer(&cdSync))[1] = 0x80039470
	(*[2]uintptr)(unsafe.Pointer(&cdReady))[1] = 0x80039490
	yaroze.ResetGraph(0)
	state("first")
	println("Nop, each way")
	c("Nop", cdlNop, nil)
	b("Nop", cdlNop, nil)
	f("Nop", cdlNop, nil)
	wait(4)
	state("after")

	println("Setloc, then a look at it")
	f("Setloc", cdlSetloc, &far[0])
	wait(4)
	state("after")

	println("SeekL with a position")
	f("SeekL far", cdlSeekL, &far[0])
	state("at once")
	wait(200)
	state("200 frames on")

	println("SeekL with a position on the disc")
	f("SeekL near", cdlSeekL, &near[0])
	state("at once")
	wait(200)
	state("200 frames on")

	println("SeekL without a parameter, after a Setloc")
	f("Setloc", cdlSetloc, &pos[0])
	wait(2)
	f("SeekL", cdlSeekL, nil)
	wait(200)
	state("200 frames on")

	println("CdControl after CdControlF at once")
	f("Nop", cdlNop, nil)
	c("Nop", cdlNop, nil)
	wait(4)
	println("twice in a row")
	f("Nop", cdlNop, nil)
	f("Nop", cdlNop, nil)
	wait(4)
	state("after")

	println("a command the drive refuses")
	f("1f", 0x1f, nil)
	wait(4)
	state("after")
	println("a bad position")
	f("Setloc", cdlSetloc, &bad[0])
	wait(4)
	state("after")
	f("SeekL bad", cdlSeekL, &bad[0])
	wait(4)
	state("after")

	println("ReadN")
	f("Setloc", cdlSetloc, &pos[0])
	wait(2)
	f("ReadN", cdlReadN, nil)
	wait(10)
	state("reading")
	b("Pause", cdlPause, nil)
	wait(4)
	state("after")

	println("a command byte over 255")
	f("0x101", 0x101, nil)
	wait(4)
	state("after")

	println("the waiting calls still work")
	b("SeekL", cdlSeekL, &pos[0])
	wait(200)
	state("after")
	newCalls()
	end()
}

// libps_biosB calls function fn of the kernel's B0 table.
//
//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

// end prints the line that stops the run, through the kernel's putchar, since
// write would put a "\r" before the "\n" and the harness looks for "END\n".
func end() {
	for _, c := range [...]byte{'E', 'N', 'D', '\n'} {
		libps_biosB(0x3d, uintptr(c), 0, 0, 0)
	}
	for {
	}
}
