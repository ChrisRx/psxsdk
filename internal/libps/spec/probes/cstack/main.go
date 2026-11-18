//go:build yaroze

// Command cstack is a probe for spec 17: how far below the caller's sp each entry
// writes, and what each call costs in CPU cycles. The stack below sp is filled with
// a pattern before the call and scanned after it, twice with different patterns,
// with interrupts off. The cost is the system clock on root counter 2 around the
// call, minus the same reading around nothing, the minimum of a few runs.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf
func sprintf(s *byte, f *byte, a2, a3, a4, a5, a6, a7 uint32) int32

//export sprintf2
func sprintf2(s *byte, f *byte, a2, a3, a4, a5, a6, a7 uint32) int32

//export printf
func printf(f *byte, a1, a2, a3, a4, a5, a6, a7 uint32) int32

//export printf2
func printf2(f *byte, a1, a2, a3, a4, a5, a6, a7 uint32) int32

//export _err_math
func errMath(a0, a1, a2, a3 uint32) uint32

//export _dbl_shift
func dblShift(a0, a1, a2, a3, a4, a5 uint32) uint32

//export FntPrint
func fntPrint(a0, a1, a2, a3, a4, a5 uint32) int32

//export KanjiFntPrint
func kanjiFntPrint(a0, a1, a2, a3, a4, a5 uint32) int32

var (
	line  [256]byte
	nline int
)

func flush() {
	yaroze.Write(1, &line[0], int32(nline))
	nline = 0
}

func ps(s string) {
	nline += copy(line[nline:], s)
}

func pi(v int32) {
	if v < 0 {
		ps("-")
		v = -v
	}
	var d [12]byte
	k := 0
	for {
		d[k] = byte('0' + v%10)
		k++
		v /= 10
		if v == 0 {
			break
		}
	}
	for k > 0 {
		k--
		line[nline] = d[k]
		nline++
	}
}

// One call, by kind. The words are set before a measurement and read inside.
const (
	kSprintf = iota
	kSprintf2
	kPrintf
	kPrintf2
	kErrMath
	kDblShift
	kFnt
	kKanji
	kWrite
)

var (
	fbuf [64]byte
	out  [2048]byte
	dbl  [2]uint32
	kind int
	w    [6]uint32
	tiny = [...]byte{'x'}
)

func z(f string) *byte {
	n := copy(fbuf[:len(fbuf)-1], f)
	fbuf[n] = 0
	return &fbuf[0]
}

var callSP uint32

//go:noinline
func call() {
	callSP = uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$29\n.set pop", nil))
	switch kind {
	case kSprintf:
		sprintf(&out[0], &fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5])
	case kSprintf2:
		sprintf2(&out[0], &fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5])
	case kPrintf:
		printf(&fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5], 0)
	case kPrintf2:
		printf2(&fbuf[0], w[0], w[1], w[2], w[3], w[4], w[5], 0)
	case kErrMath:
		errMath(w[0], 0, 0, 0)
	case kDblShift:
		dblShift(uint32(uintptr(unsafe.Pointer(&dbl[0]))), 0, w[0], w[1], w[2], 0)
	case kFnt:
		fntPrint(uint32(uintptr(unsafe.Pointer(&fbuf[0]))), w[0], w[1], w[2], w[3], w[4])
	case kKanji:
		kanjiFntPrint(uint32(uintptr(unsafe.Pointer(&fbuf[0]))), w[0], w[1], w[2], w[3], w[4])
	case kWrite:
		yaroze.Write(1, &tiny[0], 1)
	}
}

// reach measures how far below callSP the call writes: the lowest word changed.
// callSP is read at the start of call, where the compiler may not yet have made its
// frame, so a line's figure is the stack use plus that frame: the harness's sp at an
// entry against the callSP line gives the frame to take off.
const span = 8192

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
		if d := int32(callSP - a); d > most {
			most = d
		}
	}
	return most
}

const (
	t2count = 0x1f801120
	t2mode  = 0x1f801124
)

func count() uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(t2count))) & 0xffff
}

var base uint32

// timed times a call in the given counter mode, the minimum of 5.
func timed(mode uint32) uint32 {
	best := uint32(0xffffffff)
	for range 5 {
		volatile.StoreUint32((*uint32)(yaroze.Pointer(t2mode)), mode)
		volatile.StoreUint32((*uint32)(yaroze.Pointer(t2count)), 0)
		a := count()
		call()
		d := (count() - a) & 0xffff
		if d < best {
			best = d
		}
	}
	return best
}

// cycles times a call by the system clock, and by the clock over 8 when a call
// could pass the 16-bit counter.
func cycles() int32 {
	eighth := timed(0x200) * 8
	if eighth >= 60000 {
		return int32(eighth) - int32(base)
	}
	return int32(timed(0)) - int32(base)
}

func run(name string, k int, f string, a ...uint32) {
	kind = k
	z(f)
	for i := range w {
		w[i] = 0
	}
	copy(w[:], a)
	yaroze.EnterCriticalSection()
	r := reach()
	sp := callSP
	c := cycles()
	yaroze.ExitCriticalSection()
	ps(name)
	ps(" \"")
	for i := 0; i < len(f); i++ {
		if f[i] == '\n' {
			ps("\\n")
		} else {
			line[nline] = f[i]
			nline++
		}
	}
	ps("\" callSP ")
	pi(int32(sp))
	ps(" stack ")
	pi(r)
	ps(" cycles ")
	pi(c)
	ps("\n")
	flush()
}

const none = 99

var s1 = [...]byte{'h', 'e', 'l', 'l', 'o', 0}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(0, 0, 320, 240, 0, 512)

	kind = none
	yaroze.EnterCriticalSection()
	base = timed(0)
	yaroze.ExitCriticalSection()
	ps("callSP ")
	pi(int32(callSP))
	ps("\n")
	flush()
	ps("base cycles ")
	pi(int32(base))
	ps("\n")
	flush()

	str := uint32(uintptr(unsafe.Pointer(&s1[0])))
	for _, e := range [...]struct {
		name string
		k    int
	}{{"sprintf", kSprintf}, {"sprintf2", kSprintf2}} {
		run(e.name, e.k, "")
		run(e.name, e.k, "x")
		run(e.name, e.k, "plain text!!!!!!")
		run(e.name, e.k, "%d", 42)
		run(e.name, e.k, "%d", 0x80000000)
		run(e.name, e.k, "%d", 1234567890)
		run(e.name, e.k, "%x", 0xbeef)
		run(e.name, e.k, "%o", 0xffffffff)
		run(e.name, e.k, "%s", str)
		run(e.name, e.k, "%c", 65)
		run(e.name, e.k, "%20d", 42)
		run(e.name, e.k, "%-20d", 42)
		run(e.name, e.k, "%020d", 42)
		run(e.name, e.k, "%.20d", 42)
		run(e.name, e.k, "%100d", 42)
		run(e.name, e.k, "%500d", 42)
		run(e.name, e.k, "%1000d", 42)
		run(e.name, e.k, "%d %d %d %d", 1, 2, 3, 4)
	}
	for _, c := range [...]struct {
		f      string
		lo, hi uint32
	}{
		{"%f", 0, 0}, {"%f", 0, 0x3ff00000}, {"%f", 0x54442d18, 0x400921fb},
		{"%e", 0, 0}, {"%e", 0, 0x3ff00000}, {"%e", 0x54442d18, 0x400921fb},
		{"%g", 0, 0}, {"%g", 0, 0x3ff00000}, {"%g", 0x54442d18, 0x400921fb},
		{"%.15f", 0x54442d18, 0x400921fb}, {"%.40f", 0x54442d18, 0x400921fb},
		{"%.15e", 0x54442d18, 0x400921fb}, {"%.40e", 0x54442d18, 0x400921fb},
		{"%f", 0, 0x7fe00000}, {"%e", 0, 0x7fe00000}, {"%g", 0, 0x7fe00000},
		{"%f", 0, 0x00100000}, {"%e", 0, 0x00100000}, {"%g", 0, 0x00100000},
		{"%e", 1, 0}, {"%.300f", 0, 0x00100000},
		{"%.300e", 0x54442d18, 0x400921fb}, {"%.300g", 0x54442d18, 0x400921fb}, {"%.300f", 0xffffffff, 0x7fefffff},
		{"%#.300g", 0xffffffff, 0x7fefffff}, {"%500.300f", 0, 0x3ff00000}, {"%-500.300e", 0, 0x3ff00000}, {"%g", 0xffffffff, 0x7fefffff},
		{"%f", 0xffffffff, 0x7fefffff}, {"%e", 0xffffffff, 0x7fefffff},
	} {
		run("sprintf2", kSprintf2, c.f, c.lo, c.hi)
	}
	run("write", kWrite, "")
	for _, e := range [...]struct {
		name string
		k    int
	}{{"printf", kPrintf}, {"printf2", kPrintf2}} {
		run(e.name, e.k, "")
		run(e.name, e.k, "x")
		run(e.name, e.k, "plain\n")
		run(e.name, e.k, "%d", 42)
		run(e.name, e.k, "%d", 0x80000000)
		run(e.name, e.k, "%20d", 42)
		run(e.name, e.k, "%s", str)
	}
	run("printf2", kPrintf2, "%f", 0, 0x54442d18, 0x400921fb)
	run("printf2", kPrintf2, "%e", 0, 0x54442d18, 0x400921fb)
	run("printf2", kPrintf2, "%g", 0, 0x54442d18, 0x400921fb)
	run("_err_math", kErrMath, "", 33)
	run("_err_math", kErrMath, "", 0)
	for _, n := range [...]uint32{0, 1, 2, 16, 31, 32, 33, 63, 64, 65, 100, 1000, 10000} {
		dbl[0] = 0
		run("_dbl_shift", kDblShift, "", 0x89abcdef, 0x01234567, n)
	}
	run("FntPrint", kFnt, "")
	run("FntPrint", kFnt, "x")
	run("FntPrint", kFnt, "%d", 42)
	run("FntPrint", kFnt, "%20d", 42)
	run("FntPrint", kFnt, "%s", str)
	run("FntPrint", kFnt, "%x", 0xbeef)
	yaroze.KanjiFntOpen(0, 0, 320, 240, 640, 0, 0, 480, 1, 512)
	yaroze.DrawSync(0)
	run("KanjiFntPrint", kKanji, "")
	run("KanjiFntPrint", kKanji, "x")
	run("KanjiFntPrint", kKanji, "%d", 42)
	run("KanjiFntPrint", kKanji, "%20d", 42)
	run("KanjiFntPrint", kKanji, "%s", str)
	run("KanjiFntPrint", kKanji, "%x", 0xbeef)
	yaroze.FntFlush(0)
	yaroze.DrawSync(0)
	ps("END\n")
	flush()
}
