package emu

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestPassClock checks the video clock's passes against summing every step
// of every pass, for clocks across a scanline and limits on the passes,
// with increments as the loop's steps make them and with increments that
// lie halfway between two units of a binade.
func TestPassClock(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	stepped := func(p *passClock, v, edge float32, n int64) (int64, float32) {
		var done int64
		for ; done < n; done++ {
			next := v
			for _, d := range p.steps {
				next = float32(next + d)
			}
			if next >= edge {
				break
			}
			v = next
		}
		return done, v
	}
	edges := [2]float32{gpuHDrawCycles, math.Nextafter32(gpuScanlineCycles, math.MaxFloat32)}
	for trial := range 20000 {
		var steps [len(vsyncWaitPath)]float32
		e := 5 + r.IntN(7)
		for i := range steps {
			steps[i] = float32(float32(1+r.IntN(12)) * gpuCyclesPerCPU)
			if trial%3 == 0 && r.IntN(2) == 0 {
				// Halfway between two units of a binade from 2^5 to 2^11.
				odd := 2*r.IntN(19<<(23-e)) + 1<<(24-e) + 1
				steps[i] = float32(math.Ldexp(float64(odd), e-24))
			}
		}
		p := newPassClock(steps)
		edge := edges[r.IntN(2)]
		var v float32
		switch r.IntN(3) {
		case 0:
			v = float32(r.Float64()) * edge
		case 1:
			// Just short of a power of two.
			v = float32(math.Ldexp(1, r.IntN(12))) - float32(r.IntN(64))*0x1p-12
		default:
			// Just past the end of a scanline.
			v = float32(1+r.IntN(1<<16)) * 0x1p-12
		}
		v = min(max(v, 0), math.Nextafter32(edge, 0))
		if trial%5 == 2 {
			// A whole number of passes short of the top of a binade, or of
			// the edge where that is in the binade, which the last of them
			// lands on exactly.
			e := 5 + r.IntN(7)
			unit := math.Ldexp(1, e-23)
			top := min(1<<24, int64(float64(edge)/unit))
			if add := p.adds[e-passClockBottom]; add > 0 && top-8*add >= 1<<23 {
				v = float32(float64(top-int64(1+r.IntN(8))*add) * unit)
			}
		}
		n := int64(r.IntN(300))
		gotN, gotV := p.passes(v, edge, n)
		wantN, wantV := stepped(&p, v, edge, n)
		if gotN != wantN || gotV != wantV {
			t.Fatalf("trial %d: steps %v, from %v to %v, up to %d passes: %d passes to %v, want %d to %v",
				trial, p.steps, v, edge, n, gotN, gotV, wantN, wantV)
		}
	}
}

// TestRoundUnits checks the rounding of increments to units at the edges:
// nothing, next to nothing, a whole number, halfway, and either side of it.
func TestRoundUnits(t *testing.T) {
	for _, c := range []struct {
		d     float32
		u     int
		units int64
		ok    bool
	}{
		{0, -12, 0, true},
		{0x1p-40, -12, 0, true},
		{3, -12, 3 << 12, true},
		{3 + 0x1p-13, -12, 0, false},                  // halfway
		{3 + 0x1p-13 + 0x1p-21, -12, 3<<12 + 1, true}, // just past halfway
		{3 + 0x1p-13 - 0x1p-21, -12, 3 << 12, true},   // just short of it
		{3.75, 1, 2, true},
		{3, 1, 0, false},
	} {
		units, ok := roundUnits(c.d, c.u)
		if units != c.units || ok != c.ok {
			t.Errorf("roundUnits(%v, %d) = %d, %v, want %d, %v", c.d, c.u, units, ok, c.units, c.ok)
		}
	}
}
