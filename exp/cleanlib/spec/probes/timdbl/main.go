//go:build yaroze

// Command dbl times and reads _dbl_shift over x and n.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export _dbl_shift
func dblShift5(a0, a1, a2, a3, a4, a5 uint32) uint64

var buf [4]uint32

var xs = [][2]uint32{{0x667f3bca, 0x0016a09e}, {0x00000001, 0x00000000}, {0, 0x00100000}, {0xffffffff, 0x7fffffff}, {0x01234567, 0x76543210}, {0x01234567, 0xf6543210}, {0xfedcba98, 0x80000000}, {0, 0}, {0xbfa54800, 0x000fffff}}
var ns = []uint32{0, 1, 4, 8, 11, 16, 31, 32, 33, 52, 63, 64, 0xffffffff, 0xfffffff5}

func hex(v uint32) {
	const d = "0123456789abcdef"
	for i := 28; i >= 0; i -= 4 {
		print(string(d[v>>uint(i)&15]))
	}
	print(" ")
}

func main() {
	yaroze.ResetGraph(0)
	for _, a5 := range []uint32{0, 1, 2, 3, 4, 8, 0x10, 0x100, 0x10000, 0x80000000, 0xffffffff, 0x13579bdf} {
		for _, n := range []uint32{1, 4, 33, 64} {
			for _, a1 := range []uint32{0, 0x89abcdef} {
				print("D ")
				hex(a5)
				hex(n)
				hex(a1)
				print("\n")
				dblShift5(uint32(uintptr(unsafe.Pointer(&buf[0]))), a1, 0x01234567, 0xf6543210, n, a5)
				print("E ")
				hex(buf[0])
				hex(buf[1])
				print("\n")
			}
		}
	}
	print("END\n")
	for {
		yaroze.VSync(0)
	}
}
