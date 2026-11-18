//go:build yaroze

// Command fntpool is a probe for the spec: how many characters and streams
// FntOpen gives out in all, from the ids it returns after FntLoad as the
// sizes asked for add up.
package main

import "go.chrisrx.dev/psx/yaroze"

func try(sizes []int32) {
	yaroze.FntLoad(960, 256)
	print("sizes")
	for _, n := range sizes {
		print(" ", n)
	}
	print(" ->")
	for _, n := range sizes {
		print(" ", yaroze.FntOpen(0, 0, 64, 64, 0, n))
	}
	println()
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	try([]int32{1024, 1})
	try([]int32{1025})
	try([]int32{1023, 1, 1})
	try([]int32{600, 600})
	try([]int32{2000})
	try([]int32{512, 512, 1})
	try([]int32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	try([]int32{100, 100, 100, 100, 100, 100, 100, 100})
	try([]int32{0, 0})
	println("done")
	for {
		yaroze.VSync(0)
	}
}
