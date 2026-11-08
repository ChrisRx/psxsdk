package emu

// The GPU in the emulator draws in no time: GPUSTAT always says it is ready
// for a command, and a DMA to it runs a whole packet list the moment CHCR is
// written. A program's waits for the GPU never wait. Under SetDrawTime the
// GPU takes time for what it draws instead, roughly, so that a test can see
// what a library does while the GPU is still drawing: GPUSTAT's bit 26 (ready
// for a command) stays clear until the drawing is done, a packet list sent by
// DMA while the GPU is drawing is held until it is free, and a list's DMA
// ends when its last command reaches the GPU, which then goes on drawing it.
// The pixels still land in VRAM as each command arrives.
//
// The cost is a fixed time per pixel drawn. A console takes about 0.8 cycles
// a pixel for 320 by 240 semi-transparent boxes (spec 39 of exp/cleanlib), so
// a cycle a pixel is near enough.

// gpustatReadyCmd is GPUSTAT's bit 26: ready for a command.
const gpustatReadyCmd = 1 << 26

// drawCyclesPerPixel is the CPU cycles a drawn pixel costs under SetDrawTime.
const drawCyclesPerPixel = 1

// drawTime is the GPU's clock for SetDrawTime: whether it is on, the CPU's
// cycle count, when the GPU will have drawn what it has been given, and what
// the last drawing command cost.
type drawTime struct {
	on    bool
	clock *uint64
	until uint64
	last  uint64
}

// SetDrawTime sets whether the GPU takes time to draw, as described above.
// It is off unless a test turns it on, and with it off nothing about the
// emulation changes.
func (m *Machine) SetDrawTime(on bool) {
	m.gpu.time.on = on
	m.gpu.time.until = 0
}

// GPUDrawing reports whether, under SetDrawTime, the GPU is still drawing.
func (m *Machine) GPUDrawing() bool { return m.gpu.drawing() }

// drawing reports whether the GPU is still drawing what it has been given.
func (g *gpu) drawing() bool { return *g.time.clock < g.time.until }

// charge adds the time to draw pixels to the GPU's, after whatever it is
// drawing already.
func (g *gpu) charge(pixels int) {
	if !g.time.on || pixels <= 0 {
		return
	}
	t := &g.time
	t.last = uint64(pixels) * drawCyclesPerPixel
	t.until = max(t.until, *t.clock) + t.last
}

// triangleArea returns the pixels a triangle covers, roughly: its area.
func triangleArea(v0, v1, v2 vertex) int {
	a := (int(v1.x)-int(v0.x))*(int(v2.y)-int(v0.y)) - (int(v2.x)-int(v0.x))*(int(v1.y)-int(v0.y))
	return max(a, -a) / 2
}

// transferGPUTimed is the linked-list transfer under SetDrawTime: held while
// the GPU is drawing, and once run, ending when the list's last command
// reaches the GPU.
func (d *dma) transferGPUTimed() {
	g := d.gpu
	if g.drawing() {
		d.gpuEnd, d.gpuHeld = g.time.until, true
		return
	}
	g.time.last = 0
	d.transferGPULinked()
	d.gpuWords, d.pending[dmaGPU] = d.pending[dmaGPU], 0
	if end := g.time.until - g.time.last; g.drawing() && end > *g.time.clock {
		d.gpuEnd = end
		return
	}
	d.endGPU()
}

// gpuDue moves a timed GPU transfer on once its time has come: a held list
// starts, and a running one ends.
func (d *dma) gpuDue() {
	if *d.gpu.time.clock < d.gpuEnd {
		return
	}
	d.gpuEnd = 0
	if d.gpuHeld {
		d.gpuHeld = false
		d.transferGPUTimed()
		return
	}
	d.endGPU()
}

// endGPU ends a timed transfer: the channel stops, and its completion flag
// comes on the next step, as an untimed transfer's does.
func (d *dma) endGPU() {
	*d.chcr(dmaGPU) &^= chcrBusy | chcrTrigger
	*d.bcr(dmaGPU) = 0
	d.pending[dmaGPU] = d.gpuWords
}
