//go:build yaroze

// Command cdready is a probe for spec 18: what takes away a sector's response that
// CdReady has not asked for. cdcalls saw it stay over a Pause and go by the time of a
// later rest. Each case reads a few sectors, pauses, does one thing, and asks.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	cdControlF func(com int32, param *uint8) int32
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
	res [8]uint8
	pos = [4]uint8{0, 0x02, 0x16, 0}
	far = [4]uint8{0x20, 0x10, 0x00, 0}
)

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

func read() {
	cdControlF(cdlSetloc, &pos[0])
	wait(2)
	cdControlF(cdlReadN, nil)
	wait(10)
	cdControlB(cdlPause, nil, &res[0])
}

func ask(what string) {
	r := cdReady(1, &res[0])
	r2 := cdReady(1, &res[0])
	println(what, "ready", r, "then", r2, "res", res[0], res[1])
}

func main() {
	(*[2]uintptr)(unsafe.Pointer(&cdControlF))[1] = 0x80039618
	(*[2]uintptr)(unsafe.Pointer(&cdSync))[1] = 0x80039470
	(*[2]uintptr)(unsafe.Pointer(&cdReady))[1] = 0x80039490
	yaroze.ResetGraph(0)
	read()
	ask("nothing")
	read()
	wait(20)
	ask("frames")
	read()
	cdControl(cdlNop, nil, &res[0])
	ask("Nop CdControl")
	read()
	cdControlB(cdlNop, nil, &res[0])
	ask("Nop CdControlB")
	read()
	cdControlF(cdlNop, nil)
	wait(4)
	ask("Nop CdControlF")
	read()
	cdControlB(cdlGetlocP, nil, &res[0])
	ask("GetlocP CdControlB")
	read()
	cdControlF(cdlSetloc, &pos[0])
	wait(4)
	ask("Setloc CdControlF")
	read()
	cdControl(cdlSetloc, &pos[0], &res[0])
	ask("Setloc CdControl")
	read()
	cdControlB(cdlSeekL, &pos[0], &res[0])
	ask("SeekL CdControlB, position")
	read()
	cdControl(cdlSeekL, &pos[0], &res[0])
	wait(30)
	ask("SeekL CdControl, position")
	read()
	cdControlF(cdlSeekL, &pos[0])
	wait(30)
	ask("SeekL CdControlF, position")
	read()
	cdControlB(cdlSetloc, &pos[0], &res[0])
	cdControlB(cdlSeekL, nil, &res[0])
	ask("SeekL CdControlB, none")
	read()
	cdControlF(0x1f, nil)
	wait(4)
	ask("refused")
	read()
	cdSync(1, &res[0])
	ask("CdSync")
	read()
	cdControlB(cdlPause, nil, &res[0])
	ask("Pause again")
	// Is it the sending or an answer? Each asked for at once, then again later.
	for _, com := range [...]int32{cdlSeekL, 0x16, cdlReadN, 0x1b, 7, 8, 0x0a, 0x0b, 0x0c, 0x0e, cdlPause, 3, 4, 5, 0x13} {
		read()
		cdControlB(cdlSetloc, &pos[0], &res[0])
		cdControlF(com, nil)
		r := cdReady(1, &res[0])
		wait(30)
		r2 := cdReady(1, &res[0])
		cdControlB(cdlPause, nil, &res[0])
		wait(4)
		r3 := cdReady(1, &res[0])
		println("com", com, "at once", r, "later", r2, "after a pause", r3, "res", res[0], res[1])
	}
	// An error, then what takes it away.
	cdControlF(0x1f, nil)
	wait(4)
	cdControlB(cdlNop, nil, &res[0])
	println("error, Nop:", cdReady(1, &res[0]), cdReady(1, &res[0]), res[0], res[1])
	cdControlF(0x1f, nil)
	wait(4)
	cdControlB(cdlSeekL, &pos[0], &res[0])
	println("error, SeekL:", cdReady(1, &res[0]), cdReady(1, &res[0]), res[0], res[1])
	// A seek past the end.
	cdControlF(cdlSeekL, &far[0])
	wait(120)
	println("seek past the end:", cdReady(1, &res[0]), cdReady(1, &res[0]), res[0], res[1])
	// A sector, then an error: the error's report, and no sector after it.
	read()
	cdControlF(0x1f, nil)
	wait(4)
	println("sector, error:", cdReady(1, &res[0]), cdReady(1, &res[0]), res[0], res[1])
	// A command that takes parameters, with none given.
	for _, com := range [...]int32{0x0e, 0x0d, 0x14, cdlSetloc} {
		cdControlF(com, nil)
		wait(10)
		res = [8]uint8{0xa5, 0xa5}
		s := cdSync(1, &res[0])
		println("F", com, "nil: sync", s, res[0], res[1], "then ready", cdReady(1, &res[0]), res[0], res[1])
		res = [8]uint8{0xa5, 0xa5}
		r := cdControl(com, nil, &res[0])
		println("C", com, "nil: returns", r, res[0], res[1], "then ready", cdReady(1, &res[0]), res[0], res[1])
		cdSync(0, nil)
	}
	println("END")
	for {
	}
}
