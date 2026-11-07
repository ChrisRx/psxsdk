package serial

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

// loaderAddr is where TakeOver's loader goes: near the top of RAM, below the
// stack, with what it copies just under it.
const loaderAddr = 0x801f0000

// OpenMonitor reads serial/monitor's ELF for TakeOver, set to run at baud. It
// would otherwise read its speed from the memory card, and a read that failed
// would leave it at another speed from the host, with no way to say so.
func OpenMonitor(path string, baud uint32) (*Program, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	p, err := ELFProgram(f)
	if err != nil {
		return nil, err
	}
	for _, sym := range syms {
		if sym.Name != "main.savedBaud" {
			continue
		}
		for _, seg := range p.Segments {
			if at := uint32(sym.Value) - seg.Addr; uint32(sym.Value) >= seg.Addr && at+4 <= uint32(len(seg.Data)) {
				binary.LittleEndian.PutUint32(seg.Data[at:], baud)
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("serial: %s has no main.savedBaud in its data", path)
}

// TakeOver returns what replaces the boot disc's monitor with another, on
// another resident library, for that monitor to load and run. Programs can
// then return to the new monitor, with no reset.
//
// Neither can be uploaded where it belongs, because the monitor doing the upload
// is using that memory: it stops answering as an upload reaches the part of its
// library it still needs, and it can't overwrite itself. So they go to high RAM,
// with a small loader which the upload starts at. The loader turns interrupts
// off and undoes what the old monitor left in the kernel, whose addresses would
// land in the new code: the devices it added, with the kernel's dummy terminal
// back in place of its own, and the events it opened. It does this first, while
// the old code is there for the kernel to call as it removes a device. Then it
// copies each piece into place, flushes the cache and jumps to the new monitor.
func TakeOver(monitor *Program, image Segment) (*Program, error) {
	prog := &Program{PC: monitor.PC, GP: monitor.GP, SP: monitor.SP}
	down := append([]Segment{image}, monitor.Segments...)
	total := uint32(0)
	for _, seg := range down {
		total += uint32(len(seg.Data)+3) &^ 3
	}
	base := uint32(loaderAddr) - total
	var copies []copyBlock
	segs := make([]Segment, 0, len(down)+1)
	at := base
	for _, seg := range down {
		data := append(make([]byte, 0, len(seg.Data)+3), seg.Data...)
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		copies = append(copies, copyBlock{src: at, dst: seg.Addr, size: uint32(len(data))})
		segs = append(segs, Segment{Name: seg.Name, Addr: at, Data: data})
		at += uint32(len(data))
	}
	segs = append(segs, Segment{Name: "loader", Addr: loaderAddr, Data: loader(copies, prog.PC)})
	return &Program{
		Segments: segs,
		PC:       loaderAddr,
		GP:       prog.GP,
		SP:       prog.SP,
	}, nil
}

// copyBlock is a copy the loader does.
type copyBlock struct {
	src, dst, size uint32
}

// The registers the loader uses.
const (
	regA0 = 4
	regT0 = 8
	regT1 = 9
	regT2 = 10
	regT3 = 11
	regT4 = 12
	regSP = 29
)

// monitorStack is where the clean monitor's stack starts: 16 bytes below the
// __stack_top its target.json gives.
const monitorStack = 0x80090000 - 16

// iType is a MIPS instruction with an immediate.
func iType(op, rs, rt, imm uint32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | imm&0xffff
}

// li is the two instructions that set a register.
func li(r, v uint32) []uint32 {
	return []uint32{iType(0x0f, 0, r, v>>16), iType(0x0d, r, r, v)}
}

// The kernel's calls the loader makes, by table and number.
const (
	tableA         = 0xa0
	tableB         = 0xb0
	callFlushCache = 0x44 // A
	callAddDummy   = 0x99 // A
	callClose      = 0x09 // B, CloseEvent
	callRemove     = 0x48 // B, RemoveDevice
)

// The boot disc's monitor's devices, which replace or add to the kernel's, and
// how many events the boot disc's configuration gives the kernel. The first is
// the kernel's own, and an event's handle is its number with eventBase. Closing
// one that isn't open does no harm.
const (
	eventBase   = 0xf1000000
	eventsCount = 16
)

var monitorDevices = []string{"tty", "mem", "cdx"}

// namesAt is where in the loader's upload the device names go, past its code.
const namesAt = 0x1000

// loader is the code TakeOver uploads.
func loader(copies []copyBlock, entry uint32) []byte {
	var code []uint32
	add := func(w ...uint32) { code = append(code, w...) }
	call := func(table, fn uint32) {
		add(li(regT1, fn)...)
		add(li(regT0, table)...)
		add(0x0100f809, 0) // jalr t0
	}
	add(0x40086000, 0) // mfc0 t0, sr
	add(li(regT1, 0xfffffffe)...)
	add(0x01094024)       // and t0, t0, t1
	add(0x40886000, 0, 0) // mtc0 t0, sr
	add(li(regT0, 0x1f801074)...)
	add(iType(0x2b, regT0, 0, 0)) // sw zero, 0(t0): mask every interrupt
	var names []byte
	for _, name := range monitorDevices {
		add(li(regA0, loaderAddr+namesAt+uint32(len(names)))...)
		names = append(append(names, name...), 0)
		call(tableB, callRemove)
	}
	call(tableA, callAddDummy)
	for i := uint32(1); i < eventsCount; i++ {
		add(li(regA0, eventBase|i)...)
		call(tableB, callClose)
	}
	for _, c := range copies {
		add(li(regT1, c.src)...)
		add(li(regT2, c.dst)...)
		add(li(regT3, c.dst+c.size)...)
		loop := len(code)
		add(iType(0x23, regT1, regT4, 0))                        // lw t4, 0(t1)
		add(iType(0x09, regT1, regT1, 4))                        // addiu t1, t1, 4
		add(iType(0x2b, regT2, regT4, 0))                        // sw t4, 0(t2)
		add(iType(0x09, regT2, regT2, 4))                        // addiu t2, t2, 4
		add(iType(0x05, regT2, regT3, uint32(loop-len(code)-1))) // bne t2, t3, loop
		add(0)
	}
	call(tableA, callFlushCache)
	// The monitor's stack is the top of its own space, as target.json reserves
	// it. Left where the boot disc's monitor had it, at the top of RAM, it
	// would be under a program's stack, and a program that reads its stack
	// before setting it would find the monitor's leftovers there.
	add(li(regSP, monitorStack)...)
	add(li(regT0, entry)...)
	add(0x01000008, 0) // jr t0
	out := make([]byte, 0, len(code)*4)
	for _, w := range code {
		out = binary.LittleEndian.AppendUint32(out, w)
	}
	out = append(out, make([]byte, namesAt-len(out))...)
	return append(out, names...)
}
