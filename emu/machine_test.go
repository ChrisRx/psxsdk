package emu

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// newTestMachine is a machine with an empty BIOS, for exercising devices
// through the bus without running anything.
func newTestMachine(t testing.TB) *Machine {
	t.Helper()
	m, err := New(make([]byte, 512<<10))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return m
}

func TestDMAClearsOrderingTable(t *testing.T) {
	m := newTestMachine(t)
	const (
		madr = 0x1f8010e0
		bcr  = 0x1f8010e4
		chcr = 0x1f8010e8
		dpcr = 0x1f8010f0
	)
	m.Bus.Write32(dpcr, 0x08000000) // enable channel 6
	m.Bus.Write32(madr, 0x0000100c)
	m.Bus.Write32(bcr, 4)
	m.Bus.Write32(chcr, 0x11000002) // start, busy, backwards

	var table [4]uint32
	for i := range table {
		table[i] = m.Bus.Read32(0x1000 + 4*uint32(i))
	}
	// Each entry points at the one before it, and the first ends the list.
	assert.Equal(t, [4]uint32{0xffffff, 0x1000, 0x1004, 0x1008}, table)
	assert.Equal(t, uint32(0), m.Bus.Read32(chcr)&0x01000000, "the channel should no longer be busy")
}

func TestSPUKeyOnOff(t *testing.T) {
	m := newTestMachine(t)
	const (
		envx = 0x1f801c0c // voice 0
		kon  = 0x1f801d88
		koff = 0x1f801d8c
		endx = 0x1f801d9c
	)
	assert.Equal(t, uint32(0x00ffffff), m.Bus.Read32(endx), "every voice starts out ended")

	m.Bus.Write16(kon, 1)
	assert.Equal(t, uint32(0x00fffffe), m.Bus.Read32(endx))
	assert.Equal(t, uint16(0x7fff), m.Bus.Read16(envx))

	m.Bus.Write16(koff, 1)
	assert.Equal(t, uint32(0x00ffffff), m.Bus.Read32(endx))
}

func TestGPUVRAMRoundTrip(t *testing.T) {
	m := newTestMachine(t)
	const gp0 = 0x1f801810

	// Upload a 2x2 rectangle at (100, 50), then read it back.
	pixels := [4]uint16{0x1111, 0x2222, 0x3333, 0x4444}
	m.Bus.Write32(gp0, 0xa0000000)
	m.Bus.Write32(gp0, 50<<16|100)
	m.Bus.Write32(gp0, 2<<16|2)
	m.Bus.Write32(gp0, uint32(pixels[1])<<16|uint32(pixels[0]))
	m.Bus.Write32(gp0, uint32(pixels[3])<<16|uint32(pixels[2]))

	m.Bus.Write32(gp0, 0xc0000000)
	m.Bus.Write32(gp0, 50<<16|100)
	m.Bus.Write32(gp0, 2<<16|2)
	got := [2]uint32{m.Bus.Read32(gp0), m.Bus.Read32(gp0)}
	assert.Equal(t, [2]uint32{0x22221111, 0x44443333}, got)
}

func TestGPUSTAT(t *testing.T) {
	m := newTestMachine(t)
	const (
		gp0     = 0x1f801810
		gp1     = 0x1f801814
		gpustat = 0x1f801814
	)
	assert.Equal(t, uint32(0x1c800000), m.Bus.Read32(gpustat), "display off and ready")

	m.Bus.Write32(gp1, 0x03000000) // display on
	m.Bus.Write32(gp0, 0xe100060a) // draw mode: texture page and dithering bits
	assert.Equal(t, uint32(0x1c00060a), m.Bus.Read32(gpustat))
}

func TestVBlankInterrupt(t *testing.T) {
	m := newTestMachine(t)
	m.Bus.Write32(0x1f801074, irqVBlank) // I_MASK

	// The empty BIOS is all nops, which cost 20 cycles each. A frame is
	// 263 lines of 3413 GPU cycles, the vertical blank starting at line
	// 240: about 0.86 of a frame in, or 30k steps.
	for i := 0; m.ic.stat&irqVBlank == 0; i++ {
		if i == 100_000 {
			t.Fatal("no vertical blank interrupt in 100k steps")
		}
		if !assert.NoError(t, m.Step()) {
			t.FailNow()
		}
	}
	assert.Equal(t, uint32(1<<10), m.CPU.COP0[13]&(1<<10), "the CPU's interrupt line should be up")
}

func TestTimerTargetInterrupt(t *testing.T) {
	m := newTestMachine(t)
	const (
		t2Mode   = 0x1f801124
		t2Target = 0x1f801128
	)
	m.Bus.Write32(0x1f801074, irqTimer2) // I_MASK
	m.Bus.Write16(t2Target, 100)
	m.Bus.Write16(t2Mode, 0x0058) // reset at target, interrupt at target, repeat

	// psxe advances the timers 2 a step and fires once past the target.
	for range 50 {
		m.Step()
	}
	assert.Equal(t, uint32(0), m.ic.stat&irqTimer2, "not yet past the target")
	m.Step()
	assert.Equal(t, uint32(irqTimer2), m.ic.stat&irqTimer2)
}

// TestRunTakesTimerInterrupt checks Run hands back to the timers at the step
// their interrupt is due, so the CPU takes it where it would a step at a
// time, rather than once the timers next catch up.
func TestRunTakesTimerInterrupt(t *testing.T) {
	setup := func() *Machine {
		m := newTestMachine(t)
		m.Bus.Write32(0x1f801074, irqTimer2) // I_MASK
		m.Bus.Write16(0x1f801128, 100)       // timer 2's target
		m.Bus.Write16(0x1f801124, 0x0058)    // reset at target, interrupt at target, repeat
		m.CPU.COP0[12] |= 1 << 0             // interrupts enabled
		m.CPU.COP0[12] |= 1 << 10            // the interrupt controller's line unmasked
		return m
	}
	stepped := setup()
	for range 200 {
		stepped.Step()
	}
	run := setup()
	ran, err := run.Run(200)
	assert.NoError(t, err)
	assert.Equal(t, uint64(200), ran)
	assert.Equal(t, uint32(0), stepped.CPU.COP0[13]>>2&0x1f, "the interrupt should have been taken")
	assert.Equal(t, stepped.CPU.COP0[14], run.CPU.COP0[14], "EPC")
	assert.Equal(t, stepped.CPU.PC, run.CPU.PC, "PC")
}

// TestBootKernel boots a real BIOS until the kernel reports it is set up.
// The BIOS is Sony's and cannot be committed, so it is read from the file
// PSX_BIOS names, and the test is skipped without one.
func TestBootKernel(t *testing.T) {
	path := os.Getenv("PSX_BIOS")
	if path == "" {
		t.Skip("PSX_BIOS is not set")
	}
	bios, err := os.ReadFile(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m, err := New(bios)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var tty bytes.Buffer
	m.TTY = &tty

	for range 5_000_000 {
		if !assert.NoError(t, m.Step()) {
			t.FailNow()
		}
		if bytes.Contains(tty.Bytes(), []byte("KERNEL SETUP!")) {
			return
		}
	}
	t.Fatalf("no KERNEL SETUP! in 5M instructions; the TTY shows:\n%s", tty.String())
}

// TestRunMatchesStep boots OpenBIOS twice, once a step at a time and once
// with Run, which lets the CPU run steps itself between the ones the devices
// need, and checks the two machines end up the same.
func TestRunMatchesStep(t *testing.T) {
	bios, err := os.ReadFile("openbios/openbios.bin")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	boot := func() (*Machine, *bytes.Buffer) {
		m, err := New(bios)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var tty bytes.Buffer
		m.TTY = &tty
		return m, &tty
	}
	const n = 20_000_000
	stepped, steppedTTY := boot()
	for range n {
		if !assert.NoError(t, stepped.Step()) {
			t.FailNow()
		}
	}
	run, runTTY := boot()
	// In uneven pieces, so that Run is also stopped partway by its budget.
	for left := uint64(n); left > 0; {
		ran, err := run.Run(min(left, 123_457))
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		left -= ran
	}

	a, b := stepped.CPU, run.CPU
	assert.Equal(t, a.R, b.R, "registers")
	assert.Equal(t, [2]uint32{a.HI, a.LO}, [2]uint32{b.HI, b.LO}, "HI and LO")
	assert.Equal(t, a.COP0, b.COP0, "COP0")
	assert.Equal(t, [2]uint32{a.PC, a.NextPC}, [2]uint32{b.PC, b.NextPC}, "PC")
	assert.Equal(t, a.TotalCycles, b.TotalCycles, "cycles")
	assert.Equal(t, stepped.steps, run.steps, "steps")
	assert.Equal(t, stepped.spuCycles, run.spuCycles, "SPU cycles")
	assert.Equal(t, stepped.gpu.cycles, run.gpu.cycles, "GPU cycles")
	assert.Equal(t, stepped.gpu.frames, run.gpu.frames, "frames")
	assert.Equal(t, true, stepped.gpu.frames > 0, "the BIOS should have shown frames")
	assert.Equal(t, true, slices.Equal(stepped.VRAM(), run.VRAM()), "VRAM")
	assert.Equal(t, steppedTTY.String(), runTTY.String(), "TTY")
}
