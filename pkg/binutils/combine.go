// Package binutils provides an interface for manipulating object files used by
// the Sony Playstation 1.
package binutils

import (
	"github.com/ChrisRx/psxsdk/pkg/format/psx"
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

func Combine(a, b *psx.File) (*psx.File, error) {
	m := &memory{}
	m.write(a.TextAddr, a.Section("text").Data)
	m.write(b.TextAddr, b.Section("text").Data)
	data := m.read(a.TextAddr, b.TextAddr+b.TextSize)
	output := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:     psx.ExecutableSignature,
			PC0:       b.PC0,
			TextAddr:  a.TextAddr,
			TextSize:  uint32(len(data)),
			StackAddr: 0x801fff00,
		},
		Sections: []*psx.Section{
			&psx.Section{
				Name: "text",
				Addr: a.TextAddr,
				Data: data,
			},
		},
	}
	output.SetMarker("COMBINE version 1.00")
	return output, nil
}
