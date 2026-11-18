//go:build yaroze

// Command fp times the soft float by operands.
package main

import (
	"math"

	"go.chrisrx.dev/psx/yaroze"
)

//export __muldf3
func muldf3(a, b float64) float64

//export __adddf3
func adddf3(a, b float64) float64

//export __subdf3
func subdf3(a, b float64) float64

//export __divdf3
func divdf3(a, b float64) float64


var sink float64

func b(u uint64) float64 { return math.Float64frombits(u) }

func main() {
	yaroze.ResetGraph(0)
	type pair struct{ a, b uint64 }
	one, half, three := uint64(0x3ff0000000000000), uint64(0x3fe0000000000000), uint64(0x4008000000000000)
	pi := uint64(0x400921fb54442d18)
	lots := uint64(0x3fffffffffffffff)
	ms := []pair{{one, one}, {three, one}, {three, three}, {pi, one}, {one, pi}, {pi, pi}, {lots, one}, {one, lots}, {lots, lots}, {0x3ff0000000000001, one}, {one, 0x3ff0000000000001}, {0x3ff0000000000001, 0x3ff0000000000001}, {0x3ff8000000000001, one}, {0x3ff0000100000000, one}, {one, 0x3ff0000100000000}, {0x3ff0000100000000, 0x3ff0000100000000}, {0x3fffffff00000000, one}, {one, 0x3fffffff00000000}, {0, one}, {one, 0}, {half, half}, {0xbff0000000000000, one}, {pi, 0x3e50000000000000}}
	for i, p := range ms {
		print("C M ", i, " ")
		for range 3 {
			sink = muldf3(b(p.a), b(p.b))
		}
		println()
	}
	as := []pair{{one, one}, {three, one}, {one, three}, {pi, one}, {one, pi}, {pi, pi}, {one, half}, {half, one}, {one, 0x3fc0000000000000}, {one, 0x3f00000000000000}, {one, 0x3ca0000000000000}, {one, 0x3c80000000000000}, {one, 0x3000000000000000}, {0x3f00000000000000, one}, {one, 0xbff0000000000000}, {one, 0xbfe0000000000000}, {one, 0xbfefffffffffffff}, {one, 0xbfffffffffffffff}, {pi, 0xc008000000000000}, {0, one}, {one, 0}, {0x400921fb54442d18, 0xc00921fb54442d10}, {lots, lots}, {lots, one}}
	for i, p := range as {
		print("C A ", i, " ")
		for range 3 {
			sink = adddf3(b(p.a), b(p.b))
		}
		println()
	}
	ds := []pair{{one, one}, {three, one}, {one, three}, {pi, one}, {one, pi}, {pi, pi}, {lots, one}, {one, lots}, {0, one}, {pi, three}, {three, pi}, {0x3ff0000000000001, one}, {one, 0x3ff0000000000001}, {0x3fe5555555555555, one}, {one, half}, {half, one}}
	for i, p := range ds {
		print("C D ", i, " ")
		for range 3 {
			sink = divdf3(b(p.a), b(p.b))
		}
		println()
	}
	args := []float64{0, 1e-10, 0.1, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 2, 2.5, 3, 3.5, 4, 5, 6, 7, 10, 100, 1000, -1, -3.5}
	for i, a := range args {
		print("C S ", i, " ")
		for range 2 {
			sink += yaroze.Sin(a + sink*0)
		}
		println(uint64(math.Float64bits(sink)))
	}
	for i, a := range args {
		print("C Q ", i, " ")
		for range 2 {
			sink += yaroze.Sqrt(a + sink*0)
		}
		println(uint64(math.Float64bits(sink)))
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
