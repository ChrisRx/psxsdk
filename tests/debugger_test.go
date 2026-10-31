package tests

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/serial"
)

// The remote debugger the monitor's rdb starts, in HSS, the protocol of
// the Net Yaroze's gdb, checked against what Sony's answers.

// vsync is the resident library's VSync, which a program calls each frame.
const vsync = 0x8003ca68

// breakInsn is the instruction gdb plants for a breakpoint: break 5.
var breakInsn = []byte{0x0d, 0, 5, 0}

// waitText waits for the program to print want, which comes in SEND
// packets while it runs, collected in text.
func waitText(t *testing.T, d *serial.Debugger, text *bytes.Buffer, want string) {
	t.Helper()
	end := time.Now().Add(20 * time.Second)
	for !strings.Contains(text.String(), want) {
		if time.Now().After(end) {
			t.Fatalf("waiting for %q, printed %q", want, text.String())
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		d.Wait(ctx)
		cancel()
	}
}

func TestMonitorDebugger(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, _, mon := boot(t)
		ctx := t.Context()
		prog, err := serial.OpenProgram(serialProgram(t, "print"))
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		assert.NoError(t, mon.Load(ctx, prog))
		d, err := mon.RemoteDebugger(ctx)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var text bytes.Buffer
		d.Output = &text

		sp, err := d.ReadRegister(ctx, 29)
		assert.NoError(t, err)
		assert.Equal(t, prog.SP, sp, "the registers the monitor had")
		r, err := d.Request(ctx, serial.HSSPacket{Code: serial.HSSQuery})
		assert.NoError(t, err)
		assert.Equal(t, []byte{5, 0, 0, 0}, r.Data, "stopped, for a trap")

		assert.NoError(t, d.WriteMemory(ctx, 0x80100000, []byte{1, 2, 3, 4}))
		assert.Equal(t, uint32(0x04030201), m.Read32(0x80100000))
		b, err := d.ReadMemory(ctx, 0x80100000, 4)
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3, 4}, b)
		r, err = d.Request(ctx, serial.HSSPacket{Code: 20})
		assert.Equal(t, uint8(serial.HSSUnimp), r.Code, "a request it does not know")

		// gdb runs a program by pointing epc at it, with a breakpoint
		// where it wants it to stop.
		orig, err := d.ReadMemory(ctx, vsync, 4)
		assert.NoError(t, err)
		assert.NoError(t, d.WriteRegister(ctx, serial.HSSRegEPC, prog.PC))
		assert.NoError(t, d.WriteMemory(ctx, vsync, breakInsn))
		assert.NoError(t, d.Continue(ctx))
		sig, err := d.Wait(ctx)
		assert.NoError(t, err)
		assert.Equal(t, uint32(5), sig)
		pc, _ := d.ReadRegister(ctx, serial.HSSRegEPC)
		assert.Equal(t, uint32(vsync), pc, "stopped at the breakpoint")
		assert.Equal(t, true, strings.Contains(text.String(), "hello from the console"), text.String())

		assert.NoError(t, d.WriteMemory(ctx, vsync, orig))
		for _, want := range []uint32{vsync + 4, vsync + 8} {
			assert.NoError(t, d.Step(ctx))
			sig, err = d.Wait(ctx)
			assert.NoError(t, err)
			assert.Equal(t, uint32(5), sig)
			pc, _ = d.ReadRegister(ctx, serial.HSSRegEPC)
			assert.Equal(t, want, pc, "a step")
		}
		// Steps follow the branches, through to VSync's return to its
		// caller, which the breakpoint left in ra.
		ra, _ := d.ReadRegister(ctx, 31)
		steps := 0
		for pc != ra && steps < 300 {
			assert.NoError(t, d.Step(ctx))
			_, err = d.Wait(ctx)
			assert.NoError(t, err)
			pc, _ = d.ReadRegister(ctx, serial.HSSRegEPC)
			steps++
		}
		assert.Equal(t, ra, pc, "stepped out of VSync")
		assert.NoError(t, d.Continue(ctx))
		waitText(t, d, &text, "second 1")
	})
}

func TestMonitorDebuggerCrash(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, _, mon := boot(t, emutest.AllowFaults())
		ctx := t.Context()
		prog, err := serial.OpenProgram(serialProgram(t, "crash"))
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		assert.NoError(t, mon.Load(ctx, prog))
		d, err := mon.RemoteDebugger(ctx)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var text bytes.Buffer
		d.Output = &text
		assert.NoError(t, d.WriteRegister(ctx, serial.HSSRegEPC, prog.PC))
		assert.NoError(t, d.Continue(ctx))
		sig, err := d.Wait(ctx)
		assert.NoError(t, err)
		assert.Equal(t, uint32(10), sig, "an address error, as SIGBUS")
		cr, _ := d.ReadRegister(ctx, serial.HSSRegCR)
		assert.Equal(t, uint32(4), cr>>2&31, "AdEL")
		assert.Equal(t, "crashing\r\r\n", text.String())
		r, err := d.Request(ctx, serial.HSSPacket{Code: serial.HSSQuery})
		assert.NoError(t, err)
		assert.Equal(t, []byte{10, 0, 0, 0}, r.Data)
	})
}
