//go:build yaroze

// Command crand is a probe for spec 19's Terra findings: the library's rand and srand.
// It prints the sequence from the start, after each of several seeds, and whether the
// kernel's rand sees any of it (the run's .kernel file).
package main

import "go.chrisrx.dev/psx/yaroze"

//export rand
func rand() int32

//export srand
func srand(seed uint32)

func run(n int) {
	for i := 0; i < n; i++ {
		print(" ", rand())
	}
	println()
}

func main() {
	yaroze.ResetGraph(0)
	println("start")
	run(40)
	for _, s := range [...]uint32{0, 1, 2, 3, 100, 0x7fff, 0x8000, 0x10000, 0x12345678, 0x80000000, 0xffffffff} {
		srand(s)
		print("seed ", s, ":")
		run(24)
	}
	srand(1)
	sum, most := uint32(0), int32(0)
	for i := 0; i < 100000; i++ {
		v := rand()
		sum = sum*31 + uint32(v)
		if v > most {
			most = v
		}
	}
	println("sum", sum, "most", most)
	println("END")
	for {
	}
}
