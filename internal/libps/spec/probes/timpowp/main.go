//go:build yaroze

// Command powp times pow across arguments, between the writes of its marks.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

//export pow
func powC(x, y float64) float64

var sink float64

func mark(i, j int) {
	print("M ", i, " ", j, "\n")
}

var ys = []float64{0.5, 2, 3, 0.25, 1.5, -1, 0.1, 10, 0.333}
var xs = []float64{1, 2, 4, 8, 16, 32, 256, 512, 262144, 524288, 1048576, 2097152, 0.5, 0.25, 0.125, 1.25, 1310720, 3, 7, 10, 100, 1000, 0.1, 0.7, 1.5, 12345.678}

func main() {
	yaroze.ResetGraph(0)
	for i, y := range ys {
		for j, x := range xs {
			mark(i, j)
			sink = powC(x, y)
			print("E\n")
		}
	}
	print("END\n")
	for {
		yaroze.VSync(0)
	}
}
