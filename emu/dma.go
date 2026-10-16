package emu

import "fmt"

// DMA channels.
const (
	dmaMDECIn = iota
	dmaMDECOut
	dmaGPU
	dmaCDROM
	dmaSPU
	dmaPIO
	dmaOTC
)

// CHCR bits.
const (
	chcrToDevice = 1 << 0
	chcrBackward = 1 << 1
	chcrBusy     = 1 << 24
	chcrTrigger  = 1 << 28
)

// DICR bits.
const (
	dicrForce    = 1 << 15
	dicrIRQEn    = 1 << 23
	dicrFlags    = 0x7f000000
	dicrIRQSig   = 1 << 31
	dicrEnShift  = 16
	dicrFlgShift = 24
)

// CHCR reads have these bits forced on and off.
var (
	chcrSet   = [7]uint32{6: 0x00000002}
	chcrClear = [7]uint32{0x71770703, 0x71770703, 0x71770703, 0x71770703, 0x71770703, 0x71770703, 0x50000002}
)

// dma is the DMA controller at 1F801080. psxe runs a transfer to completion
// the moment it is started, and raises its completion flag on the next step,
// or for the MDEC channels, a step per word later.
type dma struct {
	// regs is MADR, BCR and CHCR for each channel in turn.
	regs [7 * 3]uint32
	dpcr uint32
	dicr uint32

	// pending counts the steps until a channel's completion flag is set.
	pending [7]int

	// settled is DICR as the last update left it.
	settled uint32

	// active is whether an update has anything to do. Only the register
	// writes, which start transfers and change DICR, and update itself
	// change what it reads, so a write sets it and update clears it once
	// nothing is pending and DICR has settled. wake is set alongside it,
	// for the CPU to hand back to the machine.
	active bool
	wake   *bool

	bus  *Bus
	ic   *ic
	stop func(error)
}

// newDMA returns a DMA controller with every channel idle, setting wake
// when it becomes active and reporting an error that stops the run to stop.
func newDMA(bus *Bus, ic *ic, wake *bool, stop func(error)) *dma {
	return &dma{bus: bus, ic: ic, wake: wake, stop: stop, dpcr: 0x07654321}
}

func (d *dma) madr(ch int) *uint32 { return &d.regs[ch*3] }
func (d *dma) bcr(ch int) *uint32  { return &d.regs[ch*3+1] }
func (d *dma) chcr(ch int) *uint32 { return &d.regs[ch*3+2] }

func (d *dma) step(ch int) uint32 {
	if *d.chcr(ch)&chcrBackward != 0 {
		return ^uint32(3) // -4
	}
	return 4
}

// reg finds a channel register. psxe indexes them as a flat array, so the
// fourth word of a channel's block reaches the next channel's MADR; the one
// after channel 6 reaches past them, which is left reading zero here.
func (d *dma) reg(off uint32) (*uint32, int, int) {
	ch, r := int(off>>4&7), int(off>>2&3)
	i := ch*3 + r
	if i >= len(d.regs) {
		return nil, ch, r
	}
	return &d.regs[i], ch, r
}

func (d *dma) Read32(off uint32) uint32 {
	if off < 0x70 {
		p, ch, r := d.reg(off)
		if p == nil {
			return 0
		}
		v := *p
		if r == 2 {
			v |= chcrSet[ch]
			v &= chcrClear[ch]
		}
		return v
	}
	switch off {
	case 0x70:
		return d.dpcr
	case 0x74:
		return d.dicr
	}
	return 0
}

func (d *dma) Read16(off uint32) uint16 {
	switch off {
	case 0x70:
		return uint16(d.dpcr)
	case 0x74:
		return uint16(d.dicr)
	case 0x76:
		return uint16(d.dicr >> 16)
	}
	return 0
}

func (d *dma) Read8(off uint32) uint8 {
	switch off {
	case 0x70:
		return uint8(d.dpcr)
	case 0x74, 0x75, 0x76, 0x77:
		return uint8(d.dicr >> (8 * (off - 0x74)))
	}
	return 0
}

// writeDICR acknowledges the flags written as 1 and replaces the rest of
// the low 24 bits.
func (d *dma) writeDICR(v uint32) {
	flags := d.dicr & dicrFlags &^ (v & dicrFlags)
	d.dicr = d.dicr&dicrIRQSig | flags | v&0xffffff
}

func (d *dma) Write32(off, v uint32) {
	d.active, *d.wake = true, true
	if off < 0x70 {
		p, ch, r := d.reg(off)
		if p == nil {
			return
		}
		*p = v
		if r == 2 {
			d.start(ch)
		}
		return
	}
	switch off {
	case 0x70:
		d.dpcr = v
	case 0x74:
		d.writeDICR(v)
	}
}

func (d *dma) Write16(off uint32, v uint16) {
	d.active, *d.wake = true, true
	switch off {
	case 0x74:
		// psxe's switch falls through, so the value lands in both halves.
		d.writeDICR(uint32(v))
		d.writeDICR(uint32(v) << 16)
	case 0x76:
		d.writeDICR(uint32(v) << 16)
	}
}

func (d *dma) Write8(off uint32, v uint8) {
	d.active, *d.wake = true, true
	switch off {
	case 0x74, 0x75, 0x76, 0x77:
		d.writeDICR(uint32(v) << (8 * (off - 0x74)))
	}
}

// start runs a transfer when its CHCR is written.
func (d *dma) start(ch int) {
	chcr := *d.chcr(ch)
	if chcr&chcrBusy == 0 {
		return
	}
	switch ch {
	case dmaMDECIn:
		d.transferMDECIn()
	case dmaMDECOut:
		d.transferMDECOut()
	case dmaGPU:
		d.transferGPU()
	case dmaCDROM:
		d.transferCDROM()
	case dmaSPU:
		d.transferSPU()
	case dmaPIO:
		// psxe does not implement it.
	case dmaOTC:
		d.transferOTC()
	}
}

func (d *dma) size(ch int) uint32   { return *d.bcr(ch) & 0xffff }
func (d *dma) blocks(ch int) uint32 { return *d.bcr(ch) >> 16 }

func (d *dma) transferMDECIn() {
	n := d.size(dmaMDECIn) * d.blocks(dmaMDECIn)
	madr := d.madr(dmaMDECIn)
	for range n {
		d.bus.Write32(0x1f801820, d.bus.Read32(*madr))
		*madr += d.step(dmaMDECIn)
	}
	d.pending[dmaMDECIn] = int(n)
	*d.chcr(dmaMDECIn), *d.bcr(dmaMDECIn) = 0, 0
}

func (d *dma) transferMDECOut() {
	n := d.size(dmaMDECOut) * d.blocks(dmaMDECOut)
	madr := d.madr(dmaMDECOut)
	for range n {
		d.bus.Write32(*madr, d.bus.Read32(0x1f801820))
		*madr += d.step(dmaMDECOut)
	}
	d.pending[dmaMDECOut] = int(n)
	*d.chcr(dmaMDECOut), *d.bcr(dmaMDECOut) = 0, 0
}

func (d *dma) transferGPU() {
	chcr := d.chcr(dmaGPU)
	switch (*chcr >> 9) & 3 {
	case 0:
		d.stop(fmt.Errorf("GPU DMA in burst mode, which psxe does not implement"))
		return
	case 1:
		d.transferGPURequest()
	case 2:
		d.transferGPULinked()
	}
	*chcr &^= chcrBusy | chcrTrigger
	*d.bcr(dmaGPU) = 0
}

func (d *dma) transferGPURequest() {
	n := d.size(dmaGPU) * d.blocks(dmaGPU)
	madr := d.madr(dmaGPU)
	toDevice := *d.chcr(dmaGPU)&chcrToDevice != 0
	for range n {
		if toDevice {
			d.bus.Write32(0x1f801810, d.bus.Read32(*madr))
		} else {
			d.bus.Write32(*madr, d.bus.Read32(0x1f801810))
		}
		*madr += d.step(dmaGPU)
	}
	d.pending[dmaGPU] = int(n)
}

// transferGPULinked walks a linked list of GPU packets: each node's header
// holds its length in words in the top byte and the next node's address in
// the rest, with 0xffffff ending the list. psxe gives up after 16384 nodes.
func (d *dma) transferGPULinked() {
	addr := *d.madr(dmaGPU)
	hdr := d.bus.Read32(addr)
	size := hdr >> 24
	for range 16384 {
		for ; size > 0; size-- {
			addr = (addr + d.step(dmaGPU)) & 0x1ffffc
			d.bus.Write32(0x1f801810, d.bus.Read32(addr))
			d.pending[dmaGPU]++
		}
		addr = hdr & 0xffffff
		if addr == 0xffffff {
			break
		}
		hdr = d.bus.Read32(addr)
		size = hdr >> 24
	}
}

func (d *dma) transferCDROM() {
	n := d.size(dmaCDROM)
	if n == 0 {
		n = 0x10000
	}
	d.pending[dmaCDROM] = 1
	madr := d.madr(dmaCDROM)
	if *d.chcr(dmaCDROM)&chcrToDevice == 0 {
		for range n {
			var v uint32
			for b := range 4 {
				v |= uint32(d.bus.Read8(0x1f801802)) << (8 * b)
			}
			d.bus.Write32(*madr, v)
			*madr += d.step(dmaCDROM)
		}
	}
	*d.chcr(dmaCDROM), *d.bcr(dmaCDROM) = 0, 0
}

func (d *dma) transferSPU() {
	n, blocks := d.size(dmaSPU), d.blocks(dmaSPU)
	d.pending[dmaSPU] = 32
	madr := d.madr(dmaSPU)
	toDevice := *d.chcr(dmaSPU)&chcrToDevice != 0
	for range blocks {
		for range n {
			if toDevice {
				v := d.bus.Read32(*madr)
				d.bus.Write16(0x1f801da8, uint16(v))
				d.bus.Write16(0x1f801da8, uint16(v>>16))
			} else {
				v := uint32(d.bus.Read16(0x1f801da8))
				v |= uint32(d.bus.Read16(0x1f801da8)) << 16
				d.bus.Write32(*madr, v)
			}
			*madr += d.step(dmaSPU)
		}
	}
	*d.chcr(dmaSPU), *d.bcr(dmaSPU) = 0, 0
}

// transferOTC clears an ordering table: it writes a linked list running
// backwards from MADR, each entry pointing at the one before it and the last
// marking the end.
func (d *dma) transferOTC() {
	const dpcrOTCEnable = 1 << 27
	chcr := *d.chcr(dmaOTC)
	if d.dpcr&dpcrOTCEnable == 0 || chcr&chcrTrigger == 0 {
		return
	}
	n := d.size(dmaOTC)
	if n == 0 {
		n = 0x10000
	}
	madr := d.madr(dmaOTC)
	for i := n; i > 0; i-- {
		next := *madr - 4
		if i == 1 {
			next = 0xffffff
		}
		d.bus.Write32(*madr, next&0xffffff)
		*madr -= 4
	}
	d.pending[dmaOTC] = int(n)
	*d.chcr(dmaOTC), *d.bcr(dmaOTC) = 0, 0
}

// update raises the completion flags that are due, and the DMA interrupt on
// the rising edge of the master flag.
func (d *dma) update() {
	if d.active {
		d.settle()
	}
}

func (d *dma) settle() {
	// With nothing pending and DICR as the last update left it, an update
	// would recompute the same signal and change nothing.
	p := &d.pending
	if p[0]|p[1]|p[2]|p[3]|p[4]|p[5]|p[6] == 0 && d.dicr == d.settled {
		d.active = false
		return
	}
	defer func() { d.settled = d.dicr }()

	for ch, n := range d.pending {
		if n == 0 {
			continue
		}
		// The MDEC channels count down a step per word; psxe completes
		// the others on the next step regardless of their size.
		if ch == dmaMDECIn || ch == dmaMDECOut {
			n--
		} else {
			n = 0
		}
		d.pending[ch] = n
		if n == 0 && d.dicr&(1<<(dicrEnShift+ch)) != 0 {
			d.dicr |= 1 << (dicrFlgShift + ch)
		}
	}

	prev := d.dicr&dicrIRQSig != 0
	signal := d.dicr&dicrForce != 0 || (d.dicr&dicrFlags != 0 && d.dicr&dicrIRQEn != 0)
	if signal && !prev {
		d.ic.raise(irqDMA)
	}
	d.dicr &^= dicrIRQSig
	if signal {
		d.dicr |= dicrIRQSig
	}
}
