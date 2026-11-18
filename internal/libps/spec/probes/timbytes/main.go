//go:build yaroze

// Command bytes reads the classes of bytes 93h to 96h after GsInitGraph.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

//export printf
func printfC(f *byte, a, b, c uint32) int32

//export toupper
func toupperC(c int32) int32

//export tolower
func tolowerC(c int32) int32

func scan(r int) {
	var f [8]byte
	for y := 0x8c; y < 0xa3; y++ {
		f[0], f[1], f[2], f[3] = 'a', byte(y), 'a', 0
		print("P ", r, " ", y, "\n")
		printfC(&f[0], 0, 0, 0)
		print("\n")
		print("U ", r, " ", y, " ", toupperC(int32(y)), " ", tolowerC(int32(y)), "\n")
	}
}

func main() {
	yaroze.ResetGraph(0)
	scan(0)
	yaroze.GsInitGraph(320, 240, 4, 0, 0)
	scan(1)
	yaroze.GsInitGraph(640, 480, 2, 1, 0)
	scan(2)
	yaroze.GsInitGraph(256, 240, 0, 0, 0)
	scan(3)
	yaroze.ResetGraph(3)
	scan(4)
	yaroze.ResetGraph(0)
	scan(5)
	print("END\n")
	for {
		yaroze.VSync(0)
	}
}
