// Package emu emulates a PlayStation, written to be driven from tests rather
// than played: it runs headless and deterministically, in emulated cycles
// with no reference to the wall clock.
//
// It follows psxe closely, so that the two can be compared instruction by
// instruction with a trace (see format/psxtrace). Devices not ported yet are
// mapped as reading zero.
package emu

import (
	"fmt"
	"image"
	"image/draw"
	"io"

	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxtrace"
)

// spuSampleCycles is how many CPU cycles there are to an SPU sample.
const spuSampleCycles = 768

// RAMSize is the size of main memory.
const RAMSize = 2 << 20

// Machine is a whole console.
type Machine struct {
	CPU *cpu.CPU
	Bus *Bus

	// TTY receives what the BIOS prints through its putchar and puts
	// functions, which is where the kernel's boot messages and a program's
	// printf output go.
	TTY io.Writer

	ram  *ram
	ic   *ic
	exp2 *exp2
	dma  *dma
	spu  *spu
	gpu  *gpu
	tmr  *timers
	cd   *cdrom
	sio  *sio0

	// spuCycles counts towards the next SPU sample.
	spuCycles uint32

	// steps counts the steps run, which is the timers' clock.
	steps uint64

	// fail is where the devices report an error.
	fail failure

	trace     *psxtrace.Writer
	traceRegs [psxtrace.NumRegs]uint32
	writes    []psxtrace.Write
}

// New returns a machine about to boot the given BIOS image.
func New(bios []byte) (*Machine, error) {
	if len(bios) == 0 || len(bios)&(len(bios)-1) != 0 {
		return nil, fmt.Errorf("BIOS image is %d bytes, which is not a power of two", len(bios))
	}

	m := &Machine{Bus: &Bus{}}
	m.CPU = cpu.New(m.Bus)

	mc2 := newMC2()
	m.ram = &ram{memory: make(memory, RAMSize), mc2: mc2}
	m.ic = &ic{setIRQ: m.CPU.SetIRQ}
	m.exp2 = &exp2{}
	// The CD-ROM drive and the pad count CPU cycles towards their events,
	// so their clock is the CPU's.
	cycles := &m.CPU.TotalCycles
	m.dma = newDMA(m.Bus, m.ic, m.fail.report)
	m.spu = newSPU(m.ic)
	m.gpu = newGPU(m.ic, m.fail.report)
	m.tmr = &timers{gpu: m.gpu, ic: m.ic, steps: &m.steps}
	m.gpu.timers = m.tmr
	m.cd = newCDROM(m.ic, cycles, m.fail.report)
	m.sio = newSIO0(m.ic, cycles, [2]*Pad{newDigitalPad(), nil})

	// The order is psxe's, which only matters where ranges overlap.
	m.Bus.setMaps([]mapping{
		{base: 0x1fc00000, size: uint32(len(bios)), delay: 18, dev: rom{memory(bios)}},
		{base: 0x00000000, size: 0x800000, dev: m.ram},
		{base: 0x1f801080, size: 0x80, dev: m.dma},
		{base: 0x1f000000, size: 0x80000, dev: rom{expansion1()}},
		{base: 0x1f802000, size: 0x1fe000, dev: m.exp2},
		{base: 0x1f801000, size: 0x24, dev: newMC1()},
		{base: 0x1f801060, size: 0x4, dev: mc2},
		{base: 0xfffe0130, size: 0x4, dev: make(registers, 1)}, // cache control
		{base: 0x1f801070, size: 0x8, dev: m.ic},
		{base: 0x1f800000, size: 0x400, dev: make(memory, 0x400)}, // scratchpad
		{base: 0x1f801810, size: 0x8, dev: m.gpu},
		{base: 0x1f801c00, size: 0x400, dev: m.spu},
		{base: 0x1f801100, size: 0x30, dev: m.tmr},
		{base: 0x1f801800, size: 0x4, dev: m.cd},
		{base: 0x1f801040, size: 0x10, dev: m.sio},
		{base: 0x1f801820, size: 0x8, dev: unmapped{}}, // MDEC
	})
	// Most accesses are to RAM, and code runs from RAM and the BIOS. The
	// first 4MB of RAM's window is populated whatever RAM_SIZE says, so it
	// reads as plain memory.
	m.Bus.direct(
		m.Bus.window(0x00000000, 0x400000, m.ram.memory),
		m.Bus.window(0x1fc00000, uint32(len(bios)), memory(bios)),
	)
	m.SetCDAudio(true)
	// The CPU fetches from RAM itself, saving a call through the bus.
	m.CPU.SetWindow(cpu.Window{
		Mem:    m.Bus.ram.mem,
		Size:   m.Bus.ram.size,
		Delay:  m.Bus.ram.delay,
		Charge: &m.Bus.cycles,
	})
	return m, nil
}

// expansion1 is an empty expansion port, which reads as all ones.
func expansion1() memory {
	m := make(memory, 0x80000)
	for i := range m {
		m[i] = 0xff
	}
	return m
}

// VRAM returns video memory: 1024x512 16-bit pixels, row by row. It is the
// GPU's own, not a copy.
func (m *Machine) VRAM() []uint16 {
	return m.gpu.vram[:]
}

// shellEntry is where the BIOS jumps to its shell once the kernel is set up,
// and where psxe loads an executable given with -x.
const shellEntry = 0x80030000

// SideloadEXE boots the BIOS until it is about to enter its shell, and then
// loads an executable in its place, as psxe does with -x: the image is
// copied into RAM, and PC, GP and, if the header gives a stack, SP and FP
// are set from the header. budget bounds the instructions the boot may take.
func (m *Machine) SideloadEXE(exe *psx.File, budget uint64) error {
	for i := uint64(0); !m.AtShell(); i++ {
		if i == budget {
			return fmt.Errorf("the BIOS did not reach its shell in %d instructions", budget)
		}
		if err := m.Step(); err != nil {
			return err
		}
	}
	return m.LoadEXE(exe)
}

// AtShell reports whether the BIOS is about to enter its shell, where
// SideloadEXE loads an executable.
func (m *Machine) AtShell() bool { return m.CPU.PC == shellEntry }

// LoadEXE loads an executable and points the CPU at it, as SideloadEXE does
// once the BIOS is ready for it.
func (m *Machine) LoadEXE(exe *psx.File) error {
	text := exe.Section("text")
	if text == nil {
		return fmt.Errorf("executable has no text")
	}
	off := exe.TextAddr & 0x7fffffff
	if int(off)+len(text.Data) > len(m.ram.memory) {
		return fmt.Errorf("executable of %d bytes at %08x does not fit in RAM", len(text.Data), exe.TextAddr)
	}
	copy(m.ram.memory[off:], text.Data)

	c := m.CPU
	c.PC = exe.PC0
	c.NextPC = c.PC + 4
	c.R[28] = exe.GP0
	if exe.StackAddr != 0 {
		c.R[29] = exe.StackAddr + exe.StackSize
		c.R[30] = c.R[29]
	}
	return nil
}

// Display returns what the screen shows: the part of VRAM the display area
// starts at, as large as the display mode makes it, worked out as psxe's
// frontend does. With the display turned off, it is black.
//
// The 24-bit display mode, used for video, is not decoded; it shows as
// 15-bit pixels.
func (m *Machine) Display() *image.RGBA {
	return m.DisplayInto(nil)
}

// DisplayInto is Display drawing into img, which it reuses if it is the
// size the display is, and otherwise replaces. It returns the image drawn.
// A frontend calling it every frame keeps one image rather than allocating
// one a frame.
func (m *Machine) DisplayInto(img *image.RGBA) *image.RGBA {
	g := m.gpu
	width := [4]int{256, 320, 512, 640}[g.displayMode&3]
	if g.displayMode&0x40 != 0 {
		width = 384
	}
	height := 240
	if g.displayMode&0x4 != 0 {
		height = 480
	} else if d := int(g.dispY2) - int(g.dispY1); d < 255-16 {
		height = max(d, 0)
	}

	if r := image.Rect(0, 0, width, height); img == nil || img.Rect != r {
		img = image.NewRGBA(r)
	}
	if g.gpustat&gpustatDisplayOff != 0 {
		draw.Draw(img, img.Rect, image.Black, image.Point{}, draw.Src)
		return img
	}
	var wrapped [vramWidth]uint16
	for y := range height {
		vy := (int(g.dispY) + y) & (vramHeight - 1)
		row := g.vram[vy*vramWidth : (vy+1)*vramWidth]
		// A display area running off the right of VRAM wraps to its left.
		src := row[int(g.dispX)&(vramWidth-1):]
		if len(src) < width {
			n := copy(wrapped[:], src)
			copy(wrapped[n:width], row)
			src = wrapped[:]
		}
		rgba15(img.Pix[y*img.Stride:y*img.Stride+4*width], src[:width])
	}
	return img
}

// Frames returns how many frames have been displayed, counted at each
// vertical blank.
func (m *Machine) Frames() uint64 {
	return m.gpu.frames
}

// SetAudio turns on or off keeping the SPU's sound for Audio to return. A
// machine starts with it off. The sound is made either way, since the
// reverb writes to SPU RAM as it plays, so this changes nothing a program
// can see.
func (m *Machine) SetAudio(on bool) { m.spu.setAudio(on) }

// Audio appends the sound made since the last call to dst, and returns it:
// interleaved left and right 16-bit samples at 44.1kHz, a pair for every
// 768 CPU cycles. Up to a second's worth is kept between calls; past that,
// the oldest is dropped.
//
// It is psxe's mix of the voices and reverb. CD audio is not emulated.
func (m *Machine) Audio(dst []int16) []int16 { return m.spu.takeAudio(dst) }

// SetCDAudio turns CD audio, CD-DA and XA-ADPCM, on or off. A machine starts
// with it on. Off, the drive and SPU do what psxe does while tracing, which
// leaves CD audio out: Play plays nothing, XA sectors are read as data, and
// the SPU's capture buffers stay as they are. Traces compared with psxe's
// need it off.
func (m *Machine) SetCDAudio(on bool) {
	m.cd.audioOn = on
	m.spu.capture = on
}

// Pad returns the controller in port 1 or 2, or nil if there is none. psxe
// connects a digital pad to port 1 only.
func (m *Machine) Pad(port int) *Pad {
	return m.sio.pads[port-1]
}

// InsertDisc puts a disc in the drive.
func (m *Machine) InsertDisc(d *Disc) {
	m.cd.insert(d)
}

// Trace starts writing a record of every instruction to w.
func (m *Machine) Trace(w *psxtrace.Writer) {
	m.trace = w
}

// Step runs one instruction.
func (m *Machine) Step() error {
	_, err := m.Run(1)
	return err
}

// Run runs up to n instructions, stopping at the first that a device cannot
// emulate, or that the trace cannot be written for. It returns how many it
// ran, counting the one it stopped at. A frontend running many instructions
// at a time runs them faster this way than one by one with Step.
func (m *Machine) Run(n uint64) (uint64, error) {
	return m.run(n, ^uint64(0))
}

// RunFrame runs to the next vertical blank, or for budget instructions if
// that comes first, as Run does. Whether the blank came shows in Frames.
func (m *Machine) RunFrame(budget uint64) (uint64, error) {
	return m.run(budget, m.gpu.frames+1)
}

// run is Run, stopping too once frames frames have been displayed. What is
// invariant over the run is worked out once rather than every instruction.
func (m *Machine) run(n, frames uint64) (uint64, error) {
	c := m.CPU
	cd, gpu, sio, tmr, dma, spu := m.cd, m.gpu, m.sio, m.tmr, m.dma, m.spu
	tty, trace := m.TTY != nil, m.trace != nil

	// While tracing, the CPU steps one at a time, to record each step.
	// Otherwise it runs steps itself until one leaves the devices
	// something to do, keeping the clocks below up to date as it goes.
	k := cpu.Clock{
		Steps: &m.steps, StepsDue: &tmr.due,
		Due:   [2]*uint64{&cd.due, &sio.due},
		Video: &gpu.cycles, VideoEdge: &gpu.edge, VideoRate: gpuCyclesPerCPU,
		Sample: &m.spuCycles, SampleCycles: spuSampleCycles,
		Busy: &dma.active, Err: &m.fail.err,
		Hook: 0xb4, Hooked: tty,
	}

	var i uint64
	for i < n && gpu.frames < frames {
		// A call through the B-function table lands at 0xb0 and jumps to
		// the handler at 0xb4, with the function number in $t1.
		if tty && c.PC&0x3fffffff == 0xb4 {
			m.bFunction()
		}

		if trace {
			m.snapshot(&m.traceRegs)
			c.Step()
			i++
			// The trace records the CPU's step alone, before the devices
			// catch up, which is where psxe takes it too. Whatever they
			// change shows up in the next record.
			if err := m.writeTrace(); err != nil {
				return i, err
			}
		} else {
			i += c.Run(n-i, &k)
		}

		// psxe updates the CD-ROM drive, GPU, pad, timers and DMA, in that
		// order, with the cycles the last step took. The drive, pad and timers
		// are only brought up to date when one has something due.
		if c.TotalCycles >= cd.due {
			cd.update()
		}
		gpu.update(c.LastCycles)
		if c.TotalCycles >= sio.due {
			sio.update()
		}
		m.steps++
		if m.steps >= tmr.due {
			tmr.update(m.steps)
		}
		dma.update()

		// Then, as psxe does while tracing, the SPU advances a sample every
		// 768 cycles: the CPU's 33.8688MHz over the SPU's 44.1kHz.
		m.spuCycles += c.LastCycles
		for m.spuCycles >= spuSampleCycles {
			m.spuCycles -= spuSampleCycles
			if cd.audioOn {
				spu.cdIn[0], spu.cdIn[1] = cd.audioSample()
			}
			spu.sample()
		}

		if m.fail.err != nil {
			return i, m.fail.take()
		}
	}
	return i, nil
}

// failure is where the devices report an error that stops the run: what
// psxe does not implement, or asserts on. They share one, through report,
// so a step checks one place. The first reported is kept until the machine
// returns it; a second in the same step is dropped, the machine being past
// emulating by then anyway.
type failure struct{ err error }

func (f *failure) report(err error) {
	if f.err == nil {
		f.err = err
	}
}

// take returns the error reported, and clears it.
func (f *failure) take() error {
	err := f.err
	f.err = nil
	return err
}

// bFunction prints what a call to the BIOS's putchar or puts is printing.
func (m *Machine) bFunction() {
	c := m.CPU
	switch c.R[9] {
	case 0x3b, 0x3d:
		m.TTY.Write([]byte{byte(c.R[4])})
	case 0x3f:
		var s []byte
		for a := c.R[4]; ; a++ {
			b := m.Bus.Read8(a)
			if b == 0 {
				break
			}
			s = append(s, b)
		}
		m.TTY.Write(s)
	}
}

func (m *Machine) snapshot(regs *[psxtrace.NumRegs]uint32) {
	c := m.CPU
	copy(regs[:32], c.R[:])
	regs[psxtrace.HI] = c.HI
	regs[psxtrace.LO] = c.LO
	regs[psxtrace.SR] = c.COP0[cpu.SR]
	regs[psxtrace.Cause] = c.COP0[cpu.Cause]
	regs[psxtrace.EPC] = c.COP0[cpu.EPC]
}

func (m *Machine) writeTrace() error {
	var after [psxtrace.NumRegs]uint32
	m.snapshot(&after)

	m.writes = m.writes[:0]
	for i, v := range after {
		if v != m.traceRegs[i] {
			m.writes = append(m.writes, psxtrace.Write{Reg: psxtrace.Reg(i), Value: v})
		}
	}

	c := m.CPU
	return m.trace.Write(psxtrace.Record{
		PC:     c.SavedPC,
		Insn:   c.Opcode,
		Excode: c.Excode,
		Cycles: uint8(min(c.LastCycles, 255)),
		Writes: m.writes,
	})
}
