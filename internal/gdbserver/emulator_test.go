package gdbserver

import (
	"encoding/binary"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/psx"
)

// The program the emulator's tests run, at 80010000: a count in t0 that a
// loop adds to for ever, and after the loop, never reached by it, a load
// from an odd address. The load in the loop is only there for
// TestEmulatorLoadDelay: the others run a nop in its place.
const (
	progBase  = 0x80010000
	progLoop  = progBase + 0x4
	progBne   = progBase + 0xc
	progFault = progBase + 0x14
)

var program = []uint32{
	0x24080000, // 00: addiu t0, zero, 0
	0x25080001, // 04: addiu t0, t0, 1
	0x8d090000, // 08: lw t1, 0(t0)
	0x1500fffd, // 0c: bne t0, zero, 04
	0x00000000, // 10: nop, its delay slot
	0x8c0a0001, // 14: lw t2, 1(zero)
	0x00000000, // 18: nop
}

// setupEmulator boots OpenBIOS as far as its shell and loads the program in
// its place, as yaroze debug does an executable, with the load at 08 made
// a nop.
func setupEmulator(t *testing.T) (*Emulator, *emu.Machine) {
	t.Helper()
	m, err := emu.New(openbios.Image)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	code := make([]byte, 2048)
	for i, w := range program {
		if i == 2 {
			w = 0
		}
		binary.LittleEndian.PutUint32(code[i*4:], w)
	}
	if !assert.NoError(t, m.SideloadEXE(psx.New(progBase, progBase, code), 50_000_000)) {
		t.FailNow()
	}
	e := NewEmulator(m)
	e.Fast = true
	return e, m
}

func TestEmulatorStep(t *testing.T) {
	e, m := setupEmulator(t)
	ctx := t.Context()
	reg := func(n int) uint32 {
		v, err := e.ReadRegister(ctx, n)
		assert.NoError(t, err)
		return v
	}
	step := func() Stop {
		assert.NoError(t, e.Step(ctx))
		stop, err := e.Wait(ctx)
		assert.NoError(t, err)
		return stop
	}
	assert.Equal(t, uint32(progBase), reg(RegPC))
	assert.Equal(t, Stop{Signal: SIGTRAP}, step())
	assert.Equal(t, uint32(progLoop), reg(RegPC))
	step()
	assert.Equal(t, uint32(1), reg(8))
	step()
	assert.Equal(t, uint32(progBne), reg(RegPC))
	// The branch and its delay slot are one step.
	step()
	assert.Equal(t, uint32(progLoop), reg(RegPC), "the branch taken, its delay slot run")
	assert.Equal(t, false, m.CPU.InDelaySlot())

	// Registers and memory are the machine's.
	assert.NoError(t, e.WriteRegister(ctx, 8, 0x100))
	step()
	assert.Equal(t, uint32(0x101), m.CPU.R[8])
	assert.NoError(t, e.WriteMemory(ctx, 0x80100000, []byte{1, 2, 3, 4}))
	assert.Equal(t, uint32(0x04030201), m.Bus.Read32(0x80100000))
	b, err := e.ReadMemory(ctx, progBase, 4)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x00, 0x08, 0x24}, b)

	// Setting the pc moves the program there.
	assert.NoError(t, e.WriteRegister(ctx, RegPC, progBase))
	step()
	assert.Equal(t, uint32(0), reg(8))
	assert.Equal(t, uint32(progLoop), reg(RegPC))
}

// TestEmulatorLoadDelay checks a register reads as the program will see
// it: the word a load is still bringing, an instruction late on an R3000,
// is there already.
func TestEmulatorLoadDelay(t *testing.T) {
	e, m := setupEmulator(t)
	ctx := t.Context()
	m.Bus.Write32(progBase+8, program[2])
	m.Bus.Write32(1, 0)
	for range 2 {
		assert.NoError(t, e.Step(ctx))
		e.Wait(ctx)
	}
	// t0 is 1, so the load is from an odd address: use one that is not.
	assert.NoError(t, e.WriteRegister(ctx, 8, progBase))
	assert.NoError(t, e.Step(ctx))
	e.Wait(ctx)
	v, err := e.ReadRegister(ctx, 9)
	assert.NoError(t, err)
	assert.Equal(t, program[0], v, "the word at 80010000, loaded into t1")
}

func TestEmulatorBreakpoint(t *testing.T) {
	e, m := setupEmulator(t)
	ctx := t.Context()
	assert.Equal(t, ErrUnsupported, e.SetBreakpoint(WatchWrite, progBase, 4))
	assert.NoError(t, e.SetBreakpoint(BreakSoftware, progBne, 4))
	assert.Equal(t, uint32(program[3]), m.Bus.Read32(progBne), "nothing is written into the program")
	for want := uint32(1); want <= 3; want++ {
		assert.NoError(t, e.Continue(ctx))
		stop, err := e.Wait(ctx)
		assert.NoError(t, err)
		assert.Equal(t, Stop{Signal: SIGTRAP}, stop)
		assert.Equal(t, uint32(progBne), m.CPU.PC)
		assert.Equal(t, want, m.CPU.R[8], "each time round the loop")
	}
	// A step from a breakpoint moves on from it.
	assert.NoError(t, e.Step(ctx))
	e.Wait(ctx)
	assert.Equal(t, uint32(progLoop), m.CPU.PC)
	assert.NoError(t, e.ClearBreakpoint(BreakSoftware, progBne, 4))
}

func TestEmulatorInterrupt(t *testing.T) {
	e, m := setupEmulator(t)
	ctx := t.Context()
	assert.NoError(t, e.Continue(ctx))
	go func() {
		time.Sleep(50 * time.Millisecond)
		e.Interrupt()
	}()
	stop, err := e.Wait(ctx)
	assert.NoError(t, err)
	assert.Equal(t, Stop{Signal: SIGINT}, stop)
	pc := m.CPU.PC
	assert.Equal(t, true, pc >= progLoop && pc <= progBne, "in the loop, not the kernel or a delay slot")
	assert.Equal(t, true, m.CPU.R[8] > 1000)
}

// TestEmulatorFault runs the load from an odd address: the program stops
// at it, for a bus error, rather than in the kernel's handler.
func TestEmulatorFault(t *testing.T) {
	for _, run := range []string{"step", "continue"} {
		t.Run(run, func(t *testing.T) {
			e, m := setupEmulator(t)
			ctx := t.Context()
			assert.NoError(t, e.WriteRegister(ctx, RegPC, progFault))
			sr := m.CPU.COP0[cpu.SR]
			if run == "step" {
				assert.NoError(t, e.Step(ctx))
			} else {
				assert.NoError(t, e.Continue(ctx))
			}
			stop, err := e.Wait(ctx)
			assert.NoError(t, err)
			assert.Equal(t, Stop{Signal: SIGBUS}, stop)
			assert.Equal(t, uint32(progFault), m.CPU.PC)
			assert.Equal(t, sr, m.CPU.COP0[cpu.SR], "the status register as it was")
			bad, _ := e.ReadRegister(ctx, RegBadVAddr)
			assert.Equal(t, uint32(1), bad)
			cause, _ := e.ReadRegister(ctx, RegCause)
			assert.Equal(t, uint32(cpu.ExcAdEL), cause>>2&31)
		})
	}
}
