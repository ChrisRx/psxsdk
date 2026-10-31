package emu

import (
	"encoding/binary"
	"math"

	"go.chrisrx.dev/psx/emu/cpu"
)

// The Net Yaroze library's VSync waits for a vertical blank by polling the
// count its interrupt handler keeps, in a loop that also counts down a
// timeout on the stack:
//
//	8003cbd8  lw    $2, 0x10($sp)
//	          nop
//	          addiu $2, $2, -1
//	          sw    $2, 0x10($sp)
//	          lw    $2, 0x10($sp)
//	          nop
//	          bne   $2, $3, 8003cc24 ; $3 is -1: not timed out yet
//	          nop
//	8003cc24  lui   $2, 0x8005
//	          lw    $2, -0x160c($2)  ; the count, at 8004e9f4
//	          nop
//	          slt   $2, $2, $4       ; $4 is the count waited for
//	          bnez  $2, 8003cbd8
//	          nop
//
// A program that has its frame's work done spends the rest of the frame
// here, which is a good part of what the emulator does for most of them.
// The machine skips through it instead: it steps through one pass to learn
// what each step costs, and from then on, whenever the CPU comes to the
// head of the loop, it runs as many whole passes at once as fit before a
// device next has something to do. A pass leaves the registers as it found
// them and the timeout one less, and the count changes only in the
// interrupt handler, so while no interrupt can come the passes are the
// clocks counted as the CPU would count them, and the timeout lowered by
// their number.
const (
	vsyncWaitHead  = 0x8003cbd8
	vsyncWaitCount = 0x8004e9f4
)

// vsyncWaitPath is the address of each step of a pass, and vsyncWaitCode
// the instruction there.
var (
	vsyncWaitPath = [14]uint32{
		0x8003cbd8, 0x8003cbdc, 0x8003cbe0, 0x8003cbe4, 0x8003cbe8, 0x8003cbec, 0x8003cbf0,
		0x8003cbf4, 0x8003cc24, 0x8003cc28, 0x8003cc2c, 0x8003cc30, 0x8003cc34, 0x8003cc38,
	}
	vsyncWaitCode = [14]uint32{
		0x8fa20010, 0x00000000, 0x2442ffff, 0xafa20010, 0x8fa20010, 0x00000000, 0x1443000c,
		0x00000000, 0x3c028005, 0x8c42e9f4, 0x00000000, 0x0044102a, 0x1440ffe8, 0x00000000,
	}
)

// SkipVSyncWait sets whether the machine skips through the library's wait
// for a vertical blank, as it does unless told otherwise. Skipping changes
// nothing a program or a frontend can see, only how long the emulator
// takes; turning it off is for checking that.
func (m *Machine) SkipVSyncWait(on bool) { m.noSpin = !on }

// spinNone is a Clock's Spin when there is no loop to stop at: no step leaves
// the PC misaligned but one that takes an address error, and stopping there
// does no harm anyway.
const spinNone = 1

// spin is what the machine has learned of the loop: the cycles each step of
// a pass costs, once a pass has been stepped through, and how far through
// the pass it is while it steps; and from the cycles, what a pass adds to
// the clocks.
type spin struct {
	cycles   [len(vsyncWaitPath)]uint32
	learned  bool
	learning bool
	pos      int

	rate       float32
	passCycles uint64
	video      passClock
}

// spinAt reports whether the loop is in RAM.
func (m *Machine) spinAt() bool {
	for i, addr := range vsyncWaitPath {
		b, ok := m.Bus.ramBytes(addr, 4)
		if !ok || binary.LittleEndian.Uint32(b) != vsyncWaitCode[i] {
			return false
		}
	}
	return true
}

// spinStep runs the CPU at the loop, for up to budget steps, and returns
// how many it ran. Until it has learned the costs of a pass, it steps
// through one, a step at a time; then it skips what passes it can, and
// runs on from there.
func (m *Machine) spinStep(k *cpu.Clock, budget uint64) uint64 {
	c, s := m.CPU, &m.spin
	if !s.learned {
		if c.PC != vsyncWaitPath[s.pos] {
			// The loop ended, or an interrupt came, before the pass did.
			s.learning, s.pos = false, 0
			return c.Run(budget, k)
		}
		s.learning = true
		ran := c.Run(1, k)
		if c.Excode != cpu.NoException {
			s.learning, s.pos = false, 0
			return ran
		}
		s.cycles[s.pos] = c.LastCycles
		if s.pos++; s.pos == len(s.cycles) {
			s.learned = c.PC == vsyncWaitHead
			s.learning, s.pos = false, 0
		}
		return ran
	}
	n := m.skipSpin(k, budget)
	return n + c.Run(budget-n, k)
}

// skipSpin runs whole passes of the loop, as the CPU's Run would count
// them, as many as end before a device has something to do and within
// budget steps, leaving at least one for the CPU. It returns the steps
// skipped.
func (m *Machine) skipSpin(k *cpu.Clock, budget uint64) uint64 {
	// The count may have changed since the last pass read it, if an
	// interrupt came after that, and the next pass would leave the loop.
	c := m.CPU
	slot, ok := m.Bus.ramBytes(c.R[29]+0x10, 4)
	count, ok2 := m.Bus.ramBytes(vsyncWaitCount, 4)
	if !ok || !ok2 || int32(binary.LittleEndian.Uint32(count)) >= int32(c.R[4]) ||
		c.R[3] != 0xffffffff || c.IRQPending() || *k.Busy ||
		*k.Sample >= k.SampleCycles || *k.StepsDue <= *k.Steps || budget < 2 {
		return 0
	}
	// Where Run would stop: at a step that reaches a device's due cycle,
	// the SPU's next sample, the timers' due step or the video's next edge.
	// The clocks only go up, so a pass reaches none of those if its last
	// step does not, and a pass whose timeout reaches -1 leaves the loop,
	// so the passes stop short of all of them.
	s := &m.spin
	if s.rate != k.VideoRate {
		var steps [len(s.cycles)]float32
		s.rate, s.passCycles = k.VideoRate, 0
		for i, cycles := range s.cycles {
			steps[i] = float32(float32(cycles) * k.VideoRate)
			s.passCycles += uint64(cycles)
		}
		s.video = newPassClock(steps)
	}
	const passSteps = uint64(len(m.spin.cycles))
	timeout := int32(binary.LittleEndian.Uint32(slot))
	n := int64(max(timeout, 0))
	if due := min(*k.Due[0], *k.Due[1], *k.Due[2], c.TotalCycles+uint64(k.SampleCycles-*k.Sample)); due > c.TotalCycles {
		n = min(n, int64((due-c.TotalCycles-1)/s.passCycles))
	} else {
		n = 0
	}
	n = min(n, int64((min(*k.StepsDue-*k.Steps, budget)-1)/passSteps))
	passes, v := s.video.passes(*k.Video, *k.VideoEdge, n)
	if passes == 0 {
		return 0
	}

	cycles, steps := uint64(passes)*s.passCycles, uint64(passes)*passSteps
	*k.Sample += uint32(cycles)
	*k.Steps += steps
	*k.Video = v
	c.TotalCycles += cycles
	binary.LittleEndian.PutUint32(slot, uint32(timeout-int32(passes)))
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
	steps [len(vsyncWaitPath)]float32
	adds  [passClockTop - passClockBottom + 1]int64 // in units, or 0 for a step at a time
}

// The binades a pass is worked out for, from 2^passClockBottom; below it the
// clock is summed a step at a time, and it never reaches 2^(passClockTop+1),
// being short of a scanline's 3413 cycles.
const passClockBottom, passClockTop = -8, 11

func newPassClock(steps [len(vsyncWaitPath)]float32) passClock {
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
