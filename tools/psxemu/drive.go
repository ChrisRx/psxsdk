package main

// Driving a program: running it frame by frame under a script of button
// presses, waiting for words of its memory to reach values, watching words
// change, poking words at chosen frames, saving the display as it goes,
// and profiling where its cycles went. These are what comparing a port
// with the program it was ported from needs: both run the same script, and
// their watched words are compared frame by frame.

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.chrisrx.dev/psx/emu"
)

var buttons = map[string]uint16{
	"u": emu.ButtonUp, "d": emu.ButtonDown, "l": emu.ButtonLeft, "r": emu.ButtonRight,
	"x": emu.ButtonCross, "o": emu.ButtonCircle, "q": emu.ButtonSquare, "t": emu.ButtonTriangle,
	"L1": emu.ButtonL1, "R1": emu.ButtonR1, "L2": emu.ButtonL2, "R2": emu.ButtonR2,
	"st": emu.ButtonStart, "se": emu.ButtonSelect,
}

// driving reports whether any of the driving options was given.
func driving() bool {
	return opts.Script != "" || len(opts.Untils) > 0 || len(opts.Watches) > 0 || len(opts.Pokes) > 0 || opts.Profile != "" || opts.Every > 0 || opts.Polygons
}

// An address names a word of memory: directly, or through the pointer held
// at addr with off added, written *addr+off.
type address struct {
	addr, off uint32
	indirect  bool
}

func parseAddress(s string) (address, error) {
	var a address
	if strings.HasPrefix(s, "*") {
		a.indirect = true
		s = s[1:]
		if base, off, ok := strings.Cut(s, "+"); ok {
			s = base
			o, err := strconv.ParseUint(off, 16, 32)
			if err != nil {
				return a, fmt.Errorf("offset %q: %w", off, err)
			}
			a.off = uint32(o)
		}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return a, fmt.Errorf("address %q: %w", s, err)
	}
	a.addr = uint32(v)
	return a, nil
}

func (a address) resolve(m *emu.Machine) uint32 {
	if a.indirect {
		return m.Bus.Read32(a.addr) + a.off
	}
	return a.addr
}

type watch struct {
	address
	words []uint32
}

type poke struct {
	address
	frame int
	or    bool
	value uint32
}

type symbol struct {
	addr uint32
	name string
}

// drive runs the program under the driving options, once the BIOS has
// started it, and reports what they asked for. frame counts vertical
// blanks from then.
func drive(m *emu.Machine, tty *strings.Builder) error {
	var watches []watch
	for _, w := range opts.Watches {
		a, n, _ := strings.Cut(w, ":")
		addr, err := parseAddress(a)
		if err != nil {
			return fmt.Errorf("--watch %q: %w", w, err)
		}
		words, err := strconv.ParseUint(n, 16, 32)
		if err != nil || words == 0 {
			return fmt.Errorf("--watch %q: want addr:words", w)
		}
		watches = append(watches, watch{addr, make([]uint32, words)})
	}
	var pokes []poke
	for _, p := range opts.Pokes {
		parts := strings.SplitN(p, ":", 3)
		if len(parts) != 3 {
			return fmt.Errorf("--poke %q: want frame:addr:value", p)
		}
		var pk poke
		var err error
		if pk.frame, err = strconv.Atoi(parts[0]); err != nil {
			return fmt.Errorf("--poke %q: %w", p, err)
		}
		if pk.address, err = parseAddress(parts[1]); err != nil {
			return fmt.Errorf("--poke %q: %w", p, err)
		}
		v := parts[2]
		if strings.HasPrefix(v, "|") {
			pk.or = true
			v = v[1:]
		}
		val, err := strconv.ParseUint(v, 16, 32)
		if err != nil {
			return fmt.Errorf("--poke %q: %w", p, err)
		}
		pk.value = uint32(val)
		pokes = append(pokes, pk)
	}
	var syms []symbol
	if opts.Profile != "" {
		var err error
		if syms, err = readSymbols(opts.Profile); err != nil {
			return err
		}
	}
	winA, winB := 0, 1<<30
	if opts.ProfileWindow != "" {
		a, b, _ := strings.Cut(opts.ProfileWindow, ":")
		winA, _ = strconv.Atoi(a)
		winB, _ = strconv.Atoi(b)
	}
	type step struct {
		buttons uint16
		frames  int
	}
	var steps []step
	if opts.Script != "" {
		for _, s := range strings.Split(opts.Script, ";") {
			keys, n, ok := strings.Cut(s, ":")
			if !ok {
				n, keys = keys, ""
			}
			frames, err := strconv.Atoi(n)
			if err != nil {
				return fmt.Errorf("--script step %q: %w", s, err)
			}
			var b uint16
			for _, k := range strings.Split(keys, "+") {
				if k == "" {
					continue
				}
				v, ok := buttons[k]
				if !ok {
					return fmt.Errorf("--script step %q: no button %q", s, k)
				}
				b |= v
			}
			steps = append(steps, step{b, frames})
		}
	}

	// With a disc, the frames are counted from the BIOS starting the
	// program, so that a script means the same whichever BIOS boots it.
	if opts.Disc != "" {
		for i := 0; !strings.Contains(tty.String(), "Execute !"); i++ {
			if i > 20000 {
				return fmt.Errorf("the BIOS did not start the program")
			}
			if _, err := m.RunFrame(4_000_000); err != nil {
				return err
			}
		}
	}
	frame := 0
	for _, until := range opts.Untils {
		a, v, _ := strings.Cut(until, ":")
		addr, err := parseAddress(a)
		if err != nil {
			return fmt.Errorf("--until %q: %w", until, err)
		}
		val, err := strconv.ParseUint(v, 16, 32)
		if err != nil {
			return fmt.Errorf("--until %q: want addr:value", until)
		}
		i := 0
		for ; m.Bus.Read32(addr.resolve(m)) != uint32(val); i++ {
			if i >= opts.UntilBudget {
				return fmt.Errorf("--until %s: not reached in %d frames", until, i)
			}
			if _, err := m.RunFrame(4_000_000); err != nil {
				return err
			}
			frame++
		}
		fmt.Printf("reached %s after %d frames (%d since the start)\n", until, i, frame)
	}

	frame0 := frame
	polygons0 := m.Polygons()
	kinds0 := m.PolygonKinds()
	samples := map[uint32]int{}
	save := func(name string) error {
		if err := os.MkdirAll(opts.Frames, 0o755); err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(opts.Frames, name))
		if err != nil {
			return err
		}
		defer f.Close()
		return png.Encode(f, m.Display())
	}
	runFrames := func(n int) error {
		for i := 0; i < n; i++ {
			if syms != nil {
				// Sampling: the cycles each short run took go to the
				// address it stopped at.
				f := m.Frames()
				for m.Frames() == f {
					before := m.CPU.TotalCycles
					if _, err := m.Run(200); err != nil {
						return fmt.Errorf("frame %d: %w", frame, err)
					}
					if frame-frame0 >= winA && frame-frame0 <= winB {
						samples[m.CPU.PC] += int(m.CPU.TotalCycles - before)
					}
				}
			} else if _, err := m.RunFrame(4_000_000); err != nil {
				return fmt.Errorf("frame %d: %w", frame, err)
			}
			frame++
			for _, pk := range pokes {
				if pk.frame != frame-frame0 {
					continue
				}
				addr := pk.resolve(m)
				v := pk.value
				if pk.or {
					v |= m.Bus.Read32(addr)
				}
				m.Bus.Write32(addr, v)
				fmt.Printf("poked %08x = %08x at +%d\n", addr, v, frame-frame0)
			}
			if opts.Every > 0 && (frame-frame0)%opts.Every == 0 {
				if err := save(fmt.Sprintf("s%05d.png", frame-frame0)); err != nil {
					return err
				}
			}
			for k := range watches {
				w := &watches[k]
				base := w.resolve(m)
				changed := false
				for j := range w.words {
					v := m.Bus.Read32(base + 4*uint32(j))
					if v != w.words[j] {
						w.words[j] = v
						changed = true
					}
				}
				if changed {
					fmt.Printf("+%d w%d:", frame-frame0, k)
					for _, v := range w.words {
						fmt.Printf(" %08x", v)
					}
					fmt.Println()
				}
			}
		}
		return nil
	}
	for _, s := range steps {
		m.Pad(1).Press(s.buttons)
		if err := runFrames(s.frames); err != nil {
			return err
		}
		m.Pad(1).Release(s.buttons)
	}
	fmt.Printf("frames %d pc %08x ra %08x sp %08x\n", frame, m.CPU.PC, m.CPU.R[31], m.CPU.R[29])
	if opts.Polygons {
		n := uint64(max(frame-frame0, 1))
		fmt.Printf("polygons per vertical blank over the script: %d\n", (m.Polygons()-polygons0)/n)
		kinds := m.PolygonKinds()
		for i := range kinds {
			fmt.Printf("  shaded %d quad %d textured %d: %d\n", i>>2&1, i>>1&1, i&1, (kinds[i]-kinds0[i])/n)
		}
	}
	if syms != nil {
		report(syms, samples, frame-frame0)
	}
	return nil
}

// readSymbols reads a symbol file: lines of a hexadecimal address, then
// optionally a type letter, then a name, which is what go tool nm prints
// and what a hand-made list of an original's names is.
func readSymbols(name string) ([]symbol, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var syms []symbol
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		a, err := strconv.ParseUint(strings.TrimPrefix(f[0], "0x"), 16, 32)
		if err != nil {
			continue
		}
		syms = append(syms, symbol{uint32(a), f[len(f)-1]})
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].addr < syms[j].addr })
	return syms, nil
}

// report prints where the cycles went, by symbol and then by the hottest
// 64-byte stretches of code, for finding the loop within a function.
func report(syms []symbol, samples map[uint32]int, frames int) {
	byName := map[string]int{}
	total := 0
	for pc, n := range samples {
		pc &= 0x1fffffff
		i := sort.Search(len(syms), func(i int) bool { return syms[i].addr&0x1fffffff > pc }) - 1
		name := fmt.Sprintf("%08x", pc)
		switch {
		case i >= 0 && pc-syms[i].addr&0x1fffffff < 0x10000:
			name = syms[i].name
		case pc >= 0x1fc00000:
			name = "bios"
		}
		byName[name] += n
		total += n
	}
	type row struct {
		name string
		n    int
	}
	var rows []row
	for k, v := range byName {
		rows = append(rows, row{k, v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })
	fmt.Printf("%d cycles, %d per vertical blank\n", total, total/max(frames, 1))
	for i, r := range rows {
		if i >= 40 || r.n*1000 < total {
			break
		}
		fmt.Printf("%5.1f%% %s\n", float64(r.n)*100/float64(total), r.name)
	}
	buckets := map[uint32]int{}
	for pc, n := range samples {
		buckets[pc&^63] += n
	}
	var hot []row
	for k, v := range buckets {
		hot = append(hot, row{fmt.Sprintf("%08x", k), v})
	}
	sort.Slice(hot, func(i, j int) bool { return hot[i].n > hot[j].n })
	for i, r := range hot {
		if i >= 16 {
			break
		}
		fmt.Printf("%5.1f%% at %s\n", float64(r.n)*100/float64(total), r.name)
	}
}
