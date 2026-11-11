package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// stepCosts runs the machine n steps and returns what each cost.
func stepCosts(t *testing.T, m *Machine, n int) []uint32 {
	t.Helper()
	costs := make([]uint32, n)
	for i := range costs {
		if !assert.NoError(t, m.Step()) {
			t.FailNow()
		}
		costs[i] = m.CPU.LastCycles
	}
	return costs
}

// TestCPUCostCache runs a line of code cold and then from the cache, and
// through KSEG1, where it is never cached.
func TestCPUCostCache(t *testing.T) {
	const at = 0x80010000 // the start of a cache line
	m := newTestMachine(t)
	m.SetCPUCost(true)
	write(m, at, nop, nop, nop, nop, nop)

	// Jumped into, the line fills from the first word: 3 cycles, 1.5 a
	// word, 2 for the jump, and its other words come with it.
	startAt(m, at)
	assert.Equal(t, []uint32{3 + 6 + 2, 0, 0, 0}, stepCosts(t, m, 4))
	// Run into, the next line costs no jump, and only the words from the
	// one entered.
	assert.Equal(t, []uint32{3 + 6}, stepCosts(t, m, 1))

	startAt(m, at)
	assert.Equal(t, []uint32{1, 1, 1, 1}, stepCosts(t, m, 4), "from the cache")

	startAt(m, at+8)
	m.SetCPUCost(true) // an empty cache
	assert.Equal(t, []uint32{3 + 3 + 2, 0}, stepCosts(t, m, 2), "entered halfway")

	startAt(m, at|0x20000000)
	assert.Equal(t, []uint32{6, 6, 6}, stepCosts(t, m, 3), "uncached")

	m.SetCPUCost(false)
	startAt(m, at)
	assert.Equal(t, []uint32{2, 2, 2}, stepCosts(t, m, 3), "with the model off")
}

// TestCPUCostLoads runs loads and stores from the cache.
func TestCPUCostLoads(t *testing.T) {
	const (
		at   = 0x80010000
		data = 0x80020000
		r5   = uint32(5)
	)
	m := newTestMachine(t)
	m.SetCPUCost(true)
	code := []uint32{
		lw(r1, 0, r4), lw(r2, 4, r4), nop, // a run of loads, then one used
		lw(r1, 0, r5), lw(r2, 4, r5), // the scratchpad
		lw(r1, 0x1814, r5),          // GPUSTAT
		sw(0, 0, r4), lw(r1, 8, r4), // a store, then a load waiting for it
		mult, mflo(r1),
	}
	write(m, at, code...)
	m.CPU.R[r4], m.CPU.R[r5] = data, 0x1f800000
	m.CPU.R[r2], m.CPU.R[r3] = 1000, 7
	startAt(m, at)
	stepCosts(t, m, len(code)) // into the cache

	startAt(m, at)
	assert.Equal(t, []uint32{
		6, 6, 1 + 1,
		1, 1,
		5,
		1, 2 + 1 + 5 + 3,
		1 + 1, 6 + 1,
	}, stepCosts(t, m, len(code)))
}

func mflo(rd uint32) uint32 { return rType(0x12, 0, 0, rd) }

// TestCPUCostOTC clears an ordering table: the step after the one starting
// it runs, and the next to reach the bus waits for the transfer.
func TestCPUCostOTC(t *testing.T) {
	const (
		at = 0x80010000
		r5 = uint32(5)
	)
	m := newTestMachine(t)
	m.SetCPUCost(true)
	m.Bus.Write32(0x1f8010f0, 0x08000000) // enable channel 6
	code := []uint32{
		nop,                                    // 2 after the warm-up's load
		sw(r2, 0x10e0, r5), sw(r3, 0x10e4, r5), // MADR and BCR
		sw(r1, 0x10e8, r5), // CHCR: start
		nop,                // the transfer starts after this one
		nop,
		nop, // by when the write buffer has drained
		lw(r2, 0, r4),
	}
	write(m, at, code...)
	m.CPU.R[r1], m.CPU.R[r2], m.CPU.R[r3] = 0x11000002, 0x00060ffc, 1024
	m.CPU.R[r4], m.CPU.R[r5] = 0x80020000, 0x1f800000
	startAt(m, at)
	stepCosts(t, m, len(code)) // into the cache

	m.CPU.R[r2] = 0x00060ffc
	startAt(m, at)
	costs := stepCosts(t, m, len(code))
	assert.Equal(t, []uint32{2, 1, 1, 1, 1, 1, 1}, costs[:7], "the cached steps run on")
	assert.Equal(t, uint32(1024*otcHoldPerMille/1000-2+1+5), costs[7], "the load waits out the transfer")
}

// TestCPUCostSkip checks that skipping through a wait under the cost model
// comes to what stepping through it does.
func TestCPUCostSkip(t *testing.T) {
	for _, at := range []uint32{0x80012340, 0xa0123450} {
		skipped := compareRuns(t, 3_000_000, func(m *Machine) {
			m.SetCPUCost(true)
			countBlanks(m, irqVBlank)
			loop := []uint32{lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop}
			write(m, at, loop...)
			write(m, at+4*uint32(len(loop)), waitAgain(at)...)
			m.CPU.R[r4] = waitCount
			startAt(m, at)
		})
		assert.Equal(t, true, skipped > 2_000_000, "at %08x, skipped %d of 3000000 steps", at, skipped)
	}
}
