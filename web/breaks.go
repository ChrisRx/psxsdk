//go:build js

package main

import (
	"strconv"
	"syscall/js"

	"go.chrisrx.dev/psx/emu"
)

// hits counts, by address, the breakpoints reached since the last run
// reported them.
var hits map[uint32]int

// setBreak sets a breakpoint at an address, which the program's symbol
// gives: the run reports each time it is reached and, with stop, stops
// there, returning {stopped: address}. It returns whether there was a
// machine to set it in.
func setBreak(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) < 1 {
		return false
	}
	addr := uint32(args[0].Int())
	stop := len(args) > 1 && args[1].Truthy()
	machine.SetBreak(addr, func(*emu.Machine) bool {
		if hits == nil {
			hits = map[uint32]int{}
		}
		hits[addr]++
		return stop
	})
	return true
}

// clearBreak takes the breakpoint at an address away.
func clearBreak(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) < 1 {
		return false
	}
	machine.ClearBreak(uint32(args[0].Int()))
	return true
}

// step runs one instruction, which is how a run stopped at a breakpoint
// moves past it, and returns {pc}, or {stopped: address} when that
// instruction brought it to a breakpoint that stops, or {error}.
func step(js.Value, []js.Value) any {
	if machine == nil {
		return map[string]any{"error": "nothing is running"}
	}
	if err := machine.Step(); err == emu.ErrBreak {
		return withHits(map[string]any{"stopped": int(machine.CPU.PC)})
	} else if err != nil {
		machine = nil
		return map[string]any{"error": err.Error()}
	}
	return withHits(map[string]any{"pc": int(machine.CPU.PC)})
}

// registers returns the CPU's state: {pc, hi, lo, cycles, regs}, regs a
// Uint32Array of the 32 general registers, or null when nothing is running.
func registers(js.Value, []js.Value) any {
	if machine == nil {
		return nil
	}
	c := machine.CPU
	regs := js.Global().Get("Uint32Array").New(len(c.R))
	for i, r := range c.R {
		regs.SetIndex(i, r)
	}
	return map[string]any{
		"pc": int(c.PC), "hi": int(c.HI), "lo": int(c.LO),
		"cycles": float64(c.TotalCycles),
		"regs":   regs,
	}
}

// withHits adds the breakpoints reached since the last report to a run's
// result, as hits, an object of counts by address, and starts counting
// afresh; and patched, when a hot reload patch was loaded meanwhile.
func withHits(result map[string]any) map[string]any {
	if patched {
		result["patched"] = true
		patched = false
	}
	if len(hits) == 0 {
		return result
	}
	counts := make(map[string]any, len(hits))
	for addr, n := range hits {
		counts[strconv.FormatUint(uint64(addr), 10)] = n
	}
	hits = nil
	result["hits"] = counts
	return result
}
