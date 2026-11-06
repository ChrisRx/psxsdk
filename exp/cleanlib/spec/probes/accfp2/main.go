//go:build yaroze

// Command fp2 times the soft float over many operands, for a fit.
package main

import (
	"math"

	"go.chrisrx.dev/psx/yaroze"
)

//export __muldf3
func muldf3(a, b float64) float64

//export __adddf3
func adddf3(a, b float64) float64

//export __divdf3
func divdf3(a, b float64) float64

var sink float64
var s uint64 = 0x9E3779B97F4A7C15

func rnd() uint64 {
	s ^= s << 13
	s ^= s >> 7
	s ^= s << 17
	return s
}
func hex(v uint64) {
	const d = "0123456789abcdef"
	for i := 60; i >= 0; i -= 4 {
		print(string(rune(d[v>>uint(i)&15])))
	}
}

func main() {
	yaroze.ResetGraph(0)
	for i := range 700 {
		r := rnd()
		m1, m2 := rnd(), rnd()
		// Sparse or dense mantissas.
		if r&3 == 0 {
			m1 &= rnd() & rnd()
		}
		if r&12 == 0 {
			m2 &= rnd() & rnd()
		}
		if r&16 != 0 {
			m1 &^= (1 << (rnd() % 52)) - 1
		}
		if r&32 != 0 {
			m2 &^= (1 << (rnd() % 52)) - 1
		}
		e1 := uint64(1023 + (r>>8)%8)
		d := (r >> 16) % 60
		if r&(1<<40) != 0 {
			d %= 4
		}
		e2 := e1 - d
		a := e1<<52 | m1&(1<<52-1)
		b := e2<<52 | m2&(1<<52-1)
		if r&(1<<24) != 0 {
			b |= 1 << 63
		}
		if r&(1<<25) != 0 {
			a |= 1 << 63
		}
		if r&(1<<26) != 0 {
			a, b = b, a
		}
		x, y := math.Float64frombits(a), math.Float64frombits(b)
		kind := i % 3
		print("C ", kind, " ")
		hex(a)
		print(" ")
		hex(b)
		println()
		for range 2 {
			switch kind {
			case 0:
				sink = adddf3(x, y)
			case 1:
				sink = muldf3(x, y)
			default:
				sink = divdf3(x, y)
			}
		}
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
