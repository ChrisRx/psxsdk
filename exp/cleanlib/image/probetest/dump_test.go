// Package probetest runs a probe disc and writes what it did, for probe.sh.
package probetest

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
)

// TestDump runs the disc PROBE_DISC for PROBE_FRAMES frames or until the TTY
// holds PROBE_END. It writes the TTY to PROBE_OUT.tty and every kernel call to
// PROBE_OUT.kernel, a line each: the table (a0, b0 or c0), the function, a0 to
// a3, ra, how many bytes the TTY held and the cycle count. PROBE_WATCH is a
// comma-separated list of addresses in hex: each time the PC reaches one, a line
// "w ADDR a0 a1 a2 a3 ra sp tty cyc" goes in the same file, and "r ADDR v0 v1 tty
// cyc" when it returns to that ra. PROBE_BUS, if set, records what the program does at
// the devices and the GTE, once the TTY holds its text, into PROBE_OUT.bus (busTrace).
func TestDump(t *testing.T) {
	disc, out := os.Getenv("PROBE_DISC"), os.Getenv("PROBE_OUT")
	if disc == "" || out == "" {
		t.Skip("PROBE_DISC and PROBE_OUT name the disc and the output")
	}
	frames, _ := strconv.Atoi(os.Getenv("PROBE_FRAMES"))
	end := os.Getenv("PROBE_END")
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.AllowFaults())
	watch := map[uint32]bool{}
	for _, a := range strings.Split(os.Getenv("PROBE_WATCH"), ",") {
		if v, err := strconv.ParseUint(a, 16, 32); err == nil {
			watch[uint32(v)] = true
		}
	}
	type call struct{ addr, ra, sp uint32 }
	var open []call
	var calls []byte
	last := m.Frames() + uint64(frames)
	step := 0
	busMark := os.Getenv("PROBE_BUS")
	bus := newBusTrace(m)
	for m.Frames() < last {
		if busMark != "" && !bus.on && step%256 == 0 && strings.Contains(m.TTY(), busMark) {
			bus.on = true
		}
		switch pc := m.CPU.PC; pc {
		case 0xa0, 0xb0, 0xc0:
			r := &m.CPU.R
			calls = fmt.Appendf(calls, "%02x %02x %08x %08x %08x %08x ra %08x tty %d cyc %d\n",
				pc, r[9], r[4], r[5], r[6], r[7], r[31], len(m.TTY()), m.CPU.TotalCycles)
		}
		if len(watch) > 0 {
			pc, r := m.CPU.PC, &m.CPU.R
			if n := len(open); n > 0 && pc == open[n-1].ra && r[29] == open[n-1].sp {
				calls = fmt.Appendf(calls, "r %08x %08x %08x tty %d cyc %d\n", open[n-1].addr, r[2], r[3], len(m.TTY()), m.CPU.TotalCycles)
				open = open[:n-1]
			}
			if watch[pc] {
				calls = fmt.Appendf(calls, "w %08x %08x %08x %08x %08x %08x %08x tty %d cyc %d\n", pc, r[4], r[5], r[6], r[7], r[31], r[29], len(m.TTY()), m.CPU.TotalCycles)
				open = append(open, call{pc, r[31], r[29]})
			}
		}
		// m.TTY copies the text, so look for the end only now and then.
		if step++; end != "" && step%4096 == 0 && strings.HasSuffix(strings.ReplaceAll(m.TTY(), "\r", ""), end) {
			break
		}
		m.Step()
	}
	os.WriteFile(out+".tty", []byte(m.TTY()), 0o666)
	os.WriteFile(out+".kernel", calls, 0o666)
	if busMark != "" {
		os.WriteFile(out+".bus", bus.lines, 0o666)
	}
}
