package emu

import (
	"io"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu/cpu"
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
		steps := make([]float32, 1+r.IntN(cpu.MaxPassSteps))
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

// Instructions for the loops below, by register number.
func iType(op, rs, rt uint32, imm int) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(imm)&0xffff
}
func rType(funct, rs, rt, rd uint32) uint32 { return rs<<21 | rt<<16 | rd<<11 | funct }

func lw(rt uint32, off int, base uint32) uint32  { return iType(0x23, base, rt, off) }
func lbu(rt uint32, off int, base uint32) uint32 { return iType(0x24, base, rt, off) }
func sw(rt uint32, off int, base uint32) uint32  { return iType(0x2b, base, rt, off) }
func addiu(rt, rs uint32, v int) uint32          { return iType(0x09, rs, rt, v) }
func lui(rt, v uint32) uint32                    { return iType(0x0f, 0, rt, int(v)) }
func subu(rd, rs, rt uint32) uint32              { return rType(0x23, rs, rt, rd) }
func add(rd, rs, rt uint32) uint32               { return rType(0x20, rs, rt, rd) }
func slt(rd, rs, rt uint32) uint32               { return rType(0x2a, rs, rt, rd) }
func jr(rs uint32) uint32                        { return rType(0x08, rs, 0, 0) }
func mfc0(rt, rd uint32) uint32                  { return 0x10<<26 | rt<<16 | rd<<11 }

// The branches take the distance in instructions from the one after them, and
// the jumps an address.
func beq(rs, rt uint32, off int) uint32 { return iType(0x04, rs, rt, off) }
func bne(rs, rt uint32, off int) uint32 { return iType(0x05, rs, rt, off) }
func bgez(rs uint32, off int) uint32    { return iType(0x01, rs, 1, off) }
func bgezal(rs uint32, off int) uint32  { return iType(0x01, rs, 0x11, off) }
func j(addr uint32) uint32              { return 0x02<<26 | addr>>2&0x3ffffff }
func jal(addr uint32) uint32            { return 0x03<<26 | addr>>2&0x3ffffff }

const (
	nop     = uint32(0)
	syscall = uint32(0x0c)
	rfe     = uint32(0x42000010)
	mult    = uint32(0x00430018) // mult $2, $3
)

// The registers the loops use: $4 holds the address of waitCount, $3 what they
// wait for it to pass.
const (
	r1, r2, r3, r4 = uint32(1), uint32(2), uint32(3), uint32(4)
	rK0, rK1       = uint32(26), uint32(27)
	rRA            = uint32(31)

	waitCount = 0x80050000 // where the interrupt handler counts
)

// write puts code in RAM at addr, seen through whichever segment addr is in.
func write(m *Machine, addr uint32, code ...uint32) {
	for i, op := range code {
		m.Bus.Write32(addr+4*uint32(i), op)
	}
}

// startAt sets the CPU going at addr.
func startAt(m *Machine, addr uint32) { m.CPU.PC, m.CPU.NextPC = addr, addr+4 }

// countBlanks makes the vertical blank interrupt count at waitCount, as a
// library's handler would.
func countBlanks(m *Machine, irqs uint32) {
	const handler = 0x80000080
	write(m, handler,
		lui(rK0, waitCount>>16),
		lw(rK1, 0, rK0),
		nop,
		addiu(rK1, rK1, 1),
		sw(rK1, 0, rK0),
		lui(rK0, 0x1f80),
		sw(0, 0x1070, rK0), // acknowledge everything
		mfc0(rK0, 14),
		nop,
		jr(rK0),
		rfe,
	)
	m.Bus.Write32(0x1f801074, irqs)    // I_MASK
	m.CPU.COP0[cpu.SR] |= 1<<0 | 1<<10 // interrupts on, the controller's line unmasked
}

// waitAgain is what a wait returns to at at: having seen the count pass, it
// asks for the next one and goes back to the head of the wait.
func waitAgain(head uint32) []uint32 {
	return []uint32{
		addiu(r3, r3, 1),
		j(head),
		nop,
	}
}

// compareRuns runs the machine setup makes twice, skipping and stepping, for
// steps in uneven pieces, checking they agree after every piece, and returns
// the steps the first skipped.
func compareRuns(t *testing.T, steps uint64, setup func(m *Machine)) uint64 {
	t.Helper()
	boot := func(skip bool) *Machine {
		m := newTestMachine(t)
		m.SkipVSyncWait(skip)
		setup(m)
		return m
	}
	skipped, stepped := boot(true), boot(false)
	r := rand.New(rand.NewPCG(1, 2))
	for left := steps; left > 0; {
		n := min(left, 1+uint64(r.IntN(200_000)))
		left -= n
		for _, m := range []*Machine{skipped, stepped} {
			ran, err := m.Run(n)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			assert.Equal(t, n, ran)
		}
		a, b := skipped.CPU, stepped.CPU
		assert.Equal(t, b.R, a.R, "registers")
		assert.Equal(t, [2]uint32{b.HI, b.LO}, [2]uint32{a.HI, a.LO}, "HI and LO")
		assert.Equal(t, b.COP0, a.COP0, "COP0")
		assert.Equal(t, [2]uint32{b.PC, b.NextPC}, [2]uint32{a.PC, a.NextPC}, "PC")
		assert.Equal(t, b.TotalCycles, a.TotalCycles, "cycles")
		assert.Equal(t, stepped.steps, skipped.steps, "steps")
		assert.Equal(t, stepped.spuCycles, skipped.spuCycles, "SPU cycles")
		assert.Equal(t, stepped.gpu.cycles, skipped.gpu.cycles, "GPU cycles")
		assert.Equal(t, stepped.gpu.frames, skipped.gpu.frames, "frames")
		assert.Equal(t, stepped.Bus.Read16(0x1f801120), skipped.Bus.Read16(0x1f801120), "timer 2")
		assert.Equal(t, true, slices.Equal(stepped.ram.memory, skipped.ram.memory), "RAM")
		if t.Failed() {
			t.FailNow()
		}
	}
	return skipped.spin.skipped
}

// TestSkipQuietLoops runs waits for the vertical blank in RAM, at addresses
// and in shapes no library has, and checks they are skipped through and that
// doing so changes nothing.
func TestSkipQuietLoops(t *testing.T) {
	for _, c := range []struct {
		name string
		at   uint32
		code func(head uint32) []uint32
	}{
		{
			name: "subtract and branch",
			at:   0x80012340,
			code: func(head uint32) []uint32 {
				return []uint32{lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop}
			},
		},
		{
			name: "in uncached RAM",
			at:   0xa0123450,
			code: func(head uint32) []uint32 {
				return []uint32{lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop}
			},
		},
		{
			name: "through the low segment at the top of RAM",
			at:   0x001fff00,
			code: func(head uint32) []uint32 {
				return []uint32{lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop}
			},
		},
		{
			name: "a load in the delay slot, landing at the head",
			at:   0x80020000,
			code: func(head uint32) []uint32 {
				return []uint32{subu(r1, r3, r2), bgez(r1, -2), lw(r2, 0, r4)}
			},
		},
		{
			name: "branching forward within the pass, and closing with a jump",
			at:   0x80033330,
			code: func(head uint32) []uint32 {
				return []uint32{
					lw(r2, 0, r4), nop, slt(r1, r3, r2), beq(r1, 0, 2), nop,
					beq(0, 0, 4), nop, // on past the jump: the count has passed
					lbu(r2, 0, r4), j(head), lui(r1, 0x1234),
				}
			},
		},
		{
			name: "as long as a pass can be",
			at:   0x80040000,
			code: func(head uint32) []uint32 {
				code := make([]uint32, cpu.MaxPassSteps-5)
				return append(code, lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, 1-cpu.MaxPassSteps), nop)
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			skipped := compareRuns(t, 3_000_000, func(m *Machine) {
				countBlanks(m, irqVBlank)
				write(m, c.at, c.code(c.at)...)
				write(m, c.at+4*uint32(len(c.code(c.at))), waitAgain(c.at)...)
				m.CPU.R[r4] = waitCount
				startAt(m, c.at)
			})
			assert.Equal(t, true, skipped > 2_000_000, "skipped %d of 3000000 steps", skipped)
		})
	}
}

// TestSkipEndsAtEveryDevice waits while other things come due: a timer's
// interrupt, which raises the count at a rate that isn't a frame's, and a DMA
// that rewrites the word polled, with no interrupt to say so.
func TestSkipEndsAtEveryDevice(t *testing.T) {
	const at = 0x80012340
	loop := []uint32{lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop}
	t.Run("timer", func(t *testing.T) {
		skipped := compareRuns(t, 2_000_000, func(m *Machine) {
			countBlanks(m, irqVBlank|irqTimer2)
			m.Bus.Write16(0x1f801128, 9000)   // timer 2's target
			m.Bus.Write16(0x1f801124, 0x0058) // reset at target, interrupt at target, repeat
			write(m, at, loop...)
			write(m, at+4*uint32(len(loop)), waitAgain(at)...)
			m.CPU.R[r4] = waitCount
			startAt(m, at)
		})
		assert.Equal(t, true, skipped > 1_000_000, "skipped %d of 2000000 steps", skipped)
	})
	t.Run("DMA", func(t *testing.T) {
		compareRuns(t, 500_000, func(m *Machine) {
			write(m, at, lw(r2, 0, r4), nop, beq(r2, r3, -3), nop, j(at+16), nop)
			m.Bus.Write32(0x1f8010f0, 0x08000000) // enable channel 6
			m.Bus.Write32(0x1f8010e0, 0x00060000)
			m.Bus.Write32(0x1f8010e4, 0x4000)
			m.Bus.Write32(0x1f8010e8, 0x11000002) // clear a table, downward through the word polled
			m.CPU.R[r4] = 0x8005f000
			startAt(m, at)
		})
	})
}

// TestNoSkipUnlessQuiet runs loops that a pass of does something, and checks
// they are not skipped through, and that they run as they would anyway.
func TestNoSkipUnlessQuiet(t *testing.T) {
	const at = 0x80012340
	for _, c := range []struct {
		name string
		code []uint32
		r4   uint32
	}{
		{
			name: "stores",
			code: []uint32{lw(r2, 0, r4), nop, sw(r2, 4, r4), beq(r2, r3, -4), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "counts down in memory",
			code: []uint32{lw(r2, 0, r4), nop, addiu(r2, r2, -1), sw(r2, 0, r4), bne(r2, r3, -5), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "reads a device",
			code: []uint32{lw(r2, 0x1814, r4), nop, j(at), nop},
			r4:   0x1f800000,
		},
		{
			name: "reads a timer through the uncached segment",
			code: []uint32{lbu(r2, 0x1120, r4), nop, j(at), nop},
			r4:   0xbf800000,
		},
		{
			name: "reads the scratchpad",
			code: []uint32{lw(r2, 0, r4), nop, j(at), nop},
			r4:   0x1f800000,
		},
		{
			name: "does not restore a register",
			code: []uint32{addiu(r2, r2, 1), bne(r2, r3, -2), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "takes an address error",
			code: []uint32{lw(r2, 1, r4), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "calls",
			code: []uint32{jal(at + 16), nop, j(at), nop, jr(rRA), nop},
			r4:   waitCount,
		},
		{
			name: "links on a branch",
			code: []uint32{bgezal(0, 0), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "reads a coprocessor",
			code: []uint32{mfc0(r2, 12), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "makes a system call",
			code: []uint32{syscall, nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "multiplies",
			code: []uint32{mult, nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "may overflow",
			code: []uint32{add(r2, r3, r3), nop, j(at), nop},
			r4:   waitCount,
		},
		{
			name: "is longer than a pass can be",
			code: append(make([]uint32, cpu.MaxPassSteps), j(at), nop),
			r4:   waitCount,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			skipped := compareRuns(t, 500_000, func(m *Machine) {
				write(m, at, c.code...)
				m.CPU.R[r4] = c.r4
				startAt(m, at)
			})
			assert.Equal(t, uint64(0), skipped)
		})
	}
}

// TestNoSkipPastAStop checks a loop with a breakpoint in it, or the BIOS call
// hook, is run step by step, so that everything watching it sees every pass,
// while a breakpoint elsewhere leaves the loop to be skipped.
func TestNoSkipPastAStop(t *testing.T) {
	const at = 0x80012340
	loop := []uint32{lw(r2, 0, r4), nop, beq(r2, 0, -3), nop}
	for _, c := range []struct {
		name  string
		bp    uint32
		wants uint64
	}{
		{
			name:  "breakpoint at the head",
			bp:    at,
			wants: 12_500,
		},
		{
			name:  "breakpoint in the delay slot",
			bp:    at + 12,
			wants: 12_500,
		},
		{
			name:  "breakpoint elsewhere",
			bp:    0x80020000,
			wants: 0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var hits uint64
			skipped := compareRuns(t, 50_000, func(m *Machine) {
				write(m, at, loop...)
				m.CPU.R[r4] = waitCount
				startAt(m, at)
				// Two machines count here, each every pass it steps.
				m.SetBreak(c.bp, func(*Machine) bool { hits++; return false })
			})
			assert.Equal(t, 2*c.wants, hits)
			assert.Equal(t, c.wants == 0, skipped > 0, "skipped %d steps", skipped)
		})
	}
	t.Run("the BIOS call vector", func(t *testing.T) {
		skipped := compareRuns(t, 50_000, func(m *Machine) {
			m.TTY = io.Discard
			write(m, 0x80000090, make([]uint32, 12)...)
			write(m, 0x800000c0, beq(0, 0, -12), nop)
			startAt(m, 0x80000090)
		})
		assert.Equal(t, uint64(0), skipped)
	})
}

// TestSkipCoversAStartInAnyStep stops the machine at every step of a pass, and
// checks a skip from there, delay slots included, comes out the same.
func TestSkipCoversAStartInAnyStep(t *testing.T) {
	const at = 0x80012340
	for offset := range uint64(5) {
		compareRuns(t, 100_000+offset, func(m *Machine) {
			write(m, at, lw(r1, 0, r4), nop, subu(r1, r3, r1), bgez(r1, -4), nop)
			m.CPU.R[r4] = waitCount
			startAt(m, at)
			// A step at a time, as the skip won't with a budget of one.
			for range offset {
				assert.NoError(t, m.Step())
			}
		})
	}
}
