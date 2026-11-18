//go:build yaroze

// Command stdin is a probe for spec 18: getc, getchar and gets, on input no
// emulator's kernel gives. The kernel's read of file 0 returns at once and writes
// nothing, so the probe puts a read of its own in front of the kernel's B0
// vector at address B0h, for function 34h alone. That read notes its arguments, stores
// the next input byte unless told not to, and returns the value it is told to.
// The kernel calls (.build/stdin-cimg/out.kernel) still show B0h 34h, and the echo
// on the TTY is between the brackets.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export getc
func getc(fd int32) int32

//export getchar
func getchar() int32

//export gets
func gets(buf *byte) unsafe.Pointer

//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

var (
	input   string
	pos     int
	ret     int32
	store   bool
	calls   int
	lastFd  int32
	lastLen int32
)

// fakeRead stands for the kernel's read(fd, buf, n). When the input runs out it
// gives newlines, so that gets ends.
func fakeRead(fd int32, buf *byte, n int32) int32 {
	calls++
	lastFd, lastLen = fd, n
	if store {
		c := byte('\n')
		if pos < len(input) {
			c = input[pos]
			pos++
		}
		*buf = c
	}
	return ret
}

var line [4096]byte

func one(what string, r int32) {
	println("]", what, "returns", r, "calls", calls, "fd", lastFd, "n", lastLen)
	calls = 0
}

func set(in string, r int32, st bool) {
	input, pos, ret, store = in, 0, r, st
}

func runGets(in string, r int32) {
	set(in, r, true)
	for i := range line {
		line[i] = 0xa5
	}
	print("[")
	p := gets(&line[0])
	print("] gets of")
	for i := 0; i < len(in); i++ {
		print(" ", in[i])
	}
	print(" read returning ", r, ": same pointer ", p == unsafe.Pointer(&line[0]), " used ", pos, " calls ", calls, " fd ", lastFd, " n ", lastLen, " stored")
	calls = 0
	n := 0
	for n < len(line) && line[n] != 0xa5 {
		k := n
		for k < len(line) && line[k] == line[n] {
			k++
		}
		print(" ", line[n], "x", k-n)
		n = k
	}
	println()
}

// fills is a string of n letters.
const letters = "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"

var stub [10]uint32

// patch sends function 34h of the kernel's B0 vector to fakeRead, and the others
// on to the vector's first three words, which are a load of t0 and a jump.
func patch() {
	f := fakeRead
	to := uint32((*[2]uintptr)(unsafe.Pointer(&f))[1])
	vec := (*[4]uint32)(unsafe.Pointer(uintptr(0xb0)))
	stub = [10]uint32{
		0x34080034,             // ori t0, zero, 34h
		0x15280005,             // bne t1, t0, the kernel's words
		0,                      // nop
		0x3c080000 | to>>16,    // lui t0, fakeRead
		0x35080000 | to&0xffff, // ori t0, t0, fakeRead
		0x01000008,             // jr t0
		0,                      // nop
		vec[0], vec[1], vec[2],
	}
	at := uint32(uintptr(unsafe.Pointer(&stub[0])))
	yaroze.EnterCriticalSection()
	vec[0] = 0x3c080000 | at>>16
	vec[1] = 0x35080000 | at&0xffff
	vec[2] = 0x01000008
	vec[3] = 0
	yaroze.ExitCriticalSection()
	yaroze.FlushCache()
}

func main() {
	patch()

	for _, c := range [...]byte{0x41, 0xff, 0x80, 0x0a, 0x0d, 0} {
		buf := [1]byte{c}
		set(string(buf[:]), 1, true)
		print("[")
		one("getc(0)", getc(0))
		set(string(buf[:]), 1, true)
		print("[")
		one("getc(7)", getc(7))
		set(string(buf[:]), 1, true)
		print("[")
		one("getchar", getchar())
	}
	for _, r := range [...]int32{0, -1, 2, -2} {
		for _, st := range [...]bool{true, false} {
			set("B", r, st)
			print("[")
			one("getc(0)", getc(0))
			set("B", r, st)
			print("[")
			one("getchar", getchar())
		}
	}
	runGets("abc\n", 1)
	runGets("abc\r", 1)
	runGets("\n", 1)
	runGets("\r", 1)
	runGets("abc\nxyz", 1)
	for c := 0; c < 256; c++ {
		switch {
		case c >= 'b' && c <= 'z', c >= 'A' && c <= 'Z', c >= '1' && c <= '9':
			continue
		}
		runGets(string([]byte{'a', byte(c), 'c', '\n'}), 1)
	}
	runGets("\x08\x08ab\n", 1)
	runGets("\x7f\x7fab\n", 1)
	runGets("ab\x08\x08\x08\x08cd\n", 1)
	runGets("ab\x7f\x7f\x7f\x7fcd\n", 1)
	for _, n := range [...]int{123, 124, 125, 126, 127, 128, 130} {
		runGets(letters[:n]+"\n", 1)
		runGets(letters[:n]+"\x07\x08Z\n", 1)
	}
	// Control V, which is neither stored nor echoed, before what might be quoted.
	runGets("a\x16\x01c\n", 1)
	runGets("a\x16\nc\n", 1)
	runGets("a\x16\rc\n", 1)
	runGets("a\x16\x08c\n", 1)
	runGets("a\x16\x7fc\n", 1)
	runGets("a\x16\x16c\n", 1)
	runGets("a\x16\tc\n", 1)
	runGets("a\x16\x80c\n", 1)
	runGets("a\x16\xc1c\n", 1)
	runGets(letters[:124]+"\x16\x01\x16\x01\n", 1)
	runGets("ab\n", -1)
	runGets("ab\n", 0)
	runGets("ab\n", 2)
	end()
}

// end prints the line that stops the run, through the kernel's putchar, since
// write would put a "\r" before the "\n" and the harness looks for "END\n".
func end() {
	for _, c := range [...]byte{'E', 'N', 'D', '\n'} {
		libps_biosB(0x3d, uintptr(c), 0, 0, 0)
	}
	for {
	}
}
