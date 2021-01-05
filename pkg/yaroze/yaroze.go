//go:generate go run generate.go

// Package yaroze provides an interface Net Yaroze functionality.
package yaroze

import (
	"encoding/binary"

	"github.com/ChrisRx/psxsdk/pkg/binutils"
	"github.com/ChrisRx/psxsdk/pkg/format/psx"
)

func Combine(f *psx.File) (*psx.File, error) {
	return binutils.Combine(Libps, f)
}

func PatchExecutable(f *psx.File) error {
	var yarozePatch = []uint32{
		0x0c00400c,
		0x0,
		0x08000000 + (f.PC0&0x03ffffff)>>2,
		0x0,
	}
	patchData := make([]byte, 4*len(yarozePatch))
	for i, val := range yarozePatch {
		binary.LittleEndian.PutUint32(patchData[4*i:], val)
	}

	f.FileHeader.PC0 += uint32(len(f.Section("text").Data))
	f.FileHeader.TextSize += uint32(len(patchData))
	f.Section("text").Data = append(f.Section("text").Data, patchData...)
	return nil
}
