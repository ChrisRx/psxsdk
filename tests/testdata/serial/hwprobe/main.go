// The program the real hardware session runs under each library (spec 35). It
// times routines with the console's own counters, with interrupts off, so the
// numbers from a console, from the emulator, and from each library can be set
// side by side. It also draws a fixed scene and prints a hash of the picture,
// which needs no capture card. Everything is measured before anything is
// printed, since printing goes down the serial cable.
//
// A "T" line is a name and five readings of the system clock, the first after
// other code has run and so with a cold instruction cache. Its readings
// include the "null" line's. An "S" line is the same at an eighth of the
// clock, for routines too long for the 16-bit counter. It returns from main,
// which restarts the monitor.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export bcopy
func bcopy(src, dst unsafe.Pointer, n int32)

//export __addsf3
func addsf3(a, b uint32) uint32

//export __muldf3
func muldf3(a, b float64) float64

//export __divdf3
func divdf3(a, b float64) float64

const samples = 5

// Timer 2's value and mode. Writing the mode restarts the count. Mode 0 counts
// the system clock, and 200h an eighth of it.
const (
	timerValue = 0x1f801120
	timerMode  = 0x1f801124
)

type test struct {
	name string
	slow bool
	run  func()
	got  [samples]uint32
}

var (
	screen  = yaroze.SVECTOR{Vx: 100, Vy: -50, Vz: 200}
	matrix  yaroze.MATRIX
	matrixB yaroze.MATRIX
	matrixC yaroze.MATRIX
	vector  = yaroze.VECTOR{Vx: 10, Vy: 20, Vz: 30}
	vectorO yaroze.VECTOR
	coordA  yaroze.GsCOORDINATE2
	sxy     uint32
	sink    int32
	sinkF   float64
	sinkU   uint32

	ot      yaroze.GsOT
	otTags  [16]yaroze.GsOT_TAG
	packets [8192]uint32
	line    = yaroze.GsLINE{X0: 10, Y0: 10, X1: 300, Y1: 200, R: 255, G: 200, B: 40}
	box     = yaroze.GsBOXF{X: 40, Y: 30, W: 120, H: 80, R: 30, G: 90, B: 200}
	sprite  = yaroze.GsSPRITE{Attribute: 2 << 24, X: 200, Y: 60, W: 16, H: 16, Tpage: 0, U: 0, V: 0, R: 128, G: 128, B: 128, Mx: 8, My: 8, Scalex: 4096 * 3, Scaley: 4096 * 3, Rotate: 45 * 4096}
	texture [8*16 + 1]uint32
	pixels  [320 * 240 / 2]uint32
	copyA   [64]uint32
	copyB   [64]uint32
	memory  [256]uint32

	// argA and argB are set at run time, or the compiler works out the
	// library's math itself. keep is so it can't drop the results.
	argA, argB float64
	keep       uint32
	divisor    = uint32(100)
)

func clearOt() {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0]))
}

var tests = []test{
	{name: "null", run: func() {}},
	{name: "loadRAM", run: func() {
		for i := range memory {
			sinkU = volatile.LoadUint32(&memory[i])
		}
	}},
	{name: "storeRAM", run: func() {
		for i := range memory {
			volatile.StoreUint32(&memory[i], 1)
		}
	}},
	{name: "rand", run: func() { sink = yaroze.Rand() }},
	{name: "GetClut", run: func() { sink = int32(yaroze.GetClut(0, 480)) }},
	{name: "GetTPage", run: func() { sink = int32(yaroze.GetTPage(2, 0, 640, 0)) }},
	{name: "GsGetActiveBuff", run: func() { sink = yaroze.GsGetActiveBuff() }},
	{name: "GsSetWorkBase", run: func() { yaroze.GsSetWorkBase(yaroze.Packets(&packets[0])) }},
	{name: "GsInitCoordinate2", run: func() { yaroze.GsInitCoordinate2(nil, &coordA) }},
	{name: "TransMatrix", run: func() { yaroze.TransMatrix(&matrix, &vector) }},
	{name: "RotMatrix", run: func() { yaroze.RotMatrix(&screen, &matrix) }},
	{name: "MulMatrix0", run: func() { yaroze.MulMatrix0(&matrix, &matrixB, &matrixC) }},
	{name: "ApplyMatrix", run: func() { yaroze.ApplyMatrix(&matrix, &screen, &vectorO) }},
	{name: "RotTransPers", run: func() { sink = yaroze.RotTransPers(&screen, &sxy) }},
	// The compiler turns a call to bcopy into its own memmove, inline if short,
	// so this isn't the library's bcopy.
	{name: "memmove256", run: func() {
		bcopy(unsafe.Pointer(&copyA[0]), unsafe.Pointer(&copyB[0]), 256)
		sinkU = volatile.LoadUint32(&copyB[0])
	}},
	{name: "addsf3", run: func() { sinkU = addsf3(0x40123456, 0x3f123456) }},
	{name: "muldf3", run: func() { sinkF = muldf3(3.25, 1.5) }},
	{name: "divdf3", run: func() { sinkF = divdf3(3.141592653589793, 2.718281828459045) }},
	{name: "GsSortLine", run: func() { clearOt(); yaroze.GsSortLine(&line, &ot, 1) }},
	{name: "GsSortBoxFill", run: func() { clearOt(); yaroze.GsSortBoxFill(&box, &ot, 1) }},
	{name: "GsSortSprite", run: func() { clearOt(); yaroze.GsSortSprite(&sprite, &ot, 1) }},
	{name: "Sin", slow: true, run: func() { sinkF = yaroze.Sin(argA); volatile.StoreUint32(&keep, uint32(sinkF)) }},
	{name: "Sqrt", slow: true, run: func() { sinkF = yaroze.Sqrt(argB); volatile.StoreUint32(&keep, uint32(sinkF)) }},
}

func measure(t *test) {
	mode := uint32(0)
	if t.slow {
		mode = 0x200
	}
	for i := range t.got {
		volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), mode)
		start := volatile.LoadUint32((*uint32)(yaroze.Pointer(timerValue)))
		t.run()
		end := volatile.LoadUint32((*uint32)(yaroze.Pointer(timerValue)))
		t.got[i] = (end - start) & 0xffff
	}
}

// frame counts the eighths of the system clock over a frame. That's about
// 70,600 on NTSC, so the 16-bit count has wrapped once: add 65,536. It's about
// 84,700 on PAL, which is the same.
func frame() uint32 {
	yaroze.VSync(0)
	volatile.StoreUint32((*uint32)(yaroze.Pointer(timerMode)), 0x200)
	yaroze.VSync(0)
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(timerValue))) & 0xffff
}

// scene draws what the hash is taken of.
func scene() uint32 {
	rect := yaroze.RECT{X: 640, Y: 0, W: 16, H: 16}
	for i := range texture {
		texture[i] = uint32(i)*0x00210421 | 0x80008000
	}
	yaroze.LoadImage(&rect, &texture[0])
	yaroze.DrawSync(0)
	sprite.Tpage = yaroze.GetTPage(2, 0, 640, 0)

	clearOt()
	yaroze.GsSortClear(20, 20, 60, &ot)
	yaroze.GsSortBoxFill(&box, &ot, 2)
	yaroze.GsSortLine(&line, &ot, 1)
	yaroze.GsSortSprite(&sprite, &ot, 0)
	yaroze.GsDrawOt(&ot)
	yaroze.DrawSync(0)

	area := yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}
	yaroze.StoreImage(&area, &pixels[0])
	yaroze.DrawSync(0)

	hash := uint32(2166136261)
	for i := range pixels {
		hash = (hash ^ pixels[i]) * 16777619
	}
	return hash
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, 0, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	yaroze.GsSetProjection(1000)
	yaroze.RotMatrix(&screen, &matrixB)
	ot.Length = 4
	ot.Org = &otTags[0]
	yaroze.SetDispMask(1)

	d := int32(volatile.LoadUint32(&divisor))
	argA, argB = float64(125)/float64(d), float64(250)/float64(d)

	frames := frame()
	hash := scene()
	yaroze.GsSwapDispBuff()

	yaroze.EnterCriticalSection()
	for i := range tests {
		measure(&tests[i])
	}
	yaroze.ExitCriticalSection()

	println("F frame", frames)
	println("H scene", hash)
	for i := range tests {
		t := &tests[i]
		if t.slow {
			print("S ")
		} else {
			print("T ")
		}
		print(t.name)
		for j := range t.got {
			print(" ", t.got[j])
		}
		println()
	}
	println("END")
}
