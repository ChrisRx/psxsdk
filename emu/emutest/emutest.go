// Package emutest runs programs in the emulator from Go tests.
//
// A test boots a machine, drives it with run budgets and controller input,
// and makes assertions about what it printed or left in memory:
//
//	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("check.cue"))
//	m.RunUntilTTY("Execute !", 10*emutest.Second)
//	m.RunFrames(60)
//	m.Hold(1, emu.ButtonSelect, 10)
//	m.RunUntilTTY("End of Main", 5*emutest.Second)
//
// Time is the emulated machine's, counted in CPU cycles and frames, so a run
// takes the same course every time however loaded the host is. Every way of
// running has a budget: a program that never gets where the test expects
// fails the test rather than hanging it.
//
// A program that executes an illegal instruction or makes a misaligned
// access fails the test there, with a report of the registers, the
// instructions leading up to it and what the program printed.
package emutest

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mewmew/mips"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/cpu"
)

// Cycles is a span of emulated time, in CPU cycles.
type Cycles uint64

// Units of emulated time.
const (
	Second      Cycles = 33_868_800
	Millisecond        = Second / 1000
)

// Machine is a console being run by a test.
type Machine struct {
	*emu.Machine

	tb  testing.TB
	tty bytes.Buffer

	// recent is a ring of the last instructions executed, for the report.
	recent [recentSteps]step
	next   int
	steps  uint64

	allowFaults bool
	symbols     map[string]uint32
}

type step struct {
	pc, insn uint32
}

const recentSteps = 32

// Option configures Boot.
type Option func(*config)

type config struct {
	bios        string
	disc        string
	elf         string
	allowFaults bool
}

// Symbols reads the symbol table of the ELF the program was linked as, for
// Sym and RunUntilFunc.
func Symbols(path string) Option { return func(c *config) { c.elf = path } }

// BIOS boots the BIOS image at path.
func BIOS(path string) Option { return func(c *config) { c.bios = path } }

// Disc puts the disc whose cue sheet is at path in the drive.
func Disc(path string) Option { return func(c *config) { c.disc = path } }

// AllowFaults lets a program take illegal instruction and address error
// exceptions without failing the test, for a test of how they are handled.
func AllowFaults() Option { return func(c *config) { c.allowFaults = true } }

// BIOSFromEnv returns the BIOS image named by PSX_BIOS, and skips the test
// if there is none. Sony's BIOS cannot be distributed with the tests, so a
// test that needs one has to be given it.
func BIOSFromEnv(tb testing.TB) string {
	tb.Helper()
	path := os.Getenv("PSX_BIOS")
	if path == "" {
		tb.Skip("PSX_BIOS is not set")
	}
	return path
}

// Boot starts a machine, about to run the BIOS from reset.
func Boot(tb testing.TB, opts ...Option) *Machine {
	tb.Helper()
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.bios == "" {
		tb.Fatal("emutest.Boot: no BIOS given")
	}
	bios, err := os.ReadFile(cfg.bios)
	if err != nil {
		tb.Fatal(err)
	}
	em, err := emu.New(bios)
	if err != nil {
		tb.Fatal(err)
	}

	m := &Machine{Machine: em, tb: tb, allowFaults: cfg.allowFaults}
	em.TTY = &m.tty
	if cfg.disc != "" {
		d, err := emu.OpenDisc(cfg.disc)
		if err != nil {
			tb.Fatal(err)
		}
		tb.Cleanup(func() { d.Close() })
		em.InsertDisc(d)
	}
	if cfg.elf != "" {
		if m.symbols, err = readSymbols(cfg.elf); err != nil {
			tb.Fatal(err)
		}
	}
	return m
}

func readSymbols(path string) (map[string]uint32, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	m := make(map[string]uint32, len(syms))
	for _, s := range syms {
		if s.Name != "" {
			m[s.Name] = uint32(s.Value)
		}
	}
	return m, nil
}

// Sym returns a symbol's address, failing the test if there is no such
// symbol. It needs the Symbols option.
func (m *Machine) Sym(name string) uint32 {
	m.tb.Helper()
	if m.symbols == nil {
		m.tb.Fatal("emutest: Sym needs the Symbols option")
	}
	addr, ok := m.symbols[name]
	if !ok {
		m.tb.Fatalf("emutest: no symbol %q", name)
	}
	return addr
}

// RunUntilFunc runs until the program is about to execute the function
// name. Being inlined leaves a function without a symbol of its own, so the
// function has to be one the compiler kept.
func (m *Machine) RunUntilFunc(name string, budget Cycles) {
	m.tb.Helper()
	addr := m.Sym(name)
	m.RunUntil(func() bool { return m.CPU.PC == addr }, budget, "reaching "+name)
}

// RunUntilCount runs until the word at the symbol name, a counter the program
// keeps, reaches n. A program's own frame counter puts a test at the same
// point in its life whichever BIOS started it, where a count of frames since
// the BIOS did would not.
func (m *Machine) RunUntilCount(name string, n uint32, budget Cycles) {
	m.tb.Helper()
	addr := m.Sym(name)
	m.RunUntil(func() bool { return m.Read32(addr) >= n }, budget, fmt.Sprintf("%s reaching %d", name, n))
}

// TTY returns everything printed so far.
func (m *Machine) TTY() string { return m.tty.String() }

// Cycles returns how long the machine has been running.
func (m *Machine) Cycles() Cycles { return Cycles(m.CPU.TotalCycles) }

// Step runs one instruction, failing the test if it faults.
func (m *Machine) Step() {
	m.tb.Helper()
	if fault := m.step(); fault != "" {
		m.fail("%s", fault)
	}
}

// step runs one instruction and describes the fault if it took one. It
// leaves failing the test to its caller, so that the run loops can call
// testing.TB.Helper once rather than once an instruction, which costs a
// walk of the stack every time.
func (m *Machine) step() string {
	if err := m.Machine.Step(); err != nil {
		return err.Error()
	}
	c := m.CPU
	m.recent[m.next] = step{c.SavedPC, c.Opcode}
	m.next = (m.next + 1) % recentSteps
	m.steps++

	if m.allowFaults {
		return ""
	}
	switch c.Excode {
	case cpu.ExcRI:
		return fmt.Sprintf("illegal instruction %08x at %08x", c.Opcode, c.SavedPC)
	case cpu.ExcAdEL:
		return fmt.Sprintf("misaligned load at %08x", c.SavedPC)
	case cpu.ExcAdES:
		return fmt.Sprintf("misaligned store at %08x", c.SavedPC)
	}
	return ""
}

// RunUntil runs until done reports true, failing the test if that has not
// happened within budget. what describes the condition, for the failure.
func (m *Machine) RunUntil(done func() bool, budget Cycles, what string) {
	m.tb.Helper()
	end := m.Cycles() + budget
	for !done() {
		if m.Cycles() >= end {
			m.fail("%s did not happen within %s", what, budget)
		}
		if fault := m.step(); fault != "" {
			m.fail("%s", fault)
		}
	}
}

// RunUntilTTY runs until the program has printed s, at any point since the
// machine started.
func (m *Machine) RunUntilTTY(s string, budget Cycles) {
	m.tb.Helper()
	want := []byte(s)
	// Only text printed since the last look needs searching, along with
	// enough before it to find s straddling the two.
	seen := -1
	m.RunUntil(func() bool {
		b := m.tty.Bytes()
		if len(b) == seen {
			return false
		}
		from := max(seen-len(want)+1, 0)
		seen = len(b)
		return bytes.Contains(b[from:], want)
	}, budget, fmt.Sprintf("printing %q", s))
}

// Run runs for d of emulated time.
func (m *Machine) Run(d Cycles) {
	m.tb.Helper()
	end := m.Cycles() + d
	for m.Cycles() < end {
		if fault := m.step(); fault != "" {
			m.fail("%s", fault)
		}
	}
}

// RunFrames runs until n more frames have been displayed. A program that
// has stopped the display from being refreshed, which a frame's worth of
// time would always see, fails the test.
func (m *Machine) RunFrames(n int) {
	m.tb.Helper()
	target := m.Frames() + uint64(n)
	// A frame is a little over 1/60s; twice that per frame is ample.
	m.RunUntil(func() bool { return m.Frames() >= target },
		Cycles(n)*Second/30, fmt.Sprintf("%d frames", n))
}

// Hold holds buttons on the controller in port for frames frames, then
// releases them and runs another frames frames, long enough for a program
// polling once a frame to see both.
func (m *Machine) Hold(port int, buttons uint16, frames int) {
	m.tb.Helper()
	pad := m.Pad(port)
	if pad == nil {
		m.tb.Fatalf("no controller in port %d", port)
	}
	pad.Press(buttons)
	m.RunFrames(frames)
	pad.Release(buttons)
	m.RunFrames(frames)
}

// Read32 reads a word of memory as the CPU would.
func (m *Machine) Read32(addr uint32) uint32 { return m.Bus.Read32(addr) }

// fail ends the test with a report of where the machine had got to.
func (m *Machine) fail(format string, args ...any) {
	m.tb.Helper()
	m.tb.Fatalf("%s\n\n%s", fmt.Sprintf(format, args...), m.report())
}

func (m *Machine) report() string {
	var sb strings.Builder
	c := m.CPU
	fmt.Fprintf(&sb, "after %d instructions, %s of emulated time, %d frames\n\n",
		m.steps, m.Cycles(), m.Frames())

	names := [32]string{
		"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
		"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
		"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
		"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
	}
	for i := 0; i < 32; i += 4 {
		for j := i; j < i+4; j++ {
			fmt.Fprintf(&sb, "  %-4s %08x", names[j], c.R[j])
		}
		sb.WriteByte('\n')
	}
	fmt.Fprintf(&sb, "  pc   %08x  sr   %08x  cause %08x  epc %08x\n",
		c.PC, c.COP0[cpu.SR], c.COP0[cpu.Cause], c.COP0[cpu.EPC])

	n := min(m.steps, recentSteps)
	fmt.Fprintf(&sb, "\nlast %d instructions:\n", n)
	for i := uint64(0); i < n; i++ {
		s := m.recent[(m.next+recentSteps-int(n)+int(i))%recentSteps]
		fmt.Fprintf(&sb, "  %08x: %08x  %s\n", s.pc, s.insn, disassemble(s.insn))
	}

	tty := m.tty.String()
	if lines := strings.Split(strings.TrimRight(tty, "\n"), "\n"); len(lines) > 10 {
		tty = strings.Join(lines[len(lines)-10:], "\n")
	}
	if tty == "" {
		tty = "(nothing)"
	}
	fmt.Fprintf(&sb, "\nTTY, last lines:\n%s\n", tty)
	return sb.String()
}

func (c Cycles) String() string {
	return fmt.Sprintf("%.3fs", float64(c)/float64(Second))
}

func disassemble(insn uint32) string {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], insn)
	inst, err := mips.Decode(b[:])
	if err != nil {
		return "?"
	}
	return inst.String()
}
