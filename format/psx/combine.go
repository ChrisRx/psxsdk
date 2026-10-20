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
	return m[start&mask : end&mask]
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

// Embed writes data into the image at addr, as a Net Yaroze's siocons put a
// program's data in memory before the program ran with its "local dload"
// command: a program that expects its data at a fixed address can then be
// run from a disc, or in an emulator, without a loader to put it there. The
// text section grows to cover data outside it, with zeros between.
func Embed(f *File, addr uint32, data []byte) error {
	text := f.Section("text")
	if text == nil {
		return fmt.Errorf("executable has no text")
	}
	start, end := text.Addr, text.Addr+uint32(len(text.Data))
	lo, hi := min(start, addr), max(end, addr+uint32(len(data)))
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
