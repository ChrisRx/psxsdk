//go:build yaroze

// Twin: the Go library's functions against the originals, in one program.
//
// The program is built like any example, laid over the resident image, and
// imports the Go library, whose exports take the plain names; the
// originals are reached under _orig names. Each case calls both on the
// same inputs and compares what came out, printing ok NAME or FAIL NAME
// with the first difference. Functions with hardware effects run on two
// machines instead: mode, which the test pokes after boot, picks the
// originals (0) or the Go versions (1) for one script of calls, and the
// test compares the machines afterwards. The report ends with twin done.
//
// The program never calls VSync by the plain name before the script, and
// never sleeps: the runtime's VSync is the Go library's, which would take
// the interrupts over from the resident library before the originals run.
package main

import (
	"unsafe"

	_ "go.chrisrx.dev/psx/exp/libps"
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// mode is which library the hardware script runs: 0 the originals, 1 the
// Go versions. The test writes it after boot, so it is read as the
// hardware is, through a volatile load: an ordinary read of a variable
// nothing in the program writes folds to its initial value, and the
// variable with it.
var mode int32

func readMode() int32 { return int32(volatile.LoadUint32((*uint32)(unsafe.Pointer(&mode)))) }

func main() {
	print("twin ready\n")
	for range 2 {
		VSync_orig(0)
	}
	inPlace()
	hardware(readMode())
	spriteCases()
	objCases()
	done()
	for {
	}
}

// inPlace runs the cases that compare an original and its Go version in
// memory, with no hardware in between.
func inPlace() {
	checkRand()
	checkGetTPage()
	checkVideoMode()
	checkCdPos()
	checkRCnt()
}

// pattern is what the hardware script loads into video memory: 16 by 16
// pixels, each its own value.
var pattern [128]uint32

// hardware runs the script whose effects the test reads from the machine.
func hardware(mode int32) {
	for i := range pattern {
		pattern[i] = uint32(i)<<16 | uint32(i)*3
	}
	rect := yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
	var pad0, pad1 *uint8
	if mode == 0 {
		ResetGraph_orig(0)
		SetVideoMode_orig(0)
		LoadImage_orig(&rect, &pattern[0])
		DrawSync_orig(0)
		a := VSync_orig(-1)
		VSync_orig(0)
		VSync_orig(0)
		VSync_orig(0)
		VSync_orig(2)
		VSync_orig(1)
		print("vsync ", VSync_orig(-1)-a, "\n")
		GetPadBuf_orig(&pad0, &pad1)
		VSync_orig(0)
		VSync_orig(0)
	} else {
		yaroze.ResetGraph(0)
		yaroze.SetVideoMode(0)
		yaroze.LoadImage(&rect, &pattern[0])
		yaroze.DrawSync(0)
		a := yaroze.VSync(-1)
		yaroze.VSync(0)
		yaroze.VSync(0)
		yaroze.VSync(0)
		yaroze.VSync(2)
		yaroze.VSync(1)
		print("vsync ", yaroze.VSync(-1)-a, "\n")
		yaroze.GetPadBuf(&pad0, &pad1)
		yaroze.VSync(0)
		yaroze.VSync(0)
	}
	b := unsafe.Slice(pad0, 4)
	print("pad ", hex(b[0]), " ", hex(b[1]), " ", hex(b[2]), " ", hex(b[3]), "\n")
	syncScript(mode)
	gsScript(mode)
	fontScript(mode)
	bgScript(mode)
	cdScript(mode)
	sndScript(mode)
	cdPlayScript(mode)
	objScript(mode)
	kanjiScript(mode)
}

// checkRand compares the generators from two seeds.
func checkRand() {
	srand_orig(1)
	srand(1)
	for i := range 16 {
		checkInt("rand "+itoa(i), rand_orig(), rand())
	}
	srand_orig(12345)
	srand(12345)
	for i := 16; i < 32; i++ {
		checkInt("rand "+itoa(i), rand_orig(), rand())
	}
}

// checkGetTPage compares the texture page values over every depth and
// blend, and coordinates past the edges of video memory.
func checkGetTPage() {
	xs := [4]int32{0, 64, 640, 1024}
	ys := [3]int32{0, 256, 512}
	for tp := int32(0); tp < 4; tp++ {
		for abr := int32(0); abr < 4; abr++ {
			for _, x := range xs {
				for _, y := range ys {
					name := "GetTPage " + itoa(int(tp)) + " " + itoa(int(abr)) + " " + itoa(int(x)) + " " + itoa(int(y))
					checkInt(name, int32(GetTPage_orig(tp, abr, x, y)), int32(yaroze.GetTPage(tp, abr, x, y)))
				}
			}
		}
	}
}

// checkVideoMode compares what each library records of the video mode.
func checkVideoMode() {
	checkInt("SetVideoMode 0", SetVideoMode_orig(0), yaroze.SetVideoMode(0))
	checkInt("GetVideoMode 0", GetVideoMode_orig(), yaroze.GetVideoMode())
	checkInt("SetVideoMode 1", SetVideoMode_orig(1), yaroze.SetVideoMode(1))
	checkInt("GetVideoMode 1", GetVideoMode_orig(), yaroze.GetVideoMode())
	SetVideoMode_orig(0)
	yaroze.SetVideoMode(0)
}

//export rand
func rand() int32

//export srand
func srand(seed uint32)

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ticks counts the vertical blanks the callback saw; the interrupt
// handler writes it, so it is read through a volatile load.
var ticks uint32

func twinTick() { ticks++ }

// tickAddr is the callback's address, as a program would pass it: the
// code word of the function's value. The function takes no argument and
// uses no context, so a call with none, as the library makes, is a
// call it answers. An exported function would be the usual way, but one
// nothing in Go calls is dropped before the link.
func tickAddr() uintptr {
	f := twinTick
	return (*[2]uintptr)(unsafe.Pointer(&f))[1]
}

// syncScript waits longer than a wait the Go library once gave up on,
// then has a callback counted over three vertical blanks and taken out
// again. The returned frame length and the count have to agree.
func syncScript(mode int32) {
	tick := tickAddr()
	var long int32
	if mode == 0 {
		long = VSync_orig(240)
		VSyncCallback_orig(tick)
		VSync_orig(0)
		VSync_orig(0)
		VSync_orig(0)
		VSyncCallback_orig(0)
		VSync_orig(0)
	} else {
		long = yaroze.VSync(240)
		yaroze.VSyncCallback(tick)
		yaroze.VSync(0)
		yaroze.VSync(0)
		yaroze.VSync(0)
		yaroze.VSyncCallback(0)
		yaroze.VSync(0)
	}
	print("long ", long, "\n")
	print("ticks ", volatile.LoadUint32(&ticks), "\n")
}
