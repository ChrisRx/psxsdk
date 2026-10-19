//go:build !yaroze

package yaroze

import (
	"encoding/binary"

	"go.chrisrx.dev/psx/binutils"
	"go.chrisrx.dev/psx/format/psx"
)

func Combine(f *psx.File) (*psx.File, error) {
	return binutils.Combine(Libps, f)
}

// Executable makes a PSX-EXE of a Net Yaroze program's image, as the
// eco2exe command does: with patch, a startup stub that initializes the
// resident library first (PatchExecutable), then the library combined in
// (Combine), so that the result runs without the Net Yaroze boot disc, and
// each of loads put into the image at its address.
func Executable(img *binutils.Image, patch bool, loads ...binutils.Load) (*psx.File, error) {
	exe := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:     psx.ExecutableSignature,
			PC0:       img.Entry,
			TextAddr:  img.Addr,
			TextSize:  img.Size(),
			StackAddr: 0x801fff00,
		},
		Sections: []*psx.Section{{Name: "text", Addr: img.Addr, Data: img.Data}},
	}
	if patch {
		if err := PatchExecutable(exe); err != nil {
			return nil, err
		}
	}
	psx.AlignTextData(exe, 2048)
	exe, err := Combine(exe)
	if err != nil {
		return nil, err
	}
	for _, l := range loads {
		if err := binutils.Embed(exe, l.Addr, l.Data); err != nil {
			return nil, err
		}
	}
	return exe, nil
}

// libpsInit is the resident library's initialization routine, which has to
// run before the program does.
const libpsInit = 0x80010030

// PatchExecutable appends a startup stub that calls libpsInit and then jumps
// to the program's entry point, and points PC0 at the stub.
//
// The stub keeps the caller's return address across the call. The program's
// crt0 saves $ra on entry and returns through it when main returns, so the
// jal must not be what it sees: that would be an address inside the stub,
// which sits at the end of the loaded image and so, for an ELF, at the start
// of .bss, cleared and reused by the time main returns.
func PatchExecutable(f *psx.File) error {
	var yarozePatch = []uint32{
		0x27bdfff8,                             // addiu sp, sp, -8
		0xafbf0000,                             // sw    ra, 0(sp)
		0x0c000000 + (libpsInit&0x03ffffff)>>2, // jal   libpsInit
		0x0,                                    // nop
		0x8fbf0000,                             // lw    ra, 0(sp)
		0x27bd0008,                             // addiu sp, sp, 8 (also fills the load delay slot)
		0x08000000 + (f.PC0&0x03ffffff)>>2,     // j     entry
		0x0,                                    // nop
	}
	patchData := make([]byte, 4*len(yarozePatch))
	for i, val := range yarozePatch {
		binary.LittleEndian.PutUint32(patchData[4*i:], val)
	}

	// The stub goes at the end of the text, which is where the entry point
	// moves to. The program's own entry, which the stub jumps to, is
	// wherever its crt0 is: at the start of the text for TinyGo's ELF, and
	// at the end for the SDK's, which links crt0 last.
	text := f.Section("text")
	f.FileHeader.PC0 = text.Addr + uint32(len(text.Data))
	f.FileHeader.TextSize += uint32(len(patchData))
	text.Data = append(text.Data, patchData...)
	return nil
}
