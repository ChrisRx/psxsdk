package link

import (
	"bytes"
	"fmt"
	"os"

	"go.chrisrx.dev/psx/format/psx"
)

// Runnable makes a PSX-EXE that runs without a Net Yaroze's boot disc of
// the executable at path, a PSX-EXE, an ECOFF or an ELF, with each of loads
// put into it at its address. A PSX-EXE is taken as it is. An ECOFF or an
// ELF is a Net Yaroze program, and gets the library laid under it, as
// Executable does, unless its image reaches down into the library's, which
// means it brought its own, as a program built with the library linked in
// does, and it is taken as Standalone does.
func Runnable(path string, loads ...Load) (*psx.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if IsPSXEXE(data) {
		exe, err := psx.ParseFile(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, l := range loads {
			if err := psx.Embed(exe, l.Addr, l.Data); err != nil {
				return nil, err
			}
		}
		return exe, nil
	}
	img, err := ReadImage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	libps, err := Libps()
	if err != nil {
		return nil, err
	}
	if img.Addr < libps.TextAddr+uint32(len(libps.Section("text").Data)) {
		return Standalone(img, loads...)
	}
	return Executable(img, true, loads...)
}

// IsPSXEXE reports whether data starts as a PSX-EXE does.
func IsPSXEXE(data []byte) bool {
	return bytes.HasPrefix(data, psx.ExecutableSignature[:])
}

// IsExecutable reports whether data starts as one of the executables
// Runnable takes does: a PSX-EXE, an ECOFF for the R3000 or an ELF.
func IsExecutable(data []byte) bool {
	return IsPSXEXE(data) ||
		bytes.HasPrefix(data, []byte{0x62, 0x01}) ||
		bytes.HasPrefix(data, []byte("\x7fELF"))
}
