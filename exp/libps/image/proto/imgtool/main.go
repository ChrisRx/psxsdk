// imgtool flattens the prototype library image's ELF into a PSX-EXE at
// 0x80010000, every loaded section at its address and zeros between.
package main

import (
	"debug/elf"
	"fmt"
	"os"

	"go.chrisrx.dev/psx/format/psx"
)

func main() {
	f, err := elf.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	const base, top = 0x80010000, 0x80090000
	img := make([]byte, top-base)
	end := uint32(0)
	for _, s := range f.Sections {
		if s.Flags&elf.SHF_ALLOC == 0 || s.Size == 0 {
			continue
		}
		a := uint32(s.Addr)
		if a < base || a+uint32(s.Size) > top {
			panic(fmt.Sprintf("%s at %#x+%#x is outside the image", s.Name, a, s.Size))
		}
		if s.Type != elf.SHT_NOBITS {
			data, err := s.Data()
			if err != nil {
				panic(err)
			}
			copy(img[a-base:], data)
		}
		end = max(end, a+uint32(s.Size))
	}
	n := (end - base + 2047) &^ 2047
	exe := psx.New(base, 0x80010030, img[:n])
	exe.StackAddr = 0x801fff00
	if err := exe.WriteFile(os.Args[2]); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s: %d KB, image ends at %#x\n", os.Args[2], n/1024, end)
}
