package psx

import (
	"fmt"
)

const (
	size = 2 * 1024 * 1024
	mask = size - 1
)

type memory [size]byte

func (m *memory) read(start, end uint32) []byte {
	// By length, since an end at the top of RAM masks to 0.
	from := start & mask
	return m[from : from+end-start]
}

func (m *memory) write(start uint32, data []byte) int {
	return copy(m[start&mask:], data)
}

// New makes an executable of a flat image: text loaded at addr and entered
// at entry, in a single section. The stack and the rest of the header are
// left for the caller.
func New(addr, entry uint32, text []byte) *File {
	return &File{
		FileHeader: FileHeader{
			Magic:    ExecutableSignature,
			PC0:      entry,
			TextAddr: addr,
			TextSize: uint32(len(text)),
		},
		Sections: []*Section{{Name: "text", Addr: addr, Data: text}},
	}
}

// Combine lays b's text over a's in one image, from a's text address to the
// end of b's, entered at b's entry point: how a Net Yaroze program is joined
// with the resident library below it.
func Combine(a, b *File) (*File, error) {
	m := &memory{}
	m.write(a.TextAddr, a.Section("text").Data)
	m.write(b.TextAddr, b.Section("text").Data)
	output := New(a.TextAddr, b.PC0, m.read(a.TextAddr, b.TextAddr+b.TextSize))
	output.StackAddr = 0x801fff00
	output.SetMarker("COMBINE version 1.00")
	return output, nil
}

// sectorSize is the unit of a text section the BIOS can load from a disc.
const sectorSize = 2048

// Embed writes data into the image at addr, as a Net Yaroze's siocons put a
// program's data in memory before the program ran with its "local dload"
// command: a program that expects its data at a fixed address can then be
// run from a disc, or in an emulator, without a loader to put it there. The
// text section grows to cover data outside it, with zeros between, and up to
// a whole sector, since the BIOS doesn't load a text size that isn't one.
func Embed(f *File, addr uint32, data []byte) error {
	// RAM is the same RAM through the uncached mirror a program may name
	// it by, at 0xa0000000, and the image is loaded through the cached one.
	if addr&0xe0000000 == 0xa0000000 {
		addr = addr&0x1fffffff | 0x80000000
	}
	text := f.Section("text")
	if text == nil {
		return fmt.Errorf("executable has no text")
	}
	start, end := text.Addr, text.Addr+uint32(len(text.Data))
	lo, hi := min(start, addr), max(end, addr+uint32(len(data)))
	hi = lo + (hi-lo+sectorSize-1)&^(sectorSize-1)
	if hi-lo > size {
		return fmt.Errorf("%d bytes at %08x do not fit in RAM with the text at %08x", len(data), addr, text.Addr)
	}
	m := &memory{}
	m.write(start, text.Data)
	m.write(addr, data)
	text.Addr = lo
	text.Data = m.read(lo, hi)
	f.TextAddr = lo
	f.TextSize = hi - lo
	return nil
}
