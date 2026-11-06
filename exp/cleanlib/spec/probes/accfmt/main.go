//go:build yaroze

// Command fmt times printf and sprintf by conversion.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf
func sprintfC(dst *byte, f *byte, a, b uint32) int32

//export printf
func printfC(f *byte, a, b, c uint32) int32

var buf [512]byte

type c struct {
	f    string
	a, b uint32
}

var str = "hello\x00"

var cases = []c{
	{"", 0, 0},
	{"\x88\x9f", 0, 0},
	{"\x88\x9f\x88\x9fa", 0, 0},
	{"a\x88\x9f", 0, 0},
	{"\xa4\xc7\xa5", 0, 0},
	{"\n", 0, 0},
	{"a\nb", 0, 0},
	{"x", 0, 0},
	{"abcd", 0, 0},
	{"%d", 0, 0}, {"%d", 7, 0}, {"%d", 12, 0}, {"%d", 123, 0}, {"%d", 1234, 0}, {"%d", 12345, 0}, {"%d", 123456, 0}, {"%d", 1234567, 0}, {"%d", 12345678, 0}, {"%d", 123456789, 0}, {"%d", 1234567890, 0},
	{"%d", 0xffffffff, 0}, {"%d", 0xffffff85, 0}, {"%d", 0xffffb2e, 0},
	{"%i", 123, 0}, {"%u", 123, 0}, {"%u", 0xffffffff, 0},
	{"%x", 0, 0}, {"%x", 0xf, 0}, {"%x", 0xff, 0}, {"%x", 0x1234, 0}, {"%x", 0xdeadbeef, 0}, {"%X", 0xabcd, 0}, {"%o", 0, 0}, {"%o", 7, 0}, {"%o", 0777, 0}, {"%p", 0x1234, 0},
	{"%c", 'a', 0}, {"%%", 0, 0}, {"%5d", 1, 0}, {"%05d", 1, 0}, {"%-5d", 1, 0}, {"%+d", 1, 0}, {"% d", 1, 0}, {"%.3d", 1, 0}, {"%8x", 0xab, 0}, {"%08x", 0xab, 0}, {"%#x", 0xab, 0}, {"%hd", 12, 0}, {"%ld", 12, 0},
	{"%3c", 'a', 0}, {"%s", 0, 0}, {"%10s", 0, 0}, {"%-10s", 0, 0}, {"%.2s", 0, 0},
	{"a%db", 12, 0}, {"%d %d", 12, 34}, {"%d,%x", 12, 34}, {"%s %d", 0, 34},
	{"%*d", 5, 12}, {"%z", 1, 0},
}

func main() {
	yaroze.ResetGraph(0)
	for ci := range cases {
		cc := &cases[ci]
		f := make([]byte, len(cc.f)+1)
		copy(f, cc.f)
		a, b := cc.a, cc.b
		for i, ch := range f {
			if ch == 's' && i > 0 {
				a = uint32(uintptr(unsafe.Pointer(unsafe.StringData(str))))
				break
			}
		}
		print("C S ", ci, " ", cc.f, " ", cc.a, " ")
		var n int32
		for range 3 {
			n = sprintfC(&buf[0], &f[0], a, b)
		}
		println(n, " ", string(buf[:n]))
		print("C P ", ci, " ", cc.f, " ", cc.a, " ")
		for range 3 {
			n = printfC(&f[0], a, b, 0)
			print("|")
		}
		println(n)
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
