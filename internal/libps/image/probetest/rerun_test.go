package probetest

import (
	"debug/elf"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var rerunDisc = flag.String("rerun", "", "the cue sheet of the rerun probe's disc, as probe.sh builds it, for TestFifthRun")

// rerun runs the program on a probe's disc again and again on one boot, as
// hwrun does on a console with one upload of the library (spec 44): when
// the program returns to whatever started it, its memory goes back to what
// was loaded, the registers to what it was entered with, and it is entered
// again. It does not run a monitor between the runs, as the console does.
type rerun struct {
	// disc is the probe's cue sheet and elf the program's ELF, which says
	// where the program lies and where its own start-up is.
	disc, elf string

	// runs is how many runs, and frames how many frames each may take
	// before it counts as hung.
	runs, frames int

	// sr is ORed into the status register as a run is entered again: 401h
	// turns the interrupts on, as the monitor's start-up has by the time it
	// starts the next program.
	sr uint32

	// elfEntry enters each run after the first at the ELF's own entry, the
	// program's start-up, past the stub the disc's executable starts with,
	// which calls the library's init routine: as a console runs a program
	// uploaded through the monitor, with the library started once. noInit
	// enters the first run there too, so that the init routine never runs:
	// as on a console where hwrun has uploaded the clean library and
	// monitor, and nothing calls the init routine.
	elfEntry, noInit bool

	// calls, if not nil, logs each kernel call, as TestDump does, from the
	// first calls frames of each run, from its last calls frames and once
	// it has printed END (every call for 0).
	calls *int

	// kram, if set, keeps the RAM below the program, the kernel's and the
	// library's, after each run, in after.
	kram  bool
	after [][]byte

	// log is what was seen: the kernel's state at the first entry and after
	// each run (kernelState), and the calls.
	log strings.Builder
}

// run runs the program, and returns how many runs returned and the machine.
func (r *rerun) run(t *testing.T) (int, *emutest.Machine) {
	f, err := elf.Open(r.elf)
	if err != nil {
		t.Fatal(err)
	}
	elfEntry := uint32(f.Entry)
	lo, hi := ^uint32(0), uint32(0)
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		lo = min(lo, uint32(p.Vaddr))
		hi = max(hi, uint32(p.Vaddr+p.Memsz))
	}
	f.Close()
	// The start-up stub goes after the text, which can reach past the ELF's
	// last segment.
	hi += 64

	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(r.disc), emutest.AllowFaults())
	m.SetDrawTime(os.Getenv("PROBE_DRAWTIME") != "")

	// The first instruction run in the program's memory is its entry.
	limit := m.Frames() + 2000
	for m.CPU.PC < lo || m.CPU.PC >= hi {
		if m.Frames() > limit {
			t.Fatalf("the program at %08x to %08x was never entered", lo, hi)
		}
		m.Step()
	}
	entry := m.CPU.PC
	regs := m.CPU.R
	image := make([]byte, hi-lo)
	for i := range image {
		image[i] = m.Bus.Read8(lo + uint32(i))
	}
	fmt.Fprintf(&r.log, "entry %08x ra %08x sp %08x, program %08x to %08x\n", entry, regs[31], regs[29], lo, hi)
	kernelState(&r.log, m, "at the first entry")
	if r.noInit {
		m.CPU.PC, m.CPU.NextPC = elfEntry, elfEntry+4
	}

	for run := 1; run <= r.runs; run++ {
		fmt.Fprintf(&r.log, "run %d tty %d\n", run, len(m.TTY()))
		start := m.Frames()
		last := start + uint64(r.frames)
		// A run also ends after as many steps as its frames could hold:
		// a CPU caught in a loop of exceptions can stop the clock.
		steps := uint64(r.frames) * frameSteps
		for !(m.CPU.PC == regs[31] && m.CPU.R[29] == regs[29]) {
			if pc := m.CPU.PC; r.calls != nil && (pc == 0xa0 || pc == 0xb0 || pc == 0xc0) && r.logging(m, start, last) {
				g := &m.CPU.R
				fmt.Fprintf(&r.log, "%02x %02x %08x %08x %08x %08x ra %08x frame %d\n", pc, g[9], g[4], g[5], g[6], g[7], g[31], m.Frames()-start)
			}
			if steps--; m.Frames() > last || steps == 0 {
				kernelState(&r.log, m, fmt.Sprintf("run %d did not return, pc %08x", run, m.CPU.PC))
				t.Logf("run %d did not return in %d frames", run, r.frames)
				return run - 1, m
			}
			m.Step()
		}
		kernelState(&r.log, m, fmt.Sprintf("after run %d", run))
		if r.kram {
			kram := make([]byte, lo&0x1fffffff)
			for i := range kram {
				kram[i] = m.Bus.Read8(uint32(i))
			}
			r.after = append(r.after, kram)
		}
		t.Logf("run %d returned", run)
		if run == r.runs {
			break
		}
		m.LoadRAM(lo, image)
		m.CPU.R = regs
		m.CPU.PC, m.CPU.NextPC = entry, entry+4
		if r.elfEntry || r.noInit {
			m.CPU.PC, m.CPU.NextPC = elfEntry, elfEntry+4
		}
		m.CPU.COP0[12] |= r.sr
	}
	return r.runs, m
}

// frameSteps is more instructions than a frame holds: 33,868,800 cycles a
// second over 60 frames, at least one cycle an instruction.
const frameSteps = 33868800 / 60

// logging reports whether a kernel call made now is logged.
func (r *rerun) logging(m *emutest.Machine, start, last uint64) bool {
	n := uint64(*r.calls)
	return n == 0 || m.Frames() < start+n || m.Frames()+n > last ||
		strings.HasSuffix(strings.ReplaceAll(m.TTY(), "\r", ""), "END\n")
}

// TestRerun runs the program on the disc PROBE_DISC again and again (rerun).
// PROBE_ELF is the program's ELF, PROBE_RUNS how many runs (7) and
// PROBE_FRAMES the frames a run may take (3000). The kernel's state at the
// first entry and after each run goes into PROBE_OUT.kernel, and the TTY into
// PROBE_OUT.tty. PROBE_CALLS, if set, logs the kernel calls into
// PROBE_OUT.kernel too (rerun.calls). PROBE_SR is rerun.sr, in hexadecimal.
// PROBE_ELFENTRY, if set, is rerun.elfEntry, and with the value "all"
// rerun.noInit. PROBE_KRAM, if set, writes the RAM below the program after
// each run to PROBE_OUT.kramN.
func TestRerun(t *testing.T) {
	disc, out, path := os.Getenv("PROBE_DISC"), os.Getenv("PROBE_OUT"), os.Getenv("PROBE_ELF")
	if disc == "" || out == "" || path == "" {
		t.Skip("PROBE_DISC, PROBE_ELF and PROBE_OUT name the disc, the program and the output")
	}
	r := &rerun{disc: disc, elf: path, runs: 7, frames: 3000, kram: os.Getenv("PROBE_KRAM") != ""}
	if n, err := strconv.Atoi(os.Getenv("PROBE_RUNS")); err == nil {
		r.runs = n
	}
	if n, err := strconv.Atoi(os.Getenv("PROBE_FRAMES")); err == nil {
		r.frames = n
	}
	if s := os.Getenv("PROBE_CALLS"); s != "" {
		n, _ := strconv.Atoi(s)
		r.calls = &n
	}
	sr, _ := strconv.ParseUint(os.Getenv("PROBE_SR"), 16, 32)
	r.sr = uint32(sr)
	r.elfEntry = os.Getenv("PROBE_ELFENTRY") != ""
	r.noInit = os.Getenv("PROBE_ELFENTRY") == "all"
	_, m := r.run(t)
	os.WriteFile(out+".tty", []byte(m.TTY()), 0o666)
	os.WriteFile(out+".kernel", []byte(r.log.String()), 0o666)
	for i, kram := range r.after {
		os.WriteFile(fmt.Sprintf("%s.kram%d", out, i+1), kram, 0o666)
	}
}

// TestFifthRun runs spec 44's probe, spec/probes/rerun, seven times on one
// boot as a console runs it on one upload of the clean library, with the
// library's init routine never run, and checks that every run returns and
// that the RAM below 100h, the exception vectors and the kernel's call
// stubs, is the same after each. Until spec 44, FntLoad took its palette
// from the image's heap, whose start nothing sets on a console, so each
// run's FntLoad wrote 32 bytes further up from address 0, and the fifth
// wrote over the exception vector at 80h and hung. Build the disc with
//
//	SIDES=clean internal/libps/image/probe.sh rerun internal/libps/spec/probes/rerun
//
// and pass -rerun=$PWD/.build/rerun-cimg-clean/rerun-cimg-clean.cue. It takes
// about two minutes.
func TestFifthRun(t *testing.T) {
	if *rerunDisc == "" {
		t.Skip("-rerun names the probe's disc")
	}
	r := &rerun{
		disc:   *rerunDisc,
		elf:    filepath.Join(filepath.Dir(*rerunDisc), "prog.elf"),
		runs:   7,
		frames: 1500,
		sr:     0x401,
		noInit: true,
		kram:   true,
	}
	runs, m := r.run(t)
	tty := strings.ReplaceAll(m.TTY(), "\r", "")
	assert.Equal(t, 7, runs, "the runs that returned")
	assert.Equal(t, 7, strings.Count(tty, "\nEND\n"), "the runs that printed END")
	for i, kram := range r.after[1:] {
		assert.Equal(t, r.after[0][:0x100], kram[:0x100], fmt.Sprintf("the RAM below 100h after run %d against run 1", i+2))
	}
}

// kernelState writes what the kernel's tables hold: each event in use (its
// number, class, status, spec, mode and handler), each priority's chain of
// interrupt handlers (each node's address and its two functions), some of
// the devices' registers, and the words at 100h to 15Fh, where the kernel
// keeps its tables' addresses.
func kernelState(w *strings.Builder, m *emutest.Machine, what string) {
	fmt.Fprintf(w, "-- %s\n", what)
	evs, size := m.Read32(0x120), m.Read32(0x124)
	used := 0
	for i := uint32(0); i < size/0x1c; i++ {
		e := evs + i*0x1c
		if m.Read32(e+4) == 0 {
			continue
		}
		used++
		fmt.Fprintf(w, "event %2d %08x class %08x status %04x spec %04x mode %04x handler %08x\n",
			i, 0xf1000000|i, m.Read32(e), m.Read32(e+4), m.Read32(e+8), m.Read32(e+12), m.Read32(e+16))
	}
	fmt.Fprintf(w, "events %d of %d\n", used, size/0x1c)
	q := m.Read32(0x100)
	for p := uint32(0); p < 4; p++ {
		fmt.Fprintf(w, "chain %d:", p)
		node := m.Read32(q + p*8)
		for n := 0; node != 0 && n < 16; n++ {
			fmt.Fprintf(w, " %08x(%08x,%08x)", node, m.Read32(node+4), m.Read32(node+8))
			node = m.Read32(node)
		}
		if node != 0 {
			fmt.Fprintf(w, " ...")
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "I_STAT %04x I_MASK %04x DPCR %08x DICR %08x GPUSTAT %08x D2 %08x D4 %08x SPUCNT %04x SPUSTAT %04x SIO0 stat %08x mode %04x ctrl %04x T0 target %04x T1 target %04x T2 target %04x\n",
		m.Read32(0x1f801070), m.Read32(0x1f801074), m.Read32(0x1f8010f0), m.Read32(0x1f8010f4), m.Read32(0x1f801814),
		m.Read32(0x1f8010a8), m.Read32(0x1f8010c8), m.Bus.Read16(0x1f801daa), m.Bus.Read16(0x1f801dae),
		m.Read32(0x1f801044), m.Bus.Read16(0x1f801048), m.Bus.Read16(0x1f80104a),
		m.Bus.Read16(0x1f801108), m.Bus.Read16(0x1f801118), m.Bus.Read16(0x1f801128))
	for a := uint32(0x100); a < 0x160; a += 16 {
		fmt.Fprintf(w, "%03x: %08x %08x %08x %08x\n", a, m.Read32(a), m.Read32(a+4), m.Read32(a+8), m.Read32(a+12))
	}
}

// TestImageNoHeap checks that the image make.sh last built has no heap
// allocation in it: no runtime.alloc. On a console nothing calls the image's
// init routine, the only place the runtime's heap start is set, so an
// allocation takes memory from address 0 up, over the exception vectors and
// the kernel's call stubs (spec 44). It skips when there is no image.
func TestImageNoHeap(t *testing.T) {
	f, err := elf.Open("../../../../.build/cleanimage/image.elf")
	if err != nil {
		t.Skip("no image: run internal/libps/image/make.sh")
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range syms {
		if s.Name == "runtime.alloc" {
			t.Errorf("the image allocates: runtime.alloc at %08x", s.Value)
		}
	}
}
