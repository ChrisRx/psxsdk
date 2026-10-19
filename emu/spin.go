package emu

import (
	"encoding/binary"

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
// the pass it is while it steps.
type spin struct {
	cycles   [len(vsyncWaitPath)]uint32
	learned  bool
	learning bool
	pos      int
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
	due := min(*k.Due[0], *k.Due[1], c.TotalCycles+uint64(k.SampleCycles-*k.Sample))
	stepsDue := min(*k.StepsDue-*k.Steps, budget)

	// The clocks only go up, so a pass reaches none of those if its last
	// step does not. The video's still has to be summed a step at a time,
	// rounding as Run does.
	var video [len(m.spin.cycles)]float32
	var passCycles uint64
	for i, cycles := range m.spin.cycles {
		video[i] = float32(float32(cycles) * k.VideoRate)
		passCycles += uint64(cycles)
	}
	const passSteps = uint64(len(m.spin.cycles))

	// A pass whose timeout reaches -1 leaves the loop, so the passes stop
	// short of that.
	timeout := int32(binary.LittleEndian.Uint32(slot))
	total, v := c.TotalCycles, *k.Video
	var steps uint64
	var passes int32
	for passes < timeout && total+passCycles < due && steps+passSteps < stepsDue {
		next := v
		for _, d := range video {
			next = float32(next + d)
		}
		if next >= *k.VideoEdge {
			break
		}
		total, v, steps = total+passCycles, next, steps+passSteps
		passes++
	}
	if passes == 0 {
		return 0
	}

	*k.Sample += uint32(total - c.TotalCycles)
	*k.Steps += steps
	*k.Video = v
	c.TotalCycles = total
	binary.LittleEndian.PutUint32(slot, uint32(timeout-passes))
	return steps
}
