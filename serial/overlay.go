package serial

import (
	"encoding/binary"
	"fmt"
)

// loaderAddr is where Relocated's loader goes: near the top of RAM, below the
// stack, with what it copies just under it.
const loaderAddr = 0x801f0000

// Relocated returns prog set to start only after each of down has been copied
// into place, for a Monitor to load as usual.
//
// It's for what can't be uploaded where it belongs, because the monitor doing
// the upload is using that memory: the monitor on the boot disc stops answering
// as an upload reaches the part of the resident library it still needs, and it
// can't overwrite itself. So down goes to high RAM, with a small loader which
// the program starts at. The loader turns interrupts off, copies each segment
// into place, flushes the cache and jumps to prog's start. It leaves gp and sp
// as Load sets them.
func Relocated(prog *Program, down ...Segment) (*Program, error) {
	total := uint32(0)
	for _, seg := range down {
		total += uint32(len(seg.Data)+3) &^ 3
	}
	base := uint32(loaderAddr) - total
	for _, seg := range prog.Segments {
		if seg.Addr+uint32(len(seg.Data)) > base {
			return nil, fmt.Errorf("serial: %s reaches past %08X, where the loader's copies go", seg.Name, base)
		}
	}
	var copies []copyBlock
	segs := make([]Segment, 0, len(down)+1+len(prog.Segments))
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
		Segments: append(segs, prog.Segments...),
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
	regT0 = 8
	regT1 = 9
	regT2 = 10
	regT3 = 11
	regT4 = 12
)

// iType is a MIPS instruction with an immediate.
func iType(op, rs, rt, imm uint32) uint32 {
	return op<<26 | rs<<21 | rt<<16 | imm&0xffff
}

// li is the two instructions that set a register.
func li(r, v uint32) []uint32 {
	return []uint32{iType(0x0f, 0, r, v>>16), iType(0x0d, r, r, v)}
}

// loader is the code Relocated uploads.
func loader(copies []copyBlock, entry uint32) []byte {
	var code []uint32
	add := func(w ...uint32) { code = append(code, w...) }
	add(0x40086000, 0) // mfc0 t0, sr
	add(li(regT1, 0xfffffffe)...)
	add(0x01094024)       // and t0, t0, t1
	add(0x40886000, 0, 0) // mtc0 t0, sr
	add(li(regT0, 0x1f801074)...)
	add(iType(0x2b, regT0, 0, 0)) // sw zero, 0(t0): mask every interrupt
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
	add(li(regT1, 0x44)...) // the kernel's FlushCache, an A function
	add(li(regT0, 0xa0)...)
	add(0x0100f809, 0) // jalr t0
	add(li(regT0, entry)...)
	add(0x01000008, 0) // jr t0
	out := make([]byte, 0, len(code)*4)
	for _, w := range code {
		out = binary.LittleEndian.AppendUint32(out, w)
	}
	return out
}
