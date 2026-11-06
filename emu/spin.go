package emu

import (
	"math"
	"slices"

	"go.chrisrx.dev/psx/emu/cpu"
)

// A program that has its frame's work done spends the rest of the frame
// polling for the vertical blank, which is a good part of what the emulator
// does for most programs. The machine skips through a wait like that, whoever
// wrote it: whenever the CPU hands back, it asks the CPU whether it is in a
// quiet loop (CPU.TryPass), one whose pass touches only the registers, reads
// only RAM and comes back with the registers as they were. If so, it runs as
// many whole passes at once as fit before a device next has something to do,
// by counting the clocks as the CPU would count them.
//
// Each pass goes the same way as the trial because nothing it depends on can
// change. It doesn't change the registers or store anywhere, and this is the
// one thing the skip relies on from outside this file and pass.go: a device
// only changes RAM, or raises an interrupt, at a point where Run would stop,
// which is the cycles in Clock.Due, the SPU's sample, the video's edges and
// every step while DMA is busy. A device that acts at any other time has to
// add it to Clock, or the skip would run past it.
//
// Nothing is remembered from one stop to the next but the costs worked out
// below, so a loop is found wherever it is and at any point in its pass, and
// the count an interrupt handler raised is seen by the next trial. A loop that
// counts a timeout down in a register or in memory is not quiet, and runs at
// the ordinary speed.

// SkipVSyncWait sets whether the machine skips through a program's wait for a
// vertical blank, and any other quiet loop, as it does unless told otherwise.
// Skipping changes nothing a program or a frontend can see, only how long the
// emulator takes; turning it off is for checking that.
func (m *Machine) SkipVSyncWait(on bool) { m.noSpin = !on }

// spin is what the machine keeps for skipping: the trial's pass, and what a
// pass costing cycles adds to the clocks, worked out when the costs change.
type spin struct {
	pass cpu.Pass

	cycles     []uint32
	passCycles uint64
	video      passClock

	// rest is how many more stops to let go by before another trial.
	rest int

	// skipped counts the steps skipped.
	skipped uint64
}

// spinRest is the stops let go by after a trial finds no loop. A program that
// isn't waiting would otherwise pay for a trial at every stop, 2% of its time
// and up to a fifth on code that keeps a trial going to the end, while a wait,
// which has about a thousand stops in a frame, loses a few of them.
const spinRest = 7

// skipSpin runs whole passes of the loop the CPU is in, if it is in one, as
// Run would count them, as many as end before a device has something to do and
// within budget steps, leaving at least one for the CPU. It returns the steps
// skipped. The machine calls it between runs, with every step so far counted.
func (m *Machine) skipSpin(k *cpu.Clock, budget uint64) uint64 {
	c, s := m.CPU, &m.spin
	if *k.Busy || *k.Sample >= k.SampleCycles || budget < 2 {
		return 0
	}
	if s.rest > 0 {
		s.rest--
		return 0
	}
	if !c.TryPass(&s.pass, k) {
		s.rest = spinRest
		return 0
	}
	if cycles := s.pass.Cycles[:s.pass.Steps]; !slices.Equal(s.cycles, cycles) {
		s.cycles = slices.Clone(cycles)
		s.passCycles = 0
		steps := make([]float32, len(cycles))
		for i, n := range cycles {
			steps[i] = float32(float32(n) * k.VideoRate)
			s.passCycles += uint64(n)
		}
		s.video = newPassClock(steps)
	}

	// Where Run would stop: at a step that reaches a device's due cycle, the
	// SPU's next sample or the video's next edge. The clocks only go up, so a
	// pass reaches none of those if its last step does not.
	due := min(*k.Due[0], *k.Due[1], *k.Due[2], *k.Due[3], c.TotalCycles+uint64(k.SampleCycles-*k.Sample))
	if due <= c.TotalCycles {
		return 0
	}
	passSteps := uint64(len(s.cycles))
	n := min((due-c.TotalCycles-1)/s.passCycles, (budget-1)/passSteps)
	passes, v := s.video.passes(*k.Video, *k.VideoEdge, int64(n))

	cycles, steps := uint64(passes)*s.passCycles, uint64(passes)*passSteps
	*k.Sample += uint32(cycles)
	*k.Steps += steps
	*k.Video = v
	c.TotalCycles += cycles
	s.skipped += steps
	return steps
}

// passClock is what a pass of the loop adds to the video clock, which Run
// keeps as a float32, adding each step's increment and rounding: the
// increments, and for each binade of the clock, the floats from a power of
// two up to the next, what a whole pass adds there.
//
// Within a binade every float is a whole number of the binade's unit, 2^-23
// of its bottom. Adding an increment to one rounds the increment to whole
// units, to the nearest, the same wherever the clock is, unless it lies
// halfway between two, when the clock's last bit decides. So while a pass
// starts and ends within the binade and no increment is halfway, each pass
// adds the same whole number of units, and the passes before the clock
// leaves the binade or reaches an edge come to a division. The pass that
// leaves it, and every pass in a binade where an increment is halfway, are
// summed a step at a time.
type passClock struct {
	steps []float32
	adds  [passClockTop - passClockBottom + 1]int64 // in units, or 0 for a step at a time
}

// The binades a pass is worked out for, from 2^passClockBottom; below it the
// clock is summed a step at a time, and it never reaches 2^(passClockTop+1),
// being short of a scanline's 3413 cycles.
const passClockBottom, passClockTop = -8, 11

func newPassClock(steps []float32) passClock {
	p := passClock{steps: steps}
	for i := range p.adds {
		for _, d := range p.steps {
			units, ok := roundUnits(d, passClockBottom+i-23)
			if !ok {
				p.adds[i] = 0
				break
			}
			p.adds[i] += units
		}
	}
	return p
}

// passes returns how many passes, up to n, take the clock from v to short of
// edge, and where they leave it.
func (p *passClock) passes(v, edge float32, n int64) (int64, float32) {
	var done int64
	for done < n && v < edge {
		if add := p.add(v); add > 0 {
			// In units of its binade, a clock there is its mantissa, with
			// the leading one: the passes that end short of the binade's
			// top, and of the edge if that is in the binade too, are those
			// that keep it short of 1<<24, or of the edge's.
			b, eb := math.Float32bits(v), math.Float32bits(edge)
			at, top := int64(b&0x7fffff|0x800000), int64(1<<24)
			if eb>>23 == b>>23 {
				top = int64(eb&0x7fffff | 0x800000)
			}
			j := min((top-at-1)/add, n-done)
			v = math.Float32frombits(b&^0x7fffff | uint32(at+j*add)&0x7fffff)
			if done += j; done == n {
				break
			}
		}
		next := v
		for _, d := range p.steps {
			next = float32(next + d)
		}
		if next >= edge {
			break
		}
		v, done = next, done+1
	}
	return done, v
}

// add returns what a pass adds to the clock at v, in units of v's binade,
// or 0 where the pass is to be summed a step at a time.
func (p *passClock) add(v float32) int64 {
	// The binade, from the exponent, which a negative clock's sign bit puts
	// out of range.
	i := int(math.Float32bits(v)>>23) - 127 - passClockBottom
	if uint(i) >= uint(len(p.adds)) {
		return 0
	}
	return p.adds[i]
}

// roundUnits rounds d to a whole number of units of 2^u, to the nearest, and
// reports false if it lies halfway between two.
func roundUnits(d float32, u int) (int64, bool) {
	// d is m units of 2^(e-23), for its exponent e, so m shifted right by s
	// units of 2^u. Less than half a unit, which zero is, rounds to none.
	b := math.Float32bits(d)
	m := int64(b&0x7fffff | 0x800000)
	s := u - (int(b>>23&0xff) - 127 - 23)
	switch {
	case s <= 0:
		return m << -s, true
	case s > 25:
		return 0, true
	}
	q, r, half := m>>s, m&(1<<s-1), int64(1)<<(s-1)
	if r == half {
		return 0, false
	}
	if r > half {
		q++
	}
	return q, true
}
