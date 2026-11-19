// Package regwatch is a harness for spec 17's probes of entries with no public
// calling convention: it runs a disc and logs every register at each arrival at
// the addresses given and at the return to the arrival's ra.
package regwatch

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
)

// TestRegs runs the disc PROBE_DISC for PROBE_FRAMES frames or until the TTY ends
// with a line "END". It writes the TTY to PROBE_OUT.tty, and to PROBE_OUT.regs a
// line per arrival ("w ADDR r1 ... r31 cyc") and return ("r ADDR r1 ... r31 cyc")
// at the addresses in PROBE_WATCH. The arrivals' registers are logged after
// PROBE_POKE, a comma-separated list of REG=HEX assignments made at each arrival.
// PROBE_KERNEL=1 logs kernel calls to the same file, "k TABLE FN a0 a1 a2 a3 ra cyc".
// The TTY is looked at only every 64K instructions (its getter copies it), so a
// long run stays fast, unlike probetest's TestDump.
func TestRegs(t *testing.T) {
	disc, out := os.Getenv("PROBE_DISC"), os.Getenv("PROBE_OUT")
	if disc == "" || out == "" {
		t.Skip("PROBE_DISC and PROBE_OUT name the disc and the output")
	}
	frames, _ := strconv.Atoi(os.Getenv("PROBE_FRAMES"))
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.AllowFaults())
	watch := map[uint32]bool{}
	for _, a := range strings.Split(os.Getenv("PROBE_WATCH"), ",") {
		if v, err := strconv.ParseUint(a, 16, 32); err == nil {
			watch[uint32(v)] = true
		}
	}
	type poke struct {
		reg int
		val uint32
	}
	var pokes []poke
	for _, s := range strings.Split(os.Getenv("PROBE_POKE"), ",") {
		if r, v, ok := strings.Cut(s, "="); ok {
			n, _ := strconv.Atoi(r)
			u, _ := strconv.ParseUint(v, 16, 32)
			pokes = append(pokes, poke{n, uint32(u)})
		}
	}
	type call struct{ addr, ra, sp uint32 }
	var open []call
	var log []byte
	regs := func(kind string, addr uint32) {
		log = fmt.Appendf(log, "%s %08x", kind, addr)
		for i := 1; i < 32; i++ {
			log = fmt.Appendf(log, " %08x", m.CPU.R[i])
		}
		log = fmt.Appendf(log, " %d\n", m.CPU.TotalCycles)
	}
	kernel := os.Getenv("PROBE_KERNEL") != ""
	last := m.Frames() + uint64(frames)
	for n := 0; m.Frames() < last; n++ {
		pc := m.CPU.PC
		if kernel && (pc == 0xa0 || pc == 0xb0 || pc == 0xc0) {
			r := &m.CPU.R
			log = fmt.Appendf(log, "k %02x %02x %08x %08x %08x %08x %08x %d\n", pc, r[9], r[4], r[5], r[6], r[7], r[31], m.CPU.TotalCycles)
		}
		if n := len(open); n > 0 && pc == open[n-1].ra && m.CPU.R[29] == open[n-1].sp {
			regs("r", open[n-1].addr)
			open = open[:n-1]
		}
		if watch[pc] {
			for _, p := range pokes {
				m.CPU.R[p.reg] = p.val
			}
			regs("w", pc)
			open = append(open, call{pc, m.CPU.R[31], m.CPU.R[29]})
		}
		if n&0xffff == 0 && strings.Contains(m.TTY(), "\nEND\r") {
			break
		}
		// A run that is cut short still leaves what it printed.
		if n&0xffffff == 0xffffff {
			os.WriteFile(out+".tty", []byte(strings.ReplaceAll(m.TTY(), "\r", "")), 0o666)
		}
		m.Step()
	}
	tty := m.TTY()
	if i := strings.Index(tty, "\nEND\r"); i >= 0 {
		tty = tty[:i+1] + "END\n"
	}
	os.WriteFile(out+".tty", []byte(strings.ReplaceAll(tty, "\r", "")), 0o666)
	os.WriteFile(out+".regs", log, 0o666)
}
