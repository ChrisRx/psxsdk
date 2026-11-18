package probetest

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

var (
	calltrace      = flag.String("calltrace", "", "the cue sheet of a callcost disc, for TestCallTrace")
	calltraceSide  = flag.String("calltrace-side", "clean", "the disc's library: clean (each instruction logged) or sony (the bus and the GTE only)")
	calltraceCost  = flag.Bool("calltrace-cost", true, "charge the console's costs (emu's SetCPUCost)")
	calltraceCalls = flag.String("calltrace-calls", "GsSortSprite,GsGetLws,GsSetLightMatrix,GsSortClear", "the entry points to follow, by name")
)

// TestCallTrace follows each call the callcost probe makes from its own code into
// the entry points named, with the console's costs on (spec 48), and logs each
// call's cycles from the entry to the return, the devices and the GTE it reaches
// with their cycles from the entry, and on the clean side each instruction with
// its cost and function. On Sony's side it reads no instruction and logs no PC:
// only the cycles, the bus and the GTE commands, from outside.
func TestCallTrace(t *testing.T) {
	if *calltrace == "" {
		t.Skip("-calltrace names the disc")
	}
	clean := *calltraceSide == "clean"
	want := map[string]bool{}
	for _, n := range strings.Split(*calltraceCalls, ",") {
		want[n] = true
	}
	entries := map[uint32]string{}
	for a, n := range entryNames() {
		if want[n] {
			entries[a] = n
		}
	}
	if len(entries) == 0 {
		t.Fatal("no entry points: run make.sh")
	}
	var syms []emu.Symbol
	if clean {
		data, err := os.ReadFile("../../../../.build/cleanimage/image.elf")
		if err != nil {
			t.Fatal(err)
		}
		if syms, err = emu.ELFSymbols(data, nil); err != nil {
			t.Fatal(err)
		}
		slices.SortFunc(syms, func(a, b emu.Symbol) int { return int(a.Addr) - int(b.Addr) })
	}
	funcOf := func(pc uint32) string {
		i := sort.Search(len(syms), func(i int) bool { return syms[i].Addr > pc }) - 1
		if i < 0 {
			return "?"
		}
		return syms[i].Name
	}

	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*calltrace), emutest.AllowFaults())
	m.SetCPUCost(*calltraceCost)
	m.RunUntilTTY("COST GsSortObject4 gt4", 120*emutest.Second)

	const programStart = 0x80090000
	var (
		in     bool
		name   string
		ra, sp uint32
		start  uint64
		lines  []string
		out    strings.Builder
		n      = map[string]int{}
		// The lines each call ran, and what ran in them since the call
		// before, on the clean side.
		callLines = map[string]*[256]bool{}
		evict     = map[string]map[string]bool{}
	)
	m.OnBus(func(addr uint32, size int, v uint32, write bool) {
		if !in || addr&0x1fffffff < 0x1f801000 || addr&0x1fffffff >= 0x1f803000 {
			return
		}
		dir := "r"
		if write {
			dir = "w"
		}
		lines = append(lines, fmt.Sprintf("    io %s %08x %d %08x at %d", dir, addr&0x1fffffff, size, v, m.CPU.TotalCycles-start))
	})
	m.OnGTE(func(op uint32, cycle uint64) {
		if in {
			lines = append(lines, fmt.Sprintf("    gte %08x at %d", op, cycle-start))
		}
	})
	for steps := 0; steps < 20_000_000; steps++ {
		pc := m.CPU.PC
		if !in {
			if e, ok := entries[pc]; ok && m.CPU.R[31] >= programStart && m.CPU.R[31] < 0x80200000 {
				in, name, ra, sp, start, lines = true, e, m.CPU.R[31], m.CPU.R[29], m.CPU.TotalCycles, nil
			}
		} else if pc == ra && m.CPU.R[29] == sp {
			in = false
			n[name]++
			fmt.Fprintf(&out, "%s call %d: %d cycles\n", name, n[name], m.CPU.TotalCycles-start)
			if clean {
				var ev []string
				for f := range evict[name] {
					ev = append(ev, f)
				}
				slices.Sort(ev)
				fmt.Fprintf(&out, "  run before it in its lines: %s\n", strings.Join(ev, " "))
				evict[name] = map[string]bool{}
			}
			for _, l := range lines {
				out.WriteString(l + "\n")
			}
		}
		if clean && !in {
			for c, l := range callLines {
				if l[pc>>4&255] && evict[c] != nil {
					evict[c][fmt.Sprintf("%s@%08x", funcOf(pc), pc&^15)] = true
				}
			}
		}
		if in && clean {
			if callLines[name] == nil {
				callLines[name] = new([256]bool)
			}
			callLines[name][pc>>4&255] = true
			op := m.Read32(pc)
			kind, _ := classify(op, &m.CPU.R)
			before := m.CPU.TotalCycles
			m.Step()
			lines = append(lines, fmt.Sprintf("  %08x %08x %-18s %3d at %4d  %s", pc, op, kind, m.CPU.TotalCycles-before, before-start, funcOf(pc)))
			continue
		}
		if !in && steps%4096 == 0 && strings.Contains(m.TTY(), "END") {
			break
		}
		m.Step()
	}
	if p := os.Getenv("CALLTRACE_OUT"); p != "" {
		os.WriteFile(p, []byte(out.String()), 0o644)
		return
	}
	t.Log(out.String())
}
