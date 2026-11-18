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
	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/psx/emu/emutest"
)

var (
	irqtrace      = flag.String("irqtrace", "", "the cue sheet of the irqsplit or irqstamp probe's disc built on the clean image, for TestIRQTrace")
	irqtraceImage = flag.String("irqtrace-image", "../../../../.build/cleanimage/image.elf", "the clean image's ELF, for its symbols")
	irqtraceBlank = flag.Int("irqtrace-blank", 8, "which 480-line blank of the probe's run to trace")
	irqtraceTail  = flag.Int("irqtrace-tail", 0, "log this many of the last steps before the poll, each with its cost")
)

// TestIRQTrace follows one vertical blank interrupt of spec 45's irqsplit probe
// (or spec 46's irqstamp) on the clean image, with the console's costs on, and
// logs what the emulator's model charges, by the probe's stretches (the kernel's
// stamps of priority 0 and 3, the callback, the poll), by region (the kernel, the
// clean library, the probe) and by kind of instruction, and the clean library's
// time by function. It is for spec 46: the console has the clean interrupt some
// 110 to 180 cycles longer than Sony's and the model does not, so the kinds of
// instruction the clean handler runs, and what the model charges each, are what
// a console measurement has to be set against. The clean side only: it reads the
// instructions it classifies, which CLEANROOM.md allows for the kernel's and the
// clean library's code and not for Sony's.
func TestIRQTrace(t *testing.T) {
	if *irqtrace == "" {
		t.Skip("-irqtrace names the clean disc")
	}
	data, err := os.ReadFile(*irqtraceImage)
	if err != nil {
		t.Fatal(err)
	}
	syms, err := emu.ELFSymbols(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(syms, func(a, b emu.Symbol) int { return int(a.Addr) - int(b.Addr) })
	// The image's end: its last symbol below the program's start, 80080000h
	// (the ELF also names the stack's top and other absolute addresses).
	var imageEnd uint32
	for _, s := range syms {
		if s.Addr < 0x80080000 {
			imageEnd = max(imageEnd, s.Addr+max(s.Size, 4))
		}
	}
	t.Logf("image ends at %08x", imageEnd)
	funcOf := func(pc uint32) string {
		i := sort.Search(len(syms), func(i int) bool { return syms[i].Addr > pc }) - 1
		if i < 0 {
			return "?"
		}
		return syms[i].Name
	}

	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(*irqtrace), emutest.AllowFaults())
	m.SetCPUCost(true)
	m.RunUntilTTY(fmt.Sprintf("BLANK 480i %d ", *irqtraceBlank), 60*emutest.Second)

	// Wait for the next interrupt to be taken.
	for m.CPU.Excode != cpu.ExcInt {
		m.Step()
	}

	type key struct {
		stretch int
		region  string
		kind    string
	}
	type sum struct {
		n      int
		cycles uint64
	}
	totals := map[key]*sum{}
	funcs := map[[2]any]*sum{} // {stretch, function}
	stretchNames := []string{"K0 to K3 (the kernel's)", "K3 to the callback", "the callback to the poll"}
	var stretchCycles [3]uint64
	var stretchSteps [3]int
	stretch := -1 // before the priority 0 stamp
	start := m.CPU.TotalCycles
	// The instruction cache's lines each region runs in each stretch, the
	// kernel's entry before the priority 0 stamp first.
	type lineKey struct {
		stretch int
		region  string
	}
	lines := map[lineKey]*[256]bool{}
	var tail []string
	regionOf := func(pc uint32) string {
		a := pc & 0x1fffffff
		switch {
		case a < 0x10000:
			return "kernel"
		case pc >= 0x80010000 && pc < imageEnd:
			return "library"
		case a >= 0x1fc00000:
			return "rom"
		}
		return "probe"
	}
	for steps := 0; steps < 200000; steps++ {
		pc := m.CPU.PC
		op := m.Read32(pc)
		region := regionOf(pc)
		kind, timer2 := classify(op, &m.CPU.R)
		if timer2 && region == "probe" {
			// One of the probe's stamps: priority 0, priority 3, the callback,
			// and then the poll's first read after the interrupt.
			stretch++
			if stretch == 3 {
				break
			}
		}
		lk := lineKey{stretch, region}
		if lines[lk] == nil {
			lines[lk] = new([256]bool)
		}
		lines[lk][pc>>4&255] = true
		before := m.CPU.TotalCycles
		m.Step()
		cost := m.CPU.TotalCycles - before
		if *irqtraceTail > 0 {
			tail = append(tail, fmt.Sprintf("%08x %-8s %-22s %2d  %s", pc, region, kind, cost, funcOf(pc)))
		}
		if stretch < 0 {
			continue
		}
		k := key{stretch, region, kind}
		s := totals[k]
		if s == nil {
			s = &sum{}
			totals[k] = s
		}
		s.n++
		s.cycles += cost
		stretchCycles[stretch] += cost
		stretchSteps[stretch]++
		if region == "library" {
			fk := [2]any{stretch, funcOf(pc)}
			f := funcs[fk]
			if f == nil {
				f = &sum{}
				funcs[fk] = f
			}
			f.n++
			f.cycles += cost
		}
	}
	t.Logf("from the exception to the poll: %d cycles", m.CPU.TotalCycles-start)

	var out strings.Builder
	for st := range 3 {
		fmt.Fprintf(&out, "\n%s: %d instructions, %d cycles\n", stretchNames[st], stretchSteps[st], stretchCycles[st])
		var keys []key
		for k := range totals {
			if k.stretch == st {
				keys = append(keys, k)
			}
		}
		slices.SortFunc(keys, func(a, b key) int {
			if a.region != b.region {
				return strings.Compare(a.region, b.region)
			}
			return int(totals[b].cycles) - int(totals[a].cycles)
		})
		for _, k := range keys {
			s := totals[k]
			fmt.Fprintf(&out, "  %-8s %-22s %6d instr %7d cycles %6.2f each\n", k.region, k.kind, s.n, s.cycles, float64(s.cycles)/float64(s.n))
		}
		var fkeys [][2]any
		for fk := range funcs {
			if fk[0] == st {
				fkeys = append(fkeys, fk)
			}
		}
		slices.SortFunc(fkeys, func(a, b [2]any) int { return int(funcs[b].cycles) - int(funcs[a].cycles) })
		for _, fk := range fkeys {
			s := funcs[fk]
			fmt.Fprintf(&out, "  library %-48s %6d instr %7d cycles\n", fk[1], s.n, s.cycles)
		}
	}
	if n := min(*irqtraceTail, len(tail)); n > 0 {
		out.WriteString("\nthe last steps before the poll (pc, region, kind, cycles, function):\n")
		for _, l := range tail[len(tail)-n:] {
			out.WriteString("  " + l + "\n")
		}
	}
	out.WriteString("\ncache lines run, by stretch and region:\n")
	for st := -1; st < 3; st++ {
		name := "the kernel's entry, to K0"
		if st >= 0 {
			name = stretchNames[st]
		}
		for _, region := range []string{"kernel", "library", "probe"} {
			if l := lines[lineKey{st, region}]; l != nil {
				fmt.Fprintf(&out, "  %-26s %-8s %s\n", name, region, lineRanges(l))
			}
		}
	}
	t.Log(out.String())
}

// lineRanges writes the set lines as hex ranges, with its size.
func lineRanges(l *[256]bool) string {
	var b strings.Builder
	n := 0
	for i := 0; i < 256; i++ {
		if !l[i] {
			continue
		}
		j := i
		for j+1 < 256 && l[j+1] {
			j++
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if j > i {
			fmt.Fprintf(&b, "%02X-%02X", i, j)
		} else {
			fmt.Fprintf(&b, "%02X", i)
		}
		n += j - i + 1
		i = j
	}
	return fmt.Sprintf("%d: %s", n, b.String())
}

// classify names the kind of instruction op is, with the registers as they
// are before it runs, and reports whether it loads timer 2's count.
func classify(op uint32, r *[32]uint32) (string, bool) {
	if op == 0 {
		return "nop", false
	}
	code := op >> 26
	switch code {
	case 0:
		switch op & 0x3f {
		case 0x08, 0x09:
			return "jr/jalr", false
		case 0x0c:
			return "syscall", false
		case 0x10, 0x12:
			return "mfhi/mflo", false
		case 0x18, 0x19:
			return "mult", false
		case 0x1a, 0x1b:
			return "div", false
		}
		return "alu", false
	case 1, 4, 5, 6, 7:
		return "branch", false
	case 2, 3:
		return "j/jal", false
	case 0x10:
		return "cop0", false
	case 0x12:
		return "cop2", false
	}
	if code >= 0x20 && code <= 0x26 || code == 0x32 || code >= 0x28 && code <= 0x2e || code == 0x3a {
		addr := r[op>>21&31] + uint32(int32(int16(op)))
		a := addr & 0x1fffffff
		var where string
		switch {
		case a < 0x800000:
			where = "ram"
		case a >= 0x1f800000 && a < 0x1f800400:
			where = "scratch"
		case a >= 0x1f801c00 && a < 0x1f802000:
			where = "spu"
		case a >= 0x1f801070 && a < 0x1f801080:
			where = "irq"
		case a >= 0x1f801080 && a < 0x1f801100:
			where = "dma"
		case a >= 0x1f801100 && a < 0x1f801130:
			where = "timer"
		case a >= 0x1f801040 && a < 0x1f801060:
			where = "sio"
		case a >= 0x1f801810 && a < 0x1f801818:
			where = "gpu"
		case a >= 0x1fc00000:
			where = "rom"
		default:
			where = "io"
		}
		width := "w"
		switch code & 3 {
		case 0:
			width = "b"
		case 1:
			width = "h"
		}
		if code == 0x22 || code == 0x26 || code == 0x2a || code == 0x2e {
			width = "wl/wr"
		}
		if code >= 0x28 {
			return "store " + where + " " + width, false
		}
		return "load " + where + " " + width, a == 0x1f801120 && code == 0x23
	}
	return "alu", false
}
